package model

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// CronJob status.
type CronJobStatus string

const (
	CronJobIdle    CronJobStatus = "idle"
	CronJobRunning CronJobStatus = "running"
	CronJobError   CronJobStatus = "error"
)

// CronJob represents a scheduled task in K8s.
type CronJob struct {
	BaseModel `bun:"table:cron_jobs,alias:cj"`

	ProjectID uuid.UUID `bun:"project_id,notnull,type:uuid" json:"project_id"`
	Project   *Project  `bun:"rel:belongs-to,join:project_id=id" json:"-"`

	Name        string `bun:"name,notnull" json:"name"`
	Description string `bun:"description" json:"description"`

	CronExpression string `bun:"cron_expression,notnull" json:"cron_expression"`
	Timezone       string `bun:"timezone,default:'UTC'" json:"timezone"`
	Command        string `bun:"command,notnull" json:"command"`

	// Source
	Image      string     `bun:"image" json:"image"`
	SourceType SourceType `bun:"source_type,default:'image'" json:"source_type"`
	GitRepo    string     `bun:"git_repo" json:"git_repo"`
	GitBranch  string     `bun:"git_branch,default:'main'" json:"git_branch"`

	// Config
	EnvVars               map[string]string `bun:"env_vars,type:jsonb,default:'{}'" json:"env_vars"`
	CPULimit              string            `bun:"cpu_limit,default:'500m'" json:"cpu_limit"`
	MemLimit              string            `bun:"mem_limit,default:'512Mi'" json:"mem_limit"`
	Enabled               bool              `bun:"enabled,default:true" json:"enabled"`
	ConcurrencyPolicy     string            `bun:"concurrency_policy,default:'Forbid'" json:"concurrency_policy"`
	RestartPolicy         string            `bun:"restart_policy,default:'OnFailure'" json:"restart_policy"`
	BackoffLimit          int               `bun:"backoff_limit,default:3" json:"backoff_limit"`
	ActiveDeadlineSeconds int               `bun:"active_deadline_seconds,default:0" json:"active_deadline_seconds"`

	// K8s
	Namespace string `bun:"namespace" json:"namespace"`
	K8sName   string `bun:"k8s_name" json:"k8s_name"`

	// Status
	LastRunAt *time.Time    `bun:"last_run_at" json:"last_run_at,omitempty"`
	Status    CronJobStatus `bun:"status,default:'idle'" json:"status"`
}

var _ bun.AfterScanRowHook = (*CronJob)(nil)

func (cj *CronJob) AfterScanRow(ctx context.Context) error {
	if cj.EnvVars == nil {
		cj.EnvVars = map[string]string{}
	}
	return nil
}

// CronJobRun status.
type CronJobRunStatus string

const (
	CronJobRunRunning   CronJobRunStatus = "running"
	CronJobRunSucceeded CronJobRunStatus = "succeeded"
	CronJobRunFailed    CronJobRunStatus = "failed"
)

// CronJobRun tracks a single execution of a CronJob.
type CronJobRun struct {
	BaseModel `bun:"table:cron_job_runs,alias:cjr"`

	CronJobID uuid.UUID `bun:"cron_job_id,notnull,type:uuid" json:"cron_job_id"`
	CronJob   *CronJob  `bun:"rel:belongs-to,join:cron_job_id=id" json:"-"`

	Status      CronJobRunStatus `bun:"status,notnull,default:'running'" json:"status"`
	StartedAt   time.Time        `bun:"started_at,notnull,default:current_timestamp" json:"started_at"`
	FinishedAt  *time.Time       `bun:"finished_at" json:"finished_at,omitempty"`
	ExitCode    *int             `bun:"exit_code" json:"exit_code,omitempty"`
	Logs        string           `bun:"logs,type:text" json:"logs,omitempty"`
	TriggerType string           `bun:"trigger_type,default:'scheduled'" json:"trigger_type"` // scheduled | manual
}
