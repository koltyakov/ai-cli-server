package middleware

import (
	"net/http"
)

// CORS is a middleware that adds CORS headers
type CORS struct {
	allowedOrigins []string
}

// NewCORS creates a new CORS middleware
func NewCORS(allowedOrigins []string) *CORS {
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"*"}
	}
	return &CORS{allowedOrigins: allowedOrigins}
}

// Handle wraps an HTTP handler with CORS support
func (c *CORS) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
