package model

import (
	"encoding/json"

	"github.com/google/uuid"
)

type NotificationChannelType string

const (
	NotifyEmail    NotificationChannelType = "email"
	NotifyTelegram NotificationChannelType = "telegram"
	NotifyDiscord  NotificationChannelType = "discord"
	NotifySlack    NotificationChannelType = "slack"
)

// NotifyEvent represents a notification event type.
type NotifyEvent string

const (
	EventDeploySuccess   NotifyEvent = "deploy_success"
	EventDeployFailed    NotifyEvent = "deploy_failed"
	EventDeployCancelled NotifyEvent = "deploy_cancelled"
	EventBuildTimeout    NotifyEvent = "build_timeout"
	EventAppCrashed      NotifyEvent = "app_crashed"
	EventAppStopped      NotifyEvent = "app_stopped"
	EventAppRestarted    NotifyEvent = "app_restarted"
	EventBackupSuccess   NotifyEvent = "backup_success"
	EventBackupFailed    NotifyEvent = "backup_failed"
	EventNodeOffline     NotifyEvent = "node_not_ready"
	EventDiskPressure    NotifyEvent = "disk_pressure"
	EventCertExpiring    NotifyEvent = "cert_expiring"
	EventMemberJoined    NotifyEvent = "member_joined"
	EventMemberRemoved   NotifyEvent = "member_removed"
	EventAlertFired      NotifyEvent = "alert_fired"
	EventAlertResolved   NotifyEvent = "alert_resolved"
	EventDatabaseCreated NotifyEvent = "database_created"
	EventDatabaseDeleted NotifyEvent = "database_deleted"
)

// AllNotifyEvents returns all available event types (for future filtering UI).
func AllNotifyEvents() []NotifyEvent {
	return []NotifyEvent{
		EventDeploySuccess, EventDeployFailed, EventDeployCancelled, EventBuildTimeout,
		EventAppCrashed, EventAppStopped, EventAppRestarted,
		EventBackupSuccess, EventBackupFailed,
		EventNodeOffline, EventDiskPressure, EventCertExpiring,
		EventMemberJoined, EventMemberRemoved,
		EventAlertFired, EventAlertResolved,
		EventDatabaseCreated, EventDatabaseDeleted,
	}
}

type NotificationChannel struct {
	BaseModel `bun:"table:notification_channels,alias:nc"`
	OrgID     uuid.UUID               `bun:"org_id,notnull,type:uuid" json:"org_id"`
	Type      NotificationChannelType `bun:"type,notnull" json:"type"`
	Enabled   bool                    `bun:"enabled,default:false" json:"enabled"`
	Config    json.RawMessage         `bun:"config,type:jsonb,default:'{}'" json:"config"`
}
