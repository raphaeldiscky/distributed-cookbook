// Package routes is the single place to look for catalog-service's HTTP surface.
package routes

import (
	"github.com/labstack/echo/v4"

	"github.com/raphaeldiscky/distributed-cookbook/services/catalog-service/internal/handler"
)

// Register attaches every endpoint to e.
//
// /healthz and /metrics are wired by pkg/httpserver.New, so this file
// only registers domain endpoints.
func Register(e *echo.Echo, products *handler.Products) {
	e.GET("/products", products.List)
	e.GET("/products/:id", products.Get)
}
