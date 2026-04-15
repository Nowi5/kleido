package main

import (
	"github.com/go-chi/chi/v5"
	_ "kleido/docs"
	httpSwagger "github.com/swaggo/http-swagger"
)

// registerSwaggerRoutes mounts the Swagger UI at /swagger/*.
// Only called when APP_ENV != "production".
func registerSwaggerRoutes(r chi.Router) {
	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
		httpSwagger.DeepLinking(true),
	))
}
