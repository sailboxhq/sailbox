// Package event provides real-time change notifications via PG LISTEN/NOTIFY.
//
// Architecture:
//
//	DB Trigger (INSERT/UPDATE/DELETE on key tables)
//	  → PG NOTIFY 'sailbox_changes', '{"table":"apps","op":"UPDATE","id":"..."}'
//	    → Listener (pgdriver LISTEN)
//	      → Broadcast to all SSE clients
package event

import (
	"context"
)

// Change represents a single database row change captured by a PG trigger.
type Change struct {
	Table string `json:"table"` // e.g. "applications", "deployments"
	Op    string `json:"op"`    // INSERT, UPDATE, DELETE
	ID    string `json:"id"`    // row UUID
}

// Subscriber receives real-time change events from the database.
type Subscriber interface {
	// Changes returns a read-only channel of database change events.
	// The channel is closed when the subscriber is stopped.
	Changes() <-chan Change

	// Close stops listening and releases the connection.
	Close() error
}

// Publisher can send a notification (used in tests or manual triggers).
type Publisher interface {
	Publish(ctx context.Context, change Change) error
}
