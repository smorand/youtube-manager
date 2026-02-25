package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/mark3labs/mcp-go/server"

	"youtube-manager/internal/mcpserver"
)

func main() {
	// Log to stderr (stdout is used for MCP JSON-RPC)
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	handler := slog.NewTextHandler(os.Stderr, opts)
	slog.SetDefault(slog.New(handler))

	ctx := context.Background()

	// Initialize auth and YouTube services before starting stdio
	slog.Info("Initializing YouTube Manager MCP server...")
	srv, err := mcpserver.NewServer(ctx)
	if err != nil {
		slog.Error("Failed to initialize server", "error", err)
		os.Exit(1)
	}

	slog.Info("MCP server ready, starting stdio transport")
	if err := server.ServeStdio(srv.MCPServer()); err != nil {
		slog.Error("Server error", "error", err)
		os.Exit(1)
	}
}
