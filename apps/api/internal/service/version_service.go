package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	appVersion "github.com/sailboxhq/sailbox/apps/api/internal/version"
)

type VersionInfo struct {
	Current     string `json:"current"`
	Latest      string `json:"latest"`
	UpdateAvail bool   `json:"update_available"`
	ReleaseURL  string `json:"release_url"`
	Changelog   string `json:"changelog"`
	PublishedAt string `json:"published_at"`
}

type VersionService struct {
	logger    *slog.Logger
	mu        sync.RWMutex
	cached    *VersionInfo
	checkedAt time.Time
}

func NewVersionService(logger *slog.Logger) *VersionService {
	svc := &VersionService{logger: logger}
	go svc.periodicCheck()
	return svc
}

func (s *VersionService) GetVersionInfo(ctx context.Context) *VersionInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.cached != nil {
		return s.cached
	}

	// Return basic info if no cache yet
	return &VersionInfo{
		Current: appVersion.Version,
		Latest:  appVersion.Version,
	}
}

func (s *VersionService) periodicCheck() {
	// Check immediately on startup
	s.checkForUpdate()

	// Then check every hour
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		s.checkForUpdate()
	}
}

func (s *VersionService) checkForUpdate() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest",
		appVersion.GitHubOwner, appVersion.GitHubRepo)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		s.logger.Debug("version check: failed to create request", slog.Any("error", err))
		s.setCached(appVersion.Version, "", false)
		return
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := notifHTTPClient.Do(req) // reuses shared 30s timeout client
	if err != nil {
		s.logger.Debug("version check: request failed", slog.Any("error", err))
		s.setCached(appVersion.Version, "", false)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		s.logger.Debug("version check: non-200 response", slog.Int("status", resp.StatusCode))
		s.setCached(appVersion.Version, "", false)
		return
	}

	var release struct {
		TagName     string `json:"tag_name"`
		HTMLURL     string `json:"html_url"`
		Body        string `json:"body"`
		PublishedAt string `json:"published_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		s.logger.Debug("version check: failed to decode", slog.Any("error", err))
		s.setCached(appVersion.Version, "", false)
		return
	}

	latest := release.TagName
	// Strip leading 'v' for comparison
	currentClean := strings.TrimPrefix(appVersion.Version, "v")
	latestClean := strings.TrimPrefix(latest, "v")

	updateAvail := latestClean != currentClean && appVersion.Version != "dev"

	s.mu.Lock()
	s.cached = &VersionInfo{
		Current:     appVersion.Version,
		Latest:      latest,
		UpdateAvail: updateAvail,
		ReleaseURL:  release.HTMLURL,
		Changelog:   release.Body,
		PublishedAt: release.PublishedAt,
	}
	s.checkedAt = time.Now()
	s.mu.Unlock()

	if updateAvail {
		s.logger.Info("new version available", slog.String("current", appVersion.Version), slog.String("latest", latest))
	}
}

func (s *VersionService) setCached(current, latest string, updateAvail bool) {
	s.mu.Lock()
	s.cached = &VersionInfo{
		Current:     current,
		Latest:      latest,
		UpdateAvail: updateAvail,
	}
	s.checkedAt = time.Now()
	s.mu.Unlock()
}
