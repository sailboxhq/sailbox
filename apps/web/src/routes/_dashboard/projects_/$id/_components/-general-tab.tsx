import { ChevronDown, ChevronRight, HeartPulse } from "lucide-react";
import { useState } from "react";
import { StatCardCompact } from "@/components/stat-card";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { usePodEvents } from "@/hooks/use-apps";
import { statusVariant } from "@/lib/constants";
import type { App, AppStatus, PodEvent, PodInfo } from "@/types/api";

// ── Helpers ────────────────────────────────────────────────────────

/** Parse a K8s resource string into a base numeric value.
 *  CPU: millicores (m), nanocores (n), or bare number (cores)
 *  Memory: Ki, Mi, Gi, or bare bytes
 */
function parseResource(raw: string): number {
  if (!raw) return 0;
  const s = raw.trim();
  const lower = s.toLowerCase();
  // CPU: nanocores
  if (lower.endsWith("n")) return Number.parseFloat(s) / 1_000_000;
  // Memory units (case-sensitive in K8s: Ki, Mi, Gi)
  if (s.endsWith("Ki")) return Number.parseFloat(s) * 1024;
  if (s.endsWith("Mi")) return Number.parseFloat(s) * 1024 * 1024;
  if (s.endsWith("Gi")) return Number.parseFloat(s) * 1024 * 1024 * 1024;
  // CPU millicores
  if (lower.endsWith("m")) return Number.parseFloat(s);
  // Bare number: could be cores (CPU) or bytes (memory)
  return Number.parseFloat(s) || 0;
}

/** Format CPU value to human-readable (e.g. "56m", "1.2 cores") */
function formatCPU(raw: string): string {
  if (!raw || raw === "0") return "0m";
  const millis = parseResource(raw);
  if (millis >= 1000) return `${(millis / 1000).toFixed(1)}`;
  return `${Math.round(millis)}m`;
}

/** Format memory value to human-readable (e.g. "36Mi", "1.2Gi") */
function formatMem(raw: string): string {
  if (!raw || raw === "0") return "0Mi";
  const bytes = parseResource(raw);
  if (bytes >= 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)}Gi`;
  if (bytes >= 1024 * 1024) return `${Math.round(bytes / (1024 * 1024))}Mi`;
  if (bytes >= 1024) return `${Math.round(bytes / 1024)}Ki`;
  return `${bytes}B`;
}

function ResourceBar({
  used,
  total,
  label,
  isCPU,
}: {
  used: string;
  total: string;
  label: string;
  isCPU?: boolean;
}) {
  const u = parseResource(used);
  const t = parseResource(total);
  const pct = t > 0 ? Math.min((u / t) * 100, 100) : 0;
  const fmt = isCPU ? formatCPU : formatMem;
  const color = pct > 90 ? "bg-red-500" : pct > 70 ? "bg-yellow-500" : "bg-primary";

  return (
    <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
      <span className="w-8 shrink-0">{label}</span>
      <div className="h-1.5 w-16 rounded-full bg-muted">
        <div
          className={`h-full rounded-full transition-all ${color}`}
          style={{ width: `${pct}%` }}
        />
      </div>
      <span>
        {fmt(used)}/{fmt(total)}
      </span>
    </div>
  );
}

// ── Pod events sub-row ─────────────────────────────────────────────

function PodEventsPanel({ appId, podName }: { appId: string; podName: string }) {
  const { data: events, isLoading } = usePodEvents(appId, podName);

  if (isLoading) {
    return <p className="px-3 py-2 text-xs text-muted-foreground">Loading events...</p>;
  }

  if (!events || events.length === 0) {
    return <p className="px-3 py-2 text-xs text-muted-foreground">No events.</p>;
  }

  return (
    <div className="space-y-1 px-3 py-2">
      {events.map((ev: PodEvent, i: number) => (
        <div key={i} className="flex items-start gap-2 text-xs">
          <Badge
            variant={ev.type === "Warning" ? "destructive" : "secondary"}
            className="mt-0.5 shrink-0 text-xs"
          >
            {ev.type}
          </Badge>
          <div className="min-w-0 flex-1">
            <span className="font-medium">{ev.reason}</span>
            {ev.count > 1 && <span className="ml-1 text-muted-foreground">x{ev.count}</span>}
            <p className="text-muted-foreground">{ev.message}</p>
          </div>
          <span className="shrink-0 text-xs text-muted-foreground">
            {new Date(ev.last_seen).toLocaleTimeString()}
          </span>
        </div>
      ))}
    </div>
  );
}

// ── Pod row ────────────────────────────────────────────────────────

function PodRow({ pod, appId }: { pod: PodInfo; appId: string }) {
  const [expanded, setExpanded] = useState(false);

  // Transitional states (not errors) — show as warning, not destructive
  const transitionalReasons = [
    "ContainerCreating",
    "PodInitializing",
    "Pending",
    "Pulling",
    "PullingImage",
  ];
  const nonRunningContainers = pod.containers?.filter((c) => c.state !== "running") ?? [];
  const hasContainerIssues = nonRunningContainers.length > 0;
  const isTransitional = nonRunningContainers.every((c) => transitionalReasons.includes(c.reason));

  return (
    <div className="animate-in fade-in duration-300 rounded-md border">
      <button
        type="button"
        className="flex w-full items-center justify-between px-3 py-2 text-sm hover:bg-muted/50"
        onClick={() => setExpanded(!expanded)}
      >
        <div className="flex items-center gap-3">
          {expanded ? (
            <ChevronDown className="h-3.5 w-3.5 text-muted-foreground" />
          ) : (
            <ChevronRight className="h-3.5 w-3.5 text-muted-foreground" />
          )}
          <Badge variant={statusVariant(pod.phase)} className="text-xs">
            {pod.phase}
          </Badge>
          <span className="font-mono text-xs">{pod.name}</span>
          {pod.restart_count > 0 && (
            <span className="text-xs font-medium text-amber-500">&#8635;{pod.restart_count}</span>
          )}
          <span
            className={`inline-block h-2 w-2 rounded-full ${pod.ready ? "bg-green-500" : "bg-red-500"}`}
            title={pod.ready ? "Ready" : "Not ready"}
          />
        </div>
        <div className="flex items-center gap-4">
          {pod.resources && (
            <>
              <ResourceBar
                used={pod.resources.cpu_used}
                total={pod.resources.cpu_total}
                label="CPU"
                isCPU
              />
              <ResourceBar
                used={pod.resources.mem_used}
                total={pod.resources.mem_total}
                label="Mem"
              />
            </>
          )}
        </div>
      </button>

      {/* Container status */}
      {hasContainerIssues && (
        <div
          className={`border-t px-3 py-1.5 ${isTransitional ? "bg-yellow-500/5" : "bg-destructive/5"}`}
        >
          {nonRunningContainers.map((c) => (
            <p
              key={c.name}
              className={`text-xs ${isTransitional ? "text-yellow-700 dark:text-yellow-400" : "text-destructive"}`}
            >
              {c.name}: {c.state}
              {c.reason ? ` (${c.reason})` : ""}
            </p>
          ))}
        </div>
      )}

      {/* Expanded events panel */}
      {expanded && (
        <div className="border-t bg-muted/30">
          <PodEventsPanel appId={appId} podName={pod.name} />
        </div>
      )}
    </div>
  );
}

// ── Main component ─────────────────────────────────────────────────

export function GeneralTab({
  app,
  appStatus,
  pods,
}: {
  app: App;
  appStatus?: AppStatus | null;
  pods: PodInfo[];
}) {
  const hc = app.health_check;

  return (
    <>
      {/* Stat cards */}
      <div className="grid gap-4 md:grid-cols-4">
        <StatCardCompact label="Status" value={appStatus?.phase || app.status} />
        <StatCardCompact
          label="Replicas"
          value={
            appStatus
              ? `${appStatus.ready_replicas}/${appStatus.desired_replicas}`
              : String(app.replicas)
          }
        />
        <StatCardCompact label="CPU" value={app.cpu_limit} />
        <StatCardCompact label="Memory" value={app.mem_limit} />
      </div>

      {/* Internal URL */}
      <Card>
        <CardContent className="p-4">
          <p className="text-xs font-medium text-muted-foreground">Internal URL</p>
          <p className="mt-1 font-mono text-sm">
            http://{app.k8s_name || app.name}:{app.ports?.[0]?.service_port || 80}
          </p>
          <p className="mt-1 text-xs text-muted-foreground">
            Accessible from any service in the {app.namespace || "default"} namespace
          </p>
        </CardContent>
      </Card>

      {/* Health check summary */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-sm">
            <HeartPulse className="h-4 w-4" /> Health Check
          </CardTitle>
        </CardHeader>
        <CardContent>
          {hc?.type ? (
            <div className="flex items-center gap-4 text-sm">
              <Badge variant="outline">{hc.type.toUpperCase()}</Badge>
              {hc.path && <span className="text-muted-foreground">Path: {hc.path}</span>}
              {hc.port > 0 && <span className="text-muted-foreground">Port: {hc.port}</span>}
              {hc.command && (
                <span className="font-mono text-xs text-muted-foreground">{hc.command}</span>
              )}
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">Not configured</p>
          )}
        </CardContent>
      </Card>

      {/* Pod list */}
      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Pods</CardTitle>
        </CardHeader>
        <CardContent>
          {pods.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              No pods running. Deploy the application to start pods.
            </p>
          ) : (
            <div className="space-y-2">
              {pods.map((pod) => (
                <PodRow key={pod.name} pod={pod} appId={app.id} />
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </>
  );
}
