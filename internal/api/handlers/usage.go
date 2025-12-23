package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/andrew/ai-cli-server/internal/api/middleware"
	"github.com/andrew/ai-cli-server/internal/database"
)

// UsageHandler handles usage tracking requests
type UsageHandler struct {
	db *database.DB
}

// NewUsageHandler creates a new usage handler
func NewUsageHandler(db *database.DB) *UsageHandler {
	return &UsageHandler{db: db}
}

// HandleGetUsage handles GET /v1/usage
func (h *UsageHandler) HandleGetUsage(w http.ResponseWriter, r *http.Request) {
	client := middleware.GetClientFromContext(r.Context())
	if client == nil {
		respondError(w, http.StatusInternalServerError, "client not found in context")
		return
	}

	// Parse query parameters
	query := r.URL.Query()
	limit := 100
	offset := 0

	if l := query.Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	if o := query.Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	var startTime, endTime *time.Time
	if st := query.Get("start_time"); st != "" {
		if t, err := time.Parse(time.RFC3339, st); err == nil {
			startTime = &t
		}
	}
	if et := query.Get("end_time"); et != "" {
		if t, err := time.Parse(time.RFC3339, et); err == nil {
			endTime = &t
		}
	}

	// Get usage logs
	logs, err := h.db.GetUsageLogs(client.ID, limit, offset, startTime, endTime)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to retrieve usage logs")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"logs":   logs,
		"limit":  limit,
		"offset": offset,
	})
}

// HandleGetUsageStats handles GET /v1/usage/stats
func (h *UsageHandler) HandleGetUsageStats(w http.ResponseWriter, r *http.Request) {
	client := middleware.GetClientFromContext(r.Context())
	if client == nil {
		respondError(w, http.StatusInternalServerError, "client not found in context")
		return
	}

	// Parse query parameters
	query := r.URL.Query()
	var startTime, endTime *time.Time

	if st := query.Get("start_time"); st != "" {
		if t, err := time.Parse(time.RFC3339, st); err == nil {
			startTime = &t
		}
	}
	if et := query.Get("end_time"); et != "" {
		if t, err := time.Parse(time.RFC3339, et); err == nil {
			endTime = &t
		}
	}

	// Get usage stats
	stats, err := h.db.GetUsageStats(client.ID, startTime, endTime)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to retrieve usage stats")
		return
	}

	respondJSON(w, http.StatusOK, stats)
}
