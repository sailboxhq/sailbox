import { Activity, Cpu, HardDrive, MemoryStick, RefreshCw } from "lucide-react";
import { useEffect } from "react";
import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  type TooltipProps,
  XAxis,
  YAxis,
} from "recharts";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { useAppPods, useAppStatus } from "@/hooks/use-apps";
import { useMonitoringSnapshots } from "@/hooks/use-monitoring";
import type { App, PodInfo } from "@/types/api";

// ── Parsing / formatting ────────────────────────────────────────

function parseToMillicores(raw: string): number {
  if (!raw) return 0;
  const s = raw.trim();
  if (s.endsWith("n")) return Number.parseFloat(s) / 1_000_000;
  if (s.endsWith("m")) return Number.parseFloat(s);
  return Number.parseFloat(s) * 1000 || 0;
}

function parseToMiB(raw: string): number {
  if (!raw) return 0;
  const s = raw.trim();
  if (s.endsWith("Ki")) return Number.parseFloat(s) / 1024;
  if (s.endsWith("Mi")) return Number.parseFloat(s);
  if (s.endsWith("Gi")) return Number.parseFloat(s) * 1024;
  return Number.parseFloat(s) / (1024 * 1024) || 0;
}

function fmtCPU(millis: number): string {
  if (millis >= 1000) return `${(millis / 1000).toFixed(1)} cores`;
  return `${Math.round(millis)}m`;
}

function fmtMem(mib: number): string {
  if (mib >= 1024) return `${(mib / 1024).toFixed(1)} Gi`;
  if (mib >= 1) return `${Math.round(mib)} Mi`;
  return `${Math.round(mib * 1024)} Ki`;
}

function fmtTime(iso: string): string {
  try {
    return new Date(iso).toLocaleTimeString([], {
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
    });
  } catch {
    return iso;
  }
}

// ── Chart tooltip ───────────────────────────────────────────────

function ChartTooltip({
  active,
  payload,
  label,
  usedKey,
  limitKey,
  formatter,
}: TooltipProps<number, string> & {
  usedKey: string;
  limitKey: string;
  formatter: (v: number) => string;
}) {
  if (!active || !payload?.length) return null;
  const used = payload.find((p) => p.dataKey === usedKey)?.value ?? 0;
  const limit = payload.find((p) => p.dataKey === limitKey)?.value ?? 0;
  const pct = limit > 0 ? ((used / limit) * 100).toFixed(1) : "0";

  return (
    <div className="rounded-lg border bg-popover px-3 py-2 text-xs shadow-md">
      <p className="mb-1 text-xs text-muted-foreground">{label}</p>
      <span className="font-semibold">{formatter(used)}</span>
      <span className="text-muted-foreground">
        {" "}
        / {formatter(limit)} ({pct}%)
      </span>
    </div>
  );
}

// ── Chart ───────────────────────────────────────────────────────

interface ChartData {
  time: string;
  used: number;
  limit: number;
}

function MetricsChart({
  data,
  color,
  yFormatter,
  tooltipFormatter,
  emptyLabel,
}: {
  data: ChartData[];
  color: string;
  yFormatter: (v: number) => string;
  tooltipFormatter: (v: number) => string;
  emptyLabel: string;
}) {
  if (data.length < 2) {
    return (
      <div className="flex h-[160px] items-center justify-center">
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <RefreshCw className="h-3.5 w-3.5 animate-spin" />
          {emptyLabel}
        </div>
      </div>
    );
  }

  const maxVal = Math.max(...data.map((d) => Math.max(d.used, d.limit)));
  const yMax = Math.ceil(maxVal * 1.15) || 1;

  return (
    <div className="select-none">
      <ResponsiveContainer width="100%" height={160}>
        <AreaChart data={data} margin={{ top: 4, right: 4, left: -20, bottom: 0 }}>
          <defs>
            <linearGradient id={`grad-${color}`} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={color} stopOpacity={0.15} />
              <stop offset="100%" stopColor={color} stopOpacity={0} />
            </linearGradient>
          </defs>
          <CartesianGrid
            strokeDasharray="3 3"
            vertical={false}
            stroke="#9ca3af"
            strokeOpacity={0.12}
          />
          <XAxis
            dataKey="time"
            axisLine={false}
            tickLine={false}
            tick={{ fontSize: 9, fill: "#9ca3af" }}
            interval="preserveStartEnd"
            minTickGap={50}
          />
          <YAxis
            axisLine={false}
            tickLine={false}
            tick={{ fontSize: 9, fill: "#9ca3af" }}
            tickFormatter={yFormatter}
            domain={[0, yMax]}
            width={45}
          />
          <Tooltip
            content={<ChartTooltip usedKey="used" limitKey="limit" formatter={tooltipFormatter} />}
            cursor={false}
          />
          <Area
            type="monotone"
            dataKey="limit"
            stroke="#9ca3af"
            strokeOpacity={0.25}
            fill="none"
            strokeDasharray="4 2"
            strokeWidth={1}
            dot={false}
            activeDot={false}
            isAnimationActive={false}
          />
          <Area
            type="monotone"
            dataKey="used"
            stroke={color}
            fill={`url(#grad-${color})`}
            strokeWidth={1.5}
            dot={false}
            activeDot={false}
            isAnimationActive={false}
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}

// ── Stat card ───────────────────────────────────────────────────

function StatCard({
  icon: Icon,
  label,
  value,
  sub,
  pct,
}: {
  icon: React.ElementType;
  label: string;
  value: string;
  sub: string;
  pct?: number;
}) {
  const pctColor =
    pct != null && pct > 90
      ? "text-destructive"
      : pct != null && pct > 70
        ? "text-yellow-500"
        : "text-primary";

  return (
    <Card>
      <CardContent className="p-4">
        <div className="flex items-center gap-2 text-muted-foreground">
          <Icon className="h-3.5 w-3.5" />
          <span className="text-xs">{label}</span>
        </div>
        <div className="mt-1.5 flex items-baseline gap-1.5">
          <span className="text-xl font-semibold tracking-tight">{value}</span>
          {pct != null && (
            <span className={`text-xs font-medium ${pctColor}`}>{pct.toFixed(0)}%</span>
          )}
        </div>
        <p className="mt-0.5 text-xs text-muted-foreground">{sub}</p>
      </CardContent>
    </Card>
  );
}

// ── Pod card ────────────────────────────────────────────────────

function MiniBar({ pct, color }: { pct: number; color: string }) {
  return (
    <div className="h-1.5 w-full rounded-full bg-muted">
      <div className={`h-full rounded-full ${color}`} style={{ width: `${Math.min(pct, 100)}%` }} />
    </div>
  );
}

function PodCard({ pod }: { pod: PodInfo }) {
  const cpuUsed = parseToMillicores(pod.resources?.cpu_used ?? "");
  const cpuTotal = parseToMillicores(pod.resources?.cpu_total ?? "");
  const memUsed = parseToMiB(pod.resources?.mem_used ?? "");
  const memTotal = parseToMiB(pod.resources?.mem_total ?? "");
  const cpuPct = cpuTotal > 0 ? (cpuUsed / cpuTotal) * 100 : 0;
  const memPct = memTotal > 0 ? (memUsed / memTotal) * 100 : 0;
  const cpuColor = cpuPct > 90 ? "bg-destructive" : cpuPct > 70 ? "bg-yellow-500" : "bg-primary";
  const memColor = memPct > 90 ? "bg-destructive" : memPct > 70 ? "bg-yellow-500" : "bg-violet-500";
  const uptime = pod.started_at ? formatUptime(new Date(pod.started_at)) : "—";

  return (
    <div className="rounded-lg border p-3">
      <div className="mb-2.5 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span
            className={`inline-block h-2 w-2 rounded-full ${pod.phase === "Running" ? "bg-green-500" : pod.phase === "Pending" ? "bg-yellow-500" : pod.phase === "Failed" ? "bg-red-500" : "bg-muted-foreground"}`}
          />
          <span className="font-mono text-xs">{pod.name}</span>
        </div>
        <div className="flex items-center gap-2">
          {pod.restart_count > 0 && (
            <span className="text-xs text-amber-500">{pod.restart_count} restarts</span>
          )}
          <span className="text-xs text-muted-foreground">{uptime}</span>
        </div>
      </div>
      <div className="grid sm:grid-cols-2 gap-4">
        <div className="space-y-1">
          <div className="flex items-center justify-between text-xs">
            <span className="text-muted-foreground">CPU</span>
            <span>
              {fmtCPU(cpuUsed)} / {fmtCPU(cpuTotal)}
            </span>
          </div>
          <MiniBar pct={cpuPct} color={cpuColor} />
        </div>
        <div className="space-y-1">
          <div className="flex items-center justify-between text-xs">
            <span className="text-muted-foreground">Memory</span>
            <span>
              {fmtMem(memUsed)} / {fmtMem(memTotal)}
            </span>
          </div>
          <MiniBar pct={memPct} color={memColor} />
        </div>
      </div>
      {pod.ip && (
        <p className="mt-2 text-xs text-muted-foreground">
          {pod.ip} &middot; {pod.node}
        </p>
      )}
    </div>
  );
}

function formatUptime(started: Date): string {
  const diff = Date.now() - started.getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "<1m";
  if (mins < 60) return `${mins}m`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ${mins % 60}m`;
  const days = Math.floor(hours / 24);
  return `${days}d ${hours % 24}h`;
}

// ── Main ────────────────────────────────────────────────────────

export function MonitoringTab({ app, appId }: { app: App; appId: string }) {
  const { data: appStatus } = useAppStatus(appId);
  const { data: pods, refetch: refetchPods } = useAppPods(appId);
  const safePods = pods ?? [];

  // Fetch persistent metrics from DB (includes build pod resources)
  const { data: snapshots } = useMonitoringSnapshots("app", appId, 10);

  // Refresh pods for live resource display (metrics-server updates ~15s)
  useEffect(() => {
    const id = setInterval(() => refetchPods(), 15_000);
    return () => clearInterval(id);
  }, [refetchPods]);

  // Transform DB snapshots to chart data
  const cpuData: ChartData[] = (snapshots ?? []).map((m) => ({
    time: fmtTime(m.collected_at),
    used: Math.round(m.cpu_used * 10) / 10,
    limit: Math.round(m.cpu_total),
  }));
  const memData: ChartData[] = (snapshots ?? []).map((m) => ({
    time: fmtTime(m.collected_at),
    used: Math.round(m.mem_used / (1024 * 1024)),
    limit: Math.round(m.mem_total / (1024 * 1024)),
  }));

  // Current totals from live pods
  const totalCPUUsed = safePods.reduce(
    (s, p) => s + parseToMillicores(p.resources?.cpu_used ?? ""),
    0,
  );
  const totalCPULimit = safePods.reduce(
    (s, p) => s + parseToMillicores(p.resources?.cpu_total ?? ""),
    0,
  );
  const totalMemUsed = safePods.reduce((s, p) => s + parseToMiB(p.resources?.mem_used ?? ""), 0);
  const totalMemLimit = safePods.reduce((s, p) => s + parseToMiB(p.resources?.mem_total ?? ""), 0);
  const totalRestarts = safePods.reduce((s, p) => s + p.restart_count, 0);
  const cpuPct = totalCPULimit > 0 ? (totalCPUUsed / totalCPULimit) * 100 : 0;
  const memPct = totalMemLimit > 0 ? (totalMemUsed / totalMemLimit) * 100 : 0;

  return (
    <div className="space-y-4">
      <div className="grid gap-4 md:grid-cols-4">
        <StatCard
          icon={Activity}
          label="Status"
          value={appStatus?.phase ?? app.status}
          sub={
            appStatus
              ? `${appStatus.ready_replicas}/${appStatus.desired_replicas} ready`
              : `${app.replicas} replica(s)`
          }
        />
        <StatCard
          icon={Cpu}
          label="CPU"
          value={fmtCPU(totalCPUUsed)}
          sub={`of ${fmtCPU(totalCPULimit)}`}
          pct={cpuPct}
        />
        <StatCard
          icon={MemoryStick}
          label="Memory"
          value={fmtMem(totalMemUsed)}
          sub={`of ${fmtMem(totalMemLimit)}`}
          pct={memPct}
        />
        <StatCard
          icon={HardDrive}
          label="Pods"
          value={String(safePods.length)}
          sub={totalRestarts > 0 ? `${totalRestarts} total restarts` : "No restarts"}
        />
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader className="pb-0">
            <div className="flex items-center justify-between">
              <CardTitle className="text-xs font-medium">CPU</CardTitle>
              <span className="text-xs text-muted-foreground">millicores</span>
            </div>
          </CardHeader>
          <CardContent className="pt-2">
            <MetricsChart
              data={cpuData}
              color="#6d5cdb"
              yFormatter={(v) => `${v}m`}
              tooltipFormatter={fmtCPU}
              emptyLabel="Collecting CPU data..."
            />
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-0">
            <div className="flex items-center justify-between">
              <CardTitle className="text-xs font-medium">Memory</CardTitle>
              <span className="text-xs text-muted-foreground">MiB</span>
            </div>
          </CardHeader>
          <CardContent className="pt-2">
            <MetricsChart
              data={memData}
              color="#8b5cf6"
              yFormatter={(v) => `${v}`}
              tooltipFormatter={fmtMem}
              emptyLabel="Collecting memory data..."
            />
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between pb-3">
          <div>
            <CardTitle className="text-xs font-medium">Pods</CardTitle>
            <CardDescription className="text-xs">Per-pod resource usage</CardDescription>
          </div>
          <Button size="icon" variant="ghost" className="h-7 w-7" onClick={() => refetchPods()}>
            <RefreshCw className="h-3 w-3" />
          </Button>
        </CardHeader>
        <CardContent>
          {safePods.length === 0 ? (
            <p className="py-6 text-center text-xs text-muted-foreground">No pods running.</p>
          ) : (
            <div className="space-y-2">
              {safePods.map((pod) => (
                <PodCard key={pod.name} pod={pod} />
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
