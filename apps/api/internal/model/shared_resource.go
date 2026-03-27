package model

import (
	"encoding/json"

	"github.com/google/uuid"
)

type ResourceType string

const (
	ResourceGitProvider   ResourceType = "git_provider"
	ResourceRegistry      ResourceType = "registry"
	ResourceSSHKey        ResourceType = "ssh_key"
	ResourceObjectStorage ResourceType = "object_storage"
)

type SharedResource struct {
	BaseModel `bun:"table:shared_resources,alias:sr"`

	OrgID    uuid.UUID       `bun:"org_id,notnull,type:uuid" json:"org_id"`
	Name     string          `bun:"name,notnull" json:"name"`
	Type     ResourceType    `bun:"type,notnull" json:"type"`
	Provider string          `bun:"provider" json:"provider"` // github | gitlab | dockerhub | ghcr | custom
	Config   json.RawMessage `bun:"config,type:jsonb,default:'{}'" json:"config"`
	Status   string          `bun:"status,default:'active'" json:"status"`
}
