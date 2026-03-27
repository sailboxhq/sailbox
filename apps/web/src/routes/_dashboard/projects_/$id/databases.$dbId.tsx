import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import {
  AlertTriangle,
  Copy,
  Database,
  Eye,
  EyeOff,
  Globe,
  HardDrive,
  Loader2,
  RotateCcw,
  Save,
  Shield,
} from "lucide-react";
import { useEffect, useState } from "react";
import { toast } from "sonner";
import { DangerZone } from "@/components/danger-zone";
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
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ToggleSwitch } from "@/components/ui/toggle-switch";
import {
  useDatabase,
  useDatabaseBackups,
  useDatabaseCredentials,
  useDatabasePods,
  useDatabaseStatus,
  useDeleteDatabase,
  useRestoreBackup,
  useTriggerBackup,
  useUpdateBackupConfig,
  useUpdateExternalAccess,
  useUsedPorts,
} from "@/hooks/use-databases";
import { useResources } from "@/hooks/use-resources";
import { ENGINE_LABELS, statusDotColor, statusVariant } from "@/lib/constants";
import type { DatabaseBackup, ManagedDB, PodInfo, SharedResource } from "@/types/api";

export const Route = createFileRoute("/_dashboard/projects_/$id/databases/$dbId")({
  component: DatabaseDetailPage,
});

// ── Helpers ──────────────────────────────────────────────────────────

function copyToClipboard(text: string) {
  navigator.clipboard.writeText(text);
  toast.success("Copied");
}

// Format K8s CPU values (nanocores/millicores) to human-readable
function formatCPU(raw: string): string {
  if (!raw) return "0m";
  if (raw.endsWith("n")) {
    const n = Number.parseInt(raw.slice(0, -1), 10);
    return `${Math.round(n / 1_000_000)}m`;
  }
  if (raw.endsWith("m")) return raw;
  // Plain number = cores
  return `${Math.round(Number.parseFloat(raw) * 1000)}m`;
}

// Format K8s memory values (Ki/Mi/Gi) to human-readable
function formatMem(raw: string): string {
  if (!raw) return "0Mi";
  if (raw.endsWith("Ki")) {
    const ki = Number.parseInt(raw.slice(0, -2), 10);
    return ki >= 1024 ? `${(ki / 1024).toFixed(0)}Mi` : `${ki}Ki`;
  }
  if (raw.endsWith("Mi") || raw.endsWith("Gi")) return raw;
  // Plain bytes
  const b = Number.parseInt(raw, 10);
  if (b >= 1024 * 1024 * 1024) return `${(b / (1024 * 1024 * 1024)).toFixed(1)}Gi`;
  if (b >= 1024 * 1024) return `${Math.round(b / (1024 * 1024))}Mi`;
  return `${Math.round(b / 1024)}Ki`;
}

function formatBytes(bytes: number): string {
  if (!bytes || bytes === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / 1024 ** i).toFixed(1)} ${units[i]}`;
}

function formatDuration(start?: string, end?: string): string {
  if (!start) return "-";
  const s = new Date(start).getTime();
  const e = end ? new Date(end).getTime() : Date.now();
  const secs = Math.round((e - s) / 1000);
  if (secs < 60) return `${secs}s`;
  return `${Math.floor(secs / 60)}m ${secs % 60}s`;
}

function backupStatusVariant(status: string): "warning" | "success" | "destructive" | "secondary" {
  switch (status) {
    case "pending":
    case "running":
      return "warning";
    case "completed":
      return "success";
    case "failed":
      return "destructive";
    default:
      return "secondary";
  }
}

// ── Credential Row ───────────────────────────────────────────────────

function CredentialRow({
  label,
  value,
  secret,
  mono,
}: {
  label: string;
  value: string;
  secret?: boolean;
  mono?: boolean;
}) {
  const [revealed, setRevealed] = useState(false);

  const display = secret && !revealed ? "••••••••••••" : value;

  return (
    <div className="flex items-center justify-between gap-4 py-2">
      <span className="shrink-0 text-sm text-muted-foreground">{label}</span>
      <div className="flex min-w-0 items-center gap-2">
        <span className={`min-w-0 truncate text-sm ${mono ? "font-mono text-xs" : ""}`}>
          {display}
        </span>
        {secret && (
          <Button
            size="icon"
            variant="ghost"
            className="h-7 w-7 shrink-0"
            aria-label="Toggle visibility"
            onClick={() => setRevealed(!revealed)}
          >
            {revealed ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
          </Button>
        )}
        <Button
          size="icon"
          variant="ghost"
          className="h-7 w-7 shrink-0"
          aria-label="Copy to clipboard"
          onClick={() => copyToClipboard(value)}
        >
          <Copy className="h-3.5 w-3.5" />
        </Button>
      </div>
    </div>
  );
}

// ── Pod Row (simplified for DB) ──────────────────────────────────────

function DbPodRow({ pod }: { pod: PodInfo }) {
  return (
    <div className="flex items-center justify-between rounded-md border px-3 py-2 text-sm">
      <div className="flex items-center gap-3">
        <Badge variant={statusVariant(pod.phase)} className="text-xs">
          {pod.phase}
        </Badge>
        <span className="font-mono text-xs">{pod.name}</span>
        <span
          className={`inline-block h-2 w-2 rounded-full ${pod.ready ? statusDotColor("running") : statusDotColor("error")}`}
          title={pod.ready ? "Ready" : "Not ready"}
        />
      </div>
      {pod.resources && (
        <div className="flex items-center gap-4 text-xs text-muted-foreground">
          <span>
            CPU: {formatCPU(pod.resources.cpu_used)}/{formatCPU(pod.resources.cpu_total)}
          </span>
          <span>
            Mem: {formatMem(pod.resources.mem_used)}/{formatMem(pod.resources.mem_total)}
          </span>
        </div>
      )}
    </div>
  );
}

// ── Backup Row ───────────────────────────────────────────────────────

function BackupRow({
  backup,
  onRestore,
  isRestoring,
  dbReady,
}: {
  backup: DatabaseBackup;
  onRestore: (backupId: string) => void;
  isRestoring: boolean;
  dbReady: boolean;
}) {
  const [showConfirm, setShowConfirm] = useState(false);
  const [confirmText, setConfirmText] = useState("");
  const restoreRunning = backup.restore_status === "running";

  return (
    <>
      <div className="flex items-center justify-between rounded-md border px-3 py-2 text-sm">
        <div className="flex items-center gap-3">
          <Badge variant={backupStatusVariant(backup.status)} className="text-xs">
            {backup.status}
          </Badge>
          {backup.restore_status && (
            <Badge
              variant={
                backup.restore_status === "completed"
                  ? "outline"
                  : backup.restore_status === "failed"
                    ? "destructive"
                    : "secondary"
              }
              className="text-xs"
            >
              {restoreRunning ? (
                <span className="flex items-center gap-1">
                  <Loader2 className="h-3 w-3 animate-spin" /> restoring
                </span>
              ) : (
                `restore ${backup.restore_status}`
              )}
            </Badge>
          )}
          <span className="text-xs text-muted-foreground">
            {new Date(backup.created_at).toLocaleString()}
          </span>
        </div>
        <div className="flex items-center gap-3 text-xs text-muted-foreground">
          <span>{formatBytes(backup.size_bytes)}</span>
          <span>{formatDuration(backup.started_at, backup.finished_at)}</span>
          {backup.status === "completed" && !restoreRunning && (
            <Button
              variant="ghost"
              size="sm"
              className="h-6 gap-1 px-2 text-xs"
              onClick={() => setShowConfirm(true)}
              disabled={isRestoring || !dbReady}
              title={!dbReady ? "Database must be running to restore" : "Restore from this backup"}
            >
              <RotateCcw className="h-3 w-3" /> Restore
            </Button>
          )}
        </div>
      </div>

      <Dialog
        open={showConfirm}
        onOpenChange={(open) => {
          if (!open) setConfirmText("");
          setShowConfirm(open);
        }}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>Restore Database</DialogTitle>
            <DialogDescription>
              This will overwrite the current database with the backup from{" "}
              <strong>{new Date(backup.created_at).toLocaleString()}</strong>. This action cannot be
              undone.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2 py-2">
            <div className="rounded-md border border-destructive/30 bg-destructive/5 p-3">
              <p className="text-xs text-destructive">
                All current data will be replaced with the backup contents.
              </p>
            </div>
            <Label>
              Type <strong>RESTORE</strong> to confirm
            </Label>
            <Input
              value={confirmText}
              onChange={(e) => setConfirmText(e.target.value)}
              placeholder="RESTORE"
              className="font-mono"
            />
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setShowConfirm(false)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              disabled={confirmText !== "RESTORE" || isRestoring}
              onClick={() => {
                onRestore(backup.id);
                setShowConfirm(false);
                setConfirmText("");
              }}
            >
              <RotateCcw className="h-3.5 w-3.5" /> Restore
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}

// ── Schedule presets ──────────────────────────────────────────────────

const BACKUP_SCHEDULE_PRESETS = [
  { value: "0 */6 * * *", label: "Every 6 hours" },
  { value: "0 2 * * *", label: "Daily at 2:00 AM (recommended)" },
  { value: "0 4 * * *", label: "Daily at 4:00 AM" },
  { value: "0 2 * * 0", label: "Weekly (Sunday 2 AM)" },
  { value: "custom", label: "Custom" },
] as const;

// ToggleSwitch imported from shared component

// ── External Access Card ─────────────────────────────────────────────

const ENGINE_PROTOCOL: Record<string, string> = {
  postgres: "postgresql",
  mysql: "mysql",
  mariadb: "mysql",
  redis: "redis",
  mongo: "mongodb",
};

// NodePort defaults — mapped to 3xxxx range to stay within K8s default 30000-32767
const ENGINE_DEFAULT_PORT: Record<string, number> = {
  postgres: 30432,
  mysql: 30306,
  mariadb: 30306,
  redis: 30379,
  mongo: 30017,
};

function ExternalAccessCard({ db }: { db: ManagedDB }) {
  const updateExternal = useUpdateExternalAccess(db.id);
  const { data: rawUsedPorts } = useUsedPorts();
  const usedPorts = rawUsedPorts ?? [];
  const externalHost = window.location.hostname;
  const protocol = ENGINE_PROTOCOL[db.engine] || db.engine;
  const defaultPort = ENGINE_DEFAULT_PORT[db.engine] || 30000;
  const [port, setPort] = useState(
    db.external_port > 0 ? String(db.external_port) : String(defaultPort),
  );

  const portNum = Number(port);
  const portInRange = !port || (portNum >= 30000 && portNum <= 32767);
  const portConflict = usedPorts.find((p) => p.port === portNum && p.database_id !== db.id);

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-sm font-medium">
          <Globe className="h-4 w-4" /> External Access
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <p className="text-sm text-muted-foreground">
          Allow external connections to this database via NodePort.
        </p>

        {!db.external_enabled && (
          <div className="space-y-3">
            <div className="space-y-2">
              <Label className="text-sm">Port</Label>
              <div className="flex items-center gap-3">
                <Input
                  type="number"
                  min={30000}
                  max={32767}
                  value={port}
                  onChange={(e) => setPort(e.target.value)}
                  placeholder={`e.g. ${defaultPort}`}
                  className="max-w-[200px] font-mono"
                />
                <Button
                  onClick={() =>
                    updateExternal.mutate({
                      enabled: true,
                      port: port ? Number(port) : undefined,
                    })
                  }
                  disabled={updateExternal.isPending || !port || !portInRange || !!portConflict}
                >
                  {updateExternal.isPending ? (
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <Globe className="h-3.5 w-3.5" />
                  )}
                  Enable
                </Button>
              </div>
              {port && !portInRange && (
                <p className="text-xs text-destructive">
                  Port must be between 30000–32767 (K8s NodePort range)
                </p>
              )}
              {portConflict && (
                <p className="text-xs text-destructive">
                  Port {portNum} is already used by &quot;{portConflict.database_name}&quot; (
                  {portConflict.engine})
                </p>
              )}
              <p className="text-xs text-muted-foreground">
                NodePort range: 30000–32767
                {usedPorts.length > 0 && (
                  <>
                    {" "}
                    &middot; In use:{" "}
                    {usedPorts.map((p) => `${p.port} (${p.database_name})`).join(", ")}
                  </>
                )}
              </p>
            </div>
          </div>
        )}

        {db.external_enabled && db.external_port > 0 && (
          <div className="space-y-3">
            <div className="flex items-center justify-between gap-4 rounded-md border px-3 py-2">
              <span className="text-sm text-muted-foreground">External URL</span>
              <div className="flex items-center gap-2">
                <span className="truncate font-mono text-sm">
                  {protocol}://{externalHost}:{db.external_port}
                </span>
                <Button
                  size="icon"
                  variant="ghost"
                  className="h-7 w-7 shrink-0"
                  aria-label="Copy to clipboard"
                  onClick={() =>
                    copyToClipboard(`${protocol}://${externalHost}:${db.external_port}`)
                  }
                >
                  <Copy className="h-3.5 w-3.5" />
                </Button>
              </div>
            </div>

            <div className="flex items-center gap-2 rounded-md border border-yellow-500/30 bg-yellow-500/10 px-4 py-3 text-sm text-yellow-400">
              <AlertTriangle className="h-4 w-4 shrink-0" />
              Exposing database to public network. Ensure strong passwords are set.
            </div>

            <Button
              variant="outline"
              size="sm"
              onClick={() => updateExternal.mutate({ enabled: false })}
              disabled={updateExternal.isPending}
            >
              {updateExternal.isPending ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : null}
              Disable External Access
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

// ── Backup Configuration Card ────────────────────────────────────────

function BackupConfigCard({ db }: { db: ManagedDB }) {
  const updateBackup = useUpdateBackupConfig(db.id);
  const { data: resources } = useResources("object_storage");
  const s3Resources = (resources ?? []).filter((r: SharedResource) => r.type === "object_storage");

  const [enabled, setEnabled] = useState(db.backup_enabled);
  const [s3Id, setS3Id] = useState(db.backup_s3_id || "");
  const [schedulePreset, setSchedulePreset] = useState(() => {
    const match = BACKUP_SCHEDULE_PRESETS.find((p) => p.value === db.backup_schedule);
    return match ? match.value : db.backup_schedule ? "custom" : "0 2 * * *";
  });
  const [customCron, setCustomCron] = useState(
    BACKUP_SCHEDULE_PRESETS.some((p) => p.value === db.backup_schedule)
      ? ""
      : db.backup_schedule || "",
  );

  // Sync local state when db data changes
  useEffect(() => {
    setEnabled(db.backup_enabled);
    setS3Id(db.backup_s3_id || "");
    const match = BACKUP_SCHEDULE_PRESETS.find((p) => p.value === db.backup_schedule);
    setSchedulePreset(match ? match.value : db.backup_schedule ? "custom" : "0 2 * * *");
    if (!match && db.backup_schedule) {
      setCustomCron(db.backup_schedule);
    }
  }, [db.backup_enabled, db.backup_s3_id, db.backup_schedule]);

  const resolvedSchedule = schedulePreset === "custom" ? customCron : schedulePreset;

  // Dirty state detection: compare current form values with saved db values
  const isDirty =
    enabled !== db.backup_enabled ||
    resolvedSchedule !== (db.backup_schedule || "0 2 * * *") ||
    (s3Id || "") !== (db.backup_s3_id || "");

  function handleSave() {
    updateBackup.mutate({
      enabled,
      schedule: resolvedSchedule,
      s3_id: s3Id || undefined,
    });
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-sm font-medium">
          <Database className="h-4 w-4" /> Backup Configuration
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex items-center gap-3">
          <ToggleSwitch checked={enabled} onChange={setEnabled} />
          <span className="text-sm font-medium">Automatic Backups</span>
        </div>

        {enabled && (
          <div className="space-y-4">
            {/* S3 Destination */}
            <div className="space-y-2">
              <Label className="text-sm">Destination</Label>
              {s3Resources.length === 0 ? (
                <p className="text-sm text-muted-foreground">
                  No S3 storage configured.{" "}
                  <Link
                    to="/resources"
                    className="text-primary underline underline-offset-4 hover:text-primary/80"
                  >
                    Add one in Resources.
                  </Link>
                </p>
              ) : (
                <Select value={s3Id} onValueChange={setS3Id}>
                  <SelectTrigger>
                    <SelectValue placeholder="Select S3 resource" />
                  </SelectTrigger>
                  <SelectContent>
                    {s3Resources.map((r: SharedResource) => (
                      <SelectItem key={r.id} value={r.id}>
                        {r.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )}
            </div>

            {/* Schedule */}
            <div className="space-y-2">
              <Label className="text-sm">Schedule</Label>
              <Select value={schedulePreset} onValueChange={setSchedulePreset}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {BACKUP_SCHEDULE_PRESETS.map((p) => (
                    <SelectItem key={p.value} value={p.value}>
                      {p.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {schedulePreset === "custom" && (
                <Input
                  value={customCron}
                  onChange={(e) => setCustomCron(e.target.value)}
                  placeholder="0 */6 * * *"
                  className="font-mono"
                />
              )}
            </div>
          </div>
        )}

        {/* Save — always visible when dirty (including when disabling backups) */}
        {isDirty && (
          <Button
            size="sm"
            onClick={handleSave}
            disabled={updateBackup.isPending || (!resolvedSchedule && enabled)}
          >
            {updateBackup.isPending ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <Save className="h-3.5 w-3.5" />
            )}{" "}
            Save
          </Button>
        )}
      </CardContent>
    </Card>
  );
}

// ── Main Page ────────────────────────────────────────────────────────

function DatabaseDetailPage() {
  const { id: projectId, dbId } = Route.useParams();
  const navigate = useNavigate();

  // ── Data ──────────────────────────────────────────────────────
  const { data: db, isLoading } = useDatabase(dbId);
  const { data: dbStatus } = useDatabaseStatus(dbId);
  const livePhaseEarly = dbStatus?.phase ?? db?.status;
  const { data: credentials, isLoading: credsLoading } = useDatabaseCredentials(
    dbId,
    livePhaseEarly === "running",
  );
  const { data: rawPods } = useDatabasePods(dbId);
  const { data: rawBackups } = useDatabaseBackups(dbId);
  const pods = rawPods ?? [];
  const backups = rawBackups ?? [];

  // ── Mutations ─────────────────────────────────────────────────
  const deleteDb = useDeleteDatabase(dbId);
  const triggerBackup = useTriggerBackup(dbId);
  const restoreBackup = useRestoreBackup(dbId);

  // ── Local state ───────────────────────────────────────────────
  const [showDelete, setShowDelete] = useState(false);
  const [confirmName, setConfirmName] = useState("");
  const [showAllBackups, setShowAllBackups] = useState(false);

  if (isLoading) return <LoadingScreen variant="detail" />;
  if (!db) return null;

  const livePhase = livePhaseEarly ?? db.status;
  const isReady = livePhase === "running";
  const isStarting = livePhase === "pending" || livePhase === "creating";

  return (
    <div>
      {/* ── Header ── */}
      <PageHeader
        title={db.name}
        useBack
        description={
          <span className="flex items-center gap-1">
            <Database className="h-3 w-3" />
            {ENGINE_LABELS[db.engine]} v{db.version}
          </span>
        }
        badges={
          <>
            <Badge variant={statusVariant(livePhase)}>
              {isStarting && <Loader2 className="mr-1 h-3 w-3 animate-spin" />}
              {livePhase}
            </Badge>
            <Badge variant="outline" className="text-xs">
              StatefulSet
            </Badge>
          </>
        }
      />
      <Separator className="my-5" />

      {/* ── Not-ready state: simplified waiting panel ── */}
      {!isReady && (
        <div className="space-y-6">
          <Card>
            <CardContent className="flex flex-col items-center justify-center py-12">
              {isStarting ? (
                <>
                  <div className="flex h-12 w-12 items-center justify-center rounded-full bg-blue-500/10">
                    <Loader2 className="h-6 w-6 animate-spin text-blue-500" />
                  </div>
                  <h3 className="mt-4 text-sm font-medium">Starting database...</h3>
                  <p className="mt-1 max-w-sm text-center text-xs text-muted-foreground">
                    Pulling image and running health checks. This may take a minute for first-time
                    deployments.
                  </p>
                </>
              ) : (
                <>
                  <div className="flex h-12 w-12 items-center justify-center rounded-full bg-yellow-500/10">
                    <AlertTriangle className="h-6 w-6 text-yellow-500" />
                  </div>
                  <h3 className="mt-4 text-sm font-medium">Database is {livePhase}</h3>
                  <p className="mt-1 max-w-sm text-center text-xs text-muted-foreground">
                    Connection info, backups, and settings will be available once the database is
                    running.
                  </p>
                </>
              )}
            </CardContent>
          </Card>

          {/* Still show pods so user can see progress */}
          {pods.length > 0 && (
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-sm font-medium">
                  <HardDrive className="h-4 w-4" /> Pods
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-2">
                  {pods.map((pod) => (
                    <DbPodRow key={pod.name} pod={pod} />
                  ))}
                </div>
              </CardContent>
            </Card>
          )}

          {/* Always allow delete even when not ready */}
          <DangerZone
            description="Delete this database. All data will be permanently lost."
            buttonLabel="Delete Database"
            onDelete={() => setShowDelete(true)}
          />
        </div>
      )}

      {/* ── Ready state: full panel ── */}
      {isReady && (
        <Tabs defaultValue="overview">
          <TabsList>
            <TabsTrigger value="overview">Overview</TabsTrigger>
            <TabsTrigger value="backups" className="gap-1.5">
              Backups
              {backups.length > 0 && (
                <Badge variant="outline" className="ml-0.5 h-5 px-1.5 text-xs">
                  {backups.length}
                </Badge>
              )}
            </TabsTrigger>
            <TabsTrigger value="settings">Settings</TabsTrigger>
          </TabsList>

          {/* ── Overview Tab ── */}
          <TabsContent value="overview" className="mt-4 space-y-6">
            {/* Connection Info */}
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-sm font-medium">
                  <Shield className="h-4 w-4" /> Connection Info
                </CardTitle>
              </CardHeader>
              <CardContent>
                {credsLoading ? (
                  <div className="space-y-3">
                    {Array.from({ length: 6 }).map((_, i) => (
                      <Skeleton key={i} className="h-5 w-full" />
                    ))}
                  </div>
                ) : credentials ? (
                  <div className="divide-y">
                    <CredentialRow label="Host" value={credentials.host} mono />
                    <CredentialRow label="Port" value={String(credentials.port)} mono />
                    <CredentialRow label="Username" value={credentials.username} mono />
                    <CredentialRow label="Password" value={credentials.password} secret mono />
                    <CredentialRow label="Database" value={credentials.database_name} mono />
                    <CredentialRow
                      label="Connection String"
                      value={credentials.connection_string}
                      secret
                      mono
                    />
                    <CredentialRow label="Internal URL" value={credentials.internal_url} mono />
                    {db.external_enabled && db.external_port > 0 && (
                      <CredentialRow
                        label="External URL"
                        value={`${window.location.hostname}:${db.external_port}`}
                        mono
                      />
                    )}
                  </div>
                ) : (
                  <p className="text-sm text-muted-foreground">
                    Credentials will be available once the database is deployed and running.
                  </p>
                )}
              </CardContent>
            </Card>

            {/* Pod Status */}
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-sm font-medium">
                  <HardDrive className="h-4 w-4" /> Pods
                </CardTitle>
              </CardHeader>
              <CardContent>
                {pods.length === 0 ? (
                  <p className="text-sm text-muted-foreground">
                    No pods running yet. Pods will appear once the database is deployed.
                  </p>
                ) : (
                  <div className="space-y-2">
                    {pods.map((pod) => (
                      <DbPodRow key={pod.name} pod={pod} />
                    ))}
                  </div>
                )}
              </CardContent>
            </Card>
          </TabsContent>

          {/* ── Backups Tab ── */}
          <TabsContent value="backups" className="mt-4 space-y-6">
            {/* Backup Configuration */}
            <BackupConfigCard db={db} />

            {/* Backup List */}
            <Card>
              <CardHeader className="flex flex-row items-center justify-between">
                <CardTitle className="flex items-center gap-2 text-sm font-medium">
                  <Database className="h-4 w-4" /> Backup History
                </CardTitle>
                <Button
                  size="sm"
                  onClick={() => triggerBackup.mutate()}
                  disabled={triggerBackup.isPending || !db.backup_s3_id || !isReady}
                  title={
                    !isReady
                      ? "Database must be running"
                      : !db.backup_s3_id
                        ? "Configure S3 storage first"
                        : "Run backup now"
                  }
                >
                  {triggerBackup.isPending ? (
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <Database className="h-3.5 w-3.5" />
                  )}{" "}
                  Run Now
                </Button>
              </CardHeader>
              <CardContent>
                {backups.length === 0 ? (
                  <p className="text-sm text-muted-foreground">
                    No backups yet. Configure automatic backups above or click "Run Now" to create
                    one.
                  </p>
                ) : (
                  <div className="space-y-2">
                    {(showAllBackups ? backups : backups.slice(0, 10)).map((backup) => (
                      <BackupRow
                        key={backup.id}
                        backup={backup}
                        onRestore={(id) => restoreBackup.mutate(id)}
                        isRestoring={restoreBackup.isPending}
                        dbReady={isReady}
                      />
                    ))}
                    {backups.length > 10 && (
                      <button
                        type="button"
                        onClick={() => setShowAllBackups(!showAllBackups)}
                        className="w-full py-2 text-center text-xs text-primary hover:underline"
                      >
                        {showAllBackups ? "Show less" : `Show all ${backups.length} backups`}
                      </button>
                    )}
                  </div>
                )}
              </CardContent>
            </Card>
          </TabsContent>

          {/* ── Settings Tab ── */}
          <TabsContent value="settings" className="mt-4 space-y-6">
            {/* External Access */}
            <ExternalAccessCard db={db} />

            {/* Danger Zone */}
            <DangerZone
              description="Delete this database. All data will be permanently lost."
              buttonLabel="Delete Database"
              onDelete={() => setShowDelete(true)}
            />
          </TabsContent>
        </Tabs>
      )}

      {/* ── Delete Confirmation (type name to confirm) ── */}
      <Dialog
        open={showDelete}
        onOpenChange={(open) => {
          if (!open) setConfirmName("");
          setShowDelete(open);
        }}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>Delete Database</DialogTitle>
            <DialogDescription>
              Permanently delete <strong>{db.name}</strong> ({ENGINE_LABELS[db.engine]})? All data
              will be lost.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-1.5 py-2">
            <Label htmlFor="confirm-db-name" className="text-sm">
              Type <strong className="font-mono">{db.name}</strong> to confirm
            </Label>
            <Input
              id="confirm-db-name"
              placeholder={db.name}
              value={confirmName}
              onChange={(e) => setConfirmName(e.target.value)}
              autoComplete="off"
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowDelete(false)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              disabled={confirmName !== db.name || deleteDb.isPending}
              onClick={() =>
                deleteDb.mutate(undefined, {
                  onSuccess: () => navigate({ to: "/projects/$id", params: { id: projectId } }),
                })
              }
            >
              {deleteDb.isPending ? "Deleting..." : "Delete"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
