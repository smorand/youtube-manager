// Package auth handles YouTube API authentication using OAuth 2.0.
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

const (
	credentialsFile = "google_credentials.json"
	tokenFile       = "youtube_token.json"
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

// getTokenFromWeb initiates OAuth flow and returns a token.
func (c *Client) getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser:\n%v\n\n", authURL)
	fmt.Printf("Enter authorization code: ")

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		return nil, fmt.Errorf("unable to read authorization code: %w", err)
	}

	token, err := config.Exchange(context.Background(), authCode)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve token from web: %w", err)
	}

	return token, nil
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
