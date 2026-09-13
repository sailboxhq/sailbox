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

// secretConfigFields are keys inside SharedResource.Config that hold a
// credential. Config is free-form JSON shared by every resource type, so the
// secret fields are identified by name rather than by struct.
var secretConfigFields = map[string]bool{
	"private_key":    true,
	"token":          true,
	"refresh_token":  true,
	"password":       true,
	"secret_key":     true,
	"client_secret":  true,
	"webhook_secret": true,
	"api_key":        true,
	"bot_token":      true,
	"webhook_url":    true, // a Slack/Discord webhook URL is itself the credential
}

// IsSecretConfigField reports whether a resource config key holds a credential.
func IsSecretConfigField(key string) bool { return secretConfigFields[key] }

// RedactConfig returns cfg with every credential field replaced by
// RedactedValue, so a client can see which fields are configured without
// learning their values. Non-secret fields (public_key, algorithm, api_url,
// org, username, access_key, ...) are passed through untouched.
func RedactConfig(cfg json.RawMessage) json.RawMessage {
	if len(cfg) == 0 {
		return cfg
	}
	var fields map[string]any
	if err := json.Unmarshal(cfg, &fields); err != nil {
		// Unparseable config could hold anything — withhold it rather than guess.
		return json.RawMessage(`{}`)
	}
	for k, v := range fields {
		if !IsSecretConfigField(k) {
			continue
		}
		if str, ok := v.(string); ok && str == "" {
			continue // not configured — leave it empty so the UI shows "unset"
		}
		fields[k] = RedactedValue
	}
	out, err := json.Marshal(fields)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return out
}

// Redacted returns a copy of the resource safe to send to a client.
func (r *SharedResource) Redacted() *SharedResource {
	if r == nil {
		return nil
	}
	clone := *r
	clone.Config = RedactConfig(r.Config)
	return &clone
}

type SharedResource struct {
	BaseModel `bun:"table:shared_resources,alias:sr"`

	OrgID    uuid.UUID       `bun:"org_id,notnull,type:uuid" json:"org_id"`
	Name     string          `bun:"name,notnull" json:"name"`
	Type     ResourceType    `bun:"type,notnull" json:"type"`
	Provider string          `bun:"provider" json:"provider"` // github | gitlab | dockerhub | ghcr | custom
	Config   json.RawMessage `bun:"config,type:jsonb,default:'{}'" json:"config"`
	Status   string          `bun:"status,default:'active'" json:"status"`
}
