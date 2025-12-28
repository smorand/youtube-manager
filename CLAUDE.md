# YouTube Manager - AI Documentation

## Project Overview

**Name:** youtube-manager
**Type:** CLI Application
**Language:** Go 1.21+
**Purpose:** Manage YouTube content using YouTube Data API v3 and yt-dlp

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
│   └── youtube-manager/      # Main entry point
│       └── main.go           # Minimal - only initialization
├── internal/                 # Private application code
│   ├── auth/                 # OAuth 2.0 authentication
│   │   └── auth.go           # Auth client and token management
│   ├── cli/                  # CLI command implementations
│   │   ├── cli.go            # Root command and registration
│   │   ├── playlist.go       # Playlist commands
│   │   ├── video.go          # Video commands
│   │   └── download.go       # Download command
│   ├── download/             # Video download functionality
│   │   └── download.go       # yt-dlp wrapper
│   └── youtube/              # YouTube API services
│       ├── playlist.go       # Playlist operations
│       └── video.go          # Video operations
├── bin/                      # Compiled binaries (git-ignored)
│   └── youtube-manager
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
- `getHTTPClient(ctx) (*http.Client, error)` - Returns authenticated HTTP client
- `getTokenFromWeb(config) (*oauth2.Token, error)` - Initiates OAuth flow
- `tokenFromFile() (*oauth2.Token, error)` - Loads cached token
- `saveToken(token) error` - Saves token to file

**Credentials:**
- Location: `~/.credentials/google_credentials.json`
- Token cache: `~/.credentials/youtube_token.json`
- Scopes: `youtube.readonly`, `youtube.force-ssl`

#### 2. CLI Commands (`internal/cli`)

**Purpose:** Command-line interface using Cobra framework.

**Files:**
- `cli.go` - Root command and command registration
- `playlist.go` - Playlist-related commands (list, get, create, delete, add-to)
- `video.go` - Video-related commands (search, get-video)
- `download.go` - Download command

**Pattern:**
- Each command has a `create*Cmd()` function that returns `*cobra.Command`
- Each command has a corresponding `run*()` function with business logic
- Flags are scoped to their command (no global flag variables)

#### 3. YouTube Services (`internal/youtube`)

**Purpose:** Business logic for YouTube API operations.

**Types:**
- `PlaylistService` - Playlist CRUD operations
- `VideoService` - Video search and retrieval

**Key Functions:**
- Playlist: `List()`, `GetItems()`, `Create()`, `Delete()`, `AddVideo()`
- Video: `Get()`, `Search()`
- Helper functions: `PrintPlaylists()`, `PrintPlaylistItems()`, `PrintVideo()`, `PrintSearchResults()`

#### 4. Download (`internal/download`)

**Purpose:** Video download using yt-dlp.

**Type:**
- `Downloader` - Manages download options and execution

**Key Functions:**
- `NewDownloader(outputDir, format, audioOnly) *Downloader` - Creates new downloader
- `Download(url) error` - Downloads video from URL

### Dependencies

- `github.com/spf13/cobra` - CLI framework
- `golang.org/x/oauth2` - OAuth 2.0 authentication
- `google.golang.org/api/youtube/v3` - YouTube Data API client
- External: `yt-dlp` binary for video downloads

## Go Standards Compliance

### Followed ✅

1. **No `/src` directory** - Code is in `cmd/` and `internal/`
2. **No `init()` functions** - Explicit initialization in `main()`
3. **Structured logging** - Using `slog` for all logging
4. **Clear separation** - Domain logic separated from CLI
5. **Proper error handling** - All errors wrapped with context using `%w`
6. **Context as first parameter** - All service methods take `context.Context`
7. **Documented exports** - All exported types and functions documented
8. **No code duplication** - Shared logic extracted into services
9. **One responsibility per function** - Each function has single purpose
10. **Object-oriented design** - Using structs with methods for services

### Code Organization

- **Entry point** (`cmd/youtube-manager/main.go`): Minimal - only initialization and wiring
- **Business logic** (`internal/`): All implementation details
- **Packages by domain**: `auth`, `youtube`, `download`, `cli`
- **No `pkg/`**: This is an application, not a library

## Common Tasks

### Adding a New Command

1. Decide which CLI file it belongs to (`playlist.go`, `video.go`, or new file)
2. Create `create*Cmd()` function returning `*cobra.Command`
3. Create `run*()` function with business logic
4. Register in appropriate `register*Commands()` function
5. Add service methods to `internal/youtube` if needed

Example:
```go
// In internal/cli/playlist.go
func createMyCmd() *cobra.Command {
    var myFlag string

    cmd := &cobra.Command{
        Use:   "my-command <arg>",
        Short: "Description",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            return runMyCommand(cmd.Context(), args[0], myFlag)
        },
    }

    cmd.Flags().StringVar(&myFlag, "flag", "default", "Help text")
    return cmd
}

func runMyCommand(ctx context.Context, arg, flag string) error {
    // Implementation
}

// Register in registerPlaylistCommands()
func registerPlaylistCommands() {
    rootCmd.AddCommand(createListPlaylistsCmd())
    // ... other commands ...
    rootCmd.AddCommand(createMyCmd())  // Add here
}
```

### Adding a New Service Method

1. Add method to appropriate service (`PlaylistService` or `VideoService`)
2. Return errors with context: `fmt.Errorf("operation failed: %w", err)`
3. Add helper print function if needed
4. Document the function

### Modifying Authentication Scopes

1. Update `scopes` variable in `internal/auth/auth.go`
2. Delete cached token: `rm ~/.credentials/youtube_token.json`
3. Re-authenticate on next run

### Testing Changes

```bash
# Build
make build

# Run
./bin/youtube-manager <command> <args>

# Format and check
make check
```

## Build and Installation

```bash
# Build
make build

# Install to /usr/local/bin
make install

# Install to custom directory
TARGET=/usr/bin make install

# Uninstall
make uninstall

# Clean
make clean

# Download dependencies
make deps

# Run all checks (format, vet, test)
make check
```

## API Rate Limits

YouTube Data API v3 has daily quota limits:
- Default: 10,000 units/day
- Each operation costs different units (1-100+)
- Monitor usage in Google Cloud Console

## Security Considerations

1. **Credentials Storage**
   - OAuth credentials: `~/.credentials/google_credentials.json` (0700 permissions)
   - Token cache: `~/.credentials/youtube_token.json` (0600 permissions)
   - Never commit credentials to git

2. **Scopes**
   - `youtube.readonly` - View-only access
   - `youtube.force-ssl` - Required for write operations

## Logging

- Uses structured logging with `slog`
- Default level: `Error` (user-facing CLI)
- Logs to stderr
- User output goes to stdout

## Error Handling

- All errors wrapped with context using `%w`
- Service layer returns detailed errors
- CLI layer displays user-friendly messages
- Logging for debugging information

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
6. Consider adding telemetry/metrics
7. Add progress bars for downloads
8. Support for multiple video downloads
9. Playlist export/import functionality

## Migration from Old Structure

The codebase has been refactored from the old `src/` structure to follow Go standards:

**Old Structure:**
```
src/
├── main.go (683 bytes)
├── cli.go (10,927 bytes - all commands)
└── auth.go (2,855 bytes)
```

**New Structure:**
```
cmd/youtube-manager/main.go (minimal entry point)
internal/
├── auth/auth.go (authentication)
├── cli/ (commands split by domain)
├── youtube/ (business logic)
└── download/ (download functionality)
```

**Key Changes:**
- ✅ Removed `/src` directory
- ✅ Removed all `init()` functions
- ✅ Added structured logging with `slog`
- ✅ Split large `cli.go` into domain-specific files
- ✅ Extracted business logic into service packages
- ✅ Added comprehensive documentation
- ✅ Proper package structure with `cmd/` and `internal/`
- ✅ Removed `fatih/color` dependency (using emojis for visual feedback)

## Exit Codes

- 0: Success
- 1: Error (from command execution)

## Environment Variables

None currently used. All configuration is file-based.
