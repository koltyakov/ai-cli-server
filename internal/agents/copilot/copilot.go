package copilot

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sync"
	"time"

	"github.com/andrew/ai-cli-server/internal/agents"
)

// Provider implements the CLI provider interface for GitHub Copilot CLI
type Provider struct {
	binaryPath   string
	timeout      time.Duration
	token        string
	modelsCache  []agents.ModelInfo
	modelsCached bool
	mu           sync.RWMutex
}

// NewProvider creates a new Copilot CLI provider
func NewProvider(binaryPath string, timeout time.Duration, token string) *Provider {
	if binaryPath == "" {
		binaryPath = "copilot"
	}
	if timeout == 0 {
		timeout = 120 * time.Second
	}
	return &Provider{
		binaryPath: binaryPath,
		timeout:    timeout,
		token:      token,
	}
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "copilot"
}

// IsAvailable checks if the copilot CLI binary is available
func (p *Provider) IsAvailable() bool {
	_, err := exec.LookPath(p.binaryPath)
	return err == nil
}

// fetchModelsFromCLI parses the copilot --help output to get available models
func (p *Provider) fetchModelsFromCLI() []agents.ModelInfo {
	cmd := exec.Command(p.binaryPath, "-h")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil
	}

	// Parse model choices from help output
	// Format: --model <model>   Set the AI model to use (choices: "model1", "model2", ...)
	helpText := string(output)

	// Find the model choices section
	re := regexp.MustCompile(`--model\s+<model>\s+[^(]*\(choices:\s*([^)]+)\)`)
	matches := re.FindStringSubmatch(helpText)
	if len(matches) < 2 {
		return nil
	}

	// Extract model names from quoted strings
	modelRe := regexp.MustCompile(`"([^"]+)"`)
	modelMatches := modelRe.FindAllStringSubmatch(matches[1], -1)

	var models []agents.ModelInfo
	for _, m := range modelMatches {
		if len(m) >= 2 {
			models = append(models, agents.ModelInfo{
				Name:    m[1],
				Enabled: true,
			})
		}
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
	if models != nil {
		p.modelsCache = models
		p.modelsCached = true
	}

	return p.modelsCache
}

// GetSupportedModels returns the models supported by Copilot CLI
func (p *Provider) GetSupportedModels() []string {
	models := p.GetModelsInfo()
	if len(models) == 0 {
		// Fallback to hardcoded list if CLI parsing fails
		return []string{
			"claude-sonnet-4.5",
			"claude-haiku-4.5",
			"claude-opus-4.5",
			"claude-sonnet-4",
			"gpt-5.1-codex-max",
			"gpt-5.1-codex",
			"gpt-5.2",
			"gpt-5.1",
			"gpt-5",
			"gpt-5.1-codex-mini",
			"gpt-5-mini",
			"gpt-4.1",
			"gemini-3-pro-preview",
		}
	}

	var names []string
	for _, m := range models {
		if m.Enabled {
			names = append(names, m.Name)
		}
	}
	return names
}

// Execute runs a prompt against the Copilot CLI
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
	// Use -s (silent) to output only the response, and --allow-all-tools for non-interactive mode
	args := []string{"-p", req.Prompt, "-s", "--allow-all-tools"}

	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}

	for _, tool := range req.AllowTools {
		args = append(args, "--allow-tool", tool)
	}

	for _, tool := range req.DenyTools {
		args = append(args, "--deny-tool", tool)
	}

	// Create command
	cmd := exec.CommandContext(ctx, p.binaryPath, args...)

	// Set environment variables
	env := os.Environ()
	if p.token != "" {
		env = append(env, "COPILOT_GITHUB_TOKEN="+p.token)
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
		return nil, fmt.Errorf("copilot CLI execution failed: %w, output: %s", err, string(output))
	}

	// Copilot CLI with -s flag returns plain text output, not JSON
	content := string(output)

	responseTime := time.Since(startTime)

	// Estimate tokens
	promptTokens := agents.EstimateTokens(req.Prompt)
	completionTokens := agents.EstimateTokens(content)

	return &agents.ExecuteResponse{
		Content:          content,
		Model:            req.Model, // Use the requested model since copilot doesn't return it
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
		ResponseTime:     responseTime,
		SessionID:        "",
	}, nil
}
