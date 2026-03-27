import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { VersionInfo } from "@/types/api";

export function useVersion() {
  return useQuery({
    queryKey: ["version"],
    queryFn: () => api.get<VersionInfo>("/api/v1/version"),
    refetchInterval: 60 * 60 * 1000, // 1 hour
    staleTime: 60 * 60 * 1000,
  });
}
