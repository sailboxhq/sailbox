import { createFileRoute } from "@tanstack/react-router";
import { HardDrive, Loader2, Maximize2, Trash2 } from "lucide-react";
import { useState } from "react";
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
import { Separator } from "@/components/ui/separator";
import { useClusterPVCs, useDeletePVC, useExpandPVC } from "@/hooks/use-cluster";
import { statusVariant } from "@/lib/constants";
import type { PVCInfo } from "@/types/api";

export const Route = createFileRoute("/_dashboard/volumes")({
  component: VolumesPage,
});

function VolumesPage() {
  const { data: pvcs, isLoading } = useClusterPVCs();

  return (
    <div>
      <PageHeader title="Volumes" description="Persistent volume claims across all namespaces." />
      <Separator className="my-5" />

      {isLoading ? (
        <div className="flex justify-center py-12">
          <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
        </div>
      ) : !pvcs || pvcs.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-10 text-muted-foreground">
            <div className="flex h-10 w-10 items-center justify-center rounded-full bg-primary/10">
              <HardDrive className="h-5 w-5 text-primary" />
            </div>
            <p className="mt-3 text-sm text-muted-foreground">No persistent volume claims found.</p>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-2">
          {pvcs.map((pvc) => (
            <PVCRow key={`${pvc.namespace}/${pvc.name}`} pvc={pvc} />
          ))}
        </div>
      )}
    </div>
  );
}

function PVCRow({ pvc }: { pvc: PVCInfo }) {
  const [expandOpen, setExpandOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);

  return (
    <>
      <Card className="transition-colors hover:bg-accent/50">
        <CardContent className="flex items-center gap-4 p-4">
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <span className="text-sm font-medium">{pvc.name}</span>
              <Badge variant="outline" className="text-xs">
                {pvc.namespace}
              </Badge>
              <Badge variant={statusVariant(pvc.status)} className="text-xs">
                {pvc.status}
              </Badge>
            </div>
            <div className="mt-1 flex items-center gap-3 text-xs text-muted-foreground">
              <span>Capacity: {pvc.capacity || "N/A"}</span>
              <span>Storage class: {pvc.storage_class || "default"}</span>
              {pvc.used_by && pvc.used_by.length > 0 && (
                <span>
                  Used by:{" "}
                  <span className="font-medium text-foreground">{pvc.used_by.join(", ")}</span>
                </span>
              )}
            </div>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            <Button variant="outline" size="sm" onClick={() => setExpandOpen(true)}>
              <Maximize2 className="mr-1.5 h-3.5 w-3.5" />
              Expand
            </Button>
            <Button variant="outline" size="sm" onClick={() => setDeleteOpen(true)}>
              <Trash2 className="mr-1.5 h-3.5 w-3.5" />
              Delete
            </Button>
          </div>
        </CardContent>
      </Card>

      <ExpandDialog pvc={pvc} open={expandOpen} onOpenChange={setExpandOpen} />
      <DeleteDialog pvc={pvc} open={deleteOpen} onOpenChange={setDeleteOpen} />
    </>
  );
}

function ExpandDialog({
  pvc,
  open,
  onOpenChange,
}: {
  pvc: PVCInfo;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const expandPVC = useExpandPVC();
  const [newSize, setNewSize] = useState("");

  function handleExpand() {
    if (!newSize) return;
    expandPVC.mutate(
      { namespace: pvc.namespace, name: pvc.name, size: newSize },
      { onSuccess: () => onOpenChange(false) },
    );
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Expand Volume</DialogTitle>
          <DialogDescription>
            Expand <span className="font-mono font-medium">{pvc.name}</span> in namespace{" "}
            <span className="font-mono font-medium">{pvc.namespace}</span>.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div>
            <Label className="text-xs text-muted-foreground">Current size</Label>
            <p className="text-sm font-medium">{pvc.capacity || "Unknown"}</p>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="new-size">New size</Label>
            <Input
              id="new-size"
              placeholder="e.g. 10Gi"
              value={newSize}
              onChange={(e) => setNewSize(e.target.value)}
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={handleExpand} disabled={!newSize || expandPVC.isPending}>
            {expandPVC.isPending ? "Expanding..." : "Expand"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function DeleteDialog({
  pvc,
  open,
  onOpenChange,
}: {
  pvc: PVCInfo;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const deletePVC = useDeletePVC();
  const [confirmName, setConfirmName] = useState("");
  const isBound = pvc.status === "Bound" && pvc.used_by && pvc.used_by.length > 0;

  function handleDelete() {
    deletePVC.mutate(
      { namespace: pvc.namespace, name: pvc.name },
      {
        onSuccess: () => {
          setConfirmName("");
          onOpenChange(false);
        },
      },
    );
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Delete Volume</DialogTitle>
          <DialogDescription>
            {isBound
              ? "This volume is currently in use by a running application."
              : `Permanently delete volume ${pvc.name}. This cannot be undone.`}
          </DialogDescription>
        </DialogHeader>
        {isBound ? (
          <div className="space-y-3 py-2">
            <div className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">
              <p>
                Cannot delete a volume that is in use. Stop the application first, then try again.
              </p>
              <p className="mt-1.5 font-medium">Used by: {pvc.used_by?.join(", ")}</p>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => onOpenChange(false)}>
                Close
              </Button>
            </DialogFooter>
          </div>
        ) : (
          <>
            <div className="space-y-1.5 py-2">
              <Label htmlFor="confirm-name">
                Type <span className="font-mono font-medium">{pvc.name}</span> to confirm
              </Label>
              <Input
                id="confirm-name"
                placeholder={pvc.name}
                value={confirmName}
                onChange={(e) => setConfirmName(e.target.value)}
              />
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => onOpenChange(false)}>
                Cancel
              </Button>
              <Button
                variant="destructive"
                onClick={handleDelete}
                disabled={confirmName !== pvc.name || deletePVC.isPending}
              >
                {deletePVC.isPending ? "Deleting..." : "Delete"}
              </Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
