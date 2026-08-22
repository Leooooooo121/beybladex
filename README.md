# ⚡ 多平台高併發購物網站補貨監控系統 (Golang Restock Monitor)

一套基於 **Golang** 從零開發的高效能、低延遲（毫秒級）、模組化多平台購物網站庫存與補貨即時監控系統。

具備 **TLS 瀏覽器指紋偽裝 (Chrome 120+)**、**線程安全代理池 (Proxy Manager)**、**隨機間隔抖動 (Jitter)**、**異常退避 (Backoff)**、**高併發調度引擎 (Worker Pool)** 以及 **多管道即時推播通知 (Console 毫秒日誌、Discord Webhook、Telegram Bot)**。

---

## 🌟 核心特性

- 🧩 **模組化策略模式 (Strategy Pattern)**：定義標準 `SiteMonitor` 介面與註冊工廠，擴充新網站監控只需實作適配器，不需修改核心調度邏輯。
- 🛡️ **進階反防護與 TLS 偽裝**：全面採用 `github.com/bogdanfinn/tls-client`，精準模擬 Chrome 120+ 的 TLS Client Hello 指紋與 HTTP/2 特性，大幅降低 Cloudflare / Akamai 等反爬蟲風控攔截率。
- 🔄 **線程安全代理池 (Proxy Manager)**：
  - 支援 Round-Robin 負載輪詢。
  - 當遭遇 403 / 429 狀態碼或網路超時時，自動標記並將該代理隔離至冷卻期（Cooldown），冷卻結束自動恢復。
  - 支援無 Proxy 模式與多 Proxy 混合運作。
- ⏱️ **請求抖動與異常退避 (Jitter & Backoff)**：
  - 輪詢間隔具備隨機微秒抖動（例如 800ms ~ 1500ms + 微抖動），打亂固定頻率特徵。
  - 遇到頻率限制時自動進行 2 ~ 5 秒隨機退避重試。
- ⚡ **高效併發調度與狀態去重**：
  - 每個監控任務由獨立 Goroutine Worker 併發執行。
  - 狀態快取表自動去重，僅在「無庫存 ➔ 有庫存」或「新上架」時派發事件，避免重覆發送警報。
  - 嚴格的資源釋放，請求後顯式呼叫 `resp.Body.Close()`，避免 Socket 耗盡與記憶體洩漏。
- 📢 **多管道即時通知 (Notifier)**：
  - **Console**：輸出毫秒級精確時間戳記日誌。
  - **Discord**：發送豐富排版的 Embed 卡片（含商品名、款式、價格、庫存與直達網址）。
  - **Telegram**：發送格式化 HTML 補貨訊息。

---

## 📁 專案目錄結構

```text
.
├── config/
│   └── config.go          # 任務配置模型、Proxy 設定、全域參數與 JSON 讀取
├── models/
│   ├── product.go         # 統一的 ProductStatus 事件結構、Shopify/BVShop 資料模型
│   └── product_test.go    # 模型反序列化與 UniqueKey 單元測試
├── proxy/
│   └── proxy.go           # 線程安全 Round-Robin Proxy Manager、IP 剔除與冷卻機制
│   └── proxy_test.go      # 代理輪詢與冷卻隔離單元測試
├── sites/
│   ├── site.go            # SiteMonitor Interface 定義、適配器註冊工廠、TLS Client 建置
│   ├── shopify.go         # Shopify REST API 庫存監控適配器
│   ├── generic_api.go     # BVShop (MMToyShop) 專屬與通用 JSON API 監控適配器
│   └── sites_test.go      # 工廠註冊與過濾條件單元測試
├── engine/
│   ├── engine.go          # 核心併發調度器、Worker Pool、狀態快取去重、隨機抖動與退避
│   └── engine_test.go     # 引擎生命週期與事件發送單元測試
├── notifier/
│   └── notifier.go        # 事件監聽器、Console (毫秒日誌)、Discord / Telegram Webhook
├── config.json            # 系統設定檔
├── main.go                # 程式進入點、優雅停機 (Graceful Shutdown)
├── go.mod                 # Go 模組定義
└── README.md              # 專案說明文件
```

---

## 🛠️ 環境需求

- **Go**: 1.20 或更高版本 (已在 Go 1.27 測試通過)
- 作業系統：Windows / Linux / macOS

---

## 🚀 快速開始

### 1. 下載與安裝依賴

```bash
git clone <your-repository-url>
cd beybladex
go mod tidy
```

### 2. 設定 `config.json`

在專案根目錄下建立或修改 `config.json`：

```json
{
  "global": {
    "poll_interval_min_ms": 800,
    "poll_interval_max_ms": 1500,
    "timeout_seconds": 10,
    "max_retries": 3,
    "backoff_min_sec": 2,
    "backoff_max_sec": 5,
    "channel_buffer_size": 2000,
    "proxy_cooldown_sec": 30
  },
  "proxies": [
    "http://username:password@ip:port",
    "http://ip:port"
  ],
  "notifiers": {
    "console": true,
    "discord_webhook_url": "https://discord.com/api/webhooks/YOUR_WEBHOOK_URL",
    "telegram_bot_token": "YOUR_TELEGRAM_BOT_TOKEN",
    "telegram_chat_id": "YOUR_TELEGRAM_CHAT_ID"
  },
  "tasks": [
    {
      "id": "mmtoyshop_beyblade",
      "site_type": "bvshop",
      "name": "M.M小舖 - 戰鬥陀螺",
      "enabled": true,
      "url": "https://mmtoyshop.com/category/query?keyword=%E6%88%B0%E9%AC%A5%E9%99%80%E8%9E%BA",
      "headers": {
        "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
        "Accept": "application/json, text/plain, */*",
        "Accept-Language": "zh-TW,zh;q=0.9,en-US;q=0.8,en;q=0.7",
        "Referer": "https://mmtoyshop.com/category?keyword=%E6%88%B0%E9%AC%A5%E9%99%80%E8%9E%BA"
      },
      "cookies": "",
      "exclude_keywords": [
        "限客訂"
      ]
    },
    {
      "id": "shopify_demo",
      "site_type": "shopify",
      "name": "Shopify 示範商城",
      "enabled": false,
      "url": "https://example-shopify-store.com/products.json?limit=30",
      "filter_keywords": ["Beyblade", "UX-01"],
      "exclude_keywords": ["Sold Out"]
    }
  ]
}
```

### 3. 編譯與執行

**直接執行：**
```powershell
go run main.go
```

**發送測試通知（驗證 Telegram / Discord 設定）：**
```powershell
go run main.go -test-notify
# 或
.\monitor.exe -test-notify
```

**指定自訂設定檔路徑：**
```powershell
go run main.go -config /path/to/my_config.json
```

**編譯為執行檔：**
```powershell
go build -o monitor.exe .
.\monitor.exe
```

**優雅停機：**
按下 `Ctrl + C`，系統會捕捉中斷訊號，安全等待所有正在執行的 Worker 結束並釋放資源後正常退出。

---

## ⚙️ 配置參數說明

| 參數分類 | 參數名稱 | 型別 | 說明 |
| :--- | :--- | :--- | :--- |
| **global** | `poll_interval_min_ms` | int | 最小輪詢間隔（毫秒，預設 `800`） |
| | `poll_interval_max_ms` | int | 最大輪詢間隔（毫秒，預設 `1500`） |
| | `timeout_seconds` | int | 單次 HTTP 請求超時時間（秒） |
| | `backoff_min_sec` | int | 遭遇 429/403 時最小退避時間（秒，預設 `2`） |
| | `backoff_max_sec` | int | 遭遇 429/403 時最大退避時間（秒，預設 `5`） |
| | `channel_buffer_size` | int | 事件通知 Channel 緩衝區大小（預設 `2000`） |
| | `proxy_cooldown_sec` | int | 異常 Proxy 隔離冷卻時間（秒，預設 `30`） |
| **proxies** | - | `[]string` | 代理列表，為空時使用本機連線 |
| **notifiers** | `console` | bool | 是否啟用終端機日誌 |
| | `discord_webhook_url`| string | Discord Webhook 網址（留空則停用） |
| | `telegram_bot_token` | string | Telegram Bot Token（留空則停用） |
| | `telegram_chat_id` | string | Telegram 接收訊息的 Chat ID |
| **tasks** | `id` | string | 任務唯一識別碼 |
| | `site_type` | string | 適配器類型：`bvshop` / `shopify` / `generic_api` |
| | `name` | string | 任務自訂名稱 |
| | `enabled` | bool | 是否啟用該任務 |
| | `url` | string | 目標 API 網址 |
| | `filter_keywords` | `[]string` | 包含關鍵字過濾（選填） |
| | `exclude_keywords` | `[]string` | 排除關鍵字過濾（如 `["限客訂"]`） |
| | `headers` | `object` | 自訂 HTTP Headers |
| | `cookies` | string | 自訂 Cookie 字串 |
| | `custom_params` | `object` | 擴充參數（如 `"max_pages": "5"`, `"page_delay_min_ms": "600"`, `"page_delay_max_ms": "1200"`） |

---

### 📄 BVShop (MMToyShop) 多分頁爬取與反封控說明

- 系統自動解析第一頁的回應中的 `lastPage`。
- 若 `lastPage > 1`，會依序發送 `&page=2` 直到 `lastPage`。
- **嚴格反封控機制**：每爬取一個分頁之間，均會加入隨機微秒抖動延遲（預設 `600ms ~ 1200ms + 微抖動`），且每次分頁請求皆會透過代理池輪詢切換 Proxy，杜絕連續請求觸發 Cloudflare 防護。


---

## 🔌 如何擴充新的購物網站適配器

採用策略模式，新增適配器只需三步：

1. 在 `sites/` 目錄下建立新檔案（如 `sites/my_shop.go`）。
2. 實作 `SiteMonitor` 介面：
   ```go
   package sites

   import (
       "context"
       "time"
       "beyblade/config"
       "beyblade/models"
       "beyblade/proxy"
   )

   func init() {
       // 註冊適配器名稱
       RegisterMonitor("my_shop", func() SiteMonitor {
           return &MyShopMonitor{}
       })
   }

   type MyShopMonitor struct {
       task     config.TaskConfig
       proxyMgr *proxy.ProxyManager
       timeout  time.Duration
   }

   func (m *MyShopMonitor) TaskID() string { return m.task.ID }
   func (m *MyShopMonitor) Name() string   { return m.task.Name }
   func (m *MyShopMonitor) Init(task config.TaskConfig, proxyMgr *proxy.ProxyManager, timeout time.Duration) error {
       m.task = task
       m.proxyMgr = proxyMgr
       m.timeout = timeout
       return nil
   }

   func (m *MyShopMonitor) CheckStock(ctx context.Context) ([]models.ProductStatus, error) {
       // 1. 取得 Proxy
       currentProxy, _ := m.proxyMgr.GetNext()
       
       // 2. 建立 TLS Client (Chrome 120 指紋)
       client, err := BuildTLSClient(currentProxy, m.timeout)
       if err != nil { return nil, err }
       
       // 3. 發送請求並解析商品資料
       // ...
       return results, nil
   }
   ```
3. 在 `config.json` 的 `tasks` 中指定 `"site_type": "my_shop"` 即可立即啟用！

---

## 🧪 執行單元測試

專案附帶完整的單元測試套件，涵蓋代理池輪詢、冷卻隔離、狀態去重、資料模型反序列化與適配器過濾邏輯：

```powershell
go test -v ./...
```

**測試結果範例：**
```text
=== RUN   TestEngine_LifecycleAndDeduplication
--- PASS: TestEngine_LifecycleAndDeduplication (0.35s)
=== RUN   TestProductStatus_UniqueKey
--- PASS: TestProductStatus_UniqueKey (0.00s)
=== RUN   TestBVShop_Unmarshal
--- PASS: TestBVShop_Unmarshal (0.00s)
=== RUN   TestShopify_Unmarshal
--- PASS: TestShopify_Unmarshal (0.00s)
=== RUN   TestProxyManager_RoundRobin
--- PASS: TestProxyManager_RoundRobin (0.00s)
=== RUN   TestProxyManager_CooldownAndBan
--- PASS: TestProxyManager_CooldownAndBan (0.32s)
=== RUN   TestSiteMonitor_Factory
--- PASS: TestSiteMonitor_Factory (0.00s)
=== RUN   TestBVShop_ParseAndFilter
--- PASS: TestBVShop_ParseAndFilter (0.00s)
PASS
ok      beyblade/engine 1.494s
ok      beyblade/models 0.340s
ok      beyblade/proxy  0.578s
ok      beyblade/sites  0.761s
```

---

## 📄 License

MIT License
