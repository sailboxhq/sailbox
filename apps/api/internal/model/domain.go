package model

import (
	"time"

	"github.com/google/uuid"
)

// Domain maps a custom domain to an application.
type Domain struct {
	BaseModel `bun:"table:domains,alias:dom"`

	AppID       uuid.UUID    `bun:"app_id,notnull,type:uuid" json:"app_id"`
	Application *Application `bun:"rel:belongs-to,join:app_id=id" json:"-"`

	Host       string `bun:"host,notnull,unique" json:"host"`
	TLS        bool   `bun:"tls" json:"tls"`
	AutoCert   bool   `bun:"auto_cert" json:"auto_cert"`
	CertSecret string `bun:"cert_secret" json:"cert_secret,omitempty"`

	ForceHTTPS   bool       `bun:"force_https" json:"force_https"`
	CertExpiry   *time.Time `bun:"cert_expiry" json:"cert_expiry,omitempty"`
	IngressReady bool       `bun:"ingress_ready" json:"ingress_ready"`
}
