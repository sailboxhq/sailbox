package orchestrator

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/sailboxhq/sailbox/apps/api/internal/model"
)

// Orchestrator defines the contract for container orchestration.
// K3s implements this today; standard K8s can implement it tomorrow.
type Orchestrator interface {
	AppManager
	DatabaseManager
	IngressManager
	StorageManager
	ClusterInspector
	LogStreamer
	TerminalExec
	BuildManager
	NamespaceManager
	SecretManager
	CronJobManager
	ConfigMapManager
	ResourceQuotaManager
	NetworkPolicyManager
	TraefikManager
	HelmInspector
	DaemonSetInspector
	ServiceAccountManager
	CleanupInspector
}

// TraefikManager handles Traefik configuration.
type TraefikManager interface {
	GetTraefikConfig(ctx context.Context) (string, error)
	UpdateTraefikConfig(ctx context.Context, yaml string) error
	RestartTraefik(ctx context.Context) error
	GetTraefikStatus(ctx context.Context) (*TraefikStatus, error)
}

type TraefikStatus struct {
	Ready    bool   `json:"ready"`
	PodName  string `json:"pod_name"`
	Restarts int32  `json:"restarts"`
	Age      string `json:"age"`
}

// HelmInspector provides information about Helm releases.
type HelmInspector interface {
	GetHelmReleases(ctx context.Context) ([]HelmRelease, error)
}

// DaemonSetInspector provides information about DaemonSets.
type DaemonSetInspector interface {
	GetDaemonSets(ctx context.Context) ([]DaemonSetInfo, error)
}

// ServiceAccountManager handles K8s ServiceAccount lifecycle.
type ServiceAccountManager interface {
	EnsureServiceAccount(ctx context.Context, namespace, name string) error
	DeleteServiceAccount(ctx context.Context, namespace, name string) error
}

// AppManager handles application lifecycle.
type AppManager interface {
	Deploy(ctx context.Context, app *model.Application, opts DeployOpts) error
	Rollback(ctx context.Context, app *model.Application, revision int64) error
	Scale(ctx context.Context, app *model.Application, replicas int32) error
	UpdateEnvVars(ctx context.Context, app *model.Application, envVars map[string]string) error
	Restart(ctx context.Context, app *model.Application) error
	Stop(ctx context.Context, app *model.Application) error
	Delete(ctx context.Context, app *model.Application) error
	GetStatus(ctx context.Context, app *model.Application) (*AppStatus, error)
	GetPods(ctx context.Context, app *model.Application) ([]PodInfo, error)
	DeletePod(ctx context.Context, podName string, app *model.Application) error
	GetPodEvents(ctx context.Context, app *model.Application, podName string) ([]PodEvent, error)
	ConfigureHPA(ctx context.Context, app *model.Application, cfg model.AutoscalingConfig) error
	DeleteHPA(ctx context.Context, app *model.Application) error
}

// DatabaseManager handles managed database lifecycle.
type DatabaseManager interface {
	DeployDatabase(ctx context.Context, db *model.ManagedDatabase) error
	DeleteDatabase(ctx context.Context, db *model.ManagedDatabase) error
	GetDatabaseStatus(ctx context.Context, db *model.ManagedDatabase) (*AppStatus, error)
	GetDatabaseCredentials(ctx context.Context, db *model.ManagedDatabase) (*DatabaseCredentials, error)
	GetDatabasePods(ctx context.Context, db *model.ManagedDatabase) ([]PodInfo, error)
	RunDatabaseBackup(ctx context.Context, db *model.ManagedDatabase, backupID uuid.UUID, s3Config *S3Config, s3Key string) error
	RestoreDatabaseBackup(ctx context.Context, db *model.ManagedDatabase, backupID uuid.UUID, s3Config *S3Config, s3Key string) error
	GetBackupJobStatus(ctx context.Context, backupID uuid.UUID) string  // returns "completed", "failed", or ""
	GetRestoreJobStatus(ctx context.Context, backupID uuid.UUID) string // returns "completed", "failed", or ""
	EnableExternalAccess(ctx context.Context, db *model.ManagedDatabase) (int32, error)
	DisableExternalAccess(ctx context.Context, db *model.ManagedDatabase) error
}

// IngressManager handles domain routing and TLS.
type IngressManager interface {
	CreateIngress(ctx context.Context, domain *model.Domain, app *model.Application) error
	UpdateIngress(ctx context.Context, domain *model.Domain, app *model.Application) error
	DeleteIngress(ctx context.Context, domain *model.Domain) error
	DeleteIngressByName(ctx context.Context, app *model.Application, name string) error
	IngressName(app *model.Application, host string) string
	LegacyIngressName(app *model.Application, host string) string
	GetIngressStatus(ctx context.Context, domain *model.Domain, app *model.Application) (*IngressStatus, error)
	GetCertExpiry(ctx context.Context, domain *model.Domain, app *model.Application) (*time.Time, error)
	EnsurePanelIngress(ctx context.Context, domain, httpsEmail string) error
	DeletePanelIngress(ctx context.Context) error
}

// StorageManager handles persistent volumes.
type StorageManager interface {
	CreateVolume(ctx context.Context, opts VolumeOpts) (string, error)
	DeleteVolume(ctx context.Context, name, namespace string) error
	ExpandVolume(ctx context.Context, name, namespace, newSize string) error
}

// NamespaceManager handles namespace lifecycle.
type NamespaceManager interface {
	CreateNamespace(ctx context.Context, name string) error
	DeleteNamespace(ctx context.Context, name string) error
}

// SecretManager handles K8s secret lifecycle.
type SecretManager interface {
	EnsureSecret(ctx context.Context, app *model.Application, secrets map[string]string) error
}

// CronJobManager handles K8s CronJob lifecycle.
type CronJobManager interface {
	CreateCronJob(ctx context.Context, cj *model.CronJob) error
	UpdateCronJob(ctx context.Context, cj *model.CronJob) error
	DeleteCronJob(ctx context.Context, cj *model.CronJob) error
	SuspendCronJob(ctx context.Context, cj *model.CronJob, suspend bool) error
	TriggerCronJob(ctx context.Context, cj *model.CronJob) (string, error) // returns job name
	GetJobStatus(ctx context.Context, cj *model.CronJob, jobName string) (string, error)
}

// ConfigMapManager handles K8s ConfigMap lifecycle.
type ConfigMapManager interface {
	EnsureConfigMap(ctx context.Context, namespace, name string, data map[string]string) error
	DeleteConfigMap(ctx context.Context, namespace, name string) error
}

// ResourceQuotaManager handles K8s ResourceQuota lifecycle.
type ResourceQuotaManager interface {
	EnsureResourceQuota(ctx context.Context, namespace string, quota model.ResourceQuotaConfig) error
	DeleteResourceQuota(ctx context.Context, namespace string) error
}

// NetworkPolicyManager handles K8s NetworkPolicy lifecycle.
type NetworkPolicyManager interface {
	EnsureNetworkPolicy(ctx context.Context, namespace string, enabled bool) error
}

// ClusterInspector provides cluster-wide information.
type ClusterInspector interface {
	GetNodes(ctx context.Context) ([]NodeInfo, error)
	GetClusterMetrics(ctx context.Context) (*ClusterMetrics, error)
	GetNamespaceMetrics(ctx context.Context, namespace string) (*ResourceMetrics, error)
	GetAllPods(ctx context.Context) ([]PodInfo, error)
	GetClusterEvents(ctx context.Context, limit int) ([]ClusterEvent, error)
	GetPVCs(ctx context.Context) ([]PVCInfo, error)
	GetNamespaces(ctx context.Context) ([]NamespaceInfo, error)
	GetNodeMetrics(ctx context.Context) ([]NodeMetrics, error)
	GetClusterTopology(ctx context.Context) (*ClusterTopology, error)
	SetNodeLabel(ctx context.Context, nodeName, key, value string) error
	RemoveNodeLabel(ctx context.Context, nodeName, key string) error
	GetNodePools(ctx context.Context) ([]string, error)
}

// ClusterTopology represents the full resource graph of the cluster.
type ClusterTopology struct {
	Nodes       []TopologyNode       `json:"nodes"`
	Deployments []TopologyDeployment `json:"deployments"`
	Pods        []TopologyPod        `json:"pods"`
	Services    []TopologyService    `json:"services"`
	Ingresses   []TopologyIngress    `json:"ingresses"`
}

type TopologyNode struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	IP     string `json:"ip"`
	Roles  string `json:"roles"`
}

type TopologyDeployment struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Ready     int32  `json:"ready"`
	Desired   int32  `json:"desired"`
	AppID     string `json:"app_id,omitempty"`
}

type TopologyPod struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Phase      string `json:"phase"`
	Node       string `json:"node"`
	IP         string `json:"ip"`
	AppID      string `json:"app_id,omitempty"`
	Deployment string `json:"deployment,omitempty"`
}

type TopologyService struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Type      string `json:"type"`
	ClusterIP string `json:"cluster_ip"`
	Ports     string `json:"ports"`
	AppID     string `json:"app_id,omitempty"`
}

type TopologyIngress struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Host      string `json:"host"`
	Service   string `json:"service"`
	AppID     string `json:"app_id,omitempty"`
}

// LogStreamer provides real-time log streaming.
type LogStreamer interface {
	StreamLogs(ctx context.Context, app *model.Application, opts LogOpts) (io.ReadCloser, error)
	StreamPodLogs(ctx context.Context, app *model.Application, podName string, opts LogOpts) (io.ReadCloser, error)
}

// TerminalExec provides interactive terminal access to containers.
type TerminalExec interface {
	ExecTerminal(ctx context.Context, app *model.Application, opts ExecOpts) (TerminalSession, error)
}

// BuildManager handles image building from source code.
type BuildManager interface {
	Build(ctx context.Context, app *model.Application, opts BuildOpts) (*BuildResult, error)
	EnsureRegistry(ctx context.Context) error
	GetBuildLogs(ctx context.Context, jobName, namespace string) (io.ReadCloser, error)
	ClearBuildCache(ctx context.Context, app *model.Application) error
	CancelBuild(ctx context.Context, app *model.Application) error
}

// ============================================================================
// Value Objects
// ============================================================================

type DeployOpts struct {
	Image       string
	Replicas    int32
	EnvVars     map[string]string
	Ports       []model.PortMapping
	CPULimit    string
	MemLimit    string
	Annotations map[string]string

	CPURequest             string
	MemRequest             string
	HealthCheck            *model.HealthCheck
	Volumes                []model.VolumeMount
	DeployStrategy         string
	DeployStrategyConfig   *model.DeployStrategyConfig
	TerminationGracePeriod int
	NodePool               string
}

type AppStatus struct {
	Phase            string    `json:"phase"` // running | pending | failed | unknown
	ReadyReplicas    int32     `json:"ready_replicas"`
	DesiredReplicas  int32     `json:"desired_replicas"`
	LastTransitionAt time.Time `json:"last_transition_at"`
	Message          string    `json:"message,omitempty"`
}

type PodInfo struct {
	Name         string            `json:"name"`
	Namespace    string            `json:"namespace"`
	Phase        string            `json:"phase"`
	Node         string            `json:"node"`
	IP           string            `json:"ip"`
	StartedAt    time.Time         `json:"started_at"`
	Resources    ResourceMetrics   `json:"resources"`
	RestartCount int32             `json:"restart_count"`
	Ready        bool              `json:"ready"`
	Containers   []ContainerStatus `json:"containers"`
	AppID        string            `json:"app_id,omitempty"`
}

type PodEvent struct {
	Type      string    `json:"type"` // Normal | Warning
	Reason    string    `json:"reason"`
	Message   string    `json:"message"`
	Count     int32     `json:"count"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
}

type ContainerStatus struct {
	Name         string `json:"name"`
	Ready        bool   `json:"ready"`
	RestartCount int32  `json:"restart_count"`
	State        string `json:"state"`  // running | waiting | terminated
	Reason       string `json:"reason"` // CrashLoopBackOff, etc.
}

type IngressStatus struct {
	Ready      bool   `json:"ready"`
	Message    string `json:"message,omitempty"`
	CertSecret string `json:"cert_secret,omitempty"` // TLS secret name from ingress spec
}

type NodeInfo struct {
	Name      string          `json:"name"`
	IP        string          `json:"ip"`
	Status    string          `json:"status"`
	Roles     []string        `json:"roles"`
	Pool      string          `json:"pool,omitempty"`
	Version   string          `json:"version"`
	OS        string          `json:"os"`
	Arch      string          `json:"arch"`
	Resources ResourceMetrics `json:"resources"`
}

type ClusterMetrics struct {
	Nodes       int             `json:"nodes"`
	TotalPods   int             `json:"total_pods"`
	RunningPods int             `json:"running_pods"`
	Resources   ResourceMetrics `json:"resources"`
}

type ResourceMetrics struct {
	CPUUsed      string `json:"cpu_used"`
	CPUTotal     string `json:"cpu_total"`
	MemUsed      string `json:"mem_used"`
	MemTotal     string `json:"mem_total"`
	StorageUsed  string `json:"storage_used,omitempty"`
	StorageTotal string `json:"storage_total,omitempty"`
}

type ClusterEvent struct {
	Type           string    `json:"type"`
	Reason         string    `json:"reason"`
	Message        string    `json:"message"`
	Namespace      string    `json:"namespace"`
	InvolvedObject string    `json:"involved_object"`
	Count          int32     `json:"count"`
	FirstSeen      time.Time `json:"first_seen"`
	LastSeen       time.Time `json:"last_seen"`
}

type PVCInfo struct {
	Name         string   `json:"name"`
	Namespace    string   `json:"namespace"`
	Status       string   `json:"status"`
	Capacity     string   `json:"capacity"`
	StorageClass string   `json:"storage_class"`
	VolumeName   string   `json:"volume_name"`
	UsedBy       []string `json:"used_by"` // Pod names using this PVC
}

type NamespaceInfo struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	PodCount int    `json:"pod_count"`
	SvcCount int    `json:"svc_count"`
}

type NodeMetrics struct {
	Name     string `json:"name"`
	CPUUsed  string `json:"cpu_used"`
	CPUTotal string `json:"cpu_total"`
	MemUsed  string `json:"mem_used"`
	MemTotal string `json:"mem_total"`
	PodCount int    `json:"pod_count"`
}

type VolumeOpts struct {
	Name         string
	Namespace    string
	Size         string
	StorageClass string
}

type LogOpts struct {
	Container  string
	Follow     bool
	TailLines  int64
	Since      time.Time
	Timestamps bool
}

type ExecOpts struct {
	Container string
	Command   []string
	TTY       bool
}

// TerminalSession represents a bidirectional terminal connection.
type TerminalSession interface {
	io.Reader
	io.Writer
	Resize(width, height uint16) error
	Close() error
}

// LogCallback is called periodically with incremental build logs during a build.
// The caller can use it to persist logs for real-time display.
type LogCallback func(logs string)

type BuildOpts struct {
	GitRepo      string
	GitBranch    string
	CommitSHA    string
	GitToken     string // access token for private repo cloning
	Dockerfile   string
	BuildContext string // subdirectory for build context (default ".")
	BuildArgs    map[string]string
	BuildEnvVars map[string]string
	BuildType    string // dockerfile | nixpacks
	NoCache      bool
	OnLog        LogCallback
}

type BuildResult struct {
	Image    string
	Duration time.Duration
	Logs     string
}

type HelmRelease struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Chart     string `json:"chart"`
	Revision  string `json:"revision"`
	Status    string `json:"status"`
	Updated   string `json:"updated"`
}

// CleanupInspector provides cluster cleanup operations.
type CleanupInspector interface {
	GetCleanupStats(ctx context.Context) (*CleanupStats, error)
	CleanupEvictedPods(ctx context.Context) (*CleanupResult, error)
	CleanupFailedPods(ctx context.Context) (*CleanupResult, error)
	CleanupCompletedPods(ctx context.Context) (*CleanupResult, error)
	CleanupStaleReplicaSets(ctx context.Context) (*CleanupResult, error)
	CleanupCompletedJobs(ctx context.Context) (*CleanupResult, error)
	// GetOrphanIngresses returns sailbox-managed ingresses not in the provided valid hosts set.
	// systemIngresses maps "namespace/name" → expected host for system-managed ingresses
	// (e.g. the panel) that should be validated by resource identity, not the global host list.
	GetOrphanIngresses(ctx context.Context, validHosts map[string]bool, systemIngresses map[string]string) ([]string, error)
	// CleanupOrphanIngresses deletes sailbox-managed ingresses not in the valid hosts set.
	CleanupOrphanIngresses(ctx context.Context, validHosts map[string]bool, systemIngresses map[string]string) (*CleanupResult, error)
}

type CleanupStats struct {
	EvictedPods        int      `json:"evicted_pods"`
	EvictedPodNames    []string `json:"evicted_pod_names"`
	FailedPods         int      `json:"failed_pods"`
	FailedPodNames     []string `json:"failed_pod_names"`
	CompletedPods      int      `json:"completed_pods"`
	CompletedPodNames  []string `json:"completed_pod_names"`
	StaleReplicaSets   int      `json:"stale_replicasets"`
	StaleRSNames       []string `json:"stale_rs_names"`
	CompletedJobs      int      `json:"completed_jobs"`
	CompletedJobNames  []string `json:"completed_job_names"`
	UnboundPVCs        int      `json:"unbound_pvcs"`
	UnboundPVCNames    []string `json:"unbound_pvc_names"`
	OrphanIngresses    int      `json:"orphan_ingresses"`
	OrphanIngressNames []string `json:"orphan_ingress_names"`
}

type CleanupResult struct {
	Deleted int    `json:"deleted"`
	Message string `json:"message"`
}

// S3Config holds credentials for S3-compatible object storage used for database backups.
type S3Config struct {
	Endpoint  string `json:"endpoint"`
	Bucket    string `json:"bucket"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	Region    string `json:"region"`
}

type DatabaseCredentials struct {
	Host             string `json:"host"`
	Port             int32  `json:"port"`
	Username         string `json:"username"`
	Password         string `json:"password"`
	DatabaseName     string `json:"database_name"`
	ConnectionString string `json:"connection_string"`
	InternalURL      string `json:"internal_url"`
}

type DaemonSetInfo struct {
	Name             string `json:"name"`
	Namespace        string `json:"namespace"`
	DesiredScheduled int32  `json:"desired_scheduled"`
	CurrentScheduled int32  `json:"current_scheduled"`
	Ready            int32  `json:"ready"`
	NodeSelector     string `json:"node_selector"`
	Images           string `json:"images"`
	CreatedAt        string `json:"created_at"`
}
