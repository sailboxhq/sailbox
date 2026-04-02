package v1

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sailboxhq/sailbox/apps/api/internal/api/middleware"
	"github.com/sailboxhq/sailbox/apps/api/internal/model"
	"github.com/sailboxhq/sailbox/apps/api/internal/service"
	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

type GitHubOAuthHandler struct {
	store  store.Store
	resSvc *service.ResourceService
	appURL string // Public URL of the Sailbox instance
	logger *slog.Logger
}

func NewGitHubOAuthHandler(s store.Store, resSvc *service.ResourceService, appURL string, logger *slog.Logger) *GitHubOAuthHandler {
	return &GitHubOAuthHandler{store: s, resSvc: resSvc, appURL: appURL, logger: logger}
}

// ── Step 1: Setup — Generate manifest for auto-creating GitHub App ──

// SetupManifest returns the manifest JSON and the GitHub URL to POST it to.
// GET /api/v1/auth/github/setup?org=my-org
// The frontend will POST this manifest to GitHub via a form submission.
func (h *GitHubOAuthHandler) SetupManifest(c *gin.Context) {
	baseURL := h.appURL

	// Store user ID in state for the callback
	stateBytes := make([]byte, 16)
	_, _ = rand.Read(stateBytes)
	state := hex.EncodeToString(stateBytes)
	_ = h.store.Settings().Set(c.Request.Context(), "github_setup_state_"+state, middleware.GetUserID(c).String())

	// Generate unique app name: sailbox-{short-host}-{hex4}
	hostPart := strings.Split(c.Request.Host, ":")[0]
	hostPart = strings.ReplaceAll(hostPart, ".", "-")
	if len(hostPart) > 12 {
		hostPart = hostPart[:12]
	}
	randBytes := make([]byte, 2)
	_, _ = rand.Read(randBytes)
	appName := fmt.Sprintf("sailbox-%s-%x", hostPart, randBytes)

	// Build manifest — hook_attributes.url is REQUIRED by GitHub
	manifest := map[string]any{
		"name":         appName,
		"url":          baseURL,
		"description":  "Sailbox PaaS — deploy and manage applications on K3s",
		"public":       false,
		"redirect_url": baseURL + "/api/v1/auth/github/setup/callback",
		"callback_urls": []string{
			baseURL + "/api/v1/auth/github/callback",
		},
		// GitHub requires a public URL for webhooks.
		// In production this will be the real server URL.
		// For localhost dev, use a placeholder — webhook can be reconfigured later.
		"hook_attributes": map[string]any{
			"url": func() string {
				if strings.Contains(baseURL, "localhost") || strings.Contains(baseURL, "127.0.0.1") {
					return "https://example.com/api/v1/webhooks/github"
				}
				return baseURL + "/api/v1/webhooks/github"
			}(),
		},
		"request_oauth_on_install": true,
		"default_permissions": map[string]string{
			"contents":       "write", // clone + read repo contents
			"metadata":       "read",  // repo metadata (list repos)
			"pull_requests":  "read",  // PR info for future CI
			"administration": "read",  // see private repos
		},
		"default_events": []string{"push", "pull_request"},
	}

	// GitHub URL: personal or org
	org := c.Query("org")
	githubURL := "https://github.com/settings/apps/new"
	if org != "" {
		githubURL = fmt.Sprintf("https://github.com/organizations/%s/settings/apps/new", url.PathEscape(org))
	}

	c.JSON(200, gin.H{
		"manifest":   manifest,
		"github_url": githubURL,
		"state":      state,
	})
}

// SetupCallback handles GitHub's redirect after the user creates the App.
// GET /api/v1/auth/github/setup/callback?code=xxx&state=xxx
func (h *GitHubOAuthHandler) SetupCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		h.redirectToFrontend(c, "/resources?error=missing_code")
		return
	}

	// Verify state
	if state != "" {
		userID, _ := h.store.Settings().Get(c.Request.Context(), "github_setup_state_"+state)
		if userID == "" {
			h.redirectToFrontend(c, "/resources?error=invalid_state")
			return
		}
		_ = h.store.Settings().Set(c.Request.Context(), "github_setup_state_"+state, "")
	}

	// Exchange code for App credentials
	// POST https://api.github.com/app-manifests/{code}/conversions
	apiURL := fmt.Sprintf("https://api.github.com/app-manifests/%s/conversions", code)
	req, _ := http.NewRequest("POST", apiURL, nil)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		h.logger.Error("GitHub manifest exchange failed", slog.Any("error", err))
		h.redirectToFrontend(c, "/resources?error=exchange_failed")
		return
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode != 201 {
		h.logger.Error("GitHub manifest exchange error", slog.Int("status", resp.StatusCode), slog.String("body", string(body)))
		h.redirectToFrontend(c, "/resources?error=exchange_error")
		return
	}

	// Parse response — contains all the App credentials
	var appConfig struct {
		ID            int    `json:"id"`
		ClientID      string `json:"client_id"`
		ClientSecret  string `json:"client_secret"`
		PEM           string `json:"pem"`
		WebhookSecret string `json:"webhook_secret"`
		Name          string `json:"name"`
		Owner         struct {
			Login string `json:"login"`
		} `json:"owner"`
	}
	if err := json.Unmarshal(body, &appConfig); err != nil {
		h.logger.Error("Failed to parse GitHub App config", slog.Any("error", err))
		h.redirectToFrontend(c, "/resources?error=parse_error")
		return
	}

	// Save all credentials to settings
	ctx := c.Request.Context()
	_ = h.store.Settings().Set(ctx, "github_app_id", fmt.Sprintf("%d", appConfig.ID))
	_ = h.store.Settings().Set(ctx, "github_app_client_id", appConfig.ClientID)
	_ = h.store.Settings().Set(ctx, "github_app_client_secret", appConfig.ClientSecret)
	_ = h.store.Settings().Set(ctx, "github_app_pem", appConfig.PEM)
	_ = h.store.Settings().Set(ctx, "github_app_webhook_secret", appConfig.WebhookSecret)
	_ = h.store.Settings().Set(ctx, "github_app_name", appConfig.Name)

	// Also save the slug for building install URLs
	slug := appConfig.Name // GitHub App slug = name in lowercase with hyphens
	_ = h.store.Settings().Set(ctx, "github_app_slug", slug)

	h.logger.Info("GitHub App created via manifest",
		slog.Int("app_id", appConfig.ID),
		slog.String("client_id", appConfig.ClientID),
		slog.String("name", appConfig.Name),
	)

	// Redirect to GitHub App installation page so user can choose which repos to grant access.
	// After installation, GitHub redirects back to our setup_url (or user navigates back manually).
	installURL := fmt.Sprintf("https://github.com/apps/%s/installations/new", slug)
	c.Redirect(302, installURL)
}

// GitHubStatus returns the GitHub App configuration status and install URL.
// GET /api/v1/auth/github/status
func (h *GitHubOAuthHandler) GitHubStatus(c *gin.Context) {
	ctx := c.Request.Context()
	appName, _ := h.store.Settings().Get(ctx, "github_app_slug")
	if appName == "" {
		appName, _ = h.store.Settings().Get(ctx, "github_app_name")
	}
	clientID, _ := h.store.Settings().Get(ctx, "github_app_client_id")

	configured := clientID != ""
	installURL := ""
	if appName != "" {
		installURL = fmt.Sprintf("https://github.com/apps/%s/installations/new", appName)
	}

	c.JSON(200, gin.H{
		"configured":  configured,
		"app_name":    appName,
		"install_url": installURL,
	})
}

// ── Step 2: Connect — OAuth authorize with the created App ──

// Connect builds the GitHub OAuth URL and returns it.
// GET /api/v1/auth/github/connect?type=personal&org=my-org
func (h *GitHubOAuthHandler) Connect(c *gin.Context) {
	clientID, _ := h.store.Settings().Get(c.Request.Context(), "github_app_client_id")
	if clientID == "" {
		c.JSON(400, gin.H{"error": "not_configured", "message": "GitHub App not configured. Click Setup first."})
		return
	}

	stateBytes := make([]byte, 16)
	_, _ = rand.Read(stateBytes)
	state := hex.EncodeToString(stateBytes)
	accountType := c.DefaultQuery("type", "personal")
	org := c.Query("org")

	// Store userID and org in state (format: "userID|org")
	stateValue := middleware.GetUserID(c).String()
	if accountType == "org" && org != "" {
		stateValue += "|" + org
	}
	_ = h.store.Settings().Set(c.Request.Context(), "github_oauth_state_"+state, stateValue)

	redirectURI := h.appURL + "/api/v1/auth/github/callback"

	params := url.Values{
		"client_id":    {clientID},
		"redirect_uri": {redirectURI},
		"state":        {state},
		"scope":        {"repo,read:org"},
	}

	if accountType == "org" && org != "" {
		params.Set("login", org)
	}

	githubURL := "https://github.com/login/oauth/authorize?" + params.Encode()
	c.JSON(200, gin.H{"url": githubURL, "state": state})
}

// Callback handles the GitHub OAuth callback after user authorization.
// GET /api/v1/auth/github/callback?code=xxx&state=xxx
func (h *GitHubOAuthHandler) Callback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" || state == "" {
		h.redirectToFrontend(c, "/resources?error=missing_code")
		return
	}

	stateValue, err := h.store.Settings().Get(c.Request.Context(), "github_oauth_state_"+state)
	if err != nil || stateValue == "" {
		h.redirectToFrontend(c, "/resources?error=invalid_state")
		return
	}
	_ = h.store.Settings().Set(c.Request.Context(), "github_oauth_state_"+state, "")

	// Parse state: "userID" or "userID|orgName"
	userIDStr := stateValue
	githubOrg := ""
	if parts := strings.SplitN(stateValue, "|", 2); len(parts) == 2 {
		userIDStr = parts[0]
		githubOrg = parts[1]
	}

	clientID, _ := h.store.Settings().Get(c.Request.Context(), "github_app_client_id")
	clientSecret, _ := h.store.Settings().Get(c.Request.Context(), "github_app_client_secret")

	tokenResp, err := h.exchangeCode(clientID, clientSecret, code)
	if err != nil {
		h.logger.Error("GitHub OAuth token exchange failed", slog.Any("error", err))
		h.redirectToFrontend(c, "/resources?error=token_exchange_failed")
		return
	}

	username, err := h.getGitHubUser(tokenResp.AccessToken)
	if err != nil {
		username = "unknown"
	}

	orgID, err := h.resolveOrgID(c.Request.Context(), userIDStr)
	if err != nil {
		h.redirectToFrontend(c, "/resources?error=user_not_found")
		return
	}

	// Build name and config — include org if this was an organization connection
	displayName := username
	if githubOrg != "" {
		displayName = githubOrg
	}
	name := fmt.Sprintf("GitHub · %s", displayName)
	configMap := map[string]string{
		"token":    tokenResp.AccessToken,
		"username": username,
		"org":      githubOrg,
	}
	if tokenResp.RefreshToken != "" {
		configMap["refresh_token"] = tokenResp.RefreshToken
		expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).UTC().Format(time.RFC3339)
		configMap["expires_at"] = expiresAt
	}
	config, _ := json.Marshal(configMap)

	_, err = h.resSvc.Create(c.Request.Context(), orgID, service.CreateResourceInput{
		Name:     name,
		Type:     model.ResourceGitProvider,
		Provider: "github",
		Config:   config,
	})
	if err != nil {
		h.logger.Error("Failed to create GitHub resource", slog.Any("error", err))
		h.redirectToFrontend(c, "/resources?error=create_failed")
		return
	}

	h.redirectToFrontend(c, "/resources?tab=git&connected=true")
}

// redirectToFrontend redirects to the frontend app with the given path.
func (h *GitHubOAuthHandler) redirectToFrontend(c *gin.Context, path string) {
	c.Redirect(302, h.appURL+path)
}

// ── Helpers ──

type githubTokenResponse struct {
	AccessToken           string `json:"access_token"`
	TokenType             string `json:"token_type"`
	Scope                 string `json:"scope"`
	RefreshToken          string `json:"refresh_token"`
	ExpiresIn             int    `json:"expires_in"`               // seconds until access_token expires
	RefreshTokenExpiresIn int    `json:"refresh_token_expires_in"` // seconds until refresh_token expires
}

func (h *GitHubOAuthHandler) exchangeCode(clientID, clientSecret, code string) (*githubTokenResponse, error) {
	data := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"code":          {code},
	}

	req, _ := http.NewRequest("POST", "https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var result githubTokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("invalid response: %w", err)
	}
	if result.AccessToken == "" {
		return nil, fmt.Errorf("no access token: %s", string(body))
	}
	return &result, nil
}

func (h *GitHubOAuthHandler) getGitHubUser(token string) (string, error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	var user struct {
		Login string `json:"login"`
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	_ = json.Unmarshal(body, &user)
	return user.Login, nil
}

func (h *GitHubOAuthHandler) resolveOrgID(ctx context.Context, userIDStr string) (uuid.UUID, error) {
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.Nil, err
	}
	user, err := h.store.Users().GetByID(ctx, userID)
	if err != nil {
		return uuid.Nil, err
	}
	return user.OrgID, nil
}
