package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/sailboxhq/sailbox/apps/api/internal/model"
	"github.com/sailboxhq/sailbox/apps/api/internal/orchestrator"
	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

type AppService struct {
	store     store.Store
	orch      orchestrator.Orchestrator
	logger    *slog.Logger
	domainSvc *DomainService
}

func NewAppService(s store.Store, orch orchestrator.Orchestrator, logger *slog.Logger, domainSvc *DomainService) *AppService {
	return &AppService{store: s, orch: orch, logger: logger, domainSvc: domainSvc}
}

type CreateAppInput struct {
	ProjectID     uuid.UUID           `json:"project_id" binding:"required"`
	Name          string              `json:"name" binding:"required,min=1,max=63"`
	SourceType    model.SourceType    `json:"source_type" binding:"required,oneof=git image"`
	GitRepo       string              `json:"git_repo" binding:"required_if=SourceType git"`
	GitBranch     string              `json:"git_branch"`
	GitProviderID *uuid.UUID          `json:"git_provider_id"`
	DockerImage   string              `json:"docker_image" binding:"required_if=SourceType image"`
	BuildType     model.BuildType     `json:"build_type"`
	Dockerfile    string              `json:"dockerfile"`
	Replicas      int32               `json:"replicas"`
	CPULimit      string              `json:"cpu_limit"`
	MemLimit      string              `json:"mem_limit"`
	EnvVars       map[string]string   `json:"env_vars"`
	Ports         []model.PortMapping `json:"ports"`
}

func (s *AppService) Create(ctx context.Context, input CreateAppInput) (*model.Application, error) {
	app := &model.Application{
		ProjectID:     input.ProjectID,
		Name:          input.Name,
		SourceType:    input.SourceType,
		GitRepo:       input.GitRepo,
		GitBranch:     input.GitBranch,
		GitProviderID: input.GitProviderID,
		DockerImage:   input.DockerImage,
		BuildType:     input.BuildType,
		Dockerfile:    input.Dockerfile,
		Replicas:      input.Replicas,
		CPULimit:      input.CPULimit,
		MemLimit:      input.MemLimit,
		EnvVars:       input.EnvVars,
		Ports:         input.Ports,
		Status:        model.AppStatusIdle,
	}

	// Apply defaults
	if app.GitBranch == "" {
		app.GitBranch = "main"
	}
	if app.Replicas == 0 {
		app.Replicas = 1
	}
	if app.CPULimit == "" {
		app.CPULimit = "500m"
	}
	if app.MemLimit == "" {
		app.MemLimit = "512Mi"
	}
	if app.BuildType == "" && app.SourceType == model.SourceGit {
		app.BuildType = model.BuildDockerfile
	}
	if app.Dockerfile == "" {
		app.Dockerfile = "Dockerfile"
	}

	// Default container port based on common images
	if len(app.Ports) == 0 {
		port := guessContainerPort(app.DockerImage)
		app.Ports = []model.PortMapping{{ContainerPort: port, ServicePort: port, Protocol: "tcp"}}
	}

	// Look up project namespace — app requires a valid namespace
	project, err := s.store.Projects().GetByID(ctx, input.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("project not found: %w", err)
	}
	if project.Namespace == "" {
		return nil, fmt.Errorf("project has no namespace configured")
	}
	app.Namespace = project.Namespace
	app.K8sName = sanitizeK8sName(app.Name)

	// Check for K8s name conflicts in the same project (apps + databases share namespace)
	existingApps, _, _ := s.store.Applications().ListByProject(ctx, input.ProjectID, store.ListParams{Page: 1, PerPage: 10000})
	for _, e := range existingApps {
		if sanitizeK8sName(e.Name) == app.K8sName {
			return nil, fmt.Errorf("an application with K8s name %q already exists (from %q)", app.K8sName, e.Name)
		}
	}
	existingDBs, _, _ := s.store.ManagedDatabases().ListByProject(ctx, input.ProjectID, store.ListParams{Page: 1, PerPage: 10000})
	for _, e := range existingDBs {
		if sanitizeK8sName(e.Name) == app.K8sName {
			return nil, fmt.Errorf("a database with K8s name %q already exists (from %q) — app and database names must not collide", app.K8sName, e.Name)
		}
	}

	// Validate git provider belongs to same org
	if app.GitProviderID != nil {
		res, err := s.store.SharedResources().GetByID(ctx, *app.GitProviderID)
		if err != nil {
			return nil, fmt.Errorf("git provider not found: %w", err)
		}
		if res.OrgID != project.OrgID {
			return nil, fmt.Errorf("git provider does not belong to this organization")
		}
	}

	if err := s.store.Applications().Create(ctx, app); err != nil {
		return nil, err
	}

	// Auto-generate default Traefik domain
	if s.domainSvc != nil {
		if domain, err := s.domainSvc.GenerateTraefikDomain(ctx, app.ID); err != nil {
			s.logger.Error("failed to auto-generate domain", slog.Any("error", err))
		} else {
			s.logger.Info("default domain generated", slog.String("host", domain.Host))
		}
	}

	s.logger.Info("application created",
		slog.String("name", app.Name),
		slog.String("id", app.ID.String()),
		slog.String("source", string(app.SourceType)),
	)
	return app, nil
}

// guessContainerPort returns a default port for common Docker images.
func guessContainerPort(image string) int {
	img := strings.ToLower(image)
	switch {
	case strings.Contains(img, "nginx"), strings.Contains(img, "httpd"), strings.Contains(img, "apache"):
		return 80
	case strings.Contains(img, "node"), strings.Contains(img, "next"), strings.Contains(img, "nuxt"):
		return 3000
	case strings.Contains(img, "rails"), strings.Contains(img, "puma"):
		return 3000
	case strings.Contains(img, "django"), strings.Contains(img, "flask"), strings.Contains(img, "uvicorn"):
		return 8000
	case strings.Contains(img, "spring"), strings.Contains(img, "tomcat"):
		return 8080
	case strings.Contains(img, "go"), strings.Contains(img, "gin"), strings.Contains(img, "fiber"):
		return 8080
	case strings.Contains(img, "postgres"):
		return 5432
	case strings.Contains(img, "mysql"), strings.Contains(img, "mariadb"):
		return 3306
	case strings.Contains(img, "redis"), strings.Contains(img, "valkey"):
		return 6379
	case strings.Contains(img, "mongo"):
		return 27017
	default:
		return 80
	}
}

type UpdateAppInput struct {
	// Source configuration
	GitRepo     *string `json:"git_repo"`
	GitBranch   *string `json:"git_branch"`
	DockerImage *string `json:"docker_image"`

	// Build configuration
	BuildType    *string           `json:"build_type"`
	Dockerfile   *string           `json:"dockerfile"`
	BuildContext *string           `json:"build_context"`
	BuildArgs    map[string]string `json:"build_args"`
	BuildEnvVars map[string]string `json:"build_env_vars"`
	WatchPaths   []string          `json:"watch_paths"`
	NoCache      *bool             `json:"no_cache"`

	// Runtime configuration
	CPULimit   *string             `json:"cpu_limit"`
	MemLimit   *string             `json:"mem_limit"`
	CPURequest *string             `json:"cpu_request"`
	MemRequest *string             `json:"mem_request"`
	Ports      []model.PortMapping `json:"ports"`
	NodePool   *string             `json:"node_pool"`

	// Advanced configuration
	HealthCheck            *model.HealthCheck          `json:"health_check"`
	Autoscaling            *model.AutoscalingConfig    `json:"autoscaling"`
	Volumes                []model.VolumeMount         `json:"volumes"`
	DeployStrategy         *string                     `json:"deploy_strategy"`
	DeployStrategyConfig   *model.DeployStrategyConfig `json:"deploy_strategy_config"`
	TerminationGracePeriod *int                        `json:"termination_grace_period"`
}

func (s *AppService) Update(ctx context.Context, id uuid.UUID, input UpdateAppInput) (*model.Application, error) {
	app, err := s.store.Applications().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Track whether runtime-affecting fields changed (need K8s deployment update)
	runtimeChanged := false

	// Apply source fields (prevent clearing required values)
	if input.GitRepo != nil {
		if app.SourceType == model.SourceGit && strings.TrimSpace(*input.GitRepo) == "" {
			return nil, fmt.Errorf("git repo cannot be empty for git-based apps")
		}
		app.GitRepo = *input.GitRepo
	}
	if input.GitBranch != nil {
		if strings.TrimSpace(*input.GitBranch) == "" {
			return nil, fmt.Errorf("git branch cannot be empty")
		}
		app.GitBranch = *input.GitBranch
	}
	if input.DockerImage != nil {
		if app.SourceType == model.SourceImage && strings.TrimSpace(*input.DockerImage) == "" {
			return nil, fmt.Errorf("docker image cannot be empty for image-based apps")
		}
		app.DockerImage = *input.DockerImage
	}

	// Apply build fields
	if input.BuildType != nil {
		app.BuildType = model.BuildType(*input.BuildType)
	}
	if input.Dockerfile != nil {
		app.Dockerfile = *input.Dockerfile
	}
	if input.BuildContext != nil {
		app.BuildContext = *input.BuildContext
	}
	if input.BuildArgs != nil {
		app.BuildArgs = input.BuildArgs
	}
	if input.BuildEnvVars != nil {
		app.BuildEnvVars = input.BuildEnvVars
	}
	if input.WatchPaths != nil {
		app.WatchPaths = input.WatchPaths
	}
	if input.NoCache != nil {
		app.NoCache = *input.NoCache
	}

	// Apply runtime fields (these affect the live K8s deployment)
	if input.CPULimit != nil {
		app.CPULimit = *input.CPULimit
		runtimeChanged = true
	}
	if input.MemLimit != nil {
		app.MemLimit = *input.MemLimit
		runtimeChanged = true
	}
	if input.CPURequest != nil {
		app.CPURequest = *input.CPURequest
		runtimeChanged = true
	}
	if input.MemRequest != nil {
		app.MemRequest = *input.MemRequest
		runtimeChanged = true
	}
	if input.Ports != nil {
		app.Ports = input.Ports
		runtimeChanged = true
	}
	if input.NodePool != nil {
		app.NodePool = *input.NodePool
		runtimeChanged = true
	}
	if input.HealthCheck != nil {
		app.HealthCheck = input.HealthCheck
		runtimeChanged = true
	}
	if input.Autoscaling != nil {
		app.Autoscaling = input.Autoscaling
	}
	if input.Volumes != nil {
		app.Volumes = input.Volumes
		runtimeChanged = true
	}
	if input.DeployStrategy != nil {
		app.DeployStrategy = *input.DeployStrategy
		runtimeChanged = true
	}
	if input.DeployStrategyConfig != nil {
		app.DeployStrategyConfig = input.DeployStrategyConfig
		runtimeChanged = true
	}
	if input.TerminationGracePeriod != nil {
		app.TerminationGracePeriod = *input.TerminationGracePeriod
		runtimeChanged = true
	}

	// Server-side validation
	if app.HealthCheck != nil && app.HealthCheck.Type != "" {
		switch app.HealthCheck.Type {
		case "http":
			if app.HealthCheck.Path == "" {
				return nil, fmt.Errorf("health check path is required for HTTP probe")
			}
			if app.HealthCheck.Port <= 0 || app.HealthCheck.Port > 65535 {
				return nil, fmt.Errorf("health check port must be between 1 and 65535")
			}
		case "tcp":
			if app.HealthCheck.Port <= 0 || app.HealthCheck.Port > 65535 {
				return nil, fmt.Errorf("health check port must be between 1 and 65535")
			}
		case "exec":
			if app.HealthCheck.Command == "" {
				return nil, fmt.Errorf("health check command is required for exec probe")
			}
		default:
			return nil, fmt.Errorf("invalid health check type: %s", app.HealthCheck.Type)
		}
	}
	if app.TerminationGracePeriod < 0 {
		return nil, fmt.Errorf("termination grace period cannot be negative")
	}
	for _, p := range app.Ports {
		if p.ContainerPort <= 0 || p.ContainerPort > 65535 {
			return nil, fmt.Errorf("container port must be between 1 and 65535")
		}
		if p.Protocol != "" && p.Protocol != "tcp" && p.Protocol != "udp" {
			return nil, fmt.Errorf("port protocol must be tcp or udp")
		}
	}
	// Validate resource format (must be valid K8s quantities)
	for _, res := range []struct{ name, val string }{
		{"cpu_limit", app.CPULimit}, {"mem_limit", app.MemLimit},
		{"cpu_request", app.CPURequest}, {"mem_request", app.MemRequest},
	} {
		if res.val != "" {
			if _, err := resource.ParseQuantity(res.val); err != nil {
				return nil, fmt.Errorf("invalid %s %q: %w", res.name, res.val, err)
			}
		}
	}
	// Validate volumes
	volNames := make(map[string]bool)
	volMounts := make(map[string]bool)
	for _, vol := range app.Volumes {
		if vol.Name == "" {
			return nil, fmt.Errorf("volume name cannot be empty")
		}
		if volNames[vol.Name] {
			return nil, fmt.Errorf("duplicate volume name: %s", vol.Name)
		}
		volNames[vol.Name] = true
		if vol.MountPath == "" {
			return nil, fmt.Errorf("volume mount path cannot be empty")
		}
		if !strings.HasPrefix(vol.MountPath, "/") {
			return nil, fmt.Errorf("volume mount path must be absolute (start with /): %s", vol.MountPath)
		}
		if volMounts[vol.MountPath] {
			return nil, fmt.Errorf("duplicate volume mount path: %s", vol.MountPath)
		}
		volMounts[vol.MountPath] = true
		if vol.Size != "" {
			if _, err := resource.ParseQuantity(vol.Size); err != nil {
				return nil, fmt.Errorf("invalid volume size %q for %s: %w", vol.Size, vol.Name, err)
			}
		}
	}
	// Validate request <= limit
	if app.CPURequest != "" && app.CPULimit != "" {
		req, _ := resource.ParseQuantity(app.CPURequest)
		lim, _ := resource.ParseQuantity(app.CPULimit)
		if req.Cmp(lim) > 0 {
			return nil, fmt.Errorf("cpu_request (%s) cannot exceed cpu_limit (%s)", app.CPURequest, app.CPULimit)
		}
	}
	if app.MemRequest != "" && app.MemLimit != "" {
		req, _ := resource.ParseQuantity(app.MemRequest)
		lim, _ := resource.ParseQuantity(app.MemLimit)
		if req.Cmp(lim) > 0 {
			return nil, fmt.Errorf("mem_request (%s) cannot exceed mem_limit (%s)", app.MemRequest, app.MemLimit)
		}
	}
	// Validate autoscaling
	if app.Autoscaling != nil && app.Autoscaling.Enabled {
		if app.Autoscaling.MinReplicas <= 0 {
			return nil, fmt.Errorf("HPA min replicas must be at least 1")
		}
		if app.Autoscaling.MaxReplicas < app.Autoscaling.MinReplicas {
			return nil, fmt.Errorf("HPA max replicas must be >= min replicas")
		}
		if app.Autoscaling.CPUTarget <= 0 && app.Autoscaling.MemTarget <= 0 {
			return nil, fmt.Errorf("HPA requires at least one metric target (CPU or memory)")
		}
	}
	// Validate deploy strategy
	if app.DeployStrategy != "" && app.DeployStrategy != "rolling" && app.DeployStrategy != "recreate" {
		return nil, fmt.Errorf("deploy strategy must be 'rolling' or 'recreate'")
	}

	// Save to DB first, then apply to K8s — rollback DB on K8s failure
	oldApp, _ := s.store.Applications().GetByID(ctx, app.ID)
	if err := s.store.Applications().Update(ctx, app); err != nil {
		return nil, err
	}

	rollback := func(reason string, cause error) (*model.Application, error) {
		if oldApp != nil {
			_ = s.store.Applications().Update(ctx, oldApp)
		}
		return nil, fmt.Errorf("%s (settings rolled back, redeploy to reconcile): %w", reason, cause)
	}

	// Reconcile K8s resources if autoscaling changed
	if input.Autoscaling != nil {
		if input.Autoscaling.Enabled {
			if err := s.orch.ConfigureHPA(ctx, app, *input.Autoscaling); err != nil {
				return rollback("failed to apply HPA", err)
			}
		} else {
			if err := s.orch.DeleteHPA(ctx, app); err != nil {
				return rollback("failed to remove HPA", err)
			}
		}
	}

	// Redeploy if runtime-affecting fields changed and app is currently deployed
	if runtimeChanged && app.Status == model.AppStatusRunning {
		mergedEnv := make(map[string]string)
		project, projErr := s.store.Projects().GetByID(ctx, app.ProjectID)
		if projErr == nil && project.EnvVars != nil {
			for k, v := range project.EnvVars {
				mergedEnv[k] = v
			}
		}
		for k, v := range app.EnvVars {
			mergedEnv[k] = v
		}
		if err := s.orch.Deploy(ctx, app, orchestrator.DeployOpts{
			Image:                  app.DockerImage,
			Replicas:               app.Replicas,
			EnvVars:                mergedEnv,
			Ports:                  app.Ports,
			CPULimit:               app.CPULimit,
			MemLimit:               app.MemLimit,
			CPURequest:             app.CPURequest,
			MemRequest:             app.MemRequest,
			HealthCheck:            app.HealthCheck,
			Volumes:                app.Volumes,
			DeployStrategy:         app.DeployStrategy,
			DeployStrategyConfig:   app.DeployStrategyConfig,
			TerminationGracePeriod: app.TerminationGracePeriod,
			NodePool:               app.NodePool,
		}); err != nil {
			return rollback("failed to apply deployment changes", err)
		}
		// Persist metadata written back by orchestrator (e.g. PVC names)
		_ = s.store.Applications().Update(ctx, app)
	}

	s.logger.Info("application updated", slog.String("name", app.Name), slog.String("id", app.ID.String()))
	return app, nil
}

func (s *AppService) GetPodEvents(ctx context.Context, id uuid.UUID, podName string) ([]orchestrator.PodEvent, error) {
	app, err := s.store.Applications().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.orch.GetPodEvents(ctx, app, podName)
}

func (s *AppService) GetByID(ctx context.Context, id uuid.UUID) (*model.Application, error) {
	return s.store.Applications().GetByID(ctx, id)
}

func (s *AppService) List(ctx context.Context, projectID uuid.UUID, params store.ListParams) ([]model.Application, int, error) {
	apps, total, err := s.store.Applications().ListByProject(ctx, projectID, params)
	if err == nil {
		s.syncLiveStatuses(ctx, apps)
	}
	return apps, total, err
}

func (s *AppService) ListAll(ctx context.Context, params store.ListParams, filter store.AppListFilter) ([]model.Application, int, error) {
	apps, total, err := s.store.Applications().ListAll(ctx, params, filter)
	if err == nil {
		s.syncLiveStatuses(ctx, apps)
	}
	return apps, total, err
}

// syncLiveStatuses queries K8s for each app's real status and overwrites
// the in-memory status before returning to the caller. Also persists to DB
// if changed (triggers SSE for other clients).
func (s *AppService) syncLiveStatuses(ctx context.Context, apps []model.Application) {
	for i := range apps {
		if apps[i].Status == model.AppStatusIdle || apps[i].Status == model.AppStatusBuilding {
			continue
		}
		status, err := s.orch.GetStatus(ctx, &apps[i])
		if err != nil {
			continue
		}
		var live model.AppStatus
		switch status.Phase {
		case "running":
			live = model.AppStatusRunning
		case "pending":
			live = model.AppStatusDeploying
		case "stopped":
			live = model.AppStatusStopped
		case "failed":
			live = model.AppStatusError
		case "partial":
			live = model.AppStatusPartial
		}
		if live == "" {
			continue
		}
		// Always overwrite in-memory for accurate response
		if live != apps[i].Status {
			apps[i].Status = live
			// Persist to DB (async — don't block response for DB write)
			go func(id uuid.UUID, s2 model.AppStatus) {
				_ = s.store.Applications().UpdateStatus(context.Background(), id, s2)
			}(apps[i].ID, live)
		}
	}
}

func (s *AppService) Delete(ctx context.Context, id uuid.UUID) error {
	app, err := s.store.Applications().GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Delete K8s resources
	if err := s.orch.Delete(ctx, app); err != nil {
		s.logger.Error("failed to delete app from orchestrator", slog.Any("error", err), slog.String("app", app.Name))
	}

	// Delete associated Ingress resources
	domains, _ := s.store.Domains().ListByApp(ctx, id)
	for _, d := range domains {
		if err := s.orch.DeleteIngress(ctx, &d); err != nil {
			s.logger.Warn("failed to delete ingress", slog.String("domain", d.Host), slog.Any("error", err))
		}
	}

	return s.store.Applications().Delete(ctx, id)
}

func (s *AppService) Scale(ctx context.Context, id uuid.UUID, replicas int32) (*model.Application, error) {
	app, err := s.store.Applications().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Block manual scaling when HPA is active
	if app.Autoscaling != nil && app.Autoscaling.Enabled {
		return nil, fmt.Errorf("manual scaling is disabled while autoscaling (HPA) is active — disable HPA first")
	}

	if replicas < 0 || replicas > 100 {
		return nil, fmt.Errorf("replicas must be between 0 and 100")
	}

	if err := s.orch.Scale(ctx, app, replicas); err != nil {
		return nil, err
	}

	app.Replicas = replicas
	if err := s.store.Applications().Update(ctx, app); err != nil {
		return nil, err
	}
	return app, nil
}

func (s *AppService) UpdateEnvVars(ctx context.Context, id uuid.UUID, envVars map[string]string) (*model.Application, error) {
	app, err := s.store.Applications().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	app.EnvVars = envVars
	if err := s.store.Applications().Update(ctx, app); err != nil {
		return nil, err
	}

	// Push to running deployment only if app is actually deployed
	if app.Status == model.AppStatusRunning || app.Status == model.AppStatusPartial {
		mergedEnv := make(map[string]string)
		project, projErr := s.store.Projects().GetByID(ctx, app.ProjectID)
		if projErr == nil && project.EnvVars != nil {
			for k, v := range project.EnvVars {
				mergedEnv[k] = v
			}
		}
		for k, v := range app.EnvVars {
			mergedEnv[k] = v
		}
		if err := s.orch.UpdateEnvVars(ctx, app, mergedEnv); err != nil {
			s.logger.Error("failed to push env vars to deployment", slog.String("app", app.Name), slog.Any("error", err))
			return nil, fmt.Errorf("environment saved, but failed to apply to running deployment: %w", err)
		}
	}

	s.logger.Info("env vars updated", slog.String("app", app.Name), slog.Int("count", len(envVars)))
	return app, nil
}

func (s *AppService) GetStatus(ctx context.Context, id uuid.UUID) (*orchestrator.AppStatus, error) {
	app, err := s.store.Applications().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	status, err := s.orch.GetStatus(ctx, app)
	if err != nil {
		return nil, err
	}

	// Reconcile DB status with live K8s status.
	// Only update DB when K8s has settled into a stable state AND
	// the DB still holds a transitional status from a previous operation.
	// Reconcile DB status with actual K8s state
	var dbStatus model.AppStatus
	switch status.Phase {
	case "running":
		dbStatus = model.AppStatusRunning
	case "stopped":
		dbStatus = model.AppStatusStopped
	case "failed":
		dbStatus = model.AppStatusError
	case "not deployed":
		dbStatus = model.AppStatusStopped
	}
	// Update DB if K8s state differs from stored status
	if dbStatus != "" && dbStatus != app.Status {
		_ = s.store.Applications().UpdateStatus(ctx, id, dbStatus)
	}

	return status, nil
}

func (s *AppService) GetPods(ctx context.Context, id uuid.UUID) ([]orchestrator.PodInfo, error) {
	app, err := s.store.Applications().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.orch.GetPods(ctx, app)
}

func (s *AppService) Restart(ctx context.Context, id uuid.UUID) error {
	app, err := s.store.Applications().GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Only allow restart on running/partial apps
	switch app.Status {
	case model.AppStatusRunning, model.AppStatusPartial, model.AppStatusError:
		// OK
	default:
		return fmt.Errorf("cannot restart app in %s state", app.Status)
	}

	_ = s.store.Applications().UpdateStatus(ctx, id, model.AppStatusRestarting)

	if err := s.orch.Restart(ctx, app); err != nil {
		_ = s.store.Applications().UpdateStatus(ctx, id, model.AppStatusError)
		return err
	}

	return nil
}

func (s *AppService) Stop(ctx context.Context, id uuid.UUID) error {
	app, err := s.store.Applications().GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Only allow stop on running/partial/error apps
	switch app.Status {
	case model.AppStatusRunning, model.AppStatusPartial, model.AppStatusError:
		// OK
	default:
		return fmt.Errorf("cannot stop app in %s state", app.Status)
	}

	// Suspend HPA first to prevent it from scaling back up
	if app.Autoscaling != nil && app.Autoscaling.Enabled {
		if err := s.orch.DeleteHPA(ctx, app); err != nil {
			return fmt.Errorf("cannot stop: failed to remove HPA (app would be scaled back up): %w", err)
		}
	}

	_ = s.store.Applications().UpdateStatus(ctx, id, model.AppStatusStopping)

	if err := s.orch.Stop(ctx, app); err != nil {
		_ = s.store.Applications().UpdateStatus(ctx, id, model.AppStatusError)
		return err
	}
	return s.store.Applications().UpdateStatus(ctx, id, model.AppStatusStopped)
}

func (s *AppService) ClearBuildCache(ctx context.Context, id uuid.UUID) error {
	app, err := s.store.Applications().GetByID(ctx, id)
	if err != nil {
		return err
	}
	return s.orch.ClearBuildCache(ctx, app)
}

// ============================================================================
// Webhook Management
// ============================================================================

type WebhookConfig struct {
	WebhookURL string `json:"webhook_url"`
	Secret     string `json:"secret"`
	AutoDeploy bool   `json:"auto_deploy"`
}

func (s *AppService) EnableWebhook(ctx context.Context, id uuid.UUID, baseURL string) (*WebhookConfig, error) {
	app, err := s.store.Applications().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Generate webhook secret
	secret := "whsec_" + randomHex(16)
	app.WebhookSecret = secret
	app.AutoDeploy = true

	if err := s.store.Applications().Update(ctx, app); err != nil {
		return nil, err
	}

	// Determine webhook URL based on source type/provider
	provider := "github"
	if strings.Contains(app.GitRepo, "gitlab") {
		provider = "gitlab"
	}

	webhookURL := fmt.Sprintf("%s/api/v1/webhooks/%s/%s", baseURL, provider, app.ID)

	return &WebhookConfig{
		WebhookURL: webhookURL,
		Secret:     secret,
		AutoDeploy: true,
	}, nil
}

func (s *AppService) DisableWebhook(ctx context.Context, id uuid.UUID) error {
	app, err := s.store.Applications().GetByID(ctx, id)
	if err != nil {
		return err
	}
	app.AutoDeploy = false
	return s.store.Applications().Update(ctx, app)
}

func (s *AppService) RegenerateWebhookSecret(ctx context.Context, id uuid.UUID, baseURL string) (*WebhookConfig, error) {
	return s.EnableWebhook(ctx, id, baseURL)
}

func (s *AppService) GetWebhookConfig(ctx context.Context, id uuid.UUID, baseURL string) (*WebhookConfig, error) {
	app, err := s.store.Applications().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	provider := "github"
	if strings.Contains(app.GitRepo, "gitlab") {
		provider = "gitlab"
	}

	return &WebhookConfig{
		WebhookURL: fmt.Sprintf("%s/api/v1/webhooks/%s/%s", baseURL, provider, app.ID),
		Secret:     app.WebhookSecret,
		AutoDeploy: app.AutoDeploy,
	}, nil
}

// ============================================================================
// Secrets Management
// ============================================================================

func (s *AppService) GetSecretKeys(ctx context.Context, id uuid.UUID) ([]string, error) {
	app, err := s.store.Applications().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(app.Secrets))
	for k := range app.Secrets {
		keys = append(keys, k)
	}
	return keys, nil
}

func (s *AppService) UpdateSecrets(ctx context.Context, id uuid.UUID, secrets map[string]string) ([]string, error) {
	app, err := s.store.Applications().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// Save old secrets for rollback
	oldSecrets := make(map[string]string)
	for k, v := range app.Secrets {
		oldSecrets[k] = v
	}

	// Merge: only overwrite keys that have non-empty values; empty value = delete key
	if app.Secrets == nil {
		app.Secrets = make(map[string]string)
	}
	for k, v := range secrets {
		if v == "" {
			delete(app.Secrets, k)
		} else {
			app.Secrets[k] = v
		}
	}
	if err := s.store.Applications().Update(ctx, app); err != nil {
		return nil, err
	}

	// Create/update the K8s Secret — rollback DB on failure
	if err := s.orch.EnsureSecret(ctx, app, app.Secrets); err != nil {
		s.logger.Error("failed to ensure K8s secret", slog.Any("error", err), slog.String("app", app.Name))
		app.Secrets = oldSecrets
		_ = s.store.Applications().Update(ctx, app)
		return nil, fmt.Errorf("failed to apply secrets to cluster (rolled back): %w", err)
	}

	keys := make([]string, 0, len(app.Secrets))
	for k := range app.Secrets {
		keys = append(keys, k)
	}
	s.logger.Info("secrets updated", slog.String("app", app.Name), slog.Int("count", len(secrets)))
	return keys, nil
}

// sanitizeK8sName normalizes a name to a valid K8s resource name.
// Must match the logic in orchestrator/k3s/deploy.go sanitize().
func sanitizeK8sName(name string) string {
	n := strings.ToLower(name)
	n = strings.ReplaceAll(n, "_", "-")
	n = strings.ReplaceAll(n, " ", "-")
	if len(n) > 63 {
		n = n[:63]
	}
	return n
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
