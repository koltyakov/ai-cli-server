package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	CLI      CLIConfig      `yaml:"cli"`
	Auth     AuthConfig     `yaml:"auth"`
	Logging  LoggingConfig  `yaml:"logging"`
}

// ServerConfig contains HTTP server configuration
type ServerConfig struct {
	Host         string        `yaml:"host"`
	Port         int           `yaml:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
}

// DatabaseConfig contains database configuration
type DatabaseConfig struct {
	Path string `yaml:"path"`
}

// CLIConfig contains CLI tool configurations
type CLIConfig struct {
	Copilot CopilotConfig `yaml:"copilot"`
	Cursor  CursorConfig  `yaml:"cursor"`
}

// CopilotConfig contains GitHub Copilot CLI configuration
type CopilotConfig struct {
	BinaryPath string        `yaml:"binary_path"`
	Timeout    time.Duration `yaml:"timeout"`
}

// CursorConfig contains Cursor CLI configuration
type CursorConfig struct {
	BinaryPath string        `yaml:"binary_path"`
	Timeout    time.Duration `yaml:"timeout"`
}

// AuthConfig contains authentication configuration
type AuthConfig struct {
	CopilotGitHubToken string `yaml:"-"` // Not in YAML, loaded from env
	CursorAPIKey       string `yaml:"-"` // Not in YAML, loaded from env
}

// LoggingConfig contains logging configuration
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// Load loads configuration from a YAML file and environment variables
func Load(configPath string) (*Config, error) {
	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Load sensitive config from environment variables
	cfg.Auth.CopilotGitHubToken = getEnv("COPILOT_GITHUB_TOKEN", getEnv("GH_TOKEN", ""))
	cfg.Auth.CursorAPIKey = getEnv("CURSOR_API_KEY", "")

	return &cfg, nil
}

// getEnv gets an environment variable with a default fallback
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Address returns the server address string
func (s *ServerConfig) Address() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}
