package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"youtube-manager/internal/auth"
	"youtube-manager/internal/youtube"
)

// registerPlaylistCommands adds playlist-related commands to the root command.
func registerPlaylistCommands() {
	rootCmd.AddCommand(createListPlaylistsCmd())
	rootCmd.AddCommand(createGetPlaylistCmd())
	rootCmd.AddCommand(createCreatePlaylistCmd())
	rootCmd.AddCommand(createDeletePlaylistCmd())
	rootCmd.AddCommand(createAddToPlaylistCmd())
}

// createListPlaylistsCmd creates the list-playlists command.
func createListPlaylistsCmd() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "list-playlists",
		Short: "List your YouTube playlists",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runListPlaylists(cmd.Context(), limit)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of playlists to return")
	return cmd
}

func runListPlaylists(ctx context.Context, limit int) error {
	authClient, err := auth.NewClient()
	if err != nil {
		return err
	}

	service, err := authClient.GetYouTubeService(ctx)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "📋 Fetching playlists...\n\n")

	playlistSvc := youtube.NewPlaylistService(service)
	playlists, err := playlistSvc.List(ctx, limit)
	if err != nil {
		return err
	}

	youtube.PrintPlaylists(playlists)
	return nil
}

// createGetPlaylistCmd creates the get-playlist command.
func createGetPlaylistCmd() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "get-playlist <playlist-id>",
		Short: "Get videos from a playlist",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGetPlaylist(cmd.Context(), args[0], limit)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of videos to return")
	return cmd
}

func runGetPlaylist(ctx context.Context, playlistID string, limit int) error {
	authClient, err := auth.NewClient()
	if err != nil {
		return err
	}

	service, err := authClient.GetYouTubeService(ctx)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "🔍 Fetching videos from playlist: %s...\n\n", playlistID)

	playlistSvc := youtube.NewPlaylistService(service)
	items, err := playlistSvc.GetItems(ctx, playlistID, limit)
	if err != nil {
		return err
	}

	youtube.PrintPlaylistItems(items)
	return nil
}

// createCreatePlaylistCmd creates the create-playlist command.
func createCreatePlaylistCmd() *cobra.Command {
	var description, privacy string

	cmd := &cobra.Command{
		Use:   "create-playlist <title>",
		Short: "Create a new playlist",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreatePlaylist(cmd.Context(), args[0], description, privacy)
		},
	}

	cmd.Flags().StringVar(&description, "description", "", "Playlist description")
	cmd.Flags().StringVar(&privacy, "privacy", "private", "Privacy status (private, public, unlisted)")
	return cmd
}

func runCreatePlaylist(ctx context.Context, title, description, privacy string) error {
	authClient, err := auth.NewClient()
	if err != nil {
		return err
	}

	service, err := authClient.GetYouTubeService(ctx)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "📝 Creating playlist: %s...\n\n", title)

	playlistSvc := youtube.NewPlaylistService(service)
	playlist, err := playlistSvc.Create(ctx, title, description, privacy)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "✅ Playlist created successfully!\n")
	fmt.Printf("   ID: %s\n", playlist.Id)
	fmt.Printf("   Link: https://www.youtube.com/playlist?list=%s\n", playlist.Id)

	return nil
}

// createDeletePlaylistCmd creates the delete-playlist command.
func createDeletePlaylistCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete-playlist <playlist-id>",
		Short: "Delete a playlist",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDeletePlaylist(cmd.Context(), args[0])
		},
	}
}

func runDeletePlaylist(ctx context.Context, playlistID string) error {
	authClient, err := auth.NewClient()
	if err != nil {
		return err
	}

	service, err := authClient.GetYouTubeService(ctx)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "🗑️  Deleting playlist: %s...\n\n", playlistID)

	playlistSvc := youtube.NewPlaylistService(service)
	if err := playlistSvc.Delete(ctx, playlistID); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "✅ Playlist deleted successfully!\n")
	return nil
}

// createAddToPlaylistCmd creates the add-to-playlist command.
func createAddToPlaylistCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add-to-playlist <playlist-id> <video-id>",
		Short: "Add a video to a playlist",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAddToPlaylist(cmd.Context(), args[0], args[1])
		},
	}
}

func runAddToPlaylist(ctx context.Context, playlistID, videoID string) error {
	authClient, err := auth.NewClient()
	if err != nil {
		return err
	}

	service, err := authClient.GetYouTubeService(ctx)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "➕ Adding video %s to playlist %s...\n\n", videoID, playlistID)

	playlistSvc := youtube.NewPlaylistService(service)
	if err := playlistSvc.AddVideo(ctx, playlistID, videoID); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "✅ Video added to playlist successfully!\n")
	return nil
}
