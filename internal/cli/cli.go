// Package cli provides the command-line interface for youtube-manager.
package cli

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "youtube-manager",
	Short: "YouTube Manager - Download videos and manage playlists",
	Long:  "Manage YouTube content using YouTube Data API v3 and yt-dlp",
}

// Execute runs the CLI application.
func Execute() error {
	// Register all commands
	registerPlaylistCommands()
	registerVideoCommands()
	registerDownloadCommands()

	return rootCmd.Execute()
}
