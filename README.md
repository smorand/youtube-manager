# YouTube Manager

A command-line tool and MCP server for managing YouTube content using YouTube Data API v3.

## Features

- **Playlist Management**
  - List your YouTube playlists
  - Get videos from a playlist
  - Create new playlists
  - Delete playlists
  - Add videos to playlists

- **Video Operations**
  - Search for videos
  - Get detailed video information
  - Download videos with cache and post-processing

- **Download Enhancements**
  - `/tmp` cache to avoid re-downloading the same video (24h expiration)
  - Audio extraction to MP3 (192kbps via ffmpeg)
  - Time-based extraction (`--extract-from` / `--extract-to` in seconds)

- **MCP Server**
  - All 8 operations exposed as MCP tools over stdio transport
  - For AI assistant integration (Claude Code, etc.)

## Prerequisites

1. **Go 1.21 or later** - Install from [golang.org](https://golang.org/)
2. **ffmpeg** (for audio extraction and time-based cutting)
   - macOS: `brew install ffmpeg`
   - Other: see [ffmpeg.org](https://ffmpeg.org/)

## Setup

### 1. YouTube API Credentials

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select an existing one
3. Enable the YouTube Data API v3
4. Create OAuth 2.0 credentials (Web application)
5. Add `http://localhost:8000/oauth2callback` as an authorized redirect URI
6. Download the credentials JSON file
7. Save it as `~/.credentials/scm-pwd-web.json`

### 2. Build and Install

```bash
# Build the CLI binary
make build

# Build the MCP server binary
make build-mcp

# Install CLI to /usr/local/bin
make install

# Install MCP server to /usr/local/bin
make install-mcp
```

## Usage

### Authentication

On first use, the tool will:
1. Start a local web server on port 8000
2. Automatically open your browser for OAuth authorization
3. Capture the authorization callback automatically
4. Save the token to `~/.credentials/youtube-token.json`

No manual copy/paste of authorization codes is required.

### CLI Commands

#### List Playlists
```bash
youtube-manager list-playlists [--limit 50]
```

#### Get Playlist Videos
```bash
youtube-manager get-playlist <playlist-id> [--limit 50]
```

#### Search Videos
```bash
youtube-manager search "search query" [--limit 10]
```

#### Get Video Details
```bash
youtube-manager get-video <video-id>
```

#### Download Video
```bash
# Download best quality video
youtube-manager download <video-url>

# Download to specific directory
youtube-manager download <video-url> --output ~/Downloads

# Download audio only (converts to MP3 via ffmpeg)
youtube-manager download <video-url> --audio-only

# Extract first 60 seconds
youtube-manager download <video-url> --extract-to 60

# Extract audio from 10s to 60s as MP3
youtube-manager download <video-url> --audio-only --extract-from 10 --extract-to 60

# Custom video quality
youtube-manager download <video-url> --format 720p
```

Downloads are cached in `/tmp/youtube-manager-cache/` for 24 hours. Repeated downloads of the same video will use the cache.

#### Create Playlist
```bash
youtube-manager create-playlist "Playlist Title" \
  --description "Playlist description" \
  --privacy private  # or public, unlisted
```

#### Delete Playlist
```bash
youtube-manager delete-playlist <playlist-id>
```

#### Add Video to Playlist
```bash
youtube-manager add-to-playlist <playlist-id> <video-id>
```

### MCP Server

The MCP server exposes all operations as tools over stdio transport for AI assistant integration.

#### Register with Claude Code
```bash
claude mcp add youtube-manager ./bin/youtube-manager-mcp-darwin-arm64
```

#### Available MCP Tools

| Tool | Description | Required Params |
|------|-------------|-----------------|
| `list_playlists` | List your YouTube playlists | - |
| `get_playlist` | Get videos from a playlist | `playlist_id` |
| `create_playlist` | Create a new playlist | `title` |
| `delete_playlist` | Delete a playlist | `playlist_id` |
| `add_to_playlist` | Add a video to a playlist | `playlist_id`, `video_id` |
| `search_videos` | Search for videos | `query` |
| `get_video` | Get video details | `video_id` |
| `download_video` | Download a video | `url_or_video_id` |

#### Test MCP Server
```bash
echo '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}},"id":1}' | ./bin/youtube-manager-mcp-darwin-arm64
```

## Cloud Run Deployment

The MCP server can be deployed to Google Cloud Run for remote access.

**Live URL:** `https://ytm.mcp.scm-platform.org`

### Prerequisites

- Google Cloud SDK (`gcloud`) authenticated
- Terraform >= 1.0
- Docker

### First-Time Setup

```bash
# 1. Deploy initialization (state backend, service accounts, APIs)
make init-plan
make init-deploy

# 2. Upload OAuth credentials to Secret Manager
gcloud secrets versions add scm-pwd-ytm-oauth-creds \
  --data-file=$HOME/.credentials/scm-pwd-web.json \
  --project=project-fb127223-bfef-43d1-94e

# 3. Deploy infrastructure (Docker build + Cloud Run + DNS)
make plan
make deploy
```

### YouTube Cookies (yt-dlp Authentication)

Cloud Run uses yt-dlp for downloads, which requires YouTube cookies for authenticated access. Cookies are stored in Secret Manager (`scm-pwd-ytm-youtube-cookies`) and must be refreshed regularly since they expire.

#### Manual Refresh

```bash
# Build the tool
make build-cookies

# Dry run (preview without uploading)
./bin/update-cookies-darwin-arm64 --dry-run

# Upload fresh cookies
make update-cookies
```

The tool exports cookies from your local Chrome browser via yt-dlp, filters to YouTube/Google domains only, and uploads them to Secret Manager.

#### Automatic Refresh (macOS LaunchAgent)

A LaunchAgent runs every hour to keep cookies fresh:

```
~/Library/LaunchAgents/com.smorand.update-youtube-cookies.plist
```

Manage it with:
```bash
# Check status
launchctl list | grep smorand

# Stop
launchctl unload ~/Library/LaunchAgents/com.smorand.update-youtube-cookies.plist

# Start
launchctl load ~/Library/LaunchAgents/com.smorand.update-youtube-cookies.plist
```

Log output: `/tmp/update-cookies-launchd.log`

> **Note:** A LaunchAgent is used instead of cron because cron processes on macOS cannot access the Keychain, which is required to decrypt Chrome cookies.

### Updating

After code changes, redeploy with:
```bash
make deploy
```

### Terraform Targets

| Target | Description |
|--------|-------------|
| `make plan` | Plan main infrastructure changes |
| `make deploy` | Deploy (Docker build + push + Cloud Run) |
| `make undeploy` | Destroy main infrastructure |
| `make init-plan` | Plan initialization resources |
| `make init-deploy` | Deploy initialization (one-time) |
| `make terraform-help` | Show detailed Terraform help |

## Development

### Build
```bash
make build       # CLI binary
make build-mcp   # MCP server binary
```

### Run Tests
```bash
make test
```

### Format Code
```bash
make fmt
```

### Run All Checks
```bash
make check  # runs fmt, vet, and test
```

### Clean Build Artifacts
```bash
make clean      # removes binaries only
```

## Project Structure

```
youtube-manager/
├── Makefile                  # Build automation + Terraform targets
├── README.md                 # This file
├── CLAUDE.md                 # AI-oriented documentation
├── config.yaml               # GCP deployment configuration
├── Dockerfile                # Multi-stage Docker build for Cloud Run
├── go.mod                    # Go module definition
├── go.sum                    # Dependency checksums
├── cmd/                      # Main applications
│   ├── youtube-manager/      # CLI entry point
│   │   └── main.go
│   ├── youtube-manager-mcp/  # MCP server entry point
│   │   └── main.go
│   └── update-cookies/       # Cookie refresh tool
│       └── main.go
├── internal/                 # Private application code
│   ├── auth/                 # OAuth 2.0 authentication
│   ├── cli/                  # CLI command implementations
│   ├── download/             # Video download (cache, ffmpeg, downloader)
│   ├── mcpserver/            # MCP tool handlers
│   └── youtube/              # YouTube API services
└── bin/                      # Compiled binaries
```

## OAuth Scopes

The tool requests the following YouTube API scopes:
- `youtube.readonly` - View YouTube account
- `youtube.force-ssl` - Manage YouTube account (for creating/deleting playlists)

## Troubleshooting

### "Credentials file not found"
Ensure you've placed your OAuth credentials at `~/.credentials/scm-pwd-web.json`

### "ffmpeg not found"
Install ffmpeg:
- macOS: `brew install ffmpeg`
- Other: see [ffmpeg.org](https://ffmpeg.org/)

### Authentication errors
Delete the token file and re-authenticate:
```bash
rm ~/.credentials/youtube-token.json
youtube-manager list-playlists
```

### "Port 8000 already in use"
Another process is using port 8000. Either stop that process or wait for it to free up the port.

## License

This project is for personal use.

## Author

Sebastien MORAND (sebastien.morand@*******)
