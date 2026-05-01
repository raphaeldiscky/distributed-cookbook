// Package routes is the single place to look for user-service's HTTP surface.
package routes

import (
	"github.com/labstack/echo/v4"

	"github.com/raphaeldiscky/distributed-cookbook/services/user-service/internal/handler"
)

// Register attaches every endpoint to e.
//
// /healthz and /metrics are wired by pkg/httpserver.New, so this file
// only registers domain endpoints.
func Register(e *echo.Echo, users *handler.Users) {
	e.GET("/users", users.List)
	e.GET("/users/:id", users.Get)
}
