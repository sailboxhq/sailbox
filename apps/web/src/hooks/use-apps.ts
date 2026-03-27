import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api } from "@/lib/api";
import type {
  App,
  AppStatus,
  Deployment,
  Domain,
  PaginatedResponse,
  PodEvent,
  PodInfo,
  WebhookConfig,
} from "@/types/api";
import { projectKeys } from "./use-projects";

export const appKeys = {
  detail: (id: string) => ["apps", id] as const,
  status: (id: string) => ["apps", id, "status"] as const,
  pods: (id: string) => ["apps", id, "pods"] as const,
  podEvents: (id: string, podName: string) => ["apps", id, "pods", podName, "events"] as const,
  deployments: (id: string) => ["apps", id, "deployments"] as const,
  deployment: (id: string) => ["deployments", id] as const,
  domains: (id: string) => ["apps", id, "domains"] as const,
};

// ── Queries ───────────────────────────────────────────────────────

export function useProjectApps(projectId: string) {
  return useQuery({
    queryKey: projectKeys.apps(projectId),
    queryFn: () => api.get<PaginatedResponse<App>>(`/api/v1/projects/${projectId}/apps`),
    select: (data) => data.items ?? [],
  });
}

export function useApp(appId: string) {
  return useQuery({
    queryKey: appKeys.detail(appId),
    queryFn: () => api.get<App>(`/api/v1/apps/${appId}`),
  });
}

export function useAppStatus(appId: string) {
  return useQuery({
    queryKey: appKeys.status(appId),
    queryFn: () => api.get<AppStatus>(`/api/v1/apps/${appId}/status`),
    refetchInterval: (query) => {
      const phase = query.state.data?.phase;
      if (!phase) return 3_000;
      const stable = ["running", "stopped", "error", "not deployed"];
      return stable.includes(phase) ? 30_000 : 3_000;
    },
  });
}

export function useAppPods(appId: string) {
  return useQuery({
    queryKey: appKeys.pods(appId),
    queryFn: () => api.get<PodInfo[]>(`/api/v1/apps/${appId}/pods`),
    refetchInterval: (query) => {
      const pods = query.state.data;
      if (!pods) return 3_000;
      if (pods.length === 0) return 30_000;
      const allRunning = pods.every((p) => p.phase === "Running");
      return allRunning ? 30_000 : 3_000; // Fast poll during transitions, slow poll when stable
    },
  });
}

export function usePodEvents(appId: string, podName: string) {
  return useQuery({
    queryKey: appKeys.podEvents(appId, podName),
    queryFn: () => api.get<PodEvent[]>(`/api/v1/apps/${appId}/pods/${podName}/events`),
    enabled: !!podName,
  });
}

export function useAppDeployments(appId: string) {
  return useQuery({
    queryKey: appKeys.deployments(appId),
    queryFn: () => api.get<PaginatedResponse<Deployment>>(`/api/v1/apps/${appId}/deployments`),
    select: (data) => data.items ?? [],
  });
}

export function useDeploymentDetail(deployId: string) {
  return useQuery({
    queryKey: appKeys.deployment(deployId),
    queryFn: () => api.get<Deployment>(`/api/v1/deployments/${deployId}`),
    enabled: !!deployId,
    refetchInterval: (query) => {
      const status = query.state.data?.status;
      // Poll every 3s while build/deploy is in progress
      if (status === "queued" || status === "building" || status === "deploying") {
        return 3000;
      }
      return false;
    },
  });
}

export function useAppDomains(appId: string) {
  return useQuery({
    queryKey: appKeys.domains(appId),
    queryFn: () => api.get<Domain[]>(`/api/v1/apps/${appId}/domains`),
  });
}

// ── Invalidation helpers ──────────────────────────────────────────

function useInvalidateApp(appId: string) {
  const qc = useQueryClient();
  return () => {
    const invalidate = () => {
      qc.invalidateQueries({ queryKey: ["apps", appId] });
      qc.invalidateQueries({ queryKey: ["projects"], type: "active" });
    };
    invalidate();
    setTimeout(invalidate, 1500);
  };
}

// ── Mutations ─────────────────────────────────────────────────────

export function useCreateApp(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: {
      name: string;
      source_type: string;
      docker_image?: string;
      git_repo?: string;
      git_branch?: string;
    }) => api.post<App>("/api/v1/apps", { ...data, project_id: projectId }),
    onSuccess: (_, vars) => {
      toast.success(`Application "${vars.name}" created`);
      qc.invalidateQueries({ queryKey: projectKeys.apps(projectId) });
    },
    onError: (err: any) => toast.error(err?.detail || "Failed to create app"),
  });
}

export function useUpdateApp(appId: string) {
  const invalidate = useInvalidateApp(appId);
  return useMutation({
    mutationFn: (data: Partial<App>) => api.patch<App>(`/api/v1/apps/${appId}`, data),
    onSuccess: () => {
      toast.success("Settings saved");
      invalidate();
    },
    onError: (err: any) => toast.error(err?.detail || "Save failed"),
  });
}

export function useDeploy(appId: string) {
  const invalidate = useInvalidateApp(appId);
  return useMutation({
    mutationFn: (opts?: { force_build?: boolean }) =>
      api.post(`/api/v1/apps/${appId}/deploy`, opts ?? {}),
    onSuccess: () => {
      toast.success("Deployment triggered");
      invalidate();
    },
    onError: (err: any) => toast.error(err?.detail || "Deploy failed"),
  });
}

export function useCancelDeploy(appId: string) {
  const invalidate = useInvalidateApp(appId);
  return useMutation({
    mutationFn: (deployId: string) => api.post(`/api/v1/deployments/${deployId}/cancel`),
    onSuccess: () => {
      toast.success("Deployment cancelled");
      invalidate();
    },
    onError: (err: any) => toast.error(err?.detail || "Cancel failed"),
  });
}

export function useClearBuildCache(appId: string) {
  return useMutation({
    mutationFn: () => api.post(`/api/v1/apps/${appId}/clear-cache`),
    onSuccess: () => toast.success("Build cache cleared"),
    onError: (err: any) => toast.error(err?.detail || "Failed to clear cache"),
  });
}

export function useRestartApp(appId: string) {
  const invalidate = useInvalidateApp(appId);
  return useMutation({
    mutationFn: () => api.post(`/api/v1/apps/${appId}/restart`),
    onSuccess: () => {
      toast.success("Restart triggered");
      invalidate();
    },
    onError: (err: any) => toast.error(err?.detail || "Restart failed"),
  });
}

export function useStopApp(appId: string) {
  const invalidate = useInvalidateApp(appId);
  return useMutation({
    mutationFn: () => api.post(`/api/v1/apps/${appId}/stop`),
    onSuccess: () => {
      toast.success("Stopped");
      invalidate();
    },
    onError: (err: any) => toast.error(err?.detail || "Stop failed"),
  });
}

export function useScaleApp(appId: string) {
  const invalidate = useInvalidateApp(appId);
  return useMutation({
    mutationFn: (replicas: number) => api.post(`/api/v1/apps/${appId}/scale`, { replicas }),
    onSuccess: (_, replicas) => {
      toast.success(`Scaled to ${replicas}`);
      invalidate();
    },
    onError: (err: any) => toast.error(err?.detail || "Scale failed"),
  });
}

export function useUpdateEnv(appId: string) {
  const invalidate = useInvalidateApp(appId);
  return useMutation({
    mutationFn: (envVars: Record<string, string>) =>
      api.put(`/api/v1/apps/${appId}/env`, { env_vars: envVars }),
    onSuccess: () => {
      toast.success("Environment saved");
      invalidate();
    },
    onError: (err: any) => toast.error(err?.detail || "Save failed"),
  });
}

export function useAddDomain(appId: string) {
  const invalidate = useInvalidateApp(appId);
  return useMutation({
    mutationFn: (data: { host: string; tls: boolean; auto_cert: boolean }) =>
      api.post(`/api/v1/apps/${appId}/domains`, data),
    onSuccess: () => {
      toast.success("Domain added");
      invalidate();
    },
    onError: (err: any) => toast.error(err?.detail || "Failed"),
  });
}

export function useGenerateDomain(appId: string) {
  const invalidate = useInvalidateApp(appId);
  return useMutation({
    mutationFn: () => api.post<Domain>(`/api/v1/apps/${appId}/domains/generate`),
    onSuccess: (domain) => {
      toast.success(`Generated: ${domain.host}`);
      invalidate();
    },
    onError: (err: any) => toast.error(err?.detail || "Failed"),
  });
}

export function useDeleteDomain(appId: string) {
  const invalidate = useInvalidateApp(appId);
  return useMutation({
    mutationFn: ({ id, host }: { id: string; host: string }) =>
      api.delete(`/api/v1/domains/${id}`).then(() => host),
    onSuccess: (host) => {
      toast.success(`${host} removed`);
      invalidate();
    },
    onError: (err: any) => toast.error(err?.detail || "Failed"),
  });
}

export function useUpdateDomain(appId: string) {
  const invalidate = useInvalidateApp(appId);
  return useMutation({
    mutationFn: ({ id, ...data }: { id: string; host?: string; force_https?: boolean }) =>
      api.patch(`/api/v1/domains/${id}`, data),
    onSuccess: () => {
      toast.success("Domain updated");
      invalidate();
    },
    onError: (err: any) => toast.error(err?.detail || "Failed"),
  });
}

// ── Webhook ──────────────────────────────────────────────────────

export function useWebhookConfig(appId: string) {
  return useQuery({
    queryKey: ["apps", appId, "webhook"] as const,
    queryFn: () => api.get<WebhookConfig>(`/api/v1/apps/${appId}/webhook`),
  });
}

export function useEnableWebhook(appId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.post<WebhookConfig>(`/api/v1/apps/${appId}/webhook/enable`),
    onSuccess: () => {
      toast.success("Webhook enabled");
      qc.invalidateQueries({ queryKey: ["apps", appId] });
    },
    onError: (err: any) => toast.error(err?.detail || "Failed"),
  });
}

export function useDisableWebhook(appId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.post(`/api/v1/apps/${appId}/webhook/disable`),
    onSuccess: () => {
      toast.success("Webhook disabled");
      qc.invalidateQueries({ queryKey: ["apps", appId] });
    },
    onError: (err: any) => toast.error(err?.detail || "Failed"),
  });
}

export function useRegenerateWebhook(appId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.post<WebhookConfig>(`/api/v1/apps/${appId}/webhook/regenerate`),
    onSuccess: () => {
      toast.success("Webhook secret regenerated");
      qc.invalidateQueries({ queryKey: ["apps", appId] });
    },
    onError: (err: any) => toast.error(err?.detail || "Failed"),
  });
}

// ── Secrets ──────────────────────────────────────────────────────

export function useAppSecrets(appId: string) {
  return useQuery({
    queryKey: ["apps", appId, "secrets"] as const,
    queryFn: () => api.get<string[]>(`/api/v1/apps/${appId}/secrets`),
  });
}

export function useUpdateSecrets(appId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (secrets: Record<string, string>) =>
      api.put<string[]>(`/api/v1/apps/${appId}/secrets`, secrets),
    onSuccess: () => {
      toast.success("Secrets saved");
      qc.invalidateQueries({ queryKey: ["apps", appId, "secrets"] });
    },
    onError: (err: any) => toast.error(err?.detail || "Failed to save secrets"),
  });
}

export function useDeleteApp(appId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.delete(`/api/v1/apps/${appId}`),
    onSuccess: () => {
      toast.success("Application deleted");
      qc.invalidateQueries({ queryKey: ["projects"], type: "active" });
      qc.removeQueries({ queryKey: ["apps", appId] });
    },
    onError: (err: any) => toast.error(err?.detail || "Failed to delete"),
  });
}
