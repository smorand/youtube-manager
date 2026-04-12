package main

import (
	"context"
	"log/slog"
	"os"
	"strconv"

	"github.com/mark3labs/mcp-go/server"

	"youtube-manager/internal/mcpserver"
	"youtube-manager/internal/observability"
)

func main() {
	// Log to stderr (stdout is used for MCP JSON-RPC in stdio mode)
	observability.InitLogger(os.Getenv("LOG_LEVEL"))

	ctx := context.Background()

	// Build configuration from environment
	config := buildConfig()

	slog.Info("Initializing YouTube Manager MCP server...")
	srv, err := mcpserver.NewServer(ctx, config)
	if err != nil {
		slog.Error("Failed to initialize server", "error", err)
		os.Exit(1)
	}

	// HTTP mode (Cloud Run) vs stdio mode (local)
	if config != nil && config.BaseURL != "" {
		slog.Info("Starting HTTP mode with OAuth2")
		if err := srv.Run(ctx); err != nil {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	} else {
		slog.Info("MCP server ready, starting stdio transport")
		if err := server.ServeStdio(srv.MCPServer()); err != nil {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	}
}

// buildConfig creates a server config from environment variables.
// Returns nil if no HTTP/OAuth2 configuration is present (stdio mode).
func buildConfig() *mcpserver.Config {
	baseURL := os.Getenv("BASE_URL")
	port := os.Getenv("PORT")

	// No BASE_URL means stdio mode
	if baseURL == "" && port == "" {
		return nil
	}

	portNum := 8080
	if port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			portNum = p
		}
	}

	return &mcpserver.Config{
		Host:            "0.0.0.0",
		Port:            portNum,
		BaseURL:         baseURL,
		SecretProject:   os.Getenv("SECRET_PROJECT"),
		SecretName:      os.Getenv("SECRET_NAME"),
		CredentialFile:  os.Getenv("OAUTH_CREDENTIALS_FILE"),
		VaultAddr:       os.Getenv("VAULT_ADDR"),
		VaultSecretPath: os.Getenv("VAULT_SECRET_PATH"),
		VaultToken:      os.Getenv("VAULT_TOKEN"),
	}
}
