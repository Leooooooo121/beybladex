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

	"beyblade/config"
	"beyblade/models"
	"beyblade/proxy"

	fhttp "github.com/bogdanfinn/fhttp"
)

func init() {
	RegisterMonitor("funbox", func() SiteMonitor {
		return &FunboxMonitor{}
	})
}

const funboxBaseURL = "https://shop.funbox.com.tw"

// FunboxMonitor 麗嬰國際 (Funbox) 商城專用庫存監控適配器
type FunboxMonitor struct {
	task     config.TaskConfig
	proxyMgr *proxy.ProxyManager
	timeout  time.Duration
}

func (f *FunboxMonitor) TaskID() string {
	return f.task.ID
}

func (f *FunboxMonitor) Name() string {
	return f.task.Name
}

func (f *FunboxMonitor) Init(task config.TaskConfig, proxyMgr *proxy.ProxyManager, timeout time.Duration) error {
	f.task = task
	f.proxyMgr = proxyMgr
	f.timeout = timeout
	return nil
}

// CheckStock 執行 Funbox 多分頁商品庫存掃描
func (f *FunboxMonitor) CheckStock(ctx context.Context) ([]models.ProductStatus, error) {
	var allResults []models.ProductStatus
	page := 1
	maxPages := 10 // 預設最多爬取 10 頁避免無窮迴圈

	if limitStr, exists := f.task.CustomParams["max_pages"]; exists {
		if limit, parseErr := strconv.Atoi(limitStr); parseErr == nil && limit > 0 {
			maxPages = limit
		}
	}

	for {
		if page > maxPages {
			break
		}

		pageURL, err := f.buildPageURL(f.task.URL, page)
		if err != nil {
			return nil, fmt.Errorf("建置 Funbox 分頁 URL 失敗: %w", err)
		}

		// 若非第 1 頁，加入防封控隨機抖動延遲
		if page > 1 {
			delay := f.calculatePageJitter()
			select {
			case <-ctx.Done():
				return allResults, ctx.Err()
			case <-time.After(delay):
			}
		}

		products, err := f.fetchAndParsePage(ctx, pageURL)
		if err != nil {
			if page == 1 {
				return nil, err
			}
			log.Printf("[WARN] [Funbox:%s] 抓取第 %d 頁失敗: %v", f.Name(), page, err)
			break
		}

		// 若當前頁面無任何商品，表示已至最後一頁
		if len(products) == 0 {
			break
		}

		pageItems := f.convertFunboxProducts(products)
		allResults = append(allResults, pageItems...)

		// 判斷是否可能還有下一頁 (預設單頁 limit 為 18)
		limit := 18
		if parsedLimit := f.extractLimitFromURL(f.task.URL); parsedLimit > 0 {
			limit = parsedLimit
		}

		if len(products) < limit {
			// 商品數小於單頁筆數，表示已為最後一頁
			break
		}

		page++
	}

	return allResults, nil
}

// fetchAndParsePage 發送單頁請求並反序列化 Funbox JSON 陣列
func (f *FunboxMonitor) fetchAndParsePage(ctx context.Context, targetURL string) ([]models.FunboxProduct, error) {
	currentProxy, err := f.proxyMgr.GetNext()
	if err != nil {
		return nil, fmt.Errorf("取得 Proxy 失敗: %w", err)
	}

	client, err := BuildTLSClient(currentProxy, f.timeout)
	if err != nil {
		return nil, err
	}

	req, err := fhttp.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("建立 Request 失敗: %w", err)
	}

	f.applyHeaders(req)

	resp, err := client.Do(req)
	if err != nil {
		f.proxyMgr.ReportFailure(currentProxy, 0)
		return nil, fmt.Errorf("Funbox 請求發送失敗 (%s): %w", targetURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 || resp.StatusCode == 429 {
		f.proxyMgr.ReportFailure(currentProxy, resp.StatusCode)
		return nil, fmt.Errorf("遇到反爬防護/頻率限制 (HTTP %d)", resp.StatusCode)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Funbox API 回應異常狀態碼: %d", resp.StatusCode)
	}

	f.proxyMgr.ReportSuccess(currentProxy)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("讀取 Response 內容失敗: %w", err)
	}

	var products []models.FunboxProduct
	if err := json.Unmarshal(body, &products); err != nil {
		return nil, fmt.Errorf("Funbox JSON 解析失敗: %w", err)
	}

	return products, nil
}

// convertFunboxProducts 將 Funbox 資料結構轉換為標準 ProductStatus
func (f *FunboxMonitor) convertFunboxProducts(products []models.FunboxProduct) []models.ProductStatus {
	var results []models.ProductStatus
	now := time.Now()

	for _, p := range products {
		if !f.matchesFilter(p.Title) {
			continue
		}

		fullURL := f.formatProductURL(p.URL)

		// 若無 variants 欄位或為空，建立單一商品狀態
		if len(p.Variants) == 0 {
			results = append(results, models.ProductStatus{
				SiteName:    f.task.Name,
				ProductID:   strconv.FormatInt(p.ID, 10),
				Title:       p.Title,
				URL:         fullURL,
				ImageURL:    p.Photo,
				VariantID:   "",
				VariantName: "標準規格",
				Price:       p.Price,
				Currency:    "TWD",
				InStock:     true,
				Quantity:    1,
				Timestamp:   now,
			})
			continue
		}

		// 遍歷所有 variants 規格
		for _, v := range p.Variants {
			varName := v.Title
			if varName == "" {
				varName = v.Name
			}
			if varName == "" {
				varName = "標準規格"
			}

			if f.isExcluded(varName) {
				continue
			}

			price := p.Price
			if v.Price > 0 {
				price = v.Price
			}

			varID := ""
			if v.ID != 0 {
				varID = strconv.FormatInt(v.ID, 10)
			}

			inStock := v.InventoryQuantity > 0

			results = append(results, models.ProductStatus{
				SiteName:    f.task.Name,
				ProductID:   strconv.FormatInt(p.ID, 10),
				Title:       p.Title,
				URL:         fullURL,
				ImageURL:    p.Photo,
				VariantID:   varID,
				VariantName: varName,
				Price:       price,
				Currency:    "TWD",
				InStock:     inStock,
				Quantity:    v.InventoryQuantity,
				Timestamp:   now,
			})
		}
	}

	return results
}

// formatProductURL 確保產生的直達網址格式為 https://shop.funbox.com.tw/products/...
func (f *FunboxMonitor) formatProductURL(rawURL string) string {
	if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		return rawURL
	}
	if strings.HasPrefix(rawURL, "/") {
		return funboxBaseURL + rawURL
	}
	return funboxBaseURL + "/" + rawURL
}

func (f *FunboxMonitor) buildPageURL(rawURL string, page int) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("page", strconv.Itoa(page))
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (f *FunboxMonitor) extractLimitFromURL(rawURL string) int {
	u, err := url.Parse(rawURL)
	if err != nil {
		return 0
	}
	limitStr := u.Query().Get("limit")
	if limitStr == "" {
		return 0
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		return 0
	}
	return limit
}

func (f *FunboxMonitor) calculatePageJitter() time.Duration {
	minMs := 600
	maxMs := 1200

	if valStr, ok := f.task.CustomParams["page_delay_min_ms"]; ok {
		if v, err := strconv.Atoi(valStr); err == nil && v > 0 {
			minMs = v
		}
	}
	if valStr, ok := f.task.CustomParams["page_delay_max_ms"]; ok {
		if v, err := strconv.Atoi(valStr); err == nil && v >= minMs {
			maxMs = v
		}
	}

	jitterMs := minMs + rand.Intn(maxMs-minMs+1)
	microJitter := rand.Intn(31)
	return time.Duration(jitterMs+microJitter) * time.Millisecond
}

func (f *FunboxMonitor) applyHeaders(req *fhttp.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-TW,zh;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Sec-Ch-Ua", `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")

	for k, v := range f.task.Headers {
		req.Header.Set(k, v)
	}

	if f.task.Cookies != "" {
		req.Header.Set("Cookie", f.task.Cookies)
	}
}

func (f *FunboxMonitor) matchesFilter(title string) bool {
	if len(f.task.FilterKeywords) == 0 && f.task.Keyword == "" {
		return true
	}

	lowerTitle := strings.ToLower(title)
	if f.task.Keyword != "" && !strings.Contains(lowerTitle, strings.ToLower(f.task.Keyword)) {
		return false
	}

	for _, kw := range f.task.FilterKeywords {
		if !strings.Contains(lowerTitle, strings.ToLower(kw)) {
			return false
		}
	}

	return true
}

func (f *FunboxMonitor) isExcluded(text string) bool {
	for _, kw := range f.task.ExcludeKeywords {
		if strings.Contains(strings.ToLower(text), strings.ToLower(kw)) {
			return true
		}
	}
	return false
}
