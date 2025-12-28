package cli

import (
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

	cmd := &cobra.Command{
		Use:   "download <url>",
		Short: "Download a YouTube video using yt-dlp",
		Long:  "Download a YouTube video. Requires yt-dlp to be installed.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDownload(args[0], outputDir, format, audioOnly)
		},
	}

	cmd.Flags().StringVar(&outputDir, "output", ".", "Output directory")
	cmd.Flags().StringVar(&format, "format", "best", "Video format")
	cmd.Flags().BoolVar(&audioOnly, "audio-only", false, "Download audio only (MP3)")

	return cmd
}

func runDownload(url, outputDir, format string, audioOnly bool) error {
	downloader := download.NewDownloader(outputDir, format, audioOnly)
	return downloader.Download(url)
}
