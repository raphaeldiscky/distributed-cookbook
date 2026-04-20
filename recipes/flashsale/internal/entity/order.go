package entity

import "time"

// Order records that Qty units of Product were sold — `flashsale.orders`.
type Order struct {
	ID        int64
	ProductID int64
	Qty       int
	CreatedAt time.Time
}
