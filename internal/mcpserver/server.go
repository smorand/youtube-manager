// Package mcpserver exposes youtube-manager operations as MCP tools.
package mcpserver

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/server"

	"youtube-manager/internal/auth"
	"youtube-manager/internal/youtube"
)

// Server wraps the MCP server with YouTube services.
type Server struct {
	mcpServer   *server.MCPServer
	playlistSvc *youtube.PlaylistService
	videoSvc    *youtube.VideoService
}

// NewServer creates a new MCP server with all tools registered.
// Auth is initialized eagerly before stdio starts.
func NewServer(ctx context.Context) (*Server, error) {
	authClient, err := auth.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create auth client: %w", err)
	}

	ytService, err := authClient.GetYouTubeService(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get YouTube service: %w", err)
	}

	s := &Server{
		mcpServer: server.NewMCPServer(
			"youtube-manager",
			"1.0.0",
			server.WithToolCapabilities(false),
		),
		playlistSvc: youtube.NewPlaylistService(ytService),
		videoSvc:    youtube.NewVideoService(ytService),
	}

	s.registerPlaylistTools()
	s.registerVideoTools()
	s.registerDownloadTools()

	return s, nil
}

// MCPServer returns the underlying MCP server for transport binding.
func (s *Server) MCPServer() *server.MCPServer {
	return s.mcpServer
}
