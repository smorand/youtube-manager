package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"youtube-manager/internal/auth"
	"youtube-manager/internal/youtube"
)

// registerVideoCommands adds video-related commands to the root command.
func registerVideoCommands() {
	rootCmd.AddCommand(createSearchCmd())
	rootCmd.AddCommand(createGetVideoCmd())
}

// createSearchCmd creates the search command.
func createSearchCmd() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search for videos on YouTube",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(cmd.Context(), args[0], limit)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum number of results")
	return cmd
}

func runSearch(ctx context.Context, query string, limit int) error {
	authClient, err := auth.NewClient()
	if err != nil {
		return err
	}

	service, err := authClient.GetYouTubeService(ctx)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "🔍 Searching for: \"%s\"...\n\n", query)

	videoSvc := youtube.NewVideoService(service)
	results, err := videoSvc.Search(ctx, query, limit)
	if err != nil {
		return err
	}

	youtube.PrintSearchResults(results)
	return nil
}

// createGetVideoCmd creates the get-video command.
func createGetVideoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get-video <video-id>",
		Short: "Get detailed information about a video",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGetVideo(cmd.Context(), args[0])
		},
	}
}

func runGetVideo(ctx context.Context, videoID string) error {
	authClient, err := auth.NewClient()
	if err != nil {
		return err
	}

	service, err := authClient.GetYouTubeService(ctx)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "📹 Fetching video info: %s...\n\n", videoID)

	videoSvc := youtube.NewVideoService(service)
	video, err := videoSvc.Get(ctx, videoID)
	if err != nil {
		return err
	}

	youtube.PrintVideo(video)
	return nil
}
