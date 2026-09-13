import { createFileRoute, useNavigate, useSearch } from "@tanstack/react-router";
import { Loader2 } from "lucide-react";
import { useEffect, useState } from "react";
import { Logo } from "@/components/logo";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { api } from "@/lib/api";
import { getToken, setTokens } from "@/lib/auth";

export const Route = createFileRoute("/auth/invite")({
  component: InvitePage,
  validateSearch: (search: Record<string, unknown>) => ({
    token: (search.token as string) || "",
  }),
});

interface InviteInfo {
  email: string;
  role: string;
  expires_at: string;
}

function InvitePage() {
  const { token } = useSearch({ from: "/auth/invite" });
  const navigate = useNavigate();
  const [status, setStatus] = useState<
    "loading" | "form" | "existing" | "accepting" | "done" | "error"
  >("loading");
  const [invite, setInvite] = useState<InviteInfo | null>(null);
  const [error, setError] = useState("");
  const [password, setPassword] = useState("");
  const [displayName, setDisplayName] = useState("");

  useEffect(() => {
    if (!token) {
      setStatus("error");
      setError("Invalid invitation link.");
      return;
    }
    // Fetch invitation info
    api
      .get<InviteInfo>(`/api/v1/team/invitations/info?token=${encodeURIComponent(token)}`)
      .then((info) => {
        setInvite(info);
        // If already logged in, try accepting directly
        if (getToken()) {
          setStatus("existing");
        } else {
          setStatus("form");
        }
      })
      .catch((err) => {
        setStatus("error");
        setError(err?.detail || "Invitation not found or expired.");
      });
  }, [token]);

  async function handleAccept(e: React.FormEvent) {
    e.preventDefault();
    setStatus("accepting");
    setError("");
    try {
      const result = await api.post<{
        access_token: string;
        refresh_token: string;
      }>("/api/v1/team/invitations/accept-public", {
        token,
        password,
        display_name: displayName,
      });
      setTokens(result.access_token, result.refresh_token);
      setStatus("done");
      setTimeout(() => navigate({ to: "/dashboard" }), 1500);
    } catch (err: any) {
      setStatus("form");
      setError(err?.detail || "Failed to accept invitation.");
    }
  }

  async function handleAcceptExisting() {
    setStatus("accepting");
    try {
      // Joining an organization revokes the tokens issued for the previous one,
      // so the response carries replacements — swap them in before navigating or
      // the next request lands on a dead session.
      const result = await api.post<{
        access_token?: string;
        refresh_token?: string;
      }>("/api/v1/team/invitations/accept", { token });
      if (result?.access_token && result?.refresh_token) {
        setTokens(result.access_token, result.refresh_token);
      }
      setStatus("done");
      setTimeout(() => navigate({ to: "/dashboard" }), 1500);
    } catch (err: any) {
      setStatus("error");
      setError(err?.detail || "Failed to accept invitation.");
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <Card className="w-full max-w-md">
        <CardHeader className="flex flex-col items-center text-center">
          <Logo className="mb-2 h-10 w-10 text-primary" />
          <CardTitle className="text-2xl">Team Invitation</CardTitle>
          <CardDescription>
            {status === "loading" && "Loading invitation..."}
            {status === "form" && "Set up your account to join the team."}
            {status === "existing" && "Accept the invitation to join the team."}
            {status === "accepting" && "Accepting invitation..."}
            {status === "done" && "Welcome! Redirecting..."}
            {status === "error" && "Something went wrong"}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {error && status !== "loading" && (
            <div className="mb-4 rounded-md bg-destructive/10 p-3 text-sm text-destructive">
              {error}
            </div>
          )}

          {status === "loading" && (
            <div className="flex justify-center py-8">
              <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
            </div>
          )}

          {status === "form" && invite && (
            <form onSubmit={handleAccept} className="space-y-4">
              <div className="space-y-2">
                <Label>Email</Label>
                <Input value={invite.email} disabled className="bg-muted" />
              </div>
              <div className="space-y-2">
                <Label>Role</Label>
                <Input value={invite.role} disabled className="bg-muted capitalize" />
              </div>
              <div className="space-y-2">
                <Label>Display Name</Label>
                <Input
                  value={displayName}
                  onChange={(e) => setDisplayName(e.target.value)}
                  placeholder="Your name"
                  required
                  autoFocus
                />
              </div>
              <div className="space-y-2">
                <Label>Password</Label>
                <Input
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="At least 8 characters"
                  required
                  minLength={8}
                />
              </div>
              <Button type="submit" className="w-full">
                Create Account & Join
              </Button>
            </form>
          )}

          {status === "existing" && invite && (
            <div className="space-y-4">
              <p className="text-sm text-muted-foreground">
                You're invited as <strong>{invite.role}</strong> ({invite.email}).
              </p>
              <Button className="w-full" onClick={handleAcceptExisting}>
                Accept Invitation
              </Button>
            </div>
          )}

          {status === "accepting" && (
            <div className="flex justify-center py-8">
              <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
            </div>
          )}

          {status === "done" && (
            <p className="py-4 text-center text-sm text-green-600">
              Invitation accepted! Redirecting...
            </p>
          )}

          {status === "error" && (
            <Button
              variant="outline"
              className="mt-2 w-full"
              onClick={() => navigate({ to: "/auth/login" })}
            >
              Go to Login
            </Button>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
