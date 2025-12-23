package api

import (
	"log"
	"net/http"

	"github.com/andrew/ai-cli-server/internal/agents/copilot"
	"github.com/andrew/ai-cli-server/internal/agents/cursor"
	"github.com/andrew/ai-cli-server/internal/api/handlers"
	"github.com/andrew/ai-cli-server/internal/api/middleware"
	"github.com/andrew/ai-cli-server/internal/database"
)

// SetupRoutes configures all API routes
func SetupRoutes(
	db *database.DB,
	copilotProvider *copilot.Provider,
	cursorProvider *cursor.Provider,
	logger *log.Logger,
) http.Handler {
	mux := http.NewServeMux()

	// Create handlers
	chatHandler := handlers.NewChatHandler(db, copilotProvider, cursorProvider)
	usageHandler := handlers.NewUsageHandler(db)

	// Create middleware
	authMiddleware := middleware.NewAuthMiddleware(db)
	rateLimitMiddleware := middleware.NewRateLimitMiddleware(db)
	loggerMiddleware := middleware.NewLogger(logger)
	corsMiddleware := middleware.NewCORS(nil)

	// Health check (no auth required)
	mux.HandleFunc("/health", handleHealth)

	// Public API routes (require auth and rate limiting)
	mux.Handle("/v1/chat/completions", applyMiddleware(
		http.HandlerFunc(chatHandler.HandleChatCompletion),
		authMiddleware.Authenticate,
		rateLimitMiddleware.RateLimit,
	))

	mux.Handle("/v1/usage", applyMiddleware(
		http.HandlerFunc(usageHandler.HandleGetUsage),
		authMiddleware.Authenticate,
	))

	mux.Handle("/v1/usage/stats", applyMiddleware(
		http.HandlerFunc(usageHandler.HandleGetUsageStats),
		authMiddleware.Authenticate,
	))

	// Admin endpoints have been removed - use the CLI client management mode instead
	// Run: ./bin/server --client

	// Apply global middleware
	handler := corsMiddleware.Handle(mux)
	handler = loggerMiddleware.Log(handler)

	return handler
}

// handleHealth handles health check requests
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// applyMiddleware applies middleware in reverse order
func applyMiddleware(h http.Handler, middleware ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middleware) - 1; i >= 0; i-- {
		h = middleware[i](h)
	}
	return h
}
