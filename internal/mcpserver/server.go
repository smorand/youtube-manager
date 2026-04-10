// Package mcpserver exposes youtube-manager operations as MCP tools.
package mcpserver

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/mark3labs/mcp-go/server"

	"youtube-manager/internal/auth"
	"youtube-manager/internal/youtube"
)

// Config holds the MCP server configuration.
type Config struct {
	// HTTP mode settings
	Host    string
	Port    int
	BaseURL string // Base URL for OAuth callbacks (e.g., https://ytm.mcp.scm-platform.org)

	// Secret Manager settings
	SecretProject string // GCP project for Secret Manager
	SecretName    string // Secret name for OAuth credentials

	// Vault settings
	VaultAddr       string // HashiCorp Vault address
	VaultSecretPath string // Vault KV v2 secret path
	VaultToken      string // Vault authentication token

	// File-based settings (fallback)
	CredentialFile string
}

// Server wraps the MCP server with YouTube services.
type Server struct {
	config       *Config
	mcpServer    *server.MCPServer
	oauth2Server *OAuth2Server

	// Pre-initialized services (stdio mode only, nil in HTTP mode)
	playlistSvc *youtube.PlaylistService
	videoSvc    *youtube.VideoService
}

// NewServer creates a new MCP server with all tools registered.
// In stdio mode (config nil or no BaseURL), auth is initialized eagerly.
// In HTTP mode (BaseURL set), auth is deferred to per-request OAuth2 flow.
func NewServer(ctx context.Context, config *Config) (*Server, error) {
	s := &Server{
		config: config,
		mcpServer: server.NewMCPServer(
			"youtube-manager",
			"1.0.0",
			server.WithToolCapabilities(false),
		),
	}

	// In stdio mode, initialize auth eagerly
	isHTTPMode := config != nil && config.BaseURL != ""
	if !isHTTPMode {
		authClient, err := auth.NewClient()
		if err != nil {
			return nil, fmt.Errorf("failed to create auth client: %w", err)
		}

		ytService, err := authClient.GetYouTubeService(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get YouTube service: %w", err)
		}

		s.playlistSvc = youtube.NewPlaylistService(ytService)
		s.videoSvc = youtube.NewVideoService(ytService)
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

// getPlaylistSvc returns the playlist service, creating one from context if needed (HTTP mode).
func (s *Server) getPlaylistSvc(ctx context.Context) (*youtube.PlaylistService, error) {
	if s.playlistSvc != nil {
		return s.playlistSvc, nil
	}
	ytService, err := auth.GetYouTubeService(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get YouTube service: %w", err)
	}
	return youtube.NewPlaylistService(ytService), nil
}

// getVideoSvc returns the video service, creating one from context if needed (HTTP mode).
func (s *Server) getVideoSvc(ctx context.Context) (*youtube.VideoService, error) {
	if s.videoSvc != nil {
		return s.videoSvc, nil
	}
	ytService, err := auth.GetYouTubeService(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get YouTube service: %w", err)
	}
	return youtube.NewVideoService(ytService), nil
}

// extractBearerToken extracts the token from the Authorization header.
func extractBearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return ""
	}

	return strings.TrimPrefix(authHeader, bearerPrefix)
}

// authMiddleware wraps an HTTP handler with OAuth2 Bearer token authentication.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		accessToken := extractBearerToken(r)

		if accessToken == "" {
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(
				`Bearer resource_metadata="%s/.well-known/oauth-protected-resource"`,
				s.config.BaseURL,
			))
			http.Error(w, "Unauthorized: Bearer token required", http.StatusUnauthorized)
			return
		}

		if s.oauth2Server == nil {
			http.Error(w, "OAuth not configured", http.StatusInternalServerError)
			return
		}

		oauthConfig, token, err := s.oauth2Server.ValidateAccessToken(ctx, accessToken)
		if err != nil {
			slog.Error("Token validation error", "error", err)
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(
				`Bearer error="invalid_token", resource_metadata="%s/.well-known/oauth-protected-resource"`,
				s.config.BaseURL,
			))
			http.Error(w, "Unauthorized: invalid token", http.StatusUnauthorized)
			return
		}

		// Inject OAuth config and token into context for YouTube API calls
		ctx = auth.WithOAuthConfig(ctx, oauthConfig)
		ctx = auth.WithAccessToken(ctx, token)

		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}

// Run starts the HTTP server with OAuth2 and blocks until shutdown.
func (s *Server) Run(ctx context.Context) error {
	// Create the streamable HTTP handler for MCP
	mcpHandler := server.NewStreamableHTTPServer(s.mcpServer)

	mux := http.NewServeMux()

	// Determine credential file path
	credFile := ""
	if s.config != nil {
		credFile = s.config.CredentialFile
	}
	if credFile == "" {
		credFile = auth.GetCredentialsPath() + "/" + auth.CredentialsFile
	}

	// Initialize OAuth2 server
	s.oauth2Server = NewOAuth2Server(&OAuth2ServerConfig{
		BaseURL:         s.config.BaseURL,
		SecretProject:   s.config.SecretProject,
		SecretName:      s.config.SecretName,
		CredentialFile:  credFile,
		VaultAddr:       s.config.VaultAddr,
		VaultSecretPath: s.config.VaultSecretPath,
		VaultToken:      s.config.VaultToken,
	})

	// Register OAuth2 routes (not protected by auth)
	s.oauth2Server.SetupRoutes(mux)
	slog.Info("OAuth2 endpoints enabled",
		"authorize", s.config.BaseURL+"/oauth/authorize",
		"callback", s.config.BaseURL+"/oauth/callback",
		"token", s.config.BaseURL+"/oauth/token",
	)

	// Health check (not protected)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	})

	// MCP endpoint (protected by OAuth2 Bearer token auth)
	mux.Handle("/mcp", s.authMiddleware(mcpHandler))

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Setup graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	errChan := make(chan error, 1)
	go func() {
		slog.Info("Starting MCP server", "addr", addr, "base_url", s.config.BaseURL)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case err := <-errChan:
		return fmt.Errorf("server error: %w", err)
	case sig := <-shutdown:
		slog.Info("Received signal, shutting down", "signal", sig)
	case <-ctx.Done():
		slog.Info("Context cancelled, shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown error: %w", err)
	}

	slog.Info("MCP server stopped")
	return nil
}
