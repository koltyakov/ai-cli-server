package copilot

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"time"

	"github.com/andrew/ai-cli-server/internal/agents"
)

// Provider implements the CLI provider interface for GitHub Copilot CLI
type Provider struct {
	agents.BaseProvider
	timeout time.Duration
	token   string
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
		BaseProvider: agents.BaseProvider{BinaryPath: binaryPath},
		timeout:      timeout,
		token:        token,
	}
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "copilot"
}

// modelPattern matches: --model <model>   Set the AI model to use (choices: "model1", "model2", ...)
var modelPattern = regexp.MustCompile(`--model\s+<model>\s+[^(]*\(choices:\s*([^)]+)\)`)

// fetchModelsFromCLI parses the copilot --help output to get available models
func (p *Provider) fetchModelsFromCLI() []agents.ModelInfo {
	cmd := exec.Command(p.BinaryPath, "-h")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil
	}
	return p.ParseModelsFromHelp(string(output), modelPattern, agents.ParseQuotedModels)
}

// GetModelsInfo returns detailed model information
func (p *Provider) GetModelsInfo() []agents.ModelInfo {
	return p.GetCachedModels(p.fetchModelsFromCLI)
}

// GetSupportedModels returns the models supported by Copilot CLI
func (p *Provider) GetSupportedModels() []string {
	return agents.ModelsToNames(p.GetModelsInfo())
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
	cmd := exec.CommandContext(ctx, p.BinaryPath, args...)

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
