import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api } from "@/lib/api";
import type { MetricAlert, MetricEvent, MetricSnapshot, PaginatedResponse } from "@/types/api";

export const monitoringKeys = {
  snapshots: (params?: Record<string, string>) => ["monitoring", "snapshots", params] as const,
  events: (page?: number) => ["monitoring", "events", page] as const,
  alerts: (params?: Record<string, string>) => ["monitoring", "alerts", params] as const,
  activeAlerts: () => ["monitoring", "alerts", "active"] as const,
};

export function useMonitoringSnapshots(sourceType: string, sourceName?: string, rangeMinutes = 60) {
  const key = [sourceType, sourceName ?? "", rangeMinutes] as const;

  return useQuery({
    queryKey: ["monitoring", "snapshots", ...key],
    queryFn: () => {
      const from = new Date(Date.now() - rangeMinutes * 60 * 1000).toISOString();
      const p = new URLSearchParams({ source_type: sourceType });
      if (sourceName) p.set("source_name", sourceName);
      p.set("from", from);
      return api.get<MetricSnapshot[]>(`/api/v1/monitoring/snapshots?${p}`);
    },
    refetchInterval: 30_000,
  });
}

export function useMonitoringEvents(page = 1) {
  return useQuery({
    queryKey: monitoringKeys.events(page),
    queryFn: () =>
      api.get<PaginatedResponse<MetricEvent>>(`/api/v1/monitoring/events?page=${page}&per_page=50`),
    refetchInterval: 30_000,
  });
}

export function useMonitoringAlerts(activeOnly = false) {
  const params: Record<string, string> = {};
  if (activeOnly) params.active = "true";
  const search = new URLSearchParams(params);

  return useQuery({
    queryKey: monitoringKeys.alerts(params),
    queryFn: () => api.get<PaginatedResponse<MetricAlert>>(`/api/v1/monitoring/alerts?${search}`),
    refetchInterval: 60_000, // SSE handles real-time push; this is fallback
  });
}

export function useActiveAlerts() {
  return useQuery({
    queryKey: monitoringKeys.activeAlerts(),
    queryFn: () =>
      api.get<{ count: number; alerts: MetricAlert[] }>("/api/v1/monitoring/alerts/active"),
    refetchInterval: 60_000,
  });
}

export function useResolveAlert() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (alertId: string) => api.post(`/api/v1/monitoring/alerts/${alertId}/resolve`),
    onSuccess: () => {
      toast.success("Alert resolved");
      qc.invalidateQueries({ queryKey: ["monitoring"] });
    },
    onError: (err: any) => toast.error(err?.detail || "Failed to resolve alert"),
  });
}
