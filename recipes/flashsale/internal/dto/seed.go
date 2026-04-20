package dto

// SeedReq is the JSON body for POST /seed. Name is optional; a sensible
// default is used when empty.
type SeedReq struct {
	ProductID int64  `json:"product_id"`
	Name      string `json:"name"`
	Stock     int    `json:"stock"`
}

// SeedResp summarizes the seed operation — Seeded reports how many adapters
// had their state primed.
type SeedResp struct {
	ProductID int64 `json:"product_id"`
	Stock     int   `json:"stock"`
	Seeded    int   `json:"seeded"`
}
