# YouTube Manager

A command-line tool for managing YouTube content using YouTube Data API v3 and yt-dlp.

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
  - Download videos using yt-dlp (supports audio-only and custom formats)

## Prerequisites

1. **Go 1.21 or later** - Install from [golang.org](https://golang.org/)
2. **yt-dlp** (optional, for downloading videos)
   - macOS: `brew install yt-dlp`
   - Other platforms: `pip install yt-dlp`

## Setup

### 1. YouTube API Credentials

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select an existing one
3. Enable the YouTube Data API v3
4. Create OAuth 2.0 credentials (Desktop application)
5. Download the credentials JSON file
6. Save it as `~/.credentials/google_credentials.json`

### 2. Build and Install

```bash
# Build the binary
make build

# Install to /usr/local/bin
make install

# Or install to a custom location
TARGET=/path/to/bin make install
```

## Usage

### Authentication

On first use, the tool will open a browser for OAuth authorization. The token will be saved to `~/.credentials/youtube_token.json`.

### Commands

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

# Download audio only (MP3)
youtube-manager download <video-url> --audio-only

# Custom format
youtube-manager download <video-url> --format "bestvideo[height<=720]+bestaudio/best"
```

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

## Development

### Build
```bash
make build
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
make clean      # removes binary only
make clean-all  # removes binary, go.mod, and go.sum
```

## Project Structure

```
youtube-manager/
├── Makefile              # Build and installation targets
├── README.md             # This file
├── CLAUDE.md             # AI-oriented documentation
├── src/                  # Go source code
│   ├── main.go          # Entry point and command registration
│   ├── cli.go           # CLI command implementations
│   ├── auth.go          # OAuth authentication
│   ├── go.mod           # Go module definition
│   └── go.sum           # Dependency checksums
└── youtube-manager      # Compiled binary
```

## OAuth Scopes

The tool requests the following YouTube API scopes:
- `youtube.readonly` - View YouTube account
- `youtube.force-ssl` - Manage YouTube account (for creating/deleting playlists)

## Troubleshooting

### "Credentials file not found"
Ensure you've placed your OAuth credentials at `~/.credentials/google_credentials.json`

### "yt-dlp not found"
Install yt-dlp using:
- macOS: `brew install yt-dlp`
- Other: `pip install yt-dlp`

### Authentication errors
Delete the token file and re-authenticate:
```bash
rm ~/.credentials/youtube_token.json
youtube-manager list-playlists
```

## License

This project is for personal use.

## Author

Sebastien MORAND (sebastien.morand@loreal.com)
