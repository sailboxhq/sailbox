package service

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"

	"github.com/sailboxhq/sailbox/apps/api/internal/apierr"
	"github.com/sailboxhq/sailbox/apps/api/internal/model"
	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

type ResourceService struct {
	store  store.Store
	logger *slog.Logger
}

func NewResourceService(s store.Store, logger *slog.Logger) *ResourceService {
	return &ResourceService{store: s, logger: logger}
}

type CreateResourceInput struct {
	Name     string             `json:"name"`
	Type     model.ResourceType `json:"type" binding:"required"`
	Provider string             `json:"provider"`
	Config   json.RawMessage    `json:"config"`
}

func (s *ResourceService) Create(ctx context.Context, orgID uuid.UUID, input CreateResourceInput) (*model.SharedResource, error) {
	if input.Name == "" {
		input.Name = s.autoName(input)
	}

	resource := &model.SharedResource{
		OrgID:    orgID,
		Name:     input.Name,
		Type:     input.Type,
		Provider: input.Provider,
		Config:   input.Config,
		Status:   "active",
	}
	if err := s.store.SharedResources().Create(ctx, resource); err != nil {
		return nil, err
	}
	return resource, nil
}

// GenerateSSHKey creates a new SSH key pair and stores it as a shared resource.
func (s *ResourceService) GenerateSSHKey(ctx context.Context, orgID uuid.UUID, algorithm string, name string) (*model.SharedResource, error) {
	var privateKeyPEM []byte
	var publicKeyStr string

	switch algorithm {
	case "ed25519":
		pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("failed to generate ed25519 key: %w", err)
		}
		privPEM, err := ssh.MarshalPrivateKey(privKey, "")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal private key: %w", err)
		}
		privateKeyPEM = pem.EncodeToMemory(privPEM)
		sshPub, err := ssh.NewPublicKey(pubKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create public key: %w", err)
		}
		publicKeyStr = strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPub)))

	case "rsa-4096":
		privKey, err := rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			return nil, fmt.Errorf("failed to generate RSA key: %w", err)
		}
		privPEM, err := ssh.MarshalPrivateKey(privKey, "")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal private key: %w", err)
		}
		privateKeyPEM = pem.EncodeToMemory(privPEM)
		sshPub, err := ssh.NewPublicKey(&privKey.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create public key: %w", err)
		}
		publicKeyStr = strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPub)))

	default:
		return nil, fmt.Errorf("unsupported algorithm: %s (use ed25519 or rsa-4096)", algorithm)
	}

	if name == "" {
		// Generate a unique short ID from the public key fingerprint
		shortID := make([]byte, 3)
		_, _ = rand.Read(shortID)
		name = fmt.Sprintf("key-%s-%x", time.Now().Format("0102"), shortID)
	}

	config, _ := json.Marshal(map[string]string{
		"private_key": string(privateKeyPEM),
		"public_key":  publicKeyStr,
		"algorithm":   algorithm,
	})

	resource := &model.SharedResource{
		OrgID:    orgID,
		Name:     name,
		Type:     model.ResourceSSHKey,
		Provider: algorithm,
		Config:   config,
		Status:   "active",
	}

	if err := s.store.SharedResources().Create(ctx, resource); err != nil {
		return nil, err
	}

	s.logger.Info("generated SSH key", slog.String("name", name), slog.String("algorithm", algorithm))
	return resource, nil
}

// shortHex returns a random hex suffix for unique naming.
func shortHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// autoName generates a unique, descriptive name for a resource.
//
// Naming convention: {provider}-{identifier}-{MMDD}-{hex}
// Examples:
//
//	github-alan-0325-a1f3
//	dockerhub-myuser-0325-b2e4
//	key-ed25519-0325-c3d5
//	r2-my-bucket-0325-d4e6
func (s *ResourceService) autoName(input CreateResourceInput) string {
	dateSuffix := time.Now().Format("0102")
	hex := shortHex(2)
	provider := strings.ToLower(input.Provider)
	if provider == "" {
		provider = "custom"
	}

	switch input.Type {
	case model.ResourceGitProvider:
		var cfg struct {
			Username string `json:"username"`
		}
		if input.Config != nil {
			_ = json.Unmarshal(input.Config, &cfg)
		}
		if cfg.Username != "" {
			return fmt.Sprintf("%s-%s-%s-%s", provider, cfg.Username, dateSuffix, hex)
		}
		return fmt.Sprintf("%s-git-%s-%s", provider, dateSuffix, hex)

	case model.ResourceRegistry:
		var cfg struct {
			Username string `json:"username"`
		}
		if input.Config != nil {
			_ = json.Unmarshal(input.Config, &cfg)
		}
		if cfg.Username != "" {
			return fmt.Sprintf("%s-%s-%s-%s", provider, cfg.Username, dateSuffix, hex)
		}
		return fmt.Sprintf("%s-registry-%s-%s", provider, dateSuffix, hex)

	case model.ResourceSSHKey:
		return fmt.Sprintf("key-%s-%s-%s", provider, dateSuffix, hex)

	case model.ResourceObjectStorage:
		var cfg struct {
			Bucket string `json:"bucket"`
		}
		if input.Config != nil {
			_ = json.Unmarshal(input.Config, &cfg)
		}
		if cfg.Bucket != "" {
			return fmt.Sprintf("%s-%s-%s-%s", provider, cfg.Bucket, dateSuffix, hex)
		}
		return fmt.Sprintf("%s-storage-%s-%s", provider, dateSuffix, hex)

	default:
		return fmt.Sprintf("resource-%s-%s", dateSuffix, hex)
	}
}

func (s *ResourceService) GetByID(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
	return s.store.SharedResources().GetByID(ctx, id)
}

func (s *ResourceService) List(ctx context.Context, orgID uuid.UUID, resourceType string) ([]model.SharedResource, error) {
	return s.store.SharedResources().ListByOrg(ctx, orgID, resourceType)
}

type UpdateResourceInput struct {
	Name     *string          `json:"name"`
	Provider *string          `json:"provider"`
	Config   *json.RawMessage `json:"config"`
}

// getOwnedResource fetches a resource and verifies it belongs to the given org.
func (s *ResourceService) getOwnedResource(ctx context.Context, orgID, id uuid.UUID) (*model.SharedResource, error) {
	resource, err := s.store.SharedResources().GetByID(ctx, id)
	if err != nil {
		return nil, apierr.ErrNotFound.WithDetail("resource not found")
	}
	if resource.OrgID != orgID {
		return nil, apierr.ErrNotFound.WithDetail("resource not found")
	}
	return resource, nil
}

func (s *ResourceService) Update(ctx context.Context, orgID, id uuid.UUID, input UpdateResourceInput) (*model.SharedResource, error) {
	resource, err := s.getOwnedResource(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	if input.Name != nil {
		resource.Name = *input.Name
	}
	if input.Provider != nil {
		resource.Provider = *input.Provider
	}
	if input.Config != nil {
		resource.Config = *input.Config
	}
	if err := s.store.SharedResources().Update(ctx, resource); err != nil {
		return nil, err
	}
	return resource, nil
}

func (s *ResourceService) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	// Verify ownership
	if _, err := s.getOwnedResource(ctx, orgID, id); err != nil {
		return err
	}

	// Check applications referencing as git provider (fail-closed: abort on query error)
	apps, _, appErr := s.store.Applications().ListAll(ctx, store.ListParams{Page: 1, PerPage: 10000}, store.AppListFilter{})
	if appErr != nil {
		return fmt.Errorf("cannot verify resource references: %w", appErr)
	}
	for _, app := range apps {
		if app.GitProviderID != nil && *app.GitProviderID == id {
			return fmt.Errorf("resource is in use by application %q as git provider", app.Name)
		}
	}

	// Check server nodes referencing as SSH key
	nodes, nodeErr := s.store.ServerNodes().List(ctx)
	if nodeErr != nil {
		return fmt.Errorf("cannot verify resource references: %w", nodeErr)
	}
	for _, n := range nodes {
		if n.SSHKeyID != nil && *n.SSHKeyID == id {
			return fmt.Errorf("resource is in use by node %q as SSH key", n.Name)
		}
	}

	// Check managed databases referencing as backup S3
	projects, _, projErr := s.store.Projects().ListByOrg(ctx, orgID, store.ListParams{Page: 1, PerPage: 10000})
	if projErr != nil {
		return fmt.Errorf("cannot verify resource references: %w", projErr)
	}
	for _, p := range projects {
		dbs, _, dbErr := s.store.ManagedDatabases().ListByProject(ctx, p.ID, store.ListParams{Page: 1, PerPage: 10000})
		if dbErr != nil {
			return fmt.Errorf("cannot verify resource references: %w", dbErr)
		}
		for _, db := range dbs {
			if db.BackupS3ID != nil && *db.BackupS3ID == id {
				return fmt.Errorf("resource is in use by database %q as backup storage", db.Name)
			}
		}
	}

	// Check system backup settings
	s3IDStr, settErr := s.store.Settings().Get(ctx, "system_backup_s3_id")
	if settErr == nil && s3IDStr == id.String() {
		return fmt.Errorf("resource is in use as system backup storage — change backup config first")
	}

	return s.store.SharedResources().Delete(ctx, id)
}

// TestConnection validates the credentials for a shared resource.
func (s *ResourceService) TestConnection(ctx context.Context, orgID, id uuid.UUID) (bool, string, error) {
	resource, err := s.getOwnedResource(ctx, orgID, id)
	if err != nil {
		return false, "", err
	}

	switch resource.Type {
	case model.ResourceGitProvider:
		return s.testGitProvider(resource)
	case model.ResourceRegistry:
		return s.testRegistry(resource)
	case model.ResourceSSHKey:
		return true, "SSH key stored", nil // SSH keys are validated on use
	case model.ResourceObjectStorage:
		return s.testObjectStorage(resource)
	default:
		return false, "unknown resource type", nil
	}
}

func (s *ResourceService) testGitProvider(resource *model.SharedResource) (bool, string, error) {
	var config struct {
		Token    string `json:"token"`
		APIURL   string `json:"api_url"`
		Username string `json:"username"`
	}
	if err := json.Unmarshal(resource.Config, &config); err != nil {
		return false, "invalid config", nil
	}

	// Test GitHub/GitLab API
	apiURL := config.APIURL
	if apiURL == "" {
		switch resource.Provider {
		case "github":
			apiURL = "https://api.github.com/user"
		case "gitlab":
			apiURL = "https://gitlab.com/api/v4/user"
		case "gitea":
			apiURL = fmt.Sprintf("%s/api/v1/user", config.APIURL)
		default:
			return false, "no API URL configured", nil
		}
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("Authorization", "Bearer "+config.Token)
	resp, err := client.Do(req)
	if err != nil {
		return false, "connection failed: " + err.Error(), nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == 200 {
		return true, "authenticated successfully", nil
	}
	return false, fmt.Sprintf("authentication failed (HTTP %d)", resp.StatusCode), nil
}

func (s *ResourceService) testRegistry(resource *model.SharedResource) (bool, string, error) {
	var config struct {
		URL      string `json:"url"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal(resource.Config, &config); err != nil {
		return false, "invalid config", nil
	}

	// Test registry v2 API
	registryURL := config.URL
	if registryURL == "" {
		registryURL = "https://registry-1.docker.io"
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("GET", registryURL+"/v2/", nil)
	if config.Username != "" {
		req.SetBasicAuth(config.Username, config.Password)
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, "connection failed: " + err.Error(), nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == 200 || resp.StatusCode == 401 {
		// 401 is expected for Docker Hub without specific scope
		return true, "registry reachable", nil
	}
	return false, fmt.Sprintf("registry error (HTTP %d)", resp.StatusCode), nil
}

func (s *ResourceService) testObjectStorage(resource *model.SharedResource) (bool, string, error) {
	var config struct {
		Endpoint  string `json:"endpoint"`
		Bucket    string `json:"bucket"`
		Region    string `json:"region"`
		AccessKey string `json:"access_key"`
		SecretKey string `json:"secret_key"`
	}
	if err := json.Unmarshal(resource.Config, &config); err != nil {
		return false, "invalid config", nil
	}
	if config.Endpoint == "" {
		return false, "endpoint is required", nil
	}
	if config.Bucket == "" {
		return false, "bucket is required", nil
	}

	// Use aws-cli to list the bucket with provided credentials (same as backup flow)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "aws", "s3", "ls",
		fmt.Sprintf("s3://%s/", config.Bucket),
		"--endpoint-url", config.Endpoint,
		"--max-items", "1",
	)
	cmd.Env = append(os.Environ(),
		"AWS_ACCESS_KEY_ID="+config.AccessKey,
		"AWS_SECRET_ACCESS_KEY="+config.SecretKey,
		"AWS_DEFAULT_REGION="+regionOrDefault(config.Region),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if msg == "" {
			msg = err.Error()
		}
		return false, "S3 connection failed: " + msg, nil
	}
	return true, "S3 bucket accessible", nil
}

func regionOrDefault(region string) string {
	if region != "" {
		return region
	}
	return "auto"
}

// GitRepo represents a repository from a git provider.
type GitRepo struct {
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	CloneURL      string `json:"clone_url"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
}

// ListRepos fetches repositories from a git provider using its stored token.
func (s *ResourceService) ListRepos(ctx context.Context, orgID, resourceID uuid.UUID) ([]GitRepo, error) {
	resource, err := s.getOwnedResource(ctx, orgID, resourceID)
	if err != nil {
		return nil, err
	}
	if resource.Type != model.ResourceGitProvider {
		return nil, fmt.Errorf("resource is not a git provider")
	}

	var config struct {
		Token        string `json:"token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresAt    string `json:"expires_at"`
		APIURL       string `json:"api_url"`
		Org          string `json:"org"`
	}
	if err := json.Unmarshal(resource.Config, &config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Auto-refresh GitHub token if expired
	if resource.Provider == "github" && config.RefreshToken != "" && config.ExpiresAt != "" {
		expiresAt, _ := time.Parse(time.RFC3339, config.ExpiresAt)
		if time.Now().After(expiresAt.Add(-5 * time.Minute)) { // refresh 5 min before expiry
			if newToken, err := s.refreshGitHubToken(ctx, resource, config.RefreshToken); err == nil {
				config.Token = newToken
			} else {
				s.logger.Error("failed to refresh GitHub token", slog.Any("error", err), slog.String("resource", resource.Name))
			}
		}
	}

	switch resource.Provider {
	case "github":
		repos, err := s.listGitHubRepos(config.Token, config.Org)
		if err != nil {
			return nil, fmt.Errorf("failed to list repositories: %w — the token may have expired, try reconnecting in Resources", err)
		}
		return repos, nil
	case "gitlab":
		apiURL := config.APIURL
		if apiURL == "" {
			apiURL = "https://gitlab.com"
		}
		return s.listGitLabRepos(config.Token, apiURL)
	case "gitea":
		if config.APIURL == "" {
			return nil, fmt.Errorf("gitea API URL not configured")
		}
		return s.listGiteaRepos(config.Token, config.APIURL)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", resource.Provider)
	}
}

// refreshGitHubToken uses the refresh_token to get a new access_token from GitHub.
func (s *ResourceService) refreshGitHubToken(ctx context.Context, resource *model.SharedResource, refreshToken string) (string, error) {
	clientID, _ := s.store.Settings().Get(ctx, "github_app_client_id")
	clientSecret, _ := s.store.Settings().Get(ctx, "github_app_client_secret")
	if clientID == "" || clientSecret == "" {
		return "", fmt.Errorf("GitHub App not configured")
	}

	data := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}

	req, _ := http.NewRequest("POST", "https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("refresh request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Error        string `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("invalid refresh response: %w", err)
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("refresh failed: %s", result.Error)
	}

	// Update stored config with new tokens
	var configMap map[string]string
	_ = json.Unmarshal(resource.Config, &configMap)
	if configMap == nil {
		configMap = make(map[string]string)
	}
	configMap["token"] = result.AccessToken
	if result.RefreshToken != "" {
		configMap["refresh_token"] = result.RefreshToken
	}
	if result.ExpiresIn > 0 {
		configMap["expires_at"] = time.Now().Add(time.Duration(result.ExpiresIn) * time.Second).UTC().Format(time.RFC3339)
	}
	newConfig, _ := json.Marshal(configMap)
	resource.Config = newConfig
	_ = s.store.SharedResources().Update(ctx, resource)

	s.logger.Info("GitHub token refreshed", slog.String("resource", resource.Name))
	return result.AccessToken, nil
}

func (s *ResourceService) listGitHubRepos(token string, org string) ([]GitRepo, error) {
	client := &http.Client{Timeout: 15 * time.Second}

	// Step 1: Find the GitHub App installation ID for this user/org.
	// This scopes repo listing to only repositories the user authorized.
	installationID, err := s.findGitHubInstallation(client, token, org)
	if err != nil || installationID == 0 {
		// Fallback: no installation found (e.g. classic OAuth token without App install).
		// Use /user/repos as before.
		return s.listGitHubReposFallback(client, token, org)
	}

	// Step 2: List repos via /user/installations/{id}/repositories — only returns authorized repos.
	var allRepos []GitRepo
	page := 1
	for {
		apiURL := fmt.Sprintf("https://api.github.com/user/installations/%d/repositories?per_page=100&page=%d", installationID, page)
		req, _ := http.NewRequest("GET", apiURL, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return nil, fmt.Errorf("GitHub token expired or revoked (HTTP %d) — please reconnect in Resources", resp.StatusCode)
		}

		var result struct {
			Repositories []struct {
				Name          string `json:"name"`
				FullName      string `json:"full_name"`
				CloneURL      string `json:"clone_url"`
				DefaultBranch string `json:"default_branch"`
				Private       bool   `json:"private"`
			} `json:"repositories"`
			TotalCount int `json:"total_count"`
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
		if err := json.Unmarshal(body, &result); err != nil {
			return allRepos, nil
		}
		for _, r := range result.Repositories {
			allRepos = append(allRepos, GitRepo{
				Name:          r.Name,
				FullName:      r.FullName,
				CloneURL:      r.CloneURL,
				DefaultBranch: r.DefaultBranch,
				Private:       r.Private,
			})
		}
		if len(result.Repositories) < 100 {
			break
		}
		page++
		if page > 10 {
			break
		}
	}
	return allRepos, nil
}

// findGitHubInstallation queries /user/installations to find the App installation ID.
// If org is set, it looks for the installation matching that org account.
func (s *ResourceService) findGitHubInstallation(client *http.Client, token, org string) (int64, error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/user/installations?per_page=100", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Installations []struct {
			ID      int64 `json:"id"`
			Account struct {
				Login string `json:"login"`
			} `json:"account"`
		} `json:"installations"`
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}

	for _, inst := range result.Installations {
		if org != "" && strings.EqualFold(inst.Account.Login, org) {
			return inst.ID, nil
		}
		if org == "" {
			// Return the first installation (user's own)
			return inst.ID, nil
		}
	}
	return 0, nil
}

// listGitHubReposFallback is used when no App installation is found (classic OAuth).
func (s *ResourceService) listGitHubReposFallback(client *http.Client, token, org string) ([]GitRepo, error) {
	var allRepos []GitRepo
	page := 1
	for {
		var apiURL string
		if org != "" {
			apiURL = fmt.Sprintf("https://api.github.com/user/repos?per_page=100&page=%d&sort=updated&affiliation=organization_member", page)
		} else {
			apiURL = fmt.Sprintf("https://api.github.com/user/repos?per_page=100&page=%d&sort=updated&affiliation=owner,collaborator", page)
		}
		req, _ := http.NewRequest("GET", apiURL, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return nil, fmt.Errorf("GitHub token expired or revoked (HTTP %d) — please reconnect in Resources", resp.StatusCode)
		}

		var repos []struct {
			Name          string `json:"name"`
			FullName      string `json:"full_name"`
			CloneURL      string `json:"clone_url"`
			DefaultBranch string `json:"default_branch"`
			Private       bool   `json:"private"`
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
		if err := json.Unmarshal(body, &repos); err != nil {
			return allRepos, nil
		}
		for _, r := range repos {
			if org != "" && !strings.HasPrefix(r.FullName, org+"/") {
				continue
			}
			allRepos = append(allRepos, GitRepo{
				Name:          r.Name,
				FullName:      r.FullName,
				CloneURL:      r.CloneURL,
				DefaultBranch: r.DefaultBranch,
				Private:       r.Private,
			})
		}
		if len(repos) < 100 {
			break
		}
		page++
		if page > 10 {
			break
		}
	}
	return allRepos, nil
}

func (s *ResourceService) listGitLabRepos(token, apiURL string) ([]GitRepo, error) {
	req, _ := http.NewRequest("GET", apiURL+"/api/v4/projects?membership=true&per_page=100&order_by=updated_at", nil)
	req.Header.Set("PRIVATE-TOKEN", token)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var projects []struct {
		Name              string `json:"name"`
		PathWithNamespace string `json:"path_with_namespace"`
		HTTPURLToRepo     string `json:"http_url_to_repo"`
		DefaultBranch     string `json:"default_branch"`
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	_ = json.Unmarshal(body, &projects)

	var repos []GitRepo
	for _, p := range projects {
		repos = append(repos, GitRepo{
			Name:          p.Name,
			FullName:      p.PathWithNamespace,
			CloneURL:      p.HTTPURLToRepo,
			DefaultBranch: p.DefaultBranch,
		})
	}
	return repos, nil
}

func (s *ResourceService) listGiteaRepos(token, apiURL string) ([]GitRepo, error) {
	req, _ := http.NewRequest("GET", apiURL+"/api/v1/user/repos?limit=50", nil)
	req.Header.Set("Authorization", "token "+token)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var giteaRepos []struct {
		Name          string `json:"name"`
		FullName      string `json:"full_name"`
		CloneURL      string `json:"clone_url"`
		DefaultBranch string `json:"default_branch"`
		Private       bool   `json:"private"`
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	_ = json.Unmarshal(body, &giteaRepos)

	var repos []GitRepo
	for _, r := range giteaRepos {
		repos = append(repos, GitRepo{
			Name:          r.Name,
			FullName:      r.FullName,
			CloneURL:      r.CloneURL,
			DefaultBranch: r.DefaultBranch,
			Private:       r.Private,
		})
	}
	return repos, nil
}
