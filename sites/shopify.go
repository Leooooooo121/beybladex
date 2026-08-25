package sites

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"beybladex/config"
	"beybladex/models"
	"beybladex/proxy"

	fhttp "github.com/bogdanfinn/fhttp"
)

func init() {
	RegisterMonitor("shopify", func() SiteMonitor {
		return &ShopifyMonitor{}
	})
}

// ShopifyMonitor Shopify 平台監控器
type ShopifyMonitor struct {
	task     config.TaskConfig
	proxyMgr *proxy.ProxyManager
	timeout  time.Duration
}

func (s *ShopifyMonitor) TaskID() string {
	return s.task.ID
}

func (s *ShopifyMonitor) Name() string {
	return s.task.Name
}

func (s *ShopifyMonitor) Init(task config.TaskConfig, proxyMgr *proxy.ProxyManager, timeout time.Duration) error {
	s.task = task
	s.proxyMgr = proxyMgr
	s.timeout = timeout
	return nil
}

// CheckStock 執行單次 Shopify 庫存檢查
func (s *ShopifyMonitor) CheckStock(ctx context.Context) ([]models.ProductStatus, error) {
	currentProxy, err := s.proxyMgr.GetNext()
	if err != nil {
		return nil, fmt.Errorf("取得 Proxy 失敗: %w", err)
	}

	client, err := BuildTLSClient(currentProxy, s.timeout)
	if err != nil {
		return nil, err
	}

	req, err := fhttp.NewRequestWithContext(ctx, "GET", s.task.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("建立 Request 失敗: %w", err)
	}

	// 設定擬真瀏覽器 Header
	s.applyHeaders(req)

	resp, err := client.Do(req)
	if err != nil {
		s.proxyMgr.ReportFailure(currentProxy, 0)
		return nil, fmt.Errorf("Shopify 請求發送失敗: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 || resp.StatusCode == 429 {
		s.proxyMgr.ReportFailure(currentProxy, resp.StatusCode)
		return nil, fmt.Errorf("遇到反爬防護/頻率限制 (HTTP %d)", resp.StatusCode)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Shopify API 回應異常狀態碼: %d", resp.StatusCode)
	}

	s.proxyMgr.ReportSuccess(currentProxy)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("讀取 Response 內容失敗: %w", err)
	}

	return s.parseProducts(body)
}

func (s *ShopifyMonitor) applyHeaders(req *fhttp.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,zh-TW;q=0.8,zh;q=0.7")
	req.Header.Set("Sec-Ch-Ua", `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")

	// 覆蓋自訂 Header
	for k, v := range s.task.Headers {
		req.Header.Set(k, v)
	}

	if s.task.Cookies != "" {
		req.Header.Set("Cookie", s.task.Cookies)
	}
}

func (s *ShopifyMonitor) parseProducts(data []byte) ([]models.ProductStatus, error) {
	// 嘗試解析多商品格式
	var multiResp models.ShopifyProductsResponse
	if err := json.Unmarshal(data, &multiResp); err == nil && len(multiResp.Products) > 0 {
		return s.convertShopifyProducts(multiResp.Products), nil
	}

	// 嘗試解析單商品格式
	var singleResp models.ShopifySingleProductResponse
	if err := json.Unmarshal(data, &singleResp); err == nil && singleResp.Product.ID != 0 {
		return s.convertShopifyProducts([]models.ShopifyProduct{singleResp.Product}), nil
	}

	return nil, fmt.Errorf("無法解析 Shopify JSON 格式")
}

func (s *ShopifyMonitor) convertShopifyProducts(products []models.ShopifyProduct) []models.ProductStatus {
	var results []models.ProductStatus
	now := time.Now()

	for _, p := range products {
		if !s.matchesFilter(p.Title) {
			continue
		}

		imgURL := ""
		if len(p.Images) > 0 {
			imgURL = p.Images[0].SRC
		}

		for _, v := range p.Variants {
			if s.isExcluded(v.Title) {
				continue
			}

			price, _ := strconv.ParseFloat(v.Price, 64)
			qty := v.InventoryQuantity
			if qty <= 0 && v.Available {
				qty = 1
			}

			results = append(results, models.ProductStatus{
				SiteName:    s.task.Name,
				ProductID:   strconv.FormatInt(p.ID, 10),
				Title:       p.Title,
				URL:         fmt.Sprintf("%s/products/%s", strings.TrimRight(s.task.URL, "/products.json"), p.Handle),
				ImageURL:    imgURL,
				VariantID:   strconv.FormatInt(v.ID, 10),
				VariantName: v.Title,
				Price:       price,
				Currency:    "USD",
				InStock:     v.Available || v.InventoryQuantity > 0,
				Quantity:    qty,
				Timestamp:   now,
			})
		}
	}

	return results
}

func (s *ShopifyMonitor) matchesFilter(title string) bool {
	if len(s.task.FilterKeywords) == 0 && s.task.Keyword == "" {
		return true
	}

	lowerTitle := strings.ToLower(title)
	if s.task.Keyword != "" && !strings.Contains(lowerTitle, strings.ToLower(s.task.Keyword)) {
		return false
	}

	for _, kw := range s.task.FilterKeywords {
		if !strings.Contains(lowerTitle, strings.ToLower(kw)) {
			return false
		}
	}

	return true
}

func (s *ShopifyMonitor) isExcluded(text string) bool {
	for _, kw := range s.task.ExcludeKeywords {
		if strings.Contains(strings.ToLower(text), strings.ToLower(kw)) {
			return true
		}
	}
	return false
}
