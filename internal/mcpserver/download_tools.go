package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"youtube-manager/internal/download"
)

func (s *Server) registerDownloadTools() {
	s.mcpServer.AddTool(
		mcp.NewTool("download_video",
			mcp.WithDescription("Download a YouTube video or extract audio. Supports time-based extraction and MP3 conversion."),
			mcp.WithString("url_or_video_id",
				mcp.Required(),
				mcp.Description("YouTube URL or video ID"),
			),
			mcp.WithString("output_dir",
				mcp.Description("Output directory (default: current directory)"),
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

func (s *Server) handleDownloadVideo(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	urlOrID, err := req.RequireString("url_or_video_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	outputDir := req.GetString("output_dir", ".")
	format := req.GetString("format", "best")
	audioOnly := req.GetBool("audio_only", false)
	extractFrom := req.GetInt("extract_from", 0)
	extractTo := req.GetInt("extract_to", 0)

	url := download.ResolveVideoURL(urlOrID)
	downloader := download.NewDownloader(outputDir, format, audioOnly, extractFrom, extractTo)

	result, err := downloader.DownloadWithResult(ctx, url)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("download failed: %v", err)), nil
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(data)), nil
}
