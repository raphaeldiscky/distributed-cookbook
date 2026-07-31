package domain

import "errors"

var (
	// ErrOutOfStock is returned when a decrement request exceeds available stock.
	ErrOutOfStock = errors.New("out of stock")
	// ErrProductNotFound is returned when the product ID does not exist.
	ErrProductNotFound = errors.New("product not found")
	// ErrRetryExhausted is returned when an optimistic-locking adapter lost the
	// compare-and-set race on every attempt. Stock may well remain: the buyer
	// was crowded out rather than sold out, which is why this is distinct from
	// ErrOutOfStock.
	ErrRetryExhausted = errors.New("retry exhausted")
	// ErrUnavailable is returned when a dependency the adapter needs is not
	// reachable, such as a broker that will not accept a write. Nothing is wrong
	// with the request, so it maps to 503 rather than to a 4xx.
	ErrUnavailable = errors.New("dependency unavailable")
)
