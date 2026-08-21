package main

// Response 代表最外層的 JSON 結構
type Response struct {
	ListName    string    `json:"listName"`
	Products    []Product `json:"products"`
	CurrentPage int       `json:"currentPage"`
	Total       int       `json:"total"`
	LastPage    int       `json:"lastPage"`
}

// Product 代表商品細節
type Product struct {
	ID    string      `json:"id"`
	Title string      `json:"title"`
	Route string      `json:"route"`
	Specs []SpecsInfo `json:"specs"`
}

type SpecsInfo struct {
	SizeName string  `json:"size_name"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
}
