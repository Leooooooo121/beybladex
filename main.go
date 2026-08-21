package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"

	// 將 net/http 替換為 fhttp
	fhttp "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

func main() {
	// 1. 建立模擬 Chrome 的 Client
	options := []tls_client.HttpClientOption{
		tls_client.WithClientProfile(profiles.Chrome_120),
		tls_client.WithNotFollowRedirects(),
	}

	client, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
	if err != nil {
		log.Fatalf("建立 Client 失敗: %v", err)
	}

	// 2. 設定目標 API 網址
	targetURL := "https://mmtoyshop.com/category/query?keyword=%E6%88%B0%E9%AC%A5%E9%99%80%E8%9E%BA&&page=1"
	targetURL = "https://mmtoyshop.com/category/query?"
	// 3. 改用 fhttp.NewRequest 建立請求
	req, err := fhttp.NewRequest("GET", targetURL, nil)
	if err != nil {
		log.Fatalf("建立 Request 失敗: %v", err)
	}

	// 4. 設定 Header (順序與內容將由 tls-client 處理以符合瀏覽器特徵)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-TW,zh;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Referer", "https://mmtoyshop.com/category?keyword=%E6%88%B0%E9%AC%A5%E9%99%80%E8%9E%BA")

	// 5. 帶入關鍵 Cookie
	cookieString := "cf_clearance=你的_cf_clearance_值; mmtoyshopcom_session=你的_session_值; XSRF-TOKEN=你的_XSRF_TOKEN_值"
	req.Header.Set("Cookie", cookieString)

	// 6. 發送請求
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("請求發送失敗: %v", err)
	}
	defer resp.Body.Close()

	// 7. 讀取回應
	jsonData, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("讀取 Response 失敗: %v", err)
	}

	fmt.Printf("HTTP 狀態碼: %d\n", resp.StatusCode)
	fmt.Println("回應內容:")
	var res *Response
	err = json.Unmarshal(jsonData, &res)
	if err != nil {
		log.Fatalf("JSON 解析失敗: %v", err)
	}
	//res.Show()
	for _, r := range res.Products {
		for _, s := range r.Specs {
			fmt.Println("=======================")
			fmt.Println("商品名稱", r.Title)
			fmt.Println("網址路徑", fmt.Sprintf("https://mmtoyshop.com/item/%s", r.Route))
			if !strings.Contains(s.SizeName, "限客訂") {
				if s.Quantity > 0 {
					fmt.Println("名稱", s.SizeName)
					fmt.Println("剩餘數量", s.Quantity)
					fmt.Println("價格", s.Price)
					break
				}
			}
			fmt.Println("=======================")
		}
	}

}

func (res *Response) Show() {
	fmt.Println("篩選名稱", res.ListName)
	fmt.Println("當前頁數", res.CurrentPage)
	fmt.Println("總共數量", res.Total)
	fmt.Println("最後頁數", res.LastPage)
	for _, r := range res.Products {
		fmt.Println("=======================")
		fmt.Println("商品名稱", r.Title)
		fmt.Println("網址路徑", r.Route)
		for _, s := range r.Specs {
			if !strings.Contains(s.SizeName, "限客訂") {
				fmt.Println("名稱", s.SizeName)
				fmt.Println("剩餘數量", s.Quantity)
				break
			}
		}
		fmt.Println("=======================")
	}
}
