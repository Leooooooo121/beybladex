package models

import (
	"fmt"
	"time"
)

// ProductStatus 代表統一標準化的商品狀態事件
type ProductStatus struct {
	SiteName    string            `json:"site_name"`       // 網站來源名稱 (如 "MMToyShop", "Shopify-Kith", "Funbox")
	ProductID   string            `json:"product_id"`      // 商品唯一識別碼
	Title       string            `json:"title"`           // 商品名稱
	URL         string            `json:"url"`             // 商品連結
	ImageURL    string            `json:"image_url"`       // 商品縮圖網址
	VariantID   string            `json:"variant_id"`      // 規格/款式識別碼
	VariantName string            `json:"variant_name"`    // 規格名稱 (如 "9月預購", "US 10.5", "Default Title")
	Price       float64           `json:"price"`           // 售價
	Currency    string            `json:"currency"`        // 幣別 (如 "TWD", "USD")
	InStock     bool              `json:"in_stock"`        // 是否有庫存
	Quantity    int               `json:"quantity"`        // 庫存數量 (若 API 無提供具體數字則為 1)
	Timestamp   time.Time         `json:"timestamp"`       // 發現時間 (毫秒精度)
	Extra       map[string]string `json:"extra,omitempty"` // 額外詮釋資料
}

// UniqueKey 產生商品規格的唯一鍵，用於狀態去重
func (p ProductStatus) UniqueKey() string {
	if p.VariantID != "" {
		return fmt.Sprintf("%s:%s:%s", p.SiteName, p.ProductID, p.VariantID)
	}
	if p.VariantName != "" {
		return fmt.Sprintf("%s:%s:%s", p.SiteName, p.ProductID, p.VariantName)
	}
	return fmt.Sprintf("%s:%s", p.SiteName, p.ProductID)
}

// ==========================================
// Shopify API 資料模型
// ==========================================

// ShopifyProductsResponse 代表 Shopify /products.json 回應
type ShopifyProductsResponse struct {
	Products []ShopifyProduct `json:"products"`
}

// ShopifySingleProductResponse 代表 Shopify /products/<handle>.json 回應
type ShopifySingleProductResponse struct {
	Product ShopifyProduct `json:"product"`
}

// ShopifyProduct 代表 Shopify 商品資料
type ShopifyProduct struct {
	ID          int64            `json:"id"`
	Title       string           `json:"title"`
	Handle      string           `json:"handle"`
	Vendor      string           `json:"vendor"`
	ProductType string           `json:"product_type"`
	Variants    []ShopifyVariant `json:"variants"`
	Images      []ShopifyImage   `json:"images"`
}

// ShopifyVariant 代表 Shopify 商品規格
type ShopifyVariant struct {
	ID                int64  `json:"id"`
	Title             string `json:"title"`
	Price             string `json:"price"`
	SKU               string `json:"sku"`
	Available         bool   `json:"available"`
	InventoryQuantity int    `json:"inventory_quantity"`
}

// ShopifyImage 代表 Shopify 商品圖片
type ShopifyImage struct {
	ID  int64  `json:"id"`
	SRC string `json:"src"`
}

// ==========================================
// BVShop / MMToyshop API 資料模型
// ==========================================

// BVShopResponse 代表 BVShop / MMToyShop 搜尋與分類回應
type BVShopResponse struct {
	ListName    string          `json:"listName"`
	Products    []BVShopProduct `json:"products"`
	CurrentPage int             `json:"currentPage"`
	Total       int             `json:"total"`
	LastPage    int             `json:"lastPage"`
}

// BVShopProduct 代表 BVShop 單項商品資料
type BVShopProduct struct {
	ID           int          `json:"id"`
	Title        string       `json:"title"`
	Photo        string       `json:"photo"`
	Quantity     int          `json:"quantity"`
	Price        float64      `json:"price"`
	SpecialPrice *float64     `json:"special_price"`
	Route        string       `json:"route"`
	Specs        []BVShopSpec `json:"specs"`
}

// BVShopSpec 代表 BVShop 商品規格
type BVShopSpec struct {
	ID           int      `json:"id"`
	SizeName     string   `json:"size_name"`
	Quantity     int      `json:"quantity"`
	Price        float64  `json:"price"`
	SpecialPrice *float64 `json:"special_price"`
}

// ==========================================
// Funbox (麗嬰國際) API 資料模型
// ==========================================

// FunboxProduct 代表 Funbox 商品資料
type FunboxProduct struct {
	ID       int64           `json:"id"`
	URL      string          `json:"url"`      // 例如 "/products/tm09709"
	Title    string          `json:"title"`    // 例如 "TOMICA Premium 賽車..."
	Price    float64         `json:"price"`    // 價格
	Photo    string          `json:"photo"`    // 圖片網址
	Variants []FunboxVariant `json:"variants"` // 規格庫存列表
}

// FunboxVariant 代表 Funbox 商品規格與庫存
type FunboxVariant struct {
	ID                int64   `json:"id,omitempty"`
	Title             string  `json:"title,omitempty"`
	Name              string  `json:"name,omitempty"`
	Price             float64 `json:"price,omitempty"`
	InventoryQuantity int     `json:"inventory_quantity"` // 庫存數量
}
