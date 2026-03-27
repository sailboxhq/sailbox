import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api } from "@/lib/api";
import type { NotificationChannel, SMTPConfig } from "@/types/api";

export function useNotificationChannels() {
  return useQuery({
    queryKey: ["notifications", "channels"],
    queryFn: () => api.get<NotificationChannel[]>("/api/v1/notifications/channels"),
  });
}

export function useSaveChannel() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { type: string; enabled: boolean; config: Record<string, string> }) =>
      api.put("/api/v1/notifications/channels", data),
    onSuccess: () => {
      toast.success("Channel saved");
      qc.invalidateQueries({ queryKey: ["notifications"] });
    },
    onError: (err: any) => toast.error(err?.detail || "Failed"),
  });
}

export function useTestChannel() {
  return useMutation({
    mutationFn: (type: string) => api.post("/api/v1/notifications/test", { type }),
    onSuccess: () => toast.success("Test notification sent"),
    onError: (err: any) => toast.error(err?.detail || "Test failed"),
  });
}

export function useSMTPConfig() {
  return useQuery({
    queryKey: ["settings", "smtp"],
    queryFn: () => api.get<SMTPConfig>("/api/v1/settings/smtp"),
  });
}

export function useSaveSMTPConfig() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: SMTPConfig) => api.put("/api/v1/settings/smtp", data),
    onSuccess: () => {
      toast.success("SMTP settings saved");
      qc.invalidateQueries({ queryKey: ["settings", "smtp"] });
    },
    onError: (err: any) => toast.error(err?.detail || "Failed"),
  });
}

export function useTestSMTP() {
  return useMutation({
    mutationFn: () => api.post("/api/v1/settings/smtp/test"),
    onSuccess: () => toast.success("Test email sent"),
    onError: (err: any) => toast.error(err?.detail || "Test failed"),
  });
}
