package dto

// StockResp is returned by GET /stock/:id and GET /stock/:adapter/:id.
type StockResp struct {
	Stock   int    `json:"stock"`
	Adapter string `json:"adapter"`
}
