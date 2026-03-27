import { createFileRoute } from "@tanstack/react-router";
import {
  Activity,
  Box,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  Cpu,
  Database,
  FolderOpen,
  HardDrive,
  HeartPulse,
  Loader2,
  MemoryStick,
  Plus,
  Server,
  Trash2,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { ClusterTopologyView } from "@/components/cluster-topology";
import { EmptyState } from "@/components/empty-state";
import { LoadingScreen } from "@/components/loading-screen";
import { StatCardCompact } from "@/components/stat-card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
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
import {
  useCleanupCompletedJobs,
  useCleanupCompletedPods,
  useCleanupEvictedPods,
  useCleanupFailedPods,
  useCleanupOrphanIngresses,
  useCleanupStaleReplicaSets,
  useCleanupStats,
  useClusterMetrics,
  useClusterNamespaces,
  useClusterNodes,
  useClusterPods,
  useClusterPVCs,
  useClusterTopology,
  useDaemonSets,
  useHelmReleases,
  useNodeMetrics,
  useNodePools,
  useSetNodePool,
} from "@/hooks/use-cluster";
import {
  useActiveAlerts,
  useMonitoringEvents,
  useMonitoringSnapshots,
  useResolveAlert,
} from "@/hooks/use-monitoring";
import { useCreateNode, useInitializeNode } from "@/hooks/use-nodes";
import { useResources } from "@/hooks/use-resources";
import { getToken } from "@/lib/auth";
import { statusVariant } from "@/lib/constants";
import type {
  DaemonSetInfo,
  HelmRelease,
  MetricAlert,
  NamespaceInfo,
  NodeMetrics as NodeMetricsType,
  PodInfo,
  PVCInfo,
} from "@/types/api";

export const Route = createFileRoute("/_dashboard/cluster")({
  component: ClusterPage,
});

// ── Helpers ──────────────────────────────────────────────────────

function timeAgo(dateStr: string): string {
  const now = Date.now();
  const then = new Date(dateStr).getTime();
  const diffMs = now - then;
  if (diffMs < 0) return "just now";
  const seconds = Math.floor(diffMs / 1000);
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

function parseResourceValue(val: string): number {
  if (!val) return 0;
  // Handle CPU: "280m" -> 280, "2" -> 2000, "1500m" -> 1500
  if (val.endsWith("m")) return parseFloat(val);
  if (val.endsWith("n")) return parseFloat(val) / 1_000_000;
  // Could be a bare number (cores)
  if (/^\d+(\.\d+)?$/.test(val)) return parseFloat(val) * 1000;
  // Handle memory: "1536Mi" -> 1536, "2Gi" -> 2048, "123456Ki" -> ~120
  if (val.endsWith("Ki")) return parseFloat(val) / 1024;
  if (val.endsWith("Mi")) return parseFloat(val);
  if (val.endsWith("Gi")) return parseFloat(val) * 1024;
  return parseFloat(val) || 0;
}

function pctString(used: string, total: string): string {
  const u = parseResourceValue(used);
  const t = parseResourceValue(total);
  if (t === 0) return "0%";
  return `${Math.round((u / t) * 100)}%`;
}

function pctNumber(used: string, total: string): number {
  const u = parseResourceValue(used);
  const t = parseResourceValue(total);
  if (t === 0) return 0;
  return Math.min(100, Math.round((u / t) * 100));
}

function ProgressBar({ value, className }: { value: number; className?: string }) {
  const color = value > 90 ? "bg-red-500" : value > 70 ? "bg-amber-500" : "bg-green-500";
  return (
    <div className={`h-2 w-full rounded-full bg-muted ${className ?? ""}`}>
      <div className={`h-2 rounded-full transition-all ${color}`} style={{ width: `${value}%` }} />
    </div>
  );
}

function eventVariant(type: string): "secondary" | "destructive" {
  return type === "Warning" ? "destructive" : "secondary";
}

function pvcStatusVariant(status: string) {
  if (status === "Bound") return "success" as const;
  if (status === "Pending") return "warning" as const;
  if (status === "Lost") return "destructive" as const;
  return "secondary" as const;
}

// ── Main Page ────────────────────────────────────────────────────

function ClusterPage() {
  const { data: nodes } = useClusterNodes();
  const { data: metrics, isLoading: metricsLoading } = useClusterMetrics();
  const { data: nodeMetrics, isLoading: nodeMetricsLoading } = useNodeMetrics();
  const { data: pods } = useClusterPods();
  // Events tab uses its own hook (useMonitoringEvents) for persisted data
  const { data: pvcs } = useClusterPVCs();
  const { data: namespaces } = useClusterNamespaces();
  const { data: activeAlertsData } = useActiveAlerts();
  const { data: topology } = useClusterTopology();
  const activeAlertCount = activeAlertsData?.count ?? 0;

  // Only block on critical data for the overview cards
  const loading = metricsLoading || nodeMetricsLoading;

  if (loading) return <LoadingScreen variant="detail" />;

  // Compute CPU/Memory percentages from nodeMetrics
  const cpuPct = nodeMetrics?.length
    ? pctString(
        `${nodeMetrics.reduce((a, n) => a + parseResourceValue(n.cpu_used), 0)}m`,
        `${nodeMetrics.reduce((a, n) => a + parseResourceValue(n.cpu_total), 0)}m`,
      )
    : "N/A";

  const memPct = nodeMetrics?.length
    ? pctString(
        `${nodeMetrics.reduce((a, n) => a + parseResourceValue(n.mem_used), 0)}Mi`,
        `${nodeMetrics.reduce((a, n) => a + parseResourceValue(n.mem_total), 0)}Mi`,
      )
    : "N/A";

  return (
    <div>
      <div>
        <h1 className="text-2xl font-bold tracking-tight">Cluster</h1>
        <p className="text-sm text-muted-foreground">K3s cluster overview and monitoring</p>
      </div>

      {/* Stat cards */}
      <div className="mt-6 grid gap-4 md:grid-cols-5">
        <StatCardCompact label="Nodes" value={metrics?.nodes ?? 0} />
        <StatCardCompact
          label="Pods"
          value={metrics ? `${metrics.running_pods}/${metrics.total_pods}` : "0"}
        />
        <StatCardCompact label="CPU" value={cpuPct} />
        <StatCardCompact label="Memory" value={memPct} />
        <StatCardCompact label="PVCs" value={pvcs?.length ?? 0} />
      </div>

      {/* Node resource trend charts */}
      <NodeTrendCharts />

      {/* Tabs */}
      <Tabs defaultValue="nodes" className="mt-6">
        <TabsList>
          <TabsTrigger value="topology">Topology</TabsTrigger>
          <TabsTrigger value="nodes">Nodes</TabsTrigger>
          <TabsTrigger value="pods">Pods</TabsTrigger>
          <TabsTrigger value="events">Events</TabsTrigger>
          <TabsTrigger value="alerts" className="relative">
            Alerts
            {activeAlertCount > 0 && (
              <span className="ml-1.5 inline-flex h-4 min-w-4 items-center justify-center rounded-full bg-destructive px-1 text-xs font-bold text-white">
                {activeAlertCount}
              </span>
            )}
          </TabsTrigger>
          <TabsTrigger value="storage">Storage</TabsTrigger>
          <TabsTrigger value="namespaces">Namespaces</TabsTrigger>
          <TabsTrigger value="helm">Helm</TabsTrigger>
          <TabsTrigger value="daemonsets">DaemonSets</TabsTrigger>
          <TabsTrigger value="health">Health</TabsTrigger>
        </TabsList>

        <TabsContent value="topology">
          {topology && <ClusterTopologyView data={topology} />}
        </TabsContent>
        <TabsContent value="nodes">
          <NodesTab nodes={nodes ?? []} nodeMetrics={nodeMetrics ?? []} />
        </TabsContent>
        <TabsContent value="pods">
          <PodsTab pods={pods ?? []} namespaces={namespaces ?? []} />
        </TabsContent>
        <TabsContent value="events">
          <EventsTab />
        </TabsContent>
        <TabsContent value="alerts">
          <AlertsTab />
        </TabsContent>
        <TabsContent value="storage">
          <StorageTab pvcs={pvcs ?? []} />
        </TabsContent>
        <TabsContent value="namespaces">
          <NamespacesTab namespaces={namespaces ?? []} />
        </TabsContent>
        <TabsContent value="helm">
          <HelmTab />
        </TabsContent>
        <TabsContent value="daemonsets">
          <DaemonSetsTab />
        </TabsContent>
        <TabsContent value="health">
          <ClusterHealthTab />
        </TabsContent>
      </Tabs>
    </div>
  );
}

// ── Node Trend Charts ────────────────────────────────────────────

function NodeTrendCharts() {
  const { data: snapshots } = useMonitoringSnapshots("node", undefined, 60);

  if (!snapshots || snapshots.length < 2) return null;

  // Group by node, then build time series
  const nodeMap = new Map<string, typeof snapshots>();
  for (const s of snapshots) {
    const arr = nodeMap.get(s.source_name) || [];
    arr.push(s);
    nodeMap.set(s.source_name, arr);
  }

  // Aggregate all nodes into total
  const timeMap = new Map<
    string,
    { time: string; cpu: number; cpuTotal: number; mem: number; memTotal: number }
  >();
  for (const s of snapshots) {
    const t = new Date(s.collected_at).toLocaleTimeString([], {
      hour: "2-digit",
      minute: "2-digit",
    });
    const key = s.collected_at;
    const existing = timeMap.get(key) || { time: t, cpu: 0, cpuTotal: 0, mem: 0, memTotal: 0 };
    existing.cpu += s.cpu_used;
    existing.cpuTotal += s.cpu_total;
    existing.mem += Math.round(s.mem_used / (1024 * 1024));
    existing.memTotal += Math.round(s.mem_total / (1024 * 1024));
    timeMap.set(key, existing);
  }
  const chartData = [...timeMap.values()];

  return (
    <div className="mt-6 grid gap-4 lg:grid-cols-2">
      <Card>
        <CardHeader className="pb-0">
          <div className="flex items-center justify-between">
            <CardTitle className="text-xs font-medium">Cluster CPU</CardTitle>
            <span className="text-xs text-muted-foreground">last hour · millicores</span>
          </div>
        </CardHeader>
        <CardContent className="pt-2">
          <div className="select-none">
            <ResponsiveContainer width="100%" height={140}>
              <AreaChart data={chartData} margin={{ top: 4, right: 4, left: 0, bottom: 0 }}>
                <defs>
                  <linearGradient id="grad-cluster-cpu" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor="#6d5cdb" stopOpacity={0.15} />
                    <stop offset="100%" stopColor="#6d5cdb" stopOpacity={0} />
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
                  tickFormatter={(v) => (v >= 1000 ? `${(v / 1000).toFixed(1)}` : `${v}m`)}
                  width={36}
                />
                <Tooltip
                  cursor={false}
                  contentStyle={{
                    fontSize: 11,
                    borderRadius: 8,
                    border: "1px solid var(--color-border)",
                    background: "var(--color-popover)",
                  }}
                  formatter={(v: number, name: string) => [
                    `${v}m`,
                    name === "cpu" ? "Used" : "Total",
                  ]}
                />
                <Area
                  type="monotone"
                  dataKey="cpuTotal"
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
                  dataKey="cpu"
                  stroke="#6d5cdb"
                  fill="url(#grad-cluster-cpu)"
                  strokeWidth={1.5}
                  dot={false}
                  activeDot={false}
                  isAnimationActive={false}
                />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        </CardContent>
      </Card>
      <Card>
        <CardHeader className="pb-0">
          <div className="flex items-center justify-between">
            <CardTitle className="text-xs font-medium">Cluster Memory</CardTitle>
            <span className="text-xs text-muted-foreground">last hour · MiB</span>
          </div>
        </CardHeader>
        <CardContent className="pt-2">
          <div className="select-none">
            <ResponsiveContainer width="100%" height={140}>
              <AreaChart data={chartData} margin={{ top: 4, right: 4, left: 0, bottom: 0 }}>
                <defs>
                  <linearGradient id="grad-cluster-mem" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor="#8b5cf6" stopOpacity={0.15} />
                    <stop offset="100%" stopColor="#8b5cf6" stopOpacity={0} />
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
                  tickFormatter={(v) => (v >= 1024 ? `${(v / 1024).toFixed(1)}G` : `${v}`)}
                  width={36}
                />
                <Tooltip
                  cursor={false}
                  contentStyle={{
                    fontSize: 11,
                    borderRadius: 8,
                    border: "1px solid var(--color-border)",
                    background: "var(--color-popover)",
                  }}
                  formatter={(v: number, name: string) => [
                    `${v} Mi`,
                    name === "mem" ? "Used" : "Total",
                  ]}
                />
                <Area
                  type="monotone"
                  dataKey="memTotal"
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
                  dataKey="mem"
                  stroke="#8b5cf6"
                  fill="url(#grad-cluster-mem)"
                  strokeWidth={1.5}
                  dot={false}
                  activeDot={false}
                  isAnimationActive={false}
                />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

// ── Nodes Tab ────────────────────────────────────────────────────

function NodesTab({
  nodes,
  nodeMetrics,
}: {
  nodes: ReturnType<typeof useClusterNodes>["data"] extends infer T ? NonNullable<T> : never;
  nodeMetrics: NodeMetricsType[];
}) {
  const [sheetOpen, setSheetOpen] = useState(false);
  const { data: pools } = useNodePools();
  const setNodePool = useSetNodePool();

  const metricsMap = useMemo(() => {
    const map = new Map<string, NodeMetricsType>();
    for (const m of nodeMetrics) map.set(m.name, m);
    return map;
  }, [nodeMetrics]);

  return (
    <div className="mt-3 space-y-3">
      <div className="flex justify-end">
        <Button size="sm" onClick={() => setSheetOpen(true)}>
          <Plus className="h-4 w-4" /> Add Node
        </Button>
      </div>

      {nodes.length === 0 ? (
        <EmptyState
          icon={Server}
          message="No nodes found"
          actionLabel="Add Node"
          onAction={() => setSheetOpen(true)}
        />
      ) : (
        nodes.map((node) => {
          const nm = metricsMap.get(node.name);
          const cpuPct = nm ? pctNumber(nm.cpu_used, nm.cpu_total) : 0;
          const memPct = nm ? pctNumber(nm.mem_used, nm.mem_total) : 0;

          return (
            <Card key={node.name}>
              <CardHeader className="flex flex-row items-start gap-3 pb-3">
                <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary/10">
                  <Server className="h-4 w-4 text-primary" />
                </div>
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <CardTitle className="text-sm">{node.name}</CardTitle>
                    <Badge variant={statusVariant(node.status)}>{node.status}</Badge>
                    {node.roles.map((r) => (
                      <Badge key={r} variant="outline" className="text-xs">
                        {r}
                      </Badge>
                    ))}
                  </div>
                  <CardDescription className="text-xs">
                    {node.ip} · {node.version} · {node.os}/{node.arch}
                  </CardDescription>
                </div>
              </CardHeader>
              <CardContent className="space-y-3">
                <div className="grid gap-4 sm:grid-cols-3">
                  <div>
                    <div className="mb-1 flex items-center justify-between text-xs text-muted-foreground">
                      <span className="flex items-center gap-1">
                        <Cpu className="h-3 w-3" /> CPU
                      </span>
                      <span>
                        {nm?.cpu_used || "N/A"} / {nm?.cpu_total || node.resources.cpu_total} (
                        {cpuPct}%)
                      </span>
                    </div>
                    <ProgressBar value={cpuPct} />
                  </div>
                  <div>
                    <div className="mb-1 flex items-center justify-between text-xs text-muted-foreground">
                      <span className="flex items-center gap-1">
                        <MemoryStick className="h-3 w-3" /> Memory
                      </span>
                      <span>
                        {nm?.mem_used || "N/A"} / {nm?.mem_total || node.resources.mem_total} (
                        {memPct}%)
                      </span>
                    </div>
                    <ProgressBar value={memPct} />
                  </div>
                  <div className="flex items-center gap-1 text-xs text-muted-foreground">
                    <Box className="h-3 w-3" /> {nm?.pod_count ?? 0} pods
                  </div>
                </div>
                <div className="flex items-center justify-between border-t pt-3">
                  <span className="text-xs text-muted-foreground">Node Pool</span>
                  <Select
                    value={node.pool || "none"}
                    onValueChange={(v) =>
                      setNodePool.mutate({ nodeName: node.name, pool: v === "none" ? "" : v })
                    }
                  >
                    <SelectTrigger className="h-7 w-40 text-xs">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="none">No pool</SelectItem>
                      {[
                        ...new Set([
                          ...(pools ?? []),
                          "default",
                          "production",
                          "development",
                          "testing",
                          "build",
                        ]),
                      ].map((p) => (
                        <SelectItem key={p} value={p}>
                          {p}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </CardContent>
            </Card>
          );
        })
      )}

      <AddNodeSheet open={sheetOpen} onOpenChange={setSheetOpen} />
    </div>
  );
}

// ── Add Node Sheet ──────────────────────────────────────────────

function AddNodeSheet({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const createNode = useCreateNode();
  const initializeNode = useInitializeNode();
  const { data: sshKeys } = useResources("ssh_key");

  const [name, setName] = useState("");
  const [host, setHost] = useState("");
  const [port, setPort] = useState("22");
  const [customPort, setCustomPort] = useState("");
  const [sshUser, setSshUser] = useState("root");
  const [customUser, setCustomUser] = useState("");
  const [authType, setAuthType] = useState("password");
  const [password, setPassword] = useState("");
  const [sshKeyId, setSshKeyId] = useState("");
  const [role, setRole] = useState("worker");

  const [phase, setPhase] = useState<"form" | "initializing" | "done" | "error">("form");
  const [logs, setLogs] = useState<string[]>([]);
  const logRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WebSocket | null>(null);

  const resetForm = useCallback(() => {
    setName("");
    setHost("");
    setPort("22");
    setCustomPort("");
    setSshUser("root");
    setCustomUser("");
    setAuthType("password");
    setPassword("");
    setSshKeyId("");
    setRole("worker");
    setPhase("form");
    setLogs([]);
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }
  }, []);

  const handleOpenChange = useCallback(
    (next: boolean) => {
      if (!next) {
        resetForm();
      }
      onOpenChange(next);
    },
    [onOpenChange, resetForm],
  );

  // Clean up WebSocket on unmount
  useEffect(() => {
    return () => {
      wsRef.current?.close();
    };
  }, []);

  // Auto-scroll terminal
  useEffect(() => {
    if (logRef.current) {
      logRef.current.scrollTop = logRef.current.scrollHeight;
    }
  }, []);

  const connectWs = useCallback((nodeId: string) => {
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const token = getToken();
    const wsUrl = `${protocol}//${window.location.host}/ws/nodes/${nodeId}/logs?token=${token}`;
    const ws = new WebSocket(wsUrl);
    wsRef.current = ws;

    ws.onmessage = (event) => {
      const msg = event.data as string;
      setLogs((prev) => [...prev, msg]);
      if (msg.includes("joined the cluster")) {
        setPhase("done");
      } else if (msg.startsWith("ERROR:") || msg.includes("Timeout:")) {
        setPhase("error");
      }
    };
    ws.onclose = () => {
      // Only mark done if not already in error state
      setPhase((prev) => (prev === "initializing" ? "done" : prev));
    };
    ws.onerror = () => {
      setLogs((prev) => [...prev, "--- WebSocket error ---"]);
    };
  }, []);

  const handleInitialize = useCallback(async () => {
    const resolvedPort = port === "custom" ? Number(customPort) : Number(port);
    const resolvedUser = sshUser === "custom" ? customUser : sshUser;

    setPhase("initializing");
    setLogs(["Creating node record..."]);

    try {
      const node = await createNode.mutateAsync({
        name,
        host,
        port: resolvedPort,
        ssh_user: resolvedUser,
        auth_type: authType,
        ...(authType === "password" ? { password } : sshKeyId ? { ssh_key_id: sshKeyId } : {}),
        role,
      });

      setLogs((prev) => [...prev, `Node "${node.name}" created. Starting initialization...`]);
      connectWs(node.id);

      await initializeNode.mutateAsync(node.id);
    } catch {
      setPhase("error");
      setLogs((prev) => [...prev, "--- Failed ---"]);
    }
  }, [
    name,
    host,
    port,
    customPort,
    sshUser,
    customUser,
    authType,
    password,
    sshKeyId,
    role,
    createNode,
    initializeNode,
    connectWs,
  ]);

  const formDisabled = phase === "initializing";

  return (
    <Sheet open={open} onOpenChange={handleOpenChange}>
      <SheetContent className="flex flex-col sm:max-w-xl">
        <SheetHeader>
          <SheetTitle>Add Node</SheetTitle>
          <SheetDescription>Add a server node to the cluster via SSH.</SheetDescription>
        </SheetHeader>

        <div className="flex flex-1 flex-col gap-4 overflow-hidden">
          {/* Form section */}
          <div className="space-y-3 overflow-y-auto">
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-1">
                <Label htmlFor="node-name">Name</Label>
                <Input
                  id="node-name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="worker-01"
                  disabled={formDisabled}
                  required
                />
              </div>
              <div className="space-y-1">
                <Label htmlFor="node-host">Host</Label>
                <Input
                  id="node-host"
                  value={host}
                  onChange={(e) => setHost(e.target.value)}
                  placeholder="192.168.1.100"
                  disabled={formDisabled}
                  required
                />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-1">
                <Label>Port</Label>
                <Select value={port} onValueChange={setPort} disabled={formDisabled}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="22">22</SelectItem>
                    <SelectItem value="2222">2222</SelectItem>
                    <SelectItem value="custom">Custom</SelectItem>
                  </SelectContent>
                </Select>
                {port === "custom" && (
                  <Input
                    type="number"
                    value={customPort}
                    onChange={(e) => setCustomPort(e.target.value)}
                    placeholder="Port"
                    disabled={formDisabled}
                    className="mt-1"
                  />
                )}
              </div>
              <div className="space-y-1">
                <Label>User</Label>
                <Select value={sshUser} onValueChange={setSshUser} disabled={formDisabled}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="root">root</SelectItem>
                    <SelectItem value="ubuntu">ubuntu</SelectItem>
                    <SelectItem value="admin">admin</SelectItem>
                    <SelectItem value="custom">Custom</SelectItem>
                  </SelectContent>
                </Select>
                {sshUser === "custom" && (
                  <Input
                    value={customUser}
                    onChange={(e) => setCustomUser(e.target.value)}
                    placeholder="Username"
                    disabled={formDisabled}
                    className="mt-1"
                  />
                )}
              </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-1">
                <Label>Auth Type</Label>
                <Select value={authType} onValueChange={setAuthType} disabled={formDisabled}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="password">Password</SelectItem>
                    <SelectItem value="ssh_key">SSH Key</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1">
                <Label>Role</Label>
                <Select value={role} onValueChange={setRole} disabled={formDisabled}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="worker">Worker</SelectItem>
                    <SelectItem value="server">Server</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>

            {authType === "password" && (
              <div className="space-y-1">
                <Label htmlFor="node-password">Password</Label>
                <Input
                  id="node-password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="SSH password"
                  disabled={formDisabled}
                />
              </div>
            )}

            {authType === "ssh_key" && (
              <div className="space-y-1">
                <Label>SSH Key</Label>
                <Select value={sshKeyId} onValueChange={setSshKeyId} disabled={formDisabled}>
                  <SelectTrigger>
                    <SelectValue placeholder="Select SSH key" />
                  </SelectTrigger>
                  <SelectContent>
                    {(sshKeys ?? []).map((k) => (
                      <SelectItem key={k.id} value={k.id}>
                        {k.name}
                      </SelectItem>
                    ))}
                    {(!sshKeys || sshKeys.length === 0) && (
                      <SelectItem value="_none" disabled>
                        No SSH keys — add one in Resources
                      </SelectItem>
                    )}
                  </SelectContent>
                </Select>
              </div>
            )}
          </div>

          {/* Terminal output section */}
          <div
            ref={logRef}
            className="min-h-[160px] flex-1 overflow-y-auto rounded-md bg-muted p-3 font-mono text-xs text-foreground"
          >
            {logs.length === 0 ? (
              <span className="text-muted-foreground">Initialization logs will appear here...</span>
            ) : (
              logs.map((line, i) => <div key={`log-${i}`}>{line}</div>)
            )}
          </div>

          {/* Action button */}
          {phase === "done" ? (
            <Button onClick={() => onOpenChange(false)} className="w-full">
              Close
            </Button>
          ) : (
            <Button
              onClick={handleInitialize}
              disabled={phase === "initializing" || !name || !host}
              className="w-full"
            >
              {phase === "initializing"
                ? "Initializing..."
                : phase === "error"
                  ? "Retry"
                  : "Initialize"}
            </Button>
          )}
        </div>
      </SheetContent>
    </Sheet>
  );
}

// ── Pods Tab ─────────────────────────────────────────────────────

function PodsTab({ pods, namespaces }: { pods: PodInfo[]; namespaces: NamespaceInfo[] }) {
  const [nsFilter, setNsFilter] = useState("all");
  const [statusFilter, setStatusFilter] = useState("all");

  const filtered = useMemo(() => {
    let result = pods;
    // Note: namespace filter is a placeholder — PodInfo currently lacks a namespace field.
    // The Select is kept for future use when namespace is added to the pod model.
    if (statusFilter !== "all") {
      result = result.filter((p) => p.phase === statusFilter);
    }
    return result;
  }, [pods, statusFilter]);

  return (
    <div className="mt-3 space-y-3">
      <div className="flex gap-3">
        <Select value={nsFilter} onValueChange={setNsFilter}>
          <SelectTrigger className="w-[200px]">
            <SelectValue placeholder="All namespaces" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All Namespaces</SelectItem>
            {namespaces.map((ns) => (
              <SelectItem key={ns.name} value={ns.name}>
                {ns.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Select value={statusFilter} onValueChange={setStatusFilter}>
          <SelectTrigger className="w-[160px]">
            <SelectValue placeholder="All statuses" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All</SelectItem>
            <SelectItem value="Running">Running</SelectItem>
            <SelectItem value="Pending">Pending</SelectItem>
            <SelectItem value="Failed">Failed</SelectItem>
            <SelectItem value="Succeeded">Succeeded</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {filtered.length === 0 ? (
        <EmptyState icon={Box} message="No pods match the current filters" />
      ) : (
        <Card>
          <CardContent className="divide-y p-0">
            {filtered.map((pod) => (
              <div key={pod.name} className="flex items-center gap-4 px-4 py-3">
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="truncate font-mono text-sm">{pod.name}</span>
                    <Badge variant={statusVariant(pod.phase)}>{pod.phase}</Badge>
                    {pod.restart_count > 0 && (
                      <Badge variant="warning" className="text-xs">
                        {pod.restart_count} restarts
                      </Badge>
                    )}
                  </div>
                  <p className="text-xs text-muted-foreground">
                    {pod.node} · {pod.ip}
                    {pod.started_at ? ` · ${timeAgo(pod.started_at)}` : ""}
                  </p>
                </div>
                <div className="hidden gap-4 text-xs text-muted-foreground md:flex">
                  <span className="flex items-center gap-1">
                    <Cpu className="h-3 w-3" /> {pod.resources.cpu_used || "N/A"}
                  </span>
                  <span className="flex items-center gap-1">
                    <MemoryStick className="h-3 w-3" /> {pod.resources.mem_used || "N/A"}
                  </span>
                </div>
              </div>
            ))}
          </CardContent>
        </Card>
      )}
    </div>
  );
}

// ── Events Tab ───────────────────────────────────────────────────

function EventsTab() {
  const [page, setPage] = useState(1);
  const { data } = useMonitoringEvents(page);
  const events = data?.items ?? [];
  const total = data?.pagination?.total ?? 0;
  const totalPages = Math.ceil(total / 50);

  return (
    <div className="mt-3 space-y-3">
      <div className="flex items-center justify-between">
        <p className="text-xs text-muted-foreground">{total} events (last 30 days)</p>
        <div className="flex items-center gap-2">
          <Button
            size="sm"
            variant="outline"
            disabled={page <= 1}
            onClick={() => setPage((p) => p - 1)}
          >
            Prev
          </Button>
          <span className="text-xs text-muted-foreground">
            {page} / {totalPages || 1}
          </span>
          <Button
            size="sm"
            variant="outline"
            disabled={page >= totalPages}
            onClick={() => setPage((p) => p + 1)}
          >
            Next
          </Button>
        </div>
      </div>

      {events.length === 0 ? (
        <EmptyState icon={Activity} message="No events recorded" />
      ) : (
        <Card>
          <CardContent className="divide-y p-0">
            {events.map((event) => (
              <div key={event.id} className="px-4 py-3">
                <div className="flex items-center gap-2">
                  <span className="shrink-0 text-xs text-muted-foreground">
                    {timeAgo(event.last_seen)}
                  </span>
                  <Badge variant={eventVariant(event.event_type)}>{event.event_type}</Badge>
                  <span className="text-sm font-medium">{event.reason}</span>
                  {event.count > 1 && (
                    <span className="text-xs text-muted-foreground">x{event.count}</span>
                  )}
                </div>
                <p className="mt-1 text-sm text-muted-foreground">{event.message}</p>
                <div className="mt-1 flex items-center gap-2 text-xs text-muted-foreground">
                  <span className="font-mono">{event.involved_object}</span>
                  <span>·</span>
                  <Badge variant="outline" className="text-xs">
                    {event.namespace}
                  </Badge>
                </div>
              </div>
            ))}
          </CardContent>
        </Card>
      )}
    </div>
  );
}

// ── Storage Tab ──────────────────────────────────────────────────

function StorageTab({ pvcs }: { pvcs: PVCInfo[] }) {
  if (pvcs.length === 0)
    return <EmptyState icon={HardDrive} message="No persistent volume claims found" />;

  return (
    <div className="mt-3 space-y-3">
      {pvcs.map((pvc) => (
        <Card key={`${pvc.namespace}/${pvc.name}`}>
          <CardContent className="flex items-center gap-4 p-4">
            <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary/10">
              <Database className="h-4 w-4 text-primary" />
            </div>
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <span className="font-mono text-sm">{pvc.name}</span>
                <Badge variant={pvcStatusVariant(pvc.status)}>{pvc.status}</Badge>
                <Badge variant="outline" className="text-xs">
                  {pvc.namespace}
                </Badge>
              </div>
              <p className="text-xs text-muted-foreground">
                {pvc.capacity && `${pvc.capacity} · `}
                {pvc.storage_class && `${pvc.storage_class}`}
                {pvc.volume_name && ` · ${pvc.volume_name}`}
              </p>
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  );
}

// ── Namespaces Tab ───────────────────────────────────────────────

function NamespacesTab({ namespaces }: { namespaces: NamespaceInfo[] }) {
  if (namespaces.length === 0)
    return <EmptyState icon={FolderOpen} message="No namespaces found" />;

  return (
    <div className="mt-3 space-y-3">
      {namespaces.map((ns) => (
        <Card key={ns.name}>
          <CardContent className="flex items-center gap-4 p-4">
            <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary/10">
              <FolderOpen className="h-4 w-4 text-primary" />
            </div>
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium">{ns.name}</span>
                <Badge variant={ns.status === "Active" ? "success" : "secondary"}>
                  {ns.status}
                </Badge>
              </div>
              <p className="text-xs text-muted-foreground">
                {ns.pod_count} pods · {ns.svc_count} services
              </p>
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  );
}

// ── Helm Tab ────────────────────────────────────────────────────

function helmStatusVariant(status: string) {
  if (status === "deployed") return "success" as const;
  if (status === "failed") return "destructive" as const;
  if (status === "pending-install" || status === "pending-upgrade") return "warning" as const;
  return "secondary" as const;
}

function HelmTab() {
  const { data: releases, isLoading } = useHelmReleases();

  if (isLoading) return <LoadingScreen variant="detail" />;

  if (!releases || releases.length === 0) {
    return <EmptyState icon={Box} message="No Helm releases found" />;
  }

  return (
    <div className="mt-3 space-y-3">
      {releases.map((release: HelmRelease) => (
        <Card key={`${release.namespace}/${release.name}`}>
          <CardContent className="flex items-center gap-4 p-4">
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <span className="text-sm font-bold">{release.name}</span>
                <Badge variant="outline" className="text-xs">
                  {release.namespace}
                </Badge>
                <Badge variant={helmStatusVariant(release.status)} className="text-xs">
                  {release.status}
                </Badge>
              </div>
              <p className="text-xs text-muted-foreground">
                {release.chart} v{release.version}
                {release.app_version && ` · app v${release.app_version}`}
                {release.updated && ` · updated ${timeAgo(release.updated)}`}
              </p>
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  );
}

// ── DaemonSets Tab ──────────────────────────────────────────────

function DaemonSetsTab() {
  const { data: daemonsets, isLoading } = useDaemonSets();

  if (isLoading) return <LoadingScreen variant="detail" />;

  if (!daemonsets || daemonsets.length === 0) {
    return <EmptyState icon={Server} message="No DaemonSets found" />;
  }

  return (
    <div className="mt-3 space-y-3">
      {daemonsets.map((ds: DaemonSetInfo) => (
        <Card key={`${ds.namespace}/${ds.name}`}>
          <CardContent className="flex items-center gap-4 p-4">
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <span className="text-sm font-bold">{ds.name}</span>
                <Badge variant="outline" className="text-xs">
                  {ds.namespace}
                </Badge>
                <span className="text-xs text-muted-foreground">
                  {ds.ready}/{ds.desired_scheduled} ready
                </span>
              </div>
              <p className="truncate text-xs text-muted-foreground">
                {ds.node_selector && `selector: ${ds.node_selector} · `}
                {ds.images &&
                  `${ds.images.length > 80 ? `${ds.images.slice(0, 80)}...` : ds.images}`}
                {ds.created_at && ` · ${timeAgo(ds.created_at)}`}
              </p>
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  );
}

// ── Alerts Tab ──────────────────────────────────────────────────

function AlertsTab() {
  const { data: activeData } = useActiveAlerts();
  const resolve = useResolveAlert();
  const alerts = activeData?.alerts ?? [];

  return (
    <div className="mt-3 space-y-3">
      {alerts.length === 0 ? (
        <EmptyState icon={Activity} message="No active alerts" />
      ) : (
        alerts.map((alert: MetricAlert) => (
          <Card key={alert.id}>
            <CardContent className="flex items-center gap-4 p-4">
              <div
                className={`h-2.5 w-2.5 shrink-0 rounded-full ${alert.severity === "critical" ? "bg-destructive" : "bg-yellow-500"}`}
              />
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium">{alert.rule_name}</span>
                  <Badge
                    variant={alert.severity === "critical" ? "destructive" : "warning"}
                    className="text-xs"
                  >
                    {alert.severity}
                  </Badge>
                  <span className="text-xs text-muted-foreground">{alert.source_name}</span>
                </div>
                <p className="text-xs text-muted-foreground">{alert.message}</p>
                <p className="text-xs text-muted-foreground">
                  Fired {new Date(alert.fired_at).toLocaleString()}
                </p>
              </div>
              <Button
                size="sm"
                variant="outline"
                onClick={() => resolve.mutate(alert.id)}
                disabled={resolve.isPending}
              >
                Resolve
              </Button>
            </CardContent>
          </Card>
        ))
      )}
    </div>
  );
}

// ── Health / Cleanup Tab ──────────────────────────────────────────

interface CleanupRowProps {
  label: string;
  description: string;
  count: number;
  names: string[];
  variant: "red" | "yellow" | "green-zero";
  mutation: { mutate: () => void; isPending: boolean };
}

function CleanupRow({ label, description, count, names = [], variant, mutation }: CleanupRowProps) {
  const [expanded, setExpanded] = useState(false);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [confirmText, setConfirmText] = useState("");

  const dotColor = count === 0 ? "bg-green-500" : variant === "red" ? "bg-red-500" : "bg-amber-500";

  return (
    <>
      <div className="flex items-center justify-between gap-4 py-3">
        <div className="flex items-center gap-3 min-w-0">
          <span className={`h-2.5 w-2.5 shrink-0 rounded-full ${dotColor}`} />
          <div className="min-w-0">
            <p className="text-sm font-medium">{label}</p>
            <p className="text-xs text-muted-foreground">{description}</p>
          </div>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          {count === 0 ? (
            <span className="flex items-center gap-1 text-xs text-green-500">
              <CheckCircle2 className="h-3.5 w-3.5" /> All clean
            </span>
          ) : (
            <>
              <Badge variant="secondary" className="text-xs">
                {count}
              </Badge>
              <Button
                size="sm"
                variant="ghost"
                className="h-7 px-2 text-xs"
                onClick={() => setExpanded(!expanded)}
              >
                {expanded ? (
                  <ChevronDown className="h-3.5 w-3.5" />
                ) : (
                  <ChevronRight className="h-3.5 w-3.5" />
                )}
                View
              </Button>
              <Button
                size="sm"
                variant="destructive"
                className="h-7 px-2 text-xs"
                onClick={() => {
                  setConfirmText("");
                  setDialogOpen(true);
                }}
              >
                <Trash2 className="mr-1 h-3 w-3" /> Clean up
              </Button>
            </>
          )}
        </div>
      </div>

      {/* Expanded name list */}
      {expanded && count > 0 && (
        <div className="mb-2 ml-5.5 rounded-md border bg-muted/50 p-3">
          <div className="max-h-40 space-y-1 overflow-y-auto">
            {names.map((name) => (
              <p key={name} className="font-mono text-xs text-muted-foreground">
                {name}
              </p>
            ))}
          </div>
        </div>
      )}

      {/* Confirm dialog */}
      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Clean up {label.toLowerCase()}</DialogTitle>
            <DialogDescription>
              This will permanently delete {count} {label.toLowerCase()}.
            </DialogDescription>
          </DialogHeader>
          <div className="max-h-48 overflow-y-auto rounded-md border bg-muted/50 p-3">
            {names.map((name) => (
              <p key={name} className="font-mono text-xs text-muted-foreground">
                {name}
              </p>
            ))}
          </div>
          <div className="space-y-2">
            <Label className="text-sm">
              Type <span className="font-mono font-bold">DELETE</span> to confirm
            </Label>
            <Input
              value={confirmText}
              onChange={(e) => setConfirmText(e.target.value)}
              placeholder="DELETE"
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogOpen(false)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              disabled={confirmText !== "DELETE" || mutation.isPending}
              onClick={() => {
                mutation.mutate();
                setDialogOpen(false);
              }}
            >
              {mutation.isPending && <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />}
              Confirm cleanup
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}

function ClusterHealthTab() {
  const { data: stats, isLoading } = useCleanupStats();
  const cleanEvicted = useCleanupEvictedPods();
  const cleanFailed = useCleanupFailedPods();
  const cleanCompleted = useCleanupCompletedPods();
  const cleanStaleRS = useCleanupStaleReplicaSets();
  const cleanJobs = useCleanupCompletedJobs();
  const cleanOrphanIngresses = useCleanupOrphanIngresses();

  if (isLoading || !stats) {
    return (
      <div className="mt-6 flex items-center justify-center py-12">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  const totalIssues =
    stats.evicted_pods +
    stats.failed_pods +
    stats.completed_pods +
    stats.stale_replicasets +
    stats.orphan_ingresses +
    stats.completed_jobs +
    stats.unbound_pvcs;

  return (
    <div className="mt-3 space-y-4">
      {/* Overview card */}
      <Card>
        <CardContent className="flex items-center gap-3 py-4">
          <div
            className={`flex h-9 w-9 items-center justify-center rounded-full ${
              totalIssues === 0
                ? "bg-green-500/10"
                : totalIssues <= 5
                  ? "bg-amber-500/10"
                  : "bg-red-500/10"
            }`}
          >
            <HeartPulse
              className={`h-5 w-5 ${
                totalIssues === 0
                  ? "text-green-500"
                  : totalIssues <= 5
                    ? "text-amber-500"
                    : "text-red-500"
              }`}
            />
          </div>
          <div>
            <p className="text-sm font-medium">
              {totalIssues === 0
                ? "All healthy"
                : `${totalIssues} issue${totalIssues !== 1 ? "s" : ""} found`}
            </p>
            <p className="text-xs text-muted-foreground">
              {totalIssues === 0
                ? "No stale or orphaned resources detected"
                : "Review and clean up stale resources below"}
            </p>
          </div>
        </CardContent>
      </Card>

      {/* Pods section */}
      <Card>
        <CardHeader className="pb-0">
          <CardTitle className="text-sm">Pods</CardTitle>
          <CardDescription className="text-xs">Evicted, failed, and completed pods</CardDescription>
        </CardHeader>
        <CardContent className="divide-y">
          <CleanupRow
            label="Evicted pods"
            description="Pods evicted by the kubelet due to resource pressure"
            count={stats.evicted_pods}
            names={stats.evicted_pod_names}
            variant="red"
            mutation={cleanEvicted}
          />
          <CleanupRow
            label="Failed pods"
            description="Pods in a permanently failed state"
            count={stats.failed_pods}
            names={stats.failed_pod_names}
            variant="red"
            mutation={cleanFailed}
          />
          <CleanupRow
            label="Completed pods"
            description="Pods that have completed their execution"
            count={stats.completed_pods}
            names={stats.completed_pod_names}
            variant="yellow"
            mutation={cleanCompleted}
          />
        </CardContent>
      </Card>

      {/* Workloads section */}
      <Card>
        <CardHeader className="pb-0">
          <CardTitle className="text-sm">Workloads</CardTitle>
          <CardDescription className="text-xs">
            Stale ReplicaSets and completed Jobs
          </CardDescription>
        </CardHeader>
        <CardContent className="divide-y">
          <CleanupRow
            label="Stale ReplicaSets"
            description="Old ReplicaSets with zero replicas from previous rollouts"
            count={stats.stale_replicasets}
            names={stats.stale_rs_names}
            variant="yellow"
            mutation={cleanStaleRS}
          />
          <CleanupRow
            label="Completed Jobs"
            description="Jobs that have finished execution successfully"
            count={stats.completed_jobs}
            names={stats.completed_job_names}
            variant="yellow"
            mutation={cleanJobs}
          />
        </CardContent>
      </Card>

      {/* Networking section */}
      <Card>
        <CardHeader className="pb-0">
          <CardTitle className="text-sm">Networking</CardTitle>
          <CardDescription className="text-xs">Orphan ingress resources</CardDescription>
        </CardHeader>
        <CardContent>
          <CleanupRow
            label="Orphan Ingresses"
            description="Ingresses without a matching domain record in the database"
            count={stats.orphan_ingresses}
            names={stats.orphan_ingress_names}
            variant="red"
            mutation={cleanOrphanIngresses}
          />
        </CardContent>
      </Card>

      {/* Storage section */}
      <Card>
        <CardHeader className="pb-0">
          <CardTitle className="text-sm">Storage</CardTitle>
          <CardDescription className="text-xs">Unbound Persistent Volume Claims</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-between gap-4 py-3">
            <div className="flex items-center gap-3 min-w-0">
              <span
                className={`h-2.5 w-2.5 shrink-0 rounded-full ${stats.unbound_pvcs === 0 ? "bg-green-500" : "bg-amber-500"}`}
              />
              <div className="min-w-0">
                <p className="text-sm font-medium">Unbound PVCs</p>
                <p className="text-xs text-muted-foreground">
                  PVCs stuck in Pending state without a matching volume
                </p>
              </div>
            </div>
            <div className="flex items-center gap-2 shrink-0">
              {stats.unbound_pvcs === 0 ? (
                <span className="flex items-center gap-1 text-xs text-green-500">
                  <CheckCircle2 className="h-3.5 w-3.5" /> All clean
                </span>
              ) : (
                <>
                  <Badge variant="secondary" className="text-xs">
                    {stats.unbound_pvcs}
                  </Badge>
                  <Link to="/volumes" className="text-xs text-primary hover:underline">
                    Manage →
                  </Link>
                </>
              )}
            </div>
          </div>
          {stats.unbound_pvcs > 0 && stats.unbound_pvc_names?.length > 0 && (
            <div className="border-t px-3 py-2">
              <div className="max-h-24 overflow-y-auto space-y-0.5">
                {(stats.unbound_pvc_names ?? []).map((name) => (
                  <p key={name} className="font-mono text-xs text-muted-foreground">
                    {name}
                  </p>
                ))}
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
