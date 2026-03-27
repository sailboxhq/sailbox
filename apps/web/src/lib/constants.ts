import type { BadgeProps } from "@/components/ui/badge";

// ── Status → Badge variant mapping ─────────────────────────────────

export function statusVariant(status: string): NonNullable<BadgeProps["variant"]> {
  switch (status) {
    case "running":
    case "success":
    case "Running":
    case "Ready":
    case "Bound":
      return "success";
    case "Succeeded":
    case "Completed":
      return "secondary";
    case "stopped":
    case "idle":
    case "not deployed":
      return "outline";
    case "building":
    case "deploying":
    case "restarting":
    case "stopping":
    case "queued":
    case "Pending":
    case "pending":
    case "partial":
      return "warning";
    case "error":
    case "failed":
    case "Failed":
    case "Evicted":
      return "destructive";
    default:
      return "secondary";
  }
}

// ── Database engines ───────────────────────────────────────────────

export const ENGINE_LABELS: Record<string, string> = {
  postgres: "PostgreSQL",
  mysql: "MySQL",
  mariadb: "MariaDB",
  redis: "Redis",
  mongo: "MongoDB",
};

// ── Select option type ─────────────────────────────────────────────

export type SelectOption = { value: string; label: string };

// ── CPU presets ────────────────────────────────────────────────────

export const CPU_OPTIONS: SelectOption[] = [
  { value: "50m", label: "50m (Minimal)" },
  { value: "100m", label: "100m (Light)" },
  { value: "250m", label: "250m (Standard)" },
  { value: "500m", label: "500m (Medium)" },
  { value: "1000m", label: "1 Core" },
  { value: "2000m", label: "2 Cores" },
  { value: "4000m", label: "4 Cores" },
];

// ── Memory presets ─────────────────────────────────────────────────

export const MEMORY_OPTIONS: SelectOption[] = [
  { value: "64Mi", label: "64 Mi (Minimal)" },
  { value: "128Mi", label: "128 Mi (Light)" },
  { value: "256Mi", label: "256 Mi (Standard)" },
  { value: "512Mi", label: "512 Mi (Medium)" },
  { value: "1Gi", label: "1 Gi" },
  { value: "2Gi", label: "2 Gi" },
  { value: "4Gi", label: "4 Gi" },
  { value: "8Gi", label: "8 Gi" },
];

// ── Storage volume sizes ───────────────────────────────────────────

export const VOLUME_SIZE_OPTIONS: SelectOption[] = [
  { value: "1Gi", label: "1 Gi" },
  { value: "5Gi", label: "5 Gi" },
  { value: "10Gi", label: "10 Gi" },
  { value: "20Gi", label: "20 Gi" },
  { value: "50Gi", label: "50 Gi" },
  { value: "100Gi", label: "100 Gi" },
];

// ── Common container/service ports ─────────────────────────────────

export const COMMON_PORTS: SelectOption[] = [
  { value: "80", label: "80 (HTTP)" },
  { value: "443", label: "443 (HTTPS)" },
  { value: "3000", label: "3000 (Node.js)" },
  { value: "5000", label: "5000 (Flask)" },
  { value: "8000", label: "8000 (Django)" },
  { value: "8080", label: "8080 (Alt HTTP)" },
  { value: "8443", label: "8443 (Alt HTTPS)" },
  { value: "9090", label: "9090 (Prometheus)" },
];

// ── Common mount paths ─────────────────────────────────────────────

export const COMMON_MOUNT_PATHS: SelectOption[] = [
  { value: "/data", label: "/data" },
  { value: "/var/lib/data", label: "/var/lib/data" },
  { value: "/app/uploads", label: "/app/uploads" },
  { value: "/var/log", label: "/var/log" },
  { value: "/tmp", label: "/tmp" },
];

// ── Health check probe paths ───────────────────────────────────────

export const HEALTH_CHECK_PATHS: SelectOption[] = [
  { value: "/healthz", label: "/healthz" },
  { value: "/health", label: "/health" },
  { value: "/ready", label: "/ready" },
  { value: "/ping", label: "/ping" },
  { value: "/api/health", label: "/api/health" },
];

// ── Probe timing presets ───────────────────────────────────────────

export const INITIAL_DELAY_OPTIONS: SelectOption[] = [
  { value: "0", label: "0s" },
  { value: "5", label: "5s" },
  { value: "10", label: "10s" },
  { value: "15", label: "15s" },
  { value: "30", label: "30s" },
  { value: "60", label: "60s" },
];

export const PERIOD_OPTIONS: SelectOption[] = [
  { value: "5", label: "5s" },
  { value: "10", label: "10s" },
  { value: "15", label: "15s" },
  { value: "30", label: "30s" },
  { value: "60", label: "60s" },
];

export const TIMEOUT_OPTIONS: SelectOption[] = [
  { value: "1", label: "1s" },
  { value: "3", label: "3s" },
  { value: "5", label: "5s" },
  { value: "10", label: "10s" },
  { value: "15", label: "15s" },
];

export const FAILURE_THRESHOLD_OPTIONS: SelectOption[] = [
  { value: "1", label: "1x" },
  { value: "2", label: "2x" },
  { value: "3", label: "3x" },
  { value: "5", label: "5x" },
  { value: "10", label: "10x" },
];

// ── Graceful termination presets ───────────────────────────────────

export const GRACE_PERIOD_OPTIONS: SelectOption[] = [
  { value: "5", label: "5 seconds" },
  { value: "10", label: "10 seconds" },
  { value: "30", label: "30 seconds (default)" },
  { value: "60", label: "1 minute" },
  { value: "120", label: "2 minutes" },
  { value: "300", label: "5 minutes" },
];

// ── Deployment strategy surge/unavailable ──────────────────────────

export const SURGE_OPTIONS: SelectOption[] = [
  { value: "0", label: "0 (No extra pods)" },
  { value: "1", label: "1 pod" },
  { value: "25%", label: "25% (default)" },
  { value: "50%", label: "50%" },
  { value: "100%", label: "100%" },
];

// ── HPA replica presets ────────────────────────────────────────────

export const HPA_MIN_REPLICAS: SelectOption[] = [
  { value: "1", label: "1" },
  { value: "2", label: "2" },
  { value: "3", label: "3" },
  { value: "5", label: "5" },
];

export const HPA_MAX_REPLICAS: SelectOption[] = [
  { value: "3", label: "3" },
  { value: "5", label: "5" },
  { value: "10", label: "10" },
  { value: "20", label: "20" },
  { value: "50", label: "50" },
  { value: "100", label: "100" },
];

export const HPA_TARGET_OPTIONS: SelectOption[] = [
  { value: "0", label: "Disabled" },
  { value: "50", label: "50%" },
  { value: "60", label: "60%" },
  { value: "70", label: "70%" },
  { value: "80", label: "80%" },
  { value: "90", label: "90%" },
];

// ── Status dot color utility ──────────────────────────────────────

export function statusDotColor(status: string): string {
  switch (status) {
    case "running":
    case "Running":
    case "Ready":
    case "Bound":
      return "bg-green-500";
    case "building":
    case "deploying":
    case "restarting":
    case "stopping":
    case "Pending":
    case "pending":
    case "queued":
      return "bg-yellow-500";
    case "error":
    case "failed":
    case "Failed":
    case "Evicted":
      return "bg-red-500";
    case "stopped":
    case "idle":
    case "not deployed":
    case "Succeeded":
    case "Completed":
      return "bg-muted-foreground/40";
    default:
      return "bg-muted-foreground/40";
  }
}

// ── Dockerfile path presets ────────────────────────────────────────

export const DOCKERFILE_OPTIONS: SelectOption[] = [
  { value: "Dockerfile", label: "Dockerfile" },
  { value: "Dockerfile.prod", label: "Dockerfile.prod" },
  { value: "Dockerfile.dev", label: "Dockerfile.dev" },
  { value: "docker/Dockerfile", label: "docker/Dockerfile" },
];
