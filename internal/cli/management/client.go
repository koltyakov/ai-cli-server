package management

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/andrew/ai-cli-server/internal/agents"
	"github.com/andrew/ai-cli-server/internal/agents/copilot"
	"github.com/andrew/ai-cli-server/internal/agents/cursor"
	"github.com/andrew/ai-cli-server/internal/auth"
	"github.com/andrew/ai-cli-server/internal/config"
	"github.com/andrew/ai-cli-server/internal/database"
	"github.com/andrew/ai-cli-server/internal/database/models"
)

// ClientManager handles interactive client management
type ClientManager struct {
	db              *database.DB
	copilotProvider *copilot.Provider
	cursorProvider  *cursor.Provider
	availableModels map[string][]string
	modelsInfo      map[string][]agents.ModelInfo
}

// NewClientManager creates a new client manager
func NewClientManager(cfg *config.Config, db *database.DB) *ClientManager {
	copilotProv := copilot.NewProvider(
		cfg.CLI.Copilot.BinaryPath,
		cfg.CLI.Copilot.Timeout,
		cfg.Auth.CopilotGitHubToken,
	)
	cursorProv := cursor.NewProvider(
		cfg.CLI.Cursor.BinaryPath,
		cfg.CLI.Cursor.Timeout,
		cfg.Auth.CursorAPIKey,
	)

	availableModels := make(map[string][]string)
	modelsInfo := make(map[string][]agents.ModelInfo)

	if copilotProv.IsAvailable() {
		availableModels["copilot"] = copilotProv.GetSupportedModels()
		modelsInfo["copilot"] = copilotProv.GetModelsInfo()
	}
	if cursorProv.IsAvailable() {
		availableModels["cursor"] = cursorProv.GetSupportedModels()
		modelsInfo["cursor"] = cursorProv.GetModelsInfo()
	}

	return &ClientManager{
		db:              db,
		copilotProvider: copilotProv,
		cursorProvider:  cursorProv,
		availableModels: availableModels,
		modelsInfo:      modelsInfo,
	}
}

// Run starts the interactive TUI
func (cm *ClientManager) Run() error {
	for {
		var action string
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("AI CLI Server - Client Management").
					Options(
						huh.NewOption("Add new client", "add"),
						huh.NewOption("List clients", "list"),
						huh.NewOption("Delete client", "delete"),
						huh.NewOption("Exit", "exit"),
					).
					Value(&action),
			),
		)

		if err := form.Run(); err != nil {
			if err == huh.ErrUserAborted {
				fmt.Println("\nGoodbye!")
				return nil
			}
			return err
		}

		switch action {
		case "add":
			if err := cm.addClientInteractive(); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		case "list":
			if err := cm.listClientsInteractive(); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		case "delete":
			if err := cm.deleteClientInteractive(); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		case "exit":
			fmt.Println("\nGoodbye!")
			return nil
		}
	}
}

// AddClientInput represents JSON input for automation
type AddClientInput struct {
	Name      string   `json:"name"`
	Provider  string   `json:"provider"`
	Models    []string `json:"models"`
	RateLimit int      `json:"rate_limit"`
}

// AddClientOutput represents JSON output for automation
type AddClientOutput struct {
	Success      bool   `json:"success"`
	ClientID     int64  `json:"client_id,omitempty"`
	APIKey       string `json:"api_key,omitempty"`
	Provider     string `json:"provider,omitempty"`
	DefaultModel string `json:"default_model,omitempty"`
	Error        string `json:"error,omitempty"`
}

// ClientOutput represents a client in JSON output
type ClientOutput struct {
	ID            int64    `json:"id"`
	Name          string   `json:"name"`
	Provider      string   `json:"provider"`
	AllowedModels []string `json:"allowed_models"`
	DefaultModel  string   `json:"default_model"`
	RateLimit     int      `json:"rate_limit"`
	IsActive      bool     `json:"is_active"`
	CreatedAt     string   `json:"created_at"`
}

// ListClientsOutput represents JSON output for list command
type ListClientsOutput struct {
	Success bool           `json:"success"`
	Clients []ClientOutput `json:"clients,omitempty"`
	Error   string         `json:"error,omitempty"`
}

// ModelInfoOutput represents model information in JSON output
type ModelInfoOutput struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// ProviderModelsOutput represents a provider's models in JSON output
type ProviderModelsOutput struct {
	Provider  string            `json:"provider"`
	Available bool              `json:"available"`
	Models    []ModelInfoOutput `json:"models"`
}

// ListModelsOutput represents JSON output for models command
type ListModelsOutput struct {
	Success   bool                   `json:"success"`
	Providers []ProviderModelsOutput `json:"providers,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// DeleteClientOutput represents JSON output for delete command
type DeleteClientOutput struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// AddClientJSON handles automated client creation with JSON I/O
func (cm *ClientManager) AddClientJSON(inputJSON string) {
	var input AddClientInput
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		output := AddClientOutput{Success: false, Error: fmt.Sprintf("invalid JSON input: %v", err)}
		cm.printJSON(output)
		os.Exit(1)
		return
	}

	// Validate input
	if input.Name == "" {
		output := AddClientOutput{Success: false, Error: "name is required"}
		cm.printJSON(output)
		os.Exit(1)
		return
	}

	// Default provider to first available
	if input.Provider == "" {
		for p := range cm.availableModels {
			input.Provider = p
			break
		}
	}

	// Validate provider is available
	if _, ok := cm.availableModels[input.Provider]; !ok {
		output := AddClientOutput{Success: false, Error: fmt.Sprintf("provider '%s' is not available", input.Provider)}
		cm.printJSON(output)
		os.Exit(1)
		return
	}

	if len(input.Models) == 0 {
		input.Models = []string{"*"}
	}
	if input.RateLimit == 0 {
		input.RateLimit = 60
	}

	// Determine default model
	defaultModel := ""
	if len(input.Models) > 0 && input.Models[0] != "*" {
		defaultModel = input.Models[0]
	} else if models, ok := cm.availableModels[input.Provider]; ok && len(models) > 0 {
		defaultModel = models[0]
	}

	// Generate API key
	apiKey, err := auth.GenerateAPIKey()
	if err != nil {
		output := AddClientOutput{Success: false, Error: fmt.Sprintf("failed to generate API key: %v", err)}
		cm.printJSON(output)
		os.Exit(1)
		return
	}

	modelsJSON, _ := json.Marshal(input.Models)

	client := &models.Client{
		Name:               input.Name,
		APIKeyHash:         auth.HashAPIKey(apiKey),
		Provider:           input.Provider,
		AllowedModels:      string(modelsJSON),
		DefaultModel:       defaultModel,
		RateLimitPerMinute: input.RateLimit,
		IsActive:           true,
	}

	if err := cm.db.CreateClient(client); err != nil {
		output := AddClientOutput{Success: false, Error: fmt.Sprintf("failed to create client: %v", err)}
		cm.printJSON(output)
		os.Exit(1)
		return
	}

	output := AddClientOutput{
		Success:      true,
		ClientID:     client.ID,
		APIKey:       apiKey,
		Provider:     input.Provider,
		DefaultModel: defaultModel,
	}
	cm.printJSON(output)
}

// ListModelsJSON handles automated model listing with JSON output
func (cm *ClientManager) ListModelsJSON() {
	var providers []ProviderModelsOutput

	// Copilot
	copilotAvailable := cm.copilotProvider.IsAvailable()
	var copilotModels []ModelInfoOutput
	if copilotAvailable {
		for _, m := range cm.modelsInfo["copilot"] {
			copilotModels = append(copilotModels, ModelInfoOutput{
				Name:    m.Name,
				Enabled: m.Enabled,
			})
		}
	}
	providers = append(providers, ProviderModelsOutput{
		Provider:  "copilot",
		Available: copilotAvailable,
		Models:    copilotModels,
	})

	// Cursor
	cursorAvailable := cm.cursorProvider.IsAvailable()
	var cursorModels []ModelInfoOutput
	if cursorAvailable {
		for _, m := range cm.modelsInfo["cursor"] {
			cursorModels = append(cursorModels, ModelInfoOutput{
				Name:    m.Name,
				Enabled: m.Enabled,
			})
		}
	}
	providers = append(providers, ProviderModelsOutput{
		Provider:  "cursor",
		Available: cursorAvailable,
		Models:    cursorModels,
	})

	output := ListModelsOutput{
		Success:   true,
		Providers: providers,
	}
	cm.printJSON(output)
}

// ListClientsJSON handles automated client listing with JSON output
func (cm *ClientManager) ListClientsJSON() {
	clients, err := cm.db.ListClients()
	if err != nil {
		output := ListClientsOutput{Success: false, Error: fmt.Sprintf("failed to list clients: %v", err)}
		cm.printJSON(output)
		os.Exit(1)
		return
	}

	clientOutputs := make([]ClientOutput, len(clients))
	for i, c := range clients {
		var models []string
		json.Unmarshal([]byte(c.AllowedModels), &models)

		clientOutputs[i] = ClientOutput{
			ID:            c.ID,
			Name:          c.Name,
			Provider:      c.Provider,
			AllowedModels: models,
			DefaultModel:  c.DefaultModel,
			RateLimit:     c.RateLimitPerMinute,
			IsActive:      c.IsActive,
			CreatedAt:     c.CreatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	output := ListClientsOutput{Success: true, Clients: clientOutputs}
	cm.printJSON(output)
}

// DeleteClientJSON handles automated client deletion with JSON I/O
func (cm *ClientManager) DeleteClientJSON(clientID int64) {
	// Delete usage logs first
	if err := cm.db.DeleteUsageLogsByClient(clientID); err != nil {
		output := DeleteClientOutput{Success: false, Error: fmt.Sprintf("failed to delete usage logs: %v", err)}
		cm.printJSON(output)
		os.Exit(1)
		return
	}

	if err := cm.db.DeleteClient(clientID); err != nil {
		output := DeleteClientOutput{Success: false, Error: fmt.Sprintf("failed to delete client: %v", err)}
		cm.printJSON(output)
		os.Exit(1)
		return
	}

	output := DeleteClientOutput{Success: true}
	cm.printJSON(output)
}

func (cm *ClientManager) printJSON(v interface{}) {
	data, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(data))
}

func (cm *ClientManager) addClientInteractive() error {
	var name string
	var selectedProvider string
	var selectedModels []string
	var rateLimit int
	var defaultModel string

	// Get available providers
	providerOptions := []huh.Option[string]{}
	for provider := range cm.availableModels {
		providerOptions = append(providerOptions, huh.NewOption(provider, provider))
	}

	if len(providerOptions) == 0 {
		return fmt.Errorf("no providers available. Make sure Copilot or Cursor CLI is installed")
	}

	// Step 1: Get client name and select provider
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Client Name").
				Placeholder("my-app").
				Value(&name).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("name cannot be empty")
					}
					return nil
				}),

			huh.NewSelect[string]().
				Title("Select AI Provider").
				Description("Each client uses one provider").
				Options(providerOptions...).
				Value(&selectedProvider),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	// Step 2: Select models from chosen provider
	modelOptions := []huh.Option[string]{}
	modelOptions = append(modelOptions, huh.NewOption("* (All models)", "*"))
	if modelsInfo, ok := cm.modelsInfo[selectedProvider]; ok {
		for _, m := range modelsInfo {
			if m.Enabled {
				modelOptions = append(modelOptions, huh.NewOption(m.Name, m.Name))
			}
		}
	}

	form = huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select Allowed Models").
				Description("Use space to select, enter to confirm").
				Options(modelOptions...).
				Value(&selectedModels).
				Validate(func(s []string) error {
					if len(s) == 0 {
						return fmt.Errorf("at least one model must be selected")
					}
					return nil
				}),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	// Step 3: Select default model
	defaultModelOptions := []huh.Option[string]{}

	// Add models that are in selectedModels (excluding "*")
	for _, model := range selectedModels {
		if model != "*" {
			defaultModelOptions = append(defaultModelOptions, huh.NewOption(model, model))
		}
	}
	// If "*" was selected or no specific models, show all models from provider
	if len(defaultModelOptions) == 0 || containsString(selectedModels, "*") {
		defaultModelOptions = []huh.Option[string]{}
		if modelsInfo, ok := cm.modelsInfo[selectedProvider]; ok {
			for _, m := range modelsInfo {
				if m.Enabled {
					defaultModelOptions = append(defaultModelOptions, huh.NewOption(m.Name, m.Name))
				}
			}
		}
	}

	if len(defaultModelOptions) > 0 {
		form = huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Default Model").
					Description("Used when request doesn't specify model").
					Options(defaultModelOptions...).
					Value(&defaultModel),
			),
		)

		if err := form.Run(); err != nil {
			return err
		}
	}

	// Step 4: Set rate limit
	rateLimitStr := "60"
	form = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Rate Limit").
				Description("Requests per minute (0 for unlimited)").
				Placeholder("60").
				Value(&rateLimitStr),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	fmt.Sscanf(rateLimitStr, "%d", &rateLimit)
	if rateLimit < 0 {
		rateLimit = 0
	}

	// Generate API key
	apiKey, err := auth.GenerateAPIKey()
	if err != nil {
		return fmt.Errorf("failed to generate API key: %w", err)
	}

	modelsJSON, _ := json.Marshal(selectedModels)

	client := &models.Client{
		Name:               name,
		APIKeyHash:         auth.HashAPIKey(apiKey),
		Provider:           selectedProvider,
		AllowedModels:      string(modelsJSON),
		DefaultModel:       defaultModel,
		RateLimitPerMinute: rateLimit,
		IsActive:           true,
	}

	if err := cm.db.CreateClient(client); err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	fmt.Println()
	fmt.Println("✅ Client created successfully!")
	fmt.Println()
	fmt.Printf("   Client ID:     %d\n", client.ID)
	fmt.Printf("   Name:          %s\n", name)
	fmt.Printf("   API Key:       %s\n", apiKey)
	fmt.Printf("   Provider:      %s\n", selectedProvider)
	fmt.Printf("   Models:        %v\n", selectedModels)
	fmt.Printf("   Default Model: %s\n", defaultModel)
	fmt.Printf("   Rate Limit:    %d req/min\n", rateLimit)
	fmt.Println()
	fmt.Println("⚠️  Save the API key - it won't be shown again!")
	fmt.Println()

	return nil
}

func (cm *ClientManager) listClientsInteractive() error {
	clients, err := cm.db.ListClients()
	if err != nil {
		return fmt.Errorf("failed to list clients: %w", err)
	}

	if len(clients) == 0 {
		fmt.Println("\nNo clients found.")
		return nil
	}

	fmt.Println("\n=== Clients ===")
	for _, client := range clients {
		var models []string
		json.Unmarshal([]byte(client.AllowedModels), &models)

		status := "✅ Active"
		if !client.IsActive {
			status = "❌ Inactive"
		}

		fmt.Printf("\nID: %d | %s\n", client.ID, status)
		fmt.Printf("   Name:          %s\n", client.Name)
		fmt.Printf("   Provider:      %s\n", client.Provider)
		fmt.Printf("   Models:        %v\n", models)
		fmt.Printf("   Default Model: %s\n", client.DefaultModel)
		fmt.Printf("   Rate Limit:    %d req/min\n", client.RateLimitPerMinute)
		fmt.Printf("   Created:       %s\n", client.CreatedAt.Format("2006-01-02 15:04:05"))
	}
	fmt.Println()

	return nil
}

func (cm *ClientManager) deleteClientInteractive() error {
	clients, err := cm.db.ListClients()
	if err != nil {
		return fmt.Errorf("failed to list clients: %w", err)
	}

	if len(clients) == 0 {
		fmt.Println("\nNo clients found.")
		return nil
	}

	// Build options
	options := []huh.Option[int64]{}
	options = append(options, huh.NewOption("Cancel", int64(0)))
	for _, c := range clients {
		label := fmt.Sprintf("%s (ID: %d)", c.Name, c.ID)
		options = append(options, huh.NewOption(label, c.ID))
	}

	var selectedID int64
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[int64]().
				Title("Select Client to Delete").
				Options(options...).
				Value(&selectedID),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	if selectedID == 0 {
		fmt.Println("\nCancelled.")
		return nil
	}

	// Find client name for confirmation
	var clientName string
	for _, c := range clients {
		if c.ID == selectedID {
			clientName = c.Name
			break
		}
	}

	// Confirm deletion
	var confirm bool
	form = huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Delete client '%s' and ALL their history?", clientName)).
				Affirmative("Yes, delete").
				Negative("No, cancel").
				Value(&confirm),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	if !confirm {
		fmt.Println("\nCancelled.")
		return nil
	}

	// Delete usage logs first
	if err := cm.db.DeleteUsageLogsByClient(selectedID); err != nil {
		return fmt.Errorf("failed to delete usage logs: %w", err)
	}

	if err := cm.db.DeleteClient(selectedID); err != nil {
		return fmt.Errorf("failed to delete client: %w", err)
	}

	fmt.Printf("\n✅ Client '%s' and all their history has been deleted.\n\n", clientName)

	return nil
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
