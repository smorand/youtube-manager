package cli

import (
	"context"

	"github.com/spf13/cobra"

	"youtube-manager/internal/download"
)

// registerDownloadCommands adds download-related commands to the root command.
func registerDownloadCommands() {
	rootCmd.AddCommand(createDownloadCmd())
}

// createDownloadCmd creates the download command.
func createDownloadCmd() *cobra.Command {
	var outputDir, format string
	var audioOnly bool
	var extractFrom, extractTo int

	cmd := &cobra.Command{
		Use:   "download <url-or-video-id>",
		Short: "Download a YouTube video",
		Long: "Download a YouTube video natively. Accepts a full URL or just a video ID.\n\n" +
			"Examples:\n" +
			"  youtube-manager download dQw4w9WgXcQ\n" +
			"  youtube-manager download https://www.youtube.com/watch?v=dQw4w9WgXcQ\n" +
			"  youtube-manager download dQw4w9WgXcQ --audio-only\n" +
			"  youtube-manager download dQw4w9WgXcQ --format 720p\n" +
			"  youtube-manager download dQw4w9WgXcQ --audio-only --extract-from 10 --extract-to 60\n" +
			"  youtube-manager download dQw4w9WgXcQ --extract-to 30",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			url := download.ResolveVideoURL(args[0])
			return runDownload(cmd.Context(), url, outputDir, format, audioOnly, extractFrom, extractTo)
		},
	}

	cmd.Flags().StringVar(&outputDir, "output", ".", "Output directory")
	cmd.Flags().StringVar(&format, "format", "best", "Video format (e.g., 720p, 1080p)")
	cmd.Flags().BoolVar(&audioOnly, "audio-only", false, "Download audio only (converts to MP3)")
	cmd.Flags().IntVar(&extractFrom, "extract-from", 0, "Start extraction at this second")
	cmd.Flags().IntVar(&extractTo, "extract-to", 0, "Stop extraction at this second")

	return cmd
}

func runDownload(ctx context.Context, url, outputDir, format string, audioOnly bool, extractFrom, extractTo int) error {
	downloader := download.NewDownloader(outputDir, format, audioOnly, extractFrom, extractTo)
	return downloader.Download(ctx, url)
}
