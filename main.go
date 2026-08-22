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
	flag.Parse()

	// 1. 讀取設定檔
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("[FATAL] 載入配置檔失敗: %v", err)
	}

	log.Printf("[INFO] 成功載入配置檔: %s (已啟用任務數: %d)", *configPath, countEnabledTasks(cfg))

	// 2. 初始化線程安全 Proxy Manager
	cooldown := time.Duration(cfg.Global.ProxyCooldownSec) * time.Second
	proxyMgr := proxy.NewProxyManager(cfg.Proxies, cooldown)
	if proxyMgr.TotalCount() > 0 {
		log.Printf("[INFO] 代理池初始化完成，載入 %d 個 Proxy (冷卻期: %v)", proxyMgr.TotalCount(), cooldown)
	} else {
		log.Printf("[INFO] 未指定 Proxy，採用本機直接連線模式")
	}

	// 3. 初始化通知發送模組 (Console, Discord, Telegram)
	notif := notifier.NewMultiNotifier(cfg.Notifiers)

	// 4. 建立調度引擎
	eng, err := engine.NewEngine(cfg, proxyMgr, notif)
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
