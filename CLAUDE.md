# YouTube Manager - AI Documentation

## Project Overview

**Name:** youtube-manager
**Type:** CLI Application + MCP Server
**Language:** Go 1.21+
**Purpose:** Manage YouTube content using YouTube Data API v3, with MCP server for AI assistant integration

## Architecture

This project follows the **Standard Go Project Layout** with proper separation of concerns:

### Directory Structure

```
youtube-manager/
├── Makefile                  # Build automation
├── CLAUDE.md                 # This file - AI documentation
├── README.md                 # User documentation
├── go.mod                    # Go module definition
├── go.sum                    # Dependency checksums
├── cmd/                      # Main applications
│   ├── youtube-manager/      # CLI entry point
│   │   └── main.go           # Minimal - only initialization
│   └── youtube-manager-mcp/  # MCP server entry point
│       └── main.go           # Auth init + stdio transport
├── internal/                 # Private application code
│   ├── auth/                 # OAuth 2.0 authentication
│   │   └── auth.go           # Auth client and token management
│   ├── cli/                  # CLI command implementations
│   │   ├── cli.go            # Root command and registration
│   │   ├── playlist.go       # Playlist commands
│   │   ├── video.go          # Video commands
│   │   └── download.go       # Download command
│   ├── download/             # Video download functionality
│   │   ├── download.go       # Downloader with cache + ffmpeg integration
│   │   ├── cache.go          # /tmp cache with 24h expiration
│   │   └── ffmpeg.go         # ffmpeg wrapper for extraction + conversion
│   ├── mcpserver/            # MCP server implementation
│   │   ├── server.go         # Server creation, auth, tool registration
│   │   ├── playlist_tools.go # 5 playlist tool handlers
│   │   ├── video_tools.go    # 2 video tool handlers
│   │   └── download_tools.go # 1 download tool handler
│   └── youtube/              # YouTube API services
│       ├── playlist.go       # Playlist operations
│       └── video.go          # Video operations
├── bin/                      # Compiled binaries (git-ignored)
│   ├── youtube-manager-*     # CLI binaries
│   └── youtube-manager-mcp-* # MCP server binaries
└── .gitignore                # Git ignore rules
```

### Core Components

#### 1. Authentication (`internal/auth`)

**Purpose:** Handles OAuth 2.0 flow and YouTube API service initialization.

**Key Type:**
- `Client` - Manages credentials and token storage

**Key Functions:**
- `NewClient() (*Client, error)` - Creates new auth client with default paths
- `GetYouTubeService(ctx) (*youtube.Service, error)` - Returns authenticated YouTube service

**Credentials:**
- Location: `~/.credentials/scm-pwd-web.json`
- Token cache: `~/.credentials/youtube-token.json`
- Scopes: `youtube.readonly`, `youtube.force-ssl`
- OAuth callback: Local server on port 8000 (`http://localhost:8000/oauth2callback`)
- All auth status messages output to stderr (MCP-safe)

#### 2. CLI Commands (`internal/cli`)

**Purpose:** Command-line interface using Cobra framework.

**Pattern:**
- Each command has a `create*Cmd()` function that returns `*cobra.Command`
- Each command has a corresponding `run*()` function with business logic
- Flags are scoped to their command (no global flag variables)

#### 3. YouTube Services (`internal/youtube`)

**Purpose:** Business logic for YouTube API operations.

**Types:**
- `PlaylistService` - Playlist CRUD operations: `List()`, `GetItems()`, `Create()`, `Delete()`, `AddVideo()`
- `VideoService` - Video operations: `Get()`, `Search()`

#### 4. Download (`internal/download`)

**Purpose:** Video download with caching and post-processing.

**Types:**
- `Downloader` - Manages download options and execution
- `Cache` - `/tmp/youtube-manager-cache/` with 24h expiration
- `ProcessOpts` - ffmpeg processing options

**Key Functions:**
- `NewDownloader(outputDir, format, audioOnly, extractFrom, extractTo) *Downloader`
- `Download(ctx, url) error` - Downloads video
- `DownloadWithResult(ctx, url) (*DownloadResult, error)` - Downloads and returns metadata
- `ExtractVideoID(input) string` - Extracts video ID from URL or returns as-is
- `ResolveVideoURL(input) string` - Returns full YouTube URL
- `CheckFFmpeg() error` - Verifies ffmpeg is installed
- `Process(ctx, input, output, opts) error` - Runs ffmpeg processing

**Download Flow:**
1. Extract video ID from URL/input
2. Check cache (`/tmp/youtube-manager-cache/<video_id>.<ext>`)
3. If not cached → download full video to cache
4. If post-processing needed (audio_only, extractFrom, extractTo) → run ffmpeg
5. Otherwise → copy cached file to output directory

#### 5. MCP Server (`internal/mcpserver`)

**Purpose:** Exposes all operations as MCP tools over stdio transport.

**Key Type:**
- `Server` - Wraps MCP server with YouTube services

**8 MCP Tools:**
- `list_playlists` - List user's playlists
- `get_playlist` - Get videos from a playlist
- `create_playlist` - Create a new playlist
- `delete_playlist` - Delete a playlist
- `add_to_playlist` - Add video to playlist
- `search_videos` - Search for videos
- `get_video` - Get video details
- `download_video` - Download video with options (audio_only, extract_from, extract_to)

**Library:** `github.com/mark3labs/mcp-go` v0.44.0

**API Notes (v0.44.0):**
- Tool definition: `mcp.WithString()`, `mcp.WithNumber()`, `mcp.WithBoolean()` (no `WithInteger` or `Default`)
- Property options: `mcp.Required()`, `mcp.Description()`, `mcp.Enum()`
- Parameter extraction: `req.GetString()`, `req.GetInt()`, `req.GetBool()`, `req.RequireString()`, etc.
- Results: `mcp.NewToolResultText()`, `mcp.NewToolResultError()`

### Dependencies

- `github.com/spf13/cobra` - CLI framework
- External: `yt-dlp` binary for YouTube video downloads (with browser cookie support locally)
- `github.com/mark3labs/mcp-go` - MCP server framework
- `golang.org/x/oauth2` - OAuth 2.0 authentication
- `google.golang.org/api/youtube/v3` - YouTube Data API client
- External: `ffmpeg` binary for audio extraction and time-based cutting

## Common Tasks

### Adding a New CLI Command

1. Decide which CLI file it belongs to (`playlist.go`, `video.go`, or new file)
2. Create `create*Cmd()` function returning `*cobra.Command`
3. Create `run*()` function with business logic
4. Register in appropriate `register*Commands()` function
5. Add service methods to `internal/youtube` if needed

### Adding a New MCP Tool

1. Add handler method to `Server` in appropriate tools file
2. Define tool with `mcp.NewTool()` in the register function
3. Extract params with `req.RequireString()` / `req.GetInt()` etc.
4. Return JSON via `json.MarshalIndent()` + `mcp.NewToolResultText()`
5. Handle errors with `mcp.NewToolResultError()`

### Testing Changes

```bash
# Build both binaries
make build && make build-mcp

# Run CLI
./bin/youtube-manager-darwin-arm64 <command> <args>

# Test MCP server
echo '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}},"id":1}' | ./bin/youtube-manager-mcp-darwin-arm64

# Format and check
make check
```

## Build and Installation

```bash
# Build CLI
make build

# Build MCP server
make build-mcp

# Install CLI to /usr/local/bin
make install

# Install MCP server to /usr/local/bin
make install-mcp

# Clean
make clean

# Run all checks (format, vet, test)
make check
```

## Cloud Run Deployment

**URL:** `https://ytm.mcp.scm-platform.org`
**GCP Project:** `project-fb127223-bfef-43d1-94e`
**Region:** `europe-west1`

### Infrastructure (Terraform)

Two-phase deployment using `init/` and `iac/` directories:

```
init/                           # One-time setup (local state)
├── provider.tf                 # GCP provider (no backend)
├── local.tf                    # Config loader from config.yaml
├── services.tf                 # API enablement
├── service-accounts.tf         # Cloud Build + Cloud Run SAs
└── state-backend.tf            # GCS bucket for terraform state

iac/                            # Main infrastructure (GCS backend)
├── provider.tf.template        # Template (before init-deploy)
├── provider.tf                 # Generated (after init-deploy)
├── local.tf                    # Config loader
├── docker.tf                   # Docker build + push via kreuzwerker/docker
├── secrets.tf                  # Secret Manager (OAuth credentials)
└── workload-mcp.tf             # Artifact Registry, Cloud Run, DNS, IAM
```

### Deployment Commands

```bash
# First time setup
make init-plan          # Plan initialization
make init-deploy        # Deploy state backend + service accounts

# Deploy infrastructure
make plan               # Plan main infrastructure
make deploy             # Deploy (Docker build + Cloud Run + DNS)

# Manage secrets (manual step after first deploy)
gcloud secrets versions add scm-pwd-ytm-oauth-creds \
  --data-file=$HOME/.credentials/scm-pwd-web.json \
  --project=project-fb127223-bfef-43d1-94e
```

### Transport Modes

The MCP server supports two transport modes:
- **Stdio** (default): Used locally, no `BASE_URL` env var. Auth from `~/.credentials/` files.
- **HTTP** (Cloud Run): When `BASE_URL` is set, serves HTTP with OAuth 2.1 authorization server.

### OAuth 2.1 Authentication (HTTP Mode)

On Cloud Run, the MCP server acts as an OAuth 2.1 authorization server:
1. MCP client discovers auth via `/.well-known/oauth-protected-resource`
2. Client redirects user to `/oauth/authorize` → server redirects to Google OAuth
3. Google returns code to `/oauth/callback` → server exchanges for Google token
4. Client exchanges code at `/oauth/token` → receives Google access/refresh tokens
5. Client sends Bearer token on `/mcp` requests → middleware validates and injects into context

OAuth credentials (client_id/secret) are loaded from Secret Manager via API.

### Credentials Resolution (CLI Mode)

For CLI and stdio MCP, credentials are resolved in order:
1. `OAUTH_CREDENTIALS_FILE` / `YOUTUBE_TOKEN_FILE` env vars (exact file paths)
2. `CREDENTIALS_DIR` env var (directory containing both files)
3. `~/.credentials/` (default)

## API Rate Limits

YouTube Data API v3 has daily quota limits:
- Default: 10,000 units/day
- Each operation costs different units (1-100+)
- Monitor usage in Google Cloud Console

## Security Considerations

1. **Credentials Storage**
   - OAuth credentials: `~/.credentials/scm-pwd-web.json` (0700 permissions)
   - Token cache: `~/.credentials/youtube-token.json` (0600 permissions)
   - Never commit credentials to git

2. **Scopes**
   - `youtube.readonly` - View-only access
   - `youtube.force-ssl` - Required for write operations

3. **MCP Server**
   - Stdio: Auth initialized eagerly before transport starts
   - HTTP: OAuth 2.1 server with per-request auth via Bearer tokens
   - All status messages to stderr (stdout reserved for JSON-RPC in stdio mode)
   - Cloud Run: OAuth credentials loaded from Secret Manager via API

## Logging

- Uses structured logging with `slog`
- CLI: `Error` level (user-facing)
- MCP: `Info` level (diagnostic)
- All logs to stderr
- User/MCP output goes to stdout

## Error Handling

- All errors wrapped with context using `%w`
- Service layer returns detailed errors
- CLI layer displays user-friendly messages
- MCP layer returns `mcp.NewToolResultError()` (never Go errors)

## Code Style

### File Element Order

1. Package declaration with documentation
2. Import statements (grouped: stdlib, external, internal)
3. Constants
4. Types and interfaces
5. Constructor functions (`New*`)
6. Methods (grouped by receiver, alphabetically)
7. Helper functions (alphabetically)

### Naming Conventions

- Clear and concise names
- No abbreviations (except standard: `id`, `api`, `ctx`, `err`)
- Boolean variables: `is*`, `has*`, `should*` prefixes
- Functions: verb or verb+noun form
- Exported types and functions are documented

## Future Improvements

1. Add unit tests for all packages
2. Add integration tests for CLI commands
3. Implement configuration file support (`~/.youtube-manager/config.yaml`)
4. Add retry logic for API failures with exponential backoff
5. Add batch operations support
6. Add progress bars for downloads
7. Support for multiple video downloads
8. Playlist export/import functionality

## Exit Codes

- 0: Success
- 1: Error (from command execution)

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `BASE_URL` | Base URL for OAuth2 (enables HTTP mode) | not set (stdio mode) |
| `PORT` | HTTP listen port | `8080` |
| `SECRET_PROJECT` | GCP project for Secret Manager | not set |
| `SECRET_NAME` | Secret name for OAuth credentials | not set |
| `OAUTH_CREDENTIALS_FILE` | Path to OAuth credentials JSON (CLI fallback) | `~/.credentials/scm-pwd-web.json` |
| `YOUTUBE_TOKEN_FILE` | Path to YouTube token JSON (CLI only) | `~/.credentials/youtube-token.json` |
| `CREDENTIALS_DIR` | Directory containing credential files (CLI only) | `~/.credentials/` |
| `ENVIRONMENT` | Deployment environment label | not set |
