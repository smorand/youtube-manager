// Command update-cookies exports YouTube cookies from a local browser
// and copies them to a remote VPS via SCP for use by the MCP server.
//
// Usage:
//
//	update-cookies --host 1.2.3.4                    # export from Chrome, SCP to VPS
//	update-cookies --host 1.2.3.4 --browser firefox  # export from Firefox
//	update-cookies --host 1.2.3.4 --file cookies.txt # upload an existing Netscape cookie file
//	update-cookies --dry-run                          # show what would be uploaded without uploading
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const cookieFileName = "cookies.txt"

// youtubeDomains are the cookie domains relevant for YouTube authentication.
var youtubeDomains = []string{
	".youtube.com",
	".google.com",
	".googleapis.com",
	".googlevideo.com",
}

func main() {
	browser := flag.String("browser", "chrome", "Browser to export cookies from (chrome, firefox, edge, safari)")
	file := flag.String("file", "", "Path to an existing Netscape cookie file (skips browser export)")
	host := flag.String("host", "", "VPS host (IP or hostname)")
	user := flag.String("user", "root", "SSH user")
	key := flag.String("key", filepath.Join(os.Getenv("HOME"), ".ssh", "id_rsa"), "Path to SSH private key")
	remotePath := flag.String("remote-path", "/app/data/youtube-manager", "Remote directory for cookie file")
	dryRun := flag.Bool("dry-run", false, "Show what would be uploaded without uploading")
	flag.Parse()

	if *host == "" {
		log.Fatal("VPS host not specified. Use --host flag.")
	}

	var cookieData []byte
	var err error

	if *file != "" {
		cookieData, err = os.ReadFile(*file)
		if err != nil {
			log.Fatalf("Failed to read cookie file: %v", err)
		}
		fmt.Fprintf(os.Stderr, "Read cookie file: %s\n", *file)
	} else {
		cookieData, err = exportBrowserCookies(*browser)
		if err != nil {
			log.Fatalf("Failed to export cookies: %v", err)
		}
	}

	// Filter to YouTube-relevant domains
	filtered := filterCookies(cookieData)

	// Show stats
	totalLines := countLines(cookieData)
	filteredLines := countLines(filtered)
	fmt.Fprintf(os.Stderr, "Total cookies:    %d lines (%s)\n", totalLines, formatBytes(int64(len(cookieData))))
	fmt.Fprintf(os.Stderr, "Filtered cookies: %d lines (%s)\n", filteredLines, formatBytes(int64(len(filtered))))
	fmt.Fprintf(os.Stderr, "Domains kept:     %s\n", strings.Join(youtubeDomains, ", "))
	fmt.Fprintf(os.Stderr, "Target:           %s@%s:%s/%s\n", *user, *host, *remotePath, cookieFileName)

	if *dryRun {
		fmt.Fprintf(os.Stderr, "\n[dry-run] Would upload %d bytes to %s@%s:%s/%s\n",
			len(filtered), *user, *host, *remotePath, cookieFileName)
		return
	}

	// Write filtered cookies to temp file, then SCP to VPS
	if err := uploadViaSCP(*host, *user, *key, *remotePath, filtered); err != nil {
		log.Fatalf("Failed to upload cookies via SCP: %v", err)
	}

	fmt.Fprintf(os.Stderr, "\nCookies uploaded successfully.\n")
}

// exportBrowserCookies uses yt-dlp to export cookies from the specified browser.
func exportBrowserCookies(browser string) ([]byte, error) {
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		return nil, fmt.Errorf("yt-dlp not found in PATH: install it with 'brew install yt-dlp'")
	}

	tmpFile := filepath.Join(os.TempDir(), "ytm-cookie-export.txt")
	defer os.Remove(tmpFile)

	fmt.Fprintf(os.Stderr, "Exporting cookies from %s via yt-dlp...\n", browser)

	cmd := exec.Command("yt-dlp",
		"--cookies-from-browser", browser,
		"--cookies", tmpFile,
		"--skip-download",
		"--no-download",
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ",
	)
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("yt-dlp cookie export failed: %w", err)
	}

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read exported cookies: %w", err)
	}

	return data, nil
}

// filterCookies keeps only cookies from YouTube-relevant domains.
func filterCookies(data []byte) []byte {
	var filtered strings.Builder
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			filtered.WriteString(line)
			filtered.WriteByte('\n')
			continue
		}

		fields := strings.Split(line, "\t")
		if len(fields) < 7 {
			continue
		}

		domain := fields[0]
		if isYouTubeDomain(domain) {
			filtered.WriteString(line)
			filtered.WriteByte('\n')
		}
	}

	return []byte(filtered.String())
}

// isYouTubeDomain checks if a domain matches YouTube-relevant domains.
func isYouTubeDomain(domain string) bool {
	for _, yd := range youtubeDomains {
		if domain == yd || strings.HasSuffix(domain, yd) {
			return true
		}
	}
	return false
}

// uploadViaSCP writes cookies to a temp file and copies it to the VPS via scp.
func uploadViaSCP(host, user, keyPath, remotePath string, data []byte) error {
	tmpFile, err := os.CreateTemp("", "ytm-cookies-upload-*.txt")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()

	// Ensure remote directory exists
	dest := fmt.Sprintf("%s@%s", user, host)
	fmt.Fprintf(os.Stderr, "Ensuring remote directory %s exists...\n", remotePath)

	mkdirCmd := exec.Command("ssh",
		"-i", keyPath,
		"-o", "StrictHostKeyChecking=accept-new",
		dest,
		"mkdir", "-p", remotePath,
	)
	mkdirCmd.Stderr = os.Stderr
	if err := mkdirCmd.Run(); err != nil {
		return fmt.Errorf("failed to create remote directory: %w", err)
	}

	// SCP the file
	remoteDest := fmt.Sprintf("%s:%s/%s", dest, remotePath, cookieFileName)
	fmt.Fprintf(os.Stderr, "Uploading cookies via SCP to %s...\n", remoteDest)

	scpCmd := exec.Command("scp",
		"-i", keyPath,
		"-o", "StrictHostKeyChecking=accept-new",
		tmpFile.Name(),
		remoteDest,
	)
	scpCmd.Stderr = os.Stderr
	if err := scpCmd.Run(); err != nil {
		return fmt.Errorf("scp failed: %w", err)
	}

	// Make the file owned by appuser (UID 1000) so the container can read and write it
	remoteFile := fmt.Sprintf("%s/%s", remotePath, cookieFileName)
	fixPermsCmd := exec.Command("ssh",
		"-i", keyPath,
		"-o", "StrictHostKeyChecking=accept-new",
		dest,
		"chown", "1000:1000", remoteFile,
	)
	fixPermsCmd.Stderr = os.Stderr
	if err := fixPermsCmd.Run(); err != nil {
		return fmt.Errorf("failed to chown remote file: %w", err)
	}

	return nil
}

func countLines(data []byte) int {
	count := 0
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "#") && strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

func formatBytes(bytes int64) string {
	const kb = 1024
	const mb = kb * 1024
	switch {
	case bytes >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
