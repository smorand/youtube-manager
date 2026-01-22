// Package auth handles YouTube API authentication using OAuth 2.0.
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

const (
	credentialsFile = "scm-pwd-web.json"
	tokenFile       = "youtube-token.json"
	callbackPort    = 8000
	callbackPath    = "/oauth2callback"
)

var scopes = []string{
	youtube.YoutubeReadonlyScope,
	youtube.YoutubeForceSslScope,
}

// Client manages YouTube API authentication and provides authenticated clients.
type Client struct {
	credentialsPath string
	tokenPath       string
}

// NewClient creates a new auth client with default credentials paths.
func NewClient() (*Client, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	credDir := filepath.Join(home, ".credentials")
	return &Client{
		credentialsPath: filepath.Join(credDir, credentialsFile),
		tokenPath:       filepath.Join(credDir, tokenFile),
	}, nil
}

// GetYouTubeService returns an authenticated YouTube service.
func (c *Client) GetYouTubeService(ctx context.Context) (*youtube.Service, error) {
	httpClient, err := c.getHTTPClient(ctx)
	if err != nil {
		return nil, err
	}

	service, err := youtube.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("unable to create YouTube service: %w", err)
	}

	return service, nil
}

// getHTTPClient returns an authenticated HTTP client.
func (c *Client) getHTTPClient(ctx context.Context) (*http.Client, error) {
	credentials, err := os.ReadFile(c.credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read credentials file %s: %w\nSee README.md for setup instructions", c.credentialsPath, err)
	}

	config, err := google.ConfigFromJSON(credentials, scopes...)
	if err != nil {
		return nil, fmt.Errorf("unable to parse credentials: %w", err)
	}

	token, err := c.tokenFromFile()
	if err != nil {
		token, err = c.getTokenFromWeb(config)
		if err != nil {
			return nil, err
		}
		if err := c.saveToken(token); err != nil {
			slog.Warn("Unable to save token", "error", err)
		}
	}

	return config.Client(ctx, token), nil
}

// getTokenFromWeb initiates OAuth flow using a local web server to capture the callback.
func (c *Client) getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	// Set the redirect URL to our local callback server
	redirectURL := fmt.Sprintf("http://localhost:%d%s", callbackPort, callbackPath)
	config.RedirectURL = redirectURL

	// Channel to receive the authorization code
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	// Create the callback handler
	mux := http.NewServeMux()
	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errMsg := r.URL.Query().Get("error")
			if errMsg == "" {
				errMsg = "no authorization code received"
			}
			errChan <- fmt.Errorf("authorization failed: %s", errMsg)
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<!DOCTYPE html><html><body>
				<h1>Authorization Failed</h1>
				<p>%s</p>
				<p>You can close this window.</p>
			</body></html>`, errMsg)
			return
		}

		codeChan <- code
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html><html><body>
			<h1>Authorization Successful!</h1>
			<p>You can close this window and return to the terminal.</p>
			<script>window.close();</script>
		</body></html>`)
	})

	// Start the local server
	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", callbackPort),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("failed to start callback server: %w", err)
		}
	}()

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)

	// Generate the auth URL and open it in the browser
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Opening browser for authorization...\n")
	fmt.Printf("If the browser doesn't open automatically, visit:\n%s\n\n", authURL)

	if err := openBrowser(authURL); err != nil {
		slog.Warn("Failed to open browser automatically", "error", err)
	}

	// Wait for the authorization code or error
	var authCode string
	select {
	case authCode = <-codeChan:
		// Success
	case err := <-errChan:
		shutdownServer(server)
		return nil, err
	case <-time.After(5 * time.Minute):
		shutdownServer(server)
		return nil, fmt.Errorf("authorization timed out after 5 minutes")
	}

	// Shutdown the server
	shutdownServer(server)

	// Exchange the authorization code for a token
	token, err := config.Exchange(context.Background(), authCode)
	if err != nil {
		return nil, fmt.Errorf("unable to exchange authorization code: %w", err)
	}

	fmt.Println("Authorization successful!")
	return token, nil
}

// openBrowser opens the specified URL in the default browser.
func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
}

// shutdownServer gracefully shuts down the HTTP server.
func shutdownServer(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		slog.Warn("Error shutting down callback server", "error", err)
	}
}

// tokenFromFile loads a token from the token file.
func (c *Client) tokenFromFile() (*oauth2.Token, error) {
	file, err := os.Open(c.tokenPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	token := &oauth2.Token{}
	if err := json.NewDecoder(file).Decode(token); err != nil {
		return nil, err
	}

	return token, nil
}

// saveToken saves a token to the token file.
func (c *Client) saveToken(token *oauth2.Token) error {
	slog.Info("Saving credentials", "path", c.tokenPath)

	if err := os.MkdirAll(filepath.Dir(c.tokenPath), 0700); err != nil {
		return fmt.Errorf("failed to create credentials directory: %w", err)
	}

	file, err := os.OpenFile(c.tokenPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create token file: %w", err)
	}
	defer file.Close()

	if err := json.NewEncoder(file).Encode(token); err != nil {
		return fmt.Errorf("failed to encode token: %w", err)
	}

	return nil
}
