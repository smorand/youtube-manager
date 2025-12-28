package main

import (
	"log/slog"
	"os"

	"youtube-manager/internal/cli"
)

func main() {
	// Initialize structured logging
	initLogging()

	// Execute CLI
	if err := cli.Execute(); err != nil {
		slog.Error("Command execution failed", "error", err)
		os.Exit(1)
	}
}

// initLogging configures structured logging with slog
func initLogging() {
	opts := &slog.HandlerOptions{
		Level: slog.LevelError, // Only log errors by default (user-facing CLI)
	}
	handler := slog.NewTextHandler(os.Stderr, opts)
	slog.SetDefault(slog.New(handler))
}
