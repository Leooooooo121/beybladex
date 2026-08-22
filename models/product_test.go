package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestProductStatus_UniqueKey(t *testing.T) {
	p1 := ProductStatus{
		SiteName:  "MMToyShop",
		ProductID: "716314",
		VariantID: "2480930",
	}
	if p1.UniqueKey() != "MMToyShop:716314:2480930" {
		t.Errorf("UniqueKey 產生不正確: %s", p1.UniqueKey())
	}

	p2 := ProductStatus{
		SiteName:    "Shopify",
		ProductID:   "12345",
		VariantName: "US 10",
	}
	if p2.UniqueKey() != "Shopify:12345:US 10" {
		t.Errorf("UniqueKey 產生不正確: %s", p2.UniqueKey())
	}
}

func TestBVShop_Unmarshal(t *testing.T) {
	jsonData := `{
		"listName": "戰鬥陀螺",
		"currentPage": 1,
		"total": 1,
		"lastPage": 1,
		"products": [
			{
				"id": 716314,
				"title": "戰鬥陀螺 UX-03",
				"route": "Shopee6a4620509d97a",
				"quantity": 1,
				"price": 295,
				"specs": [
					{
						"id": 2480930,
						"size_name": "現貨",
						"quantity": 5,
						"price": 295
					}
				]
			}
		]
	}`

	var resp BVShopResponse
	err := json.Unmarshal([]byte(jsonData), &resp)
	if err != nil {
		t.Fatalf("反序列化失敗: %v", err)
	}

	if len(resp.Products) != 1 {
		t.Fatalf("預期 1 個商品，得到 %d", len(resp.Products))
	}

	p := resp.Products[0]
	if p.Title != "戰鬥陀螺 UX-03" || len(p.Specs) != 1 || p.Specs[0].Quantity != 5 {
		t.Errorf("解析資料與預期不符: %+v", p)
	}
}

func TestShopify_Unmarshal(t *testing.T) {
	jsonData := `{
		"products": [
			{
				"id": 88888,
				"title": "Limited Beyblade",
				"handle": "limited-beyblade",
				"variants": [
					{
						"id": 99999,
						"title": "Standard",
						"price": "29.99",
						"available": true,
						"inventory_quantity": 10
					}
				]
			}
		]
	}`

	var resp ShopifyProductsResponse
	err := json.Unmarshal([]byte(jsonData), &resp)
	if err != nil {
		t.Fatalf("Shopify 反序列化失敗: %v", err)
	}

	if len(resp.Products) != 1 || resp.Products[0].Variants[0].Price != "29.99" {
		t.Errorf("Shopify 解析資料與預期不符: %+v", resp)
	}
	_ = time.Now()
}
