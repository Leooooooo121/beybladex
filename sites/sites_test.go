package sites

import (
	"context"
	"testing"
	"time"

	"beyblade/config"
	"beyblade/proxy"
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

	bvMon := &BVShopMonitor{
		task: config.TaskConfig{
			Name:            "TestShop",
			FilterKeywords:  []string{"陀螺"},
			ExcludeKeywords: []string{"限客訂"},
		},
	}

	items, err := bvMon.parseBVShop([]byte(jsonData))
	if err != nil {
		t.Fatalf("解析失敗: %v", err)
	}

	// 預期只會匹配到 UX-01 的 "現貨" 規格 (排除 "限客訂" 且過濾掉 "其他無關玩具")
	if len(items) != 1 {
		t.Fatalf("預期過濾後剩下 1 項，得到 %d", len(items))
	}

	if items[0].ProductID != "101" || items[0].VariantName != "現貨" || !items[0].InStock {
		t.Errorf("過濾結果不符: %+v", items[0])
	}
	_ = context.Background()
}
