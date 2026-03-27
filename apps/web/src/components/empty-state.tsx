import type { LucideIcon } from "lucide-react";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";

interface EmptyStateProps {
  icon: LucideIcon;
  message: string;
  actionLabel?: string;
  onAction?: () => void;
}

export function EmptyState({ icon: Icon, message, actionLabel, onAction }: EmptyStateProps) {
  return (
    <Card>
      <CardContent className="flex flex-col items-center py-16">
        <Icon className="h-12 w-12 text-muted-foreground/20" />
        <p className="mt-4 text-sm text-muted-foreground">{message}</p>
        {actionLabel && onAction && (
          <Button className="mt-4" variant="outline" size="sm" onClick={onAction}>
            <Plus className="h-4 w-4" /> {actionLabel}
          </Button>
        )}
      </CardContent>
    </Card>
  );
}
