package models

import "time"

type Client struct {
	ID                 int64      `json:"id"`
	Name               string     `json:"name"`
	APIKeyHash         string     `json:"-"`
	Provider           string     `json:"provider"`       // Single provider: copilot or cursor
	AllowedModels      string     `json:"allowed_models"` // JSON array of allowed models
	DefaultModel       string     `json:"default_model"`  // Default model for requests
	RateLimitPerMinute int        `json:"rate_limit_per_minute"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	ExpiresAt          *time.Time `json:"expires_at,omitempty"`
	IsActive           bool       `json:"is_active"`
	Metadata           string     `json:"metadata,omitempty"`
}

type UsageLog struct {
	ID               int64     `json:"id"`
	ClientID         int64     `json:"client_id"`
	SessionID        *string   `json:"session_id,omitempty"`
	Timestamp        time.Time `json:"timestamp"`
	Provider         string    `json:"provider"`
	Model            string    `json:"model"`
	Prompt           *string   `json:"prompt,omitempty"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	Cost             float64   `json:"cost"`
	ResponseTimeMs   int       `json:"response_time_ms"`
	ResponseStatus   int       `json:"response_status"`
	ErrorMessage     *string   `json:"error_message,omitempty"`
}

type UsageStats struct {
	TotalRequests int            `json:"total_requests"`
	TotalTokens   int64          `json:"total_tokens"`
	TotalCost     float64        `json:"total_cost"`
	ByProvider    map[string]int `json:"by_provider"`
	ByModel       map[string]int `json:"by_model"`
}
