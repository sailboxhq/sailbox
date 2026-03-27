import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api } from "@/lib/api";
import type { PaginatedResponse, Project } from "@/types/api";

export const projectKeys = {
  all: ["projects"] as const,
  detail: (id: string) => ["projects", id] as const,
  apps: (id: string) => ["projects", id, "apps"] as const,
  databases: (id: string) => ["projects", id, "databases"] as const,
};

export function useProjects() {
  return useQuery({
    queryKey: projectKeys.all,
    queryFn: () => api.get<PaginatedResponse<Project>>("/api/v1/projects"),
    select: (data) => data.items ?? [],
  });
}

export function useProject(id: string) {
  return useQuery({
    queryKey: projectKeys.detail(id),
    queryFn: () => api.get<Project>(`/api/v1/projects/${id}`),
  });
}

export function useCreateProject() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { name: string; description: string }) =>
      api.post<Project>("/api/v1/projects", data),
    onSuccess: (_, vars) => {
      toast.success(`Project "${vars.name}" created`);
      qc.invalidateQueries({ queryKey: projectKeys.all });
    },
    onError: (err: any) => toast.error(err?.detail || "Failed to create project"),
  });
}

export function useUpdateProject(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { name: string; description: string }) =>
      api.patch<Project>(`/api/v1/projects/${id}`, data),
    onSuccess: () => {
      toast.success("Project updated");
      qc.invalidateQueries({ queryKey: projectKeys.detail(id) });
    },
    onError: (err: any) => toast.error(err?.detail || "Failed to update"),
  });
}

export function useDeleteProject(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.delete(`/api/v1/projects/${id}`),
    onSuccess: () => {
      toast.success("Project deleted");
      qc.invalidateQueries({ queryKey: projectKeys.all });
    },
    onError: (err: any) => toast.error(err?.detail || "Failed to delete"),
  });
}

export function useUpdateProjectEnv(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (envVars: Record<string, string>) =>
      api.put(`/api/v1/projects/${id}/env`, { env_vars: envVars }),
    onSuccess: () => {
      toast.success("Environment saved");
      qc.invalidateQueries({ queryKey: ["projects", id] });
    },
    onError: (err: any) => toast.error(err?.detail || "Save failed"),
  });
}
