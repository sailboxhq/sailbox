package service

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/sailboxhq/sailbox/apps/api/internal/model"
	"github.com/sailboxhq/sailbox/apps/api/internal/orchestrator"
	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

type BuildService struct {
	store  store.Store
	orch   orchestrator.Orchestrator
	logger *slog.Logger
}

func NewBuildService(s store.Store, orch orchestrator.Orchestrator, logger *slog.Logger) *BuildService {
	return &BuildService{store: s, orch: orch, logger: logger}
}

// Build clones source code, builds a container image, and updates the app's DockerImage.
func (s *BuildService) Build(ctx context.Context, app *model.Application, deploy *model.Deployment) error {
	s.logger.Info("starting build",
		slog.String("app", app.Name),
		slog.String("repo", app.GitRepo),
		slog.String("branch", app.GitBranch),
		slog.String("buildType", string(app.BuildType)),
	)

	// Resolve git token — prefer GitHub App installation token (always fresh)
	gitToken, tokenErr := s.resolveGitToken(ctx, app)
	if tokenErr != nil {
		s.logger.Warn("failed to resolve git token, build may fail for private repos",
			slog.Any("error", tokenErr),
			slog.String("app", app.Name),
		)
	}

	opts := orchestrator.BuildOpts{
		GitRepo:      app.GitRepo,
		GitBranch:    app.GitBranch,
		CommitSHA:    deploy.CommitSHA,
		GitToken:     gitToken,
		Dockerfile:   app.Dockerfile,
		BuildContext: app.BuildContext,
		BuildArgs:    app.BuildArgs,
		BuildEnvVars: app.BuildEnvVars,
		BuildType:    string(app.BuildType),
		NoCache:      app.NoCache,
		OnLog: func(logs string) {
			// Sanitize tokens from build logs
			if gitToken != "" {
				logs = strings.ReplaceAll(logs, gitToken, "[REDACTED]")
			}
			deploy.BuildLog = logs
			_ = s.store.Deployments().Update(ctx, deploy)
		},
	}

	// Default build type to dockerfile
	if opts.BuildType == "" {
		opts.BuildType = "dockerfile"
	}

	result, err := s.orch.Build(ctx, app, opts)
	if err != nil {
		// Save build logs from the result (even on failure, logs may be available)
		if result != nil && result.Logs != "" {
			deploy.BuildLog = result.Logs + "\n\n--- Error ---\n" + err.Error()
		} else {
			deploy.BuildLog = err.Error()
		}
		_ = s.store.Deployments().Update(ctx, deploy)
		return err
	}

	// Update app with the built image
	app.DockerImage = result.Image
	if err := s.store.Applications().Update(ctx, app); err != nil {
		return fmt.Errorf("update app image: %w", err)
	}

	// Save build log and image on deployment
	deploy.BuildLog = result.Logs
	deploy.Image = result.Image
	_ = s.store.Deployments().Update(ctx, deploy)

	s.logger.Info("build completed",
		slog.String("app", app.Name),
		slog.String("image", result.Image),
		slog.Duration("duration", result.Duration),
	)
	return nil
}

// resolveGitToken gets a fresh token for git operations.
// Priority: GitHub App installation token > stored resource token.
func (s *BuildService) resolveGitToken(ctx context.Context, app *model.Application) (string, error) {
	// Try GitHub App installation token first (always fresh, never expires during build)
	token, err := s.generateInstallationToken(ctx, app.GitRepo)
	if err == nil && token != "" {
		s.logger.Info("using GitHub App installation token", slog.String("app", app.Name))
		return token, nil
	}

	// Fallback: stored token from linked git provider resource
	if app.GitProviderID != nil {
		resource, err := s.store.SharedResources().GetByID(ctx, *app.GitProviderID)
		if err == nil {
			var cfg struct {
				Token string `json:"token"`
			}
			if json.Unmarshal(resource.Config, &cfg) == nil && cfg.Token != "" {
				s.logger.Info("using stored git provider token", slog.String("app", app.Name))
				return cfg.Token, nil
			}
		}
	}

	return "", fmt.Errorf("no git token available for %s", app.Name)
}

// generateInstallationToken creates a fresh GitHub App installation access token.
func (s *BuildService) generateInstallationToken(ctx context.Context, repoURL string) (string, error) {
	// Load GitHub App credentials from settings
	appIDStr, err := s.store.Settings().Get(ctx, "github_app_id")
	if err != nil || appIDStr == "" {
		return "", fmt.Errorf("github_app_id not configured")
	}
	appID, err := strconv.ParseInt(appIDStr, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid github_app_id: %w", err)
	}

	pemKey, err := s.store.Settings().Get(ctx, "github_app_pem")
	if err != nil || pemKey == "" {
		return "", fmt.Errorf("github_app_pem not configured")
	}

	// Generate JWT
	jwtToken, err := generateAppJWT(appID, pemKey)
	if err != nil {
		return "", fmt.Errorf("generate JWT: %w", err)
	}

	// Find installation for the repo's owner
	owner := extractOwner(repoURL)
	if owner == "" {
		return "", fmt.Errorf("cannot extract owner from %s", repoURL)
	}

	installationID, err := findInstallation(ctx, jwtToken, owner)
	if err != nil {
		return "", fmt.Errorf("find installation: %w", err)
	}

	// Create installation access token
	token, err := createInstallationToken(ctx, jwtToken, installationID)
	if err != nil {
		return "", fmt.Errorf("create installation token: %w", err)
	}

	return token, nil
}

// generateAppJWT creates a short-lived JWT signed with the GitHub App's private key.
func generateAppJWT(appID int64, pemKeyStr string) (string, error) {
	block, _ := pem.Decode([]byte(pemKeyStr))
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM")
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parse private key: %w", err)
	}

	now := time.Now()
	claims := jwt.RegisteredClaims{
		Issuer:    strconv.FormatInt(appID, 10),
		IssuedAt:  jwt.NewNumericDate(now.Add(-60 * time.Second)),
		ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(key)
}

// extractOwner gets the org/user from a GitHub URL.
func extractOwner(repoURL string) string {
	// https://github.com/owner/repo.git → owner
	repoURL = strings.TrimSuffix(repoURL, ".git")
	parts := strings.Split(repoURL, "/")
	for i, p := range parts {
		if p == "github.com" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// findInstallation finds the GitHub App installation ID for an owner.
func findInstallation(ctx context.Context, jwtToken, owner string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/app/installations", nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("list installations: %s %s", resp.Status, string(body))
	}

	var installations []struct {
		ID      int64 `json:"id"`
		Account struct {
			Login string `json:"login"`
		} `json:"account"`
	}
	if err := json.Unmarshal(body, &installations); err != nil {
		return 0, fmt.Errorf("parse installations: %w", err)
	}

	for _, inst := range installations {
		if strings.EqualFold(inst.Account.Login, owner) {
			return inst.ID, nil
		}
	}
	return 0, fmt.Errorf("no installation found for %s", owner)
}

// createInstallationToken creates a fresh access token for an installation.
func createInstallationToken(ctx context.Context, jwtToken string, installationID int64) (string, error) {
	url := fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", installationID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 201 {
		return "", fmt.Errorf("create token: %s %s", resp.Status, string(body))
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse token: %w", err)
	}
	return result.Token, nil
}
