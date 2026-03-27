import { createFileRoute } from "@tanstack/react-router";
import { ChevronRight, Clock, Play, Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { EmptyState } from "@/components/empty-state";
import { LoadingScreen } from "@/components/loading-screen";
import { PageHeader } from "@/components/page-header";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
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
import {
  useCreateCronJob,
  useDeleteCronJob,
  useProjectCronJobs,
  useTriggerCronJob,
} from "@/hooks/use-cronjobs";
import { useProjects } from "@/hooks/use-projects";
import { statusVariant } from "@/lib/constants";
import type { CronJob } from "@/types/api";

export const Route = createFileRoute("/_dashboard/cronjobs")({
  component: CronJobsPage,
});

function CronJobsPage() {
  const { data: rawProjects } = useProjects();
  const projects = rawProjects ?? [];
  const [selectedProject, setSelectedProject] = useState("");
  const projectId = selectedProject || projects[0]?.id || "";
  const { data: rawCronJobs, isLoading } = useProjectCronJobs(projectId);
  const cronJobs = rawCronJobs ?? [];
  const [showCreate, setShowCreate] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<CronJob | null>(null);

  return (
    <div>
      <PageHeader
        title="CronJobs"
        description="Scheduled tasks running on your K3s cluster."
        actions={
          projects.length > 0 ? (
            <Button size="sm" onClick={() => setShowCreate(true)}>
              <Plus className="h-4 w-4" /> New CronJob
            </Button>
          ) : undefined
        }
      />
      <Separator className="my-5" />

      {projects.length > 1 && (
        <div className="mb-4">
          <Select value={selectedProject} onValueChange={setSelectedProject}>
            <SelectTrigger className="w-52">
              <SelectValue placeholder="All projects" />
            </SelectTrigger>
            <SelectContent>
              {projects.map((p) => (
                <SelectItem key={p.id} value={p.id}>
                  {p.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      )}

      {isLoading ? (
        <LoadingScreen variant="detail" />
      ) : cronJobs.length === 0 ? (
        <EmptyState
          icon={Clock}
          message="No CronJobs yet."
          actionLabel="New CronJob"
          onAction={() => setShowCreate(true)}
        />
      ) : (
        <div className="space-y-2">
          {cronJobs.map((cj) => (
            <CronJobRow key={cj.id} cronJob={cj} onDelete={() => setDeleteTarget(cj)} />
          ))}
        </div>
      )}

      {showCreate && projectId && (
        <CreateCronJobDialog
          open={showCreate}
          onOpenChange={setShowCreate}
          projectId={projectId}
          projects={projects}
        />
      )}

      {deleteTarget && (
        <DeleteCronJobDialog cronJob={deleteTarget} onClose={() => setDeleteTarget(null)} />
      )}
    </div>
  );
}

function CronJobRow({ cronJob: cj, onDelete }: { cronJob: CronJob; onDelete: () => void }) {
  const trigger = useTriggerCronJob(cj.id);

  return (
    <Card className="group transition-colors hover:bg-accent/50">
      <CardContent className="flex items-center gap-4 p-4">
        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-primary/10">
          <Clock className="h-5 w-5 text-primary" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <span className="text-sm font-medium">{cj.name}</span>
            <Badge variant={statusVariant(cj.status)} className="text-xs">
              {cj.status}
            </Badge>
            {!cj.enabled && (
              <Badge variant="secondary" className="text-xs">
                suspended
              </Badge>
            )}
          </div>
          <div className="mt-0.5 flex items-center gap-3 text-xs text-muted-foreground">
            <span className="font-mono">{cj.cron_expression}</span>
            <span>{cj.image || cj.git_repo}</span>
            {cj.last_run_at && <span>Last: {new Date(cj.last_run_at).toLocaleString()}</span>}
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-1 opacity-0 transition-opacity group-hover:opacity-100">
          <Button
            size="icon"
            variant="ghost"
            className="h-8 w-8"
            onClick={() => trigger.mutate()}
            disabled={trigger.isPending}
            title="Run now"
          >
            <Play className="h-3.5 w-3.5" />
          </Button>
          <Button
            size="icon"
            variant="ghost"
            className="h-8 w-8 text-destructive hover:text-destructive"
            onClick={onDelete}
          >
            <Trash2 className="h-3.5 w-3.5" />
          </Button>
          <ChevronRight className="h-4 w-4 text-muted-foreground" />
        </div>
      </CardContent>
    </Card>
  );
}

function CreateCronJobDialog({
  open,
  onOpenChange,
  projectId,
  projects,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  projectId: string;
  projects: { id: string; name: string }[];
}) {
  const [selectedProjId, setSelectedProjId] = useState(projectId);
  const createCronJob = useCreateCronJob(selectedProjId);
  const [form, setForm] = useState({
    name: "",
    cron_expression: "0 * * * *",
    command: "",
    image: "busybox:latest",
    timezone: "UTC",
  });

  const update = (field: string, value: string) => setForm({ ...form, [field]: value });

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    await createCronJob.mutateAsync(form);
    onOpenChange(false);
    setForm({
      name: "",
      cron_expression: "0 * * * *",
      command: "",
      image: "busybox:latest",
      timezone: "UTC",
    });
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>New CronJob</DialogTitle>
          <DialogDescription>Create a scheduled task on your K3s cluster.</DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          {projects.length > 1 && (
            <div className="space-y-2">
              <Label>Project</Label>
              <Select value={selectedProjId} onValueChange={setSelectedProjId}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {projects.map((p) => (
                    <SelectItem key={p.id} value={p.id}>
                      {p.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}
          <div className="space-y-2">
            <Label>Name</Label>
            <Input
              value={form.name}
              onChange={(e) => update("name", e.target.value)}
              placeholder="cleanup-job"
              required
            />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label>Schedule (Cron)</Label>
              <Select
                value={form.cron_expression}
                onValueChange={(v) => update("cron_expression", v)}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="* * * * *">Every minute</SelectItem>
                  <SelectItem value="*/5 * * * *">Every 5 minutes</SelectItem>
                  <SelectItem value="*/15 * * * *">Every 15 minutes</SelectItem>
                  <SelectItem value="0 * * * *">Every hour</SelectItem>
                  <SelectItem value="0 0 * * *">Daily at midnight</SelectItem>
                  <SelectItem value="0 0 * * 0">Weekly (Sunday)</SelectItem>
                  <SelectItem value="0 0 1 * *">Monthly (1st)</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label>Timezone</Label>
              <Select value={form.timezone} onValueChange={(v) => update("timezone", v)}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="UTC">UTC</SelectItem>
                  <SelectItem value="Asia/Shanghai">Asia/Shanghai</SelectItem>
                  <SelectItem value="America/New_York">America/New_York</SelectItem>
                  <SelectItem value="Europe/London">Europe/London</SelectItem>
                  <SelectItem value="Asia/Tokyo">Asia/Tokyo</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <div className="space-y-2">
            <Label>Image</Label>
            <Input
              value={form.image}
              onChange={(e) => update("image", e.target.value)}
              placeholder="busybox:latest"
            />
          </div>
          <div className="space-y-2">
            <Label>Command</Label>
            <Input
              value={form.command}
              onChange={(e) => update("command", e.target.value)}
              placeholder="echo hello"
              required
            />
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={createCronJob.isPending}>
              {createCronJob.isPending ? "Creating..." : "Create"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function DeleteCronJobDialog({ cronJob, onClose }: { cronJob: CronJob; onClose: () => void }) {
  const deleteCronJob = useDeleteCronJob(cronJob.id);
  return (
    <ConfirmDialog
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title="Delete CronJob"
      description={
        <>
          Delete <strong>{cronJob.name}</strong>? This removes the K8s CronJob resource.
        </>
      }
      confirmLabel="Delete"
      loading={deleteCronJob.isPending}
      onConfirm={() => deleteCronJob.mutate(undefined, { onSuccess: onClose })}
    />
  );
}
