package agents

import (
	"context"
	"time"
)

// ModelInfo contains information about a supported model
type ModelInfo struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// Provider defines the interface for CLI tool providers
type Provider interface {
	// Execute runs a prompt against the CLI tool and returns the response
	Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error)

	// Name returns the provider name (e.g., "copilot", "cursor")
	Name() string

	// IsAvailable checks if the CLI binary is available
	IsAvailable() bool

	// GetSupportedModels returns list of models supported by this provider
	GetSupportedModels() []string

	// GetModelsInfo returns detailed model information
	GetModelsInfo() []ModelInfo
}

// ExecuteRequest represents a request to execute a CLI command
type ExecuteRequest struct {
	Prompt           string            `json:"prompt"`
	Model            string            `json:"model,omitempty"`
	AllowTools       []string          `json:"allow_tools,omitempty"`
	DenyTools        []string          `json:"deny_tools,omitempty"`
	Force            bool              `json:"force,omitempty"`
	WorkingDirectory string            `json:"working_directory,omitempty"`
	EnvironmentVars  map[string]string `json:"environment_vars,omitempty"`
	Timeout          time.Duration     `json:"timeout,omitempty"`
}

// ExecuteResponse represents the response from a CLI execution
type ExecuteResponse struct {
	Content          string                 `json:"content"`
	Model            string                 `json:"model"`
	PromptTokens     int                    `json:"prompt_tokens"`
	CompletionTokens int                    `json:"completion_tokens"`
	TotalTokens      int                    `json:"total_tokens"`
	ResponseTime     time.Duration          `json:"response_time"`
	SessionID        string                 `json:"session_id,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// EstimateTokens provides a rough token estimate for text (4 chars ≈ 1 token)
func EstimateTokens(text string) int {
	return len(text) / 4
}
