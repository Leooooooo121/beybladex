package sites

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/url"
	"strconv"
	"strings"
	"time"

	"beybladex/config"
	"beybladex/models"
	"beybladex/proxy"

	fhttp "github.com/bogdanfinn/fhttp"
)

func init() {
	// 註冊 bvshop 與 generic_api
	RegisterMonitor("bvshop", func() SiteMonitor {
		return &BVShopMonitor{}
	})
	RegisterMonitor("generic_api", func() SiteMonitor {
		return &GenericAPIMonitor{}
	})
}

// =========================================================================
// BVShop / MMToyshop 專用分頁適配器
// =========================================================================

// BVShopMonitor 針對 BVShop 架構 (如 MMToyshop) 的多分頁庫存監控適配器
type BVShopMonitor struct {
	task     config.TaskConfig
	proxyMgr *proxy.ProxyManager
	timeout  time.Duration
}

func (b *BVShopMonitor) TaskID() string {
	return b.task.ID
}

func (b *BVShopMonitor) Name() string {
	return b.task.Name
}

func (b *BVShopMonitor) Init(task config.TaskConfig, proxyMgr *proxy.ProxyManager, timeout time.Duration) error {
	b.task = task
	b.proxyMgr = proxyMgr
	b.timeout = timeout
	return nil
}

// CheckStock 執行完整的 BVShop 分頁庫存檢查 (遵循反封控規範，加入分頁抖動延遲)
func (b *BVShopMonitor) CheckStock(ctx context.Context) ([]models.ProductStatus, error) {
	// 1. 請求第 1 頁
	page1URL, err := b.buildPageURL(b.task.URL, 1)
	if err != nil {
		return nil, fmt.Errorf("建置第 1 頁 URL 失敗: %w", err)
	}

	page1Resp, page1Items, err := b.fetchAndParsePage(ctx, page1URL)
	if err != nil {
		return nil, err
	}

	allResults := page1Items
	lastPage := page1Resp.LastPage
	if lastPage <= 1 {
		return allResults, nil
	}

	// 支援透過 custom_params["max_pages"] 限制最大抓取頁數 (預設爬到 lastPage)
	maxPages := lastPage
	if limitStr, exists := b.task.CustomParams["max_pages"]; exists {
		if limit, parseErr := strconv.Atoi(limitStr); parseErr == nil && limit > 0 && limit < maxPages {
			maxPages = limit
		}
	}

	// 2. 依序爬取剩餘頁面 (Page 2 .. maxPages)
	for p := 2; p <= maxPages; p++ {
		// 遵守反爬蟲規範：非連續無延遲呼叫，在分頁間加入隨機抖動延遲 (Jitter Delay)
		delay := b.calculatePageJitter()
		select {
		case <-ctx.Done():
			return allResults, ctx.Err()
		case <-time.After(delay):
		}

		pageURL, err := b.buildPageURL(b.task.URL, p)
		if err != nil {
			log.Printf("[WARN] [BVShop:%s] 建置第 %d 頁 URL 失敗: %v", b.Name(), p, err)
			continue
		}

		_, pageItems, err := b.fetchAndParsePage(ctx, pageURL)
		if err != nil {
			log.Printf("[WARN] [BVShop:%s] 抓取第 %d 頁失敗: %v", b.Name(), p, err)
			// 遇到頁面錯誤時記錄警告並保留已取得的資料，繼續下一頁或退出
			continue
		}

		allResults = append(allResults, pageItems...)
	}

	return allResults, nil
}

// fetchAndParsePage 發送單頁請求並解析商品資料
func (b *BVShopMonitor) fetchAndParsePage(ctx context.Context, targetURL string) (*models.BVShopResponse, []models.ProductStatus, error) {
	currentProxy, err := b.proxyMgr.GetNext()
	if err != nil {
		return nil, nil, fmt.Errorf("取得 Proxy 失敗: %w", err)
	}

	client, err := BuildTLSClient(currentProxy, b.timeout)
	if err != nil {
		return nil, nil, err
	}

	req, err := fhttp.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("建立 Request 失敗: %w", err)
	}

	b.applyHeaders(req)

	resp, err := client.Do(req)
	if err != nil {
		b.proxyMgr.ReportFailure(currentProxy, 0)
		return nil, nil, fmt.Errorf("BVShop 請求發送失敗 (%s): %w", targetURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 || resp.StatusCode == 429 {
		b.proxyMgr.ReportFailure(currentProxy, resp.StatusCode)
		return nil, nil, fmt.Errorf("遇到反爬防護/頻率限制 (HTTP %d)", resp.StatusCode)
	}

	if resp.StatusCode != 200 {
		return nil, nil, fmt.Errorf("BVShop API 回應異常狀態碼: %d", resp.StatusCode)
	}

	b.proxyMgr.ReportSuccess(currentProxy)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("讀取 Response 內容失敗: %w", err)
	}

	var bvResp models.BVShopResponse
	if err := json.Unmarshal(body, &bvResp); err != nil {
		return nil, nil, fmt.Errorf("BVShop JSON 解析失敗: %w", err)
	}

	items := b.convertBVShopProducts(bvResp.Products)
	return &bvResp, items, nil
}

// buildPageURL 為目標網址附加或替換 page 參數
func (b *BVShopMonitor) buildPageURL(rawURL string, page int) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("page", strconv.Itoa(page))
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// calculatePageJitter 計算分頁之間的隨機抖動間隔 (預設 600ms ~ 1200ms)
func (b *BVShopMonitor) calculatePageJitter() time.Duration {
	minMs := 600
	maxMs := 1200

	if valStr, ok := b.task.CustomParams["page_delay_min_ms"]; ok {
		if v, err := strconv.Atoi(valStr); err == nil && v > 0 {
			minMs = v
		}
	}
	if valStr, ok := b.task.CustomParams["page_delay_max_ms"]; ok {
		if v, err := strconv.Atoi(valStr); err == nil && v >= minMs {
			maxMs = v
		}
	}

	jitterMs := minMs + rand.Intn(maxMs-minMs+1)
	microJitter := rand.Intn(31) // 0~30ms 微抖動
	return time.Duration(jitterMs+microJitter) * time.Millisecond
}

func (b *BVShopMonitor) applyHeaders(req *fhttp.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-TW,zh;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Sec-Ch-Ua", `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")

	for k, v := range b.task.Headers {
		req.Header.Set(k, v)
	}

	if b.task.Cookies != "" {
		req.Header.Set("Cookie", b.task.Cookies)
	}
}

func (b *BVShopMonitor) convertBVShopProducts(products []models.BVShopProduct) []models.ProductStatus {
	var results []models.ProductStatus
	now := time.Now()

	for _, p := range products {
		if !b.matchesFilter(p.Title) {
			continue
		}

		itemURL := fmt.Sprintf("https://mmtoyshop.com/item/%s", p.Route)

		// 遍歷所有 Specs (規格)
		for _, s := range p.Specs {
			if b.isExcluded(s.SizeName) {
				continue
			}

			price := s.Price
			if s.SpecialPrice != nil && *s.SpecialPrice > 0 {
				price = *s.SpecialPrice
			} else if price == 0 && p.Price > 0 {
				price = p.Price
			}

			inStock := s.Quantity > 0

			results = append(results, models.ProductStatus{
				SiteName:    b.task.Name,
				ProductID:   strconv.Itoa(p.ID),
				Title:       p.Title,
				URL:         itemURL,
				ImageURL:    p.Photo,
				VariantID:   strconv.Itoa(s.ID),
				VariantName: s.SizeName,
				Price:       price,
				Currency:    "TWD",
				InStock:     inStock,
				Quantity:    s.Quantity,
				Timestamp:   now,
			})
		}
	}

	return results
}

func (b *BVShopMonitor) matchesFilter(title string) bool {
	if len(b.task.FilterKeywords) == 0 && b.task.Keyword == "" {
		return true
	}

	lowerTitle := strings.ToLower(title)
	if b.task.Keyword != "" && !strings.Contains(lowerTitle, strings.ToLower(b.task.Keyword)) {
		return false
	}

	for _, kw := range b.task.FilterKeywords {
		if !strings.Contains(lowerTitle, strings.ToLower(kw)) {
			return false
		}
	}

	return true
}

func (b *BVShopMonitor) isExcluded(text string) bool {
	for _, kw := range b.task.ExcludeKeywords {
		if strings.Contains(strings.ToLower(text), strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

// =========================================================================
// Generic JSON API 監控器 (通用 REST/JSON 監控)
// =========================================================================

// GenericAPIMonitor 通用 JSON API 適配器
type GenericAPIMonitor struct {
	task     config.TaskConfig
	proxyMgr *proxy.ProxyManager
	timeout  time.Duration
}

func (g *GenericAPIMonitor) TaskID() string {
	return g.task.ID
}

func (g *GenericAPIMonitor) Name() string {
	return g.task.Name
}

func (g *GenericAPIMonitor) Init(task config.TaskConfig, proxyMgr *proxy.ProxyManager, timeout time.Duration) error {
	g.task = task
	g.proxyMgr = proxyMgr
	g.timeout = timeout
	return nil
}

func (g *GenericAPIMonitor) CheckStock(ctx context.Context) ([]models.ProductStatus, error) {
	currentProxy, err := g.proxyMgr.GetNext()
	if err != nil {
		return nil, fmt.Errorf("取得 Proxy 失敗: %w", err)
	}

	client, err := BuildTLSClient(currentProxy, g.timeout)
	if err != nil {
		return nil, err
	}

	req, err := fhttp.NewRequestWithContext(ctx, "GET", g.task.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("建立 Request 失敗: %w", err)
	}

	for k, v := range g.task.Headers {
		req.Header.Set(k, v)
	}
	if g.task.Cookies != "" {
		req.Header.Set("Cookie", g.task.Cookies)
	}

	resp, err := client.Do(req)
	if err != nil {
		g.proxyMgr.ReportFailure(currentProxy, 0)
		return nil, fmt.Errorf("API 請求發送失敗: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 || resp.StatusCode == 429 {
		g.proxyMgr.ReportFailure(currentProxy, resp.StatusCode)
		return nil, fmt.Errorf("API 頻率限制/防護 (HTTP %d)", resp.StatusCode)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API 回應異常狀態碼: %d", resp.StatusCode)
	}

	g.proxyMgr.ReportSuccess(currentProxy)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("讀取 Response 失敗: %w", err)
	}

	// 嘗試解析為 BVShop 格式或通用陣列
	var bv models.BVShopResponse
	if err := json.Unmarshal(body, &bv); err == nil && len(bv.Products) > 0 {
		bvMonitor := &BVShopMonitor{task: g.task, proxyMgr: g.proxyMgr, timeout: g.timeout}
		items := bvMonitor.convertBVShopProducts(bv.Products)
		return items, nil
	}

	var shopify models.ShopifyProductsResponse
	if err := json.Unmarshal(body, &shopify); err == nil && len(shopify.Products) > 0 {
		shMonitor := &ShopifyMonitor{task: g.task, proxyMgr: g.proxyMgr, timeout: g.timeout}
		return shMonitor.parseProducts(body)
	}

	// 若非標準模型，回傳心跳成功
	return []models.ProductStatus{
		{
			SiteName:  g.task.Name,
			ProductID: "generic",
			Title:     "API 存活檢測 (HTTP 200 OK)",
			URL:       g.task.URL,
			InStock:   true,
			Quantity:  1,
			Timestamp: time.Now(),
		},
	}, nil
}
