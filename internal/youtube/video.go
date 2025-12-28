package youtube

import (
	"context"
	"fmt"

	"google.golang.org/api/youtube/v3"
)

// VideoService handles video operations.
type VideoService struct {
	service *youtube.Service
}

// NewVideoService creates a new video service.
func NewVideoService(service *youtube.Service) *VideoService {
	return &VideoService{service: service}
}

// Get retrieves video information.
func (vs *VideoService) Get(ctx context.Context, videoID string) (*youtube.Video, error) {
	call := vs.service.Videos.List([]string{"snippet", "contentDetails", "statistics"}).Id(videoID)
	response, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("error fetching video: %w", err)
	}

	if len(response.Items) == 0 {
		return nil, fmt.Errorf("video not found: %s", videoID)
	}

	return response.Items[0], nil
}

// Search searches for videos.
func (vs *VideoService) Search(ctx context.Context, query string, limit int) ([]*youtube.SearchResult, error) {
	call := vs.service.Search.List([]string{"snippet"}).
		Q(query).
		Type("video").
		MaxResults(int64(limit))

	response, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("error searching videos: %w", err)
	}

	return response.Items, nil
}

// PrintVideo prints video information to stdout.
func PrintVideo(video *youtube.Video) {
	snippet := video.Snippet
	stats := video.Statistics
	details := video.ContentDetails

	fmt.Printf("📹 %s\n", snippet.Title)
	fmt.Printf("   Video ID: %s\n", video.Id)
	fmt.Printf("   Channel: %s\n", snippet.ChannelTitle)
	fmt.Printf("   Published: %s\n", snippet.PublishedAt)
	fmt.Printf("   Duration: %s\n", details.Duration)
	fmt.Printf("   Views: %d\n", stats.ViewCount)
	fmt.Printf("   Likes: %d\n", stats.LikeCount)
	if stats.CommentCount > 0 {
		fmt.Printf("   Comments: %d\n", stats.CommentCount)
	}
	fmt.Printf("   Link: https://www.youtube.com/watch?v=%s\n", video.Id)

	desc := snippet.Description
	if len(desc) > 500 {
		desc = desc[:500] + "..."
	}
	fmt.Printf("\n   Description:\n   %s\n", desc)
}

// PrintSearchResults prints search results to stdout.
func PrintSearchResults(results []*youtube.SearchResult) {
	if len(results) == 0 {
		fmt.Println("No videos found.")
		return
	}

	fmt.Printf("✅ Found %d video(s):\n\n", len(results))
	for idx, item := range results {
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
}
