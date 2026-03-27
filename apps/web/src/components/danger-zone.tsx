import { AlertTriangle, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

interface DangerZoneProps {
  description: string;
  buttonLabel: string;
  onDelete: () => void;
}

export function DangerZone({ description, buttonLabel, onDelete }: DangerZoneProps) {
  return (
    <Card className="border-destructive/50">
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-sm text-destructive">
          <AlertTriangle className="h-4 w-4" /> Danger Zone
        </CardTitle>
      </CardHeader>
      <CardContent>
        <p className="text-sm text-muted-foreground">{description}</p>
        <Button variant="destructive" size="sm" className="mt-3" onClick={onDelete}>
          <Trash2 className="h-3.5 w-3.5" /> {buttonLabel}
        </Button>
      </CardContent>
    </Card>
  );
}
