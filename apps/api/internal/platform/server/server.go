package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func Handler() http.Handler {
	router := chi.NewRouter()
	router.Use(securityHeaders)
	router.Route("/api/v1", func(router chi.Router) {
		router.Get("/health/live", health("ok"))
		router.Get("/health/ready", health("ready"))
	})
	return router
}

func health(status string) http.HandlerFunc {
	return func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]string{"status": status})
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("X-Content-Type-Options", "nosniff")
		response.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(response, request)
	})
}
