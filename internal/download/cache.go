// Package download provides cache management for downloaded videos.
package download

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	cacheDir        = "/tmp/youtube-manager-cache"
	cacheExpiration = 24 * time.Hour
)

// Cache manages downloaded video file caching.
type Cache struct {
	dir string
}

// NewCache creates a new cache instance.
func NewCache() *Cache {
	return &Cache{dir: cacheDir}
}

// EnsureDir creates the cache directory if it doesn't exist.
func (c *Cache) EnsureDir() error {
	return os.MkdirAll(c.dir, 0755)
}

// GetCachedPath returns the path to a cached file and whether it exists and is valid.
func (c *Cache) GetCachedPath(videoID string) (string, bool) {
	pattern := filepath.Join(c.dir, videoID+".*")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return "", false
	}

	path := matches[0]
	info, err := os.Stat(path)
	if err != nil {
		return "", false
	}

	if time.Since(info.ModTime()) > cacheExpiration {
		os.Remove(path)
		return "", false
	}

	return path, true
}

// CachePath returns the expected cache file path for a video ID with the given extension.
func (c *Cache) CachePath(videoID, ext string) string {
	return filepath.Join(c.dir, videoID+"."+ext)
}

// CleanExpired removes cached files older than the expiration period.
func (c *Cache) CleanExpired() error {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read cache directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if time.Since(info.ModTime()) > cacheExpiration {
			os.Remove(filepath.Join(c.dir, entry.Name()))
		}
	}

	return nil
}

// ExtractVideoID extracts a video ID from a YouTube URL or returns the input as-is.
func ExtractVideoID(input string) string {
	// Try parsing as URL
	u, err := url.Parse(input)
	if err != nil {
		return input
	}

	// Handle youtube.com/watch?v=ID
	if strings.Contains(u.Host, "youtube.com") {
		v := u.Query().Get("v")
		if v != "" {
			return v
		}
	}

	// Handle youtu.be/ID
	if strings.Contains(u.Host, "youtu.be") {
		id := strings.TrimPrefix(u.Path, "/")
		if id != "" {
			return id
		}
	}

	// Not a URL, return as-is (assumed to be a video ID)
	return input
}
