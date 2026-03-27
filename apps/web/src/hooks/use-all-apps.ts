import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { App, PaginatedResponse } from "@/types/api";

export function useAllApps(page = 1, perPage = 20, search?: string, status?: string) {
  const params = new URLSearchParams({ page: String(page), per_page: String(perPage) });
  if (search) params.set("search", search);
  if (status) params.set("status", status);

  return useQuery({
    queryKey: ["apps", "all", page, perPage, search, status],
    queryFn: () => api.get<PaginatedResponse<App>>(`/api/v1/apps?${params}`),
  });
}
