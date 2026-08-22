package engine

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sync"
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
}

// NewEngine 建立並初始化引擎
func NewEngine(cfg *config.Config, proxyMgr *proxy.ProxyManager, notif notifier.Notifier) (*Engine, error) {
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

	log.Printf("[INFO] [Worker:%s] 任務啟動", mon.Name())

	for {
		select {
		case <-ctx.Done():
			log.Printf("[INFO] [Worker:%s] 任務停止", mon.Name())
			return
		default:
		}

		startTime := time.Now()
		products, err := mon.CheckStock(ctx)

		if err != nil {
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
		restockCount := 0
		for _, p := range products {
			key := p.UniqueKey()
			val, existed := e.stockMap.Load(key)

			if !existed {
				// 第一次掃描發現此商品
				e.stockMap.Store(key, p.InStock)
				if p.InStock {
					restockCount++
					e.dispatch(p)
				}
			} else {
				wasInStock := val.(bool)
				if !wasInStock && p.InStock {
					// 狀態由「無庫存」變為「有庫存」 -> 觸發補貨通知！
					restockCount++
					e.dispatch(p)
				}
				e.stockMap.Store(key, p.InStock)
			}
		}

		elapsed := time.Since(startTime)
		_ = elapsed

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
	// 額外加入 10-50ms 隨機微抖動以打亂 TLS 特徵
	microJitter := rand.Intn(41) + 10
	return time.Duration(jitterMs+microJitter) * time.Millisecond
}
