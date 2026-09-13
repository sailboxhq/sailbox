package service

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/sailboxhq/sailbox/apps/api/internal/model"
	"github.com/sailboxhq/sailbox/apps/api/internal/orchestrator"
	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

type ProjectService struct {
	store  store.Store
	orch   orchestrator.Orchestrator
	logger *slog.Logger
}

func NewProjectService(s store.Store, orch orchestrator.Orchestrator, logger *slog.Logger) *ProjectService {
	return &ProjectService{store: s, orch: orch, logger: logger}
}

type CreateProjectInput struct {
	Name        string `json:"name" binding:"required,min=1,max=63"`
	Description string `json:"description"`
}

type UpdateProjectInput struct {
	Name                 *string                    `json:"name" binding:"omitempty,min=1,max=63"`
	Description          *string                    `json:"description"`
	EnvVars              map[string]string          `json:"env_vars"`
	ResourceQuota        *model.ResourceQuotaConfig `json:"resource_quota"`
	NetworkPolicyEnabled *bool                      `json:"network_policy_enabled"`
}

// generateNamespace produces a valid K8s namespace from the project name and ID.
func generateNamespace(name string, id uuid.UUID) string {
	// Check if name is ASCII-only
	isASCII := true
	for _, r := range name {
		if r > unicode.MaxASCII {
			isASCII = false
			break
		}
	}

	var slug string
	if isASCII {
		slug = strings.ToLower(name)
		slug = strings.ReplaceAll(slug, " ", "-")
		slug = strings.ReplaceAll(slug, "_", "-")
		// Remove non-alphanumeric except hyphens
		reg := regexp.MustCompile(`[^a-z0-9-]`)
		slug = reg.ReplaceAllString(slug, "")
		// Collapse multiple hyphens
		for strings.Contains(slug, "--") {
			slug = strings.ReplaceAll(slug, "--", "-")
		}
		slug = strings.Trim(slug, "-")
	} else {
		slug = "proj"
	}

	suffix := strings.ReplaceAll(id.String(), "-", "")[:8]
	ns := "sb-" + slug + "-" + suffix

	if len(ns) > 63 {
		ns = ns[:63]
	}
	return ns
}

func (s *ProjectService) Create(ctx context.Context, orgID uuid.UUID, input CreateProjectInput) (*model.Project, error) {
	project := &model.Project{
		OrgID:       orgID,
		Name:        input.Name,
		Description: input.Description,
	}
	if err := s.store.Projects().Create(ctx, project); err != nil {
		return nil, err
	}

	// Generate and persist namespace
	project.Namespace = generateNamespace(input.Name, project.ID)
	if err := s.store.Projects().Update(ctx, project); err != nil {
		return nil, err
	}

	// Create K8s namespace
	if err := s.orch.CreateNamespace(ctx, project.Namespace); err != nil {
		s.logger.Error("failed to create K8s namespace", slog.Any("error", err), slog.String("namespace", project.Namespace))
		// Clean up the DB record — project without a namespace is unusable
		_ = s.store.Projects().Delete(ctx, project.ID)
		return nil, fmt.Errorf("failed to create namespace: %w", err)
	}

	// Create service account in the namespace
	saName := "sa-" + project.Namespace
	if err := s.orch.EnsureServiceAccount(ctx, project.Namespace, saName); err != nil {
		s.logger.Error("failed to ensure service account", slog.Any("error", err), slog.String("namespace", project.Namespace))
		_ = s.store.Projects().Delete(ctx, project.ID)
		return nil, fmt.Errorf("failed to create service account: %w", err)
	}
	project.ServiceAccount = saName
	if err := s.store.Projects().Update(ctx, project); err != nil {
		return nil, err
	}

	s.logger.Info("project created", slog.String("name", project.Name), slog.String("id", project.ID.String()), slog.String("namespace", project.Namespace))
	return project, nil
}

func (s *ProjectService) GetByID(ctx context.Context, id uuid.UUID) (*model.Project, error) {
	return s.store.Projects().GetByID(ctx, id)
}

// List returns an org's projects. projectIDs, when non-nil, narrows the result
// to projects the caller has been granted.
func (s *ProjectService) List(ctx context.Context, orgID uuid.UUID, params store.ListParams, projectIDs []uuid.UUID) ([]model.Project, int, error) {
	return s.store.Projects().ListByOrg(ctx, orgID, params, projectIDs)
}

func (s *ProjectService) Update(ctx context.Context, id uuid.UUID, input UpdateProjectInput) (*model.Project, error) {
	project, err := s.store.Projects().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if input.Name != nil {
		project.Name = *input.Name
	}
	if input.Description != nil {
		project.Description = *input.Description
	}
	if input.EnvVars != nil {
		project.EnvVars = input.EnvVars
	}
	if input.ResourceQuota != nil {
		project.ResourceQuota = input.ResourceQuota
	}
	if input.NetworkPolicyEnabled != nil {
		project.NetworkPolicyEnabled = *input.NetworkPolicyEnabled
	}
	if err := s.store.Projects().Update(ctx, project); err != nil {
		return nil, err
	}

	// Sync K8s resources if project has a namespace
	if project.Namespace != "" {
		// ConfigMap for project env vars + push to running apps
		if input.EnvVars != nil {
			if err := s.orch.EnsureConfigMap(ctx, project.Namespace, "project-env", project.EnvVars); err != nil {
				s.logger.Error("failed to ensure ConfigMap", slog.Any("error", err))
			}
			// Push merged env to all apps in this project
			apps, _, listErr := s.store.Applications().ListByProject(ctx, project.ID, store.ListParams{Page: 1, PerPage: 1000})
			if listErr != nil {
				s.logger.Error("failed to list apps for env propagation", slog.Any("error", listErr))
			} else {
				for _, app := range apps {
					merged := make(map[string]string)
					for k, v := range project.EnvVars {
						merged[k] = v
					}
					for k, v := range app.EnvVars {
						merged[k] = v
					}
					if err := s.orch.UpdateEnvVars(ctx, &app, merged); err != nil {
						s.logger.Warn("failed to push project env to app",
							slog.String("app", app.Name), slog.Any("error", err))
					}
				}
			}
			// Push merged env to all CronJobs in this project (merge at K8s level, don't modify DB)
			cjs, _, cjListErr := s.store.CronJobs().ListByProject(ctx, project.ID, store.ListParams{Page: 1, PerPage: 1000})
			if cjListErr != nil {
				s.logger.Error("failed to list cronjobs for env propagation", slog.Any("error", cjListErr))
			} else {
				for i := range cjs {
					// Build a temporary copy with merged env for K8s, don't persist
					tmp := cjs[i]
					merged := make(map[string]string)
					for k, v := range project.EnvVars {
						merged[k] = v
					}
					for k, v := range tmp.EnvVars {
						merged[k] = v
					}
					tmp.EnvVars = merged
					if err := s.orch.UpdateCronJob(ctx, &tmp); err != nil {
						s.logger.Warn("failed to push project env to cronjob",
							slog.String("cronjob", tmp.Name), slog.Any("error", err))
					}
				}
			}
		}
		// ResourceQuota
		if input.ResourceQuota != nil {
			if err := s.orch.EnsureResourceQuota(ctx, project.Namespace, *project.ResourceQuota); err != nil {
				s.logger.Error("failed to ensure ResourceQuota", slog.Any("error", err))
			}
		}
		// NetworkPolicy
		if input.NetworkPolicyEnabled != nil {
			if err := s.orch.EnsureNetworkPolicy(ctx, project.Namespace, project.NetworkPolicyEnabled); err != nil {
				s.logger.Error("failed to ensure NetworkPolicy", slog.Any("error", err))
			}
		}
	}

	return project, nil
}

// UpdateEnvVars updates only the project environment variables.
func (s *ProjectService) UpdateEnvVars(ctx context.Context, id uuid.UUID, envVars map[string]string) (*model.Project, error) {
	return s.Update(ctx, id, UpdateProjectInput{EnvVars: envVars})
}

func (s *ProjectService) Delete(ctx context.Context, id uuid.UUID) error {
	project, err := s.store.Projects().GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Delete K8s namespace if one was assigned — this removes the cluster-side
	// objects for every app, database and cron job in the project.
	if project.Namespace != "" {
		if err := s.orch.DeleteNamespace(ctx, project.Namespace); err != nil {
			s.logger.Error("failed to delete K8s namespace", slog.Any("error", err), slog.String("namespace", project.Namespace))
		}
	}

	// The database side needs the same treatment. Projects are soft-deleted, so
	// the ON DELETE CASCADE never fires: without this the project's apps and
	// databases stay live rows pointing at a deleted project and keep showing
	// up in global listings and search.
	return s.store.RunInTx(ctx, func(ctx context.Context, tx store.Store) error {
		if err := tx.Domains().DeleteByProject(ctx, id); err != nil {
			return fmt.Errorf("delete domains: %w", err)
		}
		if err := tx.Applications().DeleteByProject(ctx, id); err != nil {
			return fmt.Errorf("delete applications: %w", err)
		}
		if err := tx.ManagedDatabases().DeleteByProject(ctx, id); err != nil {
			return fmt.Errorf("delete databases: %w", err)
		}
		if err := tx.CronJobs().DeleteByProject(ctx, id); err != nil {
			return fmt.Errorf("delete cron jobs: %w", err)
		}
		return tx.Projects().Delete(ctx, id)
	})
}
