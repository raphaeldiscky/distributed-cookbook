// Package domain holds the flashsale recipe's domain types.
package domain

import "time"

// Product is a sellable item with finite stock.
type Product struct {
	ID    int64
	Name  string
	Stock int
}

// Order records that Qty units of Product were sold.
type Order struct {
	ID        int64
	ProductID int64
	Qty       int
	CreatedAt time.Time
}
