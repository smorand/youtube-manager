// Package download handles YouTube video downloads using yt-dlp.
package download

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	secretmanagerpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
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

// ytdlpInfo holds metadata extracted from yt-dlp --dump-json.
type ytdlpInfo struct {
	Title    string  `json:"title"`
	Uploader string  `json:"uploader"`
	Duration float64 `json:"duration"`
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

// CheckYtdlp verifies that yt-dlp is installed and accessible.
func CheckYtdlp() error {
	_, err := exec.LookPath("yt-dlp")
	if err != nil {
		return fmt.Errorf("yt-dlp not found in PATH: install it with 'brew install yt-dlp' or visit https://github.com/yt-dlp/yt-dlp")
	}
	return nil
}

// Download downloads a video from the given URL.
func (d *Downloader) Download(ctx context.Context, url string) error {
	_, err := d.DownloadWithResult(ctx, url)
	return err
}

// DownloadWithResult downloads a video and returns metadata about the result.
func (d *Downloader) DownloadWithResult(ctx context.Context, url string) (*DownloadResult, error) {
	if err := CheckYtdlp(); err != nil {
		return nil, err
	}

	cache := NewCache()
	if err := cache.EnsureDir(); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}
	cache.CleanExpired()

	videoID := ExtractVideoID(url)

	// Fetch video info
	fmt.Fprintf(os.Stderr, "Fetching video info: %s\n", url)
	info, err := d.getVideoInfo(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to get video info: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Title:    %s\n", info.Title)
	fmt.Fprintf(os.Stderr, "Author:   %s\n", info.Uploader)
	fmt.Fprintf(os.Stderr, "Duration: %s\n", formatDuration(info.Duration))

	// Check cache
	cachedPath, cached := cache.GetCachedPath(videoID)
	if !cached {
		cachedPath, err = d.downloadToCache(ctx, url, videoID, cache)
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

	outputPath, err := d.buildOutputPath(info.Title, cachedPath, opts)
	if err != nil {
		return nil, err
	}

	if opts.NeedsProcessing() {
		if err := CheckFFmpeg(); err != nil {
			return nil, err
		}

		fmt.Fprintf(os.Stderr, "Processing with ffmpeg...\n")
		if err := Process(ctx, cachedPath, outputPath, opts); err != nil {
			return nil, fmt.Errorf("post-processing failed: %w", err)
		}
	} else {
		fmt.Fprintf(os.Stderr, "Copying to output...\n")
		if err := copyFile(cachedPath, outputPath); err != nil {
			return nil, fmt.Errorf("failed to copy cached file: %w", err)
		}
	}

	fileInfo, err := os.Stat(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat output file: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Output:   %s (%s)\n", outputPath, formatBytes(fileInfo.Size()))

	return &DownloadResult{
		FilePath: outputPath,
		Title:    info.Title,
		Author:   info.Uploader,
		Duration: formatDuration(info.Duration),
		Format:   d.outputFormat(opts),
		FileSize: fileInfo.Size(),
		Cached:   cached,
	}, nil
}

// cookiesPath caches the path to the cookies file written from Secret Manager.
var (
	cookiesOnce sync.Once
	cookiesFile string
)

// ensureCookiesFile loads YouTube cookies from Secret Manager and writes them
// to a temp file. Returns the path or empty string if unavailable.
func ensureCookiesFile() string {
	cookiesOnce.Do(func() {
		project := os.Getenv("SECRET_PROJECT")
		if project == "" {
			return
		}
		name := fmt.Sprintf("projects/%s/secrets/scm-pwd-ytm-youtube-cookies/versions/latest", project)

		ctx := context.Background()
		client, err := secretmanager.NewClient(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to create secret manager client: %v\n", err)
			return
		}
		defer client.Close()

		resp, err := client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{Name: name})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to load cookies secret: %v\n", err)
			return
		}

		f, err := os.CreateTemp("", "yt-cookies-*.txt")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to create cookies temp file: %v\n", err)
			return
		}
		if _, err := f.Write(resp.Payload.Data); err != nil {
			f.Close()
			return
		}
		f.Close()
		cookiesFile = f.Name()
		fmt.Fprintf(os.Stderr, "Loaded YouTube cookies from Secret Manager\n")
	})
	return cookiesFile
}

// envArgs returns yt-dlp arguments based on the runtime environment.
// Local: uses browser cookies. Cloud Run: uses nodejs runtime + Secret Manager cookies.
func envArgs() []string {
	if os.Getenv("BASE_URL") != "" {
		args := []string{"--js-runtimes", "node", "--remote-components", "ejs:github"}
		if path := ensureCookiesFile(); path != "" {
			args = append(args, "--cookies", path)
		}
		return args
	}
	return []string{"--cookies-from-browser", "chrome"}
}

// getVideoInfo fetches video metadata via yt-dlp --dump-json.
func (d *Downloader) getVideoInfo(ctx context.Context, url string) (*ytdlpInfo, error) {
	args := []string{
		"--dump-json",
		"--no-download",
	}
	args = append(args, envArgs()...)
	args = append(args, url)

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("yt-dlp info failed: %w: %s", err, stderr.String())
	}

	var info ytdlpInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return nil, fmt.Errorf("failed to parse yt-dlp output: %w", err)
	}

	return &info, nil
}

// downloadToCache downloads the video to the cache directory via yt-dlp.
func (d *Downloader) downloadToCache(ctx context.Context, url, videoID string, cache *Cache) (string, error) {
	// Output template: cache dir + videoID.%(ext)s
	outputTemplate := filepath.Join(cache.dir, videoID+".%(ext)s")

	args := []string{}
	args = append(args, envArgs()...)
	args = append(args,
		"--format", "bestvideo[ext=mp4]+bestaudio[ext=m4a]/best[ext=mp4]/best",
		"--merge-output-format", "mp4",
		"--output", outputTemplate,
		"--no-playlist",
		url,
	)

	fmt.Fprintf(os.Stderr, "Downloading to cache...\n")

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("yt-dlp download failed: %w: %s", err, stderr.String())
	}
	fmt.Fprint(os.Stderr, stderr.String())

	// Find the downloaded file
	cachedPath, found := cache.GetCachedPath(videoID)
	if !found {
		return "", fmt.Errorf("download completed but cached file not found for %s", videoID)
	}

	info, _ := os.Stat(cachedPath)
	if info != nil {
		fmt.Fprintf(os.Stderr, "Downloaded: %s\n", formatBytes(info.Size()))
	}

	return cachedPath, nil
}

// buildOutputPath constructs the output file path based on title, options, and format.
func (d *Downloader) buildOutputPath(title, cachedPath string, opts ProcessOpts) (string, error) {
	base := sanitizeFilename(title)

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
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
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

// formatDuration formats seconds into a human-readable duration string.
func formatDuration(seconds float64) string {
	total := int(seconds)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60

	if h > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", h, m, s)
	}
	return fmt.Sprintf("%dm%02ds", m, s)
}
