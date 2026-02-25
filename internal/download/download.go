// Package download handles native YouTube video downloads.
package download

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/kkdai/youtube/v2"
)

// DownloadResult contains metadata about a completed download.
type DownloadResult struct {
	FilePath string `json:"file_path"`
	Title    string `json:"title"`
	Author   string `json:"author"`
	Duration string `json:"duration"`
	Format   string `json:"format"`
	FileSize int64  `json:"file_size"`
	Cached   bool   `json:"cached"`
}

// Downloader handles video downloads.
type Downloader struct {
	outputDir   string
	format      string
	audioOnly   bool
	extractFrom int // seconds, 0 = from beginning
	extractTo   int // seconds, 0 = until end
}

// NewDownloader creates a new downloader with the specified options.
func NewDownloader(outputDir, format string, audioOnly bool, extractFrom, extractTo int) *Downloader {
	return &Downloader{
		outputDir:   outputDir,
		format:      format,
		audioOnly:   audioOnly,
		extractFrom: extractFrom,
		extractTo:   extractTo,
	}
}

// ResolveVideoURL returns a full YouTube URL from a URL or video ID.
func ResolveVideoURL(input string) string {
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		return input
	}
	return "https://www.youtube.com/watch?v=" + input
}

// Download downloads a video from the given URL.
func (d *Downloader) Download(ctx context.Context, url string) error {
	_, err := d.DownloadWithResult(ctx, url)
	return err
}

// DownloadWithResult downloads a video and returns metadata about the result.
func (d *Downloader) DownloadWithResult(ctx context.Context, url string) (*DownloadResult, error) {
	cache := NewCache()
	if err := cache.EnsureDir(); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}
	cache.CleanExpired()

	videoID := ExtractVideoID(url)
	client := &youtube.Client{}

	fmt.Fprintf(os.Stderr, "Fetching video info: %s\n", url)

	video, err := client.GetVideoContext(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to get video info: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Title:    %s\n", video.Title)
	fmt.Fprintf(os.Stderr, "Author:   %s\n", video.Author)
	fmt.Fprintf(os.Stderr, "Duration: %s\n", video.Duration)

	// Check cache
	cachedPath, cached := cache.GetCachedPath(videoID)
	if !cached {
		// Download to cache
		cachedPath, err = d.downloadToCache(ctx, client, video, cache, videoID)
		if err != nil {
			return nil, err
		}
	} else {
		fmt.Fprintf(os.Stderr, "Using cached file: %s\n", cachedPath)
	}

	// Determine output filename and processing
	opts := ProcessOpts{
		AudioOnly:   d.audioOnly,
		ExtractFrom: d.extractFrom,
		ExtractTo:   d.extractTo,
	}

	outputPath, err := d.buildOutputPath(video.Title, cachedPath, opts)
	if err != nil {
		return nil, err
	}

	if opts.NeedsProcessing() {
		// Check ffmpeg availability
		if err := CheckFFmpeg(); err != nil {
			return nil, err
		}

		fmt.Fprintf(os.Stderr, "Processing with ffmpeg...\n")
		if err := Process(ctx, cachedPath, outputPath, opts); err != nil {
			return nil, fmt.Errorf("post-processing failed: %w", err)
		}
	} else {
		// Copy cached file to output
		fmt.Fprintf(os.Stderr, "Copying to output...\n")
		if err := copyFile(cachedPath, outputPath); err != nil {
			return nil, fmt.Errorf("failed to copy cached file: %w", err)
		}
	}

	info, err := os.Stat(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat output file: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Output:   %s (%s)\n", outputPath, formatBytes(info.Size()))

	return &DownloadResult{
		FilePath: outputPath,
		Title:    video.Title,
		Author:   video.Author,
		Duration: video.Duration.String(),
		Format:   d.outputFormat(opts),
		FileSize: info.Size(),
		Cached:   cached,
	}, nil
}

// downloadToCache downloads the full video to the cache directory.
func (d *Downloader) downloadToCache(ctx context.Context, client *youtube.Client, video *youtube.Video, cache *Cache, videoID string) (string, error) {
	// Always download best quality with audio for cache
	format, ext := d.selectCacheFormat(video)
	if format == nil {
		return "", fmt.Errorf("no suitable format found")
	}

	cachedPath := cache.CachePath(videoID, ext)

	fmt.Fprintf(os.Stderr, "Format:   %s (%s)\n", format.QualityLabel, format.MimeType)
	fmt.Fprintf(os.Stderr, "Downloading to cache...\n")

	stream, size, err := client.GetStreamContext(ctx, video, format)
	if err != nil {
		return "", fmt.Errorf("failed to get stream: %w", err)
	}
	defer stream.Close()

	file, err := os.Create(cachedPath)
	if err != nil {
		return "", fmt.Errorf("failed to create cache file: %w", err)
	}
	defer file.Close()

	if size > 0 {
		fmt.Fprintf(os.Stderr, "Downloading... (%s)\n", formatBytes(size))
	}

	written, err := io.Copy(file, stream)
	if err != nil {
		os.Remove(cachedPath)
		return "", fmt.Errorf("download failed: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Downloaded: %s\n", formatBytes(written))
	return cachedPath, nil
}

// selectCacheFormat picks the best format with audio for caching.
func (d *Downloader) selectCacheFormat(video *youtube.Video) (*youtube.Format, string) {
	formats := video.Formats

	// Prefer formats with both video and audio
	combined := formats.Select(func(f youtube.Format) bool {
		return f.AudioChannels > 0 && f.QualityLabel != ""
	})

	if len(combined) > 0 {
		combined.Sort()
		f := &combined[0]
		return f, extensionFromMime(f.MimeType, "mp4")
	}

	// Fallback: any format with audio
	withAudio := formats.WithAudioChannels()
	if len(withAudio) > 0 {
		withAudio.Sort()
		f := &withAudio[0]
		return f, extensionFromMime(f.MimeType, "mp4")
	}

	return nil, ""
}

// buildOutputPath constructs the output file path based on title, options, and format.
func (d *Downloader) buildOutputPath(title, cachedPath string, opts ProcessOpts) (string, error) {
	base := sanitizeFilename(title)

	// Add time range suffix
	if opts.ExtractFrom > 0 || opts.ExtractTo > 0 {
		suffix := ""
		if opts.ExtractFrom > 0 {
			suffix += fmt.Sprintf("_%ds", opts.ExtractFrom)
		}
		if opts.ExtractTo > 0 {
			suffix += fmt.Sprintf("-%ds", opts.ExtractTo)
		}
		base += suffix
	}

	// Determine extension
	ext := filepath.Ext(cachedPath)
	if opts.AudioOnly {
		ext = ".mp3"
	}

	outputPath := filepath.Join(d.outputDir, base+ext)
	return outputPath, nil
}

// outputFormat returns a description of the output format.
func (d *Downloader) outputFormat(opts ProcessOpts) string {
	if opts.AudioOnly {
		return "mp3"
	}
	return d.format
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return out.Close()
}

// extensionFromMime extracts a file extension from a MIME type.
func extensionFromMime(mimeType, fallback string) string {
	switch {
	case strings.Contains(mimeType, "mp4"):
		return "mp4"
	case strings.Contains(mimeType, "webm"):
		return "webm"
	case strings.Contains(mimeType, "mp4a"), strings.Contains(mimeType, "m4a"):
		return "m4a"
	case strings.Contains(mimeType, "opus"):
		return "webm"
	default:
		return fallback
	}
}

// sanitizeFilename removes characters that are not safe for filenames.
func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "",
		"\"", "",
		"<", "",
		">", "",
		"|", "_",
	)
	return replacer.Replace(name)
}

// formatBytes formats a byte count as a human-readable string.
func formatBytes(bytes int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case bytes >= gb:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
