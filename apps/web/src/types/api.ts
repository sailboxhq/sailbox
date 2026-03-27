// ── Shared API response types ──────────────────────────────────────

export interface PaginatedResponse<T> {
  items: T[];
  pagination: { total: number; page: number; per_page: number };
}

// ── Auth ───────────────────────────────────────────────────────────

export interface UserInfo {
  id: string;
  display_name: string;
  first_name: string;
  last_name: string;
  email: string;
  role: string;
  avatar_url?: string;
  two_fa_enabled: boolean;
}

export interface AuthResponse {
  user: UserInfo;
  access_token: string;
  refresh_token: string;
}

// ── Projects ───────────────────────────────────────────────────────

export interface ResourceQuotaConfig {
  cpu_limit?: string;
  mem_limit?: string;
  pod_limit?: number;
  pvc_limit?: number;
  storage_limit?: string;
}

export interface Project {
  id: string;
  name: string;
  namespace: string;
  description: string;
  env_vars: Record<string, string>;
  resource_quota: ResourceQuotaConfig;
  network_policy_enabled: boolean;
  service_account: string;
  created_at: string;
}

// ── Applications ───────────────────────────────────────────────────

export interface HealthCheck {
  path: string;
  port: number;
  initial_delay_seconds: number;
  period_seconds: number;
  timeout_seconds: number;
  failure_threshold: number;
  type: string; // http | tcp | exec
  command?: string;
}

export interface AutoscalingConfig {
  enabled: boolean;
  min_replicas: number;
  max_replicas: number;
  cpu_target: number; // percentage
  mem_target: number; // percentage
}

export interface VolumeMount {
  name: string;
  mount_path: string;
  size: string;
  pvc_name?: string;
}

export interface DeployStrategyConfig {
  max_surge: string;
  max_unavailable: string;
}

export interface App {
  id: string;
  name: string;
  description: string;
  source_type: "image" | "git" | "compose";
  docker_image: string;
  git_repo: string;
  git_branch: string;
  build_type: string;
  dockerfile: string;
  status: string;
  replicas: number;
  cpu_limit: string;
  mem_limit: string;
  env_vars: Record<string, string>;
  ports: PortMapping[];
  // Advanced config
  health_check: HealthCheck;
  autoscaling: AutoscalingConfig;
  cpu_request: string;
  mem_request: string;
  volumes: VolumeMount[];
  deploy_strategy: string;
  deploy_strategy_config: DeployStrategyConfig;
  termination_grace_period: number;
  build_env_vars: Record<string, string>;
  build_context: string;
  watch_paths: string[];
  no_cache: boolean;
  node_pool: string;
  auto_deploy: boolean;
  // K8s mapping
  namespace: string;
  k8s_name: string;
  project_id: string;
  created_at: string;
}

export interface PortMapping {
  container_port: number;
  service_port: number;
  protocol: "tcp" | "udp";
}

export interface AppStatus {
  phase: string;
  ready_replicas: number;
  desired_replicas: number;
}

export interface ContainerStatus {
  name: string;
  ready: boolean;
  restart_count: number;
  state: string; // running | waiting | terminated
  reason: string;
}

export interface PodInfo {
  name: string;
  phase: string;
  node: string;
  ip: string;
  started_at: string;
  restart_count: number;
  ready: boolean;
  containers: ContainerStatus[];
  resources: {
    cpu_used: string;
    cpu_total: string;
    mem_used: string;
    mem_total: string;
  };
}

export interface PodEvent {
  type: string; // Normal | Warning
  reason: string;
  message: string;
  count: number;
  first_seen: string;
  last_seen: string;
}

// ── Deployments ────────────────────────────────────────────────────

export interface Deployment {
  id: string;
  app_id: string;
  project_id?: string;
  status: string;
  commit_sha: string;
  trigger_type: string;
  image: string;
  build_log?: string;
  app_name?: string;
  created_at: string;
  started_at?: string;
  finished_at?: string;
}

// ── CronJobs ────────────────────────────────────────────────────

export interface CronJob {
  id: string;
  project_id: string;
  name: string;
  description: string;
  cron_expression: string;
  timezone: string;
  command: string;
  image: string;
  source_type: "image" | "git";
  git_repo: string;
  git_branch: string;
  env_vars: Record<string, string>;
  cpu_limit: string;
  mem_limit: string;
  enabled: boolean;
  concurrency_policy: string;
  restart_policy: string;
  backoff_limit: number;
  active_deadline_seconds: number;
  namespace: string;
  k8s_name: string;
  last_run_at?: string;
  status: string;
  created_at: string;
}

export interface CronJobRun {
  id: string;
  cron_job_id: string;
  status: string;
  started_at: string;
  finished_at?: string;
  exit_code?: number;
  logs: string;
  trigger_type: string;
  created_at: string;
}

// ── Domains ────────────────────────────────────────────────────────

export interface Domain {
  id: string;
  host: string;
  tls: boolean;
  auto_cert: boolean;
  force_https: boolean;
  cert_expiry?: string;
  ingress_ready: boolean;
}

// ── Databases ──────────────────────────────────────────────────────

export interface ManagedDB {
  id: string;
  name: string;
  database_name: string;
  engine: string;
  version: string;
  status: string;
  storage_size: string;
  cpu_limit: string;
  mem_limit: string;
  created_at: string;
  project_id: string;
  external_port: number;
  external_enabled: boolean;
  backup_enabled: boolean;
  backup_schedule: string;
  backup_s3_id?: string;
}

export interface DBVersionInfo {
  tag: string;
  label: string;
  is_recommended: boolean;
}

export interface DatabaseCredentials {
  host: string;
  port: number;
  username: string;
  password: string;
  database_name: string;
  connection_string: string;
  internal_url: string;
}

export interface DatabaseBackup {
  id: string;
  database_id: string;
  status: string;
  restore_status?: string;
  size_bytes: number;
  file_path: string;
  started_at?: string;
  finished_at?: string;
  created_at: string;
}

// ── Cluster ────────────────────────────────────────────────────────

export interface NodeInfo {
  name: string;
  ip: string;
  status: string;
  roles: string[];
  pool?: string;
  version: string;
  os: string;
  arch: string;
  resources: {
    cpu_used: string;
    cpu_total: string;
    mem_used: string;
    mem_total: string;
    storage_used: string;
    storage_total: string;
  };
}

export interface ClusterMetrics {
  nodes: number;
  total_pods: number;
  running_pods: number;
  resources: {
    cpu_used: string;
    cpu_total: string;
    mem_used: string;
    mem_total: string;
  };
}

// ── Cluster monitoring ──────────────────────────────────────────────

export interface ClusterEvent {
  type: string;
  reason: string;
  message: string;
  namespace: string;
  involved_object: string;
  count: number;
  first_seen: string;
  last_seen: string;
}

export interface PVCInfo {
  name: string;
  namespace: string;
  status: string;
  capacity: string;
  storage_class: string;
  volume_name: string;
  used_by?: string[];
}

export interface NamespaceInfo {
  name: string;
  status: string;
  pod_count: number;
  svc_count: number;
}

export interface NodeMetrics {
  name: string;
  cpu_used: string;
  cpu_total: string;
  mem_used: string;
  mem_total: string;
  pod_count: number;
}

// ── Cluster Topology ────────────────────────────────────────────

export interface ClusterTopology {
  nodes: TopologyNode[];
  deployments: TopologyDeployment[];
  pods: TopologyPod[];
  services: TopologyService[];
  ingresses: TopologyIngress[];
}

export interface TopologyNode {
  name: string;
  status: string;
  ip: string;
  roles: string;
}

export interface TopologyDeployment {
  name: string;
  namespace: string;
  ready: number;
  desired: number;
  app_id?: string;
}

export interface TopologyPod {
  name: string;
  namespace: string;
  phase: string;
  node: string;
  ip: string;
  app_id?: string;
  deployment?: string;
}

export interface TopologyService {
  name: string;
  namespace: string;
  type: string;
  cluster_ip: string;
  ports: string;
  app_id?: string;
}

export interface TopologyIngress {
  name: string;
  namespace: string;
  host: string;
  service: string;
  app_id?: string;
}

// ── Monitoring (metrics schema) ─────────────────────────────────

export interface MetricSnapshot {
  collected_at: string;
  source_type: "node" | "app";
  source_name: string;
  cpu_used: number; // millicores
  cpu_total: number;
  mem_used: number; // bytes
  mem_total: number;
  disk_used?: number;
  disk_total?: number;
  pod_count?: number;
}

export interface MetricEvent {
  id: string;
  recorded_at: string;
  event_type: "Normal" | "Warning";
  reason: string;
  message: string;
  namespace: string;
  involved_object: string;
  source_component: string;
  first_seen: string;
  last_seen: string;
  count: number;
}

export interface MetricAlert {
  id: string;
  rule_name: string;
  severity: "critical" | "warning" | "info";
  source_type: string;
  source_name: string;
  message: string;
  fired_at: string;
  resolved_at?: string;
  notified: boolean;
}

// ── Server Nodes ──────────────────────────────────────────────────

export interface ServerNode {
  id: string;
  name: string;
  host: string;
  port: number;
  ssh_user: string;
  auth_type: string; // password | ssh_key
  ssh_key_id?: string;
  role: string; // worker | server
  status: string; // pending | initializing | ready | error | offline
  status_msg: string;
  k8s_node_name: string;
  created_at: string;
}

// ── Shared Resources ──────────────────────────────────────────────

export interface SharedResource {
  id: string;
  org_id: string;
  name: string;
  type: string; // git_provider | registry | ssh_key | object_storage
  provider: string; // github | gitlab | dockerhub | ghcr | custom
  config: Record<string, string>;
  status: string;
  created_at: string;
}

// ── Webhook Config ────────────────────────────────────────────────

export interface WebhookConfig {
  webhook_url: string;
  secret: string;
  auto_deploy: boolean;
}

// ── Team ────────────────────────────────────────────────────────────

export interface TeamMember {
  id: string;
  email: string;
  display_name: string;
  first_name: string;
  last_name: string;
  avatar_url: string;
  role: string;
  created_at: string;
}

export interface Invitation {
  id: string;
  email: string;
  role: string;
  invited_by?: string;
  expires_at: string;
  accepted_at?: string;
  created_at: string;
}

// ── Notifications ─────────────────────────────────────────────────

export interface NotificationChannel {
  id: string;
  type: "email" | "telegram" | "discord" | "slack";
  enabled: boolean;
  config: Record<string, string>;
}

export interface SMTPConfig {
  host: string;
  port: string;
  user: string;
  password: string;
  from: string;
  enabled: boolean;
}

// ── Settings ───────────────────────────────────────────────────────

export type Settings = Record<string, string>;

// ── Service union (used in project detail) ─────────────────────────

export type ServiceItem =
  | { type: "app"; data: App }
  | { type: "database"; data: ManagedDB }
  | { type: "cronjob"; data: CronJob };

// ── Traefik ──────────────────────────────────────────────────────────

export interface TraefikConfig {
  yaml: string;
}

// ── Helm ─────────────────────────────────────────────────────────────

export interface HelmRelease {
  name: string;
  namespace: string;
  chart: string;
  version: string;
  app_version: string;
  status: string;
  updated: string;
}

// ── Cluster Cleanup ──────────────────────────────────────────────────

export interface CleanupStats {
  evicted_pods: number;
  evicted_pod_names: string[];
  failed_pods: number;
  failed_pod_names: string[];
  completed_pods: number;
  completed_pod_names: string[];
  stale_replicasets: number;
  stale_rs_names: string[];
  completed_jobs: number;
  completed_job_names: string[];
  unbound_pvcs: number;
  unbound_pvc_names: string[];
  orphan_ingresses: number;
  orphan_ingress_names: string[];
}

export interface CleanupResult {
  deleted: number;
  message: string;
}

// ── DaemonSets ───────────────────────────────────────────────────────

// ── Version ─────────────────────────────────────────────────────────

export interface VersionInfo {
  current: string;
  latest: string;
  update_available: boolean;
  release_url: string;
  changelog: string;
  published_at: string;
}

// ── System Backup ────────────────────────────────────────────────────

export interface SystemBackupConfig {
  enabled: boolean;
  s3_id: string;
  schedule: string;
  path: string;
  retention: number;
}

export interface S3BackupFile {
  key: string;
  file_name: string;
  size_bytes: number;
  last_modified: string;
}

export interface SystemBackup {
  id: string;
  status: string;
  size_bytes: number;
  file_name: string;
  s3_bucket: string;
  s3_path: string;
  error?: string;
  started_at?: string;
  finished_at?: string;
  created_at: string;
}

// ── DaemonSets ───────────────────────────────────────────────────────

export interface DaemonSetInfo {
  name: string;
  namespace: string;
  desired_scheduled: number;
  current_scheduled: number;
  ready: number;
  node_selector: string;
  images: string;
  created_at: string;
}
