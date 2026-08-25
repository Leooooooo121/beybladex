package sites

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"beybladex/config"
	"beybladex/models"
	"beybladex/proxy"
)

func TestSiteMonitor_Factory(t *testing.T) {
	pm := proxy.NewProxyManager([]string{}, 1*time.Second)

	shopifyTask := config.TaskConfig{
		ID:       "shopify_test",
		SiteType: "shopify",
		Name:     "Shopify Test",
		URL:      "https://example.com/products.json",
	}

	mon1, err := CreateMonitor(shopifyTask, pm, 5*time.Second)
	if err != nil {
		t.Fatalf("建立 Shopify 監控器失敗: %v", err)
	}
	if mon1.TaskID() != "shopify_test" || mon1.Name() != "Shopify Test" {
		t.Errorf("監控器資訊不符: %+v", mon1)
	}

	bvTask := config.TaskConfig{
		ID:       "bvshop_test",
		SiteType: "bvshop",
		Name:     "BVShop Test",
		URL:      "https://mmtoyshop.com/category/query",
	}

	mon2, err := CreateMonitor(bvTask, pm, 5*time.Second)
	if err != nil {
		t.Fatalf("建立 BVShop 監控器失敗: %v", err)
	}
	if mon2.TaskID() != "bvshop_test" {
		t.Errorf("監控器資訊不符: %+v", mon2)
	}

	funboxTask := config.TaskConfig{
		ID:       "funbox_test",
		SiteType: "funbox",
		Name:     "Funbox Test",
		URL:      "https://shop.funbox.com.tw/category_products/takaratomy/beyblade.json",
	}

	mon3, err := CreateMonitor(funboxTask, pm, 5*time.Second)
	if err != nil {
		t.Fatalf("建立 Funbox 監控器失敗: %v", err)
	}
	if mon3.TaskID() != "funbox_test" {
		t.Errorf("Funbox 監控器資訊不符: %+v", mon3)
	}
}

func TestBVShop_ParseAndFilter(t *testing.T) {
	jsonData := `{
		"listName": "戰鬥陀螺",
		"products": [
			{
				"id": 101,
				"title": "戰鬥陀螺 UX-01",
				"route": "ux01",
				"specs": [
					{
						"id": 1001,
						"size_name": "現貨",
						"quantity": 3,
						"price": 300
					},
					{
						"id": 1002,
						"size_name": "限客訂_user1",
						"quantity": 5,
						"price": 300
					}
				]
			},
			{
				"id": 102,
				"title": "其他無關玩具",
				"route": "other",
				"specs": [
					{
						"id": 1003,
						"size_name": "現貨",
						"quantity": 10,
						"price": 100
					}
				]
			}
		]
	}`

	var resp models.BVShopResponse
	if err := json.Unmarshal([]byte(jsonData), &resp); err != nil {
		t.Fatalf("反序列化失敗: %v", err)
	}

	bvMon := &BVShopMonitor{
		task: config.TaskConfig{
			Name:            "TestShop",
			FilterKeywords:  []string{"陀螺"},
			ExcludeKeywords: []string{"限客訂"},
		},
	}

	items := bvMon.convertBVShopProducts(resp.Products)

	// 預期只會匹配到 UX-01 的 "現貨" 規格 (排除 "限客訂" 且過濾掉 "其他無關玩具")
	if len(items) != 1 {
		t.Fatalf("預期過濾後剩下 1 項，得到 %d", len(items))
	}

	if items[0].ProductID != "101" || items[0].VariantName != "現貨" || !items[0].InStock {
		t.Errorf("過濾結果不符: %+v", items[0])
	}
	_ = context.Background()
}

func TestBVShop_BuildPageURL(t *testing.T) {
	bvMon := &BVShopMonitor{}

	// 測試原本已有 query 參數的情況
	u1, err := bvMon.buildPageURL("https://mmtoyshop.com/category/query?keyword=%E6%88%B0%E9%AC%A5%E9%99%80%E8%9E%BA", 2)
	if err != nil {
		t.Fatalf("buildPageURL 失敗: %v", err)
	}
	if u1 != "https://mmtoyshop.com/category/query?keyword=%E6%88%B0%E9%AC%A5%E9%99%80%E8%9E%BA&page=2" {
		t.Errorf("產生的分頁 URL 不正確: %s", u1)
	}

	// 測試原本已有 page 參數被替換的情況
	u2, err := bvMon.buildPageURL("https://mmtoyshop.com/category/query?keyword=test&page=1", 3)
	if err != nil {
		t.Fatalf("buildPageURL 失敗: %v", err)
	}
	if u2 != "https://mmtoyshop.com/category/query?keyword=test&page=3" {
		t.Errorf("替換 page 參數後的 URL 不正確: %s", u2)
	}
}

func TestFunbox_ConvertAndFilter(t *testing.T) {
	jsonData := `[
		{
			"id": 71427691,
			"url": "/products/tm09709",
			"title": "TAKARA TOMY 戰鬥陀螺 UX-01 蒼穹巨龍",
			"price": 450,
			"variants": [
				{
					"id": 111,
					"title": "現貨規格",
					"inventory_quantity": 10
				}
			]
		},
		{
			"id": 70137108,
			"url": "/products/tm052a7x2",
			"title": "TAKARA TOMY 戰鬥陀螺 缺貨款式",
			"price": 300,
			"variants": [
				{
					"id": 222,
					"title": "已售完",
					"inventory_quantity": 0
				}
			]
		}
	]`

	var products []models.FunboxProduct
	if err := json.Unmarshal([]byte(jsonData), &products); err != nil {
		t.Fatalf("反序列化失敗: %v", err)
	}

	funMon := &FunboxMonitor{
		task: config.TaskConfig{
			Name: "Funbox Test",
		},
	}

	items := funMon.convertFunboxProducts(products)
	if len(items) != 2 {
		t.Fatalf("預期轉換出 2 個規格狀態，得到 %d", len(items))
	}

	// 第一項：有庫存
	if items[0].ProductID != "71427691" || !items[0].InStock || items[0].Quantity != 10 {
		t.Errorf("第一項商品庫存判斷異常: %+v", items[0])
	}
	if items[0].URL != "https://shop.funbox.com.tw/products/tm09709" {
		t.Errorf("第一項商品 URL 構造異常: %s", items[0].URL)
	}

	// 第二項：無庫存
	if items[1].ProductID != "70137108" || items[1].InStock || items[1].Quantity != 0 {
		t.Errorf("第二項商品無庫存判斷異常: %+v", items[1])
	}
}

func TestFunbox_BuildPageURL(t *testing.T) {
	funMon := &FunboxMonitor{}
	rawURL := "https://shop.funbox.com.tw/category_products/takaratomy/beyblade.json?limit=18&page=1&sort_by=sell_from-desc"

	page2URL, err := funMon.buildPageURL(rawURL, 2)
	if err != nil {
		t.Fatalf("buildPageURL 失敗: %v", err)
	}

	if page2URL != "https://shop.funbox.com.tw/category_products/takaratomy/beyblade.json?limit=18&page=2&sort_by=sell_from-desc" {
		t.Errorf("產生 Funbox page 2 網址不正確: %s", page2URL)
	}
}
