import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { Loader2 } from "lucide-react";
import { useState } from "react";
import { Logo } from "@/components/logo";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { useSetupStatus } from "@/hooks/use-auth";
import { useRestoreFromS3, useScanS3Backups } from "@/hooks/use-system-backup";
import { login, register } from "@/lib/auth";
import type { S3BackupFile } from "@/types/api";

export const Route = createFileRoute("/auth/login")({ component: AuthPage });

function AuthPage() {
  const { data: setup, isLoading: setupLoading } = useSetupStatus();
  const [showRestore, setShowRestore] = useState(false);

  // Show skeleton while checking setup status
  if (setupLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background">
        <Card className="w-full max-w-sm">
          <CardContent className="space-y-4 p-6">
            <Skeleton className="mx-auto h-10 w-10 rounded-full" />
            <Skeleton className="mx-auto h-6 w-32" />
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
          </CardContent>
        </Card>
      </div>
    );
  }

  if (setup && !setup.initialized) {
    return showRestore ? (
      <RestoreForm onBack={() => setShowRestore(false)} />
    ) : (
      <SetupForm onRestore={() => setShowRestore(true)} />
    );
  }

  return <LoginForm />;
}

// ── First-time setup (registration) ──────────────────────────────

function SetupForm({ onRestore }: { onRestore: () => void }) {
  const navigate = useNavigate();
  const [form, setForm] = useState({
    email: "",
    password: "",
    displayName: "",
    orgName: "Sailbox",
  });
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      await register(form.email, form.password, form.displayName, form.orgName);
      navigate({ to: "/dashboard" });
    } catch (err: any) {
      setError(err?.detail || err?.message || "Registration failed");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <Card className="w-full max-w-sm">
        <CardHeader className="flex flex-col items-center text-center">
          <Logo className="mb-2 h-10 w-10 text-primary" />
          <CardTitle className="text-2xl">Welcome to Sailbox</CardTitle>
          <CardDescription>Create your admin account to get started</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            {error && (
              <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
                {error}
              </div>
            )}
            <div className="space-y-2">
              <label htmlFor="name" className="text-sm font-medium">
                Display Name
              </label>
              <Input
                id="name"
                value={form.displayName}
                onChange={(e) => setForm({ ...form, displayName: e.target.value })}
                placeholder="Your name"
                required
              />
            </div>
            <div className="space-y-2">
              <label htmlFor="email" className="text-sm font-medium">
                Email
              </label>
              <Input
                id="email"
                type="email"
                value={form.email}
                onChange={(e) => setForm({ ...form, email: e.target.value })}
                required
              />
            </div>
            <div className="space-y-2">
              <label htmlFor="password" className="text-sm font-medium">
                Password
              </label>
              <Input
                id="password"
                type="password"
                value={form.password}
                onChange={(e) => setForm({ ...form, password: e.target.value })}
                placeholder="Minimum 8 characters"
                required
                minLength={8}
              />
            </div>
            <Button type="submit" className="w-full" disabled={loading}>
              {loading ? "Creating account..." : "Create Account"}
            </Button>
          </form>
          <div className="mt-6 text-center">
            <p className="text-xs text-muted-foreground">or</p>
            <button
              type="button"
              onClick={onRestore}
              className="mt-2 text-sm text-primary hover:underline"
            >
              Restore from backup
            </button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

// ── Restore from S3 backup ───────────────────────────────────────

function RestoreForm({ onBack }: { onBack: () => void }) {
  const scan = useScanS3Backups();
  const restore = useRestoreFromS3();

  const [s3, setS3] = useState({
    endpoint: "",
    bucket: "",
    access_key: "",
    secret_key: "",
    path: "sailbox-backups",
  });
  const [backups, setBackups] = useState<S3BackupFile[]>([]);
  const [selected, setSelected] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState("");
  const [step, setStep] = useState<"credentials" | "select" | "restoring" | "done">("credentials");

  async function handleScan(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    try {
      const files = await scan.mutateAsync(s3);
      if (!files || files.length === 0) {
        setError("No backup files found in the specified path");
        return;
      }
      setBackups(files);
      setStep("select");
    } catch (err: any) {
      setError(err?.detail || err?.message || "Failed to scan S3 bucket");
    }
  }

  async function handleRestore() {
    setError("");
    setStep("restoring");
    try {
      await restore.mutateAsync({
        endpoint: s3.endpoint,
        bucket: s3.bucket,
        access_key: s3.access_key,
        secret_key: s3.secret_key,
        s3_key: selected,
      });
      setStep("done");
      // Wait 3 seconds to show success, then redirect
      setTimeout(() => {
        window.location.href = "/auth/login";
      }, 3000);
    } catch (err: any) {
      setError(err?.detail || err?.message || "Restore failed");
      setStep("select");
    }
  }

  function formatSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <Card className="w-full max-w-md">
        <CardHeader className="flex flex-col items-center text-center">
          <Logo className="mb-2 h-10 w-10 text-primary" />
          <CardTitle className="text-2xl">Restore Sailbox</CardTitle>
          <CardDescription>Restore from a previous backup</CardDescription>
        </CardHeader>
        <CardContent>
          {error && (
            <div className="mb-4 rounded-md bg-destructive/10 p-3 text-sm text-destructive">
              {error}
            </div>
          )}

          {step === "credentials" && (
            <form onSubmit={handleScan} className="space-y-4">
              <div className="space-y-2">
                <label htmlFor="s3-endpoint" className="text-sm font-medium">
                  S3 Endpoint
                </label>
                <Input
                  id="s3-endpoint"
                  value={s3.endpoint}
                  onChange={(e) => setS3({ ...s3, endpoint: e.target.value })}
                  placeholder="https://s3.amazonaws.com"
                  required
                />
              </div>
              <div className="space-y-2">
                <label htmlFor="s3-bucket" className="text-sm font-medium">
                  Bucket
                </label>
                <Input
                  id="s3-bucket"
                  value={s3.bucket}
                  onChange={(e) => setS3({ ...s3, bucket: e.target.value })}
                  placeholder="my-backups"
                  required
                />
              </div>
              <div className="space-y-2">
                <label htmlFor="s3-access-key" className="text-sm font-medium">
                  Access Key
                </label>
                <Input
                  id="s3-access-key"
                  value={s3.access_key}
                  onChange={(e) => setS3({ ...s3, access_key: e.target.value })}
                  required
                />
              </div>
              <div className="space-y-2">
                <label htmlFor="s3-secret-key" className="text-sm font-medium">
                  Secret Key
                </label>
                <Input
                  id="s3-secret-key"
                  type="password"
                  value={s3.secret_key}
                  onChange={(e) => setS3({ ...s3, secret_key: e.target.value })}
                  required
                />
              </div>
              <div className="space-y-2">
                <label htmlFor="s3-path" className="text-sm font-medium">
                  Path
                </label>
                <Input
                  id="s3-path"
                  value={s3.path}
                  onChange={(e) => setS3({ ...s3, path: e.target.value })}
                  placeholder="sailbox-backups"
                />
              </div>
              <Button type="submit" className="w-full" disabled={scan.isPending}>
                {scan.isPending ? "Scanning..." : "Scan for backups"}
              </Button>
            </form>
          )}

          {step === "select" && (
            <div className="space-y-4">
              <div className="space-y-2">
                <label className="text-sm font-medium">Select a backup</label>
                <div className="max-h-60 space-y-1 overflow-y-auto rounded-md border p-2">
                  {backups.map((b) => (
                    <label
                      key={b.key}
                      className={`flex cursor-pointer items-center gap-3 rounded-md p-2 text-sm hover:bg-muted ${
                        selected === b.key ? "bg-muted" : ""
                      }`}
                    >
                      <input
                        type="radio"
                        name="backup"
                        value={b.key}
                        checked={selected === b.key}
                        onChange={() => setSelected(b.key)}
                        className="accent-primary"
                      />
                      <div className="flex-1">
                        <p className="font-medium">{b.file_name}</p>
                        <p className="text-xs text-muted-foreground">
                          {formatSize(b.size_bytes)} &middot; {b.last_modified}
                        </p>
                      </div>
                    </label>
                  ))}
                </div>
              </div>

              <div className="rounded-md border border-destructive/30 bg-destructive/5 p-4 space-y-2">
                <p className="text-sm font-medium text-destructive">Warning</p>
                <p className="text-xs text-destructive/80">
                  This will completely overwrite all data in the current database. This action
                  cannot be undone.
                </p>
                <p className="text-xs text-destructive/80">
                  Only restore from backups you trust. Malicious backups could compromise your
                  system.
                </p>
              </div>

              <div className="space-y-2">
                <label htmlFor="confirm-restore" className="text-sm font-medium">
                  Type RESTORE to confirm
                </label>
                <Input
                  id="confirm-restore"
                  value={confirm}
                  onChange={(e) => setConfirm(e.target.value)}
                  placeholder="RESTORE"
                />
              </div>

              <Button
                onClick={handleRestore}
                className="w-full"
                variant="destructive"
                disabled={confirm !== "RESTORE" || !selected}
              >
                Restore
              </Button>

              <button
                type="button"
                onClick={() => {
                  setStep("credentials");
                  setSelected("");
                  setConfirm("");
                  setError("");
                }}
                className="w-full text-center text-sm text-muted-foreground hover:text-foreground"
              >
                &larr; Back to credentials
              </button>
            </div>
          )}

          {step === "restoring" && (
            <div className="flex flex-col items-center gap-5 py-8">
              <div className="flex h-14 w-14 items-center justify-center rounded-full bg-primary/10">
                <Loader2 className="h-7 w-7 animate-spin text-primary" />
              </div>
              <div className="text-center">
                <p className="text-sm font-semibold">Restoring your data...</p>
                <p className="mt-1 text-xs text-muted-foreground">
                  This may take a minute. Please do not close this page.
                </p>
              </div>
              <div className="w-full space-y-2 text-xs text-muted-foreground">
                <div className="flex items-center gap-2">
                  <span className="h-1.5 w-1.5 rounded-full bg-primary" />
                  Downloading backup from S3
                </div>
                <div className="flex items-center gap-2">
                  <span className="h-1.5 w-1.5 rounded-full bg-primary" />
                  Restoring database
                </div>
                <div className="flex items-center gap-2">
                  <span className="h-1.5 w-1.5 rounded-full bg-muted-foreground/30" />
                  Restarting system
                </div>
              </div>
            </div>
          )}

          {step === "done" && (
            <div className="flex flex-col items-center gap-4 py-8">
              <div className="flex h-14 w-14 items-center justify-center rounded-full bg-emerald-500/10">
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  className="h-7 w-7 text-emerald-500"
                  role="img"
                  aria-label="Success"
                >
                  <polyline points="20 6 9 17 4 12" />
                </svg>
              </div>
              <div className="text-center">
                <p className="text-sm font-semibold">Restore complete!</p>
                <p className="mt-1 text-xs text-muted-foreground">
                  Your system has been restored. Redirecting to login...
                </p>
              </div>
            </div>
          )}

          {step !== "restoring" && step !== "done" && (
            <button
              type="button"
              onClick={onBack}
              className="mt-4 w-full text-center text-sm text-muted-foreground hover:text-foreground"
            >
              &larr; Back to setup
            </button>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

// ── Login (with 2FA support) ─────────────────────────────────────

function LoginForm() {
  const navigate = useNavigate();
  const [step, setStep] = useState<"credentials" | "2fa">("credentials");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [twoFACode, setTwoFACode] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      const result = await login(email, password, step === "2fa" ? twoFACode : undefined);

      if (result.requires_2fa) {
        setStep("2fa");
        setLoading(false);
        return;
      }

      navigate({ to: "/dashboard" });
    } catch (err: any) {
      setError(err?.detail || err?.message || "Login failed");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <Card className="w-full max-w-sm">
        <CardHeader className="flex flex-col items-center text-center">
          <Logo className="mb-2 h-10 w-10 text-primary" />
          <CardTitle className="text-2xl">Sailbox</CardTitle>
          <CardDescription>
            {step === "credentials" ? "Sign in to your account" : "Two-factor authentication"}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {step === "credentials" ? (
            <form onSubmit={handleSubmit} className="space-y-4">
              {error && (
                <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
                  {error}
                </div>
              )}
              <div className="space-y-2">
                <label htmlFor="login-email" className="text-sm font-medium">
                  Email
                </label>
                <Input
                  id="login-email"
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  required
                />
              </div>
              <div className="space-y-2">
                <label htmlFor="login-password" className="text-sm font-medium">
                  Password
                </label>
                <Input
                  id="login-password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                />
              </div>
              <Button type="submit" className="w-full" disabled={loading}>
                {loading ? "Signing in..." : "Sign in"}
              </Button>
              <p className="text-center text-xs text-muted-foreground">
                Need access? Contact your team administrator.
              </p>
            </form>
          ) : (
            <form onSubmit={handleSubmit} className="space-y-4">
              <p className="text-center text-sm text-muted-foreground">
                Enter the 6-digit code from your authenticator app
              </p>
              {error && (
                <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
                  {error}
                </div>
              )}
              <Input
                value={twoFACode}
                onChange={(e) => setTwoFACode(e.target.value.replace(/\D/g, "").slice(0, 6))}
                placeholder="000000"
                className="text-center font-mono text-2xl tracking-widest"
                maxLength={6}
                autoFocus
              />
              <Button type="submit" className="w-full" disabled={loading || twoFACode.length !== 6}>
                {loading ? "Verifying..." : "Verify"}
              </Button>
              <button
                type="button"
                className="w-full text-center text-sm text-muted-foreground hover:text-foreground"
                onClick={() => {
                  setStep("credentials");
                  setTwoFACode("");
                  setError("");
                }}
              >
                &larr; Back to login
              </button>
            </form>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
