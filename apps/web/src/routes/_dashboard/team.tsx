import { createFileRoute } from "@tanstack/react-router";
import { Check, Clock, Copy, Mail, Trash2, UserPlus, Users as UsersIcon, X } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { LoadingScreen } from "@/components/loading-screen";
import type { BadgeProps } from "@/components/ui/badge";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { useCurrentUser } from "@/hooks/use-auth";
import {
  useCancelInvitation,
  useInviteMember,
  useRemoveMember,
  useTeamInvitations,
  useTeamMembers,
  useUpdateMemberRole,
} from "@/hooks/use-team";
import type { Invitation, TeamMember } from "@/types/api";

export const Route = createFileRoute("/_dashboard/team")({
  component: TeamPage,
});

const AVATAR_EMOJI: Record<string, string> = {
  bear: "\u{1F43B}",
  cat: "\u{1F431}",
  dog: "\u{1F436}",
  fox: "\u{1F98A}",
  koala: "\u{1F428}",
  lion: "\u{1F981}",
  monkey: "\u{1F435}",
  owl: "\u{1F989}",
  panda: "\u{1F43C}",
  penguin: "\u{1F427}",
  rabbit: "\u{1F430}",
  tiger: "\u{1F42F}",
  whale: "\u{1F433}",
  wolf: "\u{1F43A}",
};

function roleVariant(role: string): NonNullable<BadgeProps["variant"]> {
  switch (role.toLowerCase()) {
    case "owner":
      return "default";
    case "admin":
      return "outline";
    default:
      return "secondary";
  }
}

function relativeExpiry(expiresAt: string): string {
  const now = Date.now();
  const expires = new Date(expiresAt).getTime();
  const diffMs = expires - now;
  if (diffMs <= 0) return "Expired";
  const days = Math.floor(diffMs / (1000 * 60 * 60 * 24));
  const hours = Math.floor((diffMs % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60));
  if (days > 0) return `Expires in ${days}d`;
  if (hours > 0) return `Expires in ${hours}h`;
  return "Expires soon";
}

function TeamPage() {
  const { data: user, isLoading: userLoading } = useCurrentUser();
  const { data: members, isLoading: membersLoading } = useTeamMembers();
  const { data: invitations, isLoading: invitationsLoading } = useTeamInvitations();

  if (userLoading || membersLoading) return <LoadingScreen />;
  if (!user) return null;

  const isOwner = user.role?.toLowerCase() === "owner";
  const pendingInvitations = (invitations ?? []).filter((inv) => !inv.accepted_at);

  return (
    <div>
      <h1 className="text-2xl font-bold tracking-tight">Team</h1>
      <p className="text-sm text-muted-foreground">Manage your team members and invitations</p>

      <Separator className="my-6" />

      <div className="space-y-6">
        {/* Members */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle className="flex items-center gap-2 text-sm font-medium">
              <UsersIcon className="h-4 w-4" /> Members
            </CardTitle>
            {isOwner && <InviteDialog />}
          </CardHeader>
          <CardContent>
            {!members || members.length === 0 ? (
              <div className="flex flex-col items-center gap-3 py-8 text-center">
                <div className="flex h-10 w-10 items-center justify-center rounded-full bg-muted">
                  <UsersIcon className="h-5 w-5 text-muted-foreground" />
                </div>
                <p className="text-sm text-muted-foreground">No team members found</p>
              </div>
            ) : (
              <div className="space-y-1">
                {members.map((member) => (
                  <MemberRow
                    key={member.id}
                    member={member}
                    isOwner={isOwner}
                    isCurrentUser={member.id === user.id}
                  />
                ))}
              </div>
            )}
            {!isOwner && (
              <p className="mt-4 text-xs text-muted-foreground">
                Contact the owner to manage team members.
              </p>
            )}
          </CardContent>
        </Card>

        {/* Pending Invitations */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-sm font-medium">
              <Mail className="h-4 w-4" /> Pending Invitations
            </CardTitle>
          </CardHeader>
          <CardContent>
            {invitationsLoading ? (
              <LoadingScreen />
            ) : pendingInvitations.length === 0 ? (
              <div className="flex flex-col items-center gap-3 py-8 text-center">
                <div className="flex h-10 w-10 items-center justify-center rounded-full bg-muted">
                  <Mail className="h-5 w-5 text-muted-foreground" />
                </div>
                <p className="text-sm text-muted-foreground">No pending invitations</p>
              </div>
            ) : (
              <div className="space-y-1">
                {pendingInvitations.map((inv) => (
                  <InvitationRow key={inv.id} invitation={inv} isOwner={isOwner} />
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

// ── Member Row ─────────────────────────────────────────────────────

function MemberRow({
  member,
  isOwner,
  isCurrentUser,
}: {
  member: TeamMember;
  isOwner: boolean;
  isCurrentUser: boolean;
}) {
  const updateRole = useUpdateMemberRole();
  const removeMember = useRemoveMember();
  const [removeOpen, setRemoveOpen] = useState(false);
  const [removeConfirm, setRemoveConfirm] = useState("");

  const isMemberOwner = member.role.toLowerCase() === "owner";
  const showActions = isOwner && !isMemberOwner && !isCurrentUser;

  const handleRoleChange = (newRole: string) => {
    updateRole.mutate({ id: member.id, role: newRole });
  };

  const handleRemove = () => {
    if (removeConfirm !== "REMOVE") return;
    removeMember.mutate(member.id, {
      onSuccess: () => {
        setRemoveOpen(false);
        setRemoveConfirm("");
      },
    });
  };

  return (
    <div className="flex items-center gap-3 rounded-lg px-3 py-3 hover:bg-accent/50">
      {/* Avatar */}
      <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-primary/10 text-sm font-semibold text-primary">
        {member.avatar_url && AVATAR_EMOJI[member.avatar_url] ? (
          <span className="text-lg leading-none">{AVATAR_EMOJI[member.avatar_url]}</span>
        ) : (
          <span>{member.display_name?.[0]?.toUpperCase() || member.email[0]?.toUpperCase()}</span>
        )}
      </div>

      {/* Name + email */}
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium leading-tight">
          {member.display_name || `${member.first_name} ${member.last_name}`.trim() || member.email}
        </p>
        <p className="truncate text-xs text-muted-foreground">{member.email}</p>
      </div>

      {/* Role badge */}
      <Badge variant={roleVariant(member.role)} className="shrink-0 capitalize">
        {member.role}
      </Badge>

      {/* Actions */}
      {showActions && (
        <div className="flex shrink-0 items-center gap-1">
          <Select value={member.role.toLowerCase()} onValueChange={handleRoleChange}>
            <SelectTrigger className="h-7 w-[100px] text-xs">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="admin">Admin</SelectItem>
              <SelectItem value="member">Member</SelectItem>
            </SelectContent>
          </Select>

          {/* Remove dialog */}
          <Dialog
            open={removeOpen}
            onOpenChange={(open) => {
              setRemoveOpen(open);
              if (!open) setRemoveConfirm("");
            }}
          >
            <DialogTrigger asChild>
              <Button
                variant="ghost"
                size="sm"
                className="h-7 w-7 p-0 text-muted-foreground hover:text-destructive"
              >
                <Trash2 className="h-3.5 w-3.5" />
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Remove Member</DialogTitle>
                <DialogDescription>
                  Are you sure you want to remove{" "}
                  <strong>{member.display_name || member.email}</strong> from the team? Type{" "}
                  <strong>REMOVE</strong> to confirm.
                </DialogDescription>
              </DialogHeader>
              <div className="space-y-2">
                <Label>Confirmation</Label>
                <Input
                  value={removeConfirm}
                  onChange={(e) => setRemoveConfirm(e.target.value)}
                  placeholder="Type REMOVE"
                  className="font-mono"
                />
              </div>
              <DialogFooter>
                <DialogClose asChild>
                  <Button variant="outline">Cancel</Button>
                </DialogClose>
                <Button
                  variant="destructive"
                  onClick={handleRemove}
                  disabled={removeConfirm !== "REMOVE" || removeMember.isPending}
                >
                  {removeMember.isPending ? "Removing..." : "Remove"}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </div>
      )}
    </div>
  );
}

// ── Invitation Row ─────────────────────────────────────────────────

function InvitationRow({ invitation, isOwner }: { invitation: Invitation; isOwner: boolean }) {
  const cancelInvitation = useCancelInvitation();

  return (
    <div className="flex items-center gap-3 rounded-lg px-3 py-3 hover:bg-accent/50">
      <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-muted">
        <Mail className="h-4 w-4 text-muted-foreground" />
      </div>

      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium leading-tight">{invitation.email}</p>
      </div>

      <Badge variant={roleVariant(invitation.role)} className="shrink-0 capitalize">
        {invitation.role}
      </Badge>

      <span className="flex shrink-0 items-center gap-1 text-xs text-muted-foreground">
        <Clock className="h-3 w-3" />
        {relativeExpiry(invitation.expires_at)}
      </span>

      {isOwner && (
        <Button
          variant="ghost"
          size="sm"
          className="h-7 shrink-0 text-xs text-muted-foreground hover:text-destructive"
          onClick={() => cancelInvitation.mutate(invitation.id)}
          disabled={cancelInvitation.isPending}
        >
          <X className="mr-1 h-3 w-3" />
          Cancel
        </Button>
      )}
    </div>
  );
}

// ── Invite Dialog ──────────────────────────────────────────────────

function InviteDialog() {
  const [open, setOpen] = useState(false);
  const [email, setEmail] = useState("");
  const [role, setRole] = useState("member");
  const [inviteUrl, setInviteUrl] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const inviteMember = useInviteMember();

  const handleInvite = () => {
    inviteMember.mutate(
      { email, role },
      {
        onSuccess: (data) => {
          setInviteUrl(data.invite_url);
        },
      },
    );
  };

  const handleCopy = () => {
    if (!inviteUrl) return;
    navigator.clipboard.writeText(inviteUrl);
    setCopied(true);
    toast.success("Invite URL copied");
    setTimeout(() => setCopied(false), 2000);
  };

  const handleClose = (isOpen: boolean) => {
    setOpen(isOpen);
    if (!isOpen) {
      setEmail("");
      setRole("member");
      setInviteUrl(null);
      setCopied(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogTrigger asChild>
        <Button size="sm">
          <UserPlus className="mr-1 h-3.5 w-3.5" />
          Invite Member
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Invite Member</DialogTitle>
          <DialogDescription>Send an invitation to join your team.</DialogDescription>
        </DialogHeader>

        {!inviteUrl ? (
          <>
            <div className="space-y-4">
              <div className="space-y-2">
                <Label>Email</Label>
                <Input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="teammate@example.com"
                />
              </div>
              <div className="space-y-2">
                <Label>Role</Label>
                <Select value={role} onValueChange={setRole}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="admin">Admin</SelectItem>
                    <SelectItem value="member">Member</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
            <DialogFooter>
              <DialogClose asChild>
                <Button variant="outline">Cancel</Button>
              </DialogClose>
              <Button onClick={handleInvite} disabled={!email || inviteMember.isPending}>
                {inviteMember.isPending ? "Sending..." : "Send Invitation"}
              </Button>
            </DialogFooter>
          </>
        ) : (
          <>
            <div className="space-y-2">
              <Label>Invite URL</Label>
              <p className="text-xs text-muted-foreground">
                Share this link with the invitee. It will expire in 7 days.
              </p>
              <div className="flex items-center gap-2">
                <Input value={inviteUrl} readOnly className="font-mono text-xs" />
                <Button variant="outline" size="sm" onClick={handleCopy}>
                  {copied ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
                </Button>
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => handleClose(false)}>
                Done
              </Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
