// Package download handles video downloads using yt-dlp.
package download

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Downloader handles video downloads.
type Downloader struct {
	outputDir string
	format    string
	audioOnly bool
}

// NewDownloader creates a new downloader with the specified options.
func NewDownloader(outputDir, format string, audioOnly bool) *Downloader {
	return &Downloader{
		outputDir: outputDir,
		format:    format,
		audioOnly: audioOnly,
	}
}

// Download downloads a video from the given URL.
func (d *Downloader) Download(url string) error {
	// Check if yt-dlp is installed
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		return fmt.Errorf("yt-dlp not found in PATH. Please install it first:\n" +
			"  brew install yt-dlp  (macOS)\n" +
			"  pip install yt-dlp   (pip)")
	}

	fmt.Fprintf(os.Stderr, "⬇️  Downloading video: %s\n\n", url)

	// Build yt-dlp command arguments
	args := []string{
		"-o", filepath.Join(d.outputDir, "%(title)s.%(ext)s"),
	}

	if d.audioOnly {
		args = append(args,
			"-f", "bestaudio/best",
			"-x",
			"--audio-format", "mp3",
			"--audio-quality", "192K",
		)
	} else {
		if d.format == "best" {
			args = append(args, "-f", "bestvideo+bestaudio/best")
		} else {
			args = append(args, "-f", d.format)
		}
	}

	args = append(args, url)

	// Execute yt-dlp
	cmd := exec.Command("yt-dlp", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error downloading video: %w", err)
	}

	fmt.Fprintf(os.Stderr, "\n✅ Download completed successfully\n")
	return nil
}
