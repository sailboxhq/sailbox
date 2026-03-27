import { createFileRoute } from "@tanstack/react-router";
import {
  Check,
  Cloud,
  Copy,
  Edit,
  ExternalLink,
  GitBranch,
  GitFork,
  KeyRound,
  Package,
  Play,
  Server,
  Trash2,
} from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { EmptyState } from "@/components/empty-state";
import { LoadingScreen } from "@/components/loading-screen";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
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
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import {
  useCreateResource,
  useDeleteResource,
  useGenerateSSHKey,
  useGitHubStatus,
  useResources,
  useTestResource,
  useUpdateResource,
} from "@/hooks/use-resources";
import { api } from "@/lib/api";
import { statusVariant } from "@/lib/constants";
import type { SharedResource } from "@/types/api";

export const Route = createFileRoute("/_dashboard/resources")({
  component: ResourcesPage,
});

// ── Tab config ──────────────────────────────────────────────────

const TABS = [
  { value: "git_provider", label: "Git Providers", icon: GitBranch },
  { value: "registry", label: "Registries", icon: Package },
  { value: "ssh_key", label: "SSH Keys", icon: KeyRound },
  { value: "object_storage", label: "Object Storage", icon: Cloud },
] as const;

type ResourceType = (typeof TABS)[number]["value"];

// ── Provider options per type ───────────────────────────────────

const PROVIDER_OPTIONS: Record<ResourceType, { value: string; label: string }[]> = {
  git_provider: [
    { value: "github", label: "GitHub" },
    { value: "gitlab", label: "GitLab" },
    { value: "gitea", label: "Gitea" },
  ],
  registry: [
    { value: "dockerhub", label: "Docker Hub" },
    { value: "ghcr", label: "GHCR" },
    { value: "custom", label: "Custom" },
  ],
  ssh_key: [{ value: "ssh_key", label: "SSH Key" }],
  object_storage: [
    { value: "aws_s3", label: "AWS S3" },
    { value: "cloudflare_r2", label: "Cloudflare R2" },
    { value: "minio", label: "MinIO" },
    { value: "backblaze_b2", label: "Backblaze B2" },
    { value: "do_spaces", label: "DigitalOcean Spaces" },
    { value: "custom", label: "Custom (S3 Compatible)" },
  ],
};

// ── Form field definitions per type ─────────────────────────────

interface FieldDef {
  key: string;
  label: string;
  type: "text" | "password" | "textarea";
  placeholder?: string;
  required?: boolean;
}

const FIELDS: Record<ResourceType, FieldDef[]> = {
  git_provider: [
    { key: "token", label: "Token", type: "password", placeholder: "ghp_...", required: true },
    {
      key: "api_url",
      label: "API URL",
      type: "text",
      placeholder: "https://api.github.com (optional)",
    },
    { key: "username", label: "Username", type: "text", placeholder: "Username" },
  ],
  registry: [
    {
      key: "url",
      label: "URL",
      type: "text",
      placeholder: "https://registry.example.com",
      required: true,
    },
    { key: "username", label: "Username", type: "text", placeholder: "Username", required: true },
    {
      key: "password",
      label: "Password",
      type: "password",
      placeholder: "Password",
      required: true,
    },
  ],
  ssh_key: [
    {
      key: "private_key",
      label: "Private Key",
      type: "textarea",
      placeholder: "-----BEGIN OPENSSH PRIVATE KEY-----",
      required: true,
    },
    {
      key: "passphrase",
      label: "Passphrase",
      type: "password",
      placeholder: "Optional passphrase",
    },
  ],
  object_storage: [
    {
      key: "endpoint",
      label: "Endpoint",
      type: "text",
      placeholder: "https://s3.amazonaws.com",
      required: true,
    },
    { key: "bucket", label: "Bucket", type: "text", placeholder: "my-bucket", required: true },
    {
      key: "access_key",
      label: "Access Key",
      type: "text",
      placeholder: "AKIA...",
      required: true,
    },
    {
      key: "secret_key",
      label: "Secret Key",
      type: "password",
      placeholder: "Secret key",
      required: true,
    },
    { key: "region", label: "Region", type: "text", placeholder: "us-east-1" },
  ],
};

// ── Main page ───────────────────────────────────────────────────

function ResourcesPage() {
  const [activeTab, setActiveTab] = useState<ResourceType>("git_provider");

  // Detect OAuth callback result
  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    if (params.get("setup") === "complete") {
      toast.success("GitHub App created! Now click Connect GitHub to authorize.");
      window.history.replaceState({}, "", "/resources");
    } else if (params.get("connected") === "true") {
      toast.success("GitHub connected successfully!");
      window.history.replaceState({}, "", "/resources");
    } else if (params.get("error")) {
      toast.error(`Connection failed: ${params.get("error")}`);
      window.history.replaceState({}, "", "/resources");
    }
  }, []);

  return (
    <div>
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Resources</h1>
          <p className="text-sm text-muted-foreground">
            Shared integrations, credentials, and keys
          </p>
        </div>
      </div>

      <Tabs
        value={activeTab}
        onValueChange={(v) => setActiveTab(v as ResourceType)}
        className="mt-6"
      >
        <TabsList>
          {TABS.map((t) => (
            <TabsTrigger key={t.value} value={t.value}>
              {t.label}
            </TabsTrigger>
          ))}
        </TabsList>

        {TABS.map((t) => (
          <TabsContent key={t.value} value={t.value}>
            {t.value === "git_provider" ? (
              <GitProviderTab />
            ) : (
              <ResourceTab type={t.value} icon={t.icon} />
            )}
          </TabsContent>
        ))}
      </Tabs>
    </div>
  );
}

// ── Git Providers Tab ────────────────────────────────────────────

function GitProviderTab() {
  const { data, isLoading } = useResources("git_provider");
  const { data: ghStatus } = useGitHubStatus();
  const [editing, setEditing] = useState<SharedResource | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<SharedResource | null>(null);
  const deleteMutation = useDeleteResource();
  const testMutation = useTestResource();

  // GitHub OAuth dialog
  const [showGitHubDialog, setShowGitHubDialog] = useState(false);
  const [gitHubConnectType, setGitHubConnectType] = useState<"personal" | "org">("personal");
  const [gitHubOrg, setGitHubOrg] = useState("");
  const [connectingGitHub, setConnectingGitHub] = useState(false);

  // GitLab sheet
  const [showGitLab, setShowGitLab] = useState(false);
  const [gitlabToken, setGitlabToken] = useState("");
  const [gitlabName, setGitlabName] = useState("");
  const [showGitlabName, setShowGitlabName] = useState(false);

  // Gitea sheet
  const [showGitea, setShowGitea] = useState(false);
  const [giteaToken, setGiteaToken] = useState("");
  const [giteaUrl, setGiteaUrl] = useState("");
  const [giteaName, setGiteaName] = useState("");
  const [showGiteaName, setShowGiteaName] = useState(false);

  // Edit sheet
  const [editSheetOpen, setEditSheetOpen] = useState(false);

  const createMutation = useCreateResource();

  const connectGitHub = useCallback(() => {
    setGitHubConnectType("personal");
    setGitHubOrg("");
    setShowGitHubDialog(true);
  }, []);

  // Manifest Flow: auto-create GitHub App with one click on GitHub
  const handleGitHubSetup = useCallback(async () => {
    try {
      const org = gitHubConnectType === "org" ? gitHubOrg : "";
      const result = await api.get<{
        manifest: Record<string, unknown>;
        github_url: string;
        state: string;
      }>(`/api/v1/auth/github/setup${org ? `?org=${encodeURIComponent(org)}` : ""}`);

      // POST manifest to GitHub via hidden form
      const form = document.createElement("form");
      form.method = "POST";
      form.action = `${result.github_url}?state=${result.state}`;
      const input = document.createElement("input");
      input.type = "hidden";
      input.name = "manifest";
      input.value = JSON.stringify(result.manifest);
      form.appendChild(input);
      document.body.appendChild(form);
      form.submit();
    } catch (err: any) {
      toast.error(err?.detail || "Failed to start GitHub setup");
      setConnectingGitHub(false);
    }
  }, [gitHubConnectType, gitHubOrg]);

  const handleGitHubConnect = useCallback(async () => {
    setConnectingGitHub(true);
    try {
      const params =
        gitHubConnectType === "org"
          ? `?type=org&org=${encodeURIComponent(gitHubOrg)}`
          : "?type=personal";
      const result = await api.get<{ url: string }>(`/api/v1/auth/github/connect${params}`);
      window.location.href = result.url;
    } catch (err: any) {
      const errCode = err?.error || "";
      if (errCode === "not_configured") {
        setShowGitHubDialog(false);
        handleGitHubSetup();
      } else {
        toast.error(err?.message || err?.detail || "Failed to connect GitHub");
        setConnectingGitHub(false);
      }
    }
  }, [gitHubConnectType, gitHubOrg, handleGitHubSetup]);

  const handleGitLabConnect = useCallback(
    (e: React.FormEvent) => {
      e.preventDefault();
      createMutation.mutate(
        {
          name: gitlabName,
          type: "git_provider",
          provider: "gitlab",
          config: { token: gitlabToken },
        },
        {
          onSuccess: () => {
            setShowGitLab(false);
            setGitlabToken("");
            setGitlabName("");
            setShowGitlabName(false);
          },
        },
      );
    },
    [gitlabToken, gitlabName, createMutation],
  );

  const handleGiteaConnect = useCallback(
    (e: React.FormEvent) => {
      e.preventDefault();
      createMutation.mutate(
        {
          name: giteaName,
          type: "git_provider",
          provider: "gitea",
          config: { token: giteaToken, api_url: giteaUrl },
        },
        {
          onSuccess: () => {
            setShowGitea(false);
            setGiteaToken("");
            setGiteaUrl("");
            setGiteaName("");
            setShowGiteaName(false);
          },
        },
      );
    },
    [giteaToken, giteaUrl, giteaName, createMutation],
  );

  const openEdit = useCallback((r: SharedResource) => {
    setEditing(r);
    setEditSheetOpen(true);
  }, []);

  if (isLoading) return <LoadingScreen variant="detail" />;

  const resources = data ?? [];

  return (
    <div className="mt-3 space-y-4">
      {/* Provider connection cards */}
      <div className="grid gap-4 md:grid-cols-3">
        {/* GitHub - OAuth */}
        <Card
          className="cursor-pointer hover:border-primary transition-colors"
          onClick={connectGitHub}
        >
          <CardContent className="flex flex-col items-center gap-3 py-6">
            <GitBranch className="h-8 w-8" />
            <span className="font-medium">GitHub</span>
            <span className="text-xs text-muted-foreground">OAuth &middot; One-click</span>
          </CardContent>
        </Card>

        {/* GitLab - PAT */}
        <Card
          className="cursor-pointer hover:border-primary transition-colors"
          onClick={() => setShowGitLab(true)}
        >
          <CardContent className="flex flex-col items-center gap-3 py-6">
            <GitFork className="h-8 w-8" />
            <span className="font-medium">GitLab</span>
            <span className="text-xs text-muted-foreground">Personal Access Token</span>
          </CardContent>
        </Card>

        {/* Gitea - PAT */}
        <Card
          className="cursor-pointer hover:border-primary transition-colors"
          onClick={() => setShowGitea(true)}
        >
          <CardContent className="flex flex-col items-center gap-3 py-6">
            <Server className="h-8 w-8" />
            <span className="font-medium">Gitea</span>
            <span className="text-xs text-muted-foreground">Personal Access Token</span>
          </CardContent>
        </Card>
      </div>

      {/* Existing resources list */}
      {resources.length === 0 ? (
        <EmptyState icon={GitBranch as any} message="No git provider resources yet" />
      ) : (
        resources.map((r) => (
          <Card key={r.id}>
            <CardContent className="flex items-center gap-4 p-4">
              <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary/10">
                <GitBranch className="h-4 w-4 text-primary" />
              </div>
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium">{r.name}</span>
                  <Badge variant="outline" className="text-xs">
                    {r.provider}
                  </Badge>
                  <Badge variant={statusVariant(r.status)}>{r.status}</Badge>
                </div>
                <p className="text-xs text-muted-foreground">
                  Created {new Date(r.created_at).toLocaleDateString()}
                </p>
              </div>
              <div className="flex items-center gap-1">
                {r.provider === "github" && ghStatus?.install_url && (
                  <a
                    href={ghStatus.install_url}
                    target="_blank"
                    rel="noopener noreferrer"
                    title="Manage repository access"
                  >
                    <Button variant="ghost" size="icon">
                      <ExternalLink className="h-4 w-4" />
                    </Button>
                  </a>
                )}
                <Button
                  variant="ghost"
                  size="icon"
                  title="Test"
                  disabled={testMutation.isPending}
                  onClick={() => testMutation.mutate(r.id)}
                >
                  <Play className="h-4 w-4" />
                </Button>
                <Button variant="ghost" size="icon" title="Edit" onClick={() => openEdit(r)}>
                  <Edit className="h-4 w-4" />
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  title="Delete"
                  onClick={() => setDeleteTarget(r)}
                >
                  <Trash2 className="h-4 w-4 text-destructive" />
                </Button>
              </div>
            </CardContent>
          </Card>
        ))
      )}

      {/* GitHub OAuth Dialog */}
      <Dialog open={showGitHubDialog} onOpenChange={setShowGitHubDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Connect GitHub</DialogTitle>
            <DialogDescription>
              Choose whether to connect a personal account or an organization.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="flex gap-3">
              <Button
                variant={gitHubConnectType === "personal" ? "default" : "outline"}
                className="flex-1"
                onClick={() => setGitHubConnectType("personal")}
              >
                Personal Account
              </Button>
              <Button
                variant={gitHubConnectType === "org" ? "default" : "outline"}
                className="flex-1"
                onClick={() => setGitHubConnectType("org")}
              >
                Organization
              </Button>
            </div>
            {gitHubConnectType === "org" && (
              <div className="space-y-2">
                <Label>Organization Name</Label>
                <Input
                  value={gitHubOrg}
                  onChange={(e) => setGitHubOrg(e.target.value)}
                  placeholder="my-org"
                />
              </div>
            )}
          </div>
          <DialogFooter>
            <Button
              onClick={handleGitHubConnect}
              disabled={connectingGitHub || (gitHubConnectType === "org" && !gitHubOrg.trim())}
            >
              <GitBranch className="h-4 w-4" />
              {connectingGitHub ? "Connecting..." : "Connect with GitHub"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* GitLab Sheet */}
      <Sheet
        open={showGitLab}
        onOpenChange={(open) => {
          setShowGitLab(open);
          if (!open) {
            setGitlabToken("");
            setGitlabName("");
            setShowGitlabName(false);
          }
        }}
      >
        <SheetContent>
          <SheetHeader>
            <SheetTitle>Connect GitLab</SheetTitle>
            <SheetDescription>
              Provide a Personal Access Token to connect your GitLab account.
            </SheetDescription>
          </SheetHeader>
          <form
            onSubmit={handleGitLabConnect}
            className="flex flex-1 flex-col gap-4 overflow-y-auto"
          >
            <Button
              type="button"
              variant="outline"
              onClick={() =>
                window.open("https://gitlab.com/-/user_settings/personal_access_tokens", "_blank")
              }
            >
              <ExternalLink className="h-4 w-4" />
              Generate Token on GitLab
            </Button>
            <div className="space-y-1">
              <Label>Token</Label>
              <Input
                type="password"
                value={gitlabToken}
                onChange={(e) => setGitlabToken(e.target.value)}
                placeholder="glpat-..."
                required
              />
            </div>
            {showGitlabName ? (
              <div className="space-y-1">
                <Label>Name</Label>
                <Input
                  value={gitlabName}
                  onChange={(e) => setGitlabName(e.target.value)}
                  placeholder="Auto-generated"
                />
                <p className="text-xs text-muted-foreground">Leave empty to auto-generate</p>
              </div>
            ) : (
              <button
                type="button"
                className="text-xs text-primary hover:underline text-left"
                onClick={() => setShowGitlabName(true)}
              >
                Custom name
              </button>
            )}
            <div className="mt-auto pt-4">
              <Button type="submit" className="w-full" disabled={createMutation.isPending}>
                {createMutation.isPending ? "Connecting..." : "Connect"}
              </Button>
            </div>
          </form>
        </SheetContent>
      </Sheet>

      {/* Gitea Sheet */}
      <Sheet
        open={showGitea}
        onOpenChange={(open) => {
          setShowGitea(open);
          if (!open) {
            setGiteaToken("");
            setGiteaUrl("");
            setGiteaName("");
            setShowGiteaName(false);
          }
        }}
      >
        <SheetContent>
          <SheetHeader>
            <SheetTitle>Connect Gitea</SheetTitle>
            <SheetDescription>
              Provide your Gitea instance URL and a Personal Access Token.
            </SheetDescription>
          </SheetHeader>
          <form
            onSubmit={handleGiteaConnect}
            className="flex flex-1 flex-col gap-4 overflow-y-auto"
          >
            <div className="space-y-1">
              <Label>Instance URL</Label>
              <Input
                value={giteaUrl}
                onChange={(e) => setGiteaUrl(e.target.value)}
                placeholder="https://gitea.example.com"
                required
              />
            </div>
            <div className="space-y-1">
              <Label>Token</Label>
              <Input
                type="password"
                value={giteaToken}
                onChange={(e) => setGiteaToken(e.target.value)}
                placeholder="Token"
                required
              />
            </div>
            {showGiteaName ? (
              <div className="space-y-1">
                <Label>Name</Label>
                <Input
                  value={giteaName}
                  onChange={(e) => setGiteaName(e.target.value)}
                  placeholder="Auto-generated"
                />
                <p className="text-xs text-muted-foreground">Leave empty to auto-generate</p>
              </div>
            ) : (
              <button
                type="button"
                className="text-xs text-primary hover:underline text-left"
                onClick={() => setShowGiteaName(true)}
              >
                Custom name
              </button>
            )}
            <div className="mt-auto pt-4">
              <Button type="submit" className="w-full" disabled={createMutation.isPending}>
                {createMutation.isPending ? "Connecting..." : "Connect"}
              </Button>
            </div>
          </form>
        </SheetContent>
      </Sheet>

      {/* Edit Sheet (reuses generic ResourceSheet) */}
      <ResourceSheet
        open={editSheetOpen}
        onOpenChange={setEditSheetOpen}
        type="git_provider"
        resource={editing}
      />

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null);
        }}
        title="Delete resource"
        description={`Are you sure you want to delete "${deleteTarget?.name}"? This cannot be undone.`}
        confirmLabel="Delete"
        loading={deleteMutation.isPending}
        onConfirm={() => {
          if (deleteTarget) {
            deleteMutation.mutate(deleteTarget.id, {
              onSuccess: () => setDeleteTarget(null),
            });
          }
        }}
      />
    </div>
  );
}

// ── Per-type tab (registries, SSH keys, object storage) ──────────

type ResType = "git_provider" | "registry" | "ssh_key" | "object_storage";

function ResourceCard({
  resource: r,
  type,
  icon: Icon,
  onTest,
  onEdit,
  onDelete,
  testing,
  installUrl,
}: {
  resource: SharedResource;
  type: ResType;
  icon: React.ComponentType<{ className?: string }>;
  onTest: () => void;
  onEdit: () => void;
  onDelete: () => void;
  testing: boolean;
  installUrl?: string;
}) {
  const [copiedField, setCopiedField] = useState<string | null>(null);
  const config = r.config as Record<string, string>;
  const publicKey = type === "ssh_key" ? config?.public_key : null;
  const privateKey = type === "ssh_key" ? config?.private_key : null;

  function copyToClipboard(text: string, label: string) {
    navigator.clipboard.writeText(text);
    setCopiedField(label);
    toast.success(`${label} copied!`);
    setTimeout(() => setCopiedField(null), 2000);
  }

  return (
    <Card>
      <CardContent className="p-4">
        <div className="flex items-center gap-4">
          <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary/10">
            <Icon className="h-4 w-4 text-primary" />
          </div>
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <span className="text-sm font-medium">{r.name}</span>
              <Badge variant="outline" className="text-xs">
                {r.provider}
              </Badge>
              <Badge variant={statusVariant(r.status)}>{r.status}</Badge>
            </div>
            <p className="text-xs text-muted-foreground">
              Created {new Date(r.created_at).toLocaleDateString()}
            </p>
          </div>
          <div className="flex items-center gap-1">
            {installUrl && r.provider === "github" && (
              <a
                href={installUrl}
                target="_blank"
                rel="noopener noreferrer"
                title="Manage repository access on GitHub"
              >
                <Button variant="ghost" size="icon">
                  <ExternalLink className="h-4 w-4" />
                </Button>
              </a>
            )}
            {type !== "ssh_key" && (
              <Button variant="ghost" size="icon" title="Test" disabled={testing} onClick={onTest}>
                <Play className="h-4 w-4" />
              </Button>
            )}
            <Button variant="ghost" size="icon" title="Edit" onClick={onEdit}>
              <Edit className="h-4 w-4" />
            </Button>
            <Button variant="ghost" size="icon" title="Delete" onClick={onDelete}>
              <Trash2 className="h-4 w-4 text-destructive" />
            </Button>
          </div>
        </div>

        {/* SSH Key: public + private key with copy buttons */}
        {(publicKey || privateKey) && (
          <div className="mt-3 space-y-2">
            {publicKey && (
              <div className="space-y-1">
                <p className="text-xs font-medium text-muted-foreground">Public Key</p>
                <div className="flex items-start gap-2 rounded-lg border bg-muted p-3">
                  <code className="flex-1 break-all font-mono text-xs text-foreground">
                    {publicKey}
                  </code>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-7 w-7 shrink-0"
                    onClick={() => copyToClipboard(publicKey, "Public key")}
                    title="Copy public key"
                  >
                    {copiedField === "Public key" ? (
                      <Check className="h-3.5 w-3.5 text-green-500" />
                    ) : (
                      <Copy className="h-3.5 w-3.5" />
                    )}
                  </Button>
                </div>
              </div>
            )}
            {privateKey && (
              <div className="space-y-1">
                <p className="text-xs font-medium text-muted-foreground">Private Key</p>
                <div className="flex items-start gap-2 rounded-lg border bg-muted p-3">
                  <code className="flex-1 break-all font-mono text-xs text-foreground">
                    {privateKey.slice(0, 60)}...
                  </code>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-7 w-7 shrink-0"
                    onClick={() => copyToClipboard(privateKey, "Private key")}
                    title="Copy private key"
                  >
                    {copiedField === "Private key" ? (
                      <Check className="h-3.5 w-3.5 text-green-500" />
                    ) : (
                      <Copy className="h-3.5 w-3.5" />
                    )}
                  </Button>
                </div>
              </div>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function ResourceTab({
  type,
  icon: Icon,
}: {
  type: ResType;
  icon: React.ComponentType<{ className?: string }>;
}) {
  const { data, isLoading } = useResources(type);
  const { data: ghStatus } = useGitHubStatus();
  const [sheetOpen, setSheetOpen] = useState(false);
  const [editing, setEditing] = useState<SharedResource | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<SharedResource | null>(null);
  const deleteMutation = useDeleteResource();
  const testMutation = useTestResource();
  const generateSSH = useGenerateSSHKey();
  const [sshAlgorithm, setSSHAlgorithm] = useState("ed25519");
  const [showGenerate, setShowGenerate] = useState(false);

  const openCreate = useCallback(() => {
    setEditing(null);
    setSheetOpen(true);
  }, []);

  const openEdit = useCallback((r: SharedResource) => {
    setEditing(r);
    setSheetOpen(true);
  }, []);

  if (isLoading) return <LoadingScreen variant="detail" />;

  const resources = data ?? [];

  return (
    <div className="mt-3 space-y-3">
      <div className="flex items-center justify-end gap-2">
        {type === "ssh_key" && (
          <Button size="sm" variant="outline" onClick={() => setShowGenerate(true)}>
            Generate Key
          </Button>
        )}
        <Button size="sm" onClick={openCreate}>
          Add
        </Button>
      </div>

      {/* SSH Key Generate Sheet */}
      {type === "ssh_key" && (
        <Sheet open={showGenerate} onOpenChange={setShowGenerate}>
          <SheetContent>
            <SheetHeader>
              <SheetTitle>Generate SSH Key</SheetTitle>
              <SheetDescription>
                Create a new SSH key pair. The private key will be stored securely.
              </SheetDescription>
            </SheetHeader>
            <div className="mt-6 space-y-4">
              <div className="space-y-1.5">
                <Label className="text-sm font-medium">Algorithm</Label>
                <Select value={sshAlgorithm} onValueChange={setSSHAlgorithm}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="ed25519">Ed25519 (recommended)</SelectItem>
                    <SelectItem value="rsa-4096">RSA 4096-bit</SelectItem>
                  </SelectContent>
                </Select>
                <p className="text-xs text-muted-foreground">
                  Ed25519 is faster and more secure. RSA 4096 for legacy compatibility.
                </p>
              </div>
              <Button
                className="w-full"
                onClick={() => {
                  generateSSH.mutate(
                    { algorithm: sshAlgorithm },
                    { onSuccess: () => setShowGenerate(false) },
                  );
                }}
                disabled={generateSSH.isPending}
              >
                {generateSSH.isPending ? "Generating..." : "Generate Key Pair"}
              </Button>
            </div>
          </SheetContent>
        </Sheet>
      )}

      {resources.length === 0 ? (
        <EmptyState
          icon={Icon as any}
          message={`No ${type.replace("_", " ")} resources yet`}
          actionLabel="Add"
          onAction={openCreate}
        />
      ) : (
        resources.map((r) => (
          <ResourceCard
            key={r.id}
            resource={r}
            type={type}
            icon={Icon}
            onTest={() => testMutation.mutate(r.id)}
            onEdit={() => openEdit(r)}
            onDelete={() => setDeleteTarget(r)}
            testing={testMutation.isPending}
            installUrl={ghStatus?.install_url}
          />
        ))
      )}

      <ResourceSheet open={sheetOpen} onOpenChange={setSheetOpen} type={type} resource={editing} />

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null);
        }}
        title="Delete resource"
        description={`Are you sure you want to delete "${deleteTarget?.name}"? This cannot be undone.`}
        confirmLabel="Delete"
        loading={deleteMutation.isPending}
        onConfirm={() => {
          if (deleteTarget) {
            deleteMutation.mutate(deleteTarget.id, {
              onSuccess: () => setDeleteTarget(null),
            });
          }
        }}
      />
    </div>
  );
}

// ── Add / Edit Sheet ────────────────────────────────────────────

function ResourceSheet({
  open,
  onOpenChange,
  type,
  resource,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  type: ResType;
  resource: SharedResource | null;
}) {
  const isEdit = !!resource;
  const createMutation = useCreateResource();
  const updateMutation = useUpdateResource();

  const [name, setName] = useState("");
  const [provider, setProvider] = useState("");
  const [config, setConfig] = useState<Record<string, string>>({});

  // Sync form state when resource changes (edit mode) or sheet opens
  useEffect(() => {
    if (open) {
      if (resource) {
        setName(resource.name);
        setProvider(resource.provider);
        setConfig(
          typeof resource.config === "object" && resource.config ? { ...resource.config } : {},
        );
      } else {
        setName("");
        setProvider(PROVIDER_OPTIONS[type]?.[0]?.value ?? "");
        setConfig({});
      }
    }
  }, [open, resource, type]);

  const setConfigField = useCallback((key: string, value: string) => {
    setConfig((prev) => ({ ...prev, [key]: value }));
  }, []);

  const saving = createMutation.isPending || updateMutation.isPending;

  const handleSubmit = useCallback(
    (e: React.FormEvent) => {
      e.preventDefault();
      if (isEdit && resource) {
        updateMutation.mutate(
          { id: resource.id, name, provider, config },
          { onSuccess: () => onOpenChange(false) },
        );
      } else {
        createMutation.mutate(
          { name, type, provider, config },
          { onSuccess: () => onOpenChange(false) },
        );
      }
    },
    [isEdit, resource, name, provider, config, type, createMutation, updateMutation, onOpenChange],
  );

  const fields = FIELDS[type];
  const providers = PROVIDER_OPTIONS[type];

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent>
        <SheetHeader>
          <SheetTitle>
            {isEdit ? "Edit" : "Add"} {type.replace("_", " ")}
          </SheetTitle>
          <SheetDescription>
            {isEdit ? "Update the resource configuration." : "Connect a new resource."}
          </SheetDescription>
        </SheetHeader>

        <form onSubmit={handleSubmit} className="flex flex-1 flex-col gap-4 overflow-y-auto">
          <div className="space-y-1">
            <Label htmlFor="res-name">Name</Label>
            <Input
              id="res-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Auto-generated"
            />
            <p className="text-xs text-muted-foreground">Leave empty to auto-generate</p>
          </div>

          {providers.length > 1 && (
            <div className="space-y-1">
              <Label>Provider</Label>
              <Select value={provider} onValueChange={setProvider}>
                <SelectTrigger>
                  <SelectValue placeholder="Select provider" />
                </SelectTrigger>
                <SelectContent>
                  {providers.map((p) => (
                    <SelectItem key={p.value} value={p.value}>
                      {p.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}

          {fields.map((f) => (
            <div key={f.key} className="space-y-1">
              <Label htmlFor={`res-${f.key}`}>{f.label}</Label>
              {f.type === "textarea" ? (
                <Textarea
                  id={`res-${f.key}`}
                  value={config[f.key] ?? ""}
                  onChange={(e) => setConfigField(f.key, e.target.value)}
                  placeholder={f.placeholder}
                  required={f.required}
                  rows={6}
                  className="font-mono text-xs"
                />
              ) : (
                <Input
                  id={`res-${f.key}`}
                  type={f.type}
                  value={config[f.key] ?? ""}
                  onChange={(e) => setConfigField(f.key, e.target.value)}
                  placeholder={f.placeholder}
                  required={f.required}
                />
              )}
            </div>
          ))}

          <div className="mt-auto pt-4">
            <Button type="submit" className="w-full" disabled={saving}>
              {saving ? "Saving..." : isEdit ? "Update" : "Create"}
            </Button>
          </div>
        </form>
      </SheetContent>
    </Sheet>
  );
}
