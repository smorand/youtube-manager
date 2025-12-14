# YouTube Manager - AI Documentation

## Project Overview

**Name:** youtube-manager
**Type:** CLI Application
**Language:** Go 1.21+
**Purpose:** Manage YouTube content using YouTube Data API v3 and yt-dlp

## Architecture

### Entry Point
- `src/main.go` - Main entry point, initializes Cobra commands and executes root command

### Core Modules

1. **Authentication (`src/auth.go`)**
   - OAuth 2.0 flow implementation
   - Token management (storage/retrieval)
   - YouTube API service initialization
   - Credentials: `~/.credentials/google_credentials.json`
   - Token cache: `~/.credentials/youtube_token.json`

2. **CLI Commands (`src/cli.go`)**
   - All command implementations using Cobra framework
   - Commands: list-playlists, get-playlist, download, search, get-video, create-playlist, delete-playlist, add-to-playlist
   - Global flag variables for each command
   - Color-coded output using fatih/color

3. **Dependencies**
   - `github.com/spf13/cobra` - CLI framework
   - `github.com/fatih/color` - Terminal color output
   - `golang.org/x/oauth2` - OAuth 2.0 authentication
   - `google.golang.org/api/youtube/v3` - YouTube Data API client
   - External: `yt-dlp` binary for video downloads

## Key Design Patterns

### Command Structure
Each command follows this pattern:
```go
var cmdName = &cobra.Command{
    Use:   "command-name <args>",
    Short: "Description",
    Args:  cobra.ExactArgs(n),
    RunE:  runCommandName,
}

func runCommandName(cmd *cobra.Command, args []string) error {
    ctx := context.Background()
    // Implementation
    return nil
}
```

### Authentication Flow
1. Read credentials from `~/.credentials/google_credentials.json`
2. Check for cached token at `~/.credentials/youtube_token.json`
3. If no token, initiate OAuth flow (browser-based)
4. Save token for future use
5. Return authenticated HTTP client

### Error Handling
- All functions return errors with context using `fmt.Errorf("message: %w", err)`
- Errors are propagated to main and printed to stderr
- Non-zero exit code on failure

## Code Organization Compliance

### Current Issues
1. **init() functions** - Multiple init() functions for flag parsing (lines cli.go:31, 76, 147, 209, 324)
   - Should be replaced with explicit initialization
   - Flags should be registered in main.go or command constructors

2. **Logging** - Uses fmt.Fprintf for all output
   - Should use structured logging (slog)
   - Distinguish between user output (stdout) and logs (stderr)

### File Structure
```
youtube-manager/
├── Makefile           # Build automation
├── README.md          # User documentation
├── CLAUDE.md          # This file - AI documentation
├── .gitignore         # Git ignore rules
└── src/
    ├── main.go        # Entry point (683 bytes)
    ├── cli.go         # Command implementations (10,927 bytes)
    ├── auth.go        # OAuth/API initialization (2,855 bytes)
    ├── go.mod         # Module definition
    └── go.sum         # Dependency lock
```

## Common Tasks

### Adding a New Command
1. Create command variable in `cli.go`
2. Implement RunE function
3. Register command in `main.go` using `rootCmd.AddCommand()`
4. Add flags if needed using command.Flags()

### Modifying Authentication Scopes
- Update `scopes` variable in `auth.go:22-25`
- Delete cached token: `rm ~/.credentials/youtube_token.json`
- Re-authenticate on next run

### Testing Changes
```bash
cd src
go run . <command> <args>
```

## API Rate Limits

YouTube Data API v3 has daily quota limits:
- Default: 10,000 units/day
- Each operation costs different units (1-100+)
- Monitor usage in Google Cloud Console

## Security Considerations

1. **Credentials Storage**
   - OAuth credentials stored at `~/.credentials/` with 0700 permissions
   - Token file has 0600 permissions
   - Never commit credentials to git

2. **Scopes**
   - `youtube.readonly` - View-only access
   - `youtube.force-ssl` - Required for write operations (create/delete playlists)

## Future Improvements

1. Replace init() functions with explicit flag registration
2. Add structured logging (slog)
3. Split cli.go into separate files per functionality:
   - playlist_commands.go
   - video_commands.go
   - download_commands.go
4. Add unit tests for each command
5. Add configuration file support (~/.youtube-manager/config.yaml)
6. Implement retry logic for API failures
7. Add batch operations support

## Golang Standards Compliance

### Followed ✅
- Error wrapping with %w
- Context as first parameter
- Clear function and variable naming
- No code duplication
- Proper error handling (no ignored errors)

### Needs Improvement ⚠️
- Remove init() functions (forbidden per standards)
- Add structured logging
- Consider moving from src/ to root-level package structure
- Add comprehensive tests
- Document all exported functions

## Build and Installation

```bash
# Build
make build

# Install
make install

# Test
make test

# Format
make fmt

# All checks
make check
```

## Environment Variables

None currently used. Authentication is file-based.

## Exit Codes

- 0: Success
- 1: Error (from cobra.Command execution or manual os.Exit(1))
