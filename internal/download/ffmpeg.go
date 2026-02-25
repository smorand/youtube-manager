// Package download provides ffmpeg integration for audio/video processing.
package download

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
)

// ProcessOpts configures ffmpeg processing options.
type ProcessOpts struct {
	AudioOnly   bool
	ExtractFrom int // seconds, 0 = from beginning
	ExtractTo   int // seconds, 0 = until end
}

// NeedsProcessing returns true if any post-processing is required.
func (o ProcessOpts) NeedsProcessing() bool {
	return o.AudioOnly || o.ExtractFrom > 0 || o.ExtractTo > 0
}

// CheckFFmpeg verifies that ffmpeg is installed and accessible.
func CheckFFmpeg() error {
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg not found in PATH: install it with 'brew install ffmpeg' or visit https://ffmpeg.org")
	}
	return nil
}

// Process runs ffmpeg to process input file into output with the given options.
func Process(ctx context.Context, input, output string, opts ProcessOpts) error {
	args := []string{"-y"}

	// Time-based extraction (before input for seeking efficiency)
	if opts.ExtractFrom > 0 {
		args = append(args, "-ss", strconv.Itoa(opts.ExtractFrom))
	}

	args = append(args, "-i", input)

	if opts.ExtractTo > 0 {
		args = append(args, "-to", strconv.Itoa(opts.ExtractTo-opts.ExtractFrom))
	}

	if opts.AudioOnly {
		// Convert to MP3
		args = append(args, "-vn", "-acodec", "libmp3lame", "-ab", "192k")
	} else {
		// Copy streams without re-encoding
		args = append(args, "-c", "copy")
	}

	args = append(args, output)

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg failed: %w\nOutput: %s", err, string(out))
	}

	return nil
}
