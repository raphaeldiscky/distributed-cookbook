// Package handler implements catalog-service's HTTP handlers.
//
// In-memory store: the seeded product list lives on the handler struct.
// If a future recipe needs persistence, swap this for a repository
// interface with a Postgres adapter — no changes to routes or main.
package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/raphaeldiscky/distributed-cookbook/services/catalog-service/internal/domain"
	"github.com/raphaeldiscky/distributed-cookbook/services/catalog-service/internal/metrics"
)

// Products wires the in-memory product list and metrics into HTTP endpoints.
type Products struct {
	products []domain.Product
	byID     map[int]domain.Product
	metrics  *metrics.Metrics
}

// NewProducts constructs the handler with seeded data.
func NewProducts(seed []domain.Product, m *metrics.Metrics) *Products {
	idx := make(map[int]domain.Product, len(seed))
	for _, p := range seed {
		idx[p.ID] = p
	}

	return &Products{products: seed, byID: idx, metrics: m}
}

// List handles GET /products.
func (h *Products) List(c echo.Context) error {
	start := time.Now()

	err := c.JSON(http.StatusOK, h.products)
	h.observe("/products", http.StatusOK, start, err)

	return err
}

// Get handles GET /products/:id.
func (h *Products) Get(c echo.Context) error {
	start := time.Now()

	id, parseErr := strconv.Atoi(c.Param("id"))
	if parseErr != nil || id <= 0 {
		err := c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
		h.observe("/products/:id", http.StatusBadRequest, start, err)

		return err
	}

	p, ok := h.byID[id]
	if !ok {
		err := c.JSON(http.StatusNotFound, map[string]string{"error": "product_not_found"})
		h.observe("/products/:id", http.StatusNotFound, start, err)

		return err
	}

	err := c.JSON(http.StatusOK, p)
	h.observe("/products/:id", http.StatusOK, start, err)

	return err
}

func (h *Products) observe(route string, status int, start time.Time, jsonErr error) {
	if jsonErr != nil {
		status = http.StatusInternalServerError
	}

	h.metrics.Latency.WithLabelValues(route).Observe(time.Since(start).Seconds())
	h.metrics.Requests.WithLabelValues(route, strconv.Itoa(status)).Inc()
}
