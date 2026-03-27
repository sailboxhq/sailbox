import Dagre from "@dagrejs/dagre";
import {
  Background,
  type Edge,
  Handle,
  type Node,
  Panel,
  Position,
  ReactFlow,
  ReactFlowProvider,
  useEdgesState,
  useNodesState,
  useReactFlow,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { Box, Container, Globe, Network, Server } from "lucide-react";
import { useCallback, useEffect, useMemo } from "react";
import type { ClusterTopology as TopoData } from "@/types/api";

// ── Theme ───────────────────────────────────────────────────────

const TYPE_META: Record<string, { icon: React.ElementType; color: string; label: string }> = {
  ingress: { icon: Globe, color: "#ec4899", label: "Ingress" },
  service: { icon: Network, color: "#f59e0b", label: "Service" },
  deployment: { icon: Box, color: "#3b82f6", label: "Deployment" },
  pod: { icon: Container, color: "#22c55e", label: "Pod" },
  node: { icon: Server, color: "#6d5cdb", label: "Node" },
};

// ── Custom node ─────────────────────────────────────────────────

function TopoNode({
  data,
}: {
  data: { type: string; label: string; sublabel: string; status: string };
}) {
  const meta = TYPE_META[data.type] ?? TYPE_META.pod;
  const Icon = meta.icon;
  const isOk =
    data.status === "Running" ||
    data.status === "Ready" ||
    data.status === "active" ||
    data.status === "Succeeded";

  return (
    <div
      className="relative rounded-lg border bg-card px-3 py-2 shadow-sm transition-shadow hover:shadow-md"
      style={{ borderLeftColor: meta.color, borderLeftWidth: 3, minWidth: 170, maxWidth: 220 }}
    >
      <Handle
        type="target"
        position={Position.Left}
        className="!h-1.5 !w-1.5 !border-none !bg-transparent"
      />
      <Handle
        type="source"
        position={Position.Right}
        className="!h-1.5 !w-1.5 !border-none !bg-transparent"
      />
      <div className="flex items-center gap-2">
        <Icon className="h-3.5 w-3.5 shrink-0" style={{ color: meta.color }} />
        <span className="truncate text-[11px] font-medium">{data.label}</span>
        <span
          className={`ml-auto inline-block h-2 w-2 shrink-0 rounded-full ${isOk ? "bg-green-500" : "bg-yellow-500"}`}
        />
      </div>
      {data.sublabel && (
        <p className="mt-0.5 truncate text-[9px] text-muted-foreground">{data.sublabel}</p>
      )}
    </div>
  );
}

const nodeTypes = { topo: TopoNode };

// ── Dagre auto-layout ───────────────────────────────────────────

const NODE_WIDTH = 190;
const NODE_HEIGHT = 48;

function applyDagreLayout(nodes: Node[], edges: Edge[]): Node[] {
  const g = new Dagre.graphlib.Graph().setDefaultEdgeLabel(() => ({}));
  g.setGraph({
    rankdir: "LR", // left to right
    ranksep: 100, // horizontal spacing between ranks
    nodesep: 30, // vertical spacing within a rank
    edgesep: 20,
    marginx: 20,
    marginy: 20,
  });

  for (const node of nodes) {
    g.setNode(node.id, { width: NODE_WIDTH, height: NODE_HEIGHT });
  }
  for (const edge of edges) {
    g.setEdge(edge.source, edge.target);
  }

  Dagre.layout(g);

  return nodes.map((node) => {
    const pos = g.node(node.id);
    return {
      ...node,
      position: {
        x: pos.x - NODE_WIDTH / 2,
        y: pos.y - NODE_HEIGHT / 2,
      },
    };
  });
}

// ── Edge factory ────────────────────────────────────────────────

function makeEdge(
  id: string,
  source: string,
  target: string,
  color: string,
  animated = false,
): Edge {
  return {
    id,
    source,
    target,
    type: "smoothstep",
    animated,
    style: { stroke: color, strokeWidth: 1.5, opacity: 0.5 },
    pathOptions: { borderRadius: 12 },
  };
}

// ── Build graph from API data ───────────────────────────────────

function buildGraph(topo: TopoData): { nodes: Node[]; edges: Edge[] } {
  const rawNodes: Node[] = [];
  const edges: Edge[] = [];

  // Nodes (infrastructure)
  topo.nodes?.forEach((n) => {
    rawNodes.push({
      id: `node-${n.name}`,
      type: "topo",
      position: { x: 0, y: 0 },
      data: {
        type: "node",
        label: n.name,
        sublabel: `${n.ip} · ${n.roles || "worker"}`,
        status: n.status,
      },
    });
  });

  // Deployments
  topo.deployments?.forEach((d) => {
    rawNodes.push({
      id: `dep-${d.namespace}-${d.name}`,
      type: "topo",
      position: { x: 0, y: 0 },
      data: {
        type: "deployment",
        label: d.name,
        sublabel: `${d.ready}/${d.desired} ready · ${d.namespace}`,
        status: d.ready >= d.desired ? "Running" : "Pending",
      },
    });
  });

  // Pods
  topo.pods?.forEach((p) => {
    const id = `pod-${p.namespace}-${p.name}`;
    rawNodes.push({
      id,
      type: "topo",
      position: { x: 0, y: 0 },
      data: {
        type: "pod",
        label: p.name.split("-").slice(-2).join("-"),
        sublabel: `${p.ip || "—"} · ${p.node}`,
        status: p.phase,
      },
    });

    // Deployment → Pod
    if (p.deployment) {
      edges.push(
        makeEdge(
          `e-dep-pod-${p.name}`,
          `dep-${p.namespace}-${p.deployment}`,
          id,
          TYPE_META.pod.color,
          p.phase !== "Running",
        ),
      );
    }

    // Pod → Node
    if (p.node) {
      edges.push({
        id: `e-pod-node-${p.name}`,
        source: id,
        target: `node-${p.node}`,
        type: "smoothstep",
        style: { stroke: "#9ca3af", strokeWidth: 1, opacity: 0.2, strokeDasharray: "4 2" },
      });
    }
  });

  // Services
  topo.services?.forEach((s) => {
    const id = `svc-${s.namespace}-${s.name}`;
    rawNodes.push({
      id,
      type: "topo",
      position: { x: 0, y: 0 },
      data: {
        type: "service",
        label: s.name,
        sublabel: `${s.cluster_ip} · ${s.ports}`,
        status: "active",
      },
    });

    // Service → Deployment (match by name)
    const dep = topo.deployments?.find((d) => d.name === s.name);
    if (dep) {
      edges.push(
        makeEdge(
          `e-svc-dep-${s.name}`,
          id,
          `dep-${dep.namespace}-${dep.name}`,
          TYPE_META.service.color,
        ),
      );
    }
  });

  // Ingresses
  topo.ingresses?.forEach((ing, i) => {
    const id = `ing-${i}`;
    rawNodes.push({
      id,
      type: "topo",
      position: { x: 0, y: 0 },
      data: {
        type: "ingress",
        label: ing.host.length > 30 ? `${ing.host.slice(0, 28)}...` : ing.host,
        sublabel: `→ ${ing.service}`,
        status: "active",
      },
    });

    // Ingress → Service
    if (ing.service) {
      edges.push(
        makeEdge(
          `e-ing-svc-${i}`,
          id,
          `svc-${ing.namespace}-${ing.service}`,
          TYPE_META.ingress.color,
        ),
      );
    }
  });

  // Apply Dagre layout
  const layoutNodes = applyDagreLayout(rawNodes, edges);
  return { nodes: layoutNodes, edges };
}

// ── React Flow wrapper ──────────────────────────────────────────

function TopologyInner({ data }: { data: TopoData }) {
  const { nodes: initNodes, edges: initEdges } = useMemo(() => buildGraph(data), [data]);
  const [nodes, setNodes, onNodesChange] = useNodesState(initNodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState(initEdges);
  const { fitView } = useReactFlow();

  useEffect(() => {
    setNodes(initNodes);
    setEdges(initEdges);
    // Defer fitView to next frame so layout is applied
    requestAnimationFrame(() => fitView({ padding: 0.12 }));
  }, [initNodes, initEdges, setNodes, setEdges, fitView]);

  const onInit = useCallback(() => {
    fitView({ padding: 0.12 });
  }, [fitView]);

  return (
    <div className="h-[540px] w-full overflow-hidden rounded-lg border bg-card">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onInit={onInit}
        nodeTypes={nodeTypes}
        fitView
        fitViewOptions={{ padding: 0.12 }}
        minZoom={0.2}
        maxZoom={2.5}
        proOptions={{ hideAttribution: true }}
        nodesDraggable
        nodesConnectable={false}
        elementsSelectable={false}
        defaultEdgeOptions={{ type: "smoothstep" }}
      >
        <Background gap={16} size={1} className="opacity-30" />
        <Panel position="top-left">
          <div className="flex items-center gap-3 rounded-md border bg-card/90 px-3 py-1.5 text-[10px] shadow-sm backdrop-blur">
            {(["ingress", "service", "deployment", "pod", "node"] as const).map((type) => {
              const m = TYPE_META[type];
              const Icon = m.icon;
              return (
                <span key={type} className="flex items-center gap-1">
                  <Icon className="h-3 w-3" style={{ color: m.color }} />
                  {m.label}
                </span>
              );
            })}
          </div>
        </Panel>
      </ReactFlow>
    </div>
  );
}

export function ClusterTopologyView({ data }: { data: TopoData }) {
  return (
    <ReactFlowProvider>
      <TopologyInner data={data} />
    </ReactFlowProvider>
  );
}
