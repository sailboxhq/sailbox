package model

// Template represents a pre-configured application template for one-click deploy.
type Template struct {
	BaseModel `bun:"table:templates,alias:tpl"`

	Name        string            `bun:"name,notnull,unique" json:"name"`
	Description string            `bun:"description" json:"description"`
	LogoURL     string            `bun:"logo_url" json:"logo_url,omitempty"`
	Category    string            `bun:"category" json:"category"`
	ComposeYAML string            `bun:"compose_yaml,type:text,notnull" json:"compose_yaml"`
	EnvSchema   map[string]EnvVar `bun:"env_schema,type:jsonb,default:'{}'" json:"env_schema"`
	MinCPU      string            `bun:"min_cpu,default:'250m'" json:"min_cpu"`
	MinMemory   string            `bun:"min_memory,default:'256Mi'" json:"min_memory"`
}

// EnvVar describes a template environment variable.
type EnvVar struct {
	Description string `json:"description"`
	Default     string `json:"default,omitempty"`
	Required    bool   `json:"required"`
	Secret      bool   `json:"secret"`
}
