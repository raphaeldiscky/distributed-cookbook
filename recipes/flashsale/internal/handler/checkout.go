// Package handler implements the HTTP handlers for the flashsale recipe.
package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/raphaeldiscky/distributed-cookbook/recipes/flashsale/internal/domain"
	"github.com/raphaeldiscky/distributed-cookbook/recipes/flashsale/internal/metrics"
	"github.com/raphaeldiscky/distributed-cookbook/recipes/flashsale/internal/stock"
)

// Checkout wires the decrement adapter, metrics, and logger into HTTP handlers.
type Checkout struct {
	dec     stock.Decrementer
	metrics *Metrics
	log     *slog.Logger

	// Oversell detection: /seed records the initial stock per product; every
	// successful /checkout increments orderCount. If orderCount exceeds
	// initialStock, that checkout is an oversell. This captures lost-update
	// races that the naive adapter produces, which don't necessarily drive
	// stock_remaining below zero (the DB value is whatever the last writer
	// happened to compute).
	mu           sync.Mutex
	initialStock map[int64]int
	orderCount   map[int64]int
}

// Metrics is a local alias so callers don't import the metrics pkg for type hints.
type Metrics = metrics.Metrics

// NewCheckout constructs a Checkout handler bound to one stock adapter.
func NewCheckout(dec stock.Decrementer, m *metrics.Metrics, log *slog.Logger) *Checkout {
	return &Checkout{
		dec:          dec,
		metrics:      m,
		log:          log,
		initialStock: make(map[int64]int),
		orderCount:   make(map[int64]int),
	}
}

// Register attaches the checkout endpoints to e.
func (h *Checkout) Register(e *echo.Echo) {
	e.POST("/checkout", h.Post)
	e.POST("/seed", h.Seed)
	e.GET("/stock/:id", h.Stock)
}

type checkoutReq struct {
	ProductID int64 `json:"product_id"`
	Qty       int   `json:"qty"`
}

type checkoutResp struct {
	OrderID        int64 `json:"order_id"`
	StockRemaining int   `json:"stock_remaining"`
}

type errResp struct {
	Error string `json:"error"`
}

// Post handles POST /checkout — attempts to decrement stock and record an order.
func (h *Checkout) Post(c echo.Context) error {
	var req checkoutReq
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errResp{Error: "invalid json"})
	}

	if req.ProductID <= 0 || req.Qty <= 0 {
		return c.JSON(http.StatusBadRequest, errResp{Error: "product_id and qty must be positive"})
	}

	adapter := string(h.dec.Name())
	start := time.Now()

	res, err := h.dec.Decrement(c.Request().Context(), req.ProductID, req.Qty)

	h.metrics.CheckoutLatency.WithLabelValues(adapter).Observe(time.Since(start).Seconds())

	switch {
	case err == nil:
		h.metrics.CheckoutAttempts.WithLabelValues(adapter, "ok").Inc()
		h.metrics.StockRemaining.WithLabelValues(strconv.FormatInt(req.ProductID, 10)).
			Set(float64(res.StockRemaining))

		// Oversell check: successful checkouts beyond the seeded stock are oversells.
		h.mu.Lock()

		h.orderCount[req.ProductID] += req.Qty
		if init, ok := h.initialStock[req.ProductID]; ok && h.orderCount[req.ProductID] > init {
			h.metrics.OversellTotal.Inc()
		}

		h.mu.Unlock()

		return c.JSON(http.StatusOK, checkoutResp{
			OrderID:        res.OrderID,
			StockRemaining: res.StockRemaining,
		})
	case errors.Is(err, domain.ErrOutOfStock):
		h.metrics.CheckoutAttempts.WithLabelValues(adapter, "out_of_stock").Inc()

		return c.JSON(http.StatusConflict, errResp{Error: "out_of_stock"})
	case errors.Is(err, domain.ErrProductNotFound):
		h.metrics.CheckoutAttempts.WithLabelValues(adapter, "not_found").Inc()

		return c.JSON(http.StatusNotFound, errResp{Error: "product_not_found"})
	default:
		h.metrics.CheckoutAttempts.WithLabelValues(adapter, "error").Inc()
		h.log.ErrorContext(
			c.Request().Context(),
			"checkout failed",
			slog.String("err", err.Error()),
		)

		return c.JSON(http.StatusInternalServerError, errResp{Error: "internal"})
	}
}

type seedReq struct {
	ProductID int64  `json:"product_id"`
	Name      string `json:"name"`
	Stock     int    `json:"stock"`
}

// Seed handles POST /seed — resets stock for a product and clears prior orders.
func (h *Checkout) Seed(c echo.Context) error {
	var req seedReq
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errResp{Error: "invalid json"})
	}

	if req.ProductID <= 0 || req.Stock < 0 {
		return c.JSON(http.StatusBadRequest, errResp{Error: "product_id > 0, stock >= 0"})
	}

	if req.Name == "" {
		req.Name = "flashsale-item-" + strconv.FormatInt(req.ProductID, 10)
	}

	if err := h.dec.Seed(c.Request().Context(), req.ProductID, req.Name, req.Stock); err != nil {
		h.log.ErrorContext(c.Request().Context(), "seed failed", slog.String("err", err.Error()))

		return c.JSON(http.StatusInternalServerError, errResp{Error: "internal"})
	}

	h.mu.Lock()
	h.initialStock[req.ProductID] = req.Stock
	h.orderCount[req.ProductID] = 0
	h.mu.Unlock()

	h.metrics.StockRemaining.WithLabelValues(strconv.FormatInt(req.ProductID, 10)).
		Set(float64(req.Stock))

	return c.JSON(http.StatusOK, map[string]any{"product_id": req.ProductID, "stock": req.Stock})
}

// Stock handles GET /stock/:id — returns current stock from the adapter's source of truth.
func (h *Checkout) Stock(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, errResp{Error: "invalid id"})
	}

	s, err := h.dec.Stock(c.Request().Context(), id)
	switch {
	case err == nil:
		return c.JSON(http.StatusOK, map[string]any{"stock": s})
	case errors.Is(err, domain.ErrProductNotFound):
		return c.JSON(http.StatusNotFound, errResp{Error: "product_not_found"})
	default:
		return c.JSON(http.StatusInternalServerError, errResp{Error: "internal"})
	}
}
