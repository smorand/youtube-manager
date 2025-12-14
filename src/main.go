package main

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "youtube-manager",
	Short: "YouTube Manager - Download videos and manage playlists",
	Long:  "Manage YouTube content using YouTube Data API v3 and yt-dlp",
}

func main() {
	// Initialize structured logging
	initLogging()

	// Register all commands
	rootCmd.AddCommand(listPlaylistsCmd)
	rootCmd.AddCommand(getPlaylistCmd)
	rootCmd.AddCommand(downloadCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(getVideoCmd)
	rootCmd.AddCommand(createPlaylistCmd)
	rootCmd.AddCommand(deletePlaylistCmd)
	rootCmd.AddCommand(addToPlaylistCmd)

	if err := rootCmd.Execute(); err != nil {
		slog.Error("Command execution failed", "error", err)
		os.Exit(1)
	}
}

func initLogging() {
	// Configure structured logging with slog
	opts := &slog.HandlerOptions{
		Level: slog.LevelError, // Only log errors by default (user-facing CLI)
	}
	handler := slog.NewTextHandler(os.Stderr, opts)
	slog.SetDefault(slog.New(handler))
}
