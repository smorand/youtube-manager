// Package mcpserver provides the MCP server implementation for youtube-manager.
// This file implements OAuth 2.1 authorization server endpoints as per MCP specification.
//
// MCP OAuth2 Flow:
// 1. Client discovers auth server via /.well-known/oauth-protected-resource
// 2. Client fetches auth server metadata from /.well-known/oauth-authorization-server
// 3. Client registers via /oauth/register (Dynamic Client Registration)
// 4. Client redirects user to /oauth/authorize → we redirect to Google OAuth
// 5. Google returns code to /oauth/callback → we exchange with Google
// 6. Client exchanges code at /oauth/token → we return Google tokens
// 7. Client sends Bearer token on MCP requests → we validate and use for YouTube API
package mcpserver

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"youtube-manager/internal/auth"
)

// protectedResourceMetadata represents RFC 9728 protected resource metadata.
type protectedResourceMetadata struct {
	Resource               string   `json:"resource"`
	AuthorizationServers   []string `json:"authorization_servers"`
	BearerMethodsSupported []string `json:"bearer_methods_supported,omitempty"`
	ScopesSupported        []string `json:"scopes_supported,omitempty"`
}

// authorizationServerMetadata represents RFC 8414 authorization server metadata.
type authorizationServerMetadata struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	RegistrationEndpoint              string   `json:"registration_endpoint,omitempty"`
	ScopesSupported                   []string `json:"scopes_supported,omitempty"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	GrantTypesSupported               []string `json:"grant_types_supported"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
}

// clientRegistrationRequest represents RFC 7591 dynamic client registration request.
type clientRegistrationRequest struct {
	ClientName   string   `json:"client_name,omitempty"`
	RedirectURIs []string `json:"redirect_uris"`
}

// clientRegistrationResponse represents RFC 7591 dynamic client registration response.
type clientRegistrationResponse struct {
	ClientID                string   `json:"client_id"`
	ClientSecret            string   `json:"client_secret,omitempty"`
	ClientIDIssuedAt        int64    `json:"client_id_issued_at,omitempty"`
	ClientSecretExpiresAt   int64    `json:"client_secret_expires_at,omitempty"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types,omitempty"`
	ResponseTypes           []string `json:"response_types,omitempty"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method,omitempty"`
}

// tokenResponse represents an OAuth2 token response.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// tokenErrorResponse represents an OAuth2 error response.
type tokenErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// registeredClient stores registered OAuth client information.
type registeredClient struct {
	clientID     string
	clientSecret string
	redirectURIs []string
	createdAt    time.Time
}

// authorizationState stores OAuth authorization state.
type authorizationState struct {
	clientID      string
	redirectURI   string
	codeChallenge string
	codeMethod    string
	createdAt     time.Time
}

// authorizationCode stores issued authorization codes.
type authorizationCode struct {
	clientID      string
	redirectURI   string
	codeChallenge string
	codeMethod    string
	googleToken   *oauth2.Token
	createdAt     time.Time
}

// OAuth2Server handles OAuth 2.1 authorization server endpoints.
type OAuth2Server struct {
	baseURL       string
	oauthConfig   *oauth2.Config
	oauthConfigMu sync.RWMutex

	clientsMu sync.RWMutex
	clients   map[string]*registeredClient

	statesMu sync.RWMutex
	states   map[string]*authorizationState

	codesMu sync.RWMutex
	codes   map[string]*authorizationCode

	secretProject   string
	secretName      string
	credentialFile  string
	vaultAddr       string
	vaultSecretPath string
	vaultToken      string
}

// OAuth2ServerConfig holds configuration for the OAuth2 server.
type OAuth2ServerConfig struct {
	BaseURL         string
	SecretProject   string
	SecretName      string
	CredentialFile  string
	VaultAddr       string
	VaultSecretPath string
	VaultToken      string
}

// NewOAuth2Server creates a new OAuth2 authorization server.
func NewOAuth2Server(cfg *OAuth2ServerConfig) *OAuth2Server {
	s := &OAuth2Server{
		baseURL:         cfg.BaseURL,
		secretProject:   cfg.SecretProject,
		secretName:      cfg.SecretName,
		credentialFile:  cfg.CredentialFile,
		vaultAddr:       cfg.VaultAddr,
		vaultSecretPath: cfg.VaultSecretPath,
		vaultToken:      cfg.VaultToken,
		clients:         make(map[string]*registeredClient),
		states:          make(map[string]*authorizationState),
		codes:           make(map[string]*authorizationCode),
	}

	go s.cleanupExpiredStates()
	go s.cleanupExpiredCodes()

	return s
}

// cleanupExpiredStates removes expired authorization states.
func (s *OAuth2Server) cleanupExpiredStates() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.statesMu.Lock()
		now := time.Now()
		for state, entry := range s.states {
			if now.Sub(entry.createdAt) > 10*time.Minute {
				delete(s.states, state)
			}
		}
		s.statesMu.Unlock()
	}
}

// cleanupExpiredCodes removes expired authorization codes.
func (s *OAuth2Server) cleanupExpiredCodes() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.codesMu.Lock()
		now := time.Now()
		for code, entry := range s.codes {
			if now.Sub(entry.createdAt) > 10*time.Minute {
				delete(s.codes, code)
			}
		}
		s.codesMu.Unlock()
	}
}

// LoadCredentials loads Google OAuth credentials from Vault, Secret Manager, or file.
func (s *OAuth2Server) LoadCredentials(ctx context.Context) error {
	s.oauthConfigMu.Lock()
	defer s.oauthConfigMu.Unlock()

	if s.oauthConfig != nil {
		return nil
	}

	var credentialsJSON []byte
	var err error

	// Try Vault first
	if s.vaultAddr != "" && s.vaultToken != "" {
		credentialsJSON, err = s.loadFromVault()
		if err != nil {
			slog.Warn("Failed to load credentials from Vault", "error", err)
		} else if credentialsJSON != nil {
			slog.Info("OAuth credentials loaded from Vault", "addr", s.vaultAddr)
		}
	}

	// Try Secret Manager
	if credentialsJSON == nil && s.secretProject != "" && s.secretName != "" {
		credentialsJSON, err = loadFromSecretManager(ctx, s.secretProject, s.secretName)
		if err != nil {
			slog.Warn("Failed to load credentials from Secret Manager", "error", err)
		} else {
			slog.Info("OAuth credentials loaded from Secret Manager", "project", s.secretProject, "secret", s.secretName)
		}
	}

	// Fall back to local file
	if credentialsJSON == nil && s.credentialFile != "" {
		credentialsJSON, err = os.ReadFile(s.credentialFile)
		if err != nil {
			return fmt.Errorf("failed to read credentials file %s: %w", s.credentialFile, err)
		}
		slog.Info("OAuth credentials loaded from file", "path", s.credentialFile)
	}

	if credentialsJSON == nil {
		return fmt.Errorf("no OAuth credentials available: configure Vault, Secret Manager, or credential file")
	}

	config, err := google.ConfigFromJSON(credentialsJSON, auth.Scopes...)
	if err != nil {
		return fmt.Errorf("failed to parse OAuth credentials: %w", err)
	}

	// Set redirect to our internal callback
	config.RedirectURL = s.baseURL + "/oauth/callback"

	s.oauthConfig = config
	return nil
}

// getOAuthConfig returns the loaded OAuth config, loading lazily if needed.
func (s *OAuth2Server) getOAuthConfig(ctx context.Context) (*oauth2.Config, error) {
	s.oauthConfigMu.RLock()
	config := s.oauthConfig
	s.oauthConfigMu.RUnlock()

	if config != nil {
		return config, nil
	}

	if err := s.LoadCredentials(ctx); err != nil {
		return nil, err
	}

	s.oauthConfigMu.RLock()
	defer s.oauthConfigMu.RUnlock()
	return s.oauthConfig, nil
}

// SetupRoutes registers all OAuth2 endpoints.
func (s *OAuth2Server) SetupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/.well-known/oauth-protected-resource", s.handleProtectedResourceMetadata)
	mux.HandleFunc("/.well-known/oauth-authorization-server", s.handleAuthorizationServerMetadata)
	mux.HandleFunc("/oauth/register", s.handleClientRegistration)
	mux.HandleFunc("/oauth/authorize", s.handleAuthorize)
	mux.HandleFunc("/oauth/callback", s.handleCallback)
	mux.HandleFunc("/oauth/token", s.handleToken)
}

// handleProtectedResourceMetadata serves RFC 9728 protected resource metadata.
func (s *OAuth2Server) handleProtectedResourceMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	metadata := protectedResourceMetadata{
		Resource:               s.baseURL,
		AuthorizationServers:   []string{s.baseURL},
		BearerMethodsSupported: []string{"header"},
		ScopesSupported:        []string{"youtube:read", "youtube:write"},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metadata)
}

// handleAuthorizationServerMetadata serves RFC 8414 authorization server metadata.
func (s *OAuth2Server) handleAuthorizationServerMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	metadata := authorizationServerMetadata{
		Issuer:                            s.baseURL,
		AuthorizationEndpoint:             s.baseURL + "/oauth/authorize",
		TokenEndpoint:                     s.baseURL + "/oauth/token",
		RegistrationEndpoint:              s.baseURL + "/oauth/register",
		ScopesSupported:                   []string{"youtube:read", "youtube:write"},
		ResponseTypesSupported:            []string{"code"},
		GrantTypesSupported:               []string{"authorization_code", "refresh_token"},
		CodeChallengeMethodsSupported:     []string{"S256"},
		TokenEndpointAuthMethodsSupported: []string{"none", "client_secret_basic", "client_secret_post"},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metadata)
}

// handleClientRegistration implements RFC 7591 Dynamic Client Registration.
func (s *OAuth2Server) handleClientRegistration(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req clientRegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeOAuthError(w, "invalid_request", "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if len(req.RedirectURIs) == 0 {
		writeOAuthError(w, "invalid_request", "redirect_uris is required", http.StatusBadRequest)
		return
	}

	clientID := generateSecureToken(16)
	clientSecret := generateSecureToken(32)

	client := &registeredClient{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURIs: req.RedirectURIs,
		createdAt:    time.Now(),
	}

	s.clientsMu.Lock()
	s.clients[clientID] = client
	s.clientsMu.Unlock()

	slog.Info("Registered new OAuth client", "client_id", clientID, "client_name", req.ClientName)

	resp := clientRegistrationResponse{
		ClientID:                clientID,
		ClientSecret:            clientSecret,
		ClientIDIssuedAt:        time.Now().Unix(),
		ClientSecretExpiresAt:   0,
		RedirectURIs:            req.RedirectURIs,
		GrantTypes:              []string{"authorization_code", "refresh_token"},
		ResponseTypes:           []string{"code"},
		TokenEndpointAuthMethod: "client_secret_basic",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// handleAuthorize handles the authorization request from the MCP client.
func (s *OAuth2Server) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	clientID := r.URL.Query().Get("client_id")
	redirectURI := r.URL.Query().Get("redirect_uri")
	responseType := r.URL.Query().Get("response_type")
	state := r.URL.Query().Get("state")
	codeChallenge := r.URL.Query().Get("code_challenge")
	codeChallengeMethod := r.URL.Query().Get("code_challenge_method")

	if clientID == "" {
		writeOAuthError(w, "invalid_request", "client_id is required", http.StatusBadRequest)
		return
	}
	if redirectURI == "" {
		writeOAuthError(w, "invalid_request", "redirect_uri is required", http.StatusBadRequest)
		return
	}
	if responseType != "code" {
		writeOAuthError(w, "unsupported_response_type", "Only 'code' response type is supported", http.StatusBadRequest)
		return
	}

	// Auto-register client if not exists
	s.clientsMu.RLock()
	client, exists := s.clients[clientID]
	s.clientsMu.RUnlock()

	if !exists {
		client = &registeredClient{
			clientID:     clientID,
			redirectURIs: []string{redirectURI},
			createdAt:    time.Now(),
		}
		s.clientsMu.Lock()
		s.clients[clientID] = client
		s.clientsMu.Unlock()
		slog.Info("Auto-registered OAuth client", "client_id", clientID, "redirect_uri", redirectURI)
	}

	// Validate redirect_uri, add if not present
	hasRedirectURI := false
	for _, uri := range client.redirectURIs {
		if uri == redirectURI {
			hasRedirectURI = true
			break
		}
	}
	if !hasRedirectURI {
		s.clientsMu.Lock()
		client.redirectURIs = append(client.redirectURIs, redirectURI)
		s.clientsMu.Unlock()
	}

	// Generate internal state
	internalState := generateSecureToken(32)

	authState := &authorizationState{
		clientID:      clientID,
		redirectURI:   redirectURI,
		codeChallenge: codeChallenge,
		codeMethod:    codeChallengeMethod,
		createdAt:     time.Now(),
	}

	s.statesMu.Lock()
	s.states[internalState] = authState
	s.statesMu.Unlock()

	// Append client state to our internal state
	if state != "" {
		internalState = internalState + "." + state
	}

	config, err := s.getOAuthConfig(ctx)
	if err != nil {
		slog.Error("Failed to load OAuth config", "error", err)
		http.Error(w, "OAuth configuration error", http.StatusInternalServerError)
		return
	}

	authURL := config.AuthCodeURL(internalState, oauth2.AccessTypeOffline, oauth2.ApprovalForce)

	slog.Info("Authorization request", "client_id", clientID, "redirect_uri", redirectURI)

	http.Redirect(w, r, authURL, http.StatusFound)
}

// handleCallback handles the OAuth callback from Google.
func (s *OAuth2Server) handleCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if errParam := r.URL.Query().Get("error"); errParam != "" {
		errDesc := r.URL.Query().Get("error_description")
		slog.Error("Google OAuth error", "error", errParam, "description", errDesc)
		http.Error(w, fmt.Sprintf("OAuth error: %s - %s", errParam, errDesc), http.StatusBadRequest)
		return
	}

	code := r.URL.Query().Get("code")
	fullState := r.URL.Query().Get("state")

	if code == "" || fullState == "" {
		writeOAuthError(w, "invalid_request", "Missing code or state", http.StatusBadRequest)
		return
	}

	// Split internal state from client state
	internalState := fullState
	clientState := ""
	if dotIdx := strings.Index(fullState, "."); dotIdx > 0 {
		internalState = fullState[:dotIdx]
		clientState = fullState[dotIdx+1:]
	}

	// Validate internal state
	s.statesMu.Lock()
	authState, exists := s.states[internalState]
	if exists {
		delete(s.states, internalState)
	}
	s.statesMu.Unlock()

	if !exists {
		writeOAuthError(w, "invalid_request", "Invalid or expired state", http.StatusBadRequest)
		return
	}

	// Exchange code with Google
	config, err := s.getOAuthConfig(ctx)
	if err != nil {
		slog.Error("Failed to load OAuth config", "error", err)
		http.Error(w, "OAuth configuration error", http.StatusInternalServerError)
		return
	}

	googleToken, err := config.Exchange(ctx, code)
	if err != nil {
		slog.Error("Failed to exchange code with Google", "error", err)
		writeOAuthError(w, "invalid_grant", "Failed to exchange authorization code", http.StatusBadRequest)
		return
	}

	slog.Info("Google OAuth exchange successful", "has_refresh_token", googleToken.RefreshToken != "")

	// Generate our own authorization code
	ourCode := generateSecureToken(32)

	codeEntry := &authorizationCode{
		clientID:      authState.clientID,
		redirectURI:   authState.redirectURI,
		codeChallenge: authState.codeChallenge,
		codeMethod:    authState.codeMethod,
		googleToken:   googleToken,
		createdAt:     time.Now(),
	}

	s.codesMu.Lock()
	s.codes[ourCode] = codeEntry
	s.codesMu.Unlock()

	redirectURL := authState.redirectURI + "?code=" + ourCode
	if clientState != "" {
		redirectURL += "&state=" + clientState
	}

	slog.Info("Redirecting to client", "redirect_uri", authState.redirectURI)

	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// handleToken handles token exchange and refresh requests.
func (s *OAuth2Server) handleToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, "invalid_request", "Invalid form data", http.StatusBadRequest)
		return
	}

	grantType := r.FormValue("grant_type")
	code := r.FormValue("code")
	clientID := r.FormValue("client_id")
	codeVerifier := r.FormValue("code_verifier")
	refreshToken := r.FormValue("refresh_token")

	// Check for client credentials in Authorization header
	if clientID == "" {
		if username, _, ok := r.BasicAuth(); ok {
			clientID = username
		}
	}

	switch grantType {
	case "authorization_code":
		s.handleAuthorizationCodeGrant(w, clientID, code, codeVerifier)
	case "refresh_token":
		s.handleRefreshTokenGrant(ctx, w, clientID, refreshToken)
	default:
		writeOAuthError(w, "unsupported_grant_type", "Only authorization_code and refresh_token are supported", http.StatusBadRequest)
	}
}

// handleAuthorizationCodeGrant handles the authorization_code grant type.
func (s *OAuth2Server) handleAuthorizationCodeGrant(w http.ResponseWriter, clientID, code, codeVerifier string) {
	if code == "" {
		writeOAuthError(w, "invalid_request", "code is required", http.StatusBadRequest)
		return
	}

	s.codesMu.Lock()
	codeEntry, exists := s.codes[code]
	if exists {
		delete(s.codes, code) // Single use
	}
	s.codesMu.Unlock()

	if !exists {
		writeOAuthError(w, "invalid_grant", "Invalid or expired authorization code", http.StatusBadRequest)
		return
	}

	if clientID != "" && clientID != codeEntry.clientID {
		writeOAuthError(w, "invalid_client", "client_id mismatch", http.StatusUnauthorized)
		return
	}

	// Validate PKCE
	if codeEntry.codeChallenge != "" {
		if codeVerifier == "" {
			writeOAuthError(w, "invalid_request", "code_verifier is required", http.StatusBadRequest)
			return
		}
		if !validatePKCE(codeVerifier, codeEntry.codeChallenge, codeEntry.codeMethod) {
			writeOAuthError(w, "invalid_grant", "Invalid code_verifier", http.StatusBadRequest)
			return
		}
	}

	googleToken := codeEntry.googleToken

	resp := tokenResponse{
		AccessToken:  googleToken.AccessToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(time.Until(googleToken.Expiry).Seconds()),
		RefreshToken: googleToken.RefreshToken,
		Scope:        "youtube:read youtube:write",
	}

	slog.Info("Token issued for client", "client_id", codeEntry.clientID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleRefreshTokenGrant handles the refresh_token grant type.
func (s *OAuth2Server) handleRefreshTokenGrant(ctx context.Context, w http.ResponseWriter, clientID, refreshToken string) {
	if refreshToken == "" {
		writeOAuthError(w, "invalid_request", "refresh_token is required", http.StatusBadRequest)
		return
	}

	config, err := s.getOAuthConfig(ctx)
	if err != nil {
		slog.Error("Failed to load OAuth config", "error", err)
		writeOAuthError(w, "server_error", "OAuth configuration error", http.StatusInternalServerError)
		return
	}

	token := &oauth2.Token{
		RefreshToken: refreshToken,
	}

	tokenSource := config.TokenSource(ctx, token)
	newToken, err := tokenSource.Token()
	if err != nil {
		slog.Error("Failed to refresh token", "error", err)
		writeOAuthError(w, "invalid_grant", "Failed to refresh token", http.StatusBadRequest)
		return
	}

	resp := tokenResponse{
		AccessToken:  newToken.AccessToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(time.Until(newToken.Expiry).Seconds()),
		RefreshToken: newToken.RefreshToken,
		Scope:        "youtube:read youtube:write",
	}

	if resp.RefreshToken == "" {
		resp.RefreshToken = refreshToken
	}

	slog.Info("Token refreshed for client", "client_id", clientID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ValidateAccessToken validates a Bearer token and returns the OAuth config and token for API calls.
func (s *OAuth2Server) ValidateAccessToken(ctx context.Context, accessToken string) (*oauth2.Config, *oauth2.Token, error) {
	config, err := s.getOAuthConfig(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load OAuth config: %w", err)
	}

	token := &oauth2.Token{
		AccessToken: accessToken,
	}

	return config, token, nil
}

// generateSecureToken generates a cryptographically secure random token.
func generateSecureToken(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		slog.Warn("crypto/rand failed, using time-based seed")
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return base64.URLEncoding.EncodeToString(b)[:length+length/3]
}

// validatePKCE validates the PKCE code_verifier against the stored code_challenge.
func validatePKCE(verifier, challenge, method string) bool {
	if method != "S256" && method != "" {
		return false
	}

	h := sha256.Sum256([]byte(verifier))
	computed := base64.RawURLEncoding.EncodeToString(h[:])

	return computed == challenge
}

// writeOAuthError writes a standard OAuth2 error response.
func writeOAuthError(w http.ResponseWriter, errorCode, description string, statusCode int) {
	resp := tokenErrorResponse{
		Error:            errorCode,
		ErrorDescription: description,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(resp)
}

// loadFromSecretManager loads credentials from Google Secret Manager.
func loadFromSecretManager(ctx context.Context, project, secretName string) ([]byte, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create Secret Manager client: %w", err)
	}
	defer client.Close()

	secretPath := fmt.Sprintf("projects/%s/secrets/%s/versions/latest", project, secretName)
	result, err := client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: secretPath,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to access secret %s: %w", secretPath, err)
	}

	return result.Payload.Data, nil
}

// loadFromVault loads credentials from HashiCorp Vault KV v2 secret store.
func (s *OAuth2Server) loadFromVault() ([]byte, error) {
	if s.vaultAddr == "" || s.vaultToken == "" {
		return nil, nil
	}

	secretPath := s.vaultSecretPath
	if secretPath == "" {
		secretPath = "secret/credentials/google-credentials"
	}

	return loadFromVaultHTTP(s.vaultAddr, s.vaultToken, secretPath)
}

// loadFromVaultHTTP fetches a secret from Vault KV v2 via HTTP API.
func loadFromVaultHTTP(vaultAddr, vaultToken, secretPath string) ([]byte, error) {
	url := fmt.Sprintf("%s/v1/secret/data/%s", strings.TrimRight(vaultAddr, "/"), secretPath)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create vault request: %w", err)
	}
	req.Header.Set("X-Vault-Token", vaultToken)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vault connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vault returned status %d", resp.StatusCode)
	}

	var vaultResp struct {
		Data struct {
			Data json.RawMessage `json:"data"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&vaultResp); err != nil {
		return nil, fmt.Errorf("parse vault response: %w", err)
	}

	if len(vaultResp.Data.Data) == 0 {
		return nil, fmt.Errorf("vault returned empty credentials")
	}

	var kvData map[string]string
	if json.Unmarshal(vaultResp.Data.Data, &kvData) == nil && len(kvData) == 1 {
		for _, v := range kvData {
			if json.Valid([]byte(v)) {
				return []byte(v), nil
			}
		}
	}

	return vaultResp.Data.Data, nil
}
