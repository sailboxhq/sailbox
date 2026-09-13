package model

import (
	"strings"
	"time"
)

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

// secretSettings are settings whose value is a credential. They are stored in
// the same table as ordinary settings but must never be sent to a client: the
// k3s token alone is enough to take over the cluster, and the GitHub App key
// grants access to every connected repository.
var secretSettings = map[string]bool{
	"k3s_token":                 true,
	"github_app_pem":            true,
	"github_app_client_secret":  true,
	"github_app_webhook_secret": true,
	"smtp_password":             true,
}

// writableSettings are the keys a client may change through the settings API.
// Everything else (credentials, values Sailbox derives itself such as
// server_ip) is written by the server only.
var writableSettings = map[string]bool{
	SettingBaseDomain:  true,
	SettingPanelDomain: true,
	SettingHTTPSEmail:  true,
}

// internalSettingPrefixes cover keys the settings table uses as scratch space
// rather than configuration: one-time OAuth state, keyed by nonce.
//
// These must not merely be redacted, because the nonce lives in the *key*: an
// unconsumed state value is what binds a GitHub callback to a specific user, so
// learning the key is enough to hijack that binding. They are omitted from the
// settings listing entirely.
var internalSettingPrefixes = []string{
	"github_oauth_state_",
	"github_setup_state_",
}

// IsSecretSetting reports whether a setting key holds a credential.
func IsSecretSetting(key string) bool { return secretSettings[key] }

// IsInternalSetting reports whether a setting key is server-side scratch space
// that no client should see at all, key included.
func IsInternalSetting(key string) bool {
	for _, prefix := range internalSettingPrefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

// IsWritableSetting reports whether a setting key may be set through the API.
func IsWritableSetting(key string) bool { return writableSettings[key] }

// RedactedValue is returned in place of a configured secret so a client can
// tell "set" from "not set" without learning the value.
const RedactedValue = "••••••••"
