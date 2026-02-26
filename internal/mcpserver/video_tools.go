package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

type videoResponse struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Channel     string `json:"channel"`
	PublishedAt string `json:"published_at"`
	Duration    string `json:"duration"`
	Views       uint64 `json:"views"`
	Likes       uint64 `json:"likes"`
	Comments    uint64 `json:"comments"`
	Description string `json:"description"`
	URL         string `json:"url"`
}

type searchResultResponse struct {
	VideoID     string `json:"video_id"`
	Title       string `json:"title"`
	Channel     string `json:"channel"`
	Description string `json:"description"`
	URL         string `json:"url"`
}

func (s *Server) registerVideoTools() {
	// search_videos
	s.mcpServer.AddTool(
		mcp.NewTool("search_videos",
			mcp.WithDescription("Search for videos on YouTube"),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Search query"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results (default: 10)"),
			),
		),
		s.handleSearchVideos,
	)

	// get_video
	s.mcpServer.AddTool(
		mcp.NewTool("get_video",
			mcp.WithDescription("Get detailed information about a video"),
			mcp.WithString("video_id",
				mcp.Required(),
				mcp.Description("The video ID"),
			),
		),
		s.handleGetVideo,
	)
}

func (s *Server) handleSearchVideos(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := req.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	limit := req.GetInt("limit", 10)

	svc, err := s.getVideoSvc(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get YouTube service: %v", err)), nil
	}

	results, err := svc.Search(ctx, query, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to search videos: %v", err)), nil
	}

	response := make([]searchResultResponse, 0, len(results))
	for _, item := range results {
		response = append(response, searchResultResponse{
			VideoID:     item.Id.VideoId,
			Title:       item.Snippet.Title,
			Channel:     item.Snippet.ChannelTitle,
			Description: item.Snippet.Description,
			URL:         "https://www.youtube.com/watch?v=" + item.Id.VideoId,
		})
	}

	data, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(data)), nil
}

func (s *Server) handleGetVideo(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	videoID, err := req.RequireString("video_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	svc, err := s.getVideoSvc(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get YouTube service: %v", err)), nil
	}

	video, err := svc.Get(ctx, videoID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get video: %v", err)), nil
	}

	result := videoResponse{
		ID:          video.Id,
		Title:       video.Snippet.Title,
		Channel:     video.Snippet.ChannelTitle,
		PublishedAt: video.Snippet.PublishedAt,
		Duration:    video.ContentDetails.Duration,
		Views:       video.Statistics.ViewCount,
		Likes:       video.Statistics.LikeCount,
		Comments:    video.Statistics.CommentCount,
		Description: video.Snippet.Description,
		URL:         "https://www.youtube.com/watch?v=" + video.Id,
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(data)), nil
}
