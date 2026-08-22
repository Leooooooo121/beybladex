package engine

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"beyblade/config"
	"beyblade/models"
	"beyblade/notifier"
	"beyblade/proxy"
	"beyblade/sites"
)

// Engine 核心併發調度引擎
type Engine struct {
	cfg       *config.Config
	proxyMgr  *proxy.ProxyManager
	notifier  notifier.Notifier
	eventChan chan models.ProductStatus
	monitors  []sites.SiteMonitor
	stockMap  sync.Map // 快取商品歷史庫存狀態: key(string) -> inStock(bool)
	wg        sync.WaitGroup
	verbose   bool

	// 統計數據 (線程安全原子計數)
	startTime      time.Time
	totalPolls     atomic.Uint64
	totalRestocks  atomic.Uint64
	totalErrors    atomic.Uint64
	totalLatencyMs atomic.Uint64
}

// NewEngine 建立並初始化引擎
func NewEngine(cfg *config.Config, proxyMgr *proxy.ProxyManager, notif notifier.Notifier, verbose bool) (*Engine, error) {
	bufferSize := cfg.Global.ChannelBufferSize
	if bufferSize <= 0 {
		bufferSize = 1000
	}

	eng := &Engine{
		cfg:       cfg,
		proxyMgr:  proxyMgr,
		notifier:  notif,
		eventChan: make(chan models.ProductStatus, bufferSize),
		monitors:  make([]sites.SiteMonitor, 0),
		verbose:   verbose,
		startTime: time.Now(),
	}

	// 依據 Task 設定實例化網站監控器
	timeout := time.Duration(cfg.Global.TimeoutSeconds) * time.Second
	for _, task := range cfg.Tasks {
		if !task.Enabled {
			continue
		}

		mon, err := sites.CreateMonitor(task, proxyMgr, timeout)
		if err != nil {
			return nil, fmt.Errorf("載入任務 [%s] 失敗: %w", task.Name, err)
		}

		eng.monitors = append(eng.monitors, mon)
	}

	return eng, nil
}

// Start 啟動所有監控 Worker 與事件處理中樞
func (e *Engine) Start(ctx context.Context) error {
	if len(e.monitors) == 0 {
		return fmt.Errorf("沒有任何啟用的監控任務")
	}

	log.Printf("[INFO] [Engine] 正在啟動引擎，已載入 %d 個監控任務 | 代理數量: %d",
		len(e.monitors), e.proxyMgr.TotalCount())

	// 啟動通知事件消費 Listener
	notifier.StartListener(ctx, e.eventChan, e.notifier)

	// 啟動定期健康狀態看板 (每 30 秒自動輸出一次統計摘要)
	go e.startStatsDashboard(ctx)

	// 為每個監控任務啟動獨立的 Worker Goroutine
	for _, mon := range e.monitors {
		e.wg.Add(1)
		go e.runWorker(ctx, mon)
	}

	// 等待 Context 取消
	<-ctx.Done()
	log.Println("[INFO] [Engine] 收到終止訊號，正在等待所有 Worker 安全退出...")
	e.wg.Wait()
	close(e.eventChan)
	log.Println("[INFO] [Engine] 所有 Worker 已安全終止，資料流關閉完畢。")

	return nil
}

// runWorker 單一監控任務的調度迴圈 (包含 Jitter 抖動、異常退避與狀態去重)
func (e *Engine) runWorker(ctx context.Context, mon sites.SiteMonitor) {
	defer e.wg.Done()

	log.Printf("[INFO] [Worker:%s] 任務啟動 (輪詢間隔: %d~%d ms)", mon.Name(),
		e.cfg.Global.PollIntervalMinMs, e.cfg.Global.PollIntervalMaxMs)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[INFO] [Worker:%s] 任務停止", mon.Name())
			return
		default:
		}

		reqStart := time.Now()
		products, err := mon.CheckStock(ctx)
		latency := time.Since(reqStart)

		e.totalPolls.Add(1)
		e.totalLatencyMs.Add(uint64(latency.Milliseconds()))

		if err != nil {
			e.totalErrors.Add(1)

			// 異常處理與隨機退避 (Backoff: 2s ~ 5s)
			backoffSec := e.cfg.Global.BackoffMinSec
			if diff := e.cfg.Global.BackoffMaxSec - e.cfg.Global.BackoffMinSec; diff > 0 {
				backoffSec += rand.Intn(diff + 1)
			}
			backoffDuration := time.Duration(backoffSec) * time.Second

			log.Printf("[WARN] [Worker:%s] 檢查失敗 (%v) -> 觸發退避冷卻 %v 後重試",
				mon.Name(), err, backoffDuration)

			select {
			case <-ctx.Done():
				return
			case <-time.After(backoffDuration):
				continue
			}
		}

		// 處理商品狀態與庫存變更比對 (State Caching & Deduplication)
		inStockCount := 0
		newRestockCount := 0

		for _, p := range products {
			if p.InStock {
				inStockCount++
			}

			key := p.UniqueKey()
			val, existed := e.stockMap.Load(key)

			if !existed {
				// 第一次掃描發現此商品
				e.stockMap.Store(key, p.InStock)
				if p.InStock {
					newRestockCount++
					e.totalRestocks.Add(1)
					e.dispatch(p)
				}
			} else {
				wasInStock := val.(bool)
				if !wasInStock && p.InStock {
					// 狀態由「無庫存」變為「有庫存」 -> 觸發補貨通知！
					newRestockCount++
					e.totalRestocks.Add(1)
					e.dispatch(p)
				}
				e.stockMap.Store(key, p.InStock)
			}
		}

		// 若啟用詳細日誌模式 (-v)，印出每次輪詢結果
		if e.verbose {
			ts := time.Now().Format("15:04:05.000")
			log.Printf("[%s] [POLL] [%s] 掃描完成 (耗時: %dms | 規格總數: %d | 有庫存: %d)",
				ts, mon.Name(), latency.Milliseconds(), len(products), inStockCount)
		}

		// 計算隨機時間抖動 (Jitter)
		sleepDuration := e.calculateJitter()

		select {
		case <-ctx.Done():
			return
		case <-time.After(sleepDuration):
		}
	}
}

// dispatch 將補貨事件發送到 Channel，非阻塞防止 Worker 卡死
func (e *Engine) dispatch(status models.ProductStatus) {
	select {
	case e.eventChan <- status:
	default:
		log.Printf("[WARN] [Engine] 事件緩衝區已滿，丟棄事件: %s", status.Title)
	}
}

// calculateJitter 計算隨機輪詢間隔 (例如 800ms ~ 1500ms + 隨機微秒抖動)
func (e *Engine) calculateJitter() time.Duration {
	minMs := e.cfg.Global.PollIntervalMinMs
	maxMs := e.cfg.Global.PollIntervalMaxMs

	if minMs <= 0 {
		minMs = 800
	}
	if maxMs < minMs {
		maxMs = minMs + 500
	}

	jitterMs := minMs + rand.Intn(maxMs-minMs+1)
	microJitter := rand.Intn(41) + 10
	return time.Duration(jitterMs+microJitter) * time.Millisecond
}

// startStatsDashboard 定期印出運行狀態看板
func (e *Engine) startStatsDashboard(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.PrintStats()
		}
	}
}

// PrintStats 印出當前系統運作指標看板
func (e *Engine) PrintStats() {
	uptime := time.Since(e.startTime).Truncate(time.Second)
	polls := e.totalPolls.Load()
	errors := e.totalErrors.Load()
	restocks := e.totalRestocks.Load()
	latencySum := e.totalLatencyMs.Load()

	avgLatency := int64(0)
	if polls > 0 {
		avgLatency = int64(latencySum / polls)
	}

	// 計算當前追蹤的規格數量
	trackedItems := 0
	e.stockMap.Range(func(key, value interface{}) bool {
		trackedItems++
		return true
	})

	fmt.Println("\n---------------------------------------------------------------")
	fmt.Printf("📊 【系統運作健康看板】(運行時間: %v)\n", uptime)
	fmt.Printf("🚀 運行中任務數: %d 個 | 代理池可用: %d/%d\n",
		len(e.monitors), e.proxyMgr.ActiveCount(), e.proxyMgr.TotalCount())
	fmt.Printf("⚡ 累計輪詢次數: %d 次 | 平均連線延遲: %d ms\n", polls, avgLatency)
	fmt.Printf("📦 監控中規格數: %d 項 | 累計補貨推播: %d 次\n", trackedItems, restocks)
	fmt.Printf("🛡️ 異常/封鎖次數: %d 次\n", errors)
	fmt.Println("---------------------------------------------------------------")
}

// RunHealthCheck 對所有啟用的任務執行一次診斷測試並輸出完整結果
func RunHealthCheck(cfg *config.Config, proxyMgr *proxy.ProxyManager) error {
	timeout := time.Duration(cfg.Global.TimeoutSeconds) * time.Second
	fmt.Println("\n🔍 ================== 系統診斷模式 (Health Check) ==================")
	fmt.Printf("📅 檢測時間: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("🌐 代理池配置: %d 個 Proxy\n\n", proxyMgr.TotalCount())

	ctx, cancel := context.WithTimeout(context.Background(), timeout*time.Duration(len(cfg.Tasks)+1))
	defer cancel()

	successCount := 0
	failCount := 0

	for _, task := range cfg.Tasks {
		if !task.Enabled {
			fmt.Printf("⚪ [任務跳過] %s (已停用)\n\n", task.Name)
			continue
		}

		fmt.Printf("▶️ [正在檢測] 任務: %s (%s)\n", task.Name, task.SiteType)
		fmt.Printf("🔗 目標網址: %s\n", task.URL)

		mon, err := sites.CreateMonitor(task, proxyMgr, timeout)
		if err != nil {
			fmt.Printf("❌ 建立適配器失敗: %v\n\n", err)
			failCount++
			continue
		}

		start := time.Now()
		products, err := mon.CheckStock(ctx)
		elapsed := time.Since(start)

		if err != nil {
			fmt.Printf("❌ 請求失敗: %v (耗時: %v)\n\n", err, elapsed)
			failCount++
			continue
		}

		successCount++
		inStockCount := 0
		for _, p := range products {
			if p.InStock {
				inStockCount++
			}
		}

		fmt.Printf("✅ 連線成功！ (耗時: %d ms | 掃描規格總數: %d 項 | 有庫存: %d 項)\n",
			elapsed.Milliseconds(), len(products), inStockCount)

		// 印出前 5 個商品範例
		showLimit := 5
		if len(products) < showLimit {
			showLimit = len(products)
		}
		if showLimit > 0 {
			fmt.Println("   📋 商品抽樣預覽:")
			for i := 0; i < showLimit; i++ {
				p := products[i]
				stockStr := "❌ 無庫存"
				if p.InStock {
					stockStr = fmt.Sprintf("✅ 有庫存 (數量: %d)", p.Quantity)
				}
				fmt.Printf("   - [%s] %s | 規格: %s | 價格: %s %.2f | %s\n",
					stockStr, p.Title, p.VariantName, p.Currency, p.Price, p.URL)
			}
			if len(products) > showLimit {
				fmt.Printf("   ... 其餘 %d 項已略過\n", len(products)-showLimit)
			}
		}
		fmt.Println()
	}

	fmt.Println("================================================================")
	fmt.Printf("🏁 診斷完成: 成功 %d 個任務 / 失敗 %d 個任務\n", successCount, failCount)
	fmt.Println("================================================================")

	return nil
}
