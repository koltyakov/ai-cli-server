package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/andrew/ai-cli-server/internal/database/models"
)

// CreateUsageLog inserts a new usage log entry
func (db *DB) CreateUsageLog(log *models.UsageLog) error {
	query := `
		INSERT INTO usage_logs (
			client_id, session_id, timestamp, provider, model,
			prompt, prompt_tokens, completion_tokens, total_tokens,
			cost, response_time_ms, response_status, error_message
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := db.conn.Exec(
		query,
		log.ClientID,
		log.SessionID,
		log.Timestamp,
		log.Provider,
		log.Model,
		log.Prompt,
		log.PromptTokens,
		log.CompletionTokens,
		log.TotalTokens,
		log.Cost,
		log.ResponseTimeMs,
		log.ResponseStatus,
		log.ErrorMessage,
	)
	if err != nil {
		return fmt.Errorf("failed to insert usage log: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}
	log.ID = id

	return nil
}

// GetUsageLogs retrieves usage logs for a client with optional filters
func (db *DB) GetUsageLogs(clientID int64, limit, offset int, startTime, endTime *time.Time) ([]models.UsageLog, error) {
	query := `
		SELECT id, client_id, session_id, timestamp, provider, model,
			   prompt, prompt_tokens, completion_tokens, total_tokens,
			   cost, response_time_ms, response_status, error_message
		FROM usage_logs
		WHERE client_id = ?
	`
	args := []interface{}{clientID}

	if startTime != nil {
		query += " AND timestamp >= ?"
		args = append(args, startTime)
	}
	if endTime != nil {
		query += " AND timestamp <= ?"
		args = append(args, endTime)
	}

	query += " ORDER BY timestamp DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query usage logs: %w", err)
	}
	defer rows.Close()

	var logs []models.UsageLog
	for rows.Next() {
		var log models.UsageLog
		err := rows.Scan(
			&log.ID,
			&log.ClientID,
			&log.SessionID,
			&log.Timestamp,
			&log.Provider,
			&log.Model,
			&log.Prompt,
			&log.PromptTokens,
			&log.CompletionTokens,
			&log.TotalTokens,
			&log.Cost,
			&log.ResponseTimeMs,
			&log.ResponseStatus,
			&log.ErrorMessage,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan usage log: %w", err)
		}
		logs = append(logs, log)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating usage logs: %w", err)
	}

	return logs, nil
}

// GetUsageStats calculates aggregated usage statistics for a client
func (db *DB) GetUsageStats(clientID int64, startTime, endTime *time.Time) (*models.UsageStats, error) {
	query := `
		SELECT 
			COUNT(*) as total_requests,
			COALESCE(SUM(total_tokens), 0) as total_tokens,
			COALESCE(SUM(cost), 0) as total_cost
		FROM usage_logs
		WHERE client_id = ?
	`
	args := []interface{}{clientID}

	if startTime != nil {
		query += " AND timestamp >= ?"
		args = append(args, startTime)
	}
	if endTime != nil {
		query += " AND timestamp <= ?"
		args = append(args, endTime)
	}

	var stats models.UsageStats
	err := db.conn.QueryRow(query, args...).Scan(
		&stats.TotalRequests,
		&stats.TotalTokens,
		&stats.TotalCost,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage stats: %w", err)
	}

	// Get breakdown by provider
	stats.ByProvider = make(map[string]int)
	providerQuery := `
		SELECT provider, COUNT(*) as count
		FROM usage_logs
		WHERE client_id = ?
	`
	providerArgs := []interface{}{clientID}
	if startTime != nil {
		providerQuery += " AND timestamp >= ?"
		providerArgs = append(providerArgs, startTime)
	}
	if endTime != nil {
		providerQuery += " AND timestamp <= ?"
		providerArgs = append(providerArgs, endTime)
	}
	providerQuery += " GROUP BY provider"

	rows, err := db.conn.Query(providerQuery, providerArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var provider string
		var count int
		if err := rows.Scan(&provider, &count); err != nil {
			return nil, fmt.Errorf("failed to scan provider stats: %w", err)
		}
		stats.ByProvider[provider] = count
	}

	// Get breakdown by model
	stats.ByModel = make(map[string]int)
	modelQuery := `
		SELECT model, COUNT(*) as count
		FROM usage_logs
		WHERE client_id = ?
	`
	modelArgs := []interface{}{clientID}
	if startTime != nil {
		modelQuery += " AND timestamp >= ?"
		modelArgs = append(modelArgs, startTime)
	}
	if endTime != nil {
		modelQuery += " AND timestamp <= ?"
		modelArgs = append(modelArgs, endTime)
	}
	modelQuery += " GROUP BY model"

	rows, err = db.conn.Query(modelQuery, modelArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to get model stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var model string
		var count int
		if err := rows.Scan(&model, &count); err != nil {
			return nil, fmt.Errorf("failed to scan model stats: %w", err)
		}
		stats.ByModel[model] = count
	}

	return &stats, nil
}

// DeleteUsageLogsByClient deletes all usage logs for a specific client
func (db *DB) DeleteUsageLogsByClient(clientID int64) error {
	query := `DELETE FROM usage_logs WHERE client_id = ?`
	_, err := db.conn.Exec(query, clientID)
	return err
}

// IncrementRateLimitBucket increments the request count for a client's rate limit bucket
func (db *DB) IncrementRateLimitBucket(clientID int64, windowStart time.Time) error {
	query := `
		INSERT INTO rate_limit_buckets (client_id, window_start, request_count)
		VALUES (?, ?, 1)
		ON CONFLICT(client_id, window_start) DO UPDATE SET request_count = request_count + 1
	`
	_, err := db.conn.Exec(query, clientID, windowStart)
	if err != nil {
		return fmt.Errorf("failed to increment rate limit bucket: %w", err)
	}
	return nil
}

// GetRateLimitCount returns the current request count for a client's rate limit window
func (db *DB) GetRateLimitCount(clientID int64, windowStart time.Time) (int, error) {
	query := `
		SELECT COALESCE(request_count, 0)
		FROM rate_limit_buckets
		WHERE client_id = ? AND window_start = ?
	`
	var count int
	err := db.conn.QueryRow(query, clientID, windowStart).Scan(&count)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get rate limit count: %w", err)
	}
	return count, nil
}

// CleanupOldRateLimitBuckets removes rate limit buckets older than the specified time
func (db *DB) CleanupOldRateLimitBuckets(before time.Time) error {
	query := `DELETE FROM rate_limit_buckets WHERE window_start < ?`
	_, err := db.conn.Exec(query, before)
	if err != nil {
		return fmt.Errorf("failed to cleanup old rate limit buckets: %w", err)
	}
	return nil
}
