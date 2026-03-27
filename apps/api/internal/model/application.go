package model

import (
	"context"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// Source type for deployment.
type SourceType string

const (
	SourceGit     SourceType = "git"
	SourceImage   SourceType = "image"
	SourceCompose SourceType = "compose"
)

// Build type for source-based deployments.
type BuildType string

const (
	BuildDockerfile BuildType = "dockerfile"
	BuildBuildpacks BuildType = "buildpacks"
	BuildNixpacks   BuildType = "nixpacks"
)

// Application status.
type AppStatus string

const (
	AppStatusIdle       AppStatus = "idle"
	AppStatusBuilding   AppStatus = "building"
	AppStatusDeploying  AppStatus = "deploying"
	AppStatusRestarting AppStatus = "restarting"
	AppStatusRunning    AppStatus = "running"
	AppStatusStopping   AppStatus = "stopping"
	AppStatusStopped    AppStatus = "stopped"
	AppStatusError      AppStatus = "error"
)

// PortMapping defines a container-to-service port mapping.
type PortMapping struct {
	ContainerPort int    `json:"container_port"`
	ServicePort   int    `json:"service_port"`
	Protocol      string `json:"protocol"` // tcp | udp
}

// HealthCheck configures liveness/readiness probes.
type HealthCheck struct {
	Path                string `json:"path"`
	Port                int    `json:"port"`
	InitialDelaySeconds int    `json:"initial_delay_seconds"`
	PeriodSeconds       int    `json:"period_seconds"`
	TimeoutSeconds      int    `json:"timeout_seconds"`
	FailureThreshold    int    `json:"failure_threshold"`
	Type                string `json:"type"`              // http | tcp | exec
	Command             string `json:"command,omitempty"` // for exec type
}

// AutoscalingConfig configures Horizontal Pod Autoscaler.
type AutoscalingConfig struct {
	Enabled     bool  `json:"enabled"`
	MinReplicas int32 `json:"min_replicas"`
	MaxReplicas int32 `json:"max_replicas"`
	CPUTarget   int32 `json:"cpu_target"` // percentage, 0 = disabled
	MemTarget   int32 `json:"mem_target"` // percentage, 0 = disabled
}

// VolumeMount configures a persistent volume claim mount.
type VolumeMount struct {
	Name      string `json:"name"`
	MountPath string `json:"mount_path"`
	Size      string `json:"size"`               // e.g. "1Gi"
	PVCName   string `json:"pvc_name,omitempty"` // filled after PVC creation
}

// DeployStrategyConfig configures rolling update parameters.
type DeployStrategyConfig struct {
	MaxSurge       string `json:"max_surge"`       // "25%" or "1"
	MaxUnavailable string `json:"max_unavailable"` // "25%" or "0"
}

// Application represents a deployable application.
type Application struct {
	BaseModel `bun:"table:applications,alias:app"`

	ProjectID uuid.UUID `bun:"project_id,notnull,type:uuid" json:"project_id"`
	Project   *Project  `bun:"rel:belongs-to,join:project_id=id" json:"-"`

	Name        string `bun:"name,notnull" json:"name"`
	Description string `bun:"description" json:"description"`

	// Source configuration
	SourceType    SourceType `bun:"source_type,notnull" json:"source_type"`
	GitRepo       string     `bun:"git_repo" json:"git_repo"`
	GitBranch     string     `bun:"git_branch,default:'main'" json:"git_branch"`
	GitProviderID *uuid.UUID `bun:"git_provider_id,type:uuid" json:"git_provider_id,omitempty"`
	DockerImage   string     `bun:"docker_image" json:"docker_image"`
	ComposeFile   string     `bun:"compose_file,type:text" json:"compose_file,omitempty"`

	// Build configuration
	BuildType    BuildType         `bun:"build_type" json:"build_type"`
	Dockerfile   string            `bun:"dockerfile,default:'Dockerfile'" json:"dockerfile"`
	BuildContext string            `bun:"build_context,default:'.'" json:"build_context"`
	WatchPaths   []string          `bun:"watch_paths,type:jsonb,default:'[]'" json:"watch_paths"`
	BuildArgs    map[string]string `bun:"build_args,type:jsonb,default:'{}'" json:"build_args"`
	NoCache      bool              `bun:"no_cache,default:false" json:"no_cache"`

	// Runtime configuration
	Replicas int32             `bun:"replicas,default:1" json:"replicas"`
	CPULimit string            `bun:"cpu_limit,default:'500m'" json:"cpu_limit"`
	MemLimit string            `bun:"mem_limit,default:'512Mi'" json:"mem_limit"`
	EnvVars  map[string]string `bun:"env_vars,type:jsonb,default:'{}'" json:"env_vars"`
	Secrets  map[string]string `bun:"secrets,type:jsonb,default:'{}'" json:"-"`
	Ports    []PortMapping     `bun:"ports,type:jsonb,default:'[]'" json:"ports"`

	// Advanced configuration
	HealthCheck            *HealthCheck          `bun:"health_check,type:jsonb,default:'{}'" json:"health_check"`
	Autoscaling            *AutoscalingConfig    `bun:"autoscaling,type:jsonb,default:'{}'" json:"autoscaling"`
	CPURequest             string                `bun:"cpu_request,default:'50m'" json:"cpu_request"`
	MemRequest             string                `bun:"mem_request,default:'64Mi'" json:"mem_request"`
	Volumes                []VolumeMount         `bun:"volumes,type:jsonb,default:'[]'" json:"volumes"`
	DeployStrategy         string                `bun:"deploy_strategy,default:'rolling'" json:"deploy_strategy"`
	DeployStrategyConfig   *DeployStrategyConfig `bun:"deploy_strategy_config,type:jsonb,default:'{}'" json:"deploy_strategy_config"`
	TerminationGracePeriod int                   `bun:"termination_grace_period,default:30" json:"termination_grace_period"`
	NodePool               string                `bun:"node_pool,default:''" json:"node_pool"`
	BuildEnvVars           map[string]string     `bun:"build_env_vars,type:jsonb,default:'{}'" json:"build_env_vars"`

	// K8s mapping
	Namespace string `bun:"namespace" json:"namespace"`
	K8sName   string `bun:"k8s_name" json:"k8s_name"`

	// Status
	Status AppStatus `bun:"status,default:'idle'" json:"status"`

	// Webhook / CI/CD
	WebhookSecret string `bun:"webhook_secret" json:"-"`
	AutoDeploy    bool   `bun:"auto_deploy,default:false" json:"auto_deploy"`

	// Relations (only populated when explicitly joined)
	Domains     []Domain     `bun:"rel:has-many,join:id=app_id" json:"-"`
	Deployments []Deployment `bun:"rel:has-many,join:id=app_id" json:"-"`
}

// AfterScanRow ensures nil maps/slices are initialized after reading from DB,
// so JSON serialization produces {} / [] instead of null.
var _ bun.AfterScanRowHook = (*Application)(nil)

func (a *Application) AfterScanRow(ctx context.Context) error {
	if a.EnvVars == nil {
		a.EnvVars = map[string]string{}
	}
	if a.BuildArgs == nil {
		a.BuildArgs = map[string]string{}
	}
	if a.Ports == nil {
		a.Ports = []PortMapping{}
	}
	if a.WatchPaths == nil {
		a.WatchPaths = []string{}
	}
	if a.Volumes == nil {
		a.Volumes = []VolumeMount{}
	}
	if a.BuildEnvVars == nil {
		a.BuildEnvVars = map[string]string{}
	}
	if a.Secrets == nil {
		a.Secrets = map[string]string{}
	}
	if a.HealthCheck == nil {
		a.HealthCheck = &HealthCheck{}
	}
	if a.Autoscaling == nil {
		a.Autoscaling = &AutoscalingConfig{}
	}
	if a.DeployStrategyConfig == nil {
		a.DeployStrategyConfig = &DeployStrategyConfig{}
	}
	return nil
}
