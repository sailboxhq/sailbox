package k3s

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sailboxhq/sailbox/apps/api/internal/model"
	"github.com/sailboxhq/sailbox/apps/api/internal/orchestrator"
)

// Database engine → Docker image mapping
var engineImages = map[model.DBEngine]string{
	model.DBPostgres: "postgres",
	model.DBMySQL:    "mysql",
	model.DBMariaDB:  "mariadb",
	model.DBRedis:    "redis",
	model.DBMongo:    "mongo",
}

// Database engine → default port
var enginePorts = map[model.DBEngine]int32{
	model.DBPostgres: 5432,
	model.DBMySQL:    3306,
	model.DBMariaDB:  3306,
	model.DBRedis:    6379,
	model.DBMongo:    27017,
}

func dbNamespace(db *model.ManagedDatabase) string {
	if db.Namespace != "" {
		return db.Namespace
	}
	return "default"
}

func dbK8sName(db *model.ManagedDatabase) string {
	if db.K8sName != "" {
		return db.K8sName
	}
	return sanitize(db.Name)
}

// dbProbes returns engine-specific readiness and liveness probes.
func dbProbes(engine model.DBEngine) (*corev1.Probe, *corev1.Probe) {
	var cmd []string
	switch engine {
	case model.DBPostgres:
		cmd = []string{"pg_isready", "-U", "sailbox"}
	case model.DBMySQL:
		cmd = []string{"sh", "-c", "mysqladmin ping -u root -p$MYSQL_ROOT_PASSWORD"}
	case model.DBMariaDB:
		cmd = []string{"sh", "-c", "mariadb-admin ping -u root -p$MARIADB_ROOT_PASSWORD || mysqladmin ping -u root -p$MARIADB_ROOT_PASSWORD"}
	case model.DBRedis:
		cmd = []string{"redis-cli", "ping"}
	case model.DBMongo:
		cmd = []string{"mongosh", "--eval", "db.adminCommand('ping')"}
	}
	readiness := &corev1.Probe{
		ProbeHandler:        corev1.ProbeHandler{Exec: &corev1.ExecAction{Command: cmd}},
		InitialDelaySeconds: 5,
		PeriodSeconds:       10,
		TimeoutSeconds:      5,
	}
	liveness := &corev1.Probe{
		ProbeHandler:        corev1.ProbeHandler{Exec: &corev1.ExecAction{Command: cmd}},
		InitialDelaySeconds: 15,
		PeriodSeconds:       20,
		TimeoutSeconds:      5,
	}
	return readiness, liveness
}

// dbConnectionString builds an engine-specific connection string.
func dbConnectionString(engine model.DBEngine, host string, port int32, password, dbName string) string {
	switch engine {
	case model.DBPostgres:
		return fmt.Sprintf("postgresql://sailbox:%s@%s:%d/%s", password, host, port, dbName)
	case model.DBMySQL, model.DBMariaDB:
		return fmt.Sprintf("mysql://root:%s@%s:%d/%s", password, host, port, dbName)
	case model.DBRedis:
		return fmt.Sprintf("redis://:%s@%s:%d", password, host, port)
	case model.DBMongo:
		return fmt.Sprintf("mongodb://sailbox:%s@%s:%d/%s", password, host, port, dbName)
	default:
		return ""
	}
}

// dbUsername returns the default username for the engine.
func dbUsername(engine model.DBEngine) string {
	switch engine {
	case model.DBPostgres, model.DBMongo:
		return "sailbox"
	case model.DBMySQL, model.DBMariaDB:
		return "root"
	default:
		return ""
	}
}

// dbName returns the default database name for the engine.
// dbDatabaseName is no longer used — database name comes from db.Name

func (o *Orchestrator) DeployDatabase(ctx context.Context, db *model.ManagedDatabase) error {
	ns := dbNamespace(db)
	k8sName := dbK8sName(db)
	image := fmt.Sprintf("%s:%s", engineImages[db.Engine], db.Version)
	port := enginePorts[db.Engine]

	if err := o.ensureNamespace(ctx, ns); err != nil {
		return err
	}

	labels := map[string]string{
		"app.kubernetes.io/name":       k8sName,
		"app.kubernetes.io/managed-by": "sailbox",
		"sailbox/db-id":                db.ID.String(),
		"sailbox/engine":               string(db.Engine),
	}

	// Generate random password
	passBytes := make([]byte, 16)
	_, _ = rand.Read(passBytes)
	password := hex.EncodeToString(passBytes)

	// Build service host for connection string
	host := fmt.Sprintf("%s.%s.svc.cluster.local", k8sName, ns)
	connStr := dbConnectionString(db.Engine, host, port, password, db.DatabaseName)

	// Create Secret with credentials + Sailbox connection info
	secretName := fmt.Sprintf("%s-credentials", k8sName)
	secretData := dbEnvSecret(db.Engine, password, db.DatabaseName)
	secretData["SAILBOX_HOST"] = host
	secretData["SAILBOX_PORT"] = strconv.Itoa(int(port))
	secretData["SAILBOX_USERNAME"] = dbUsername(db.Engine)
	secretData["SAILBOX_PASSWORD"] = password
	secretData["SAILBOX_DB"] = db.DatabaseName
	secretData["SAILBOX_CONNECTION_STRING"] = connStr

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: ns, Labels: labels},
		StringData: secretData,
	}
	existing, err := o.client.CoreV1().Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			if _, createErr := o.client.CoreV1().Secrets(ns).Create(ctx, secret, metav1.CreateOptions{}); createErr != nil {
				return createErr
			}
		}
	} else {
		// Reuse existing password to avoid credential mismatch on retry
		if p, ok := existing.Data["SAILBOX_PASSWORD"]; ok {
			password = string(p)
			_ = dbConnectionString(db.Engine, host, port, password, db.DatabaseName) // connection string recomputed if needed
		}
	}

	// Create PVC
	pvcName := fmt.Sprintf("%s-data", k8sName)
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: pvcName, Namespace: ns, Labels: labels},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources:   corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(db.StorageSize)}},
		},
	}
	_, err = o.client.CoreV1().PersistentVolumeClaims(ns).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("create PVC: %w", err)
	}

	// Health probes
	readiness, liveness := dbProbes(db.Engine)

	// Create StatefulSet
	var replicas int32 = 1
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: k8sName, Namespace: ns, Labels: labels},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  k8sName,
							Image: image,
							Ports: []corev1.ContainerPort{{ContainerPort: port}},
							EnvFrom: []corev1.EnvFromSource{
								{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: secretName}}},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "data", MountPath: dbDataPath(db.Engine)},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(db.CPULimit),
									corev1.ResourceMemory: resource.MustParse(db.MemLimit),
								},
							},
							ReadinessProbe: readiness,
							LivenessProbe:  liveness,
						},
					},
					Volumes: []corev1.Volume{
						{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName}}},
					},
				},
			},
		},
	}

	_, err = o.client.AppsV1().StatefulSets(ns).Create(ctx, sts, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("create StatefulSet: %w", err)
	}

	// Create Service
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: k8sName, Namespace: ns, Labels: labels},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports:    []corev1.ServicePort{{Port: port, TargetPort: *intOrString(int(port))}},
		},
	}
	_, err = o.client.CoreV1().Services(ns).Create(ctx, svc, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("create db service: %w", err)
	}

	// Set K8s metadata on the db pointer so the caller can persist it
	db.Namespace = ns
	db.K8sName = k8sName
	db.CredentialsSecret = secretName
	db.Status = "pending" // actual status will be reconciled via GetDatabaseStatus

	o.logger.Info("database deployed", slog.String("name", k8sName), slog.String("engine", string(db.Engine)))
	return nil
}

func (o *Orchestrator) DeleteDatabase(ctx context.Context, db *model.ManagedDatabase) error {
	ns := dbNamespace(db)
	name := dbK8sName(db)
	propagation := metav1.DeletePropagationForeground

	err := o.client.AppsV1().StatefulSets(ns).Delete(ctx, name, metav1.DeleteOptions{PropagationPolicy: &propagation})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("delete StatefulSet %s: %w", name, err)
	}
	_ = o.client.CoreV1().Services(ns).Delete(ctx, name, metav1.DeleteOptions{})
	_ = o.client.CoreV1().Services(ns).Delete(ctx, fmt.Sprintf("%s-external", name), metav1.DeleteOptions{})
	_ = o.client.CoreV1().Secrets(ns).Delete(ctx, fmt.Sprintf("%s-credentials", name), metav1.DeleteOptions{})
	_ = o.client.CoreV1().PersistentVolumeClaims(ns).Delete(ctx, fmt.Sprintf("%s-data", name), metav1.DeleteOptions{})

	o.logger.Info("database deleted", slog.String("name", name))
	return nil
}

func (o *Orchestrator) GetDatabaseStatus(ctx context.Context, db *model.ManagedDatabase) (*orchestrator.AppStatus, error) {
	ns := dbNamespace(db)
	name := dbK8sName(db)

	sts, err := o.client.AppsV1().StatefulSets(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return &orchestrator.AppStatus{Phase: "not deployed"}, nil
		}
		return nil, err
	}

	phase := "pending"
	if sts.Status.ReadyReplicas == *sts.Spec.Replicas {
		phase = "running"
	}

	return &orchestrator.AppStatus{
		Phase:           phase,
		ReadyReplicas:   sts.Status.ReadyReplicas,
		DesiredReplicas: *sts.Spec.Replicas,
	}, nil
}

func (o *Orchestrator) GetDatabaseCredentials(ctx context.Context, db *model.ManagedDatabase) (*orchestrator.DatabaseCredentials, error) {
	ns := dbNamespace(db)
	k8sName := dbK8sName(db)
	secretName := fmt.Sprintf("%s-credentials", k8sName)

	secret, err := o.client.CoreV1().Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get credentials secret: %w", err)
	}

	getString := func(key string) string {
		if v, ok := secret.Data[key]; ok {
			return string(v)
		}
		return ""
	}

	port, _ := strconv.Atoi(getString("SAILBOX_PORT"))

	host := getString("SAILBOX_HOST")
	return &orchestrator.DatabaseCredentials{
		Host:             host,
		Port:             int32(port),
		Username:         getString("SAILBOX_USERNAME"),
		Password:         getString("SAILBOX_PASSWORD"),
		DatabaseName:     getString("SAILBOX_DB"),
		ConnectionString: getString("SAILBOX_CONNECTION_STRING"),
		InternalURL:      fmt.Sprintf("%s:%d", host, port),
	}, nil
}

func (o *Orchestrator) GetDatabasePods(ctx context.Context, db *model.ManagedDatabase) ([]orchestrator.PodInfo, error) {
	ns := dbNamespace(db)
	k8sName := dbK8sName(db)

	pods, err := o.client.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app.kubernetes.io/name=%s", k8sName),
	})
	if err != nil {
		return nil, fmt.Errorf("list database pods: %w", err)
	}

	// Fetch real-time metrics from metrics-server
	podMetrics := o.fetchPodMetrics(ctx, ns, k8sName)

	var result []orchestrator.PodInfo
	for _, p := range pods.Items {
		// Get resource usage from metrics
		var resources orchestrator.ResourceMetrics
		if m, ok := podMetrics[p.Name]; ok {
			resources.CPUUsed = m.cpuUsed
			resources.MemUsed = m.memUsed
		}
		// Get resource limits from container spec
		if len(p.Spec.Containers) > 0 {
			if lim := p.Spec.Containers[0].Resources.Limits; lim != nil {
				if cpu, ok := lim[corev1.ResourceCPU]; ok {
					resources.CPUTotal = cpu.String()
				}
				if mem, ok := lim[corev1.ResourceMemory]; ok {
					resources.MemTotal = mem.String()
				}
			}
		}

		pi := orchestrator.PodInfo{
			Name:      p.Name,
			Phase:     string(p.Status.Phase),
			Node:      p.Spec.NodeName,
			IP:        p.Status.PodIP,
			StartedAt: p.CreationTimestamp.Time,
			Ready:     isPodReady(&p),
			Resources: resources,
		}
		for _, cs := range p.Status.ContainerStatuses {
			state := "waiting"
			reason := ""
			if cs.State.Running != nil {
				state = "running"
			} else if cs.State.Terminated != nil {
				state = "terminated"
				reason = cs.State.Terminated.Reason
			} else if cs.State.Waiting != nil {
				reason = cs.State.Waiting.Reason
			}
			pi.Containers = append(pi.Containers, orchestrator.ContainerStatus{
				Name:         cs.Name,
				Ready:        cs.Ready,
				RestartCount: cs.RestartCount,
				State:        state,
				Reason:       reason,
			})
			pi.RestartCount += cs.RestartCount
		}
		result = append(result, pi)
	}
	return result, nil
}

func isPodReady(pod *corev1.Pod) bool {
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func (o *Orchestrator) EnableExternalAccess(ctx context.Context, db *model.ManagedDatabase) (int32, error) {
	ns := dbNamespace(db)
	k8sName := dbK8sName(db)
	port := enginePorts[db.Engine]
	externalSvcName := fmt.Sprintf("%s-external", k8sName)

	labels := map[string]string{
		"app.kubernetes.io/name":       k8sName,
		"app.kubernetes.io/managed-by": "sailbox",
		"sailbox/db-id":                db.ID.String(),
		"sailbox/engine":               string(db.Engine),
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      externalSvcName,
			Namespace: ns,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeNodePort,
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Port:       port,
					TargetPort: *intOrString(int(port)),
				},
			},
		},
	}

	// If a specific NodePort was requested, set it
	if db.ExternalPort > 0 {
		svc.Spec.Ports[0].NodePort = db.ExternalPort
	}

	created, err := o.client.CoreV1().Services(ns).Create(ctx, svc, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			// Update existing service
			existing, getErr := o.client.CoreV1().Services(ns).Get(ctx, externalSvcName, metav1.GetOptions{})
			if getErr != nil {
				return 0, fmt.Errorf("get existing external service: %w", getErr)
			}
			return existing.Spec.Ports[0].NodePort, nil
		}
		return 0, fmt.Errorf("create external service: %w", err)
	}

	assignedPort := created.Spec.Ports[0].NodePort
	o.logger.Info("external access enabled", slog.String("db", k8sName), slog.Int("node_port", int(assignedPort)))
	return assignedPort, nil
}

func (o *Orchestrator) DisableExternalAccess(ctx context.Context, db *model.ManagedDatabase) error {
	ns := dbNamespace(db)
	k8sName := dbK8sName(db)
	externalSvcName := fmt.Sprintf("%s-external", k8sName)

	err := o.client.CoreV1().Services(ns).Delete(ctx, externalSvcName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("delete external service: %w", err)
	}

	o.logger.Info("external access disabled", slog.String("db", k8sName))
	return nil
}

func (o *Orchestrator) RunDatabaseBackup(ctx context.Context, db *model.ManagedDatabase, backupID uuid.UUID, s3Config *orchestrator.S3Config, s3Key string) error {
	ns := dbNamespace(db)
	k8sName := dbK8sName(db)
	secretName := fmt.Sprintf("%s-credentials", k8sName)
	bid := backupID.String()

	// Resolve the Pod IP directly — avoids DNS/Service issues (e.g. names starting with digits)
	podIP, err := o.getDatabasePodIP(ctx, db)
	if err != nil {
		return fmt.Errorf("resolve database pod IP: %w", err)
	}

	var dumpCmd []string
	var dumpImage string
	var backupExt string
	switch db.Engine {
	case model.DBPostgres:
		dumpImage = fmt.Sprintf("postgres:%s", db.Version)
		dumpCmd = []string{"sh", "-c", fmt.Sprintf("PGPASSWORD=$SAILBOX_PASSWORD pg_dump -h %s -U $SAILBOX_USERNAME -d $SAILBOX_DB -f /backup/%s.sql", podIP, bid)}
		backupExt = "sql"
	case model.DBMySQL:
		dumpImage = fmt.Sprintf("mysql:%s", db.Version)
		dumpCmd = []string{"sh", "-c", fmt.Sprintf("mysqldump -h %s -u $SAILBOX_USERNAME -p$SAILBOX_PASSWORD $SAILBOX_DB > /backup/%s.sql", podIP, bid)}
		backupExt = "sql"
	case model.DBMariaDB:
		dumpImage = fmt.Sprintf("mariadb:%s", db.Version)
		dumpCmd = []string{"sh", "-c", fmt.Sprintf("(mariadb-dump -h %s -u $SAILBOX_USERNAME -p$SAILBOX_PASSWORD $SAILBOX_DB || mysqldump -h %s -u $SAILBOX_USERNAME -p$SAILBOX_PASSWORD $SAILBOX_DB) > /backup/%s.sql", podIP, podIP, bid)}
		backupExt = "sql"
	case model.DBMongo:
		dumpImage = fmt.Sprintf("mongo:%s", db.Version)
		dumpCmd = []string{"sh", "-c", fmt.Sprintf("mongodump --host %s --username $SAILBOX_USERNAME --password $SAILBOX_PASSWORD --authenticationDatabase admin --db $SAILBOX_DB --archive=/backup/%s.archive", podIP, bid)}
		backupExt = "archive"
	case model.DBRedis:
		dumpImage = fmt.Sprintf("redis:%s", db.Version)
		// Redis: probe whether auth is required before using -a flag
		dumpCmd = []string{"sh", "-c", fmt.Sprintf(
			`if redis-cli -h %s PING 2>/dev/null | grep -q PONG; then AUTH=""; `+
				`elif [ -n "$SAILBOX_PASSWORD" ]; then AUTH="-a $SAILBOX_PASSWORD --no-auth-warning"; `+
				`else AUTH=""; fi; `+
				`redis-cli -h %s $AUTH BGSAVE && sleep 3 && redis-cli -h %s $AUTH --rdb /backup/%s.rdb`,
			podIP, podIP, podIP, bid)}
		backupExt = "rdb"
	default:
		return fmt.Errorf("unsupported engine for backup: %s", db.Engine)
	}

	jobName := fmt.Sprintf("backup-%s-%s", k8sName, bid[:8])
	var backoffLimit int32 = 0
	var ttl int32 = 3600
	jobLabels := map[string]string{
		"app.kubernetes.io/managed-by": "sailbox",
		"sailbox/backup-id":            backupID.String(),
		"sailbox/db-id":                db.ID.String(),
	}

	if s3Config != nil {
		// S3 backup: init container dumps to emptyDir, main container uploads to S3
		backupFile := fmt.Sprintf("/backup/%s.%s", bid, backupExt)

		// Create a temporary secret for S3 credentials
		s3SecretName := fmt.Sprintf("backup-s3-%s", bid[:8])
		s3Secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      s3SecretName,
				Namespace: ns,
				Labels:    jobLabels,
			},
			StringData: map[string]string{
				"AWS_ACCESS_KEY_ID":     s3Config.AccessKey,
				"AWS_SECRET_ACCESS_KEY": s3Config.SecretKey,
				"AWS_DEFAULT_REGION":    defaultRegion(s3Config.Region),
				"S3_ENDPOINT":           s3Config.Endpoint,
				"S3_BUCKET":             s3Config.Bucket,
			},
		}
		_, err := o.client.CoreV1().Secrets(ns).Create(ctx, s3Secret, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			return fmt.Errorf("create S3 secret: %w", err)
		}

		uploadCmd := fmt.Sprintf("aws s3 cp %s s3://$S3_BUCKET/%s --endpoint-url $S3_ENDPOINT", backupFile, s3Key)

		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{Name: jobName, Namespace: ns, Labels: jobLabels},
			Spec: batchv1.JobSpec{
				BackoffLimit:            &backoffLimit,
				TTLSecondsAfterFinished: &ttl,
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						RestartPolicy: corev1.RestartPolicyNever,
						InitContainers: []corev1.Container{
							{
								Name:    "dump",
								Image:   dumpImage,
								Command: dumpCmd,
								EnvFrom: []corev1.EnvFromSource{
									{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: secretName}}},
								},
								VolumeMounts: []corev1.VolumeMount{
									{Name: "backup", MountPath: "/backup"},
								},
							},
						},
						Containers: []corev1.Container{
							{
								Name:    "upload",
								Image:   "amazon/aws-cli:latest",
								Command: []string{"sh", "-c", uploadCmd},
								EnvFrom: []corev1.EnvFromSource{
									{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: s3SecretName}}},
								},
								VolumeMounts: []corev1.VolumeMount{
									{Name: "backup", MountPath: "/backup"},
								},
							},
						},
						Volumes: []corev1.Volume{
							{Name: "backup", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
						},
					},
				},
			},
		}

		_, err = o.client.BatchV1().Jobs(ns).Create(ctx, job, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("create S3 backup job: %w", err)
		}

		o.logger.Info("S3 database backup job created",
			slog.String("job", jobName),
			slog.String("backup_id", bid),
			slog.String("s3_bucket", s3Config.Bucket),
		)
		return nil
	}

	// PVC backup (existing fallback logic)
	backupPVCName := "backup-storage"
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: backupPVCName, Namespace: ns},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources:   corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}},
		},
	}
	_, err = o.client.CoreV1().PersistentVolumeClaims(ns).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("create backup PVC: %w", err)
	}

	// Rewrite dump commands to use /backups/ path for PVC mode (uses Pod IP)
	switch db.Engine {
	case model.DBPostgres:
		dumpCmd = []string{"sh", "-c", fmt.Sprintf("PGPASSWORD=$SAILBOX_PASSWORD pg_dump -h %s -U $SAILBOX_USERNAME -d $SAILBOX_DB -f /backups/%s.sql", podIP, bid)}
	case model.DBMySQL:
		dumpCmd = []string{"sh", "-c", fmt.Sprintf("mysqldump -h %s -u $SAILBOX_USERNAME -p$SAILBOX_PASSWORD $SAILBOX_DB > /backups/%s.sql", podIP, bid)}
	case model.DBMariaDB:
		dumpCmd = []string{"sh", "-c", fmt.Sprintf("(mariadb-dump -h %s -u $SAILBOX_USERNAME -p$SAILBOX_PASSWORD $SAILBOX_DB || mysqldump -h %s -u $SAILBOX_USERNAME -p$SAILBOX_PASSWORD $SAILBOX_DB) > /backups/%s.sql", podIP, podIP, bid)}
	case model.DBMongo:
		dumpCmd = []string{"sh", "-c", fmt.Sprintf("mongodump --host %s --username $SAILBOX_USERNAME --password $SAILBOX_PASSWORD --authenticationDatabase admin --db $SAILBOX_DB --archive=/backups/%s.archive", podIP, bid)}
	case model.DBRedis:
		dumpCmd = []string{"sh", "-c", fmt.Sprintf(
			`if redis-cli -h %s PING 2>/dev/null | grep -q PONG; then AUTH=""; `+
				`elif [ -n "$SAILBOX_PASSWORD" ]; then AUTH="-a $SAILBOX_PASSWORD --no-auth-warning"; `+
				`else AUTH=""; fi; `+
				`redis-cli -h %s $AUTH BGSAVE && sleep 3 && redis-cli -h %s $AUTH --rdb /backups/%s.rdb`,
			podIP, podIP, podIP, bid)}
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: jobName, Namespace: ns, Labels: jobLabels},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    "backup",
							Image:   dumpImage,
							Command: dumpCmd,
							EnvFrom: []corev1.EnvFromSource{
								{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: secretName}}},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "backups", MountPath: "/backups"},
							},
						},
					},
					Volumes: []corev1.Volume{
						{Name: "backups", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: backupPVCName}}},
					},
				},
			},
		},
	}

	_, err = o.client.BatchV1().Jobs(ns).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create backup job: %w", err)
	}

	o.logger.Info("database backup job created", slog.String("job", jobName), slog.String("backup_id", bid))
	return nil
}

// getDatabasePodIP returns the IP of the first ready pod for a database StatefulSet.
// This is used for backup/restore Jobs to connect directly via IP, avoiding DNS/Service issues.
func (o *Orchestrator) getDatabasePodIP(ctx context.Context, db *model.ManagedDatabase) (string, error) {
	ns := dbNamespace(db)
	k8sName := dbK8sName(db)

	pods, err := o.client.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app.kubernetes.io/name=%s", k8sName),
	})
	if err != nil {
		return "", fmt.Errorf("list pods: %w", err)
	}
	for _, p := range pods.Items {
		if p.Status.PodIP != "" && p.Status.Phase == corev1.PodRunning {
			return p.Status.PodIP, nil
		}
	}
	return "", fmt.Errorf("no running pod found for database %q", db.Name)
}

func dbEnvSecret(engine model.DBEngine, password, dbName string) map[string]string {
	switch engine {
	case model.DBPostgres:
		return map[string]string{"POSTGRES_PASSWORD": password, "POSTGRES_USER": "sailbox", "POSTGRES_DB": dbName}
	case model.DBMySQL:
		return map[string]string{"MYSQL_ROOT_PASSWORD": password, "MYSQL_DATABASE": dbName}
	case model.DBMariaDB:
		return map[string]string{"MARIADB_ROOT_PASSWORD": password, "MARIADB_DATABASE": dbName}
	case model.DBRedis:
		return map[string]string{"REDIS_PASSWORD": password}
	case model.DBMongo:
		return map[string]string{"MONGO_INITDB_ROOT_USERNAME": "sailbox", "MONGO_INITDB_ROOT_PASSWORD": password}
	default:
		return map[string]string{"PASSWORD": password}
	}
}

func dbDataPath(engine model.DBEngine) string {
	switch engine {
	case model.DBPostgres:
		return "/var/lib/postgresql/data"
	case model.DBMySQL, model.DBMariaDB:
		return "/var/lib/mysql"
	case model.DBRedis:
		return "/data"
	case model.DBMongo:
		return "/data/db"
	default:
		return "/data"
	}
}

func (o *Orchestrator) RestoreDatabaseBackup(ctx context.Context, db *model.ManagedDatabase, backupID uuid.UUID, s3Config *orchestrator.S3Config, s3Key string) error {
	if s3Config == nil {
		return fmt.Errorf("S3 config is required for restore")
	}

	ns := dbNamespace(db)
	k8sName := dbK8sName(db)
	secretName := fmt.Sprintf("%s-credentials", k8sName)
	bid := backupID.String()

	// Resolve Pod IP directly
	podIP, err := o.getDatabasePodIP(ctx, db)
	if err != nil {
		return fmt.Errorf("resolve database pod IP: %w", err)
	}

	// Determine file extension and restore command per engine
	var restoreCmd []string
	var restoreImage string
	var backupExt string
	switch db.Engine {
	case model.DBPostgres:
		restoreImage = fmt.Sprintf("postgres:%s", db.Version)
		backupExt = "sql"
		restoreCmd = []string{"sh", "-c", fmt.Sprintf("PGPASSWORD=$SAILBOX_PASSWORD psql -h %s -U $SAILBOX_USERNAME -d $SAILBOX_DB -f /backup/%s.%s", podIP, bid, backupExt)}
	case model.DBMySQL:
		restoreImage = fmt.Sprintf("mysql:%s", db.Version)
		backupExt = "sql"
		restoreCmd = []string{"sh", "-c", fmt.Sprintf("mysql -h %s -u $SAILBOX_USERNAME -p$SAILBOX_PASSWORD $SAILBOX_DB < /backup/%s.%s", podIP, bid, backupExt)}
	case model.DBMariaDB:
		restoreImage = fmt.Sprintf("mariadb:%s", db.Version)
		backupExt = "sql"
		restoreCmd = []string{"sh", "-c", fmt.Sprintf("(mariadb -h %s -u $SAILBOX_USERNAME -p$SAILBOX_PASSWORD $SAILBOX_DB || mysql -h %s -u $SAILBOX_USERNAME -p$SAILBOX_PASSWORD $SAILBOX_DB) < /backup/%s.%s", podIP, podIP, bid, backupExt)}
	case model.DBMongo:
		restoreImage = fmt.Sprintf("mongo:%s", db.Version)
		backupExt = "archive"
		restoreCmd = []string{"sh", "-c", fmt.Sprintf("mongorestore --host %s --username $SAILBOX_USERNAME --password $SAILBOX_PASSWORD --authenticationDatabase admin --db $SAILBOX_DB --archive=/backup/%s.%s --drop", podIP, bid, backupExt)}
	case model.DBRedis:
		restoreImage = fmt.Sprintf("redis:%s", db.Version)
		backupExt = "rdb"
		// Redis restore: scale down → copy RDB into PVC → scale up
		pvcName := fmt.Sprintf("%s-data", k8sName)
		jobName := fmt.Sprintf("restore-%s-%s", k8sName, bid[:8])
		jobLabels := map[string]string{
			"app.kubernetes.io/managed-by": "sailbox",
			"sailbox/restore-id":           backupID.String(),
			"sailbox/db-id":                db.ID.String(),
		}
		restoreCmd = []string{"sh", "-c", fmt.Sprintf("cp /backup/%s.%s /redis-data/dump.rdb && echo 'RDB restored to PVC'", bid, backupExt)}
		// Override volumes: mount both backup (emptyDir) and redis PVC
		s3SecretName := fmt.Sprintf("restore-s3-%s", bid[:8])
		s3Secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: s3SecretName, Namespace: ns, Labels: jobLabels},
			StringData: map[string]string{
				"AWS_ACCESS_KEY_ID": s3Config.AccessKey, "AWS_SECRET_ACCESS_KEY": s3Config.SecretKey,
				"AWS_DEFAULT_REGION": defaultRegion(s3Config.Region), "S3_ENDPOINT": s3Config.Endpoint, "S3_BUCKET": s3Config.Bucket,
			},
		}
		_, _ = o.client.CoreV1().Secrets(ns).Create(ctx, s3Secret, metav1.CreateOptions{})
		backupFile := fmt.Sprintf("/backup/%s.%s", bid, backupExt)
		downloadCmd := fmt.Sprintf("aws s3 cp s3://$S3_BUCKET/%s %s --endpoint-url $S3_ENDPOINT", s3Key, backupFile)

		// Scale down Redis first so it releases the PVC
		var zero int32 = 0
		sts, scaleErr := o.client.AppsV1().StatefulSets(ns).Get(ctx, k8sName, metav1.GetOptions{})
		if scaleErr == nil {
			sts.Spec.Replicas = &zero
			_, _ = o.client.AppsV1().StatefulSets(ns).Update(ctx, sts, metav1.UpdateOptions{})
			// Wait for pod to terminate
			for i := 0; i < 30; i++ {
				pods, _ := o.client.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
					LabelSelector: fmt.Sprintf("app.kubernetes.io/name=%s", k8sName),
				})
				if len(pods.Items) == 0 {
					break
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(1 * time.Second):
				}
			}
		}

		var backoffLimit32 int32 = 0
		var ttl32 int32 = 3600
		redisJob := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{Name: jobName, Namespace: ns, Labels: jobLabels},
			Spec: batchv1.JobSpec{
				BackoffLimit: &backoffLimit32, TTLSecondsAfterFinished: &ttl32,
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						RestartPolicy: corev1.RestartPolicyNever,
						InitContainers: []corev1.Container{{
							Name: "download", Image: "amazon/aws-cli:latest", Command: []string{"sh", "-c", downloadCmd},
							EnvFrom:      []corev1.EnvFromSource{{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: s3SecretName}}}},
							VolumeMounts: []corev1.VolumeMount{{Name: "backup", MountPath: "/backup"}},
						}},
						Containers: []corev1.Container{{
							Name: "restore", Image: restoreImage, Command: restoreCmd,
							VolumeMounts: []corev1.VolumeMount{
								{Name: "backup", MountPath: "/backup"},
								{Name: "redis-data", MountPath: "/redis-data"},
							},
						}},
						Volumes: []corev1.Volume{
							{Name: "backup", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
							{Name: "redis-data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName}}},
						},
					},
				},
			},
		}
		_, err = o.client.BatchV1().Jobs(ns).Create(ctx, redisJob, metav1.CreateOptions{})
		if err != nil {
			// Scale back up on failure
			var one int32 = 1
			if sts, e := o.client.AppsV1().StatefulSets(ns).Get(ctx, k8sName, metav1.GetOptions{}); e == nil {
				sts.Spec.Replicas = &one
				_, _ = o.client.AppsV1().StatefulSets(ns).Update(ctx, sts, metav1.UpdateOptions{})
			}
			return fmt.Errorf("create redis restore job: %w", err)
		}
		// Scale back up after job creation — the job will run while redis is down
		go func() {
			// Wait for job to complete then scale up
			for i := 0; i < 300; i++ {
				time.Sleep(2 * time.Second)
				status := o.GetRestoreJobStatus(context.Background(), backupID)
				if status == "completed" || status == "failed" {
					break
				}
			}
			var one int32 = 1
			if sts, e := o.client.AppsV1().StatefulSets(ns).Get(context.Background(), k8sName, metav1.GetOptions{}); e == nil {
				sts.Spec.Replicas = &one
				_, _ = o.client.AppsV1().StatefulSets(ns).Update(context.Background(), sts, metav1.UpdateOptions{})
			}
		}()
		o.logger.Info("redis restore job created (pod scaled down, will scale up after restore)",
			slog.String("job", jobName), slog.String("backup_id", bid))
		return nil
	default:
		return fmt.Errorf("unsupported engine for restore: %s", db.Engine)
	}

	jobName := fmt.Sprintf("restore-%s-%s", k8sName, bid[:8])
	var backoffLimit int32 = 0
	var ttl int32 = 3600
	jobLabels := map[string]string{
		"app.kubernetes.io/managed-by": "sailbox",
		"sailbox/restore-id":           backupID.String(),
		"sailbox/db-id":                db.ID.String(),
	}

	backupFile := fmt.Sprintf("/backup/%s.%s", bid, backupExt)
	downloadCmd := fmt.Sprintf("aws s3 cp s3://$S3_BUCKET/%s %s --endpoint-url $S3_ENDPOINT", s3Key, backupFile)

	// Create a temporary secret for S3 credentials
	s3SecretName := fmt.Sprintf("restore-s3-%s", bid[:8])
	s3Secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s3SecretName,
			Namespace: ns,
			Labels:    jobLabels,
		},
		StringData: map[string]string{
			"AWS_ACCESS_KEY_ID":     s3Config.AccessKey,
			"AWS_SECRET_ACCESS_KEY": s3Config.SecretKey,
			"AWS_DEFAULT_REGION":    defaultRegion(s3Config.Region),
			"S3_ENDPOINT":           s3Config.Endpoint,
			"S3_BUCKET":             s3Config.Bucket,
		},
	}
	_, err = o.client.CoreV1().Secrets(ns).Create(ctx, s3Secret, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("create S3 secret for restore: %w", err)
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: jobName, Namespace: ns, Labels: jobLabels},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					InitContainers: []corev1.Container{
						{
							Name:    "download",
							Image:   "amazon/aws-cli:latest",
							Command: []string{"sh", "-c", downloadCmd},
							EnvFrom: []corev1.EnvFromSource{
								{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: s3SecretName}}},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "backup", MountPath: "/backup"},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:    "restore",
							Image:   restoreImage,
							Command: restoreCmd,
							EnvFrom: []corev1.EnvFromSource{
								{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: secretName}}},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "backup", MountPath: "/backup"},
							},
						},
					},
					Volumes: []corev1.Volume{
						{Name: "backup", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
					},
				},
			},
		},
	}

	_, err = o.client.BatchV1().Jobs(ns).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create restore job: %w", err)
	}

	o.logger.Info("database restore job created",
		slog.String("job", jobName),
		slog.String("backup_id", bid),
		slog.String("s3_bucket", s3Config.Bucket),
	)
	return nil
}

func (o *Orchestrator) GetRestoreJobStatus(ctx context.Context, backupID uuid.UUID) string {
	jobs, err := o.client.BatchV1().Jobs("").List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("sailbox/restore-id=%s", backupID.String()),
	})
	if err != nil || len(jobs.Items) == 0 {
		return ""
	}
	job := jobs.Items[0]
	for _, cond := range job.Status.Conditions {
		if cond.Type == batchv1.JobComplete && cond.Status == corev1.ConditionTrue {
			return "completed"
		}
		if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
			return "failed"
		}
	}
	return "" // still running
}

func (o *Orchestrator) GetBackupJobStatus(ctx context.Context, backupID uuid.UUID) string {
	bid := backupID.String()[:8]
	// Search all namespaces for backup job
	jobs, err := o.client.BatchV1().Jobs("").List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("sailbox/backup-id=%s", backupID.String()),
	})
	if err != nil || len(jobs.Items) == 0 {
		// Fallback: try job name pattern
		// Jobs are named backup-{dbname}-{bid8}
		// We can't derive dbname here, so just search by label
		return ""
	}
	job := jobs.Items[0]
	_ = bid
	for _, cond := range job.Status.Conditions {
		if cond.Type == batchv1.JobComplete && cond.Status == corev1.ConditionTrue {
			return "completed"
		}
		if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
			return "failed"
		}
	}
	return "" // still running
}

func defaultRegion(r string) string {
	if r == "" {
		return "auto" // works for R2 and most S3-compatible stores
	}
	return r
}
