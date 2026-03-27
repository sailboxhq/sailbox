package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all application configuration.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Valkey   ValkeyConfig
	K8s      K8sConfig
	Auth     AuthConfig
}

type ServerConfig struct {
	Host            string
	Port            int
	ShutdownTimeout time.Duration
	AppURL          string // Public URL of the Sailbox instance (e.g. https://sailbox.example.com)
}

type DatabaseConfig struct {
	URL             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type ValkeyConfig struct {
	URL string
}

type K8sConfig struct {
	Kubeconfig string
	InCluster  bool
}

type AuthConfig struct {
	JWTSecret     string
	TokenExpiry   time.Duration
	RefreshExpiry time.Duration
}

// Load reads configuration from .env file (if present) and environment variables.
func Load() (*Config, error) {
	// Load .env file silently — not required in production
	_ = godotenv.Load()
	cfg := &Config{
		Server: ServerConfig{
			Host:            envStr("SERVER_HOST", "0.0.0.0"),
			Port:            envInt("SERVER_PORT", 8080),
			ShutdownTimeout: envDuration("SERVER_SHUTDOWN_TIMEOUT", 15*time.Second),
			AppURL:          envStr("APP_URL", "http://localhost:3000"),
		},
		Database: DatabaseConfig{
			URL:             envStr("DATABASE_URL", "postgres://sailbox:sailbox@localhost:5433/sailbox?sslmode=disable"),
			MaxOpenConns:    envInt("DATABASE_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    envInt("DATABASE_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: envDuration("DATABASE_CONN_MAX_LIFETIME", 5*time.Minute),
		},
		Valkey: ValkeyConfig{
			URL: envStr("VALKEY_URL", "redis://localhost:6380/0"),
		},
		K8s: K8sConfig{
			Kubeconfig: envStr("KUBECONFIG", ""),
			InCluster:  envBool("K8S_IN_CLUSTER", false),
		},
		Auth: AuthConfig{
			JWTSecret:     envStr("JWT_SECRET", ""),
			TokenExpiry:   envDuration("JWT_TOKEN_EXPIRY", 24*time.Hour),
			RefreshExpiry: envDuration("JWT_REFRESH_EXPIRY", 7*24*time.Hour),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.Auth.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}
	return nil
}

func (c *Config) ListenAddr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// Environment helpers

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
