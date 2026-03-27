package k3s

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sailboxhq/sailbox/apps/api/internal/orchestrator"
)

func (o *Orchestrator) GetNodes(ctx context.Context) ([]orchestrator.NodeInfo, error) {
	nodes, err := o.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var result []orchestrator.NodeInfo
	for _, node := range nodes.Items {
		status := "NotReady"
		for _, cond := range node.Status.Conditions {
			if cond.Type == "Ready" && cond.Status == "True" {
				status = "Ready"
			}
		}

		var roles []string
		for label := range node.Labels {
			if label == "node-role.kubernetes.io/control-plane" {
				roles = append(roles, "control-plane")
			}
			if label == "node-role.kubernetes.io/master" {
				roles = append(roles, "master")
			}
		}
		if len(roles) == 0 {
			roles = []string{"worker"}
		}

		// Get InternalIP from node addresses
		nodeIP := ""
		for _, addr := range node.Status.Addresses {
			if addr.Type == "InternalIP" {
				nodeIP = addr.Address
				break
			}
		}

		cpuCap := node.Status.Capacity.Cpu()
		memCap := node.Status.Capacity.Memory()
		storageCap := node.Status.Capacity.StorageEphemeral()

		result = append(result, orchestrator.NodeInfo{
			Name:    node.Name,
			IP:      nodeIP,
			Status:  status,
			Roles:   roles,
			Pool:    node.Labels["sailbox/pool"],
			Version: node.Status.NodeInfo.KubeletVersion,
			OS:      node.Status.NodeInfo.OperatingSystem,
			Arch:    node.Status.NodeInfo.Architecture,
			Resources: orchestrator.ResourceMetrics{
				CPUTotal:     cpuCap.String(),
				MemTotal:     fmt.Sprintf("%dMi", memCap.Value()/(1024*1024)),
				StorageTotal: fmt.Sprintf("%dGi", storageCap.Value()/(1024*1024*1024)),
			},
		})
	}

	return result, nil
}

func (o *Orchestrator) GetClusterMetrics(ctx context.Context) (*orchestrator.ClusterMetrics, error) {
	nodes, err := o.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	pods, err := o.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	running := 0
	for _, pod := range pods.Items {
		if pod.Status.Phase == "Running" {
			running++
		}
	}

	var totalCPU, totalMem int64
	for _, node := range nodes.Items {
		totalCPU += node.Status.Capacity.Cpu().MilliValue()
		totalMem += node.Status.Capacity.Memory().Value() / (1024 * 1024)
	}

	// Fetch real-time usage from metrics-server
	var usedCPU, usedMem int64
	nodeMetrics := o.fetchNodeMetrics(ctx)
	for _, m := range nodeMetrics {
		usedCPU += parseMilliValue(m.cpuUsed)
		usedMem += parseMemMiB(m.memUsed)
	}

	return &orchestrator.ClusterMetrics{
		Nodes:       len(nodes.Items),
		TotalPods:   len(pods.Items),
		RunningPods: running,
		Resources: orchestrator.ResourceMetrics{
			CPUUsed:  fmt.Sprintf("%dm", usedCPU),
			CPUTotal: fmt.Sprintf("%dm", totalCPU),
			MemUsed:  fmt.Sprintf("%dMi", usedMem),
			MemTotal: fmt.Sprintf("%dMi", totalMem),
		},
	}, nil
}

func parseMilliValue(s string) int64 {
	if s == "" || s == "0" {
		return 0
	}
	if strings.HasSuffix(s, "n") {
		s = strings.TrimSuffix(s, "n")
		v := int64(0)
		for _, c := range s {
			if c >= '0' && c <= '9' {
				v = v*10 + int64(c-'0')
			}
		}
		return v / 1_000_000
	}
	if strings.HasSuffix(s, "m") {
		s = strings.TrimSuffix(s, "m")
		v := int64(0)
		for _, c := range s {
			if c >= '0' && c <= '9' {
				v = v*10 + int64(c-'0')
			}
		}
		return v
	}
	v := int64(0)
	for _, c := range s {
		if c >= '0' && c <= '9' {
			v = v*10 + int64(c-'0')
		}
	}
	return v * 1000
}

func parseMemMiB(s string) int64 {
	if s == "" || s == "0" {
		return 0
	}
	if strings.HasSuffix(s, "Ki") {
		s = strings.TrimSuffix(s, "Ki")
		v := int64(0)
		for _, c := range s {
			if c >= '0' && c <= '9' {
				v = v*10 + int64(c-'0')
			}
		}
		return v / 1024
	}
	if strings.HasSuffix(s, "Mi") {
		s = strings.TrimSuffix(s, "Mi")
		v := int64(0)
		for _, c := range s {
			if c >= '0' && c <= '9' {
				v = v*10 + int64(c-'0')
			}
		}
		return v
	}
	if strings.HasSuffix(s, "Gi") {
		s = strings.TrimSuffix(s, "Gi")
		v := int64(0)
		for _, c := range s {
			if c >= '0' && c <= '9' {
				v = v*10 + int64(c-'0')
			}
		}
		return v * 1024
	}
	return 0
}

func (o *Orchestrator) GetNamespaceMetrics(ctx context.Context, namespace string) (*orchestrator.ResourceMetrics, error) {
	return &orchestrator.ResourceMetrics{}, nil
}

func (o *Orchestrator) GetAllPods(ctx context.Context) ([]orchestrator.PodInfo, error) {
	pods, err := o.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	// Fetch metrics for all pods across all namespaces
	allPodMetrics := o.fetchAllPodMetrics(ctx)

	var result []orchestrator.PodInfo
	for _, pod := range pods.Items {
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

		cpuUsed, memUsed := "", ""
		if m, ok := allPodMetrics[pod.Name]; ok {
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
			AppID:        pod.Labels["sailbox/app-id"],
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

// fetchAllPodMetrics queries metrics-server for all pods across all namespaces.
func (o *Orchestrator) fetchAllPodMetrics(ctx context.Context) map[string]podMetric {
	result := make(map[string]podMetric)

	raw, err := o.client.Discovery().RESTClient().Get().
		AbsPath("/apis/metrics.k8s.io/v1beta1/pods").
		DoRaw(ctx)
	if err != nil {
		return result
	}

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
			cpu = c.Usage.CPU
			mem = c.Usage.Memory
		}
		result[item.Metadata.Name] = podMetric{cpuUsed: cpu, memUsed: mem}
	}
	return result
}

func (o *Orchestrator) GetClusterEvents(ctx context.Context, limit int) ([]orchestrator.ClusterEvent, error) {
	events, err := o.client.CoreV1().Events("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	// Sort by LastTimestamp descending
	sort.Slice(events.Items, func(i, j int) bool {
		ti := events.Items[i].LastTimestamp.Time
		tj := events.Items[j].LastTimestamp.Time
		return ti.After(tj)
	})

	count := len(events.Items)
	if count > limit {
		count = limit
	}

	result := make([]orchestrator.ClusterEvent, 0, count)
	for i := 0; i < count; i++ {
		e := events.Items[i]
		involvedObj := fmt.Sprintf("%s/%s", e.InvolvedObject.Kind, e.InvolvedObject.Name)
		result = append(result, orchestrator.ClusterEvent{
			Type:           e.Type,
			Reason:         e.Reason,
			Message:        e.Message,
			Namespace:      e.Namespace,
			InvolvedObject: involvedObj,
			Count:          e.Count,
			FirstSeen:      e.FirstTimestamp.Time,
			LastSeen:       e.LastTimestamp.Time,
		})
	}
	return result, nil
}

func (o *Orchestrator) GetPVCs(ctx context.Context) ([]orchestrator.PVCInfo, error) {
	pvcs, err := o.client.CoreV1().PersistentVolumeClaims("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	// Build PVC→App map by scanning all pods
	pods, _ := o.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	pvcUsers := make(map[string]map[string]bool) // "namespace/pvcName" → set of app names
	if pods != nil {
		for _, pod := range pods.Items {
			// Derive app name from owner or labels
			appName := pod.Labels["app"]
			if appName == "" {
				appName = pod.Labels["app.kubernetes.io/name"]
			}
			if appName == "" {
				// Fallback: strip replicaset hash from pod name
				appName = pod.Name
			}
			for _, vol := range pod.Spec.Volumes {
				if vol.PersistentVolumeClaim != nil {
					key := pod.Namespace + "/" + vol.PersistentVolumeClaim.ClaimName
					if pvcUsers[key] == nil {
						pvcUsers[key] = make(map[string]bool)
					}
					pvcUsers[key][appName] = true
				}
			}
		}
	}

	var result []orchestrator.PVCInfo
	for _, pvc := range pvcs.Items {
		capacity := ""
		if qty, ok := pvc.Status.Capacity[corev1.ResourceStorage]; ok {
			capacity = qty.String()
		} else if req, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
			capacity = req.String()
		}

		storageClass := ""
		if pvc.Spec.StorageClassName != nil {
			storageClass = *pvc.Spec.StorageClassName
		}

		key := pvc.Namespace + "/" + pvc.Name
		var usedBy []string
		for name := range pvcUsers[key] {
			usedBy = append(usedBy, name)
		}
		result = append(result, orchestrator.PVCInfo{
			Name:         pvc.Name,
			Namespace:    pvc.Namespace,
			Status:       string(pvc.Status.Phase),
			Capacity:     capacity,
			StorageClass: storageClass,
			VolumeName:   pvc.Spec.VolumeName,
			UsedBy:       usedBy,
		})
	}
	return result, nil
}

func (o *Orchestrator) GetNamespaces(ctx context.Context) ([]orchestrator.NamespaceInfo, error) {
	namespaces, err := o.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var result []orchestrator.NamespaceInfo
	for _, ns := range namespaces.Items {
		pods, _ := o.client.CoreV1().Pods(ns.Name).List(ctx, metav1.ListOptions{})
		podCount := 0
		if pods != nil {
			podCount = len(pods.Items)
		}

		svcs, _ := o.client.CoreV1().Services(ns.Name).List(ctx, metav1.ListOptions{})
		svcCount := 0
		if svcs != nil {
			svcCount = len(svcs.Items)
		}

		result = append(result, orchestrator.NamespaceInfo{
			Name:     ns.Name,
			Status:   string(ns.Status.Phase),
			PodCount: podCount,
			SvcCount: svcCount,
		})
	}
	return result, nil
}

func (o *Orchestrator) GetNodeMetrics(ctx context.Context) ([]orchestrator.NodeMetrics, error) {
	nodes, err := o.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	// Get all pods to count per node
	allPods, _ := o.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	podCountByNode := make(map[string]int)
	if allPods != nil {
		for _, pod := range allPods.Items {
			if pod.Spec.NodeName != "" {
				podCountByNode[pod.Spec.NodeName]++
			}
		}
	}

	// Fetch node metrics from metrics-server
	nodeMetricsMap := o.fetchNodeMetrics(ctx)

	var result []orchestrator.NodeMetrics
	for _, node := range nodes.Items {
		cpuCap := node.Status.Capacity.Cpu()
		memCap := node.Status.Capacity.Memory()

		cpuUsed, memUsed := "", ""
		if m, ok := nodeMetricsMap[node.Name]; ok {
			cpuUsed = m.cpuUsed
			memUsed = m.memUsed
		}

		result = append(result, orchestrator.NodeMetrics{
			Name:     node.Name,
			CPUUsed:  cpuUsed,
			CPUTotal: cpuCap.String(),
			MemUsed:  memUsed,
			MemTotal: fmt.Sprintf("%dMi", memCap.Value()/(1024*1024)),
			PodCount: podCountByNode[node.Name],
		})
	}
	return result, nil
}

type nodeMetric struct {
	cpuUsed string
	memUsed string
}

func (o *Orchestrator) fetchNodeMetrics(ctx context.Context) map[string]nodeMetric {
	result := make(map[string]nodeMetric)

	raw, err := o.client.Discovery().RESTClient().Get().
		AbsPath("/apis/metrics.k8s.io/v1beta1/nodes").
		DoRaw(ctx)
	if err != nil {
		return result
	}

	var resp struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Usage struct {
				CPU    string `json:"cpu"`
				Memory string `json:"memory"`
			} `json:"usage"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return result
	}

	for _, item := range resp.Items {
		result[item.Metadata.Name] = nodeMetric{
			cpuUsed: item.Usage.CPU,
			memUsed: item.Usage.Memory,
		}
	}
	return result
}

func (o *Orchestrator) GetClusterTopology(ctx context.Context) (*orchestrator.ClusterTopology, error) {
	topo := &orchestrator.ClusterTopology{}

	// Nodes
	nodeList, err := o.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, n := range nodeList.Items {
			status := "NotReady"
			for _, c := range n.Status.Conditions {
				if c.Type == corev1.NodeReady && c.Status == corev1.ConditionTrue {
					status = "Ready"
				}
			}
			var roles []string
			for label := range n.Labels {
				if strings.HasPrefix(label, "node-role.kubernetes.io/") {
					roles = append(roles, strings.TrimPrefix(label, "node-role.kubernetes.io/"))
				}
			}
			ip := ""
			if len(n.Status.Addresses) > 0 {
				ip = n.Status.Addresses[0].Address
			}
			topo.Nodes = append(topo.Nodes, orchestrator.TopologyNode{
				Name:   n.Name,
				Status: status,
				IP:     ip,
				Roles:  strings.Join(roles, ","),
			})
		}
	}

	// Deployments (sailbox-managed)
	deps, err := o.client.AppsV1().Deployments("").List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/managed-by=sailbox",
	})
	if err == nil {
		for _, d := range deps.Items {
			desired := int32(1)
			if d.Spec.Replicas != nil {
				desired = *d.Spec.Replicas
			}
			topo.Deployments = append(topo.Deployments, orchestrator.TopologyDeployment{
				Name:      d.Name,
				Namespace: d.Namespace,
				Ready:     d.Status.ReadyReplicas,
				Desired:   desired,
				AppID:     d.Labels["sailbox/app-id"],
			})
		}
	}

	// Pods (sailbox-managed)
	podList, err := o.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/managed-by=sailbox",
	})
	if err == nil {
		for _, p := range podList.Items {
			dep := ""
			for _, ref := range p.OwnerReferences {
				if ref.Kind == "ReplicaSet" {
					parts := strings.Split(ref.Name, "-")
					if len(parts) > 1 {
						dep = strings.Join(parts[:len(parts)-1], "-")
					}
				}
			}
			topo.Pods = append(topo.Pods, orchestrator.TopologyPod{
				Name:       p.Name,
				Namespace:  p.Namespace,
				Phase:      string(p.Status.Phase),
				Node:       p.Spec.NodeName,
				IP:         p.Status.PodIP,
				AppID:      p.Labels["sailbox/app-id"],
				Deployment: dep,
			})
		}
	}

	// Services (sailbox-managed)
	svcList, err := o.client.CoreV1().Services("").List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/managed-by=sailbox",
	})
	if err == nil {
		for _, s := range svcList.Items {
			var ports []string
			for _, p := range s.Spec.Ports {
				ports = append(ports, fmt.Sprintf("%d/%s", p.Port, p.Protocol))
			}
			topo.Services = append(topo.Services, orchestrator.TopologyService{
				Name:      s.Name,
				Namespace: s.Namespace,
				Type:      string(s.Spec.Type),
				ClusterIP: s.Spec.ClusterIP,
				Ports:     strings.Join(ports, ", "),
				AppID:     s.Labels["sailbox/app-id"],
			})
		}
	}

	// Ingresses (sailbox-managed)
	ingList, err := o.client.NetworkingV1().Ingresses("").List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/managed-by=sailbox",
	})
	if err == nil {
		for _, ing := range ingList.Items {
			for _, rule := range ing.Spec.Rules {
				svcName := ""
				if rule.HTTP != nil && len(rule.HTTP.Paths) > 0 {
					svcName = rule.HTTP.Paths[0].Backend.Service.Name
				}
				topo.Ingresses = append(topo.Ingresses, orchestrator.TopologyIngress{
					Name:      ing.Name,
					Namespace: ing.Namespace,
					Host:      rule.Host,
					Service:   svcName,
					AppID:     ing.Labels["sailbox/app-id"],
				})
			}
		}
	}

	return topo, nil
}

func (o *Orchestrator) SetNodeLabel(ctx context.Context, nodeName, key, value string) error {
	node, err := o.client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if node.Labels == nil {
		node.Labels = make(map[string]string)
	}
	node.Labels[key] = value
	_, err = o.client.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	return err
}

func (o *Orchestrator) RemoveNodeLabel(ctx context.Context, nodeName, key string) error {
	node, err := o.client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	delete(node.Labels, key)
	_, err = o.client.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	return err
}

func (o *Orchestrator) GetNodePools(ctx context.Context) ([]string, error) {
	nodes, err := o.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	poolSet := make(map[string]bool)
	for _, n := range nodes.Items {
		if pool, ok := n.Labels["sailbox/pool"]; ok && pool != "" {
			poolSet[pool] = true
		}
	}
	var pools []string
	for p := range poolSet {
		pools = append(pools, p)
	}
	sort.Strings(pools)
	return pools, nil
}
