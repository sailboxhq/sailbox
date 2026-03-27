import { Check, Database, HardDrive, Plus, Save, Unplug } from "lucide-react";
import { useEffect, useState } from "react";
import { toast } from "sonner";
import { ConfirmDialog } from "@/components/confirm-dialog";
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
import { useDeploy, useUpdateApp } from "@/hooks/use-apps";
import { COMMON_MOUNT_PATHS, VOLUME_SIZE_OPTIONS } from "@/lib/constants";
import type { App, VolumeMount } from "@/types/api";

export function VolumesTab({ app, appId }: { app: App; appId: string }) {
  const updateApp = useUpdateApp(appId);
  const deploy = useDeploy(appId);
  const [volumes, setVolumes] = useState<VolumeMount[]>(app.volumes ?? []);
  const [showRedeploy, setShowRedeploy] = useState(false);

  // Sync with server data after save
  useEffect(() => {
    setVolumes(app.volumes ?? []);
  }, [app.volumes]);

  // Add volume form
  const [newName, setNewName] = useState("");
  const [newPath, setNewPath] = useState("/data");
  const [newPathCustom, setNewPathCustom] = useState("");
  const [newSize, setNewSize] = useState("5Gi");
  const [useCustomPath, setUseCustomPath] = useState(false);

  // Delete confirmation
  const [deleteIdx, setDeleteIdx] = useState<number | null>(null);

  const serialize = (v: VolumeMount[]) => v.map((x) => `${x.mount_path}:${x.size}`).join(",");
  const dirty = serialize(volumes) !== serialize(app.volumes ?? []);

  const effectivePath = useCustomPath ? newPathCustom : newPath;

  function autoName(path: string): string {
    return path.replace(/^\//, "").replace(/\//g, "-") || "data";
  }

  function addVolume() {
    const path = effectivePath.trim();
    if (!path) {
      toast.error("Mount path is required");
      return;
    }
    if (!path.startsWith("/")) {
      toast.error("Mount path must start with /");
      return;
    }
    if (volumes.some((v) => v.mount_path === path)) {
      toast.error("Mount path already in use");
      return;
    }
    const name = newName.trim() || autoName(path);
    setVolumes([...volumes, { name, mount_path: path, size: newSize }]);
    setNewName("");
    setNewPathCustom("");
    setUseCustomPath(false);
  }

  function confirmDelete(idx: number) {
    setDeleteIdx(idx);
  }

  function handleDelete() {
    if (deleteIdx === null) return;
    setVolumes(volumes.filter((_, i) => i !== deleteIdx));
    setDeleteIdx(null);
  }

  function handleSave() {
    updateApp.mutate(
      { volumes },
      {
        onSuccess: () => setShowRedeploy(true),
      },
    );
  }

  return (
    <div className="space-y-6">
      {/* Mounted Volumes */}
      <Card>
        <CardHeader className="flex flex-row items-start justify-between gap-4">
          <div className="flex items-start gap-3">
            <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary/10">
              <HardDrive className="h-4 w-4 text-primary" />
            </div>
            <div>
              <CardTitle className="text-sm">Mounted Volumes</CardTitle>
              <CardDescription className="text-xs">
                Persistent storage that survives pod restarts and redeployments. Changes require a
                redeploy to take effect.
              </CardDescription>
            </div>
          </div>
          {dirty && (
            <Button size="sm" onClick={handleSave} disabled={updateApp.isPending}>
              <Save className="h-3.5 w-3.5" /> {updateApp.isPending ? "Saving..." : "Save"}
            </Button>
          )}
        </CardHeader>
        <CardContent>
          {volumes.length === 0 ? (
            <div className="flex flex-col items-center py-10 text-center">
              <HardDrive className="h-10 w-10 text-muted-foreground/20" />
              <p className="mt-3 text-sm text-muted-foreground">
                No volumes mounted. Add a volume below to persist data.
              </p>
            </div>
          ) : (
            <div className="space-y-2">
              {volumes.map((vol, i) => (
                <div
                  key={vol.mount_path}
                  className="flex items-center gap-4 rounded-lg border px-4 py-3 transition-colors hover:bg-accent/30"
                >
                  <Database className="h-4 w-4 shrink-0 text-primary" />
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span className="font-mono text-sm font-medium">{vol.mount_path}</span>
                      {vol.pvc_name && (
                        <Badge variant="success" className="text-xs">
                          <Check className="mr-0.5 h-2.5 w-2.5" />
                          Bound
                        </Badge>
                      )}
                    </div>
                    <div className="mt-0.5 flex items-center gap-3 text-xs text-muted-foreground">
                      <span>Name: {vol.name}</span>
                      <span>Size: {vol.size}</span>
                      {vol.pvc_name && <span className="font-mono">PVC: {vol.pvc_name}</span>}
                    </div>
                  </div>
                  <Badge variant="secondary">{vol.size}</Badge>
                  <Button
                    size="icon"
                    variant="ghost"
                    className="h-7 w-7 shrink-0 text-muted-foreground hover:text-destructive"
                    onClick={() => confirmDelete(i)}
                    title="Unmount volume"
                  >
                    <Unplug className="h-3 w-3" />
                  </Button>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Add Volume Form */}
      <Card>
        <CardHeader>
          <div className="flex items-start gap-3">
            <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary/10">
              <Plus className="h-4 w-4 text-primary" />
            </div>
            <div>
              <CardTitle className="text-sm">Add Volume</CardTitle>
              <CardDescription className="text-xs">
                Mount a new persistent volume to this application.
              </CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          {/* Mount path */}
          <div className="space-y-1.5">
            <div className="flex items-center justify-between">
              <Label className="text-xs">Mount Path</Label>
              <button
                type="button"
                onClick={() => setUseCustomPath(!useCustomPath)}
                className="text-xs text-primary hover:underline"
              >
                {useCustomPath ? "Use preset" : "Custom path"}
              </button>
            </div>
            {useCustomPath ? (
              <Input
                value={newPathCustom}
                onChange={(e) => setNewPathCustom(e.target.value)}
                placeholder="/app/data"
                className="font-mono text-sm"
              />
            ) : (
              <Select value={newPath} onValueChange={setNewPath}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {COMMON_MOUNT_PATHS.map((p) => (
                    <SelectItem key={p.value} value={p.value}>
                      {p.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            {/* Size */}
            <div className="space-y-1.5">
              <Label className="text-xs">Size</Label>
              <Select value={newSize} onValueChange={setNewSize}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {VOLUME_SIZE_OPTIONS.map((s) => (
                    <SelectItem key={s.value} value={s.value}>
                      {s.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            {/* Name (optional) */}
            <div className="space-y-1.5">
              <Label className="text-xs">
                Name <span className="text-muted-foreground">(optional)</span>
              </Label>
              <Input
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                placeholder={autoName(effectivePath || "/data")}
                className="text-sm"
              />
            </div>
          </div>

          <Button onClick={addVolume} variant="outline" className="w-full">
            <Plus className="h-3.5 w-3.5" /> Add Volume
          </Button>

          {dirty && (
            <p className="text-center text-xs text-muted-foreground">
              Click <strong>Save</strong> above to apply changes. A redeploy is needed for volumes
              to take effect.
            </p>
          )}
        </CardContent>
      </Card>

      {/* Redeploy prompt */}
      <ConfirmDialog
        open={showRedeploy}
        onOpenChange={setShowRedeploy}
        title="Redeploy Required"
        description="Volume changes are saved. Redeploy now to apply them to the running application?"
        confirmLabel="Redeploy"
        onConfirm={() => {
          deploy.mutate();
          setShowRedeploy(false);
        }}
      />

      {/* Unmount confirmation */}
      <ConfirmDialog
        open={deleteIdx !== null}
        onOpenChange={(open) => {
          if (!open) setDeleteIdx(null);
        }}
        title="Unmount Volume"
        description={
          deleteIdx !== null ? (
            <>
              Unmount <strong className="font-mono">{volumes[deleteIdx]?.mount_path}</strong> from
              this application? The underlying PVC and its data will be preserved. You can delete
              the PVC later from Infrastructure → Volumes. A redeploy is required.
            </>
          ) : (
            ""
          )
        }
        confirmLabel="Unmount"
        onConfirm={handleDelete}
      />
    </div>
  );
}
