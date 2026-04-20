// Package entity holds the flashsale recipe's persistence-mapped types.
// Each entity corresponds one-to-one with a row in the `flashsale.*` schema.
//
// Kept separate from `internal/dto` (the HTTP wire shape) so the two can
// evolve independently: storage can gain columns without breaking the API,
// and the API can rename fields without altering the DB.
package entity

// Product is a sellable item with finite stock — `flashsale.products`.
type Product struct {
	ID    int64
	Name  string
	Stock int
}
