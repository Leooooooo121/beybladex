package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config 代表整個系統的配置設定
type Config struct {
	Global    GlobalConfig    `json:"global"`
	Proxies   []string        `json:"proxies"`
	Notifiers NotifiersConfig `json:"notifiers"`
	Tasks     []TaskConfig    `json:"tasks"`
}

// GlobalConfig 全域運作參數
type GlobalConfig struct {
	PollIntervalMinMs int `json:"poll_interval_min_ms"` // 最小輪詢間隔 (毫秒)
	PollIntervalMaxMs int `json:"poll_interval_max_ms"` // 最大輪詢間隔 (毫秒)
	TimeoutSeconds    int `json:"timeout_seconds"`      // 請求超時時間 (秒)
	MaxRetries        int `json:"max_retries"`          // 重試次數
	BackoffMinSec     int `json:"backoff_min_sec"`      // 429/403 最小退避時間 (秒)
	BackoffMaxSec     int `json:"backoff_max_sec"`      // 429/403 最大退避時間 (秒)
	ChannelBufferSize int `json:"channel_buffer_size"`  // 事件緩衝區大小
	ProxyCooldownSec  int `json:"proxy_cooldown_sec"`   // 異常 Proxy 冷卻時間 (秒)
}

// NotifiersConfig 通知模組配置
type NotifiersConfig struct {
	Console           bool   `json:"console"`
	DiscordWebhookURL string `json:"discord_webhook_url"`
	TelegramBotToken  string `json:"telegram_bot_token"`
	TelegramChatID    string `json:"telegram_chat_id"`
}

// TaskConfig 單個監控任務配置
type TaskConfig struct {
	ID              string            `json:"id"`                         // 任務唯一 ID
	SiteType        string            `json:"site_type"`                  // 網站適配器類型 ("shopify", "bvshop", "funbox", "generic_api")
	Name            string            `json:"name"`                       // 任務名稱
	Enabled         bool              `json:"enabled"`                    // 是否啟用
	URL             string            `json:"url"`                        // 目標 API 網址
	Keyword         string            `json:"keyword,omitempty"`          // 搜尋關鍵字 (選填)
	FilterKeywords  []string          `json:"filter_keywords,omitempty"`  // 包含關鍵字 (選填)
	ExcludeKeywords []string          `json:"exclude_keywords,omitempty"` // 排除關鍵字 (例如 "限客訂")
	Headers         map[string]string `json:"headers,omitempty"`          // 自訂 Headers
	Cookies         string            `json:"cookies,omitempty"`          // 自訂 Cookie 字串
	PollIntervalMs  int               `json:"poll_interval_ms,omitempty"` // 單獨指定輪詢間隔 (0 則使用 Global)
	AutoAddToCart   bool              `json:"auto_add_to_cart,omitempty"` // 發現庫存時是否自動加入購物車 (ATC)
	SessionCookie   string            `json:"session_cookie,omitempty"`   // 登入狀態 Session Cookie
	CustomParams    map[string]string `json:"custom_params,omitempty"`    // 擴充自訂參數
}

// DefaultConfig 提供開箱即用的預設配置
func DefaultConfig() *Config {
	return &Config{
		Global: GlobalConfig{
			PollIntervalMinMs: 800,
			PollIntervalMaxMs: 1500,
			TimeoutSeconds:    10,
			MaxRetries:        3,
			BackoffMinSec:     2,
			BackoffMaxSec:     5,
			ChannelBufferSize: 2000,
			ProxyCooldownSec:  30,
		},
		Proxies: []string{},
		Notifiers: NotifiersConfig{
			Console:           true,
			DiscordWebhookURL: "",
			TelegramBotToken:  "",
			TelegramChatID:    "",
		},
		Tasks: []TaskConfig{
			{
				ID:            "mmtoyshop_beyblade",
				SiteType:      "bvshop",
				Name:          "M.M小舖 - 戰鬥陀螺",
				Enabled:       true,
				AutoAddToCart: true,
				URL:           "https://mmtoyshop.com/category/query?keyword=%E6%88%B0%E9%AC%A5%E9%99%80%E8%9E%BA",
				Headers: map[string]string{
					"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
					"Accept":          "application/json, text/plain, */*",
					"Accept-Language": "zh-TW,zh;q=0.9,en-US;q=0.8,en;q=0.7",
					"Referer":         "https://mmtoyshop.com/category?keyword=%E6%88%B0%E9%AC%A5%E9%99%80%E8%9E%BA",
				},
				ExcludeKeywords: []string{"限客訂"},
			},
			{
				ID:       "funbox_beyblade",
				SiteType: "funbox",
				Name:     "Funbox 麗嬰國際 - 戰鬥陀螺",
				Enabled:  true,
				URL:      "https://shop.funbox.com.tw/category_products/takaratomy/beyblade.json?limit=18&page=1&sort_by=sell_from-desc",
				Headers: map[string]string{
					"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
					"Accept":          "application/json, text/plain, */*",
					"Accept-Language": "zh-TW,zh;q=0.9,en-US;q=0.8,en;q=0.7",
					"Referer":         "https://shop.funbox.com.tw/categories/takaratomy/beyblade",
				},
			},
			{
				ID:       "shopify_demo",
				SiteType: "shopify",
				Name:     "Shopify 示範商城",
				Enabled:  false, // 預設關閉，由使用者依需開啟
				URL:      "https://kith.com/products.json?limit=30",
				Headers: map[string]string{
					"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
					"Accept":     "application/json, text/plain, */*",
				},
			},
		},
	}
}

// LoadConfig 從指定的 JSON 檔案路徑讀取配置，若檔案不存在則建立預設配置檔
func LoadConfig(filePath string) (*Config, error) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		cfg := DefaultConfig()
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("產生預設配置失敗: %w", err)
		}
		if err := os.WriteFile(filePath, data, 0644); err != nil {
			return nil, fmt.Errorf("寫入預設配置檔失敗: %w", err)
		}
		return cfg, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("讀取配置檔失敗: %w", err)
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置檔失敗: %w", err)
	}

	return cfg, nil
}
