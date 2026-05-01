// Package domain holds the user-service's data types.
//
// In-memory by design — recipes that need persistence can layer a
// Postgres adapter behind a repository interface later. For the
// universal demo (envoy-gateway, cilium-gateway, future mesh/tracing
// recipes) the in-memory store keeps the service zero-dependency and fast.
package domain

// User is the canonical user record.
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

// SeedUsers returns a deterministic, ordered list of users used to
// initialize the in-memory store at startup. Stable IDs let recipe
// load tests fetch /users/:id without a setup phase.
func SeedUsers() []User {
	return []User{
		{ID: 1, Name: "Ada Lovelace", Email: "ada@example.com", Role: "admin"},
		{ID: 2, Name: "Alan Turing", Email: "alan@example.com", Role: "admin"},
		{ID: 3, Name: "Grace Hopper", Email: "grace@example.com", Role: "operator"},
		{ID: 4, Name: "Edsger Dijkstra", Email: "edsger@example.com", Role: "operator"},
		{ID: 5, Name: "Donald Knuth", Email: "donald@example.com", Role: "user"},
		{ID: 6, Name: "Barbara Liskov", Email: "barbara@example.com", Role: "user"},
		{ID: 7, Name: "Tony Hoare", Email: "tony@example.com", Role: "user"},
		{ID: 8, Name: "Leslie Lamport", Email: "leslie@example.com", Role: "user"},
		{ID: 9, Name: "Margaret Hamilton", Email: "margaret@example.com", Role: "user"},
		{ID: 10, Name: "Joe Armstrong", Email: "joe@example.com", Role: "user"},
	}
}
