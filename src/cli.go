package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"google.golang.org/api/youtube/v3"
)

var (
	green = color.New(color.FgGreen).SprintFunc()
	red   = color.New(color.FgRed).SprintFunc()
	cyan  = color.New(color.FgCyan).SprintFunc()
)

// listPlaylistsCmd lists user's playlists
var listLimit int

var listPlaylistsCmd = createListPlaylistsCmd()

func createListPlaylistsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-playlists",
		Short: "List your YouTube playlists",
		RunE:  runListPlaylists,
	}
	cmd.Flags().IntVar(&listLimit, "limit", 50, "Maximum number of playlists to return")
	return cmd
}

func runListPlaylists(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	service, err := getYouTubeService(ctx)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "📋 Fetching playlists...\n\n")

	call := service.Playlists.List([]string{"snippet", "contentDetails"}).Mine(true).MaxResults(int64(listLimit))
	response, err := call.Do()
	if err != nil {
		return fmt.Errorf("error fetching playlists: %w", err)
	}

	if len(response.Items) == 0 {
		fmt.Println("No playlists found.")
		return nil
	}

	fmt.Fprintf(os.Stderr, "✅ Found %d playlist(s):\n\n", len(response.Items))
	for _, playlist := range response.Items {
		fmt.Printf("📁 %s\n", playlist.Snippet.Title)
		fmt.Printf("   ID: %s\n", playlist.Id)
		fmt.Printf("   Videos: %d\n", playlist.ContentDetails.ItemCount)
		fmt.Printf("   Link: https://www.youtube.com/playlist?list=%s\n\n", playlist.Id)
	}

	return nil
}

// getPlaylistCmd gets videos from a playlist
var playlistLimit int

var getPlaylistCmd = createGetPlaylistCmd()

func createGetPlaylistCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get-playlist <playlist-id>",
		Short: "Get videos from a playlist",
		Args:  cobra.ExactArgs(1),
		RunE:  runGetPlaylist,
	}
	cmd.Flags().IntVar(&playlistLimit, "limit", 50, "Maximum number of videos to return")
	return cmd
}

func runGetPlaylist(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	playlistID := args[0]

	service, err := getYouTubeService(ctx)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "🔍 Fetching videos from playlist: %s...\n\n", playlistID)

	var allVideos []*youtube.PlaylistItem
	pageToken := ""

	for {
		call := service.PlaylistItems.List([]string{"snippet", "contentDetails"}).
			PlaylistId(playlistID).
			MaxResults(int64(playlistLimit))

		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		response, err := call.Do()
		if err != nil {
			return fmt.Errorf("error fetching playlist items: %w", err)
		}

		allVideos = append(allVideos, response.Items...)

		if response.NextPageToken == "" {
			break
		}
		pageToken = response.NextPageToken
	}

	if len(allVideos) == 0 {
		fmt.Println("No videos found in this playlist.")
		return nil
	}

	fmt.Fprintf(os.Stderr, "✅ Found %d video(s):\n\n", len(allVideos))
	for idx, video := range allVideos {
		fmt.Printf("%d. %s\n", idx+1, video.Snippet.Title)
		fmt.Printf("   Video ID: %s\n", video.ContentDetails.VideoId)
		fmt.Printf("   Channel: %s\n", video.Snippet.ChannelTitle)
		fmt.Printf("   Link: https://www.youtube.com/watch?v=%s\n\n", video.ContentDetails.VideoId)
	}

	return nil
}

// downloadCmd downloads a YouTube video
var (
	outputDir string
	format    string
	audioOnly bool
)

var downloadCmd = createDownloadCmd()

func createDownloadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "download <url>",
		Short: "Download a YouTube video using yt-dlp",
		Long:  "Download a YouTube video. Requires yt-dlp to be installed.",
		Args:  cobra.ExactArgs(1),
		RunE:  runDownload,
	}
	cmd.Flags().StringVar(&outputDir, "output", ".", "Output directory")
	cmd.Flags().StringVar(&format, "format", "best", "Video format")
	cmd.Flags().BoolVar(&audioOnly, "audio-only", false, "Download audio only (MP3)")
	return cmd
}

func runDownload(cmd *cobra.Command, args []string) error {
	url := args[0]

	// Check if yt-dlp is installed
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		return fmt.Errorf("yt-dlp not found in PATH. Please install it first:\n" +
			"  brew install yt-dlp  (macOS)\n" +
			"  pip install yt-dlp   (pip)")
	}

	fmt.Fprintf(os.Stderr, "⬇️  Downloading video: %s\n\n", url)

	// Build yt-dlp command
	args = []string{
		"-o", filepath.Join(outputDir, "%(title)s.%(ext)s"),
	}

	if audioOnly {
		args = append(args,
			"-f", "bestaudio/best",
			"-x",
			"--audio-format", "mp3",
			"--audio-quality", "192K",
		)
	} else {
		if format == "best" {
			args = append(args, "-f", "bestvideo+bestaudio/best")
		} else {
			args = append(args, "-f", format)
		}
	}

	args = append(args, url)

	ytdlp := exec.Command("yt-dlp", args...)
	ytdlp.Stdout = os.Stdout
	ytdlp.Stderr = os.Stderr

	if err := ytdlp.Run(); err != nil {
		return fmt.Errorf("error downloading video: %w", err)
	}

	fmt.Fprintf(os.Stderr, "\n✅ Download completed successfully\n")
	return nil
}

// searchCmd searches for videos
var searchLimit int

var searchCmd = createSearchCmd()

func createSearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search for videos on YouTube",
		Args:  cobra.ExactArgs(1),
		RunE:  runSearch,
	}
	cmd.Flags().IntVar(&searchLimit, "limit", 10, "Maximum number of results")
	return cmd
}

func runSearch(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	query := args[0]

	service, err := getYouTubeService(ctx)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "🔍 Searching for: \"%s\"...\n\n", query)

	call := service.Search.List([]string{"snippet"}).
		Q(query).
		Type("video").
		MaxResults(int64(searchLimit))

	response, err := call.Do()
	if err != nil {
		return fmt.Errorf("error searching videos: %w", err)
	}

	if len(response.Items) == 0 {
		fmt.Println("No videos found.")
		return nil
	}

	fmt.Fprintf(os.Stderr, "✅ Found %d video(s):\n\n", len(response.Items))
	for idx, item := range response.Items {
		fmt.Printf("%d. %s\n", idx+1, item.Snippet.Title)
		fmt.Printf("   Video ID: %s\n", item.Id.VideoId)
		fmt.Printf("   Channel: %s\n", item.Snippet.ChannelTitle)
		desc := item.Snippet.Description
		if len(desc) > 100 {
			desc = desc[:100] + "..."
		}
		fmt.Printf("   Description: %s\n", desc)
		fmt.Printf("   Link: https://www.youtube.com/watch?v=%s\n\n", item.Id.VideoId)
	}

	return nil
}

// getVideoCmd gets video information
var getVideoCmd = &cobra.Command{
	Use:   "get-video <video-id>",
	Short: "Get detailed information about a video",
	Args:  cobra.ExactArgs(1),
	RunE:  runGetVideo,
}

func runGetVideo(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	videoID := args[0]

	service, err := getYouTubeService(ctx)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "📹 Fetching video info: %s...\n\n", videoID)

	call := service.Videos.List([]string{"snippet", "contentDetails", "statistics"}).Id(videoID)
	response, err := call.Do()
	if err != nil {
		return fmt.Errorf("error fetching video: %w", err)
	}

	if len(response.Items) == 0 {
		fmt.Println("Video not found.")
		return nil
	}

	video := response.Items[0]
	snippet := video.Snippet
	stats := video.Statistics
	details := video.ContentDetails

	fmt.Printf("📹 %s\n", snippet.Title)
	fmt.Printf("   Video ID: %s\n", videoID)
	fmt.Printf("   Channel: %s\n", snippet.ChannelTitle)
	fmt.Printf("   Published: %s\n", snippet.PublishedAt)
	fmt.Printf("   Duration: %s\n", details.Duration)
	fmt.Printf("   Views: %d\n", stats.ViewCount)
	fmt.Printf("   Likes: %d\n", stats.LikeCount)
	if stats.CommentCount > 0 {
		fmt.Printf("   Comments: %d\n", stats.CommentCount)
	}
	fmt.Printf("   Link: https://www.youtube.com/watch?v=%s\n", videoID)

	desc := snippet.Description
	if len(desc) > 500 {
		desc = desc[:500] + "..."
	}
	fmt.Printf("\n   Description:\n   %s\n", desc)

	return nil
}

// createPlaylistCmd creates a new playlist
var (
	description string
	privacy     string
)

var createPlaylistCmd = createCreatePlaylistCmd()

func createCreatePlaylistCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-playlist <title>",
		Short: "Create a new playlist",
		Args:  cobra.ExactArgs(1),
		RunE:  runCreatePlaylist,
	}
	cmd.Flags().StringVar(&description, "description", "", "Playlist description")
	cmd.Flags().StringVar(&privacy, "privacy", "private", "Privacy status (private, public, unlisted)")
	return cmd
}

func runCreatePlaylist(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	title := args[0]

	service, err := getYouTubeService(ctx)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "📝 Creating playlist: %s...\n\n", title)

	playlist := &youtube.Playlist{
		Snippet: &youtube.PlaylistSnippet{
			Title:       title,
			Description: description,
		},
		Status: &youtube.PlaylistStatus{
			PrivacyStatus: privacy,
		},
	}

	call := service.Playlists.Insert([]string{"snippet", "status"}, playlist)
	response, err := call.Do()
	if err != nil {
		return fmt.Errorf("error creating playlist: %w", err)
	}

	fmt.Fprintf(os.Stderr, "✅ Playlist created successfully!\n")
	fmt.Printf("   ID: %s\n", response.Id)
	fmt.Printf("   Link: https://www.youtube.com/playlist?list=%s\n", response.Id)

	return nil
}

// deletePlaylistCmd deletes a playlist
var deletePlaylistCmd = &cobra.Command{
	Use:   "delete-playlist <playlist-id>",
	Short: "Delete a playlist",
	Args:  cobra.ExactArgs(1),
	RunE:  runDeletePlaylist,
}

func runDeletePlaylist(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	playlistID := args[0]

	service, err := getYouTubeService(ctx)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "🗑️  Deleting playlist: %s...\n\n", playlistID)

	call := service.Playlists.Delete(playlistID)
	if err := call.Do(); err != nil {
		return fmt.Errorf("error deleting playlist: %w", err)
	}

	fmt.Fprintf(os.Stderr, "✅ Playlist deleted successfully!\n")
	return nil
}

// addToPlaylistCmd adds a video to a playlist
var addToPlaylistCmd = &cobra.Command{
	Use:   "add-to-playlist <playlist-id> <video-id>",
	Short: "Add a video to a playlist",
	Args:  cobra.ExactArgs(2),
	RunE:  runAddToPlaylist,
}

func runAddToPlaylist(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	playlistID := args[0]
	videoID := args[1]

	service, err := getYouTubeService(ctx)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "➕ Adding video %s to playlist %s...\n\n", videoID, playlistID)

	playlistItem := &youtube.PlaylistItem{
		Snippet: &youtube.PlaylistItemSnippet{
			PlaylistId: playlistID,
			ResourceId: &youtube.ResourceId{
				Kind:    "youtube#video",
				VideoId: videoID,
			},
		},
	}

	call := service.PlaylistItems.Insert([]string{"snippet"}, playlistItem)
	if _, err := call.Do(); err != nil {
		return fmt.Errorf("error adding video to playlist: %w", err)
	}

	fmt.Fprintf(os.Stderr, "✅ Video added to playlist successfully!\n")
	return nil
}

// Helper functions

func printJSON(v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
