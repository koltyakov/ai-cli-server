package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/andrew/ai-cli-server/internal/auth"
	"github.com/andrew/ai-cli-server/internal/database"
	"github.com/andrew/ai-cli-server/internal/database/models"
	"golang.org/x/time/rate"
)

// ClientContextKey is the key for storing client in request context
type contextKey string

const ClientContextKey contextKey = "client"

// AuthMiddleware validates API keys and loads client information
type AuthMiddleware struct {
	db *database.DB
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(db *database.DB) *AuthMiddleware {
	return &AuthMiddleware{db: db}
}

// Authenticate validates the API key and loads client into context
func (m *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract API key from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			respondJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "missing authorization header",
			})
			return
		}

		// Parse Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			respondJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "invalid authorization header format",
			})
			return
		}

		apiKey := parts[1]

		// Validate API key format
		if !auth.ValidateAPIKeyFormat(apiKey) {
			respondJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "invalid API key format",
			})
			return
		}

		// Hash and lookup client
		keyHash := auth.HashAPIKey(apiKey)
		client, err := m.db.GetClientByAPIKeyHash(keyHash)
		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "failed to validate API key",
			})
			return
		}

		if client == nil {
			respondJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "invalid API key",
			})
			return
		}

		// Check if client is active
		if !client.IsActive {
			respondJSON(w, http.StatusForbidden, map[string]string{
				"error": "API key is inactive",
			})
			return
		}

		// Check if client is expired
		if client.ExpiresAt != nil && client.ExpiresAt.Before(time.Now()) {
			respondJSON(w, http.StatusForbidden, map[string]string{
				"error": "API key has expired",
			})
			return
		}

		// Add client to context
		ctx := context.WithValue(r.Context(), ClientContextKey, client)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RateLimitMiddleware implements per-client rate limiting
type RateLimitMiddleware struct {
	db       *database.DB
	limiters map[int64]*rate.Limiter
	mu       sync.RWMutex
}

// NewRateLimitMiddleware creates a new rate limiting middleware
func NewRateLimitMiddleware(db *database.DB) *RateLimitMiddleware {
	m := &RateLimitMiddleware{
		db:       db,
		limiters: make(map[int64]*rate.Limiter),
	}

	// Start cleanup goroutine
	go m.cleanupLimiters()

	return m
}

// RateLimit enforces rate limits per client
func (m *RateLimitMiddleware) RateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		client := GetClientFromContext(r.Context())
		if client == nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "client not found in context",
			})
			return
		}

		// Get or create limiter for this client
		limiter := m.getLimiter(client.ID, client.RateLimitPerMinute)

		// Check rate limit
		if !limiter.Allow() {
			respondJSON(w, http.StatusTooManyRequests, map[string]string{
				"error": "rate limit exceeded",
			})
			return
		}

		// Record in database for persistent tracking
		windowStart := time.Now().Truncate(time.Minute)
		if err := m.db.IncrementRateLimitBucket(client.ID, windowStart); err != nil {
			// Log error but don't fail the request
		}

		next.ServeHTTP(w, r)
	})
}

// getLimiter gets or creates a rate limiter for a client
func (m *RateLimitMiddleware) getLimiter(clientID int64, ratePerMinute int) *rate.Limiter {
	m.mu.RLock()
	limiter, exists := m.limiters[clientID]
	m.mu.RUnlock()

	if exists {
		return limiter
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if limiter, exists := m.limiters[clientID]; exists {
		return limiter
	}

	// Create new limiter (rate per minute converted to per second)
	ratePerSecond := float64(ratePerMinute) / 60.0
	limiter = rate.NewLimiter(rate.Limit(ratePerSecond), ratePerMinute)
	m.limiters[clientID] = limiter

	return limiter
}

// cleanupLimiters removes inactive limiters periodically
func (m *RateLimitMiddleware) cleanupLimiters() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		// Cleanup old rate limit buckets in database
		if err := m.db.CleanupOldRateLimitBuckets(time.Now().Add(-1 * time.Hour)); err != nil {
			// Log error
		}
	}
}

// GetClientFromContext retrieves the client from request context
func GetClientFromContext(ctx context.Context) *models.Client {
	client, ok := ctx.Value(ClientContextKey).(*models.Client)
	if !ok {
		return nil
	}
	return client
}

// respondJSON sends a JSON response
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
