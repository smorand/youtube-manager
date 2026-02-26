package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/mark3labs/mcp-go/mcp"

	"youtube-manager/internal/download"
)

const signedURLExpiry = 10 * time.Minute

func (s *Server) registerDownloadTools() {
	s.mcpServer.AddTool(
		mcp.NewTool("download_video",
			mcp.WithDescription("Download a YouTube video or extract audio. Supports time-based extraction and MP3 conversion. Returns a signed download URL when running on Cloud Run."),
			mcp.WithString("url_or_video_id",
				mcp.Required(),
				mcp.Description("YouTube URL or video ID"),
			),
			mcp.WithString("output_dir",
				mcp.Description("Output directory (default: current directory). Ignored on Cloud Run."),
			),
			mcp.WithString("format",
				mcp.Description("Video format/quality (e.g., 720p, 1080p, best). Default: best"),
			),
			mcp.WithBoolean("audio_only",
				mcp.Description("Download audio only and convert to MP3 (default: false)"),
			),
			mcp.WithNumber("extract_from",
				mcp.Description("Start extraction at this second (default: 0, from beginning)"),
			),
			mcp.WithNumber("extract_to",
				mcp.Description("Stop extraction at this second (default: 0, until end)"),
			),
		),
		s.handleDownloadVideo,
	)
}

// downloadResponse extends DownloadResult with optional download URL for remote mode.
type downloadResponse struct {
	FilePath    string `json:"file_path,omitempty"`
	Title       string `json:"title"`
	Author      string `json:"author"`
	Duration    string `json:"duration"`
	Format      string `json:"format"`
	FileSize    int64  `json:"file_size"`
	Cached      bool   `json:"cached"`
	FileName    string `json:"file_name,omitempty"`
	DownloadURL string `json:"download_url,omitempty"`
}

func (s *Server) handleDownloadVideo(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	urlOrID, err := req.RequireString("url_or_video_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	bucket := os.Getenv("DOWNLOADS_BUCKET")
	remoteMode := bucket != ""

	outputDir := req.GetString("output_dir", ".")
	format := req.GetString("format", "best")
	audioOnly := req.GetBool("audio_only", false)
	extractFrom := req.GetInt("extract_from", 0)
	extractTo := req.GetInt("extract_to", 0)

	// On Cloud Run, use a server-local temp directory instead of client path
	if remoteMode {
		tmpDir, err := os.MkdirTemp("", "ytm-download-*")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to create temp dir: %v", err)), nil
		}
		defer os.RemoveAll(tmpDir)
		outputDir = tmpDir
	}

	url := download.ResolveVideoURL(urlOrID)
	downloader := download.NewDownloader(outputDir, format, audioOnly, extractFrom, extractTo)

	result, err := downloader.DownloadWithResult(ctx, url)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("download failed: %v", err)), nil
	}

	resp := downloadResponse{
		FilePath: result.FilePath,
		Title:    result.Title,
		Author:   result.Author,
		Duration: result.Duration,
		Format:   result.Format,
		FileSize: result.FileSize,
		Cached:   result.Cached,
	}

	// On Cloud Run, upload to GCS and return a signed URL
	if remoteMode {
		signedURL, err := uploadAndSign(ctx, bucket, result.FilePath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to upload to GCS: %v", err)), nil
		}
		resp.FileName = filepath.Base(result.FilePath)
		resp.DownloadURL = signedURL
		resp.FilePath = "" // Server path is meaningless to the client
	}

	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(data)), nil
}

// uploadAndSign uploads a file to GCS and returns a signed URL for download.
func uploadAndSign(ctx context.Context, bucket, filePath string) (string, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to create GCS client: %w", err)
	}
	defer client.Close()

	filename := filepath.Base(filePath)
	objectName := fmt.Sprintf("downloads/%d/%s", time.Now().UnixMilli(), filename)

	// Upload file to GCS
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	writer := client.Bucket(bucket).Object(objectName).NewWriter(ctx)
	writer.ContentType = mimeTypeFromExt(filepath.Ext(filePath))
	writer.ContentDisposition = fmt.Sprintf(`attachment; filename="%s"`, filename)

	if _, err := f.WriteTo(writer); err != nil {
		writer.Close()
		return "", fmt.Errorf("failed to write to GCS: %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to close GCS writer: %w", err)
	}

	// Generate signed URL
	signedURL, err := client.Bucket(bucket).SignedURL(objectName, &storage.SignedURLOptions{
		Method:  "GET",
		Expires: time.Now().Add(signedURLExpiry),
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate signed URL: %w", err)
	}

	return signedURL, nil
}

// mimeTypeFromExt returns a MIME type based on file extension.
func mimeTypeFromExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".mp3":
		return "audio/mpeg"
	case ".mp4":
		return "video/mp4"
	case ".m4a":
		return "audio/mp4"
	case ".webm":
		return "video/webm"
	case ".mkv":
		return "video/x-matroska"
	default:
		return "application/octet-stream"
	}
}
