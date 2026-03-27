import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

interface ConfirmDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description: React.ReactNode;
  confirmLabel?: string;
  secondaryLabel?: string;
  variant?: "destructive" | "default";
  loading?: boolean;
  onConfirm: () => void;
  onSecondary?: () => void;
}

export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  confirmLabel = "Confirm",
  secondaryLabel,
  variant = "destructive",
  loading,
  onConfirm,
  onSecondary,
}: ConfirmDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          {secondaryLabel && onSecondary && (
            <Button variant="outline" onClick={onSecondary} disabled={loading}>
              {loading ? "..." : secondaryLabel}
            </Button>
          )}
          <Button variant={variant} onClick={onConfirm} disabled={loading}>
            {loading ? "..." : confirmLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
