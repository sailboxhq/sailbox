import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api } from "@/lib/api";
import type { UserInfo } from "@/types/api";

export const userKeys = {
  me: ["auth", "me"] as const,
};

export function useCurrentUser() {
  return useQuery({
    queryKey: userKeys.me,
    queryFn: () => api.get<UserInfo>("/api/v1/auth/me"),
    staleTime: 5 * 60_000, // user info rarely changes
    retry: false,
  });
}

export function useUpdateProfile() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: {
      first_name?: string;
      last_name?: string;
      display_name?: string;
      avatar_url?: string;
    }) => api.patch<UserInfo>("/api/v1/auth/profile", data),
    onSuccess: () => {
      toast.success("Profile updated");
      qc.invalidateQueries({ queryKey: ["auth", "me"] });
    },
    onError: (err: any) => toast.error(err?.detail || "Failed to update profile"),
  });
}

export function useChangePassword() {
  return useMutation({
    mutationFn: (data: { current_password: string; new_password: string }) =>
      api.post("/api/v1/auth/change-password", data),
    onSuccess: () => toast.success("Password changed"),
    onError: (err: any) => toast.error(err?.detail || "Failed to change password"),
  });
}

export function useAvatars() {
  return useQuery({
    queryKey: ["auth", "avatars"],
    queryFn: () => api.get<string[]>("/api/v1/auth/avatars"),
  });
}

export function useSetup2FA() {
  return useMutation({
    mutationFn: () => api.post<{ secret: string; qr_code: string }>("/api/v1/auth/2fa/setup"),
  });
}

export function useVerify2FA() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (code: string) => api.post("/api/v1/auth/2fa/verify", { code }),
    onSuccess: () => {
      toast.success("2FA enabled");
      qc.invalidateQueries({ queryKey: ["auth", "me"] });
    },
    onError: (err: any) => toast.error(err?.detail || "Invalid code"),
  });
}

export function useSetupStatus() {
  return useQuery({
    queryKey: ["auth", "setup-status"],
    queryFn: () => api.get<{ initialized: boolean }>("/api/v1/auth/setup-status"),
  });
}

export function useDisable2FA() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (code: string) => api.post("/api/v1/auth/2fa/disable", { code }),
    onSuccess: () => {
      toast.success("2FA disabled");
      qc.invalidateQueries({ queryKey: ["auth", "me"] });
    },
    onError: (err: any) => toast.error(err?.detail || "Invalid code"),
  });
}
