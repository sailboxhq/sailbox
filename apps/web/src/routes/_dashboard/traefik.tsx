import { createFileRoute } from "@tanstack/react-router";
import { AlertTriangle, Check, Copy, Loader2, Lock, Unlock } from "lucide-react";
import { useState } from "react";
import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import { useTraefikConfig, useUpdateTraefikConfig } from "@/hooks/use-cluster";

export const Route = createFileRoute("/_dashboard/traefik")({
  component: TraefikPage,
});

function TraefikPage() {
  const { data, isLoading, isError } = useTraefikConfig();
  const updateConfig = useUpdateTraefikConfig();
  const [unlocked, setUnlocked] = useState(false);
  const [yaml, setYaml] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const displayYaml = yaml ?? data?.yaml ?? "";

  function handleToggleLock() {
    if (unlocked) {
      // Re-lock: discard edits
      setYaml(null);
      setUnlocked(false);
    } else {
      // Unlock: start editing from current value
      setYaml(data?.yaml ?? "");
      setUnlocked(true);
    }
  }

  function handleSave() {
    if (!displayYaml.trim()) return;
    updateConfig.mutate(displayYaml, {
      onSuccess: () => {
        setYaml(null);
        setUnlocked(false);
      },
    });
  }

  function handleCopy() {
    navigator.clipboard.writeText(displayYaml).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  }

  return (
    <div>
      <PageHeader
        title="Traefik"
        description="Ingress controller configuration."
        actions={
          <div className="flex items-center gap-2">
            <Button variant="outline" size="sm" onClick={handleCopy} disabled={!displayYaml}>
              {copied ? (
                <Check className="mr-1.5 h-3.5 w-3.5" />
              ) : (
                <Copy className="mr-1.5 h-3.5 w-3.5" />
              )}
              {copied ? "Copied" : "Copy"}
            </Button>
            <Button
              variant={unlocked ? "destructive" : "outline"}
              size="sm"
              onClick={handleToggleLock}
              disabled={isLoading || isError}
            >
              {unlocked ? (
                <Unlock className="mr-1.5 h-3.5 w-3.5" />
              ) : (
                <Lock className="mr-1.5 h-3.5 w-3.5" />
              )}
              {unlocked ? "Lock" : "Unlock"}
            </Button>
            {unlocked && (
              <Button size="sm" onClick={handleSave} disabled={updateConfig.isPending}>
                {updateConfig.isPending ? (
                  <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                ) : null}
                Save
              </Button>
            )}
          </div>
        }
      />
      <Separator className="my-5" />

      {unlocked && (
        <div className="mb-4 flex items-center gap-2 rounded-md border border-yellow-500/30 bg-yellow-500/10 px-4 py-3 text-sm text-yellow-400">
          <AlertTriangle className="h-4 w-4 shrink-0" />
          Editing Traefik config may affect all services. Proceed with caution.
        </div>
      )}

      {isLoading ? (
        <div className="flex justify-center py-12">
          <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
        </div>
      ) : isError ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12 text-muted-foreground">
            <div className="mb-2 flex h-10 w-10 items-center justify-center rounded-full bg-destructive/10">
              <AlertTriangle className="h-5 w-5 text-destructive" />
            </div>
            <p className="text-sm">Failed to load Traefik configuration.</p>
          </CardContent>
        </Card>
      ) : (
        <Card>
          <CardContent className="p-0">
            <textarea
              className="min-h-[500px] w-full resize-y rounded-md bg-muted p-4 font-mono text-sm text-foreground outline-none focus:ring-1 focus:ring-ring"
              value={displayYaml}
              onChange={(e) => setYaml(e.target.value)}
              readOnly={!unlocked}
              spellCheck={false}
            />
          </CardContent>
        </Card>
      )}
    </div>
  );
}
