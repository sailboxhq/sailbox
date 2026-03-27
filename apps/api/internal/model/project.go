package model

import (
	"context"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// ResourceQuotaConfig defines project-level resource quotas.
type ResourceQuotaConfig struct {
	CPULimit     string `json:"cpu_limit,omitempty"`     // e.g. "4000m"
	MemLimit     string `json:"mem_limit,omitempty"`     // e.g. "8Gi"
	PodLimit     int    `json:"pod_limit,omitempty"`     // max pods
	PVCLimit     int    `json:"pvc_limit,omitempty"`     // max PVCs
	StorageLimit string `json:"storage_limit,omitempty"` // e.g. "50Gi"
}

// Project is a logical grouping of applications within an organization.
type Project struct {
	BaseModel `bun:"table:projects,alias:p"`

	OrgID        uuid.UUID     `bun:"org_id,notnull,type:uuid" json:"org_id"`
	Organization *Organization `bun:"rel:belongs-to,join:org_id=id" json:"-"`

	Name        string `bun:"name,notnull" json:"name"`
	Namespace   string `bun:"namespace" json:"namespace"`
	Description string `bun:"description" json:"description"`

	// Project environment (shared by all services)
	EnvVars map[string]string `bun:"env_vars,type:jsonb,default:'{}'" json:"env_vars"`

	// Service account
	ServiceAccount string `bun:"service_account,default:''" json:"service_account"`

	// Resource controls
	ResourceQuota        *ResourceQuotaConfig `bun:"resource_quota,type:jsonb,default:'{}'" json:"resource_quota"`
	NetworkPolicyEnabled bool                 `bun:"network_policy_enabled,default:false" json:"network_policy_enabled"`

	// Relations
	Applications []Application `bun:"rel:has-many,join:id=project_id" json:"-"`
	CronJobs     []CronJob     `bun:"rel:has-many,join:id=project_id" json:"-"`
}

var _ bun.AfterScanRowHook = (*Project)(nil)

func (p *Project) AfterScanRow(ctx context.Context) error {
	if p.EnvVars == nil {
		p.EnvVars = map[string]string{}
	}
	if p.ResourceQuota == nil {
		p.ResourceQuota = &ResourceQuotaConfig{}
	}
	return nil
}
