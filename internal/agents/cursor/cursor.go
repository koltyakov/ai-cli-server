package cursor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sync"
	"time"

	"github.com/andrew/ai-cli-server/internal/agents"
)

// Provider implements the CLI provider interface for Cursor CLI
type Provider struct {
	binaryPath   string
	timeout      time.Duration
	apiKey       string
	modelsCache  []agents.ModelInfo
	modelsCached bool
	mu           sync.RWMutex
}

// NewProvider creates a new Cursor CLI provider
func NewProvider(binaryPath string, timeout time.Duration, apiKey string) *Provider {
	if binaryPath == "" {
		binaryPath = "cursor-agent"
	}
	if timeout == 0 {
		timeout = 120 * time.Second
	}
	return &Provider{
		binaryPath: binaryPath,
		timeout:    timeout,
		apiKey:     apiKey,
	}
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "cursor"
}

// IsAvailable checks if the cursor-agent CLI binary is available
func (p *Provider) IsAvailable() bool {
	_, err := exec.LookPath(p.binaryPath)
	return err == nil
}

// fetchModelsFromCLI parses the cursor-agent --help output to get available models
func (p *Provider) fetchModelsFromCLI() []agents.ModelInfo {
	cmd := exec.Command(p.binaryPath, "-h")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil
	}

	// Parse model examples from help output
	// Format: --model <model>  Model to use (e.g., gpt-5, sonnet-4, sonnet-4-thinking)
	helpText := string(output)

	// Find the model examples section
	re := regexp.MustCompile(`--model\s+<model>\s+[^(]*\(e\.g\.?,?\s*([^)]+)\)`)
	matches := re.FindStringSubmatch(helpText)
	if len(matches) < 2 {
		return nil
	}

	// Extract model names (comma-separated, may have spaces)
	modelStr := matches[1]
	modelRe := regexp.MustCompile(`[a-zA-Z0-9._-]+`)
	modelMatches := modelRe.FindAllString(modelStr, -1)

	var models []agents.ModelInfo
	for _, name := range modelMatches {
		models = append(models, agents.ModelInfo{
			Name:    name,
			Enabled: true,
		})
	}

	return models
}

// GetModelsInfo returns detailed model information
func (p *Provider) GetModelsInfo() []agents.ModelInfo {
	p.mu.RLock()
	if p.modelsCached {
		defer p.mu.RUnlock()
		return p.modelsCache
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check after acquiring write lock
	if p.modelsCached {
		return p.modelsCache
	}

	models := p.fetchModelsFromCLI()
	if len(models) > 0 {
		p.modelsCache = models
		p.modelsCached = true
	}

	return p.modelsCache
}

// GetSupportedModels returns the models supported by Cursor CLI
func (p *Provider) GetSupportedModels() []string {
	models := p.GetModelsInfo()
	if len(models) == 0 {
		// Fallback if CLI parsing fails - return empty, provider unusable without models
		return nil
	}

	var names []string
	for _, m := range models {
		if m.Enabled {
			names = append(names, m.Name)
		}
	}
	return names
}

// Execute runs a prompt against the Cursor CLI
func (p *Provider) Execute(ctx context.Context, req agents.ExecuteRequest) (*agents.ExecuteResponse, error) {
	startTime := time.Now()

	// Set timeout
	timeout := p.timeout
	if req.Timeout > 0 {
		timeout = req.Timeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Build command arguments
	args := []string{"-p", "--output-format", "json", req.Prompt}

	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}

	if req.Force {
		args = append(args, "--force")
	}

	// Create command
	cmd := exec.CommandContext(ctx, p.binaryPath, args...)

	// Set environment variables
	env := os.Environ()
	if p.apiKey != "" {
		env = append(env, "CURSOR_API_KEY="+p.apiKey)
	}
	if req.WorkingDirectory != "" {
		cmd.Dir = req.WorkingDirectory
	}
	for k, v := range req.EnvironmentVars {
		env = append(env, k+"="+v)
	}
	cmd.Env = env

	// Execute command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("cursor CLI execution failed: %w, output: %s", err, string(output))
	}

	// Parse JSON output
	var result struct {
		Content  string `json:"content"`
		Model    string `json:"model"`
		Metadata struct {
			SessionID string `json:"session_id"`
		} `json:"metadata"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		// If JSON parsing fails, return raw output
		result.Content = string(output)
	}

	responseTime := time.Since(startTime)

	// Estimate tokens
	promptTokens := agents.EstimateTokens(req.Prompt)
	completionTokens := agents.EstimateTokens(result.Content)

	return &agents.ExecuteResponse{
		Content:          result.Content,
		Model:            result.Model,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
		ResponseTime:     responseTime,
		SessionID:        result.Metadata.SessionID,
	}, nil
}
