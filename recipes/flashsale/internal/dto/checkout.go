// Package dto holds the HTTP request/response shapes for the flashsale recipe.
// These types ARE the API contract — change them with care, they're visible
// to clients.
//
// Split across files by endpoint to stay under revive's max-public-structs
// limit: checkout.go, seed.go, stock.go, error.go.
package dto

// CheckoutReq is the JSON body for POST /checkout and POST /checkout/:adapter.
type CheckoutReq struct {
	ProductID int64 `json:"product_id"`
	Qty       int   `json:"qty"`
}

// CheckoutResp is returned on a successful checkout (HTTP 200).
type CheckoutResp struct {
	OrderID        int64  `json:"order_id"`
	StockRemaining int    `json:"stock_remaining"`
	Adapter        string `json:"adapter"`
}
