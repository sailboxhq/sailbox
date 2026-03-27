package model

import "time"

// Setting stores global key-value configuration in the database.
type Setting struct {
	Key       string    `bun:"key,pk" json:"key"`
	Value     string    `bun:"value,notnull" json:"value"`
	UpdatedAt time.Time `bun:"updated_at,notnull,default:current_timestamp" json:"updated_at"`
}

// Well-known setting keys
const (
	SettingBaseDomain  = "base_domain"  // e.g. "203.0.113.5.sslip.io" or "mysite.com"
	SettingServerIP    = "server_ip"    // auto-detected server IP
	SettingSetupDone   = "setup_done"   // "true" after first-time setup
	SettingPanelDomain = "panel_domain" // domain for the Sailbox panel (e.g. "panel.example.com")
	SettingHTTPSEmail  = "https_email"  // email for Let's Encrypt ACME certificates
)
