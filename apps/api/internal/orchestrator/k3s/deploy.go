package k3s

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/sailboxhq/sailbox/apps/api/internal/model"
	"github.com/sailboxhq/sailbox/apps/api/internal/orchestrator"
)

func (o *Orchestrator) ensureNamespace(ctx context.Context, name string) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"managed-by": "sailbox"},
		},
	}
	_, err := o.client.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func appNamespace(app *model.Application) string {
	if app.Namespace != "" {
		return app.Namespace
	}
	return "default"
}

func appK8sName(app *model.Application) string {
	if app.K8sName != "" {
		return app.K8sName
	}
	return sanitize(app.Name)
}

func sanitize(name string) string {
	n := strings.ToLower(name)
	n = strings.ReplaceAll(n, "_", "-")
	n = strings.ReplaceAll(n, " ", "-")
	if len(n) > 63 {
		n = n[:63]
	}
	return n
}

func (o *Orchestrator) Deploy(ctx context.Context, app *model.Application, opts orchestrator.DeployOpts) error {
	ns := appNamespace(app)
	name := appK8sName(app)

	if err := o.ensureNamespace(ctx, ns); err != nil {
		return fmt.Errorf("ensure namespace: %w", err)
	}

	labels := map[string]string{
		"app.kubernetes.io/name":       name,
		"app.kubernetes.io/managed-by": "sailbox",
		"sailbox/app-id":               app.ID.String(),
	}

	// Build container ports
	var containerPorts []corev1.ContainerPort
	for _, p := range opts.Ports {
		containerPorts = append(containerPorts, corev1.ContainerPort{
			ContainerPort: int32(p.ContainerPort),
			Protocol:      corev1.ProtocolTCP,
		})
	}
	if len(containerPorts) == 0 {
		containerPorts = []corev1.ContainerPort{{ContainerPort: 80, Protocol: corev1.ProtocolTCP}}
	}

	// Build env vars
	var envVars []corev1.EnvVar
	for k, v := range opts.EnvVars {
		envVars = append(envVars, corev1.EnvVar{Name: k, Value: v})
	}

	// Resource limits
	resources := corev1.ResourceRequirements{}
	if opts.CPULimit != "" || opts.MemLimit != "" {
		resources.Limits = corev1.ResourceList{}
		resources.Requests = corev1.ResourceList{}
		if opts.CPULimit != "" {
			resources.Limits[corev1.ResourceCPU] = parseResourceOrDefault(opts.CPULimit, "500m")
			cpuReq := "50m"
			if opts.CPURequest != "" {
				cpuReq = opts.CPURequest
			}
			resources.Requests[corev1.ResourceCPU] = parseResourceOrDefault(cpuReq, "50m")
		}
		if opts.MemLimit != "" {
			resources.Limits[corev1.ResourceMemory] = parseResourceOrDefault(opts.MemLimit, "512Mi")
			memReq := "64Mi"
			if opts.MemRequest != "" {
				memReq = opts.MemRequest
			}
			resources.Requests[corev1.ResourceMemory] = parseResourceOrDefault(memReq, "64Mi")
		}
	}

	replicas := opts.Replicas
	if replicas == 0 {
		replicas = 1
	}

	// Build volume mounts and PVCs
	var volumeMounts []corev1.VolumeMount
	var volumes []corev1.Volume
	for _, vol := range opts.Volumes {
		pvcName := fmt.Sprintf("%s-%s", name, vol.Name)

		// Ensure PVC exists
		if err := o.ensurePVC(ctx, ns, pvcName, vol.Size, labels); err != nil {
			o.logger.Error("failed to create PVC", slog.String("name", pvcName), slog.Any("error", err))
			continue
		}

		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      vol.Name,
			MountPath: vol.MountPath,
		})
		volumes = append(volumes, corev1.Volume{
			Name: vol.Name,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		})
	}

	// Build health probes
	var livenessProbe, readinessProbe *corev1.Probe
	if opts.HealthCheck != nil && opts.HealthCheck.Type != "" {
		probe := buildProbe(opts.HealthCheck)
		livenessProbe = probe
		readinessProbe = probe
	}

	// Deployment strategy
	var strategy appsv1.DeploymentStrategy
	if opts.DeployStrategy == "recreate" {
		strategy = appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType}
	} else {
		strategy = appsv1.DeploymentStrategy{
			Type: appsv1.RollingUpdateDeploymentStrategyType,
		}
		if opts.DeployStrategyConfig != nil && (opts.DeployStrategyConfig.MaxSurge != "" || opts.DeployStrategyConfig.MaxUnavailable != "") {
			ru := &appsv1.RollingUpdateDeployment{}
			if opts.DeployStrategyConfig.MaxSurge != "" {
				v := intstr.Parse(opts.DeployStrategyConfig.MaxSurge)
				ru.MaxSurge = &v
			}
			if opts.DeployStrategyConfig.MaxUnavailable != "" {
				v := intstr.Parse(opts.DeployStrategyConfig.MaxUnavailable)
				ru.MaxUnavailable = &v
			}
			strategy.RollingUpdate = ru
		}
	}

	// Termination grace period
	var terminationGrace *int64
	if opts.TerminationGracePeriod > 0 {
		g := int64(opts.TerminationGracePeriod)
		terminationGrace = &g
	}

	// Mount secrets as env vars from K8s Secret reference if the app has secrets.
	var envFrom []corev1.EnvFromSource
	if len(app.Secrets) > 0 {
		envFrom = append(envFrom, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: name + "-secrets",
				},
				Optional: func() *bool { b := true; return &b }(),
			},
		})
	}

	container := corev1.Container{
		Name:           name,
		Image:          opts.Image,
		Ports:          containerPorts,
		Env:            envVars,
		EnvFrom:        envFrom,
		Resources:      resources,
		VolumeMounts:   volumeMounts,
		LivenessProbe:  livenessProbe,
		ReadinessProbe: readinessProbe,
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas:                &replicas,
			RevisionHistoryLimit:    int32Ptr(3),
			ProgressDeadlineSeconds: int32Ptr(300), // 5 min — stop retrying if can't progress
			Selector:                &metav1.LabelSelector{MatchLabels: labels},
			Strategy:                strategy,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
					Annotations: map[string]string{
						"sailbox/deploy-time": time.Now().Format(time.RFC3339),
					},
				},
				Spec: func() corev1.PodSpec {
					spec := corev1.PodSpec{
						Containers:                    []corev1.Container{container},
						Volumes:                       volumes,
						TerminationGracePeriodSeconds: terminationGrace,
					}
					if opts.NodePool != "" {
						spec.NodeSelector = map[string]string{
							"sailbox/pool": opts.NodePool,
						}
					}
					return spec
				}(),
			},
		},
	}

	// Create or update
	existing, err := o.client.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err = o.client.AppsV1().Deployments(ns).Create(ctx, deployment, metav1.CreateOptions{})
		} else {
			return err
		}
	} else {
		existing.Spec = deployment.Spec
		_, err = o.client.AppsV1().Deployments(ns).Update(ctx, existing, metav1.UpdateOptions{})
	}
	if err != nil {
		return fmt.Errorf("deploy: %w", err)
	}

	// Ensure Service with full port mapping
	if err := o.ensureService(ctx, ns, name, labels, opts.Ports); err != nil {
		o.logger.Error("failed to create service", slog.Any("error", err))
	}

	// Sync ingress backend ports if they changed
	if err := o.SyncIngressPorts(ctx, app); err != nil {
		o.logger.Error("failed to sync ingress ports", slog.Any("error", err))
	}

	o.logger.Info("deployed to K3s", slog.String("name", name), slog.String("ns", ns), slog.String("image", opts.Image))
	return nil
}

// parseResourceOrDefault safely parses a K8s resource quantity, falling back to a default.
func parseResourceOrDefault(value, defaultVal string) resource.Quantity {
	if q, err := resource.ParseQuantity(value); err == nil {
		return q
	}
	return resource.MustParse(defaultVal)
}

// ensurePVC creates a PersistentVolumeClaim if it doesn't already exist.
func (o *Orchestrator) ensurePVC(ctx context.Context, ns, name, size string, labels map[string]string) error {
	_, err := o.client.CoreV1().PersistentVolumeClaims(ns).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return nil // already exists
	}
	if !errors.IsNotFound(err) {
		return err
	}

	storageSize := parseResourceOrDefault(size, "1Gi")
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    labels,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: storageSize,
				},
			},
		},
	}

	_, err = o.client.CoreV1().PersistentVolumeClaims(ns).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	o.logger.Info("created PVC", slog.String("name", name), slog.String("size", size))
	return nil
}

// buildProbe creates a K8s probe from the app health check config.
func buildProbe(hc *model.HealthCheck) *corev1.Probe {
	probe := &corev1.Probe{
		InitialDelaySeconds: int32(hc.InitialDelaySeconds),
		PeriodSeconds:       int32(hc.PeriodSeconds),
		TimeoutSeconds:      int32(hc.TimeoutSeconds),
		FailureThreshold:    int32(hc.FailureThreshold),
	}
	if probe.PeriodSeconds == 0 {
		probe.PeriodSeconds = 10
	}
	if probe.TimeoutSeconds == 0 {
		probe.TimeoutSeconds = 3
	}
	if probe.FailureThreshold == 0 {
		probe.FailureThreshold = 3
	}

	switch hc.Type {
	case "http":
		probe.ProbeHandler = corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: hc.Path,
				Port: intstr.FromInt32(int32(hc.Port)),
			},
		}
	case "tcp":
		probe.ProbeHandler = corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.FromInt32(int32(hc.Port)),
			},
		}
	case "exec":
		probe.ProbeHandler = corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: strings.Split(hc.Command, " "),
			},
		}
	}
	return probe
}

func (o *Orchestrator) ensureService(ctx context.Context, ns, name string, labels map[string]string, ports []model.PortMapping) error {
	var svcPorts []corev1.ServicePort
	for _, p := range ports {
		svcPort := int32(p.ServicePort)
		if svcPort == 0 {
			svcPort = int32(p.ContainerPort)
		}
		proto := corev1.ProtocolTCP
		if p.Protocol == "udp" {
			proto = corev1.ProtocolUDP
		}
		svcPorts = append(svcPorts, corev1.ServicePort{
			Port:       svcPort,
			TargetPort: *intOrString(int(p.ContainerPort)),
			Protocol:   proto,
			Name:       fmt.Sprintf("port-%d", svcPort),
		})
	}
	if len(svcPorts) == 0 {
		svcPorts = []corev1.ServicePort{{
			Port:       80,
			TargetPort: *intOrString(80),
			Protocol:   corev1.ProtocolTCP,
			Name:       "port-80",
		}}
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports:    svcPorts,
			Type:     corev1.ServiceTypeClusterIP,
		},
	}

	existing, err := o.client.CoreV1().Services(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err = o.client.CoreV1().Services(ns).Create(ctx, svc, metav1.CreateOptions{})
			return err
		}
		return err
	}
	existing.Spec.Ports = svcPorts
	existing.Spec.Selector = labels
	_, err = o.client.CoreV1().Services(ns).Update(ctx, existing, metav1.UpdateOptions{})
	return err
}

func (o *Orchestrator) Rollback(ctx context.Context, app *model.Application, revision int64) error {
	// Trigger rollout undo by updating annotation (forces new rollout)
	ns := appNamespace(app)
	name := appK8sName(app)

	dep, err := o.client.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if dep.Spec.Template.Annotations == nil {
		dep.Spec.Template.Annotations = make(map[string]string)
	}
	dep.Spec.Template.Annotations["sailbox/rollback-time"] = time.Now().Format(time.RFC3339)
	_, err = o.client.AppsV1().Deployments(ns).Update(ctx, dep, metav1.UpdateOptions{})
	return err
}

func (o *Orchestrator) Scale(ctx context.Context, app *model.Application, replicas int32) error {
	ns := appNamespace(app)
	name := appK8sName(app)

	scale, err := o.client.AppsV1().Deployments(ns).GetScale(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	scale.Spec.Replicas = replicas
	_, err = o.client.AppsV1().Deployments(ns).UpdateScale(ctx, name, scale, metav1.UpdateOptions{})
	return err
}

func (o *Orchestrator) Restart(ctx context.Context, app *model.Application) error {
	ns := appNamespace(app)
	name := appK8sName(app)

	dep, err := o.client.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if dep.Spec.Template.Annotations == nil {
		dep.Spec.Template.Annotations = make(map[string]string)
	}
	dep.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)
	_, err = o.client.AppsV1().Deployments(ns).Update(ctx, dep, metav1.UpdateOptions{})
	return err
}

func (o *Orchestrator) Stop(ctx context.Context, app *model.Application) error {
	return o.Scale(ctx, app, 0)
}

func (o *Orchestrator) Delete(ctx context.Context, app *model.Application) error {
	ns := appNamespace(app)
	name := appK8sName(app)
	propagation := metav1.DeletePropagationForeground

	// Delete Deployment
	_ = o.client.AppsV1().Deployments(ns).Delete(ctx, name, metav1.DeleteOptions{PropagationPolicy: &propagation})
	// Delete Service
	_ = o.client.CoreV1().Services(ns).Delete(ctx, name, metav1.DeleteOptions{})

	o.logger.Info("deleted from K3s", slog.String("name", name), slog.String("ns", ns))
	return nil
}

func (o *Orchestrator) GetStatus(ctx context.Context, app *model.Application) (*orchestrator.AppStatus, error) {
	ns := appNamespace(app)
	name := appK8sName(app)

	dep, err := o.client.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return &orchestrator.AppStatus{Phase: "not deployed"}, nil
		}
		return nil, err
	}

	// Replicas can be nil (defaults to 1 in K8s)
	replicas := int32(1)
	if dep.Spec.Replicas != nil {
		replicas = *dep.Spec.Replicas
	}

	phase := "pending"
	msg := ""
	if replicas == 0 {
		phase = "stopped"
	} else if dep.Status.ReadyReplicas == replicas {
		phase = "running"
	} else if dep.Status.ReadyReplicas > 0 {
		phase = "partial"
		msg = fmt.Sprintf("%d/%d ready", dep.Status.ReadyReplicas, replicas)
	}
	for _, cond := range dep.Status.Conditions {
		if cond.Type == appsv1.DeploymentProgressing && cond.Status == corev1.ConditionFalse {
			phase = "failed"
			msg = cond.Message
		}
	}

	return &orchestrator.AppStatus{
		Phase:           phase,
		ReadyReplicas:   dep.Status.ReadyReplicas,
		DesiredReplicas: replicas,
		Message:         msg,
	}, nil
}

func (o *Orchestrator) GetPods(ctx context.Context, app *model.Application) ([]orchestrator.PodInfo, error) {
	ns := appNamespace(app)
	name := appK8sName(app)

	pods, err := o.client.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app.kubernetes.io/name=%s", name),
	})
	if err != nil {
		return nil, err
	}

	// Fetch real-time metrics from metrics-server (built into K3s).
	// Map: podName -> {cpu, memory} usage strings
	podMetrics := o.fetchPodMetrics(ctx, ns, name)

	var result []orchestrator.PodInfo
	for _, pod := range pods.Items {
		// Skip terminated pods (Succeeded/Failed/Evicted) — they are historical,
		// not part of the current running set. Evicted pods are auto-deleted.
		if pod.Status.Phase == corev1.PodSucceeded {
			continue
		}
		if pod.Status.Phase == corev1.PodFailed && pod.Status.Reason == "Evicted" {
			// Auto-delete evicted pods to prevent buildup
			_ = o.client.CoreV1().Pods(ns).Delete(ctx, pod.Name, metav1.DeleteOptions{})
			continue
		}

		startedAt := time.Time{}
		if pod.Status.StartTime != nil {
			startedAt = pod.Status.StartTime.Time
		}

		var containers []orchestrator.ContainerStatus
		var totalRestarts int32
		allReady := true
		for _, cs := range pod.Status.ContainerStatuses {
			state := "waiting"
			reason := ""
			if cs.State.Running != nil {
				state = "running"
			} else if cs.State.Terminated != nil {
				state = "terminated"
				reason = cs.State.Terminated.Reason
			} else if cs.State.Waiting != nil {
				state = "waiting"
				reason = cs.State.Waiting.Reason
			}
			if !cs.Ready {
				allReady = false
			}
			totalRestarts += cs.RestartCount
			containers = append(containers, orchestrator.ContainerStatus{
				Name:         cs.Name,
				Ready:        cs.Ready,
				RestartCount: cs.RestartCount,
				State:        state,
				Reason:       reason,
			})
		}

		// Derive resource totals from container specs (requests/limits)
		var cpuTotal, memTotal string
		for _, c := range pod.Spec.Containers {
			if lim := c.Resources.Limits; lim != nil {
				if cpu, ok := lim[corev1.ResourceCPU]; ok {
					cpuTotal = cpu.String()
				}
				if mem, ok := lim[corev1.ResourceMemory]; ok {
					memTotal = mem.String()
				}
			}
		}

		// Merge real-time usage from metrics-server
		cpuUsed, memUsed := "", ""
		if m, ok := podMetrics[pod.Name]; ok {
			cpuUsed = m.cpuUsed
			memUsed = m.memUsed
		}

		result = append(result, orchestrator.PodInfo{
			Name:         pod.Name,
			Phase:        string(pod.Status.Phase),
			Node:         pod.Spec.NodeName,
			IP:           pod.Status.PodIP,
			StartedAt:    startedAt,
			RestartCount: totalRestarts,
			Ready:        allReady,
			Containers:   containers,
			Resources: orchestrator.ResourceMetrics{
				CPUUsed:  cpuUsed,
				CPUTotal: cpuTotal,
				MemUsed:  memUsed,
				MemTotal: memTotal,
			},
		})
	}
	return result, nil
}

// podMetric holds CPU/memory usage for a single pod.
type podMetric struct {
	cpuUsed string
	memUsed string
}

// fetchPodMetrics queries the metrics-server API for real-time resource usage.
// Returns a map of podName -> usage. Silently returns empty map if metrics-server is unavailable.
func (o *Orchestrator) fetchPodMetrics(ctx context.Context, namespace, labelName string) map[string]podMetric {
	result := make(map[string]podMetric)

	// Call: GET /apis/metrics.k8s.io/v1beta1/namespaces/{ns}/pods?labelSelector=...
	raw, err := o.client.Discovery().RESTClient().Get().
		AbsPath("/apis/metrics.k8s.io/v1beta1").
		Suffix("namespaces", namespace, "pods").
		Param("labelSelector", fmt.Sprintf("app.kubernetes.io/name=%s", labelName)).
		DoRaw(ctx)
	if err != nil {
		return result // metrics-server not available, return empty
	}

	// Parse the JSON response manually to avoid importing k8s.io/metrics
	var resp struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Containers []struct {
				Usage struct {
					CPU    string `json:"cpu"`
					Memory string `json:"memory"`
				} `json:"usage"`
			} `json:"containers"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return result
	}

	for _, item := range resp.Items {
		var cpu, mem string
		for _, c := range item.Containers {
			if cpu == "" {
				cpu = c.Usage.CPU
			}
			if mem == "" {
				mem = c.Usage.Memory
			}
		}
		result[item.Metadata.Name] = podMetric{cpuUsed: cpu, memUsed: mem}
	}
	return result
}

func (o *Orchestrator) DeletePod(ctx context.Context, podName string, app *model.Application) error {
	ns := appNamespace(app)
	return o.client.CoreV1().Pods(ns).Delete(ctx, podName, metav1.DeleteOptions{})
}

func (o *Orchestrator) GetPodEvents(ctx context.Context, app *model.Application, podName string) ([]orchestrator.PodEvent, error) {
	ns := appNamespace(app)
	events, err := o.client.CoreV1().Events(ns).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod", podName),
	})
	if err != nil {
		return nil, err
	}
	var result []orchestrator.PodEvent
	for _, e := range events.Items {
		result = append(result, orchestrator.PodEvent{
			Type:      e.Type,
			Reason:    e.Reason,
			Message:   e.Message,
			Count:     e.Count,
			FirstSeen: e.FirstTimestamp.Time,
			LastSeen:  e.LastTimestamp.Time,
		})
	}
	if result == nil {
		result = []orchestrator.PodEvent{}
	}
	return result, nil
}

func (o *Orchestrator) ConfigureHPA(ctx context.Context, app *model.Application, cfg model.AutoscalingConfig) error {
	ns := appNamespace(app)
	name := appK8sName(app)

	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    map[string]string{"app.kubernetes.io/managed-by": "sailbox"},
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       name,
			},
			MinReplicas: &cfg.MinReplicas,
			MaxReplicas: cfg.MaxReplicas,
		},
	}
	if cfg.CPUTarget > 0 {
		hpa.Spec.Metrics = append(hpa.Spec.Metrics, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name:   corev1.ResourceCPU,
				Target: autoscalingv2.MetricTarget{Type: autoscalingv2.UtilizationMetricType, AverageUtilization: &cfg.CPUTarget},
			},
		})
	}
	if cfg.MemTarget > 0 {
		hpa.Spec.Metrics = append(hpa.Spec.Metrics, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name:   corev1.ResourceMemory,
				Target: autoscalingv2.MetricTarget{Type: autoscalingv2.UtilizationMetricType, AverageUtilization: &cfg.MemTarget},
			},
		})
	}

	// Try update first, create if not found
	_, err := o.client.AutoscalingV2().HorizontalPodAutoscalers(ns).Update(ctx, hpa, metav1.UpdateOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err = o.client.AutoscalingV2().HorizontalPodAutoscalers(ns).Create(ctx, hpa, metav1.CreateOptions{})
		}
	}
	return err
}

func (o *Orchestrator) DeleteHPA(ctx context.Context, app *model.Application) error {
	ns := appNamespace(app)
	name := appK8sName(app)
	err := o.client.AutoscalingV2().HorizontalPodAutoscalers(ns).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	return nil
}

// CreateNamespace creates a K8s namespace (idempotent).
func (o *Orchestrator) CreateNamespace(ctx context.Context, name string) error {
	return o.ensureNamespace(ctx, name)
}

// DeleteNamespace deletes a K8s namespace.
func (o *Orchestrator) DeleteNamespace(ctx context.Context, name string) error {
	err := o.client.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	return nil
}

// EnsureSecret creates or updates a K8s Secret for the application.
func (o *Orchestrator) EnsureSecret(ctx context.Context, app *model.Application, secrets map[string]string) error {
	ns := appNamespace(app)
	name := appK8sName(app) + "-secrets"

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "sailbox",
				"sailbox/app-id":               app.ID.String(),
			},
		},
		StringData: secrets,
		Type:       corev1.SecretTypeOpaque,
	}

	existing, err := o.client.CoreV1().Secrets(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err = o.client.CoreV1().Secrets(ns).Create(ctx, secret, metav1.CreateOptions{})
			return err
		}
		return err
	}
	existing.StringData = secrets
	_, err = o.client.CoreV1().Secrets(ns).Update(ctx, existing, metav1.UpdateOptions{})
	return err
}
