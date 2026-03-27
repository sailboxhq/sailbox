import { createFileRoute, Link } from "@tanstack/react-router";
import { Clock, Filter, Layers, Loader2 } from "lucide-react";
import { useState } from "react";
import { PageHeader } from "@/components/page-header";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useAllDeployments, useDeploymentQueue } from "@/hooks/use-deployments";
import { statusVariant } from "@/lib/constants";
import type { Deployment } from "@/types/api";

export const Route = createFileRoute("/_dashboard/deployments")({
  component: DeploymentsPage,
});

function DeploymentsPage() {
  const [page, setPage] = useState(1);
  const [statusFilter, setStatusFilter] = useState<string>("");
  const { data: queueData } = useDeploymentQueue();
  const { data: allData, isLoading } = useAllDeployments(page, 20, statusFilter || undefined);

  const queue = queueData?.items ?? [];
  const all = allData?.items ?? [];
  const pagination = allData?.pagination;
  const totalPages = pagination ? Math.ceil(pagination.total / pagination.per_page) : 1;

  return (
    <div>
      <PageHeader
        title="Deployments"
        description="Build and deploy history across all applications."
      />
      <Separator className="my-5" />

      <Tabs defaultValue="all">
        <TabsList>
          <TabsTrigger value="all" className="gap-1.5">
            <Layers className="h-3.5 w-3.5" />
            All
          </TabsTrigger>
          <TabsTrigger value="queue" className="gap-1.5">
            <Loader2 className="h-3.5 w-3.5" />
            Queue
            {queue.length > 0 && (
              <Badge variant="warning" className="ml-1 h-5 px-1.5 text-xs">
                {queue.length}
              </Badge>
            )}
          </TabsTrigger>
        </TabsList>

        <TabsContent value="queue" className="mt-4">
          {queue.length === 0 ? (
            <Card>
              <CardContent className="flex flex-col items-center justify-center py-10 text-muted-foreground">
                <div className="flex h-10 w-10 items-center justify-center rounded-full bg-primary/10">
                  <Clock className="h-5 w-5 text-primary" />
                </div>
                <p className="mt-3 text-sm text-muted-foreground">
                  No active deployments in queue.
                </p>
              </CardContent>
            </Card>
          ) : (
            <div className="space-y-4">
              {queue.map((d) => (
                <DeploymentRow key={d.id} deploy={d} />
              ))}
            </div>
          )}
        </TabsContent>

        <TabsContent value="all" className="mt-4 space-y-4">
          <div className="flex items-center gap-2">
            <Filter className="h-4 w-4 text-muted-foreground" />
            <Select
              value={statusFilter || "all"}
              onValueChange={(v) => {
                setStatusFilter(v === "all" ? "" : v);
                setPage(1);
              }}
            >
              <SelectTrigger className="w-40">
                <SelectValue placeholder="All statuses" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All</SelectItem>
                <SelectItem value="queued">Queued</SelectItem>
                <SelectItem value="building">Building</SelectItem>
                <SelectItem value="deploying">Deploying</SelectItem>
                <SelectItem value="success">Success</SelectItem>
                <SelectItem value="failed">Failed</SelectItem>
                <SelectItem value="cancelled">Cancelled</SelectItem>
              </SelectContent>
            </Select>
          </div>

          {isLoading ? (
            <div className="flex justify-center py-12">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          ) : all.length === 0 ? (
            <Card>
              <CardContent className="flex flex-col items-center justify-center py-10 text-muted-foreground">
                <div className="flex h-10 w-10 items-center justify-center rounded-full bg-primary/10">
                  <Layers className="h-5 w-5 text-primary" />
                </div>
                <p className="mt-3 text-sm text-muted-foreground">No deployments found.</p>
              </CardContent>
            </Card>
          ) : (
            <>
              <div className="space-y-4">
                {all.map((d) => (
                  <DeploymentRow key={d.id} deploy={d} />
                ))}
              </div>
              {pagination && totalPages > 1 && (
                <div className="flex items-center justify-between">
                  <p className="text-xs text-muted-foreground">
                    Page {pagination.page} of {totalPages} ({pagination.total} total)
                  </p>
                  <div className="flex gap-2">
                    <Button
                      size="sm"
                      variant="outline"
                      disabled={page <= 1}
                      onClick={() => setPage(page - 1)}
                    >
                      Previous
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      disabled={page >= totalPages}
                      onClick={() => setPage(page + 1)}
                    >
                      Next
                    </Button>
                  </div>
                </div>
              )}
            </>
          )}
        </TabsContent>
      </Tabs>
    </div>
  );
}

function DeploymentRow({ deploy }: { deploy: Deployment }) {
  const duration =
    deploy.started_at && deploy.finished_at
      ? formatDuration(
          new Date(deploy.finished_at).getTime() - new Date(deploy.started_at).getTime(),
        )
      : deploy.started_at
        ? "running..."
        : "-";

  const linkTo = deploy.project_id
    ? `/projects/${deploy.project_id}/apps/${deploy.app_id}`
    : undefined;

  const Wrapper = linkTo ? Link : "div";
  const wrapperProps = linkTo ? { to: linkTo, className: "block" } : {};

  return (
    <Wrapper {...(wrapperProps as any)}>
      <Card className="transition-colors hover:bg-accent/50">
        <CardContent className="flex items-center gap-4 p-4">
          {/* Status badge */}
          <Badge variant={statusVariant(deploy.status)} className="w-20 justify-center text-xs">
            {deploy.status}
          </Badge>

          {/* App name + commit */}
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-medium">{deploy.app_name || "Unknown App"}</p>
            {deploy.commit_sha && (
              <p className="mt-0.5 font-mono text-xs text-muted-foreground">
                {deploy.commit_sha.slice(0, 7)}
              </p>
            )}
          </div>

          {/* Trigger */}
          <Badge variant="outline" className="shrink-0 text-xs">
            {deploy.trigger_type}
          </Badge>

          {/* Duration */}
          <span className="w-16 shrink-0 text-right text-xs text-muted-foreground">{duration}</span>

          {/* Time */}
          <span className="w-36 shrink-0 text-right text-xs text-muted-foreground">
            {new Date(deploy.created_at).toLocaleString()}
          </span>
        </CardContent>
      </Card>
    </Wrapper>
  );
}

function formatDuration(ms: number): string {
  const s = Math.floor(ms / 1000);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  const sec = s % 60;
  return `${m}m ${sec}s`;
}
