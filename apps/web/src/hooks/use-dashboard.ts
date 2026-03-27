import { useQuery } from "@tanstack/react-query";
import { useProjects } from "@/hooks/use-projects";
import { api } from "@/lib/api";
import type { App, ManagedDB } from "@/types/api";

export function useDashboardApps() {
  const { data: projects } = useProjects();
  const projectIds =
    projects
      ?.map((p) => p.id)
      .sort()
      .join(",") ?? "";
  return useQuery({
    queryKey: ["dashboard", "apps", projectIds],
    queryFn: async () => {
      if (!projects?.length) return [];
      const results = await Promise.all(
        projects.map((p) =>
          api
            .get<{ items: App[] }>(`/api/v1/projects/${p.id}/apps`)
            .then((r) => r.items ?? [])
            .catch(() => []),
        ),
      );
      return results.flat();
    },
    enabled: !!projects?.length,
  });
}

export function useDashboardDatabases() {
  const { data: projects } = useProjects();
  const projectIds =
    projects
      ?.map((p) => p.id)
      .sort()
      .join(",") ?? "";
  return useQuery({
    queryKey: ["dashboard", "databases", projectIds],
    queryFn: async () => {
      if (!projects?.length) return [];
      const results = await Promise.all(
        projects.map((p) =>
          api
            .get<{ items: ManagedDB[] }>(`/api/v1/projects/${p.id}/databases`)
            .then((r) => r.items ?? [])
            .catch(() => []),
        ),
      );
      return results.flat();
    },
    enabled: !!projects?.length,
  });
}
