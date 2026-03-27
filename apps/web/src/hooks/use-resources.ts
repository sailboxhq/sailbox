import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api } from "@/lib/api";
import type { SharedResource } from "@/types/api";

export const resourceKeys = {
  all: ["resources"] as const,
  byType: (type: string) => ["resources", type] as const,
};

export function useResources(type?: string) {
  return useQuery({
    queryKey: type ? resourceKeys.byType(type) : resourceKeys.all,
    queryFn: () => api.get<SharedResource[]>(`/api/v1/resources${type ? `?type=${type}` : ""}`),
  });
}

export function useCreateResource() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: {
      name: string;
      type: string;
      provider: string;
      config: Record<string, string>;
    }) => api.post<SharedResource>("/api/v1/resources", data),
    onSuccess: (data, vars) => {
      toast.success(`"${data.name || vars.name}" created`);
      qc.invalidateQueries({ queryKey: ["resources"] });
    },
    onError: (err: any) => toast.error(err?.detail || "Failed"),
  });
}

export function useUpdateResource() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      ...data
    }: {
      id: string;
      name?: string;
      provider?: string;
      config?: Record<string, string>;
    }) => api.patch<SharedResource>(`/api/v1/resources/${id}`, data),
    onSuccess: () => {
      toast.success("Updated");
      qc.invalidateQueries({ queryKey: ["resources"] });
    },
    onError: (err: any) => toast.error(err?.detail || "Failed"),
  });
}

export function useDeleteResource() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete(`/api/v1/resources/${id}`),
    onSuccess: () => {
      toast.success("Deleted");
      qc.invalidateQueries({ queryKey: ["resources"] });
    },
    onError: (err: any) => toast.error(err?.detail || "Failed"),
  });
}

export function useGenerateSSHKey() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { algorithm: string; name?: string }) =>
      api.post<SharedResource>("/api/v1/resources/generate-ssh-key", data),
    onSuccess: (data) => {
      toast.success(`SSH key "${data.name}" generated`);
      qc.invalidateQueries({ queryKey: ["resources"] });
    },
    onError: (err: any) => toast.error(err?.detail || "Generation failed"),
  });
}

export interface GitRepo {
  name: string;
  full_name: string;
  clone_url: string;
  default_branch: string;
  private: boolean;
}

export function useGitRepos(resourceId: string) {
  return useQuery({
    queryKey: ["resources", resourceId, "repos"] as const,
    queryFn: () => api.get<GitRepo[]>(`/api/v1/resources/${resourceId}/repos`),
    enabled: !!resourceId,
  });
}

export function useGitHubStatus() {
  return useQuery({
    queryKey: ["github", "status"] as const,
    queryFn: () =>
      api.get<{ configured: boolean; app_name: string; install_url: string }>(
        "/api/v1/auth/github/status",
      ),
  });
}

export function useTestResource() {
  return useMutation({
    mutationFn: (id: string) =>
      api.post<{ success: boolean; message: string }>(`/api/v1/resources/${id}/test`),
    onSuccess: (data) => {
      if (data.success) {
        toast.success(data.message);
      } else {
        toast.error(data.message);
      }
    },
    onError: (err: any) => toast.error(err?.detail || "Test failed"),
  });
}
