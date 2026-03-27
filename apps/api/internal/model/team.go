package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// ProjectMember represents a user's access to a specific project.
type ProjectMember struct {
	bun.BaseModel `bun:"table:project_members,alias:pm"`

	ID        uuid.UUID `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	ProjectID uuid.UUID `bun:"project_id,notnull,type:uuid" json:"project_id"`
	UserID    uuid.UUID `bun:"user_id,notnull,type:uuid" json:"user_id"`
	Role      string    `bun:"role,notnull,default:'viewer'" json:"role"` // "admin" or "viewer"
	CreatedAt time.Time `bun:"created_at,notnull,default:current_timestamp" json:"created_at"`
}

// Invitation represents a pending invitation to join an organization.
type Invitation struct {
	bun.BaseModel `bun:"table:invitations,alias:inv"`

	ID         uuid.UUID  `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	OrgID      uuid.UUID  `bun:"org_id,notnull,type:uuid" json:"org_id"`
	Email      string     `bun:"email,notnull" json:"email"`
	Role       string     `bun:"role,notnull,default:'member'" json:"role"` // "admin" or "member"
	Token      string     `bun:"token,notnull" json:"-"`                    // never expose in JSON
	InvitedBy  *uuid.UUID `bun:"invited_by,type:uuid" json:"invited_by,omitempty"`
	ExpiresAt  time.Time  `bun:"expires_at,notnull" json:"expires_at"`
	AcceptedAt *time.Time `bun:"accepted_at" json:"accepted_at,omitempty"`
	CreatedAt  time.Time  `bun:"created_at,notnull,default:current_timestamp" json:"created_at"`
}

// TeamMember is a view struct for the team list (user + role).
type TeamMember struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	FirstName   string    `json:"first_name"`
	LastName    string    `json:"last_name"`
	AvatarURL   string    `json:"avatar_url"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
}
