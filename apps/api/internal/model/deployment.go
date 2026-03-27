package model

import (
	"time"

	"github.com/google/uuid"
)

// Deployment status.
type DeployStatus string

const (
	DeployQueued    DeployStatus = "queued"
	DeployBuilding  DeployStatus = "building"
	DeployDeploying DeployStatus = "deploying"
	DeploySuccess   DeployStatus = "success"
	DeployFailed    DeployStatus = "failed"
	DeployCancelled DeployStatus = "cancelled"
)

// Deployment tracks a single deployment attempt for an application.
type Deployment struct {
	BaseModel `bun:"table:deployments,alias:d"`

	AppID       uuid.UUID    `bun:"app_id,notnull,type:uuid" json:"app_id"`
	Application *Application `bun:"rel:belongs-to,join:app_id=id" json:"-"`

	Status    DeployStatus `bun:"status,notnull,default:'queued'" json:"status"`
	CommitSHA string       `bun:"commit_sha" json:"commit_sha,omitempty"`
	Image     string       `bun:"image" json:"image,omitempty"`
	BuildLog  string       `bun:"build_log,type:text" json:"build_log,omitempty"`

	StartedAt  *time.Time `bun:"started_at" json:"started_at,omitempty"`
	FinishedAt *time.Time `bun:"finished_at" json:"finished_at,omitempty"`

	// Who triggered the deployment
	TriggerType string     `bun:"trigger_type" json:"trigger_type"` // manual | webhook | rollback
	TriggeredBy *uuid.UUID `bun:"triggered_by,type:uuid" json:"triggered_by,omitempty"`

	// Denormalized for global queries
	AppName   string    `bun:"app_name" json:"app_name,omitempty"`
	ProjectID uuid.UUID `bun:"project_id,type:uuid" json:"project_id,omitempty"`
}
