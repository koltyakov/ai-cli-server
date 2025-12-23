package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/andrew/ai-cli-server/internal/agents"
	"github.com/andrew/ai-cli-server/internal/agents/copilot"
	"github.com/andrew/ai-cli-server/internal/agents/cursor"
	"github.com/andrew/ai-cli-server/internal/api/middleware"
	"github.com/andrew/ai-cli-server/internal/database"
	"github.com/andrew/ai-cli-server/internal/database/models"
)

// ChatHandler handles chat completion requests
type ChatHandler struct {
	db        *database.DB
	providers map[string]agents.Provider
}

// NewChatHandler creates a new chat handler
func NewChatHandler(db *database.DB, copilotProvider *copilot.Provider, cursorProvider *cursor.Provider) *ChatHandler {
	return &ChatHandler{
		db: db,
		providers: map[string]agents.Provider{
			"copilot": copilotProvider,
			"cursor":  cursorProvider,
		},
	}
}

// ChatCompletionRequest represents an incoming chat completion request
type ChatCompletionRequest struct {
	Provider         string    `json:"provider"`
	Model            string    `json:"model"`
	Messages         []Message `json:"messages"`
	AllowTools       []string  `json:"allow_tools,omitempty"`
	DenyTools        []string  `json:"deny_tools,omitempty"`
	Force            bool      `json:"force,omitempty"`
	WorkingDirectory string    `json:"working_directory,omitempty"`
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionResponse represents the response
type ChatCompletionResponse struct {
	ID               string  `json:"id"`
	Provider         string  `json:"provider"`
	Model            string  `json:"model"`
	Content          string  `json:"content"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	Cost             float64 `json:"cost"`
	DurationMs       int64   `json:"duration_ms"`
}

// HandleChatCompletion handles POST /v1/chat/completions
func (h *ChatHandler) HandleChatCompletion(w http.ResponseWriter, r *http.Request) {
	client := middleware.GetClientFromContext(r.Context())
	if client == nil {
		respondError(w, http.StatusInternalServerError, "client not found in context")
		return
	}

	// Parse request
	var req ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Client has a single provider - always use it
	req.Provider = client.Provider

	// Use client default model if not specified
	if req.Model == "" {
		if client.DefaultModel != "" {
			req.Model = client.DefaultModel
		} else {
			// Use first available model from provider
			if provider, ok := h.providers[req.Provider]; ok {
				models := provider.GetSupportedModels()
				if len(models) > 0 {
					req.Model = models[0]
				}
			}
		}
	}

	// Validate we have both provider and model
	if req.Model == "" {
		respondError(w, http.StatusBadRequest, "model is required (no default configured)")
		return
	}

	// Get provider
	provider, ok := h.providers[req.Provider]
	if !ok {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("unknown provider: %s", req.Provider))
		return
	}

	// Check if provider is available
	if !provider.IsAvailable() {
		respondError(w, http.StatusServiceUnavailable, fmt.Sprintf("provider %s is not available", req.Provider))
		return
	}

	// Check if model is allowed for this client
	if !database.IsModelAllowed(client, req.Model) && !database.IsModelAllowed(client, "*") {
		respondError(w, http.StatusForbidden, fmt.Sprintf("model %s is not allowed for this client", req.Model))
		return
	}

	// Convert messages to prompt (simple concatenation)
	prompt := h.messagesToPrompt(req.Messages)

	// Execute CLI request
	startTime := time.Now()
	cliReq := agents.ExecuteRequest{
		Prompt:           prompt,
		Model:            req.Model,
		AllowTools:       req.AllowTools,
		DenyTools:        req.DenyTools,
		Force:            req.Force,
		WorkingDirectory: req.WorkingDirectory,
	}

	resp, err := provider.Execute(r.Context(), cliReq)
	if err != nil {
		// Log error usage
		errorMsg := err.Error()
		usageLog := &models.UsageLog{
			ClientID:       client.ID,
			Timestamp:      time.Now(),
			Provider:       req.Provider,
			Model:          req.Model,
			Prompt:         &prompt,
			ResponseStatus: http.StatusInternalServerError,
			ResponseTimeMs: int(time.Since(startTime).Milliseconds()),
			ErrorMessage:   &errorMsg,
		}
		h.db.CreateUsageLog(usageLog)

		respondError(w, http.StatusInternalServerError, fmt.Sprintf("CLI execution failed: %v", err))
		return
	}

	// Calculate cost (simplified pricing)
	cost := h.calculateCost(req.Model, resp.TotalTokens)

	// Log usage
	usageLog := &models.UsageLog{
		ClientID:         client.ID,
		SessionID:        &resp.SessionID,
		Timestamp:        time.Now(),
		Provider:         req.Provider,
		Model:            resp.Model,
		Prompt:           &prompt,
		PromptTokens:     resp.PromptTokens,
		CompletionTokens: resp.CompletionTokens,
		TotalTokens:      resp.TotalTokens,
		Cost:             cost,
		ResponseStatus:   http.StatusOK,
		ResponseTimeMs:   int(resp.ResponseTime.Milliseconds()),
	}
	if err := h.db.CreateUsageLog(usageLog); err != nil {
		// Log error but don't fail the request
	}

	// Return response
	response := ChatCompletionResponse{
		ID:               fmt.Sprintf("chatcmpl-%d", usageLog.ID),
		Provider:         req.Provider,
		Model:            resp.Model,
		Content:          resp.Content,
		PromptTokens:     resp.PromptTokens,
		CompletionTokens: resp.CompletionTokens,
		TotalTokens:      resp.TotalTokens,
		Cost:             cost,
		DurationMs:       resp.ResponseTime.Milliseconds(),
	}

	respondJSON(w, http.StatusOK, response)
}

// messagesToPrompt converts messages to a single prompt string
func (h *ChatHandler) messagesToPrompt(messages []Message) string {
	var prompt string
	for _, msg := range messages {
		if msg.Role == "user" {
			prompt += msg.Content + "\n"
		}
	}
	return prompt
}

// calculateCost calculates the cost of a request based on tokens
func (h *ChatHandler) calculateCost(model string, tokens int) float64 {
	// Simplified pricing (per 1000 tokens)
	pricePerThousand := 0.01 // Default $0.01 per 1k tokens

	switch model {
	case "gpt-5":
		pricePerThousand = 0.05
	case "gpt-4o":
		pricePerThousand = 0.03
	case "claude-sonnet-4.5":
		pricePerThousand = 0.03
	case "claude-sonnet-3.5":
		pricePerThousand = 0.02
	case "gpt-4":
		pricePerThousand = 0.03
	case "o1-preview":
		pricePerThousand = 0.10
	case "o1-mini":
		pricePerThousand = 0.05
	}

	return float64(tokens) / 1000.0 * pricePerThousand
}
