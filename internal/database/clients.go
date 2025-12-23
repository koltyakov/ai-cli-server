package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/andrew/ai-cli-server/internal/database/models"
)

// CreateClient creates a new client in the database
func (db *DB) CreateClient(client *models.Client) error {
	query := `
		INSERT INTO clients (name, api_key_hash, provider, allowed_models, default_model, rate_limit_per_minute, expires_at, is_active, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := db.conn.Exec(
		query,
		client.Name,
		client.APIKeyHash,
		client.Provider,
		client.AllowedModels,
		client.DefaultModel,
		client.RateLimitPerMinute,
		client.ExpiresAt,
		client.IsActive,
		client.Metadata,
	)
	if err != nil {
		return fmt.Errorf("failed to insert client: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}
	client.ID = id
	client.CreatedAt = time.Now()
	client.UpdatedAt = time.Now()

	return nil
}

// GetClientByAPIKeyHash retrieves a client by API key hash
func (db *DB) GetClientByAPIKeyHash(keyHash string) (*models.Client, error) {
	query := `
		SELECT id, name, api_key_hash, provider, allowed_models, COALESCE(default_model, ''),
			   rate_limit_per_minute, created_at, updated_at, expires_at, is_active, metadata
		FROM clients
		WHERE api_key_hash = ?
	`

	var client models.Client
	err := db.conn.QueryRow(query, keyHash).Scan(
		&client.ID,
		&client.Name,
		&client.APIKeyHash,
		&client.Provider,
		&client.AllowedModels,
		&client.DefaultModel,
		&client.RateLimitPerMinute,
		&client.CreatedAt,
		&client.UpdatedAt,
		&client.ExpiresAt,
		&client.IsActive,
		&client.Metadata,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}

	return &client, nil
}

// GetClientByID retrieves a client by ID
func (db *DB) GetClientByID(id int64) (*models.Client, error) {
	query := `
		SELECT id, name, api_key_hash, provider, allowed_models, COALESCE(default_model, ''),
			   rate_limit_per_minute, created_at, updated_at, expires_at, is_active, metadata
		FROM clients
		WHERE id = ?
	`

	var client models.Client
	err := db.conn.QueryRow(query, id).Scan(
		&client.ID,
		&client.Name,
		&client.APIKeyHash,
		&client.Provider,
		&client.AllowedModels,
		&client.DefaultModel,
		&client.RateLimitPerMinute,
		&client.CreatedAt,
		&client.UpdatedAt,
		&client.ExpiresAt,
		&client.IsActive,
		&client.Metadata,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}

	return &client, nil
}

// ListClients retrieves all clients
func (db *DB) ListClients() ([]models.Client, error) {
	query := `
		SELECT id, name, api_key_hash, provider, allowed_models, COALESCE(default_model, ''),
			   rate_limit_per_minute, created_at, updated_at, expires_at, is_active, metadata
		FROM clients
		ORDER BY created_at DESC
	`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query clients: %w", err)
	}
	defer rows.Close()

	var clients []models.Client
	for rows.Next() {
		var client models.Client
		err := rows.Scan(
			&client.ID,
			&client.Name,
			&client.APIKeyHash,
			&client.Provider,
			&client.AllowedModels,
			&client.DefaultModel,
			&client.RateLimitPerMinute,
			&client.CreatedAt,
			&client.UpdatedAt,
			&client.ExpiresAt,
			&client.IsActive,
			&client.Metadata,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan client: %w", err)
		}
		clients = append(clients, client)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating clients: %w", err)
	}

	return clients, nil
}

// UpdateClient updates a client's information
func (db *DB) UpdateClient(client *models.Client) error {
	query := `
		UPDATE clients
		SET name = ?, provider = ?, allowed_models = ?, default_model = ?,
			rate_limit_per_minute = ?, expires_at = ?, is_active = ?, metadata = ?, updated_at = ?
		WHERE id = ?
	`

	client.UpdatedAt = time.Now()
	_, err := db.conn.Exec(
		query,
		client.Name,
		client.Provider,
		client.AllowedModels,
		client.DefaultModel,
		client.RateLimitPerMinute,
		client.ExpiresAt,
		client.IsActive,
		client.Metadata,
		client.UpdatedAt,
		client.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update client: %w", err)
	}

	return nil
}

// DeleteClient deletes a client by ID
func (db *DB) DeleteClient(id int64) error {
	query := `DELETE FROM clients WHERE id = ?`
	_, err := db.conn.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete client: %w", err)
	}
	return nil
}

// IsModelAllowed checks if a model is in the client's allowed models list
func IsModelAllowed(client *models.Client, model string) bool {
	var allowedModels []string
	if err := json.Unmarshal([]byte(client.AllowedModels), &allowedModels); err != nil {
		return false
	}

	for _, allowedModel := range allowedModels {
		if allowedModel == model || allowedModel == "*" {
			return true
		}
	}
	return false
}
