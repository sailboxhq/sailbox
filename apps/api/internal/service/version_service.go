package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	appVersion "github.com/sailboxhq/sailbox/apps/api/internal/version"
)

const upgradeStatusFile = "/opt/sailbox/upgrade_status.json"

type VersionInfo struct {
	Current     string `json:"current"`
	Latest      string `json:"latest"`
	UpdateAvail bool   `json:"update_available"`
	ReleaseURL  string `json:"release_url"`
	Changelog   string `json:"changelog"`
	PublishedAt string `json:"published_at"`
}

type UpgradeStatus struct {
	Status  string `json:"status"` // idle, upgrading, done, error
	Message string `json:"message"`
}

type VersionService struct {
	logger    *slog.Logger
	mu        sync.RWMutex
	cached    *VersionInfo
	upgradeMu sync.Mutex
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

	return &VersionInfo{
		Current: appVersion.Version,
		Latest:  appVersion.Version,
	}
}

func (s *VersionService) periodicCheck() {
	s.checkForUpdate()
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

	resp, err := notifHTTPClient.Do(req)
	if err != nil {
		s.logger.Debug("version check: request failed", slog.Any("error", err))
		s.setCached(appVersion.Version, "", false)
		return
	}
	defer func() { _ = resp.Body.Close() }()

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
	updateAvail := appVersion.Version != "dev" && shouldUpdate(latest, appVersion.Version)

	s.mu.Lock()
	s.cached = &VersionInfo{
		Current:     appVersion.Version,
		Latest:      latest,
		UpdateAvail: updateAvail,
		ReleaseURL:  release.HTMLURL,
		Changelog:   release.Body,
		PublishedAt: release.PublishedAt,
	}
	s.mu.Unlock()

	if updateAvail {
		s.logger.Info("new version available", slog.String("current", appVersion.Version), slog.String("latest", latest))
	}
}

func shouldUpdate(latest, current string) bool {
	latestClean := strings.TrimPrefix(latest, "v")
	currentClean := strings.TrimPrefix(current, "v")

	if latestClean == currentClean {
		return false
	}

	if strings.Contains(currentClean, "-") {
		currentBase := currentClean[:strings.IndexByte(currentClean, '-')]
		latestBase := latestClean
		if idx := strings.IndexByte(latestBase, '-'); idx != -1 {
			latestBase = latestBase[:idx]
		}
		if currentBase == latestBase && !strings.Contains(latestClean, "-") {
			return true
		}
	}

	return isNewer(latest, current)
}

func isNewer(latest, current string) bool {
	parse := func(v string) (int, int, int) {
		v = strings.TrimPrefix(v, "v")
		if idx := strings.IndexByte(v, '-'); idx != -1 {
			v = v[:idx]
		}
		parts := strings.SplitN(v, ".", 3)
		a, _ := strconv.Atoi(parts[0])
		b, c := 0, 0
		if len(parts) > 1 {
			b, _ = strconv.Atoi(parts[1])
		}
		if len(parts) > 2 {
			c, _ = strconv.Atoi(parts[2])
		}
		return a, b, c
	}
	la, lb, lc := parse(latest)
	ca, cb, cc := parse(current)
	if la != ca {
		return la > ca
	}
	if lb != cb {
		return lb > cb
	}
	return lc > cc
}

// ============================================================================
// Upgrade — spawns a one-shot upgrader container that runs upgrade-lib.sh
// ============================================================================

// GetUpgradeStatus reads the shared status file written by the upgrader.
func (s *VersionService) GetUpgradeStatus() UpgradeStatus {
	data, err := os.ReadFile(upgradeStatusFile)
	if err != nil {
		return UpgradeStatus{Status: "idle"}
	}
	var st UpgradeStatus
	if err := json.Unmarshal(data, &st); err != nil {
		return UpgradeStatus{Status: "idle"}
	}
	return st
}

// ClearUpgradeStatus resets the status file after frontend acknowledges.
func (s *VersionService) ClearUpgradeStatus() {
	_ = os.Remove(upgradeStatusFile)
}

// TriggerUpgrade spawns a one-shot "upgrader" container that runs the shared
// upgrade-lib.sh script. The upgrader is an independent process — it survives
// the main sailbox container being replaced, so it can do health checks and rollback.
func (s *VersionService) TriggerUpgrade() error {
	s.upgradeMu.Lock()
	defer s.upgradeMu.Unlock()

	// Check if upgrader container is actually running (not just status file)
	if out, err := exec.Command("docker", "inspect", "-f", "{{.State.Running}}", "sailbox-upgrader").Output(); err == nil {
		if strings.TrimSpace(string(out)) == "true" {
			return fmt.Errorf("upgrade already in progress (upgrader container is running)")
		}
		// Container exists but not running — remove it
		_ = exec.Command("docker", "rm", "-f", "sailbox-upgrader").Run()
	}

	// Also check status file as secondary guard
	current := s.GetUpgradeStatus()
	if current.Status == "upgrading" {
		return fmt.Errorf("upgrade already in progress")
	}

	// Preflight: verify docker.sock
	if _, err := os.Stat("/var/run/docker.sock"); err != nil {
		return fmt.Errorf("docker socket not mounted — add /var/run/docker.sock volume to the sailbox container")
	}
	// Preflight: verify upgrade-lib.sh exists
	if _, err := os.Stat("/opt/sailbox/upgrade-lib.sh"); err != nil {
		return fmt.Errorf("upgrade-lib.sh not found at /opt/sailbox — ensure /opt/sailbox is mounted")
	}

	// Write initial status
	if err := writeUpgradeStatus(UpgradeStatus{Status: "upgrading", Message: "Starting upgrade..."}); err != nil {
		return fmt.Errorf("cannot persist upgrade state: %w", err)
	}

	// Spawn upgrader container:
	// - docker:cli image has docker + docker compose + curl + sh
	// - Host network for healthz checks on localhost:3000
	// - Mounts docker.sock (to manage containers) and /opt/sailbox (shared state + scripts)
	// - 10 minute timeout via --stop-timeout
	// - Runs the same upgrade-lib.sh that upgrade.sh uses
	cmd := exec.Command("docker", "run", "-d", "--rm",
		"--name", "sailbox-upgrader",
		"--network", "host",
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		"-v", "/opt/sailbox:/opt/sailbox",
		"docker:cli",
		"sh", "-c", ". /opt/sailbox/upgrade-lib.sh && run_upgrade container",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := fmt.Sprintf("Failed to start upgrader: %s", strings.TrimSpace(string(out)))
		s.logger.Error("upgrade: "+msg, slog.Any("error", err))
		_ = writeUpgradeStatus(UpgradeStatus{Status: "error", Message: msg})
		return fmt.Errorf("%s", msg)
	}

	containerID := strings.TrimSpace(string(out))
	s.logger.Info("upgrade: upgrader container started", slog.String("container_id", containerID[:12]))
	return nil
}

func writeUpgradeStatus(st UpgradeStatus) error {
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return os.WriteFile(upgradeStatusFile, data, 0o644)
}

func (s *VersionService) setCached(current, latest string, updateAvail bool) {
	s.mu.Lock()
	s.cached = &VersionInfo{
		Current:     current,
		Latest:      latest,
		UpdateAvail: updateAvail,
	}
	s.mu.Unlock()
}
