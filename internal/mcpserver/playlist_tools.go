package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

type playlistResponse struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	VideoCount  int64  `json:"video_count"`
	URL         string `json:"url"`
}

type playlistItemResponse struct {
	Position int    `json:"position"`
	VideoID  string `json:"video_id"`
	Title    string `json:"title"`
	Channel  string `json:"channel"`
	URL      string `json:"url"`
}

func (s *Server) registerPlaylistTools() {
	// list_playlists
	s.mcpServer.AddTool(
		mcp.NewTool("list_playlists",
			mcp.WithDescription("List your YouTube playlists"),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of playlists to return (default: 50)"),
			),
		),
		s.handleListPlaylists,
	)

	// get_playlist
	s.mcpServer.AddTool(
		mcp.NewTool("get_playlist",
			mcp.WithDescription("Get videos from a playlist"),
			mcp.WithString("playlist_id",
				mcp.Required(),
				mcp.Description("The playlist ID"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of videos to return (default: 50)"),
			),
		),
		s.handleGetPlaylist,
	)

	// create_playlist
	s.mcpServer.AddTool(
		mcp.NewTool("create_playlist",
			mcp.WithDescription("Create a new YouTube playlist"),
			mcp.WithString("title",
				mcp.Required(),
				mcp.Description("Playlist title"),
			),
			mcp.WithString("description",
				mcp.Description("Playlist description"),
			),
			mcp.WithString("privacy",
				mcp.Enum("private", "public", "unlisted"),
				mcp.Description("Privacy status (default: private)"),
			),
		),
		s.handleCreatePlaylist,
	)

	// delete_playlist
	s.mcpServer.AddTool(
		mcp.NewTool("delete_playlist",
			mcp.WithDescription("Delete a YouTube playlist"),
			mcp.WithString("playlist_id",
				mcp.Required(),
				mcp.Description("The playlist ID to delete"),
			),
		),
		s.handleDeletePlaylist,
	)

	// add_to_playlist
	s.mcpServer.AddTool(
		mcp.NewTool("add_to_playlist",
			mcp.WithDescription("Add a video to a playlist"),
			mcp.WithString("playlist_id",
				mcp.Required(),
				mcp.Description("The playlist ID"),
			),
			mcp.WithString("video_id",
				mcp.Required(),
				mcp.Description("The video ID to add"),
			),
		),
		s.handleAddToPlaylist,
	)
}

func (s *Server) handleListPlaylists(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	limit := req.GetInt("limit", 50)

	playlists, err := s.playlistSvc.List(ctx, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list playlists: %v", err)), nil
	}

	results := make([]playlistResponse, 0, len(playlists))
	for _, p := range playlists {
		results = append(results, playlistResponse{
			ID:          p.Id,
			Title:       p.Snippet.Title,
			Description: p.Snippet.Description,
			VideoCount:  p.ContentDetails.ItemCount,
			URL:         "https://www.youtube.com/playlist?list=" + p.Id,
		})
	}

	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(data)), nil
}

func (s *Server) handleGetPlaylist(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	playlistID, err := req.RequireString("playlist_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	limit := req.GetInt("limit", 50)

	items, err := s.playlistSvc.GetItems(ctx, playlistID, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get playlist items: %v", err)), nil
	}

	results := make([]playlistItemResponse, 0, len(items))
	for i, item := range items {
		results = append(results, playlistItemResponse{
			Position: i + 1,
			VideoID:  item.ContentDetails.VideoId,
			Title:    item.Snippet.Title,
			Channel:  item.Snippet.ChannelTitle,
			URL:      "https://www.youtube.com/watch?v=" + item.ContentDetails.VideoId,
		})
	}

	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(data)), nil
}

func (s *Server) handleCreatePlaylist(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	title, err := req.RequireString("title")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	description := req.GetString("description", "")
	privacy := req.GetString("privacy", "private")

	playlist, err := s.playlistSvc.Create(ctx, title, description, privacy)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create playlist: %v", err)), nil
	}

	result := playlistResponse{
		ID:          playlist.Id,
		Title:       playlist.Snippet.Title,
		Description: playlist.Snippet.Description,
		VideoCount:  0,
		URL:         "https://www.youtube.com/playlist?list=" + playlist.Id,
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(data)), nil
}

func (s *Server) handleDeletePlaylist(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	playlistID, err := req.RequireString("playlist_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := s.playlistSvc.Delete(ctx, playlistID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to delete playlist: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf(`{"status":"deleted","playlist_id":"%s"}`, playlistID)), nil
}

func (s *Server) handleAddToPlaylist(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	playlistID, err := req.RequireString("playlist_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	videoID, err := req.RequireString("video_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := s.playlistSvc.AddVideo(ctx, playlistID, videoID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to add video to playlist: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf(`{"status":"added","playlist_id":"%s","video_id":"%s"}`, playlistID, videoID)), nil
}
