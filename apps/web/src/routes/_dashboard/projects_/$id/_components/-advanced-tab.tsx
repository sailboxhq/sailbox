import {
  Activity,
  Cpu,
  ExternalLink,
  Gauge,
  Hammer,
  Info,
  Minus,
  Network,
  Plus,
  Save,
  Server,
  Settings2,
  Shield,
  Timer,
  Trash2,
  X,
  Zap,
} from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { DangerZone } from "@/components/danger-zone";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
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
import { useClearBuildCache, useScaleApp, useUpdateApp } from "@/hooks/use-apps";
import { useNodePools } from "@/hooks/use-cluster";
import {
  COMMON_PORTS,
  CPU_OPTIONS,
  FAILURE_THRESHOLD_OPTIONS,
  GRACE_PERIOD_OPTIONS,
  HEALTH_CHECK_PATHS,
  HPA_MAX_REPLICAS,
  HPA_MIN_REPLICAS,
  HPA_TARGET_OPTIONS,
  INITIAL_DELAY_OPTIONS,
  MEMORY_OPTIONS,
  PERIOD_OPTIONS,
  SURGE_OPTIONS,
  TIMEOUT_OPTIONS,
} from "@/lib/constants";
import type { App, PortMapping } from "@/types/api";

// ── Shared UI ────────────────────────────────────────────────────

function SectionCard({
  icon: Icon,
  title,
  description,
  dirty,
  saving,
  onSave,
  children,
}: {
  icon: React.ElementType;
  title: string;
  description: string;
  dirty?: boolean;
  saving?: boolean;
  onSave?: () => void;
  children: React.ReactNode;
}) {
  return (
    <Card className={dirty ? "border-primary/50" : ""}>
      <CardHeader className="flex flex-row items-start justify-between gap-4">
        <div className="flex items-start gap-3">
          <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary/10">
            <Icon className="h-4 w-4 text-primary" />
          </div>
          <div>
            <CardTitle className="text-sm">{title}</CardTitle>
            <CardDescription className="text-xs">{description}</CardDescription>
          </div>
        </div>
        {onSave && (
          <Button
            size="sm"
            onClick={onSave}
            disabled={saving || !dirty}
            variant={dirty ? "default" : "outline"}
          >
            <Save className="h-3.5 w-3.5" /> {saving ? "Saving..." : dirty ? "Save" : "Saved"}
          </Button>
        )}
      </CardHeader>
      <CardContent>{children}</CardContent>
    </Card>
  );
}

function InfoBanner({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex items-center gap-2 rounded-lg border border-blue-500/20 bg-blue-500/5 px-3 py-2 text-xs text-blue-600 dark:text-blue-400">
      <Info className="h-3.5 w-3.5 shrink-0" />
      {children}
    </div>
  );
}

// ── 1. Health Checks ─────────────────────────────────────────────

function HealthCheckCard({ app, appId }: { app: App; appId: string }) {
  const updateApp = useUpdateApp(appId);
  const hc = app.health_check;
  const [type, setType] = useState(hc?.type || "");
  const [path, setPath] = useState(hc?.path || "/healthz");
  const [port, setPort] = useState(String(hc?.port || "8080"));
  const [command, setCommand] = useState(hc?.command || "");
  const [initialDelay, setInitialDelay] = useState(String(hc?.initial_delay_seconds || 0));
  const [period, setPeriod] = useState(String(hc?.period_seconds || 10));
  const [timeout, setTimeoutVal] = useState(String(hc?.timeout_seconds || 3));
  const [failureThreshold, setFailureThreshold] = useState(String(hc?.failure_threshold || 3));

  const dirty =
    type !== (hc?.type || "") ||
    path !== (hc?.path || "/healthz") ||
    port !== String(hc?.port || "8080") ||
    command !== (hc?.command || "") ||
    initialDelay !== String(hc?.initial_delay_seconds || 0) ||
    period !== String(hc?.period_seconds || 10) ||
    timeout !== String(hc?.timeout_seconds || 3) ||
    failureThreshold !== String(hc?.failure_threshold || 3);

  function handleSave() {
    updateApp.mutate({
      health_check: {
        type,
        path,
        port: Number(port),
        command,
        initial_delay_seconds: Number(initialDelay),
        period_seconds: Number(period),
        timeout_seconds: Number(timeout),
        failure_threshold: Number(failureThreshold),
      },
    });
  }

  return (
    <SectionCard
      icon={Shield}
      title="Health Checks"
      description="Configure liveness and readiness probes to monitor your application."
      dirty={dirty}
      saving={updateApp.isPending}
      onSave={handleSave}
    >
      <div className="space-y-4">
        <div className="space-y-1.5">
          <Label className="text-xs">Probe Type</Label>
          <Select value={type || "none"} onValueChange={(v) => setType(v === "none" ? "" : v)}>
            <SelectTrigger className="w-48">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="none">None (disabled)</SelectItem>
              <SelectItem value="http">HTTP GET</SelectItem>
              <SelectItem value="tcp">TCP Socket</SelectItem>
              <SelectItem value="exec">Exec Command</SelectItem>
            </SelectContent>
          </Select>
        </div>

        {type === "http" && (
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-1.5">
              <Label className="text-xs">Path</Label>
              <Select value={path} onValueChange={setPath}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {HEALTH_CHECK_PATHS.map((p) => (
                    <SelectItem key={p.value} value={p.value}>
                      {p.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label className="text-xs">Port</Label>
              <Select value={port} onValueChange={setPort}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {COMMON_PORTS.map((p) => (
                    <SelectItem key={p.value} value={p.value}>
                      {p.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
        )}

        {type === "tcp" && (
          <div className="space-y-1.5">
            <Label className="text-xs">Port</Label>
            <Select value={port} onValueChange={setPort}>
              <SelectTrigger className="w-48">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {COMMON_PORTS.map((p) => (
                  <SelectItem key={p.value} value={p.value}>
                    {p.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        )}

        {type === "exec" && (
          <div className="space-y-1.5">
            <Label className="text-xs">Command</Label>
            <Input
              value={command}
              onChange={(e) => setCommand(e.target.value)}
              placeholder="cat /tmp/healthy"
              className="font-mono text-sm"
            />
          </div>
        )}

        {type && type !== "none" && (
          <>
            <Separator />
            <div className="grid gap-4 sm:grid-cols-4">
              <div className="space-y-1.5">
                <Label className="text-xs">Initial Delay</Label>
                <Select value={initialDelay} onValueChange={setInitialDelay}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {INITIAL_DELAY_OPTIONS.map((o) => (
                      <SelectItem key={o.value} value={o.value}>
                        {o.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs">Period</Label>
                <Select value={period} onValueChange={setPeriod}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {PERIOD_OPTIONS.map((o) => (
                      <SelectItem key={o.value} value={o.value}>
                        {o.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs">Timeout</Label>
                <Select value={timeout} onValueChange={setTimeoutVal}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {TIMEOUT_OPTIONS.map((o) => (
                      <SelectItem key={o.value} value={o.value}>
                        {o.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs">Failures</Label>
                <Select value={failureThreshold} onValueChange={setFailureThreshold}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {FAILURE_THRESHOLD_OPTIONS.map((o) => (
                      <SelectItem key={o.value} value={o.value}>
                        {o.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>
          </>
        )}
      </div>
    </SectionCard>
  );
}

// ── 2. Autoscaling ───────────────────────────────────────────────

function AutoscalingCard({ app, appId }: { app: App; appId: string }) {
  const updateApp = useUpdateApp(appId);
  const as = app.autoscaling;
  const [enabled, setEnabled] = useState(as?.enabled || false);
  const [minReplicas, setMinReplicas] = useState(String(as?.min_replicas || 1));
  const [maxReplicas, setMaxReplicas] = useState(String(as?.max_replicas || 10));
  const [cpuTarget, setCpuTarget] = useState(String(as?.cpu_target || 80));
  const [memTarget, setMemTarget] = useState(String(as?.mem_target || 0));

  const dirty =
    enabled !== (as?.enabled || false) ||
    minReplicas !== String(as?.min_replicas || 1) ||
    maxReplicas !== String(as?.max_replicas || 10) ||
    cpuTarget !== String(as?.cpu_target || 80) ||
    memTarget !== String(as?.mem_target || 0);

  function handleSave() {
    updateApp.mutate({
      autoscaling: {
        enabled,
        min_replicas: Number(minReplicas),
        max_replicas: Number(maxReplicas),
        cpu_target: Number(cpuTarget),
        mem_target: Number(memTarget),
      },
    });
  }

  return (
    <SectionCard
      icon={Zap}
      title="Autoscaling (HPA)"
      description="Automatically scale pods based on CPU or memory usage."
      dirty={dirty}
      saving={updateApp.isPending}
      onSave={handleSave}
    >
      <div className="space-y-4">
        <label className="flex items-center gap-2 text-sm font-medium">
          <input
            type="checkbox"
            checked={enabled}
            onChange={(e) => setEnabled(e.target.checked)}
            className="rounded"
          />
          Enable Horizontal Pod Autoscaler
        </label>

        {enabled && (
          <>
            <InfoBanner>Scaling is managed by HPA. Manual scaling will be disabled.</InfoBanner>
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
              <div className="space-y-1.5">
                <Label className="text-xs">Min Replicas</Label>
                <Select value={minReplicas} onValueChange={setMinReplicas}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {HPA_MIN_REPLICAS.map((o) => (
                      <SelectItem key={o.value} value={o.value}>
                        {o.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs">Max Replicas</Label>
                <Select value={maxReplicas} onValueChange={setMaxReplicas}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {HPA_MAX_REPLICAS.map((o) => (
                      <SelectItem key={o.value} value={o.value}>
                        {o.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs">CPU Target</Label>
                <Select value={cpuTarget} onValueChange={setCpuTarget}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {HPA_TARGET_OPTIONS.filter((o) => o.value !== "0").map((o) => (
                      <SelectItem key={o.value} value={o.value}>
                        {o.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs">Memory Target</Label>
                <Select value={memTarget} onValueChange={setMemTarget}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {HPA_TARGET_OPTIONS.map((o) => (
                      <SelectItem key={o.value} value={o.value}>
                        {o.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>
          </>
        )}
      </div>
    </SectionCard>
  );
}

// ── 3. Resources ─────────────────────────────────────────────────

function ResourcesCard({ app, appId }: { app: App; appId: string }) {
  const updateApp = useUpdateApp(appId);
  const [cpuReq, setCpuReq] = useState(app.cpu_request || "250m");
  const [cpuLim, setCpuLim] = useState(app.cpu_limit || "500m");
  const [memReq, setMemReq] = useState(app.mem_request || "256Mi");
  const [memLim, setMemLim] = useState(app.mem_limit || "512Mi");

  const dirty =
    cpuReq !== (app.cpu_request || "250m") ||
    cpuLim !== (app.cpu_limit || "500m") ||
    memReq !== (app.mem_request || "256Mi") ||
    memLim !== (app.mem_limit || "512Mi");

  function handleSave() {
    updateApp.mutate({
      cpu_request: cpuReq,
      cpu_limit: cpuLim,
      mem_request: memReq,
      mem_limit: memLim,
    });
  }

  function ResourceSelect({
    label,
    value,
    onChange,
    options,
  }: {
    label: string;
    value: string;
    onChange: (v: string) => void;
    options: { value: string; label: string }[];
  }) {
    return (
      <div className="space-y-1.5">
        <Label className="text-xs">{label}</Label>
        <Select value={value} onValueChange={onChange}>
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {options.map((o) => (
              <SelectItem key={o.value} value={o.value}>
                {o.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
    );
  }

  return (
    <SectionCard
      icon={Cpu}
      title="Resources"
      description="Set CPU and memory requests (guaranteed) and limits (maximum)."
      dirty={dirty}
      saving={updateApp.isPending}
      onSave={handleSave}
    >
      <div className="grid gap-4 sm:grid-cols-2">
        <div className="space-y-3 rounded-lg border p-3">
          <p className="text-xs font-medium text-muted-foreground">Request (guaranteed)</p>
          <ResourceSelect label="CPU" value={cpuReq} onChange={setCpuReq} options={CPU_OPTIONS} />
          <ResourceSelect
            label="Memory"
            value={memReq}
            onChange={setMemReq}
            options={MEMORY_OPTIONS}
          />
        </div>
        <div className="space-y-3 rounded-lg border p-3">
          <p className="text-xs font-medium text-muted-foreground">Limit (maximum)</p>
          <ResourceSelect label="CPU" value={cpuLim} onChange={setCpuLim} options={CPU_OPTIONS} />
          <ResourceSelect
            label="Memory"
            value={memLim}
            onChange={setMemLim}
            options={MEMORY_OPTIONS}
          />
        </div>
      </div>
    </SectionCard>
  );
}

// ── 4. Port Configuration ────────────────────────────────────────

function PortsCard({ app, appId }: { app: App; appId: string }) {
  const updateApp = useUpdateApp(appId);
  const [ports, setPorts] = useState<PortMapping[]>(
    app.ports?.length
      ? app.ports
      : [{ container_port: 3000, service_port: 3000, protocol: "tcp" as const }],
  );

  const serialize = (p: PortMapping[]) =>
    p.map((x) => `${x.container_port}:${x.service_port}:${x.protocol}`).join(",");
  const dirty = serialize(ports) !== serialize(app.ports || []);

  function updatePort(index: number, field: keyof PortMapping, value: string | number) {
    setPorts(
      ports.map((p, i) =>
        i === index ? { ...p, [field]: typeof p[field] === "number" ? Number(value) : value } : p,
      ),
    );
  }

  function addPort() {
    setPorts([...ports, { container_port: 8080, service_port: 80, protocol: "tcp" }]);
  }

  function handleSave() {
    const valid = ports.filter((p) => p.container_port > 0 && p.service_port > 0);
    if (valid.length === 0) {
      toast.error("At least one port mapping is required");
      return;
    }
    updateApp.mutate({ ports: valid });
  }

  return (
    <SectionCard
      icon={Network}
      title="Port Configuration"
      description="Map container ports to service ports for external access."
      dirty={dirty}
      saving={updateApp.isPending}
      onSave={handleSave}
    >
      <div className="space-y-3">
        {ports.map((p, i) => (
          <div key={`${p.container_port}-${p.service_port}-${i}`} className="flex items-end gap-3">
            <div className="space-y-1.5">
              {i === 0 && <Label className="text-xs">Container Port</Label>}
              <Input
                type="number"
                min={1}
                max={65535}
                value={p.container_port || ""}
                onChange={(e) => updatePort(i, "container_port", e.target.value)}
                placeholder="3000"
                className="w-28 font-mono text-xs"
              />
            </div>
            <span className="mb-2 text-muted-foreground">&rarr;</span>
            <div className="space-y-1.5">
              {i === 0 && <Label className="text-xs">Service Port</Label>}
              <Input
                type="number"
                min={1}
                max={65535}
                value={p.service_port || ""}
                onChange={(e) => updatePort(i, "service_port", e.target.value)}
                placeholder="3000"
                className="w-28 font-mono text-xs"
              />
            </div>
            <div className="space-y-1.5">
              {i === 0 && <Label className="text-xs">Protocol</Label>}
              <Select value={p.protocol} onValueChange={(v) => updatePort(i, "protocol", v)}>
                <SelectTrigger className="w-20">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="tcp">TCP</SelectItem>
                  <SelectItem value="udp">UDP</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <Button
              size="icon"
              variant="ghost"
              className="mb-0.5 h-8 w-8 shrink-0 text-destructive"
              onClick={() => setPorts(ports.filter((_, j) => j !== i))}
              disabled={ports.length <= 1}
            >
              <Trash2 className="h-3.5 w-3.5" />
            </Button>
          </div>
        ))}

        <Button size="sm" variant="outline" onClick={addPort}>
          <Plus className="h-3.5 w-3.5" /> Add Port
        </Button>
      </div>
    </SectionCard>
  );
}

// ── 6. Source Provider ───────────────────────────────────────────

function SourceProviderCard({ app, appId }: { app: App; appId: string }) {
  const updateApp = useUpdateApp(appId);
  const isGit = app.source_type === "git";

  const [dockerImage, setDockerImage] = useState(app.docker_image || "");
  const [gitRepo, setGitRepo] = useState(app.git_repo || "");
  const [gitBranch, setGitBranch] = useState(app.git_branch || "main");

  const imageDirty = dockerImage !== (app.docker_image || "");
  const repoDirty = gitRepo !== (app.git_repo || "") || gitBranch !== (app.git_branch || "main");
  const dirty = imageDirty || repoDirty;

  function handleSave() {
    if (isGit) {
      updateApp.mutate({ git_repo: gitRepo, git_branch: gitBranch });
    } else {
      updateApp.mutate({ docker_image: dockerImage });
    }
  }

  return (
    <SectionCard
      icon={Settings2}
      title="Source Provider"
      description="Where your application code comes from."
      dirty={dirty}
      saving={updateApp.isPending}
      onSave={handleSave}
    >
      {/* Source type indicator */}
      <div className="mb-4 inline-flex rounded-lg border bg-muted p-0.5">
        <span
          className={`rounded-md px-3 py-1 text-xs font-medium ${!isGit ? "bg-background text-foreground shadow-sm" : "text-muted-foreground"}`}
        >
          Docker Image
        </span>
        <span
          className={`rounded-md px-3 py-1 text-xs font-medium ${isGit ? "bg-background text-foreground shadow-sm" : "text-muted-foreground"}`}
        >
          Git Repository
        </span>
      </div>

      {isGit ? (
        <div className="space-y-3">
          <div className="space-y-1.5">
            <Label className="text-xs">Repository</Label>
            <div className="flex items-center gap-2">
              <Input
                value={gitRepo}
                onChange={(e) => setGitRepo(e.target.value)}
                placeholder="https://github.com/org/repo"
                className="flex-1 font-mono text-xs"
              />
              {app.git_repo && (
                <a href={app.git_repo} target="_blank" rel="noopener noreferrer">
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-8 w-8 shrink-0"
                    title="Open repository"
                  >
                    <ExternalLink className="h-3.5 w-3.5" />
                  </Button>
                </a>
              )}
            </div>
          </div>
          <div className="space-y-1.5">
            <Label className="text-xs">Branch</Label>
            <Select value={gitBranch} onValueChange={setGitBranch}>
              <SelectTrigger className="w-48">
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
          <div className="flex items-center justify-between gap-4 py-2">
            <span className="text-sm text-muted-foreground">Auto Deploy</span>
            <Badge variant={app.auto_deploy ? "success" : "secondary"}>
              {app.auto_deploy ? "Enabled" : "Disabled"}
            </Badge>
          </div>
        </div>
      ) : (
        <div className="space-y-1.5">
          <Label className="text-xs">Image</Label>
          <Input
            value={dockerImage}
            onChange={(e) => setDockerImage(e.target.value)}
            placeholder="nginx:latest"
            className="font-mono text-xs"
          />
        </div>
      )}
    </SectionCard>
  );
}

// ── 6b. Build Configuration ─────────────────────────────────────

function BuildConfigCard({ app, appId }: { app: App; appId: string }) {
  const updateApp = useUpdateApp(appId);
  const clearCache = useClearBuildCache(appId);
  const [buildType, setBuildType] = useState(app.build_type || "dockerfile");
  const [dockerfile, setDockerfile] = useState(app.dockerfile || "Dockerfile");
  const [buildContext, setBuildContext] = useState(app.build_context || ".");
  const [watchPaths, setWatchPaths] = useState<string[]>(app.watch_paths || []);
  const [noCache, setNoCache] = useState(app.no_cache || false);
  const [newPath, setNewPath] = useState("");

  const dirty =
    buildType !== (app.build_type || "dockerfile") ||
    dockerfile !== (app.dockerfile || "Dockerfile") ||
    buildContext !== (app.build_context || ".") ||
    noCache !== (app.no_cache || false) ||
    JSON.stringify(watchPaths) !== JSON.stringify(app.watch_paths || []);

  const addWatchPath = () => {
    const trimmed = newPath.trim();
    if (trimmed && !watchPaths.includes(trimmed)) {
      setWatchPaths([...watchPaths, trimmed]);
      setNewPath("");
    }
  };

  const removeWatchPath = (path: string) => {
    setWatchPaths(watchPaths.filter((p) => p !== path));
  };

  function handleSave() {
    updateApp.mutate({
      build_type: buildType,
      dockerfile: buildType === "dockerfile" ? dockerfile : "",
      build_context: buildContext,
      watch_paths: watchPaths,
      no_cache: noCache,
    });
  }

  return (
    <SectionCard
      icon={Hammer}
      title="Build Configuration"
      description="How your code is built into a container image."
      dirty={dirty}
      saving={updateApp.isPending}
      onSave={handleSave}
    >
      <div className="space-y-4">
        {/* Build Type — visual selector */}
        <div className="space-y-1.5">
          <Label className="text-xs">Build Type</Label>
          <div className="grid grid-cols-2 gap-4">
            {[
              { value: "dockerfile", label: "Dockerfile", desc: "Use your own Dockerfile" },
              { value: "nixpacks", label: "Nixpacks", desc: "Auto-detect language & build" },
            ].map((opt) => (
              <button
                key={opt.value}
                type="button"
                onClick={() => setBuildType(opt.value)}
                className={`rounded-lg border p-3 text-left transition-colors ${
                  buildType === opt.value ? "border-primary bg-primary/5" : "hover:bg-accent"
                }`}
              >
                <p
                  className={`text-sm font-medium ${buildType === opt.value ? "text-primary" : ""}`}
                >
                  {opt.label}
                </p>
                <p className="text-xs text-muted-foreground">{opt.desc}</p>
              </button>
            ))}
          </div>
        </div>

        {/* Dockerfile path */}
        {buildType === "dockerfile" && (
          <div className="space-y-1.5">
            <Label className="text-xs">Dockerfile Path</Label>
            <Input
              value={dockerfile}
              onChange={(e) => setDockerfile(e.target.value)}
              placeholder="Dockerfile"
              className="w-64 font-mono text-xs"
            />
            <p className="text-xs text-muted-foreground">
              Relative to build context, e.g. Dockerfile, Dockerfile.prod, docker/Dockerfile
            </p>
          </div>
        )}

        {buildType === "nixpacks" && (
          <InfoBanner>
            Nixpacks automatically detects your language and builds the optimal image. No
            configuration needed.
          </InfoBanner>
        )}

        {/* Build Context */}
        <div className="space-y-1.5">
          <Label className="text-xs">Build Context</Label>
          <Input
            value={buildContext}
            onChange={(e) => setBuildContext(e.target.value)}
            placeholder="/ (repository root)"
            className="w-64 font-mono text-xs"
          />
          <p className="text-xs text-muted-foreground">
            Leave empty for repository root. For monorepos, e.g. apps/api, packages/web
          </p>
        </div>

        {/* Watch Paths */}
        <div className="space-y-1.5">
          <Label className="text-xs">Watch Paths</Label>
          <p className="text-xs text-muted-foreground">
            Only trigger auto-deploy when files in these paths change. Leave empty to deploy on any
            change.
          </p>
          {watchPaths.length > 0 && (
            <div className="flex flex-wrap gap-1.5">
              {watchPaths.map((p) => (
                <span
                  key={p}
                  className="inline-flex items-center gap-1 rounded-md border bg-muted px-2 py-0.5 font-mono text-xs"
                >
                  {p}
                  <button
                    type="button"
                    onClick={() => removeWatchPath(p)}
                    className="text-muted-foreground hover:text-foreground"
                  >
                    <X className="h-3 w-3" />
                  </button>
                </span>
              ))}
            </div>
          )}
          <div className="flex items-center gap-2">
            <Input
              value={newPath}
              onChange={(e) => setNewPath(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  e.preventDefault();
                  addWatchPath();
                }
              }}
              placeholder="apps/api/**, *.go, src/"
              className="w-64 font-mono text-xs"
            />
            <Button
              type="button"
              variant="outline"
              size="icon"
              className="h-8 w-8 shrink-0"
              onClick={addWatchPath}
              disabled={!newPath.trim()}
            >
              <Plus className="h-3.5 w-3.5" />
            </Button>
          </div>
        </div>

        {/* Build Cache */}
        <Separator />
        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm font-medium">Build Cache</p>
            <p className="text-xs text-muted-foreground">
              Disable to force a clean build every time. Slower but avoids stale layers.
            </p>
          </div>
          <div className="flex items-center gap-3">
            <Button
              size="sm"
              variant="outline"
              onClick={() => clearCache.mutate(undefined)}
              disabled={clearCache.isPending}
            >
              <Trash2 className="h-3.5 w-3.5" />{" "}
              {clearCache.isPending ? "Clearing..." : "Clear Cache"}
            </Button>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={!noCache}
                onChange={(e) => setNoCache(!e.target.checked)}
                className="rounded"
              />
              Enabled
            </label>
          </div>
        </div>
      </div>
    </SectionCard>
  );
}

// ── 7. Deployment Strategy ───────────────────────────────────────

function DeployStrategyCard({ app, appId }: { app: App; appId: string }) {
  const updateApp = useUpdateApp(appId);
  const [strategy, setStrategy] = useState(app.deploy_strategy || "rolling");
  const [maxSurge, setMaxSurge] = useState(app.deploy_strategy_config?.max_surge || "25%");
  const [maxUnavailable, setMaxUnavailable] = useState(
    app.deploy_strategy_config?.max_unavailable || "25%",
  );

  const dirty =
    strategy !== (app.deploy_strategy || "rolling") ||
    maxSurge !== (app.deploy_strategy_config?.max_surge || "25%") ||
    maxUnavailable !== (app.deploy_strategy_config?.max_unavailable || "25%");

  function handleSave() {
    updateApp.mutate({
      deploy_strategy: strategy,
      deploy_strategy_config:
        strategy === "rolling"
          ? { max_surge: maxSurge, max_unavailable: maxUnavailable }
          : { max_surge: "", max_unavailable: "" },
    });
  }

  return (
    <SectionCard
      icon={Activity}
      title="Deployment Strategy"
      description="Control how new versions are rolled out."
      dirty={dirty}
      saving={updateApp.isPending}
      onSave={handleSave}
    >
      <div className="space-y-4">
        <div className="space-y-1.5">
          <Label className="text-xs">Strategy</Label>
          <Select value={strategy} onValueChange={setStrategy}>
            <SelectTrigger className="w-56">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="rolling">Rolling Update (zero downtime)</SelectItem>
              <SelectItem value="recreate">Recreate (brief downtime)</SelectItem>
            </SelectContent>
          </Select>
        </div>
        {strategy === "rolling" && (
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-1.5">
              <Label className="text-xs">Max Surge</Label>
              <Select value={maxSurge} onValueChange={setMaxSurge}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {SURGE_OPTIONS.map((o) => (
                    <SelectItem key={o.value} value={o.value}>
                      {o.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label className="text-xs">Max Unavailable</Label>
              <Select value={maxUnavailable} onValueChange={setMaxUnavailable}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {SURGE_OPTIONS.map((o) => (
                    <SelectItem key={o.value} value={o.value}>
                      {o.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
        )}
      </div>
    </SectionCard>
  );
}

// ── 8. Graceful Termination ──────────────────────────────────────

function TerminationCard({ app, appId }: { app: App; appId: string }) {
  const updateApp = useUpdateApp(appId);
  const [seconds, setSeconds] = useState(String(app.termination_grace_period || 30));

  const dirty = seconds !== String(app.termination_grace_period || 30);

  function handleSave() {
    updateApp.mutate({ termination_grace_period: Number(seconds) });
  }

  return (
    <SectionCard
      icon={Timer}
      title="Graceful Termination"
      description="Time to wait for the process to exit before force-killing."
      dirty={dirty}
      saving={updateApp.isPending}
      onSave={handleSave}
    >
      <Select value={seconds} onValueChange={setSeconds}>
        <SelectTrigger className="w-56">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {GRACE_PERIOD_OPTIONS.map((o) => (
            <SelectItem key={o.value} value={o.value}>
              {o.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </SectionCard>
  );
}

// ── 9. Manual Scaling ────────────────────────────────────────────

function ScalingSection({
  appId,
  currentReplicas,
  hpaEnabled,
}: {
  appId: string;
  currentReplicas: number;
  hpaEnabled: boolean;
}) {
  const scale = useScaleApp(appId);
  const [input, setInput] = useState(currentReplicas);

  return (
    <SectionCard
      icon={Gauge}
      title="Manual Scaling"
      description="Set the number of running pod replicas."
    >
      {hpaEnabled ? (
        <InfoBanner>Autoscaling (HPA) is enabled. Manual scaling is disabled.</InfoBanner>
      ) : (
        <div className="flex items-center gap-3">
          <Button
            size="icon"
            variant="outline"
            className="h-8 w-8"
            onClick={() => setInput(Math.max(0, input - 1))}
          >
            <Minus className="h-3 w-3" />
          </Button>
          <Input
            type="number"
            min={0}
            max={100}
            value={input}
            onChange={(e) => setInput(Number.parseInt(e.target.value, 10) || 0)}
            className="w-20 text-center"
          />
          <Button
            size="icon"
            variant="outline"
            className="h-8 w-8"
            onClick={() => setInput(input + 1)}
          >
            <Plus className="h-3 w-3" />
          </Button>
          <Button
            onClick={() => scale.mutate(input)}
            disabled={scale.isPending || input === currentReplicas}
            size="sm"
          >
            {scale.isPending ? "Scaling..." : "Apply"}
          </Button>
        </div>
      )}
    </SectionCard>
  );
}

// ── Main export ──────────────────────────────────────────────────

// ── Node Pool ─────────────────────────────────────────────────────

function NodePoolCard({ app, appId }: { app: App; appId: string }) {
  const updateApp = useUpdateApp(appId);
  const { data: pools } = useNodePools();
  const [nodePool, setNodePool] = useState(app.node_pool || "");
  const dirty = nodePool !== (app.node_pool || "");

  return (
    <SectionCard
      icon={Server}
      title="Node Pool"
      description="Deploy to a specific group of nodes."
      dirty={dirty}
      saving={updateApp.isPending}
      onSave={() => updateApp.mutate({ node_pool: nodePool })}
    >
      <div className="space-y-2">
        <Select value={nodePool || "any"} onValueChange={(v) => setNodePool(v === "any" ? "" : v)}>
          <SelectTrigger className="w-56">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="any">Any node (default)</SelectItem>
            {pools?.map((p) => (
              <SelectItem key={p} value={p}>
                {p}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <p className="text-xs text-muted-foreground">
          Assign node pools via Cluster → Nodes. Only nodes with the matching{" "}
          <code className="rounded bg-muted px-1">sailbox/pool</code> label will be selected.
        </p>
      </div>
    </SectionCard>
  );
}

export function SettingsTab({
  app,
  appId,
  onDelete,
}: {
  app: App;
  appId: string;
  onDelete: () => void;
}) {
  return (
    <div className="space-y-6">
      {/* Source & Build — top of settings */}
      <SourceProviderCard app={app} appId={appId} />
      {app.source_type === "git" && <BuildConfigCard app={app} appId={appId} />}

      {/* Runtime configuration */}
      <ResourcesCard app={app} appId={appId} />
      <PortsCard app={app} appId={appId} />
      <HealthCheckCard app={app} appId={appId} />
      <DeployStrategyCard app={app} appId={appId} />
      <AutoscalingCard app={app} appId={appId} />
      <ScalingSection
        appId={appId}
        currentReplicas={app.replicas}
        hpaEnabled={app.autoscaling?.enabled || false}
      />
      <NodePoolCard app={app} appId={appId} />
      <TerminationCard app={app} appId={appId} />
      <DangerZone
        description="Delete this application and all K3s resources."
        buttonLabel="Delete"
        onDelete={onDelete}
      />
    </div>
  );
}
