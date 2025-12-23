package middleware

import (
	"log"
	"net/http"
	"time"
)

// Logger is a middleware that logs HTTP requests
type Logger struct {
	logger *log.Logger
}

// NewLogger creates a new logging middleware
func NewLogger(logger *log.Logger) *Logger {
	return &Logger{logger: logger}
}

// Log wraps an HTTP handler with request logging
func (l *Logger) Log(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a custom response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Process request
		next.ServeHTTP(wrapped, r)

		// Log request details
		duration := time.Since(start)
		l.logger.Printf(
			"%s %s %d %s",
			r.Method,
			r.URL.Path,
			wrapped.statusCode,
			duration,
		)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
