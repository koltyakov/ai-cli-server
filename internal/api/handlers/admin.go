package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/andrew/ai-cli-server/internal/auth"
	"github.com/andrew/ai-cli-server/internal/database"
	"github.com/andrew/ai-cli-server/internal/database/models"
)

// AdminHandler handles administrative operations
type AdminHandler struct {
	db *database.DB
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(db *database.DB) *AdminHandler {
	return &AdminHandler{db: db}
}

// CreateClientRequest represents a request to create a new client
type CreateClientRequest struct {
	Name               string   `json:"name"`
	AllowedModels      []string `json:"allowed_models"`
	RateLimitPerMinute int      `json:"rate_limit_per_minute"`
	ExpiresAt          *string  `json:"expires_at,omitempty"`
}

// CreateClientResponse represents the response with the generated API key
type CreateClientResponse struct {
	Client *models.Client `json:"client"`
	APIKey string         `json:"api_key"`
}

// HandleCreateClient handles POST /admin/clients
func (h *AdminHandler) HandleCreateClient(w http.ResponseWriter, r *http.Request) {
	var req CreateClientRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate request
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	if len(req.AllowedModels) == 0 {
		respondError(w, http.StatusBadRequest, "allowed_models is required")
		return
	}
	if req.RateLimitPerMinute <= 0 {
		req.RateLimitPerMinute = 60 // Default
	}

	// Generate API key
	apiKey, err := auth.GenerateAPIKey()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate API key")
		return
	}

	// Hash API key
	keyHash := auth.HashAPIKey(apiKey)

	// Convert allowed models to JSON
	allowedModelsJSON, err := json.Marshal(req.AllowedModels)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to serialize allowed models")
		return
	}

	// Parse expires_at if provided
	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid expires_at format, use RFC3339")
			return
		}
		expiresAt = &t
	}

	// Create client
	client := &models.Client{
		Name:               req.Name,
		APIKeyHash:         keyHash,
		AllowedModels:      string(allowedModelsJSON),
		RateLimitPerMinute: req.RateLimitPerMinute,
		ExpiresAt:          expiresAt,
		IsActive:           true,
	}

	if err := h.db.CreateClient(client); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create client")
		return
	}

	// Return client and API key (only time the key is shown)
	response := CreateClientResponse{
		Client: client,
		APIKey: apiKey,
	}

	respondJSON(w, http.StatusCreated, response)
}

// HandleListClients handles GET /admin/clients
func (h *AdminHandler) HandleListClients(w http.ResponseWriter, r *http.Request) {
	clients, err := h.db.ListClients()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list clients")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"clients": clients,
	})
}

// HandleGetClient handles GET /admin/clients/{id}
func (h *AdminHandler) HandleGetClient(w http.ResponseWriter, r *http.Request) {
	// Extract ID from path (simplified - in production use a router)
	idStr := r.URL.Path[len("/admin/clients/"):]
	id := int64(0)
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		respondError(w, http.StatusBadRequest, "invalid client ID")
		return
	}

	client, err := h.db.GetClientByID(id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get client")
		return
	}

	if client == nil {
		respondError(w, http.StatusNotFound, "client not found")
		return
	}

	respondJSON(w, http.StatusOK, client)
}

// HandleDeleteClient handles DELETE /admin/clients/{id}
func (h *AdminHandler) HandleDeleteClient(w http.ResponseWriter, r *http.Request) {
	// Extract ID from path
	idStr := r.URL.Path[len("/admin/clients/"):]
	id := int64(0)
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		respondError(w, http.StatusBadRequest, "invalid client ID")
		return
	}

	if err := h.db.DeleteClient(id); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete client")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
