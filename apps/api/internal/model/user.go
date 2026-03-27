package model

import "github.com/google/uuid"

type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

// User represents an authenticated user.
type User struct {
	BaseModel `bun:"table:users,alias:u"`

	OrgID        uuid.UUID     `bun:"org_id,notnull,type:uuid" json:"org_id"`
	Organization *Organization `bun:"rel:belongs-to,join:org_id=id" json:"-"`

	Email        string `bun:"email,notnull,unique" json:"email"`
	PasswordHash string `bun:"password_hash,notnull" json:"-"`
	DisplayName  string `bun:"display_name" json:"display_name"`
	FirstName    string `bun:"first_name,default:''" json:"first_name"`
	LastName     string `bun:"last_name,default:''" json:"last_name"`
	Role         Role   `bun:"role,notnull,default:'member'" json:"role"`
	AvatarURL    string `bun:"avatar_url" json:"avatar_url,omitempty"`
	TwoFASecret  string `bun:"two_fa_secret,default:''" json:"-"`
	TwoFAEnabled bool   `bun:"two_fa_enabled,default:false" json:"two_fa_enabled"`
}

// PredefinedAvatars is the list of available avatar keys.
var PredefinedAvatars = []string{
	"bear", "cat", "dog", "fox", "koala", "lion", "monkey", "owl",
	"panda", "penguin", "rabbit", "tiger", "whale", "wolf",
}

// IsValidAvatar checks whether the given avatar key is in the predefined list.
func IsValidAvatar(avatar string) bool {
	for _, a := range PredefinedAvatars {
		if a == avatar {
			return true
		}
	}
	return false
}
