import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api } from "@/lib/api";
import type { Invitation, TeamMember } from "@/types/api";

export const teamKeys = {
  members: ["team", "members"] as const,
  invitations: ["team", "invitations"] as const,
};

export function useTeamMembers() {
  return useQuery({
    queryKey: teamKeys.members,
    queryFn: () => api.get<TeamMember[]>("/api/v1/team/members"),
  });
}

export function useTeamInvitations() {
  return useQuery({
    queryKey: teamKeys.invitations,
    queryFn: () => api.get<Invitation[]>("/api/v1/team/invitations"),
  });
}

export function useInviteMember() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { email: string; role: string }) =>
      api.post<{ invitation: Invitation; invite_url: string }>("/api/v1/team/invitations", data),
    onSuccess: () => {
      toast.success("Invitation sent");
      qc.invalidateQueries({ queryKey: teamKeys.invitations });
    },
    onError: (err: any) => toast.error(err?.detail || "Failed to invite"),
  });
}

export function useCancelInvitation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete(`/api/v1/team/invitations/${id}`),
    onSuccess: () => {
      toast.success("Invitation cancelled");
      qc.invalidateQueries({ queryKey: teamKeys.invitations });
    },
    onError: (err: any) => toast.error(err?.detail || "Failed"),
  });
}

export function useUpdateMemberRole() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, role }: { id: string; role: string }) =>
      api.patch(`/api/v1/team/members/${id}/role`, { role }),
    onSuccess: () => {
      toast.success("Role updated");
      qc.invalidateQueries({ queryKey: teamKeys.members });
    },
    onError: (err: any) => toast.error(err?.detail || "Failed"),
  });
}

export function useRemoveMember() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete(`/api/v1/team/members/${id}`),
    onSuccess: () => {
      toast.success("Member removed");
      qc.invalidateQueries({ queryKey: teamKeys.members });
    },
    onError: (err: any) => toast.error(err?.detail || "Failed"),
  });
}
