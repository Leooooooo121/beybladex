package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"beyblade/config"
	"beyblade/engine"
	"beyblade/models"
	"beyblade/notifier"
	"beyblade/proxy"
)

const banner = `
===============================================================
  ⚡ 多平台高併發購物網站補貨監控系統 (Golang Restock Monitor)
  🚀 核心特性: 毫秒級輪詢 | TLS 指紋偽裝 (Chrome 120+) | 代理池輪詢
===============================================================
`

func main() {
	fmt.Print(banner)

	configPath := flag.String("config", "config.json", "設定檔路徑")
	testNotify := flag.Bool("test-notify", false, "發送測試通知至已啟用的管道 (Telegram / Discord / Console)")
	checkMode := flag.Bool("check", false, "單次診斷檢查模式 (即時檢測所有目標網站連線與庫存狀態並輸出報告)")
	verbose := flag.Bool("v", false, "詳細日誌模式 (印出每次輪詢毫秒耗時與掃描結果)")
	flag.Parse()

	// 1. 讀取設定檔
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("[FATAL] 載入配置檔失敗: %v", err)
	}

	log.Printf("[INFO] 成功載入配置檔: %s (已啟用任務數: %d)", *configPath, countEnabledTasks(cfg))

	// 2. 初始化通知發送模組 (Console, Discord, Telegram)
	notif := notifier.NewMultiNotifier(cfg.Notifiers)

	// 測試通知模式
	if *testNotify {
		log.Println("[INFO] 正在發送測試補貨通知訊息...")
		testStatus := models.ProductStatus{
			SiteName:    "測試通知平台",
			ProductID:   "TEST-001",
			Title:       "【測試商品】TAKARA TOMY 戰鬥陀螺 UX 測試通知",
			URL:         "https://mmtoyshop.com",
			VariantID:   "VAR-999",
			VariantName: "現貨測試規格",
			Price:       295.00,
			Currency:    "TWD",
			InStock:     true,
			Quantity:    99,
			Timestamp:   time.Now(),
		}
		if err := notif.Send(testStatus); err != nil {
			log.Printf("[ERROR] 測試通知發送失敗: %v", err)
		} else {
			log.Println("[INFO] 測試通知發送指令已完成！請檢查 Telegram / Discord / 終端機。")
		}
		return
	}

	// 3. 初始化線程安全 Proxy Manager
	cooldown := time.Duration(cfg.Global.ProxyCooldownSec) * time.Second
	proxyMgr := proxy.NewProxyManager(cfg.Proxies, cooldown)
	if proxyMgr.TotalCount() > 0 {
		log.Printf("[INFO] 代理池初始化完成，載入 %d 個 Proxy (冷卻期: %v)", proxyMgr.TotalCount(), cooldown)
	} else {
		log.Printf("[INFO] 未指定 Proxy，採用本機直接連線模式")
	}

	// 單次診斷檢查模式 (-check)
	if *checkMode {
		if err := engine.RunHealthCheck(cfg, proxyMgr); err != nil {
			log.Fatalf("[FATAL] 診斷檢測異常: %v", err)
		}
		return
	}

	// 4. 建立調度引擎
	eng, err := engine.NewEngine(cfg, proxyMgr, notif, *verbose)
	if err != nil {
		log.Fatalf("[FATAL] 引擎初始化失敗: %v", err)
	}

	// 5. 設定優雅停機訊號監聽
	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("[INFO] 接收到系統中斷訊號 (%v)，正在停止監控引擎...", sig)
		cancel()
	}()

	// 6. 啟動監控引擎 (阻塞運行)
	if err := eng.Start(ctx); err != nil {
		log.Fatalf("[FATAL] 引擎運行異常: %v", err)
	}

	log.Println("[INFO] 程式正常退出。")
}

func countEnabledTasks(cfg *config.Config) int {
	count := 0
	for _, t := range cfg.Tasks {
		if t.Enabled {
			count++
		}
	}
	return count
}
