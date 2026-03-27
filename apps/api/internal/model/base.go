package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// BaseModel provides common fields for all database models.
type BaseModel struct {
	bun.BaseModel `bun:"-"`

	ID        uuid.UUID  `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	CreatedAt time.Time  `bun:"created_at,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time  `bun:"updated_at,notnull,default:current_timestamp" json:"updated_at"`
	DeletedAt *time.Time `bun:"deleted_at,soft_delete,nullzero" json:"-"`
}
