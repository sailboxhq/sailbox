import { ArrowRight, Box, Container, Globe, Lock, Network } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { statusVariant } from "@/lib/constants";
import type { App, AppStatus, Domain, PodInfo } from "@/types/api";

// ── Flow Node ───────────────────────────────────────────────────

function FlowNode({
  icon: Icon,
  title,
  children,
  highlight,
}: {
  icon: React.ElementType;
  title: string;
  children: React.ReactNode;
  highlight?: boolean;
}) {
  return (
    <div
      className={`rounded-lg border bg-card p-4 shadow-sm transition-all ${
        highlight ? "border-primary/40 shadow-primary/5" : ""
      }`}
    >
      <div className="mb-2 flex items-center gap-2">
        <div className="flex h-6 w-6 items-center justify-center rounded-md bg-primary/10">
          <Icon className="h-3.5 w-3.5 text-primary" />
        </div>
        <span className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
          {title}
        </span>
      </div>
      {children}
    </div>
  );
}

function FlowArrow() {
  return (
    <div className="flex items-center justify-center px-1">
      <ArrowRight className="h-4 w-4 text-muted-foreground/40" />
    </div>
  );
}

// ── Main ────────────────────────────────────────────────────────

export function TopologyTab({
  app,
  appStatus,
  pods,
  domains,
}: {
  app: App;
  appStatus?: AppStatus | null;
  pods: PodInfo[];
  domains: Domain[];
}) {
  const phase = appStatus?.phase ?? app.status;
  const readyReplicas = appStatus?.ready_replicas ?? 0;
  const desiredReplicas = appStatus?.desired_replicas ?? app.replicas;
  const port = app.ports?.[0];

  return (
    <div className="space-y-6">
      {/* Horizontal flow — large screens */}
      <div className="hidden items-stretch gap-1 lg:flex">
        {/* App */}
        <div className="flex-1">
          <FlowNode icon={Box} title="Application" highlight>
            <p className="text-sm font-medium">{app.name}</p>
            <Badge variant={statusVariant(phase)} className="mt-1 text-xs">
              {phase}
            </Badge>
            <p className="mt-2 text-xs text-muted-foreground">
              {app.source_type === "git" ? app.git_repo?.split("/").slice(-1)[0] : app.docker_image}
            </p>
          </FlowNode>
        </div>

        <FlowArrow />

        {/* Pods */}
        <div className="flex-1">
          <FlowNode icon={Container} title={`Pods (${readyReplicas}/${desiredReplicas})`}>
            {pods.length === 0 ? (
              <p className="text-xs text-muted-foreground">No pods</p>
            ) : (
              <div className="space-y-1.5">
                {pods.map((pod) => (
                  <div key={pod.name} className="flex items-center gap-2">
                    <span
                      className={`inline-block h-2 w-2 shrink-0 rounded-full ${
                        pod.phase === "Running"
                          ? "bg-green-500"
                          : pod.phase === "Pending"
                            ? "bg-yellow-500"
                            : "bg-red-500"
                      }`}
                    />
                    <span className="truncate font-mono text-xs">
                      {pod.name.split("-").slice(-1)[0]}
                    </span>
                    {pod.ip && <span className="text-xs text-muted-foreground">{pod.ip}</span>}
                  </div>
                ))}
              </div>
            )}
          </FlowNode>
        </div>

        <FlowArrow />

        {/* Service */}
        <div className="flex-1">
          <FlowNode icon={Network} title="Service">
            <p className="font-mono text-xs">{app.k8s_name || app.name}</p>
            {port && (
              <p className="mt-1 text-xs text-muted-foreground">
                :{port.container_port} → :{port.service_port} / {port.protocol}
              </p>
            )}
            <p className="mt-1 text-xs text-muted-foreground">ClusterIP</p>
          </FlowNode>
        </div>

        <FlowArrow />

        {/* Ingress / Domains */}
        <div className="flex-1">
          <FlowNode icon={Globe} title={`Domains (${domains.length})`}>
            {domains.length === 0 ? (
              <p className="text-xs text-muted-foreground">No domains configured</p>
            ) : (
              <div className="space-y-1.5">
                {domains.map((d) => (
                  <div key={d.id} className="flex items-center gap-1.5">
                    {d.tls && <Lock className="h-3 w-3 text-green-500" />}
                    <a
                      href={`${d.tls ? "https" : "http"}://${d.host}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="truncate text-xs text-primary hover:underline"
                    >
                      {d.host}
                    </a>
                    <span
                      className={`inline-block h-1.5 w-1.5 rounded-full ${d.ingress_ready ? "bg-green-500" : "bg-yellow-500"}`}
                    />
                  </div>
                ))}
              </div>
            )}
          </FlowNode>
        </div>
      </div>

      {/* Vertical flow — small screens */}
      <div className="space-y-3 lg:hidden">
        <FlowNode icon={Box} title="Application" highlight>
          <p className="text-sm font-medium">{app.name}</p>
          <Badge variant={statusVariant(phase)} className="mt-1 text-xs">
            {phase}
          </Badge>
        </FlowNode>

        <div className="flex justify-center">
          <ArrowRight className="h-4 w-4 rotate-90 text-muted-foreground/40" />
        </div>

        <FlowNode icon={Container} title={`Pods (${readyReplicas}/${desiredReplicas})`}>
          <div className="space-y-1">
            {pods.map((pod) => (
              <div key={pod.name} className="flex items-center gap-2">
                <span
                  className={`inline-block h-2 w-2 rounded-full ${pod.phase === "Running" ? "bg-green-500" : pod.phase === "Failed" ? "bg-red-500" : "bg-yellow-500"}`}
                />
                <span className="truncate font-mono text-xs">{pod.name}</span>
              </div>
            ))}
          </div>
        </FlowNode>

        <div className="flex justify-center">
          <ArrowRight className="h-4 w-4 rotate-90 text-muted-foreground/40" />
        </div>

        <FlowNode icon={Network} title="Service">
          <p className="font-mono text-xs">{app.k8s_name || app.name}</p>
          {port && <p className="text-xs text-muted-foreground">:{port.service_port}</p>}
        </FlowNode>

        <div className="flex justify-center">
          <ArrowRight className="h-4 w-4 rotate-90 text-muted-foreground/40" />
        </div>

        <FlowNode icon={Globe} title="Domains">
          {domains.length === 0 ? (
            <p className="text-xs text-muted-foreground">No domains</p>
          ) : (
            domains.map((d) => (
              <div key={d.id} className="flex items-center gap-1.5">
                {d.tls && <Lock className="h-3 w-3 text-green-500" />}
                <span className="text-xs">{d.host}</span>
              </div>
            ))
          )}
        </FlowNode>
      </div>

      {/* Resource summary */}
      <div className="grid gap-4 md:grid-cols-3">
        <div className="rounded-lg border p-3">
          <p className="text-xs font-medium text-muted-foreground">Namespace</p>
          <p className="mt-0.5 font-mono text-sm">{app.namespace || "default"}</p>
        </div>
        <div className="rounded-lg border p-3">
          <p className="text-xs font-medium text-muted-foreground">Replicas</p>
          <p className="mt-0.5 font-mono text-sm">
            {readyReplicas} / {desiredReplicas}
          </p>
        </div>
        <div className="rounded-lg border p-3">
          <p className="text-xs font-medium text-muted-foreground">Strategy</p>
          <p className="mt-0.5 font-mono text-sm">{app.deploy_strategy || "rolling"}</p>
        </div>
      </div>
    </div>
  );
}
