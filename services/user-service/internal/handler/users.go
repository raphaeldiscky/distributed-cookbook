// Package handler implements user-service's HTTP handlers.
//
// In-memory store: the seeded user list lives on the handler struct.
// If a future recipe needs persistence, swap this for a repository
// interface with a Postgres adapter — no changes to routes or main.
package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/raphaeldiscky/distributed-cookbook/services/user-service/internal/domain"
	"github.com/raphaeldiscky/distributed-cookbook/services/user-service/internal/metrics"
)

// Users wires the in-memory user list and metrics into HTTP endpoints.
type Users struct {
	users   []domain.User
	byID    map[int]domain.User
	metrics *metrics.Metrics
}

// NewUsers constructs the handler with seeded data.
func NewUsers(seed []domain.User, m *metrics.Metrics) *Users {
	idx := make(map[int]domain.User, len(seed))
	for _, u := range seed {
		idx[u.ID] = u
	}

	return &Users{users: seed, byID: idx, metrics: m}
}

// List handles GET /users.
func (h *Users) List(c echo.Context) error {
	start := time.Now()

	err := c.JSON(http.StatusOK, h.users)
	h.observe("/users", http.StatusOK, start, err)

	return err
}

// Get handles GET /users/:id.
func (h *Users) Get(c echo.Context) error {
	start := time.Now()

	id, parseErr := strconv.Atoi(c.Param("id"))
	if parseErr != nil || id <= 0 {
		err := c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
		h.observe("/users/:id", http.StatusBadRequest, start, err)

		return err
	}

	u, ok := h.byID[id]
	if !ok {
		err := c.JSON(http.StatusNotFound, map[string]string{"error": "user_not_found"})
		h.observe("/users/:id", http.StatusNotFound, start, err)

		return err
	}

	err := c.JSON(http.StatusOK, u)
	h.observe("/users/:id", http.StatusOK, start, err)

	return err
}

func (h *Users) observe(route string, status int, start time.Time, jsonErr error) {
	if jsonErr != nil {
		status = http.StatusInternalServerError
	}

	h.metrics.Latency.WithLabelValues(route).Observe(time.Since(start).Seconds())
	h.metrics.Requests.WithLabelValues(route, strconv.Itoa(status)).Inc()
}
