import { createFileRoute, Link } from "@tanstack/react-router";
import {
  AlertTriangle,
  ChevronRight,
  Cpu,
  Database,
  FolderKanban,
  HardDrive,
  Layers,
  Server,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { useClusterMetrics, useClusterNodes, useClusterPods } from "@/hooks/use-cluster";
import { useDashboardApps, useDashboardDatabases } from "@/hooks/use-dashboard";
import { useActiveAlerts } from "@/hooks/use-monitoring";
import { useProjects } from "@/hooks/use-projects";
import { statusDotColor, statusVariant } from "@/lib/constants";
import type { PodInfo } from "@/types/api";

// ── Helpers ─────────────────────────────────────────────────────

function parseMillicores(raw: string): number {
  if (!raw) return 0;
  const s = raw.trim();
  if (s.endsWith("n")) return Number.parseFloat(s) / 1_000_000;
  if (s.endsWith("m")) return Number.parseFloat(s);
  return Number.parseFloat(s) * 1000 || 0;
}

function parseMiB(raw: string): number {
  if (!raw) return 0;
  const s = raw.trim();
  if (s.endsWith("Ki")) return Number.parseFloat(s) / 1024;
  if (s.endsWith("Mi")) return Number.parseFloat(s);
  if (s.endsWith("Gi")) return Number.parseFloat(s) * 1024;
  return Number.parseFloat(s) / (1024 * 1024) || 0;
}

function timeAgo(date: string): string {
  const diff = Date.now() - new Date(date).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

function pct(used: number, total: number): number {
  if (total <= 0) return 0;
  return Math.min(Math.round((used / total) * 100), 100);
}

// ── Stat Card ───────────────────────────────────────────────────

function StatCard({
  icon: Icon,
  label,
  value,
  sub,
  percent,
  color = "bg-primary",
}: {
  icon: React.ElementType;
  label: string;
  value: string;
  sub: string;
  percent: number;
  color?: string;
}) {
  return (
    <Card className="overflow-hidden">
      <CardContent className="p-5">
        <div className="flex items-center gap-3">
          <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-primary/10">
            <Icon className="h-4 w-4 text-primary" />
          </div>
          <div className="min-w-0 flex-1">
            <p className="text-xs font-medium text-muted-foreground">{label}</p>
            <div className="flex items-baseline gap-2">
              <span className="text-2xl font-bold tracking-tight">{value}</span>
              <span className="text-xs text-muted-foreground">{sub}</span>
            </div>
          </div>
          <span className="text-sm font-semibold text-muted-foreground">{percent}%</span>
        </div>
        <div className="mt-3 h-1.5 overflow-hidden rounded-full bg-muted">
          <div
            className={`h-full rounded-full transition-all duration-500 ${color}`}
            style={{ width: `${percent}%` }}
          />
        </div>
      </CardContent>
    </Card>
  );
}

// ── Route ───────────────────────────────────────────────────────

export const Route = createFileRoute("/_dashboard/dashboard")({
  component: DashboardPage,
});

function DashboardPage() {
  const { data: projects, isLoading } = useProjects();
  const { data: apps } = useDashboardApps();
  const { data: databases } = useDashboardDatabases();
  const { data: cluster } = useClusterMetrics();
  const { data: nodes } = useClusterNodes();
  const { data: pods } = useClusterPods();
  const { data: alertsData } = useActiveAlerts();

  const allApps = apps ?? [];
  const allDbs = databases ?? [];
  const allPods = pods ?? [];
  const allNodes = nodes ?? [];
  const alertCount = alertsData?.count ?? 0;
  const alerts = alertsData?.alerts ?? [];

  const cpuUsed = parseMillicores(cluster?.resources.cpu_used ?? "");
  const cpuTotal = parseMillicores(cluster?.resources.cpu_total ?? "");
  const memUsed = parseMiB(cluster?.resources.mem_used ?? "");
  const memTotal = parseMiB(cluster?.resources.mem_total ?? "");
  const runningPods = allPods.filter((p: PodInfo) => p.phase === "Running").length;
  const readyNodes = allNodes.filter((n) => n.status === "Ready").length;

  if (isLoading) {
    return (
      <div className="space-y-8">
        <Skeleton className="h-8 w-48" />
        <div className="grid gap-4 md:grid-cols-4">
          {[1, 2, 3, 4].map((i) => (
            <Skeleton key={i} className="h-24" />
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-8">
      {/* ── Header ── */}
      <div>
        <p className="text-sm font-medium text-muted-foreground">Overview</p>
        <h1 className="text-2xl font-bold tracking-tight">Dashboard</h1>
      </div>

      {/* ── Metrics ── */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          icon={Cpu}
          label="CPU Usage"
          value={`${Math.round(cpuUsed)}m`}
          sub={`of ${Math.round(cpuTotal)}m`}
          percent={pct(cpuUsed, cpuTotal)}
        />
        <StatCard
          icon={HardDrive}
          label="Memory"
          value={`${Math.round(memUsed)}Mi`}
          sub={`of ${Math.round(memTotal)}Mi`}
          percent={pct(memUsed, memTotal)}
          color="bg-violet-500"
        />
        <StatCard
          icon={Layers}
          label="Pods"
          value={String(runningPods)}
          sub={`of ${allPods.length} total`}
          percent={pct(runningPods, allPods.length || 1)}
          color="bg-emerald-500"
        />
        <StatCard
          icon={Server}
          label="Nodes"
          value={String(readyNodes)}
          sub={`of ${allNodes.length} ready`}
          percent={pct(readyNodes, allNodes.length || 1)}
          color="bg-emerald-500"
        />
      </div>

      {/* ── Content Grid ── */}
      <div className="grid gap-6 lg:grid-cols-2">
        {/* Alerts */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-4">
            <CardTitle className="flex items-center gap-2 text-sm font-medium">
              <AlertTriangle className="h-4 w-4" />
              Active Alerts
              {alertCount > 0 && (
                <Badge variant="destructive" className="text-xs">
                  {alertCount}
                </Badge>
              )}
            </CardTitle>
            <Link to="/cluster" className="text-xs text-primary hover:underline">
              View all →
            </Link>
          </CardHeader>
          <CardContent>
            {alerts.length === 0 ? (
              <div className="flex flex-col items-center py-8 text-center">
                <div className="flex h-10 w-10 items-center justify-center rounded-full bg-emerald-500/10">
                  <AlertTriangle className="h-5 w-5 text-emerald-500" />
                </div>
                <p className="mt-3 text-sm text-muted-foreground">All clear — no active alerts</p>
              </div>
            ) : (
              <div className="space-y-2">
                {alerts.slice(0, 5).map((alert) => (
                  <div
                    key={alert.id}
                    className="flex items-center gap-3 rounded-lg border px-3 py-2.5 transition-colors hover:bg-accent/30"
                  >
                    <div
                      className={`h-2 w-2 shrink-0 rounded-full ${alert.severity === "critical" ? "bg-destructive" : "bg-yellow-500"}`}
                    />
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm">{alert.message}</p>
                      <p className="text-xs text-muted-foreground">{timeAgo(alert.fired_at)}</p>
                    </div>
                    <Badge
                      variant={alert.severity === "critical" ? "destructive" : "warning"}
                      className="shrink-0 text-xs"
                    >
                      {alert.severity}
                    </Badge>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>

        {/* Applications */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-4">
            <CardTitle className="flex items-center gap-2 text-sm font-medium">
              <Layers className="h-4 w-4" />
              Applications
              <span className="text-xs font-normal text-muted-foreground">({allApps.length})</span>
            </CardTitle>
            <Link to="/apps" className="text-xs text-primary hover:underline">
              View all →
            </Link>
          </CardHeader>
          <CardContent>
            {allApps.length === 0 ? (
              <p className="py-8 text-center text-sm text-muted-foreground">No applications yet</p>
            ) : (
              <div className="space-y-1">
                {allApps.slice(0, 6).map((app) => (
                  <div
                    key={app.id}
                    className="flex items-center gap-3 rounded-lg px-2 py-2 transition-colors hover:bg-accent/50"
                  >
                    <span
                      className={`inline-block h-2 w-2 shrink-0 rounded-full ${statusDotColor(app.status)}`}
                    />
                    <span className="flex-1 truncate text-sm font-medium">{app.name}</span>
                    <Badge variant={statusVariant(app.status)} className="text-xs">
                      {app.status}
                    </Badge>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>

        {/* Databases */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-4">
            <CardTitle className="flex items-center gap-2 text-sm font-medium">
              <Database className="h-4 w-4" />
              Databases
              <span className="text-xs font-normal text-muted-foreground">({allDbs.length})</span>
            </CardTitle>
          </CardHeader>
          <CardContent>
            {allDbs.length === 0 ? (
              <p className="py-8 text-center text-sm text-muted-foreground">No databases yet</p>
            ) : (
              <div className="space-y-1">
                {allDbs.slice(0, 6).map((db) => (
                  <Link
                    key={db.id}
                    to="/projects/$id/databases/$dbId"
                    params={{ id: db.project_id, dbId: db.id }}
                    className="block"
                  >
                    <div className="flex items-center gap-3 rounded-lg px-2 py-2 transition-colors hover:bg-accent/50">
                      <Database className="h-3.5 w-3.5 shrink-0 text-primary" />
                      <span className="flex-1 truncate text-sm font-medium">{db.name}</span>
                      <Badge variant={statusVariant(db.status)} className="text-xs">
                        {db.status}
                      </Badge>
                    </div>
                  </Link>
                ))}
              </div>
            )}
          </CardContent>
        </Card>

        {/* Projects */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-4">
            <CardTitle className="flex items-center gap-2 text-sm font-medium">
              <FolderKanban className="h-4 w-4" />
              Projects
              <span className="text-xs font-normal text-muted-foreground">
                ({projects?.length ?? 0})
              </span>
            </CardTitle>
            <Link to="/projects" className="text-xs text-primary hover:underline">
              View all →
            </Link>
          </CardHeader>
          <CardContent>
            {!projects?.length ? (
              <p className="py-8 text-center text-sm text-muted-foreground">No projects yet</p>
            ) : (
              <div className="space-y-1">
                {projects.slice(0, 6).map((p) => (
                  <Link key={p.id} to="/projects/$id" params={{ id: p.id }} className="block">
                    <div className="flex items-center justify-between rounded-lg px-2 py-2.5 transition-colors hover:bg-accent/50">
                      <div className="flex items-center gap-3">
                        <div className="flex h-7 w-7 items-center justify-center rounded-md bg-primary/10">
                          <FolderKanban className="h-3.5 w-3.5 text-primary" />
                        </div>
                        <div>
                          <p className="text-sm font-medium">{p.name}</p>
                          <p className="text-xs text-muted-foreground">
                            {p.description || "No description"}
                          </p>
                        </div>
                      </div>
                      <ChevronRight className="h-4 w-4 text-muted-foreground" />
                    </div>
                  </Link>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
