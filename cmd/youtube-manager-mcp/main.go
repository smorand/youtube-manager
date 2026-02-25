package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/mark3labs/mcp-go/server"

	"youtube-manager/internal/mcpserver"
)

func main() {
	// Log to stderr (stdout is used for MCP JSON-RPC in stdio mode)
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	handler := slog.NewTextHandler(os.Stderr, opts)
	slog.SetDefault(slog.New(handler))

	ctx := context.Background()

	// Initialize auth and YouTube services
	slog.Info("Initializing YouTube Manager MCP server...")
	srv, err := mcpserver.NewServer(ctx)
	if err != nil {
		slog.Error("Failed to initialize server", "error", err)
		os.Exit(1)
	}

	// Check if PORT is set (Cloud Run / HTTP mode)
	port := os.Getenv("PORT")
	if port != "" {
		startHTTP(srv, port)
	} else {
		startStdio(srv)
	}
}

// startStdio starts the MCP server over stdio transport.
func startStdio(srv *mcpserver.Server) {
	slog.Info("MCP server ready, starting stdio transport")
	if err := server.ServeStdio(srv.MCPServer()); err != nil {
		slog.Error("Server error", "error", err)
		os.Exit(1)
	}
}

// startHTTP starts the MCP server over HTTP transport with a health endpoint.
func startHTTP(srv *mcpserver.Server, port string) {
	// Create the StreamableHTTP server for MCP
	httpServer := server.NewStreamableHTTPServer(srv.MCPServer())

	// Create HTTP mux with health check and MCP endpoint
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	})
	mux.Handle("/mcp", httpServer)

	addr := ":" + port
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	slog.Info("MCP server ready, starting HTTP transport", "addr", addr)
	if err := httpSrv.ListenAndServe(); err != nil {
		slog.Error("HTTP server error", "error", err)
		os.Exit(1)
	}
}
