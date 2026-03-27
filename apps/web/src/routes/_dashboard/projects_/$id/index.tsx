import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import {
  Box,
  ChevronRight,
  Container,
  Database,
  Plus,
  Rocket,
  Save,
  Settings2,
  Trash2,
  X,
} from "lucide-react";
import { useState } from "react";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { DangerZone } from "@/components/danger-zone";
import { EmptyState } from "@/components/empty-state";
import { LoadingScreen } from "@/components/loading-screen";
import { PageHeader } from "@/components/page-header";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
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
import { Separator } from "@/components/ui/separator";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useCreateApp, useDeleteApp, useDeploy, useProjectApps } from "@/hooks/use-apps";
import {
  useCreateDatabase,
  useDatabaseVersions,
  useDeleteDatabase,
  useProjectDatabases,
} from "@/hooks/use-databases";
import {
  useDeleteProject,
  useProject,
  useUpdateProject,
  useUpdateProjectEnv,
} from "@/hooks/use-projects";
import { useGitRepos, useResources } from "@/hooks/use-resources";
import { ENGINE_LABELS, statusVariant } from "@/lib/constants";
import type { App, ManagedDB, ServiceItem, SharedResource } from "@/types/api";

export const Route = createFileRoute("/_dashboard/projects_/$id/")({
  component: ProjectDetailPage,
});

function ProjectDetailPage() {
  const { id: projectId } = Route.useParams();
  const navigate = useNavigate();

  // ── Data ──────────────────────────────────────────────────────
  const { data: project, isLoading: projectLoading } = useProject(projectId);
  const { data: rawApps, isLoading: appsLoading } = useProjectApps(projectId);
  const { data: rawDatabases, isLoading: dbsLoading } = useProjectDatabases(projectId);
  const apps = rawApps ?? [];
  const databases = rawDatabases ?? [];
  const loading = projectLoading || appsLoading || dbsLoading;

  // ── Mutations ─────────────────────────────────────────────────
  const updateProject = useUpdateProject(projectId);
  const deleteProject = useDeleteProject(projectId);
  const createApp = useCreateApp(projectId);
  const createDatabase = useCreateDatabase(projectId);

  // ── Local state ───────────────────────────────────────────────
  const [showCreate, setShowCreate] = useState(false);
  const [showDeleteProject, setShowDeleteProject] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<ServiceItem | null>(null);
  const [serviceType, setServiceType] = useState<"app" | "database">("app");
  const [appForm, setAppForm] = useState({
    name: "",
    source_type: "image",
    docker_image: "",
    git_repo: "",
    git_branch: "main",
  });
  const [dbForm, setDbForm] = useState({
    name: "",
    database_name: "",
    engine: "postgres",
    version: "",
    storage_size: "1Gi",
  });
  const [editName, setEditName] = useState(project?.name ?? "");
  const [editDesc, setEditDesc] = useState(project?.description ?? "");

  // Sync edit fields when project loads
  if (project && editName === "" && project.name) {
    setEditName(project.name);
    setEditDesc(project.description);
  }

  // ── Derived ───────────────────────────────────────────────────
  const services: ServiceItem[] = [
    ...apps.map((a): ServiceItem => ({ type: "app", data: a })),
    ...databases.map((d): ServiceItem => ({ type: "database", data: d })),
  ];

  // ── Handlers ──────────────────────────────────────────────────
  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    if (serviceType === "app") {
      // Validate required fields
      if (!appForm.name.trim()) {
        const { toast } = await import("sonner");
        toast.error("Name is required");
        return;
      }
      if (appForm.source_type === "git" && !appForm.git_repo.trim()) {
        const { toast } = await import("sonner");
        toast.error("Please select a repository");
        return;
      }
      if (appForm.source_type === "image" && !appForm.docker_image.trim()) {
        const { toast } = await import("sonner");
        toast.error("Docker image is required");
        return;
      }
      await createApp.mutateAsync(appForm);
      setAppForm({
        name: "",
        source_type: "image",
        docker_image: "",
        git_repo: "",
        git_branch: "main",
      });
    } else {
      await createDatabase.mutateAsync(dbForm);
      setDbForm({
        name: "",
        database_name: "",
        engine: "postgres",
        version: "",
        storage_size: "1Gi",
      });
    }
    setShowCreate(false);
  }

  if (loading) return <LoadingScreen variant="detail" />;
  if (!project) return null;

  return (
    <div>
      <PageHeader
        title={project.name}
        description={
          <>
            {project.description}
            {project.namespace && (
              <span className="ml-2 font-mono text-xs text-muted-foreground">
                ns: {project.namespace}
              </span>
            )}
          </>
        }
        backTo="/projects"
      />
      <Separator className="my-5" />

      <Tabs defaultValue="services">
        <div className="flex items-center justify-between">
          <TabsList>
            <TabsTrigger value="services">Services</TabsTrigger>
            <TabsTrigger value="environment">Environment</TabsTrigger>
            <TabsTrigger value="settings">Settings</TabsTrigger>
          </TabsList>
          <Button size="sm" onClick={() => setShowCreate(true)}>
            <Plus className="h-4 w-4" /> New Service
          </Button>
        </div>

        {/* ── Services tab ── */}
        <TabsContent value="services" className="mt-4">
          {services.length === 0 ? (
            <EmptyState
              icon={Container}
              message="No services yet."
              actionLabel="New Service"
              onAction={() => setShowCreate(true)}
            />
          ) : (
            <ServiceList services={services} projectId={projectId} onDelete={setDeleteTarget} />
          )}
        </TabsContent>

        {/* ── Environment tab ── */}
        <TabsContent value="environment" className="mt-4">
          <ProjectEnvEditor projectId={projectId} envVars={project.env_vars} />
        </TabsContent>

        {/* ── Settings tab ── */}
        <TabsContent value="settings" className="mt-4 space-y-6">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-sm font-medium">
                <Settings2 className="h-4 w-4" /> General
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="space-y-2">
                <Label>Project Name</Label>
                <Input value={editName} onChange={(e) => setEditName(e.target.value)} />
              </div>
              <div className="space-y-2">
                <Label>Description</Label>
                <Input value={editDesc} onChange={(e) => setEditDesc(e.target.value)} />
              </div>
              <Button
                onClick={() => updateProject.mutate({ name: editName, description: editDesc })}
                disabled={updateProject.isPending}
              >
                {updateProject.isPending ? "Saving..." : "Save Changes"}
              </Button>
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle className="text-sm font-medium">Service Account</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="font-mono text-sm">
                {project.service_account || (
                  <span className="text-muted-foreground">Auto-created on deploy</span>
                )}
              </p>
            </CardContent>
          </Card>
          <DangerZone
            description="Permanently delete this project and all services."
            buttonLabel="Delete Project"
            onDelete={() => setShowDeleteProject(true)}
          />
        </TabsContent>
      </Tabs>

      {/* ── Create Service Dialog ── */}
      <CreateServiceDialog
        open={showCreate}
        onOpenChange={setShowCreate}
        serviceType={serviceType}
        onServiceTypeChange={setServiceType}
        appForm={appForm}
        onAppFormChange={setAppForm}
        dbForm={dbForm}
        onDbFormChange={setDbForm}
        creating={createApp.isPending || createDatabase.isPending}
        onSubmit={handleCreate}
      />

      {/* ── Delete Service Dialog ── */}
      {deleteTarget && (
        <DeleteServiceDialog service={deleteTarget} onClose={() => setDeleteTarget(null)} />
      )}

      {/* ── Delete Project Dialog ── */}
      <DeleteProjectDialog
        open={showDeleteProject}
        onOpenChange={setShowDeleteProject}
        project={project}
        apps={apps}
        databases={databases}
        loading={deleteProject.isPending}
        onConfirm={() =>
          deleteProject.mutate(undefined, {
            onSuccess: () => navigate({ to: "/projects" }),
          })
        }
      />
    </div>
  );
}

// ── Service list ──────────────────────────────────────────────────

function ServiceList({
  services,
  projectId,
  onDelete,
}: {
  services: ServiceItem[];
  projectId: string;
  onDelete: (svc: ServiceItem) => void;
}) {
  return (
    <div className="space-y-2">
      {services.map((svc) => (
        <ServiceRow
          key={`${svc.type}-${svc.data.id}`}
          service={svc}
          projectId={projectId}
          onDelete={() => onDelete(svc)}
        />
      ))}
    </div>
  );
}

function ServiceRow({
  service: svc,
  projectId,
  onDelete,
}: {
  service: ServiceItem;
  projectId: string;
  onDelete: () => void;
}) {
  const isApp = svc.type === "app";
  const item = svc.data;
  const Icon = isApp ? Box : Database;
  const deploy = useDeploy(item.id);

  const typeBadge = isApp ? "Application" : ENGINE_LABELS[(item as ManagedDB).engine] || "Database";

  const subtitle = isApp
    ? (item as App).source_type === "image"
      ? (item as App).docker_image
      : `${(item as App).git_repo} @ ${(item as App).git_branch}`
    : `v${(item as ManagedDB).version} · ${(item as ManagedDB).storage_size}`;

  const detailTo = isApp ? "/projects/$id/apps/$appId" : "/projects/$id/databases/$dbId";
  const detailParams = isApp ? { id: projectId, appId: item.id } : { id: projectId, dbId: item.id };

  return (
    <Card className="group transition-colors hover:bg-accent/50">
      <CardContent className="flex items-center gap-4 p-4">
        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-primary/10">
          <Icon className="h-5 w-5 text-primary" />
        </div>
        <Link to={detailTo as any} params={detailParams as any} className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <span className="text-sm font-medium">{item.name}</span>
            <Badge variant="outline" className="text-xs">
              {typeBadge}
            </Badge>
            <Badge variant={statusVariant(item.status)} className="text-xs">
              {item.status}
            </Badge>
          </div>
          <p className="truncate text-xs text-muted-foreground">{subtitle}</p>
        </Link>
        <div className="flex shrink-0 items-center gap-1 opacity-0 transition-opacity group-hover:opacity-100">
          {isApp && (
            <Button
              size="icon"
              variant="ghost"
              className="h-8 w-8"
              onClick={() => deploy.mutate()}
              disabled={deploy.isPending}
            >
              <Rocket className="h-3.5 w-3.5" />
            </Button>
          )}
          <Button
            size="icon"
            variant="ghost"
            className="h-8 w-8 text-destructive hover:text-destructive"
            onClick={onDelete}
          >
            <Trash2 className="h-3.5 w-3.5" />
          </Button>
          <Link to={detailTo as any} params={detailParams as any}>
            <ChevronRight className="h-4 w-4 text-muted-foreground" />
          </Link>
        </div>
      </CardContent>
    </Card>
  );
}

// ── Delete Service Dialog ──────────────────────────────────────────

function DeleteServiceDialog({ service, onClose }: { service: ServiceItem; onClose: () => void }) {
  const deleteApp = useDeleteApp(service.data.id);
  const deleteDb = useDeleteDatabase(service.data.id);
  const mutation = service.type === "app" ? deleteApp : deleteDb;

  return (
    <ConfirmDialog
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title="Delete Service"
      description={
        <>
          Delete <strong>{service.data.name}</strong>? This removes the{" "}
          {service.type === "app" ? "Deployment" : "StatefulSet"} from K3s.
        </>
      }
      confirmLabel="Delete"
      loading={mutation.isPending}
      onConfirm={() => mutation.mutate(undefined as any, { onSuccess: onClose })}
    />
  );
}

// ── Project Env Editor ────────────────────────────────────────────

type EnvPair = { key: string; value: string };

function ProjectEnvEditor({
  projectId,
  envVars,
}: {
  projectId: string;
  envVars: Record<string, string>;
}) {
  const updateEnv = useUpdateProjectEnv(projectId);
  const initial = Object.entries(envVars || {}).map(([key, value]) => ({ key, value }));
  const [pairs, setPairs] = useState<EnvPair[]>(initial);

  function pairsToRecord(p: EnvPair[]): Record<string, string> {
    const result: Record<string, string> = {};
    for (const pair of p) {
      if (pair.key.trim()) result[pair.key.trim()] = pair.value;
    }
    return result;
  }

  const dirty = JSON.stringify(pairsToRecord(pairs)) !== JSON.stringify(envVars || {});

  function handleSave() {
    updateEnv.mutate(pairsToRecord(pairs));
  }

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="text-sm">Environment Variables</CardTitle>
        <div className="flex items-center gap-2">
          <Button
            size="sm"
            variant="outline"
            onClick={() => setPairs([...pairs, { key: "", value: "" }])}
          >
            <Plus className="h-3.5 w-3.5" /> Add Variable
          </Button>
          {dirty && (
            <Button size="sm" onClick={handleSave} disabled={updateEnv.isPending}>
              <Save className="h-3.5 w-3.5" /> {updateEnv.isPending ? "Saving..." : "Save"}
            </Button>
          )}
        </div>
      </CardHeader>
      <CardContent>
        {pairs.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            No environment variables. Click <strong>Add Variable</strong> to add one.
          </p>
        ) : (
          <div className="space-y-2">
            {pairs.map((pair, i) => (
              <div key={i} className="flex items-center gap-2">
                <Input
                  className="w-48 font-mono text-sm"
                  placeholder="KEY"
                  value={pair.key}
                  onChange={(e) => {
                    const next = [...pairs];
                    next[i] = { ...next[i], key: e.target.value };
                    setPairs(next);
                  }}
                />
                <span className="text-muted-foreground">=</span>
                <Input
                  className="flex-1 font-mono text-sm"
                  placeholder="value"
                  value={pair.value}
                  onChange={(e) => {
                    const next = [...pairs];
                    next[i] = { ...next[i], value: e.target.value };
                    setPairs(next);
                  }}
                />
                <Button
                  size="icon"
                  variant="ghost"
                  className="h-8 w-8 shrink-0 text-destructive"
                  onClick={() => setPairs(pairs.filter((_, j) => j !== i))}
                >
                  <X className="h-3.5 w-3.5" />
                </Button>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

// ── Create Service Dialog ─────────────────────────────────────────

function CreateServiceDialog({
  open,
  onOpenChange,
  serviceType,
  onServiceTypeChange,
  appForm,
  onAppFormChange,
  dbForm,
  onDbFormChange,
  creating,
  onSubmit,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  serviceType: "app" | "database";
  onServiceTypeChange: (v: "app" | "database") => void;
  appForm: {
    name: string;
    source_type: string;
    docker_image: string;
    git_repo: string;
    git_branch: string;
  };
  onAppFormChange: (v: typeof appForm) => void;
  dbForm: { name: string; engine: string; version: string; storage_size: string };
  onDbFormChange: (v: typeof dbForm) => void;
  creating: boolean;
  onSubmit: (e: React.FormEvent) => void;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>New Service</DialogTitle>
          <DialogDescription>Deploy a new service to your K3s cluster.</DialogDescription>
        </DialogHeader>

        {/* Type selector */}
        <div className="grid grid-cols-2 gap-4">
          {(["app", "database"] as const).map((type) => {
            const active = serviceType === type;
            const Icon = type === "app" ? Box : Database;
            const label = type === "app" ? "Application" : "Database";
            return (
              <button
                key={type}
                type="button"
                onClick={() => onServiceTypeChange(type)}
                className={`flex flex-col items-center gap-2 rounded-lg border p-4 text-sm transition-colors ${
                  active ? "border-primary bg-primary/5" : "hover:bg-accent"
                }`}
              >
                <Icon className={`h-6 w-6 ${active ? "text-primary" : "text-muted-foreground"}`} />
                <span className="font-medium">{label}</span>
              </button>
            );
          })}
        </div>
        <Separator />

        <form onSubmit={onSubmit} className="space-y-4">
          {serviceType === "app" ? (
            <AppFormFields form={appForm} onChange={onAppFormChange} />
          ) : (
            <DbFormFields form={dbForm} onChange={onDbFormChange} />
          )}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={creating}>
              {creating ? "Creating..." : "Create"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function ImageSourceFields({
  form,
  onChange,
  registries,
}: {
  form: { docker_image: string; [key: string]: string };
  onChange: (v: typeof form) => void;
  registries: SharedResource[];
}) {
  const update = (field: string, value: string) => onChange({ ...form, [field]: value });
  const [selectedRegistry, setSelectedRegistry] = useState("dockerhub");

  const handleRegistryChange = (registryId: string) => {
    setSelectedRegistry(registryId);
    if (registryId === "dockerhub") {
      // Clear any registry prefix
      update("docker_image", "");
    } else {
      const reg = registries.find((r) => r.id === registryId);
      const url = (reg?.config as Record<string, string>)?.url || "";
      // Clean up URL: remove protocol, trailing slash
      const host = url.replace(/^https?:\/\//, "").replace(/\/$/, "");
      update("docker_image", host ? `${host}/` : "");
    }
  };

  return (
    <div className="space-y-3">
      <div className="space-y-2">
        <Label>Registry</Label>
        <Select value={selectedRegistry} onValueChange={handleRegistryChange}>
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="dockerhub">Docker Hub (public)</SelectItem>
            {registries.map((r) => (
              <SelectItem key={r.id} value={r.id}>
                {r.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {registries.length === 0 && (
          <p className="text-xs text-muted-foreground">
            Add custom registries (GHCR, ECR, etc.) in{" "}
            <Link to="/resources" className="text-primary underline underline-offset-4">
              Resources
            </Link>
          </p>
        )}
      </div>
      <div className="space-y-2">
        <Label>Image</Label>
        <Input
          value={form.docker_image}
          onChange={(e) => update("docker_image", e.target.value)}
          placeholder={selectedRegistry === "dockerhub" ? "nginx:latest" : "org/image:tag"}
          required
        />
        <p className="text-xs text-muted-foreground">
          Full image reference including tag, e.g. <code className="text-xs">nginx:latest</code> or{" "}
          <code className="text-xs">ghcr.io/user/app:v1.0</code>
        </p>
      </div>
    </div>
  );
}

function GitSourceFields({
  form,
  onChange,
  gitProviders,
}: {
  form: { git_repo: string; git_branch: string; [key: string]: string };
  onChange: (v: typeof form) => void;
  gitProviders: { id: string; name: string; provider: string }[];
}) {
  const [selectedProviderId, setSelectedProviderId] = useState("");
  const [selectedRepoName, setSelectedRepoName] = useState("");
  const { data: repos, isLoading: loadingRepos } = useGitRepos(selectedProviderId);

  function handleProviderChange(providerId: string) {
    setSelectedProviderId(providerId);
    setSelectedRepoName("");
    onChange({ ...form, git_repo: "", git_branch: "main", git_provider_id: providerId });
  }

  function handleRepoSelect(fullName: string) {
    setSelectedRepoName(fullName);
    const repo = repos?.find((r) => r.full_name === fullName);
    if (repo) {
      onChange({
        ...form,
        git_repo: repo.clone_url,
        git_branch: repo.default_branch || form.git_branch,
        git_provider_id: selectedProviderId,
      });
    }
  }

  function update(field: string, value: string) {
    onChange({ ...form, [field]: value });
  }

  return (
    <div className="space-y-3">
      {/* Step 1: Git Provider */}
      {gitProviders.length > 0 ? (
        <div className="space-y-2">
          <Label>Git Provider</Label>
          <Select value={selectedProviderId} onValueChange={handleProviderChange}>
            <SelectTrigger>
              <SelectValue placeholder="Select a provider..." />
            </SelectTrigger>
            <SelectContent>
              {gitProviders.map((p) => (
                <SelectItem key={p.id} value={p.id}>
                  {p.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      ) : (
        <div className="rounded-lg border border-dashed p-4 text-center">
          <p className="text-sm text-muted-foreground">No Git providers configured.</p>
          <a href="/resources" className="mt-1 inline-block text-sm text-primary hover:underline">
            Connect GitHub, GitLab, or Gitea →
          </a>
        </div>
      )}

      {/* Step 2: Repository (from provider or manual) */}
      {selectedProviderId ? (
        <div className="space-y-2">
          <Label>Repository</Label>
          {loadingRepos ? (
            <div className="flex h-9 items-center rounded-md border px-3 text-sm text-muted-foreground">
              Loading repositories...
            </div>
          ) : repos && repos.length > 0 ? (
            <Select value={selectedRepoName} onValueChange={handleRepoSelect}>
              <SelectTrigger>
                <SelectValue placeholder="Select a repository..." />
              </SelectTrigger>
              <SelectContent className="max-h-[300px]">
                {repos.map((r) => (
                  <SelectItem key={r.full_name} value={r.full_name}>
                    {r.full_name}
                    {r.private ? " 🔒" : ""}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          ) : (
            <div className="space-y-2">
              <p className="text-xs text-muted-foreground">
                No repositories found or failed to load.
              </p>
              <Input
                value={form.git_repo}
                onChange={(e) => update("git_repo", e.target.value)}
                placeholder="https://github.com/user/repo"
                required
              />
            </div>
          )}
        </div>
      ) : gitProviders.length > 0 ? null : (
        <div className="space-y-2">
          <Label>Repository URL</Label>
          <Input
            value={form.git_repo}
            onChange={(e) => update("git_repo", e.target.value)}
            placeholder="https://github.com/user/repo"
            required
          />
        </div>
      )}

      {/* Step 3: Branch + Build Type */}
      {form.git_repo && (
        <div className="grid grid-cols-2 gap-4">
          <div className="space-y-2">
            <Label>Branch</Label>
            <Select
              value={form.git_branch || "main"}
              onValueChange={(v) => update("git_branch", v)}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="main">main</SelectItem>
                <SelectItem value="master">master</SelectItem>
                <SelectItem value="develop">develop</SelectItem>
                <SelectItem value="staging">staging</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <Label>Build Type</Label>
            <Select value="dockerfile" onValueChange={() => {}}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="dockerfile">Dockerfile</SelectItem>
                <SelectItem value="nixpacks">Nixpacks (auto)</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
      )}
    </div>
  );
}

function AppFormFields({
  form,
  onChange,
}: {
  form: {
    name: string;
    source_type: string;
    docker_image: string;
    git_repo: string;
    git_branch: string;
  };
  onChange: (v: typeof form) => void;
}) {
  const update = (field: string, value: string) => onChange({ ...form, [field]: value });

  // Fetch git providers and registries from shared resources
  const { data: gitProviders } = useResources("git_provider");
  const { data: registries } = useResources("registry");

  return (
    <>
      <div className="space-y-2">
        <Label>Name</Label>
        <Input
          value={form.name}
          onChange={(e) => update("name", e.target.value)}
          placeholder="my-app"
          required
        />
      </div>
      <div className="space-y-2">
        <Label>Source</Label>
        <Select value={form.source_type} onValueChange={(v) => update("source_type", v)}>
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="image">Docker Image</SelectItem>
            <SelectItem value="git">Git Repository</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {form.source_type === "image" ? (
        <ImageSourceFields form={form} onChange={onChange} registries={registries ?? []} />
      ) : (
        <GitSourceFields form={form} onChange={onChange} gitProviders={gitProviders ?? []} />
      )}
    </>
  );
}

function DbFormFields({
  form,
  onChange,
}: {
  form: {
    name: string;
    database_name: string;
    engine: string;
    version: string;
    storage_size: string;
  };
  onChange: (v: typeof form) => void;
}) {
  const update = (field: string, value: string) => onChange({ ...form, [field]: value });
  const { data: rawVersions, isLoading: versionsLoading } = useDatabaseVersions(form.engine);
  const versions = rawVersions ?? [];

  // Auto-select recommended version when engine changes and versions load
  if (versions.length > 0 && !versions.some((v) => v.tag === form.version)) {
    const recommended = versions.find((v) => v.is_recommended);
    const fallback = recommended?.tag ?? versions[0]?.tag ?? "";
    if (fallback && fallback !== form.version) {
      onChange({ ...form, version: fallback });
    }
  }

  return (
    <>
      <div className="space-y-2">
        <Label>Name</Label>
        <Input
          value={form.name}
          onChange={(e) => update("name", e.target.value)}
          placeholder="my-database"
          required
        />
      </div>
      <div className="space-y-2">
        <Label>
          Database Name <span className="text-muted-foreground font-normal">(optional)</span>
        </Label>
        <Input
          value={form.database_name}
          onChange={(e) => update("database_name", e.target.value)}
          placeholder={form.name || "defaults to service name"}
        />
      </div>
      <div className="space-y-2">
        <Label>Engine</Label>
        <Select
          value={form.engine}
          onValueChange={(v) => onChange({ ...form, engine: v, version: "" })}
        >
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {Object.entries(ENGINE_LABELS).map(([key, label]) => (
              <SelectItem key={key} value={key}>
                {label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <Label>Version</Label>
          <Select
            value={form.version}
            onValueChange={(v) => update("version", v)}
            disabled={versionsLoading || versions.length === 0}
          >
            <SelectTrigger>
              <SelectValue placeholder={versionsLoading ? "Loading..." : "Select version"} />
            </SelectTrigger>
            <SelectContent>
              {versions.map((v) => (
                <SelectItem key={v.tag} value={v.tag}>
                  {v.label}
                  {v.is_recommended && " ⭐"}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-2">
          <Label>Storage</Label>
          <Select value={form.storage_size} onValueChange={(v) => update("storage_size", v)}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {["1Gi", "2Gi", "5Gi", "10Gi", "20Gi", "50Gi", "100Gi"].map((size) => (
                <SelectItem key={size} value={size}>
                  {size}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>
    </>
  );
}

// ── Delete Project Dialog ─────────────────────────────────────────

function DeleteProjectDialog({
  open,
  onOpenChange,
  project,
  apps,
  databases,
  loading,
  onConfirm,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  project: { name: string; namespace: string };
  apps: { name: string }[];
  databases: { name: string }[];
  loading: boolean;
  onConfirm: () => void;
}) {
  const [confirmName, setConfirmName] = useState("");
  const totalResources = apps.length + databases.length;

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) setConfirmName("");
        onOpenChange(v);
      }}
    >
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Delete Project</DialogTitle>
          <DialogDescription>
            This will permanently destroy the namespace and all resources inside it.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          {/* Resource summary */}
          {totalResources > 0 && (
            <div className="rounded-md border border-destructive/30 bg-destructive/5 p-3">
              <p className="mb-2 text-xs font-medium text-destructive">
                The following will be permanently deleted:
              </p>
              <div className="space-y-1.5">
                {apps.length > 0 && (
                  <div className="flex items-start gap-2 text-sm">
                    <span className="mt-1.5 inline-block h-1.5 w-1.5 shrink-0 rounded-full bg-destructive" />
                    <span>
                      <strong>{apps.length}</strong> app{apps.length > 1 ? "s" : ""}:{" "}
                      <span className="text-muted-foreground">
                        {apps.map((a) => a.name).join(", ")}
                      </span>
                    </span>
                  </div>
                )}
                {databases.length > 0 && (
                  <div className="flex items-start gap-2 text-sm">
                    <span className="mt-1.5 inline-block h-1.5 w-1.5 shrink-0 rounded-full bg-destructive" />
                    <span>
                      <strong>{databases.length}</strong> database{databases.length > 1 ? "s" : ""}:{" "}
                      <span className="text-muted-foreground">
                        {databases.map((d) => d.name).join(", ")}
                      </span>
                    </span>
                  </div>
                )}
                <div className="flex items-start gap-2 text-sm">
                  <span className="mt-1.5 inline-block h-1.5 w-1.5 shrink-0 rounded-full bg-destructive" />
                  <span>All volumes, secrets, domains, and environment variables</span>
                </div>
              </div>
            </div>
          )}

          {/* Namespace info */}
          {project.namespace && (
            <p className="text-xs text-muted-foreground">
              Namespace <code className="rounded bg-muted px-1 py-0.5">{project.namespace}</code>{" "}
              will be deleted from the cluster.
            </p>
          )}

          {/* Confirm input */}
          <div className="space-y-1.5">
            <Label htmlFor="confirm-project-name" className="text-sm">
              Type <strong className="font-mono">{project.name}</strong> to confirm
            </Label>
            <Input
              id="confirm-project-name"
              placeholder={project.name}
              value={confirmName}
              onChange={(e) => setConfirmName(e.target.value)}
              autoComplete="off"
            />
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            variant="destructive"
            disabled={confirmName !== project.name || loading}
            onClick={onConfirm}
          >
            {loading ? "Deleting..." : "Delete Project"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
