import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api } from "@/lib/api";
import type { Settings } from "@/types/api";

export const settingsKeys = {
  all: ["settings"] as const,
};

export function useSettings() {
  return useQuery({
    queryKey: settingsKeys.all,
    queryFn: () => api.get<Settings>("/api/v1/settings"),
  });
}

export function useUpdateSetting() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { key: string; value: string }) => api.put("/api/v1/settings", data),
    onSuccess: () => {
      toast.success("Setting updated");
      qc.invalidateQueries({ queryKey: settingsKeys.all });
    },
    onError: (err: any) => toast.error(err?.detail || "Failed to save"),
  });
}
