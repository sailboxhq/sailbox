import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api } from "@/lib/api";
import type {
  CleanupResult,
  CleanupStats,
  ClusterEvent,
  ClusterMetrics,
  ClusterTopology,
  DaemonSetInfo,
  HelmRelease,
  NamespaceInfo,
  NodeInfo,
  NodeMetrics,
  PodInfo,
  PVCInfo,
  TraefikConfig,
} from "@/types/api";

export const clusterKeys = {
  nodes: ["cluster", "nodes"] as const,
  metrics: ["cluster", "metrics"] as const,
  pods: ["cluster", "pods"] as const,
  events: ["cluster", "events"] as const,
  pvcs: ["cluster", "pvcs"] as const,
  namespaces: ["cluster", "namespaces"] as const,
  nodeMetrics: ["cluster", "node-metrics"] as const,
  topology: ["cluster", "topology"] as const,
};

export function useClusterNodes() {
  return useQuery({
    queryKey: clusterKeys.nodes,
    queryFn: () => api.get<NodeInfo[]>("/api/v1/cluster/nodes"),
  });
}

export function useClusterMetrics() {
  return useQuery({
    queryKey: clusterKeys.metrics,
    queryFn: () => api.get<ClusterMetrics>("/api/v1/cluster/metrics"),
    refetchInterval: 30_000, // live K8s data, not DB-backed
  });
}

export function useClusterPods() {
  return useQuery({
    queryKey: clusterKeys.pods,
    queryFn: () => api.get<PodInfo[]>("/api/v1/cluster/pods"),
  });
}

export function useClusterEvents() {
  return useQuery({
    queryKey: clusterKeys.events,
    queryFn: () => api.get<ClusterEvent[]>("/api/v1/cluster/events?limit=100"),
  });
}

export function useClusterPVCs() {
  return useQuery({
    queryKey: clusterKeys.pvcs,
    queryFn: () => api.get<PVCInfo[]>("/api/v1/cluster/pvcs"),
  });
}

export function useClusterNamespaces() {
  return useQuery({
    queryKey: clusterKeys.namespaces,
    queryFn: () => api.get<NamespaceInfo[]>("/api/v1/cluster/namespaces"),
  });
}

export function useNodeMetrics() {
  return useQuery({
    queryKey: clusterKeys.nodeMetrics,
    queryFn: () => api.get<NodeMetrics[]>("/api/v1/cluster/node-metrics"),
  });
}

export function useSetNodePool() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ nodeName, pool }: { nodeName: string; pool: string }) =>
      api.put(`/api/v1/cluster/nodes/${nodeName}/pool`, { pool }),
    onSuccess: () => {
      toast.success("Node pool updated");
      qc.invalidateQueries({ queryKey: ["cluster"] });
    },
    onError: (err: any) => toast.error(err?.detail || "Failed to update node pool"),
  });
}

export function useNodePools() {
  return useQuery({
    queryKey: [...clusterKeys.nodes, "pools"] as const,
    queryFn: () => api.get<string[]>("/api/v1/cluster/node-pools"),
  });
}

export function useClusterTopology() {
  return useQuery({
    queryKey: clusterKeys.topology,
    queryFn: () => api.get<ClusterTopology>("/api/v1/cluster/topology"),
  });
}

export function useTraefikConfig() {
  return useQuery({
    queryKey: ["cluster", "traefik-config"],
    queryFn: () => api.get<TraefikConfig>("/api/v1/cluster/traefik-config"),
  });
}

export function useUpdateTraefikConfig() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (yaml: string) => api.put("/api/v1/cluster/traefik-config", { yaml }),
    onSuccess: () => {
      toast.success("Traefik config updated");
      qc.invalidateQueries({ queryKey: ["cluster", "traefik-config"] });
    },
    onError: (err: any) => toast.error(err?.detail || "Failed to update config"),
  });
}

export function useHelmReleases() {
  return useQuery({
    queryKey: ["cluster", "helm-releases"],
    queryFn: () => api.get<HelmRelease[]>("/api/v1/cluster/helm-releases"),
  });
}

export function useDaemonSets() {
  return useQuery({
    queryKey: ["cluster", "daemonsets"],
    queryFn: () => api.get<DaemonSetInfo[]>("/api/v1/cluster/daemonsets"),
  });
}

export function useDeletePVC() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ namespace, name }: { namespace: string; name: string }) =>
      api.delete(`/api/v1/cluster/pvcs/${namespace}/${name}`),
    onSuccess: () => {
      toast.success("Volume deleted");
      qc.invalidateQueries({ queryKey: ["cluster"] });
    },
    onError: (err: any) => toast.error(err?.detail || "Failed to delete volume"),
  });
}

export function useExpandPVC() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ namespace, name, size }: { namespace: string; name: string; size: string }) =>
      api.put(`/api/v1/cluster/pvcs/${namespace}/${name}/expand`, { size }),
    onSuccess: () => {
      toast.success("Volume expanded");
      qc.invalidateQueries({ queryKey: ["cluster"] });
    },
    onError: (err: any) => toast.error(err?.detail || "Failed to expand volume"),
  });
}

// ── Cluster Health / Cleanup ──────────────────────────────────────

export function useCleanupStats() {
  return useQuery({
    queryKey: ["cluster", "cleanup-stats"],
    queryFn: () => api.get<CleanupStats>("/api/v1/cluster/cleanup/stats"),
    refetchInterval: 30_000,
  });
}

function useCleanupMutation(endpoint: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.post<CleanupResult>(`/api/v1/cluster/cleanup/${endpoint}`),
    onSuccess: (result) => {
      toast.success(result.message);
      // Immediate + delayed refresh to catch K8s propagation delay
      qc.invalidateQueries({ queryKey: ["cluster"] });
      setTimeout(() => qc.invalidateQueries({ queryKey: ["cluster"] }), 1500);
    },
    onError: (err: any) => toast.error(err?.detail || "Cleanup failed"),
  });
}

export function useCleanupEvictedPods() {
  return useCleanupMutation("evicted-pods");
}
export function useCleanupFailedPods() {
  return useCleanupMutation("failed-pods");
}
export function useCleanupCompletedPods() {
  return useCleanupMutation("completed-pods");
}
export function useCleanupStaleReplicaSets() {
  return useCleanupMutation("stale-replicasets");
}
export function useCleanupCompletedJobs() {
  return useCleanupMutation("completed-jobs");
}
export function useCleanupOrphanIngresses() {
  return useCleanupMutation("orphan-ingresses");
}
