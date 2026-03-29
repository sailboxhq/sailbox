package orchestrator

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/sailboxhq/sailbox/apps/api/internal/model"
)

// NoopOrchestrator is a stub orchestrator for development without K3s.
// It logs operations but does not actually create any resources.
type NoopOrchestrator struct {
	logger *slog.Logger
}

func NewNoop(logger *slog.Logger) *NoopOrchestrator {
	return &NoopOrchestrator{logger: logger}
}

func (n *NoopOrchestrator) Deploy(ctx context.Context, app *model.Application, opts DeployOpts) error {
	n.logger.Info("[noop] deploy", slog.String("app", app.Name))
	return nil
}

func (n *NoopOrchestrator) Rollback(ctx context.Context, app *model.Application, revision int64) error {
	n.logger.Info("[noop] rollback", slog.String("app", app.Name))
	return nil
}

func (n *NoopOrchestrator) Scale(ctx context.Context, app *model.Application, replicas int32) error {
	n.logger.Info("[noop] scale", slog.String("app", app.Name), slog.Int("replicas", int(replicas)))
	return nil
}

func (n *NoopOrchestrator) UpdateEnvVars(ctx context.Context, app *model.Application, envVars map[string]string) error {
	n.logger.Info("[noop] update env vars", slog.String("app", app.Name), slog.Int("count", len(envVars)))
	return nil
}

func (n *NoopOrchestrator) Restart(ctx context.Context, app *model.Application) error {
	n.logger.Info("[noop] restart", slog.String("app", app.Name))
	return nil
}

func (n *NoopOrchestrator) Stop(ctx context.Context, app *model.Application) error {
	n.logger.Info("[noop] stop", slog.String("app", app.Name))
	return nil
}

func (n *NoopOrchestrator) Delete(ctx context.Context, app *model.Application) error {
	n.logger.Info("[noop] delete", slog.String("app", app.Name))
	return nil
}

func (n *NoopOrchestrator) GetStatus(ctx context.Context, app *model.Application) (*AppStatus, error) {
	return &AppStatus{Phase: "running", ReadyReplicas: app.Replicas, DesiredReplicas: app.Replicas, LastTransitionAt: time.Now()}, nil
}

func (n *NoopOrchestrator) GetPods(ctx context.Context, app *model.Application) ([]PodInfo, error) {
	// Simulate pods based on replicas
	pods := make([]PodInfo, 0, app.Replicas)
	for i := int32(0); i < app.Replicas; i++ {
		pods = append(pods, PodInfo{
			Name:         fmt.Sprintf("%s-%d", app.Name, i),
			Phase:        "Running",
			Node:         "sailbox-server",
			IP:           fmt.Sprintf("10.42.0.%d", 10+i),
			StartedAt:    time.Now().Add(-time.Duration(i+1) * time.Hour),
			Resources:    ResourceMetrics{CPUUsed: "50m", CPUTotal: app.CPULimit, MemUsed: "64Mi", MemTotal: app.MemLimit},
			RestartCount: 0,
			Ready:        true,
			Containers: []ContainerStatus{
				{
					Name:         app.Name,
					Ready:        true,
					RestartCount: 0,
					State:        "running",
				},
			},
		})
	}
	return pods, nil
}

func (n *NoopOrchestrator) DeletePod(ctx context.Context, podName string, app *model.Application) error {
	n.logger.Info("[noop] delete pod", slog.String("pod", podName))
	return nil
}

func (n *NoopOrchestrator) GetPodEvents(ctx context.Context, app *model.Application, podName string) ([]PodEvent, error) {
	n.logger.Info("[noop] get pod events", slog.String("pod", podName))
	return []PodEvent{
		{
			Type:      "Normal",
			Reason:    "Scheduled",
			Message:   fmt.Sprintf("Successfully assigned default/%s to sailbox-server", podName),
			Count:     1,
			FirstSeen: time.Now().Add(-1 * time.Hour),
			LastSeen:  time.Now().Add(-1 * time.Hour),
		},
		{
			Type:      "Normal",
			Reason:    "Started",
			Message:   fmt.Sprintf("Started container %s", app.Name),
			Count:     1,
			FirstSeen: time.Now().Add(-59 * time.Minute),
			LastSeen:  time.Now().Add(-59 * time.Minute),
		},
	}, nil
}

func (n *NoopOrchestrator) ConfigureHPA(ctx context.Context, app *model.Application, cfg model.AutoscalingConfig) error {
	n.logger.Info("[noop] configure HPA", slog.String("app", app.Name), slog.Int("min", int(cfg.MinReplicas)), slog.Int("max", int(cfg.MaxReplicas)))
	return nil
}

func (n *NoopOrchestrator) DeleteHPA(ctx context.Context, app *model.Application) error {
	n.logger.Info("[noop] delete HPA", slog.String("app", app.Name))
	return nil
}

func (n *NoopOrchestrator) DeployDatabase(ctx context.Context, db *model.ManagedDatabase) error {
	n.logger.Info("[noop] deploy database", slog.String("name", db.Name), slog.String("engine", string(db.Engine)))
	return nil
}

func (n *NoopOrchestrator) DeleteDatabase(ctx context.Context, db *model.ManagedDatabase) error {
	n.logger.Info("[noop] delete database", slog.String("name", db.Name))
	return nil
}

func (n *NoopOrchestrator) GetDatabaseStatus(ctx context.Context, db *model.ManagedDatabase) (*AppStatus, error) {
	return &AppStatus{Phase: "running", ReadyReplicas: 1, DesiredReplicas: 1}, nil
}

func (n *NoopOrchestrator) GetDatabaseCredentials(ctx context.Context, db *model.ManagedDatabase) (*DatabaseCredentials, error) {
	n.logger.Info("[noop] get database credentials", slog.String("name", db.Name))
	host := fmt.Sprintf("%s.default.svc.cluster.local", db.Name)
	return &DatabaseCredentials{
		Host:             host,
		Port:             5432,
		Username:         "sailbox",
		Password:         "noop-password-abc123",
		DatabaseName:     "app",
		ConnectionString: fmt.Sprintf("postgresql://sailbox:noop-password-abc123@%s:5432/app", host),
		InternalURL:      fmt.Sprintf("%s:5432", host),
	}, nil
}

func (n *NoopOrchestrator) GetDatabasePods(ctx context.Context, db *model.ManagedDatabase) ([]PodInfo, error) {
	n.logger.Info("[noop] get database pods", slog.String("name", db.Name))
	return []PodInfo{
		{
			Name:         fmt.Sprintf("%s-0", db.Name),
			Phase:        "Running",
			Node:         "sailbox-server",
			IP:           "10.42.0.20",
			StartedAt:    time.Now().Add(-6 * time.Hour),
			RestartCount: 0,
			Ready:        true,
			Containers: []ContainerStatus{
				{Name: db.Name, Ready: true, RestartCount: 0, State: "running"},
			},
			Resources: ResourceMetrics{CPUUsed: "45m", CPUTotal: db.CPULimit, MemUsed: "200Mi", MemTotal: db.MemLimit},
		},
	}, nil
}

func (n *NoopOrchestrator) RunDatabaseBackup(ctx context.Context, db *model.ManagedDatabase, backupID uuid.UUID, s3Config *S3Config, s3Key string) error {
	n.logger.Info("[noop] run database backup", slog.String("name", db.Name), slog.String("backup_id", backupID.String()))
	return nil
}

func (n *NoopOrchestrator) GetBackupJobStatus(ctx context.Context, backupID uuid.UUID) string {
	return "completed"
}

func (n *NoopOrchestrator) RestoreDatabaseBackup(ctx context.Context, db *model.ManagedDatabase, backupID uuid.UUID, s3Config *S3Config, s3Key string) error {
	n.logger.Info("[noop] restore database backup", slog.String("name", db.Name), slog.String("backup_id", backupID.String()))
	return nil
}

func (n *NoopOrchestrator) GetRestoreJobStatus(ctx context.Context, backupID uuid.UUID) string {
	return "completed"
}

func (n *NoopOrchestrator) EnableExternalAccess(ctx context.Context, db *model.ManagedDatabase) (int32, error) {
	n.logger.Info("[noop] enable external access", slog.String("name", db.Name))
	return 30000, nil
}

func (n *NoopOrchestrator) DisableExternalAccess(ctx context.Context, db *model.ManagedDatabase) error {
	n.logger.Info("[noop] disable external access", slog.String("name", db.Name))
	return nil
}

func (n *NoopOrchestrator) CreateIngress(ctx context.Context, domain *model.Domain, app *model.Application) error {
	n.logger.Info("[noop] create ingress", slog.String("host", domain.Host))
	return nil
}

func (n *NoopOrchestrator) UpdateIngress(ctx context.Context, domain *model.Domain, app *model.Application) error {
	return nil
}

func (n *NoopOrchestrator) DeleteIngress(ctx context.Context, domain *model.Domain) error {
	return nil
}

func (n *NoopOrchestrator) DeleteIngressByName(ctx context.Context, app *model.Application, name string) error {
	return nil
}

func (n *NoopOrchestrator) IngressName(app *model.Application, host string) string {
	return "noop"
}

func (n *NoopOrchestrator) LegacyIngressName(app *model.Application, host string) string {
	return "noop"
}

func (n *NoopOrchestrator) GetIngressStatus(ctx context.Context, domain *model.Domain, app *model.Application) (*IngressStatus, error) {
	return &IngressStatus{Ready: true}, nil
}

func (n *NoopOrchestrator) GetCertExpiry(ctx context.Context, domain *model.Domain, app *model.Application) (*time.Time, error) {
	expiry := time.Now().Add(60 * 24 * time.Hour)
	return &expiry, nil
}

func (n *NoopOrchestrator) CreateVolume(ctx context.Context, opts VolumeOpts) (string, error) {
	return "noop-pvc", nil
}

func (n *NoopOrchestrator) DeleteVolume(ctx context.Context, name, namespace string) error {
	return nil
}

func (n *NoopOrchestrator) GetNodes(ctx context.Context) ([]NodeInfo, error) {
	return []NodeInfo{
		{
			Name:    "sailbox-server",
			IP:      "127.0.0.1",
			Status:  "Ready",
			Roles:   []string{"control-plane", "master"},
			Version: "v1.31.4+k3s1",
			OS:      "linux",
			Arch:    "amd64",
			Resources: ResourceMetrics{
				CPUUsed: "250m", CPUTotal: "4000m",
				MemUsed: "1.2Gi", MemTotal: "8Gi",
				StorageUsed: "12Gi", StorageTotal: "50Gi",
			},
		},
	}, nil
}

func (n *NoopOrchestrator) GetClusterMetrics(ctx context.Context) (*ClusterMetrics, error) {
	return &ClusterMetrics{
		Nodes:       1,
		TotalPods:   5,
		RunningPods: 5,
		Resources: ResourceMetrics{
			CPUUsed: "250m", CPUTotal: "4000m",
			MemUsed: "1.2Gi", MemTotal: "8Gi",
		},
	}, nil
}

func (n *NoopOrchestrator) GetNamespaceMetrics(ctx context.Context, namespace string) (*ResourceMetrics, error) {
	return &ResourceMetrics{}, nil
}

func (n *NoopOrchestrator) GetAllPods(ctx context.Context) ([]PodInfo, error) {
	return []PodInfo{
		{
			Name: "nginx-abc12", Phase: "Running", Node: "sailbox-server",
			IP: "10.42.0.10", StartedAt: time.Now().Add(-2 * time.Hour), RestartCount: 0, Ready: true,
			Containers: []ContainerStatus{{Name: "nginx", Ready: true, State: "running"}},
			Resources:  ResourceMetrics{CPUUsed: "30m", CPUTotal: "250m", MemUsed: "48Mi", MemTotal: "256Mi"},
		},
		{
			Name: "api-server-def34", Phase: "Running", Node: "sailbox-server",
			IP: "10.42.0.11", StartedAt: time.Now().Add(-5 * time.Hour), RestartCount: 1, Ready: true,
			Containers: []ContainerStatus{{Name: "api", Ready: true, State: "running"}},
			Resources:  ResourceMetrics{CPUUsed: "120m", CPUTotal: "500m", MemUsed: "128Mi", MemTotal: "512Mi"},
		},
		{
			Name: "worker-ghi56", Phase: "Running", Node: "sailbox-server",
			IP: "10.42.0.12", StartedAt: time.Now().Add(-1 * time.Hour), RestartCount: 0, Ready: true,
			Containers: []ContainerStatus{{Name: "worker", Ready: true, State: "running"}},
			Resources:  ResourceMetrics{CPUUsed: "80m", CPUTotal: "500m", MemUsed: "96Mi", MemTotal: "256Mi"},
		},
		{
			Name: "coredns-jkl78", Phase: "Running", Node: "sailbox-server",
			IP: "10.42.0.2", StartedAt: time.Now().Add(-24 * time.Hour), RestartCount: 0, Ready: true,
			Containers: []ContainerStatus{{Name: "coredns", Ready: true, State: "running"}},
			Resources:  ResourceMetrics{CPUUsed: "5m", CPUTotal: "100m", MemUsed: "20Mi", MemTotal: "128Mi"},
		},
		{
			Name: "postgres-mno90", Phase: "Running", Node: "sailbox-server",
			IP: "10.42.0.13", StartedAt: time.Now().Add(-12 * time.Hour), RestartCount: 0, Ready: true,
			Containers: []ContainerStatus{{Name: "postgres", Ready: true, State: "running"}},
			Resources:  ResourceMetrics{CPUUsed: "45m", CPUTotal: "500m", MemUsed: "200Mi", MemTotal: "1Gi"},
		},
	}, nil
}

func (n *NoopOrchestrator) GetClusterEvents(ctx context.Context, limit int) ([]ClusterEvent, error) {
	return []ClusterEvent{
		{
			Type: "Normal", Reason: "Scheduled", Message: "Successfully assigned default/nginx-abc12 to sailbox-server",
			Namespace: "default", InvolvedObject: "Pod/nginx-abc12", Count: 1,
			FirstSeen: time.Now().Add(-2 * time.Hour), LastSeen: time.Now().Add(-2 * time.Hour),
		},
		{
			Type: "Normal", Reason: "Pulled", Message: "Container image \"nginx:latest\" already present on machine",
			Namespace: "default", InvolvedObject: "Pod/nginx-abc12", Count: 1,
			FirstSeen: time.Now().Add(-2 * time.Hour), LastSeen: time.Now().Add(-2 * time.Hour),
		},
		{
			Type: "Warning", Reason: "BackOff", Message: "Back-off restarting failed container",
			Namespace: "sailbox-apps", InvolvedObject: "Pod/api-server-def34", Count: 3,
			FirstSeen: time.Now().Add(-6 * time.Hour), LastSeen: time.Now().Add(-5 * time.Hour),
		},
		{
			Type: "Warning", Reason: "CrashLoopBackOff", Message: "Back-off 5m0s restarting failed container=worker pod=worker-old",
			Namespace: "sailbox-apps", InvolvedObject: "Pod/worker-old", Count: 12,
			FirstSeen: time.Now().Add(-1 * time.Hour), LastSeen: time.Now().Add(-10 * time.Minute),
		},
	}, nil
}

func (n *NoopOrchestrator) GetPVCs(ctx context.Context) ([]PVCInfo, error) {
	return []PVCInfo{
		{
			Name: "data-postgres-0", Namespace: "sailbox-apps", Status: "Bound",
			Capacity: "10Gi", StorageClass: "local-path", VolumeName: "pvc-abc123",
		},
		{
			Name: "uploads-api", Namespace: "sailbox-apps", Status: "Pending",
			Capacity: "5Gi", StorageClass: "local-path", VolumeName: "",
		},
	}, nil
}

func (n *NoopOrchestrator) GetNamespaces(ctx context.Context) ([]NamespaceInfo, error) {
	return []NamespaceInfo{
		{Name: "default", Status: "Active", PodCount: 2, SvcCount: 1},
		{Name: "kube-system", Status: "Active", PodCount: 5, SvcCount: 3},
		{Name: "sailbox-apps", Status: "Active", PodCount: 3, SvcCount: 2},
	}, nil
}

func (n *NoopOrchestrator) GetNodeMetrics(ctx context.Context) ([]NodeMetrics, error) {
	return []NodeMetrics{
		{
			Name:     "sailbox-server",
			CPUUsed:  "280m",
			CPUTotal: "4000m",
			MemUsed:  "1536Mi",
			MemTotal: "8192Mi",
			PodCount: 5,
		},
	}, nil
}

func (n *NoopOrchestrator) StreamLogs(ctx context.Context, app *model.Application, opts LogOpts) (io.ReadCloser, error) {
	r, w := io.Pipe()
	go func() {
		defer w.Close()
		lines := []string{
			fmt.Sprintf("[sailbox] Pod %s-0 started", app.Name),
			fmt.Sprintf("[sailbox] Pulling image: %s", app.DockerImage),
			"[sailbox] Image pull complete",
			fmt.Sprintf("[sailbox] Container %s created", app.Name),
			fmt.Sprintf("[sailbox] Container %s started", app.Name),
			"[nginx] /docker-entrypoint.sh: Configuration complete; ready for start up",
			"[nginx] 2026/03/24 15:00:00 [notice] 1#1: nginx/1.27.0",
			"[nginx] 2026/03/24 15:00:00 [notice] 1#1: built by gcc 12.2.0",
			"[nginx] 2026/03/24 15:00:00 [notice] 1#1: OS: Linux 6.1.0",
			"[nginx] 2026/03/24 15:00:00 [notice] 1#1: start worker processes",
		}
		for _, line := range lines {
			fmt.Fprintln(w, line)
			time.Sleep(300 * time.Millisecond)
		}
		// Keep producing periodic log lines
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		i := 0
		for range ticker.C {
			i++
			fmt.Fprintf(w, "[nginx] 10.42.0.1 - - [24/Mar/2026:15:%02d:%02d +0000] \"GET / HTTP/1.1\" 200 615\n", i/60, i%60)
		}
	}()
	return r, nil
}

func (n *NoopOrchestrator) StreamPodLogs(ctx context.Context, app *model.Application, podName string, opts LogOpts) (io.ReadCloser, error) {
	return n.StreamLogs(ctx, app, opts)
}

func (n *NoopOrchestrator) Build(ctx context.Context, app *model.Application, opts BuildOpts) (*BuildResult, error) {
	n.logger.Info("[noop] build", slog.String("app", app.Name), slog.String("repo", opts.GitRepo))
	logs := fmt.Sprintf("[noop] Cloning %s@%s...\n[noop] Building with %s...\n[noop] Build complete.\n", opts.GitRepo, opts.GitBranch, opts.BuildType)
	if opts.OnLog != nil {
		opts.OnLog(logs)
	}
	time.Sleep(2 * time.Second) // simulate build time
	return &BuildResult{
		Image:    fmt.Sprintf("registry.sailbox-system:5000/%s:noop-build", app.Name),
		Duration: 2 * time.Second,
		Logs:     logs,
	}, nil
}

func (n *NoopOrchestrator) EnsureRegistry(ctx context.Context) error {
	n.logger.Info("[noop] ensure registry")
	return nil
}

func (n *NoopOrchestrator) CancelBuild(ctx context.Context, app *model.Application) error {
	n.logger.Info("[noop] cancel build", slog.String("app", app.Name))
	return nil
}

func (n *NoopOrchestrator) ClearBuildCache(ctx context.Context, app *model.Application) error {
	n.logger.Info("[noop] clear build cache", slog.String("app", app.Name))
	return nil
}

func (n *NoopOrchestrator) GetBuildLogs(ctx context.Context, jobName, namespace string) (io.ReadCloser, error) {
	r, w := io.Pipe()
	go func() {
		defer w.Close()
		fmt.Fprintln(w, "[noop] Build logs not available in noop mode")
	}()
	return r, nil
}

func (n *NoopOrchestrator) ExecTerminal(ctx context.Context, app *model.Application, opts ExecOpts) (TerminalSession, error) {
	return newNoopTerminal(app.Name), nil
}

func (n *NoopOrchestrator) GetClusterTopology(ctx context.Context) (*ClusterTopology, error) {
	return &ClusterTopology{
		Nodes: []TopologyNode{{Name: "noop-node", Status: "Ready", IP: "127.0.0.1", Roles: "control-plane"}},
	}, nil
}

func (n *NoopOrchestrator) SetNodeLabel(ctx context.Context, nodeName, key, value string) error {
	n.logger.Info("[noop] set node label", slog.String("node", nodeName), slog.String("key", key), slog.String("value", value))
	return nil
}

func (n *NoopOrchestrator) RemoveNodeLabel(ctx context.Context, nodeName, key string) error {
	n.logger.Info("[noop] remove node label", slog.String("node", nodeName), slog.String("key", key))
	return nil
}

func (n *NoopOrchestrator) GetNodePools(ctx context.Context) ([]string, error) {
	return []string{"default", "build"}, nil
}

func (n *NoopOrchestrator) CreateNamespace(ctx context.Context, name string) error {
	n.logger.Info("[noop] create namespace", slog.String("namespace", name))
	return nil
}

func (n *NoopOrchestrator) DeleteNamespace(ctx context.Context, name string) error {
	n.logger.Info("[noop] delete namespace", slog.String("namespace", name))
	return nil
}

func (n *NoopOrchestrator) EnsureSecret(ctx context.Context, app *model.Application, secrets map[string]string) error {
	n.logger.Info("[noop] ensure secret", slog.String("app", app.Name), slog.Int("count", len(secrets)))
	return nil
}

// ── CronJobManager ──────────────────────────────────────────────

func (n *NoopOrchestrator) CreateCronJob(ctx context.Context, cj *model.CronJob) error {
	n.logger.Info("[noop] create cronjob", slog.String("name", cj.Name), slog.String("schedule", cj.CronExpression))
	return nil
}

func (n *NoopOrchestrator) UpdateCronJob(ctx context.Context, cj *model.CronJob) error {
	n.logger.Info("[noop] update cronjob", slog.String("name", cj.Name))
	return nil
}

func (n *NoopOrchestrator) DeleteCronJob(ctx context.Context, cj *model.CronJob) error {
	n.logger.Info("[noop] delete cronjob", slog.String("name", cj.Name))
	return nil
}

func (n *NoopOrchestrator) SuspendCronJob(ctx context.Context, cj *model.CronJob, suspend bool) error {
	n.logger.Info("[noop] suspend cronjob", slog.String("name", cj.Name), slog.Bool("suspend", suspend))
	return nil
}

func (n *NoopOrchestrator) TriggerCronJob(ctx context.Context, cj *model.CronJob) (string, error) {
	n.logger.Info("[noop] trigger cronjob", slog.String("name", cj.Name))
	return fmt.Sprintf("%s-manual-%d", cj.Name, time.Now().Unix()), nil
}

func (n *NoopOrchestrator) GetJobStatus(ctx context.Context, cj *model.CronJob, jobName string) (string, error) {
	n.logger.Info("[noop] get job status", slog.String("job", jobName))
	return "succeeded", nil
}

// ── ConfigMapManager ────────────────────────────────────────────

func (n *NoopOrchestrator) EnsureConfigMap(ctx context.Context, namespace, name string, data map[string]string) error {
	n.logger.Info("[noop] ensure configmap", slog.String("namespace", namespace), slog.String("name", name))
	return nil
}

func (n *NoopOrchestrator) DeleteConfigMap(ctx context.Context, namespace, name string) error {
	n.logger.Info("[noop] delete configmap", slog.String("namespace", namespace), slog.String("name", name))
	return nil
}

// ── ResourceQuotaManager ────────────────────────────────────────

func (n *NoopOrchestrator) EnsureResourceQuota(ctx context.Context, namespace string, quota model.ResourceQuotaConfig) error {
	n.logger.Info("[noop] ensure resource quota", slog.String("namespace", namespace))
	return nil
}

func (n *NoopOrchestrator) DeleteResourceQuota(ctx context.Context, namespace string) error {
	n.logger.Info("[noop] delete resource quota", slog.String("namespace", namespace))
	return nil
}

// ── NetworkPolicyManager ────────────────────────────────────────

func (n *NoopOrchestrator) EnsureNetworkPolicy(ctx context.Context, namespace string, enabled bool) error {
	n.logger.Info("[noop] ensure network policy", slog.String("namespace", namespace), slog.Bool("enabled", enabled))
	return nil
}

// ── StorageManager (expand) ─────────────────────────────────────

func (n *NoopOrchestrator) ExpandVolume(ctx context.Context, name, namespace, newSize string) error {
	n.logger.Info("[noop] expand volume", slog.String("name", name), slog.String("size", newSize))
	return nil
}

// ── Panel Ingress ───────────────────────────────────────────────

func (n *NoopOrchestrator) EnsurePanelIngress(ctx context.Context, domain, httpsEmail string) error {
	n.logger.Info("[noop] ensure panel ingress", slog.String("domain", domain), slog.String("email", httpsEmail))
	return nil
}

func (n *NoopOrchestrator) DeletePanelIngress(ctx context.Context) error {
	n.logger.Info("[noop] delete panel ingress")
	return nil
}

// ── TraefikManager ──────────────────────────────────────────────

func (n *NoopOrchestrator) GetTraefikConfig(ctx context.Context) (string, error) {
	n.logger.Info("[noop] get traefik config")
	return "# noop traefik config\nentryPoints:\n  web:\n    address: \":80\"\n  websecure:\n    address: \":443\"\n", nil
}

func (n *NoopOrchestrator) UpdateTraefikConfig(ctx context.Context, yaml string) error {
	n.logger.Info("[noop] update traefik config")
	return nil
}

func (n *NoopOrchestrator) RestartTraefik(ctx context.Context) error {
	n.logger.Info("[noop] restart traefik")
	return nil
}

func (n *NoopOrchestrator) GetTraefikStatus(ctx context.Context) (*TraefikStatus, error) {
	return &TraefikStatus{Ready: true, PodName: "traefik-noop", Age: "0s"}, nil
}

// ── HelmInspector ───────────────────────────────────────────────

func (n *NoopOrchestrator) GetHelmReleases(ctx context.Context) ([]HelmRelease, error) {
	n.logger.Info("[noop] get helm releases")
	return []HelmRelease{
		{Name: "traefik", Namespace: "kube-system", Chart: "traefik", Revision: "1", Status: "deployed", Updated: "2026-03-25 12:00:00"},
	}, nil
}

// ── DaemonSetInspector ──────────────────────────────────────────

func (n *NoopOrchestrator) GetDaemonSets(ctx context.Context) ([]DaemonSetInfo, error) {
	n.logger.Info("[noop] get daemonsets")
	return []DaemonSetInfo{
		{Name: "svclb-traefik", Namespace: "kube-system", DesiredScheduled: 1, CurrentScheduled: 1, Ready: 1, Images: "rancher/klipper-lb:v0.4.9", CreatedAt: "2026-03-25T12:00:00Z"},
	}, nil
}

// ── ServiceAccountManager ───────────────────────────────────────

func (n *NoopOrchestrator) EnsureServiceAccount(ctx context.Context, namespace, name string) error {
	n.logger.Info("[noop] ensure service account", slog.String("namespace", namespace), slog.String("name", name))
	return nil
}

func (n *NoopOrchestrator) DeleteServiceAccount(ctx context.Context, namespace, name string) error {
	n.logger.Info("[noop] delete service account", slog.String("namespace", namespace), slog.String("name", name))
	return nil
}

// ── CleanupInspector ─────────────────────────────────────────────

func (n *NoopOrchestrator) GetCleanupStats(ctx context.Context) (*CleanupStats, error) {
	n.logger.Info("[noop] get cleanup stats")
	return &CleanupStats{
		EvictedPods:       2,
		EvictedPodNames:   []string{"default/old-pod-abc12", "sailbox-apps/crashed-xyz99"},
		FailedPods:        1,
		FailedPodNames:    []string{"sailbox-apps/bad-config-def34"},
		CompletedPods:     3,
		CompletedPodNames: []string{"sailbox-apps/migration-001", "sailbox-apps/migration-002", "default/setup-job-pod"},
		StaleReplicaSets:  2,
		StaleRSNames:      []string{"sailbox-apps/api-server-old-rs1", "sailbox-apps/api-server-old-rs2"},
		CompletedJobs:     1,
		CompletedJobNames: []string{"sailbox-apps/backup-daily-001"},
		UnboundPVCs:       1,
		UnboundPVCNames:   []string{"sailbox-apps/orphaned-pvc-data"},
	}, nil
}

func (n *NoopOrchestrator) CleanupEvictedPods(ctx context.Context) (*CleanupResult, error) {
	n.logger.Info("[noop] cleanup evicted pods")
	return &CleanupResult{Deleted: 2, Message: "Deleted 2 evicted pods"}, nil
}

func (n *NoopOrchestrator) CleanupFailedPods(ctx context.Context) (*CleanupResult, error) {
	n.logger.Info("[noop] cleanup failed pods")
	return &CleanupResult{Deleted: 1, Message: "Deleted 1 failed pods"}, nil
}

func (n *NoopOrchestrator) CleanupCompletedPods(ctx context.Context) (*CleanupResult, error) {
	n.logger.Info("[noop] cleanup completed pods")
	return &CleanupResult{Deleted: 3, Message: "Deleted 3 completed pods"}, nil
}

func (n *NoopOrchestrator) CleanupStaleReplicaSets(ctx context.Context) (*CleanupResult, error) {
	n.logger.Info("[noop] cleanup stale replicasets")
	return &CleanupResult{Deleted: 2, Message: "Deleted 2 stale replicasets"}, nil
}

func (n *NoopOrchestrator) CleanupCompletedJobs(ctx context.Context) (*CleanupResult, error) {
	n.logger.Info("[noop] cleanup completed jobs")
	return &CleanupResult{Deleted: 1, Message: "Deleted 1 completed jobs"}, nil
}

func (n *NoopOrchestrator) GetOrphanIngresses(ctx context.Context, validHosts map[string]bool) ([]string, error) {
	n.logger.Info("[noop] get orphan ingresses")
	return []string{"default/orphan-ingress-1"}, nil
}

func (n *NoopOrchestrator) CleanupOrphanIngresses(ctx context.Context, validHosts map[string]bool) (*CleanupResult, error) {
	n.logger.Info("[noop] cleanup orphan ingresses")
	return &CleanupResult{Deleted: 1, Message: "Deleted 1 orphan ingresses"}, nil
}

type noopTerminal struct {
	reader *io.PipeReader
	writer *io.PipeWriter
	name   string
}

func newNoopTerminal(name string) *noopTerminal {
	r, w := io.Pipe()
	t := &noopTerminal{reader: r, writer: w, name: name}
	// Write welcome message
	go func() {
		w.Write([]byte(fmt.Sprintf("sailbox-noop@%s:~$ ", name)))
	}()
	return t
}

func (t *noopTerminal) Read(p []byte) (int, error) {
	return t.reader.Read(p)
}

func (t *noopTerminal) Write(p []byte) (int, error) {
	// Echo input back and add prompt
	go func() {
		t.writer.Write(p)
		if len(p) > 0 && p[len(p)-1] == '\r' {
			t.writer.Write([]byte(fmt.Sprintf("\nsailbox-noop@%s:~$ ", t.name)))
		}
	}()
	return len(p), nil
}

func (t *noopTerminal) Resize(width, height uint16) error { return nil }
func (t *noopTerminal) Close() error {
	t.writer.Close()
	t.reader.Close()
	return nil
}
