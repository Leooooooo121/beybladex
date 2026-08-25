package atc

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"strings"
	"time"

	"beybladex/sites"

	fhttp "github.com/bogdanfinn/fhttp"
)

// DefaultMMToyShopSession 預設寫死在程式碼中的 MMToyshop 登入 Session
const DefaultMMToyShopSession = "eyJpdiI6ImE4c1k5dDIreGRoL2lyOW1wUmFMeEE9PSIsInZhbHVlIjoiTFhMQStZOW1wNE41VUpsbXg0OW5mdWlUaGprb3ZZc2ZPaUErS3lvT2VYY08zWURLUzR0aGF6clI1ZTZGR1pXd3k5cFl3K0txdGMydDNpcWJFM2oza3IzS0hpRVlObkVwd3ZMUmwrdG5sd050VURVUE8wZExwSnJkeEludXNRb2ciLCJtYWMiOiI3NzFjMmNlZDg1ZTI1MTRjNWM4NDA1MmE2NjMxOGFkZmU0Y2U4OGUyZTNhZDRmNDMyM2JhODVjMmZkMzJkYzM1IiwidGFnIjoiIn0%3D"

const mmtoyshopATCURL = "https://mmtoyshop.com/cart/addon_gift_store"

// AddToCartMMToyshop 自動將指定商品加入 MMToyshop 購物車
func AddToCartMMToyshop(ctx context.Context, productID string, sessionCookie string, proxyURL string, timeout time.Duration) (string, error) {
	if sessionCookie == "" {
		sessionCookie = DefaultMMToyShopSession
	}

	if timeout <= 0 {
		timeout = 8 * time.Second
	}

	startTime := time.Now()
	log.Printf("[INFO] [ATC:MMToyShop] ⚡ 觸發搶購！正在將商品 ID [%s] 自動加入購物車...", productID)

	client, err := sites.BuildTLSClient(proxyURL, timeout)
	if err != nil {
		return "", fmt.Errorf("建立 ATC TLS Client 失敗: %w", err)
	}

	// 構建 x-www-form-urlencoded 表單資料: add_product=<product_id>
	form := url.Values{}
	form.Set("add_product", productID)
	formData := form.Encode()

	req, err := fhttp.NewRequestWithContext(ctx, "POST", mmtoyshopATCURL, strings.NewReader(formData))
	if err != nil {
		return "", fmt.Errorf("建立 ATC Request 失敗: %w", err)
	}

	// 擬真瀏覽器 Header 與帶入登入 Session Cookie
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("Accept-Language", "zh-TW,zh;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Origin", "https://mmtoyshop.com")
	req.Header.Set("Referer", "https://mmtoyshop.com/")
	req.Header.Set("Sec-Ch-Ua", `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")

	// 組合 Cookie
	cookieHeader := fmt.Sprintf("mmtoyshopcom_session=%s", sessionCookie)
	req.Header.Set("Cookie", cookieHeader)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("發送 ATC 請求失敗: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("讀取 ATC 回應失敗: %w", err)
	}

	bodyStr := string(bodyBytes)
	elapsed := time.Since(startTime)

	if resp.StatusCode == 200 {
		log.Printf("[SUCCESS] [ATC:MMToyShop] 🎉 商品 ID [%s] 自動加入購物車成功！(耗時: %dms | 回應: %s)",
			productID, elapsed.Milliseconds(), bodyStr)
		return bodyStr, nil
	}

	return "", fmt.Errorf("ATC 請求失敗 (HTTP %d): %s", resp.StatusCode, bodyStr)
}
