// Package youtube provides YouTube-specific functionality.
package youtube

import (
	"context"
	"fmt"
	"os"

	"google.golang.org/api/youtube/v3"
)

// PlaylistService handles playlist operations.
type PlaylistService struct {
	service *youtube.Service
}

// NewPlaylistService creates a new playlist service.
func NewPlaylistService(service *youtube.Service) *PlaylistService {
	return &PlaylistService{service: service}
}

// List retrieves user's playlists.
func (ps *PlaylistService) List(ctx context.Context, limit int) ([]*youtube.Playlist, error) {
	call := ps.service.Playlists.List([]string{"snippet", "contentDetails"}).
		Mine(true).
		MaxResults(int64(limit))

	response, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("error fetching playlists: %w", err)
	}

	return response.Items, nil
}

// GetItems retrieves videos from a playlist.
func (ps *PlaylistService) GetItems(ctx context.Context, playlistID string, limit int) ([]*youtube.PlaylistItem, error) {
	var allVideos []*youtube.PlaylistItem
	pageToken := ""

	for {
		call := ps.service.PlaylistItems.List([]string{"snippet", "contentDetails"}).
			PlaylistId(playlistID).
			MaxResults(int64(limit))

		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		response, err := call.Do()
		if err != nil {
			return nil, fmt.Errorf("error fetching playlist items: %w", err)
		}

		allVideos = append(allVideos, response.Items...)

		if response.NextPageToken == "" {
			break
		}
		pageToken = response.NextPageToken
	}

	return allVideos, nil
}

// Create creates a new playlist.
func (ps *PlaylistService) Create(ctx context.Context, title, description, privacy string) (*youtube.Playlist, error) {
	playlist := &youtube.Playlist{
		Snippet: &youtube.PlaylistSnippet{
			Title:       title,
			Description: description,
		},
		Status: &youtube.PlaylistStatus{
			PrivacyStatus: privacy,
		},
	}

	call := ps.service.Playlists.Insert([]string{"snippet", "status"}, playlist)
	response, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("error creating playlist: %w", err)
	}

	return response, nil
}

// Delete deletes a playlist.
func (ps *PlaylistService) Delete(ctx context.Context, playlistID string) error {
	call := ps.service.Playlists.Delete(playlistID)
	if err := call.Do(); err != nil {
		return fmt.Errorf("error deleting playlist: %w", err)
	}

	return nil
}

// AddVideo adds a video to a playlist.
func (ps *PlaylistService) AddVideo(ctx context.Context, playlistID, videoID string) error {
	playlistItem := &youtube.PlaylistItem{
		Snippet: &youtube.PlaylistItemSnippet{
			PlaylistId: playlistID,
			ResourceId: &youtube.ResourceId{
				Kind:    "youtube#video",
				VideoId: videoID,
			},
		},
	}

	call := ps.service.PlaylistItems.Insert([]string{"snippet"}, playlistItem)
	if _, err := call.Do(); err != nil {
		return fmt.Errorf("error adding video to playlist: %w", err)
	}

	return nil
}

// PrintPlaylists prints playlists to stdout.
func PrintPlaylists(playlists []*youtube.Playlist) {
	if len(playlists) == 0 {
		fmt.Println("No playlists found.")
		return
	}

	fmt.Fprintf(os.Stderr, "✅ Found %d playlist(s):\n\n", len(playlists))
	for _, playlist := range playlists {
		fmt.Printf("📁 %s\n", playlist.Snippet.Title)
		fmt.Printf("   ID: %s\n", playlist.Id)
		fmt.Printf("   Videos: %d\n", playlist.ContentDetails.ItemCount)
		fmt.Printf("   Link: https://www.youtube.com/playlist?list=%s\n\n", playlist.Id)
	}
}

// PrintPlaylistItems prints playlist items to stdout.
func PrintPlaylistItems(items []*youtube.PlaylistItem) {
	if len(items) == 0 {
		fmt.Println("No videos found in this playlist.")
		return
	}

	fmt.Fprintf(os.Stderr, "✅ Found %d video(s):\n\n", len(items))
	for idx, video := range items {
		fmt.Printf("%d. %s\n", idx+1, video.Snippet.Title)
		fmt.Printf("   Video ID: %s\n", video.ContentDetails.VideoId)
		fmt.Printf("   Channel: %s\n", video.Snippet.ChannelTitle)
		fmt.Printf("   Link: https://www.youtube.com/watch?v=%s\n\n", video.ContentDetails.VideoId)
	}
}
