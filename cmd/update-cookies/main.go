// Command update-cookies exports YouTube cookies from a local browser
// and uploads them to GCP Secret Manager for use by the Cloud Run MCP server.
//
// Usage:
//
//	update-cookies                          # export from Chrome, upload to Secret Manager
//	update-cookies --browser firefox        # export from Firefox
//	update-cookies --file cookies.txt       # upload an existing Netscape cookie file
//	update-cookies --project my-gcp-project # override GCP project
//	update-cookies --dry-run                # show what would be uploaded without uploading
package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	secretmanagerpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"gopkg.in/yaml.v3"
)

const (
	secretID    = "scm-pwd-ytm-youtube-cookies"
	configFile  = "config.yaml"
)

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
	project := flag.String("project", "", "GCP project ID (default: from config.yaml)")
	dryRun := flag.Bool("dry-run", false, "Show what would be uploaded without uploading")
	flag.Parse()

	// Resolve GCP project
	projectID := *project
	if projectID == "" {
		projectID = os.Getenv("SECRET_PROJECT")
	}
	if projectID == "" {
		projectID = loadProjectFromConfig()
	}
	if projectID == "" {
		log.Fatal("GCP project not specified. Use --project, SECRET_PROJECT env var, or config.yaml")
	}

	var cookieData []byte
	var err error

	if *file != "" {
		// Use provided file
		cookieData, err = os.ReadFile(*file)
		if err != nil {
			log.Fatalf("Failed to read cookie file: %v", err)
		}
		fmt.Fprintf(os.Stderr, "Read cookie file: %s\n", *file)
	} else {
		// Export from browser via yt-dlp
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
	fmt.Fprintf(os.Stderr, "Target project:   %s\n", projectID)
	fmt.Fprintf(os.Stderr, "Target secret:    %s\n", secretID)

	if *dryRun {
		fmt.Fprintf(os.Stderr, "\n[dry-run] Would upload %d bytes to projects/%s/secrets/%s\n", len(filtered), projectID, secretID)
		return
	}

	// Upload to Secret Manager
	if err := uploadSecret(projectID, filtered); err != nil {
		log.Fatalf("Failed to upload to Secret Manager: %v", err)
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

	// yt-dlp can export cookies with --cookies-from-browser and --cookies
	// We use a dummy URL with --skip-download to trigger cookie export only
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
	// Increase buffer for potentially long lines
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)

	for scanner.Scan() {
		line := scanner.Text()

		// Keep comment lines (Netscape header)
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			filtered.WriteString(line)
			filtered.WriteByte('\n')
			continue
		}

		// Cookie lines are tab-separated: domain, flag, path, secure, expiry, name, value
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

// uploadSecret creates a new version of the secret with the given data.
func uploadSecret(projectID string, data []byte) error {
	ctx := context.Background()
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create Secret Manager client: %w", err)
	}
	defer client.Close()

	secretName := fmt.Sprintf("projects/%s/secrets/%s", projectID, secretID)

	// Ensure secret exists
	_, err = client.GetSecret(ctx, &secretmanagerpb.GetSecretRequest{Name: secretName})
	if err != nil {
		// Create the secret if it doesn't exist
		fmt.Fprintf(os.Stderr, "Creating secret %s...\n", secretID)
		_, err = client.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
			Parent:   fmt.Sprintf("projects/%s", projectID),
			SecretId: secretID,
			Secret: &secretmanagerpb.Secret{
				Replication: &secretmanagerpb.Replication{
					Replication: &secretmanagerpb.Replication_Automatic_{
						Automatic: &secretmanagerpb.Replication_Automatic{},
					},
				},
			},
		})
		if err != nil {
			return fmt.Errorf("failed to create secret: %w", err)
		}
	}

	// Add new version
	fmt.Fprintf(os.Stderr, "Uploading new secret version...\n")
	resp, err := client.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent: secretName,
		Payload: &secretmanagerpb.SecretPayload{
			Data: data,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to add secret version: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Created version: %s\n", resp.Name)
	return nil
}

// loadProjectFromConfig reads the GCP project ID from config.yaml.
func loadProjectFromConfig() string {
	// Try current directory, then walk up
	paths := []string{configFile}
	for _, p := range []string{".", ".."} {
		paths = append(paths, filepath.Join(p, configFile))
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var config struct {
			GCP struct {
				ProjectID string `yaml:"project_id"`
			} `yaml:"gcp"`
		}
		if err := yaml.Unmarshal(data, &config); err != nil {
			continue
		}
		if config.GCP.ProjectID != "" {
			return config.GCP.ProjectID
		}
	}

	return ""
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
