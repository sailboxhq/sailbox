package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sailboxhq/sailbox/apps/api/internal/model"
	"github.com/sailboxhq/sailbox/apps/api/internal/orchestrator"
	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

type DeployService struct {
	store    store.Store
	orch     orchestrator.Orchestrator
	logger   *slog.Logger
	buildSvc *BuildService
	notifSvc *NotificationService

	mu      sync.Mutex
	cancels map[uuid.UUID]context.CancelFunc // deployID -> cancel
}

func NewDeployService(s store.Store, orch orchestrator.Orchestrator, logger *slog.Logger, buildSvc *BuildService, notifSvc *NotificationService) *DeployService {
	svc := &DeployService{store: s, orch: orch, logger: logger, buildSvc: buildSvc, notifSvc: notifSvc, cancels: make(map[uuid.UUID]context.CancelFunc)}
	go svc.periodicStaleCheck()
	return svc
}

// periodicStaleCheck runs stale deployment recovery on startup and every 5 minutes.
func (s *DeployService) periodicStaleCheck() {
	s.recoverStaleDeployments()
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		s.recoverStaleDeployments()
	}
}

// recoverStaleDeployments marks deployments stuck for >30 minutes as failed.
// Only targets truly orphaned deploys, not ones still running in K8s.
func (s *DeployService) recoverStaleDeployments() {
	ctx := context.Background()
	staleThreshold := 30 * time.Minute
	for _, status := range []string{"queued", "building", "deploying"} {
		deploys, _, err := s.store.Deployments().ListAll(ctx, store.ListParams{Page: 1, PerPage: 1000}, store.DeploymentListFilter{Status: status})
		if err != nil {
			continue
		}
		for _, d := range deploys {
			age := time.Since(d.CreatedAt)
			if age < staleThreshold {
				s.logger.Info("skipping recent in-progress deployment",
					slog.String("deploy_id", d.ID.String()),
					slog.String("app_name", d.AppName),
					slog.Duration("age", age),
				)
				continue
			}
			now := time.Now()
			d.Status = model.DeployFailed
			d.FinishedAt = &now
			d.BuildLog += "\n\n--- Deployment timed out (stale for >30min) ---"
			s.updateDeploy(ctx, &d)
			s.setAppStatus(ctx, d.AppID, model.AppStatusError)
			s.logger.Warn("recovered stale deployment",
				slog.String("deploy_id", d.ID.String()),
				slog.String("app_name", d.AppName),
				slog.String("was_status", status),
				slog.Duration("age", age),
			)
		}
	}
}

type TriggerDeployInput struct {
	AppID       uuid.UUID  `json:"app_id" binding:"required"`
	ForceBuild  bool       `json:"force_build"`
	TriggeredBy *uuid.UUID `json:"-"`
	TriggerType string     `json:"-"` // manual | webhook | rollback
}

// Trigger creates a new deployment record and enqueues the build/deploy job.
func (s *DeployService) Trigger(ctx context.Context, input TriggerDeployInput) (*model.Deployment, error) {
	app, err := s.store.Applications().GetByID(ctx, input.AppID)
	if err != nil {
		return nil, err
	}

	// Prevent triggering deploy while another operation is in progress
	switch app.Status {
	case model.AppStatusBuilding, model.AppStatusDeploying, model.AppStatusRestarting, model.AppStatusStopping:
		return nil, fmt.Errorf("app is currently %s, wait for it to finish first", app.Status)
	}

	now := time.Now()
	deploy := &model.Deployment{
		AppID:       app.ID,
		Status:      model.DeployQueued,
		TriggerType: input.TriggerType,
		TriggeredBy: input.TriggeredBy,
		StartedAt:   &now,
		AppName:     app.Name,
		ProjectID:   app.ProjectID,
	}

	if err := s.store.Deployments().Create(ctx, deploy); err != nil {
		return nil, err
	}

	// Update app status to building
	s.setAppStatus(ctx, app.ID, model.AppStatusBuilding)

	s.logger.Info("deployment triggered",
		slog.String("deployment_id", deploy.ID.String()),
		slog.String("app", app.Name),
		slog.String("trigger", input.TriggerType),
	)

	// Execute in background with timeout (30 min max)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	s.mu.Lock()
	s.cancels[deploy.ID] = cancel
	s.mu.Unlock()

	appCopy := *app
	deployCopy := *deploy
	forceBuild := input.ForceBuild
	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.cancels, deployCopy.ID)
			s.mu.Unlock()
			cancel() // safe: context.CancelFunc is idempotent (multiple calls are no-op)
		}()
		s.executeDeploy(ctx, &appCopy, &deployCopy, forceBuild)
	}()

	return deploy, nil
}

// executeDeploy runs the deploy via orchestrator and updates status.
func (s *DeployService) executeDeploy(ctx context.Context, app *model.Application, deploy *model.Deployment, forceBuild bool) {
	if app.SourceType == model.SourceGit {
		skipBuild := false

		// Check if we can skip the build: same commit SHA as last successful deploy
		if !forceBuild && !app.NoCache && deploy.CommitSHA != "" {
			lastDeploy, err := s.store.Deployments().GetLatestByApp(ctx, app.ID)
			if err == nil && lastDeploy.Status == model.DeploySuccess && lastDeploy.Image != "" &&
				lastDeploy.CommitSHA != "" && lastDeploy.CommitSHA == deploy.CommitSHA {
				skipBuild = true
				s.logger.Info("skipping build — same commit, reusing image",
					slog.String("app", app.Name),
					slog.String("commit", deploy.CommitSHA),
					slog.String("image", lastDeploy.Image),
				)
				deploy.BuildLog = fmt.Sprintf("Build skipped — commit %s unchanged, reusing image.\nUse 'Force Build' to rebuild from scratch.", deploy.CommitSHA[:minLen(7, len(deploy.CommitSHA))])
				deploy.Image = lastDeploy.Image
				app.DockerImage = lastDeploy.Image
				s.updateDeploy(ctx, deploy)
			}
		}

		if !skipBuild {
			s.logger.Info("building from source", slog.String("app", app.Name))
			if err := s.buildSvc.Build(ctx, app, deploy); err != nil {
				now := time.Now()
				deploy.FinishedAt = &now
				if ctx.Err() == context.DeadlineExceeded {
					deploy.Status = model.DeployFailed
					deploy.BuildLog += "\n\n--- Build timed out (30 min limit) ---"
					s.notifyDeploy(app, model.EventBuildTimeout, "build timed out (30 min limit)")
				} else if ctx.Err() != nil {
					deploy.Status = model.DeployCancelled
					deploy.BuildLog += "\n\n--- Cancelled ---"
					s.notifyDeploy(app, model.EventDeployCancelled, "deploy was cancelled")
				} else {
					deploy.Status = model.DeployFailed
					s.notifyDeploy(app, model.EventDeployFailed, fmt.Sprintf("build failed: %v", err))
				}
				s.updateDeploy(context.Background(), deploy)
				s.setAppStatus(context.Background(), app.ID, model.AppStatusError)
				s.logger.Error("build failed", slog.Any("error", err), slog.String("app", app.Name))
				return
			}
			if ctx.Err() != nil {
				return
			}
			updatedApp, err := s.store.Applications().GetByID(ctx, app.ID)
			if err != nil {
				s.logger.Error("failed to reload app after build", slog.Any("error", err))
				now := time.Now()
				deploy.Status = model.DeployFailed
				deploy.FinishedAt = &now
				deploy.BuildLog += "\n\n--- Failed to reload app after build ---"
				s.updateDeploy(context.Background(), deploy)
				s.setAppStatus(context.Background(), app.ID, model.AppStatusError)
				s.notifyDeploy(app, model.EventDeployFailed, "failed to reload app after build")
				return
			}
			app = updatedApp
		}
	}

	// Deploy
	deploy.Status = model.DeployDeploying
	s.updateDeploy(ctx, deploy)

	// Merge environment variables: project env → app env (app overrides project)
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

	err := s.orch.Deploy(ctx, app, orchestrator.DeployOpts{
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
	})

	now := time.Now()
	deploy.FinishedAt = &now

	if err != nil {
		deploy.Status = model.DeployFailed
		s.updateDeploy(ctx, deploy)
		s.setAppStatus(ctx, app.ID, model.AppStatusError)
		s.logger.Error("deploy failed", slog.Any("error", err), slog.String("app", app.Name))
		s.notifyDeploy(app, model.EventDeployFailed, fmt.Sprintf("deploy failed: %v", err))
		return
	}

	deploy.Status = model.DeploySuccess
	deploy.Image = app.DockerImage
	s.updateDeploy(ctx, deploy)
	s.setAppStatus(ctx, app.ID, model.AppStatusRunning)
	// Persist any metadata written back by orchestrator (e.g. PVC names)
	_ = s.store.Applications().Update(ctx, app)

	// Restore HPA if autoscaling is configured (may have been removed by Stop)
	if app.Autoscaling != nil && app.Autoscaling.Enabled {
		if err := s.orch.ConfigureHPA(ctx, app, *app.Autoscaling); err != nil {
			s.logger.Error("failed to restore HPA after deploy — autoscaling inactive until manually re-saved",
				slog.String("app", app.Name), slog.Any("error", err))
			s.notifyDeploy(app, model.EventDeploySuccess,
				fmt.Sprintf("%s deployed successfully, but autoscaling (HPA) failed to restore — re-save autoscaling settings to fix", app.Name))
			return
		}
	}

	s.logger.Info("deploy succeeded", slog.String("app", app.Name), slog.String("deploy", deploy.ID.String()))
	s.notifyDeploy(app, model.EventDeploySuccess, fmt.Sprintf("%s deployed successfully", app.Name))

	// Cleanup: remove completed/succeeded pods from previous deployments
	go s.cleanupCompletedPods(app)
}

// notifyDeploy sends a deploy notification for the given app. It resolves the orgID
// from the project and fires the notification asynchronously.
func (s *DeployService) notifyDeploy(app *model.Application, event model.NotifyEvent, detail string) {
	if s.notifSvc == nil {
		return
	}
	project, err := s.store.Projects().GetByID(context.Background(), app.ProjectID)
	if err != nil {
		s.logger.Warn("failed to resolve org for deploy notification", slog.Any("error", err))
		return
	}
	title := fmt.Sprintf("%s: %s", app.Name, string(event))
	s.notifSvc.NotifyAsync(project.OrgID, event, title, detail)
}

func (s *DeployService) updateDeploy(ctx context.Context, deploy *model.Deployment) {
	if err := s.store.Deployments().Update(ctx, deploy); err != nil {
		s.logger.Error("failed to update deployment",
			slog.String("deploy_id", deploy.ID.String()),
			slog.String("status", string(deploy.Status)),
			slog.Any("error", err),
		)
	}
}

// cleanupCompletedPods removes Succeeded/Completed pods for an app after successful deploy.
func (s *DeployService) cleanupCompletedPods(app *model.Application) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pods, err := s.orch.GetPods(ctx, app)
	if err != nil {
		return
	}
	for _, pod := range pods {
		if pod.Phase == "Succeeded" || pod.Phase == "Failed" {
			if err := s.orch.DeletePod(ctx, pod.Name, app); err != nil {
				s.logger.Warn("failed to cleanup completed pod", slog.String("pod", pod.Name), slog.Any("error", err))
			} else {
				s.logger.Info("cleaned up completed pod", slog.String("pod", pod.Name))
			}
		}
	}
}

func (s *DeployService) setAppStatus(ctx context.Context, appID uuid.UUID, status model.AppStatus) {
	if err := s.store.Applications().UpdateStatus(ctx, appID, status); err != nil {
		s.logger.Error("failed to update app status",
			slog.String("app_id", appID.String()),
			slog.String("target_status", string(status)),
			slog.Any("error", err),
		)
	}
}

func minLen(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Cancel stops a running build/deploy by cancelling its context and cleaning up K8s resources.
func (s *DeployService) Cancel(ctx context.Context, deployID uuid.UUID) error {
	deploy, err := s.store.Deployments().GetByID(ctx, deployID)
	if err != nil {
		return err
	}

	if deploy.Status != model.DeployQueued && deploy.Status != model.DeployBuilding && deploy.Status != model.DeployDeploying {
		return fmt.Errorf("deployment is not in progress (status: %s)", deploy.Status)
	}

	// Cancel the goroutine context
	s.mu.Lock()
	cancel, ok := s.cancels[deployID]
	s.mu.Unlock()
	if ok {
		cancel()
	}

	// Clean up any build jobs in K8s
	app, err := s.store.Applications().GetByID(ctx, deploy.AppID)
	if err == nil {
		s.cleanupBuildJobs(ctx, app)
	}

	// Update status
	now := time.Now()
	deploy.Status = model.DeployCancelled
	deploy.FinishedAt = &now
	if deploy.BuildLog != "" {
		deploy.BuildLog += "\n\n--- Cancelled by user ---"
	} else {
		deploy.BuildLog = "Cancelled by user"
	}
	s.updateDeploy(ctx, deploy)

	// Send cancellation notification
	if app != nil {
		s.notifyDeploy(app, model.EventDeployCancelled, fmt.Sprintf("%s deploy cancelled by user", app.Name))
	}

	// Reconcile app status with actual K8s state instead of blindly setting idle
	if app != nil {
		status, statusErr := s.orch.GetStatus(ctx, app)
		if statusErr == nil && status.Phase == "running" {
			s.setAppStatus(ctx, deploy.AppID, model.AppStatusRunning)
		} else {
			s.setAppStatus(ctx, deploy.AppID, model.AppStatusIdle)
		}
	} else {
		s.setAppStatus(ctx, deploy.AppID, model.AppStatusIdle)
	}

	s.logger.Info("deployment cancelled", slog.String("deploy", deployID.String()))
	return nil
}

func (s *DeployService) cleanupBuildJobs(ctx context.Context, app *model.Application) {
	_ = s.orch.CancelBuild(ctx, app)
}

// GetByID returns a deployment by ID.
func (s *DeployService) GetByID(ctx context.Context, id uuid.UUID) (*model.Deployment, error) {
	return s.store.Deployments().GetByID(ctx, id)
}

// List returns deployments for an app.
func (s *DeployService) List(ctx context.Context, appID uuid.UUID, params store.ListParams) ([]model.Deployment, int, error) {
	return s.store.Deployments().ListByApp(ctx, appID, params)
}

// ListAll returns deployments across all apps with optional status filter.
func (s *DeployService) ListAll(ctx context.Context, params store.ListParams, filter store.DeploymentListFilter) ([]model.Deployment, int, error) {
	return s.store.Deployments().ListAll(ctx, params, filter)
}

// Rollback re-deploys a specific previous deployment.
func (s *DeployService) Rollback(ctx context.Context, deployID uuid.UUID, triggeredBy *uuid.UUID) (*model.Deployment, error) {
	prev, err := s.store.Deployments().GetByID(ctx, deployID)
	if err != nil {
		return nil, err
	}

	app, err := s.store.Applications().GetByID(ctx, prev.AppID)
	if err != nil {
		return nil, err
	}

	if err := s.orch.Rollback(ctx, app, 0); err != nil {
		return nil, err
	}

	now := time.Now()
	deploy := &model.Deployment{
		AppID:       app.ID,
		Status:      model.DeploySuccess,
		Image:       prev.Image,
		CommitSHA:   prev.CommitSHA,
		TriggerType: "rollback",
		TriggeredBy: triggeredBy,
		StartedAt:   &now,
		FinishedAt:  &now,
	}

	if err := s.store.Deployments().Create(ctx, deploy); err != nil {
		return nil, err
	}

	s.setAppStatus(ctx, app.ID, model.AppStatusRunning)
	s.logger.Info("rollback succeeded", slog.String("app", app.Name), slog.String("to_deploy", deployID.String()))

	return deploy, nil
}
