package auth

import (
"crypto/rand"
"crypto/sha256"
"encoding/base64"
"fmt"
)

const (
// APIKeyLength is the length of generated API keys in bytes (32 bytes = 256 bits)
APIKeyLength = 32

// APIKeyPrefix is the prefix for all API keys
APIKeyPrefix = "aics_"
)

// GenerateAPIKey generates a new random API key
func GenerateAPIKey() (string, error) {
	bytes := make([]byte, APIKeyLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	key := APIKeyPrefix + base64.URLEncoding.EncodeToString(bytes)
	return key, nil
}

// HashAPIKey creates a SHA-256 hash of an API key for storage
func HashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return base64.URLEncoding.EncodeToString(hash[:])
}

// ValidateAPIKeyFormat checks if an API key has the correct format
func ValidateAPIKeyFormat(key string) bool {
	if len(key) < len(APIKeyPrefix) {
		return false
	}
	return key[:len(APIKeyPrefix)] == APIKeyPrefix
}
