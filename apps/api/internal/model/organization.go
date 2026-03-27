package model

// Organization is the top-level tenant for multi-tenancy.
type Organization struct {
	BaseModel `bun:"table:organizations,alias:org"`

	Name        string    `bun:"name,notnull,unique" json:"name"`
	Description string    `bun:"description" json:"description"`
	Users       []User    `bun:"rel:has-many,join:id=org_id" json:"-"`
	Projects    []Project `bun:"rel:has-many,join:id=org_id" json:"-"`
}
