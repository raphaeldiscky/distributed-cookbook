package domain

import "errors"

var (
	// ErrOutOfStock is returned when a decrement request exceeds available stock.
	ErrOutOfStock = errors.New("out of stock")
	// ErrProductNotFound is returned when the product ID does not exist.
	ErrProductNotFound = errors.New("product not found")
)
