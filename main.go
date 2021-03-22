package main

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func main() {
	e := echo.New()
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Welcome to DataVaccinator Vault")
	})

	e.POST("/index.php", protocolHandler) // bind protocol handler

	e.Logger.Fatal(e.Start(":8080"))
}

func protocolHandler(c echo.Context) error {
	return c.String(http.StatusOK, "You reached index.php")
}
