// Package domain holds the catalog-service's data types.
//
// In-memory by design — recipes that need persistence can layer a
// Postgres adapter behind a repository interface later.
package domain

// Product is the canonical product record.
type Product struct {
	ID    int     `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
	Stock int     `json:"stock"`
}

// SeedProducts returns a deterministic, ordered list of products used
// to initialize the in-memory store at startup. Stable IDs let recipe
// load tests fetch /products/:id without a setup phase.
func SeedProducts() []Product {
	return []Product{
		{ID: 1, Name: "Mechanical Keyboard", Price: 129.99, Stock: 42},
		{ID: 2, Name: "Wireless Mouse", Price: 49.99, Stock: 120},
		{ID: 3, Name: "27\" Monitor", Price: 349.00, Stock: 18},
		{ID: 4, Name: "USB-C Hub", Price: 39.50, Stock: 200},
		{ID: 5, Name: "Standing Desk", Price: 599.00, Stock: 5},
		{ID: 6, Name: "Ergonomic Chair", Price: 449.00, Stock: 12},
		{ID: 7, Name: "Webcam 4K", Price: 89.00, Stock: 64},
		{ID: 8, Name: "Bluetooth Headphones", Price: 199.99, Stock: 33},
		{ID: 9, Name: "External SSD 1TB", Price: 119.00, Stock: 80},
		{ID: 10, Name: "Laptop Stand", Price: 29.99, Stock: 150},
	}
}
