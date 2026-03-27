import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api } from "@/lib/api";
import type {
  PaginatedResponse,
  S3BackupFile,
  SystemBackup,
  SystemBackupConfig,
} from "@/types/api";

export function useSystemBackupConfig() {
  return useQuery({
    queryKey: ["system", "backup", "config"],
    queryFn: () => api.get<SystemBackupConfig>("/api/v1/system/backup/config"),
  });
}

export function useSaveSystemBackupConfig() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: SystemBackupConfig) => api.put("/api/v1/system/backup/config", data),
    onSuccess: () => {
      toast.success("Backup configuration saved");
      qc.invalidateQueries({ queryKey: ["system", "backup"] });
    },
    onError: (err: any) => toast.error(err?.detail || "Failed"),
  });
}

export function useSystemBackups() {
  return useQuery({
    queryKey: ["system", "backup", "list"],
    queryFn: () => api.get<PaginatedResponse<SystemBackup>>("/api/v1/system/backup/list"),
    select: (data) => data.items ?? [],
    refetchInterval: (query) => {
      const items = Array.isArray(query.state.data) ? query.state.data : [];
      if (items.some((b: any) => b.status === "pending" || b.status === "running")) return 5000;
      return false;
    },
  });
}

export function useTriggerSystemBackup() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.post<SystemBackup>("/api/v1/system/backup/trigger"),
    onSuccess: () => {
      toast.success("Backup started");
      qc.invalidateQueries({ queryKey: ["system", "backup"] });
    },
    onError: (err: any) => toast.error(err?.detail || "Backup failed"),
  });
}

export function useScanS3Backups() {
  return useMutation({
    mutationFn: (data: {
      endpoint: string;
      bucket: string;
      access_key: string;
      secret_key: string;
      path: string;
    }) => api.post<S3BackupFile[]>("/api/v1/system/restore/scan", data),
  });
}

export function useRestoreFromS3() {
  return useMutation({
    mutationFn: (data: {
      endpoint: string;
      bucket: string;
      access_key: string;
      secret_key: string;
      s3_key: string;
    }) => api.post("/api/v1/system/restore/execute", data),
  });
}
