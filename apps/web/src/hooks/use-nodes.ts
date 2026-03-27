import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api } from "@/lib/api";
import type { ServerNode } from "@/types/api";

export const nodeKeys = {
  all: ["nodes"] as const,
  detail: (id: string) => ["nodes", id] as const,
};

export function useNodes() {
  return useQuery({
    queryKey: nodeKeys.all,
    queryFn: () => api.get<ServerNode[]>("/api/v1/nodes"),
  });
}

export function useCreateNode() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: {
      name: string;
      host: string;
      port?: number;
      ssh_user?: string;
      auth_type: string;
      ssh_key_id?: string;
      password?: string;
      role?: string;
    }) => api.post<ServerNode>("/api/v1/nodes", data),
    onSuccess: (_, vars) => {
      toast.success(`Node "${vars.name}" added`);
      qc.invalidateQueries({ queryKey: nodeKeys.all });
    },
    onError: (err: any) => toast.error(err?.detail || "Failed to add node"),
  });
}

export function useInitializeNode() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.post(`/api/v1/nodes/${id}/initialize`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: nodeKeys.all });
    },
    onError: (err: any) => toast.error(err?.detail || "Failed to initialize"),
  });
}

export function useDeleteNode() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete(`/api/v1/nodes/${id}`),
    onSuccess: () => {
      toast.success("Node removed");
      qc.invalidateQueries({ queryKey: nodeKeys.all });
    },
    onError: (err: any) => toast.error(err?.detail || "Failed to remove"),
  });
}
