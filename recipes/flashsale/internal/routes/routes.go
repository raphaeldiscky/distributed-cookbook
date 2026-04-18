// Package routes is the single place to look for the flashsale recipe's
// full HTTP surface. Each handler struct stays focused on its business
// logic; this file is the API contract in route form.
//
// Adding a handler = add it as a parameter here and register its endpoints.
// Moving URLs around = edit this file and this file only.
package routes

import (
	"github.com/labstack/echo/v4"

	"github.com/raphaeldiscky/distributed-cookbook/recipes/flashsale/internal/handler"
)

// Register attaches every flashsale endpoint to e.
//
// Path-routed variants of /checkout and /stock let one running server
// serve all three adapters; the bare /checkout and /stock/:id aliases
// fall back to the default adapter (RECIPE_FLASHSALE_ADAPTER).
func Register(e *echo.Echo, checkout *handler.Checkout) {
	e.POST("/checkout", checkout.Post)
	e.POST("/checkout/:adapter", checkout.Post)
	e.POST("/seed", checkout.Seed)
	e.GET("/stock/:id", checkout.Stock)
	e.GET("/stock/:adapter/:id", checkout.Stock)
}
