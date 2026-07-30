// Package handler implements the HTTP handlers for the flashsale recipe.
//
// Handlers are deliberately thin: bind DTO → call service → render DTO.
// Orchestration, metrics, and oversell tracking live in `internal/service`;
// atomic persistence lives in `internal/repository`.
package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/raphaeldiscky/distributed-cookbook/recipes/flashsale/internal/domain"
	"github.com/raphaeldiscky/distributed-cookbook/recipes/flashsale/internal/dto"
	"github.com/raphaeldiscky/distributed-cookbook/recipes/flashsale/internal/repository"
	"github.com/raphaeldiscky/distributed-cookbook/recipes/flashsale/internal/service"
)

// Error strings returned in dto.ErrResp, shared by more than one handler.
const (
	errInvalidJSON     = "invalid json"
	errProductNotFound = "product_not_found"
)

// Checkout wires the application service and logger into HTTP endpoints.
type Checkout struct {
	svc *service.Checkout
	log *slog.Logger
}

// NewCheckout constructs a Checkout handler bound to the given service.
func NewCheckout(svc *service.Checkout, log *slog.Logger) *Checkout {
	return &Checkout{svc: svc, log: log}
}

// Post handles POST /checkout and POST /checkout/:adapter — decrements stock
// and records an order, routed through the chosen Inventory implementation.
func (h *Checkout) Post(c echo.Context) error {
	var req dto.CheckoutReq
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, dto.ErrResp{Error: errInvalidJSON})
	}

	if req.ProductID <= 0 || req.Qty <= 0 {
		return c.JSON(
			http.StatusBadRequest,
			dto.ErrResp{Error: "product_id and qty must be positive"},
		)
	}

	kind := repository.Kind(c.Param("adapter"))

	res, err := h.svc.Checkout(c.Request().Context(), kind, req.ProductID, req.Qty)
	switch {
	case err == nil:
		return c.JSON(http.StatusOK, dto.CheckoutResp{
			OrderID:        res.OrderID,
			StockRemaining: res.StockRemaining,
			Adapter:        string(res.Kind),
		})
	case errors.Is(err, domain.ErrOutOfStock):
		return c.JSON(http.StatusConflict, dto.ErrResp{Error: "out_of_stock"})
	case errors.Is(err, domain.ErrProductNotFound):
		return c.JSON(http.StatusNotFound, dto.ErrResp{Error: errProductNotFound})
	default:
		return c.JSON(http.StatusBadRequest, dto.ErrResp{Error: err.Error()})
	}
}

// Seed handles POST /seed — resets stock for a product across every
// Inventory implementation so any `/checkout/:adapter` path works
// afterwards.
func (h *Checkout) Seed(c echo.Context) error {
	var req dto.SeedReq
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, dto.ErrResp{Error: errInvalidJSON})
	}

	if req.ProductID <= 0 || req.Stock < 0 {
		return c.JSON(http.StatusBadRequest, dto.ErrResp{Error: "product_id > 0, stock >= 0"})
	}

	if req.Name == "" {
		req.Name = "flashsale-item-" + strconv.FormatInt(req.ProductID, 10)
	}

	seeded, err := h.svc.Seed(c.Request().Context(), req.ProductID, req.Name, req.Stock)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, dto.ErrResp{Error: "internal"})
	}

	return c.JSON(http.StatusOK, dto.SeedResp{
		ProductID: req.ProductID,
		Stock:     req.Stock,
		Seeded:    seeded,
	})
}

// Stock handles GET /stock/:id and GET /stock/:adapter/:id — returns current
// stock from the chosen Inventory's source of truth.
func (h *Checkout) Stock(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, dto.ErrResp{Error: "invalid id"})
	}

	kind := repository.Kind(c.Param("adapter"))

	v, resolvedKind, err := h.svc.Stock(c.Request().Context(), kind, id)
	switch {
	case err == nil:
		return c.JSON(http.StatusOK, dto.StockResp{
			Stock:   v,
			Adapter: string(resolvedKind),
		})
	case errors.Is(err, domain.ErrProductNotFound):
		return c.JSON(http.StatusNotFound, dto.ErrResp{Error: errProductNotFound})
	default:
		return c.JSON(http.StatusBadRequest, dto.ErrResp{Error: err.Error()})
	}
}
