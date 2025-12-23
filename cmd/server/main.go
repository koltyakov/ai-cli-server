package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/andrew/ai-cli-server/internal/agents/copilot"
	"github.com/andrew/ai-cli-server/internal/agents/cursor"
	"github.com/andrew/ai-cli-server/internal/api"
	"github.com/andrew/ai-cli-server/internal/cli/management"
	"github.com/andrew/ai-cli-server/internal/config"
	"github.com/andrew/ai-cli-server/internal/database"
)

func main() {
	// Parse command-line flags
	manageCmd := flag.Bool("manage", false, "Run interactive client management TUI")

	// Automation subcommands for scripting
	addClient := flag.String("add", "", "Add client with JSON input: {\"name\":\"...\", \"provider\":\"copilot\", \"models\":[\"*\"], \"rate_limit\":60}")
	listClients := flag.Bool("list", false, "List all clients (JSON output)")
	deleteClient := flag.Int64("delete", 0, "Delete client by ID")
	listModels := flag.Bool("models", false, "List available models (JSON output)")

	flag.Parse()

	// Setup logger
	logger := log.New(os.Stdout, "[ai-cli-server] ", log.LstdFlags)

	// Load configuration
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		logger.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	db, err := database.New(cfg.Database.Path)
	if err != nil {
		logger.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Handle automation commands (JSON I/O for scripting)
	if *listModels {
		manager := management.NewClientManager(cfg, db)
		manager.ListModelsJSON()
		return
	}

	if *addClient != "" {
		manager := management.NewClientManager(cfg, db)
		manager.AddClientJSON(*addClient)
		return
	}

	if *listClients {
		manager := management.NewClientManager(cfg, db)
		manager.ListClientsJSON()
		return
	}

	if *deleteClient > 0 {
		manager := management.NewClientManager(cfg, db)
		manager.DeleteClientJSON(*deleteClient)
		return
	}

	// Handle interactive management mode
	if *manageCmd {
		runClientManagement(cfg, db)
		return
	}

	// Default: run server
	runServer(cfg, db, logger)
}

func runServer(cfg *config.Config, db *database.DB, logger *log.Logger) {
	logger.Printf("Starting AI CLI Server on %s", cfg.Server.Address())
	logger.Printf("Database initialized at %s", cfg.Database.Path)

	// Initialize CLI providers
	copilotProvider := copilot.NewProvider(
		cfg.CLI.Copilot.BinaryPath,
		cfg.CLI.Copilot.Timeout,
		cfg.Auth.CopilotGitHubToken,
	)
	cursorProvider := cursor.NewProvider(
		cfg.CLI.Cursor.BinaryPath,
		cfg.CLI.Cursor.Timeout,
		cfg.Auth.CursorAPIKey,
	)

	// Check provider availability
	if copilotProvider.IsAvailable() {
		logger.Printf("Copilot CLI provider available")
	} else {
		logger.Printf("WARNING: Copilot CLI not found at %s", cfg.CLI.Copilot.BinaryPath)
	}

	if cursorProvider.IsAvailable() {
		logger.Printf("Cursor CLI provider available")
	} else {
		logger.Printf("WARNING: Cursor CLI not found at %s", cfg.CLI.Cursor.BinaryPath)
	}

	// Setup routes
	handler := api.SetupRoutes(db, copilotProvider, cursorProvider, logger)

	// Create HTTP server
	server := &http.Server{
		Addr:         cfg.Server.Address(),
		Handler:      handler,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start server in a goroutine
	go func() {
		logger.Printf("Server listening on http://%s", cfg.Server.Address())
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Println("Server shutting down...")

	// Gracefully shutdown the server with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Fatalf("Server forced to shutdown: %v", err)
	}

	logger.Println("Server exited")
}

func runClientManagement(cfg *config.Config, db *database.DB) {
	manager := management.NewClientManager(cfg, db)
	if err := manager.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
