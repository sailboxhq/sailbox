import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api } from "@/lib/api";
import type {
  AppStatus,
  DatabaseBackup,
  DatabaseCredentials,
  DBVersionInfo,
  ManagedDB,
  PaginatedResponse,
  PodInfo,
} from "@/types/api";
import { projectKeys } from "./use-projects";

export const dbKeys = {
  detail: (id: string) => ["databases", id] as const,
};

export function useProjectDatabases(projectId: string) {
  return useQuery({
    queryKey: projectKeys.databases(projectId),
    queryFn: () => api.get<PaginatedResponse<ManagedDB>>(`/api/v1/projects/${projectId}/databases`),
    select: (data) => data.items ?? [],
  });
}

export function useDatabase(dbId: string) {
  return useQuery({
    queryKey: dbKeys.detail(dbId),
    queryFn: () => api.get<ManagedDB>(`/api/v1/databases/${dbId}`),
  });
}

export function useCreateDatabase(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { name: string; engine: string; version: string; storage_size: string }) =>
      api.post<ManagedDB>("/api/v1/databases", {
        ...data,
        project_id: projectId,
      }),
    onSuccess: (_, vars) => {
      toast.success(`Database "${vars.name}" created`);
      qc.invalidateQueries({ queryKey: projectKeys.databases(projectId) });
    },
    onError: (err: any) => toast.error(err?.detail || "Failed to create database"),
  });
}

export function useDeleteDatabase(dbId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.delete(`/api/v1/databases/${dbId}`),
    onSuccess: () => {
      toast.success("Database deleted");
      qc.invalidateQueries({ queryKey: ["projects"], type: "active" });
      qc.removeQueries({ queryKey: dbKeys.detail(dbId) });
    },
    onError: (err: any) => toast.error(err?.detail || "Failed to delete"),
  });
}

export function useDatabaseVersions(engine?: string) {
  return useQuery({
    queryKey: ["databases", "versions", engine],
    queryFn: () => {
      const params = engine ? `?engine=${engine}` : "";
      return api.get<DBVersionInfo[]>(`/api/v1/databases/versions${params}`);
    },
  });
}

export function useDatabaseStatus(dbId: string) {
  return useQuery({
    queryKey: ["databases", dbId, "status"],
    queryFn: () => api.get<AppStatus>(`/api/v1/databases/${dbId}/status`),
    enabled: !!dbId,
    refetchInterval: (query) => {
      const phase = query.state.data?.phase;
      if (!phase) return 3_000;
      const stable = ["running", "stopped", "error", "not deployed"];
      return stable.includes(phase) ? 30_000 : 3_000;
    },
  });
}

export function useDatabaseCredentials(dbId: string, ready = true) {
  return useQuery({
    queryKey: ["databases", dbId, "credentials"],
    queryFn: () => api.get<DatabaseCredentials>(`/api/v1/databases/${dbId}/credentials`),
    enabled: !!dbId && ready,
  });
}

export function useDatabasePods(dbId: string) {
  return useQuery({
    queryKey: ["databases", dbId, "pods"],
    queryFn: () => api.get<PodInfo[]>(`/api/v1/databases/${dbId}/pods`),
    enabled: !!dbId,
    refetchInterval: 30_000,
  });
}

export function useDatabaseBackups(dbId: string) {
  return useQuery({
    queryKey: ["databases", dbId, "backups"],
    queryFn: () => api.get<PaginatedResponse<DatabaseBackup>>(`/api/v1/databases/${dbId}/backups`),
    enabled: !!dbId,
    select: (data) => data.items ?? [],
    refetchInterval: (query) => {
      const items = (query.state.data as any)?.items ?? query.state.data;
      if (
        Array.isArray(items) &&
        items.some(
          (b: any) =>
            b.status === "pending" || b.status === "running" || b.restore_status === "running",
        )
      )
        return 5_000;
      return false;
    },
  });
}

export function useTriggerBackup(dbId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.post<DatabaseBackup>(`/api/v1/databases/${dbId}/backups`),
    onSuccess: () => {
      toast.success("Backup started");
      qc.invalidateQueries({ queryKey: ["databases", dbId, "backups"] });
    },
    onError: (err: any) => toast.error(err?.detail || "Backup failed"),
  });
}

export function useRestoreBackup(dbId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (backupId: string) =>
      api.post(`/api/v1/databases/${dbId}/backups/${backupId}/restore`),
    onSuccess: () => {
      toast.success("Restore started");
      qc.invalidateQueries({ queryKey: ["databases", dbId, "backups"] });
    },
    onError: (err: any) => toast.error(err?.detail || "Restore failed"),
  });
}

export function useUsedPorts() {
  return useQuery({
    queryKey: ["databases", "used-ports"],
    queryFn: () =>
      api.get<{ database_id: string; database_name: string; engine: string; port: number }[]>(
        "/api/v1/databases/used-ports",
      ),
  });
}

export function useUpdateExternalAccess(dbId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { enabled: boolean; port?: number }) =>
      api.post(`/api/v1/databases/${dbId}/external-access`, data),
    onSuccess: () => {
      toast.success("External access updated");
      qc.invalidateQueries({ queryKey: ["databases"] });
    },
    onError: (err: any) => toast.error(err?.detail || "Failed"),
  });
}

export function useUpdateBackupConfig(dbId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { enabled: boolean; schedule: string; s3_id?: string }) =>
      api.put(`/api/v1/databases/${dbId}/backup-config`, data),
    onSuccess: () => {
      toast.success("Backup configuration saved");
      qc.invalidateQueries({ queryKey: ["databases"] });
    },
    onError: (err: any) => toast.error(err?.detail || "Failed"),
  });
}
