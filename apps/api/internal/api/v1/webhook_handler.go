package v1

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/sailboxhq/sailbox/apps/api/internal/service"
	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

type WebhookHandler struct {
	store     store.Store
	deploySvc *service.DeployService
	logger    *slog.Logger
}

func NewWebhookHandler(s store.Store, deploySvc *service.DeployService, logger *slog.Logger) *WebhookHandler {
	return &WebhookHandler{store: s, deploySvc: deploySvc, logger: logger}
}

// GitHub handles GitHub push webhook events.
// POST /api/v1/webhooks/github/:appId (public, no JWT)
func (h *WebhookHandler) GitHub(c *gin.Context) {
	appID, err := uuid.Parse(c.Param("appId"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid app ID"})
		return
	}

	// Only handle push events
	event := c.GetHeader("X-GitHub-Event")
	if event == "ping" {
		c.JSON(200, gin.H{"message": "pong"})
		return
	}
	if event != "push" {
		c.JSON(200, gin.H{"message": "ignored event: " + event})
		return
	}

	// Read raw body for HMAC verification
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 10<<20)) // 10MB limit
	if err != nil {
		c.JSON(400, gin.H{"error": "failed to read body"})
		return
	}

	// Load app
	app, err := h.store.Applications().GetByID(c.Request.Context(), appID)
	if err != nil {
		c.JSON(404, gin.H{"error": "app not found"})
		return
	}

	if !app.AutoDeploy {
		c.JSON(200, gin.H{"message": "auto-deploy disabled"})
		return
	}

	// Verify HMAC signature
	signature := c.GetHeader("X-Hub-Signature-256")
	if app.WebhookSecret != "" && signature != "" {
		if !verifyGitHubSignature(app.WebhookSecret, body, signature) {
			h.logger.Warn("webhook signature mismatch", slog.String("app", app.Name))
			c.JSON(401, gin.H{"error": "invalid signature"})
			return
		}
	}

	// Parse push payload
	var payload struct {
		Ref    string `json:"ref"`   // "refs/heads/main"
		After  string `json:"after"` // commit SHA
		Pusher struct {
			Name string `json:"name"`
		} `json:"pusher"`
		Commits []struct {
			ID       string   `json:"id"`
			Message  string   `json:"message"`
			Added    []string `json:"added"`
			Removed  []string `json:"removed"`
			Modified []string `json:"modified"`
		} `json:"commits"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		c.JSON(400, gin.H{"error": "invalid payload"})
		return
	}

	// Check branch match
	branch := strings.TrimPrefix(payload.Ref, "refs/heads/")
	if branch != app.GitBranch {
		c.JSON(200, gin.H{"message": fmt.Sprintf("ignored push to %s (watching %s)", branch, app.GitBranch)})
		return
	}

	// Check watch paths — only deploy if changed files match watched patterns
	if len(app.WatchPaths) > 0 {
		matched := false
		for _, commit := range payload.Commits {
			allFiles := append(append(commit.Added, commit.Removed...), commit.Modified...)
			for _, file := range allFiles {
				for _, pattern := range app.WatchPaths {
					if matchPath(file, pattern) {
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			c.JSON(200, gin.H{"message": "no files matched watch paths, skipping deploy"})
			return
		}
	}

	// Trigger deployment
	input := service.TriggerDeployInput{
		AppID:       appID,
		TriggerType: "webhook",
	}

	deploy, err := h.deploySvc.Trigger(c.Request.Context(), input)
	if err != nil {
		h.logger.Error("webhook deploy failed", slog.Any("error", err), slog.String("app", app.Name))
		c.JSON(500, gin.H{"error": "deploy failed"})
		return
	}

	// Update commit SHA on the deployment
	if payload.After != "" {
		deploy.CommitSHA = payload.After
		_ = h.store.Deployments().Update(c.Request.Context(), deploy)
	}

	h.logger.Info("webhook triggered deploy",
		slog.String("app", app.Name),
		slog.String("branch", branch),
		slog.String("commit", payload.After[:min(8, len(payload.After))]),
		slog.String("pusher", payload.Pusher.Name),
	)

	c.JSON(200, gin.H{
		"message":       "deployment triggered",
		"deployment_id": deploy.ID,
		"commit":        payload.After,
	})
}

// GitLab handles GitLab push webhook events.
// POST /api/v1/webhooks/gitlab/:appId (public, no JWT)
func (h *WebhookHandler) GitLab(c *gin.Context) {
	appID, err := uuid.Parse(c.Param("appId"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid app ID"})
		return
	}

	app, err := h.store.Applications().GetByID(c.Request.Context(), appID)
	if err != nil {
		c.JSON(404, gin.H{"error": "app not found"})
		return
	}

	if !app.AutoDeploy {
		c.JSON(200, gin.H{"message": "auto-deploy disabled"})
		return
	}

	// GitLab uses X-Gitlab-Token header for verification
	token := c.GetHeader("X-Gitlab-Token")
	if app.WebhookSecret != "" && token != app.WebhookSecret {
		c.JSON(401, gin.H{"error": "invalid token"})
		return
	}

	body, _ := io.ReadAll(io.LimitReader(c.Request.Body, 10<<20))

	var payload struct {
		Ref     string `json:"ref"`
		After   string `json:"after"`
		Project struct {
			Name string `json:"name"`
		} `json:"project"`
		UserUsername string `json:"user_username"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		c.JSON(400, gin.H{"error": "invalid payload"})
		return
	}

	branch := strings.TrimPrefix(payload.Ref, "refs/heads/")
	if branch != app.GitBranch {
		c.JSON(200, gin.H{"message": fmt.Sprintf("ignored push to %s", branch)})
		return
	}

	input := service.TriggerDeployInput{
		AppID:       appID,
		TriggerType: "webhook",
	}

	deploy, err := h.deploySvc.Trigger(c.Request.Context(), input)
	if err != nil {
		c.JSON(500, gin.H{"error": "deploy failed"})
		return
	}

	if payload.After != "" {
		deploy.CommitSHA = payload.After
		_ = h.store.Deployments().Update(c.Request.Context(), deploy)
	}

	h.logger.Info("gitlab webhook triggered deploy",
		slog.String("app", app.Name),
		slog.String("branch", branch),
		slog.String("commit", payload.After[:min(8, len(payload.After))]),
	)

	c.JSON(200, gin.H{"message": "deployment triggered", "deployment_id": deploy.ID})
}

// matchPath checks whether a file path matches a glob-like watch pattern.
// Supports patterns like "apps/api/**", "*.go", "src/", etc.
func matchPath(file, pattern string) bool {
	// Trailing slash means "anything under this directory"
	if strings.HasSuffix(pattern, "/") {
		return strings.HasPrefix(file, pattern)
	}
	// "dir/**" matches anything under dir/
	if prefix, ok := strings.CutSuffix(pattern, "/**"); ok {
		return strings.HasPrefix(file, prefix+"/")
	}
	// Try exact filepath.Match (handles *, ?)
	if matched, _ := filepath.Match(pattern, file); matched {
		return true
	}
	// Also match against just the filename for patterns like "*.go"
	if matched, _ := filepath.Match(pattern, filepath.Base(file)); matched {
		return true
	}
	// Simple prefix match for directory patterns like "apps/api"
	if !strings.Contains(pattern, "*") && !strings.Contains(pattern, "?") {
		return strings.HasPrefix(file, pattern+"/") || file == pattern
	}
	return false
}

func verifyGitHubSignature(secret string, payload []byte, signature string) bool {
	sig := strings.TrimPrefix(signature, "sha256=")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(sig))
}
