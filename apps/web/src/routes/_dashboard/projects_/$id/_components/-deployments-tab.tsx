import {
  Check,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  Clock,
  Container,
  Copy,
  Eye,
  EyeOff,
  GitBranch,
  Loader2,
  RefreshCw,
  Rocket,
  RotateCcw,
  Square,
  XCircle,
} from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { EmptyState } from "@/components/empty-state";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  useCancelDeploy,
  useDeploy,
  useDeploymentDetail,
  useDisableWebhook,
  useEnableWebhook,
  useRegenerateWebhook,
  useWebhookConfig,
} from "@/hooks/use-apps";
import { statusVariant } from "@/lib/constants";
import type { App, Deployment } from "@/types/api";

// ── Helpers ────────────────────────────────────────────────────────

function formatDuration(start: string, end: string): string {
  const ms = new Date(end).getTime() - new Date(start).getTime();
  if (ms < 0) return "0s";
  const totalSeconds = Math.floor(ms / 1000);
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  if (minutes > 0) return `${minutes}m ${seconds}s`;
  return `${seconds}s`;
}

function deployIcon(status: string) {
  switch (status) {
    case "success":
      return <CheckCircle2 className="h-4 w-4 text-green-500" />;
    case "failed":
      return <XCircle className="h-4 w-4 text-destructive" />;
    case "cancelled":
      return <Square className="h-4 w-4 text-muted-foreground" />;
    case "queued":
    case "building":
    case "deploying":
      return <Loader2 className="h-4 w-4 animate-spin text-yellow-500" />;
    default:
      return <Clock className="h-4 w-4 text-muted-foreground" />;
  }
}

// ── Webhook section ───────────────────────────────────────────────

function WebhookSection({ app, appId }: { app: App; appId: string }) {
  const { data: webhook } = useWebhookConfig(appId);
  const enableWebhook = useEnableWebhook(appId);
  const disableWebhook = useDisableWebhook(appId);
  const regenerateWebhook = useRegenerateWebhook(appId);
  const [copiedField, setCopiedField] = useState<string | null>(null);
  const [showSecret, setShowSecret] = useState(false);

  if (app.source_type !== "git") return null;

  const isEnabled = app.auto_deploy;
  const isGitLab = app.git_repo?.toLowerCase().includes("gitlab");
  const provider = isGitLab ? "GitLab" : "GitHub";

  function copyToClipboard(text: string, label: string) {
    navigator.clipboard.writeText(text);
    setCopiedField(label);
    toast.success(`${label} copied!`);
    setTimeout(() => setCopiedField(null), 2000);
  }

  return (
    <Card>
      <CardHeader className="flex flex-row items-start justify-between gap-4">
        <div className="flex items-start gap-3">
          <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary/10">
            <GitBranch className="h-4 w-4 text-primary" />
          </div>
          <div>
            <CardTitle className="text-sm">Auto Deploy (Webhook)</CardTitle>
            <CardDescription className="text-xs">
              Automatically deploy when you push to{" "}
              <code className="rounded bg-muted px-1">{app.git_branch || "main"}</code>
            </CardDescription>
          </div>
        </div>
        <Button
          size="sm"
          variant={isEnabled ? "destructive" : "default"}
          disabled={enableWebhook.isPending || disableWebhook.isPending}
          onClick={() => (isEnabled ? disableWebhook.mutate() : enableWebhook.mutate())}
        >
          {enableWebhook.isPending || disableWebhook.isPending ? (
            <Loader2 className="h-3.5 w-3.5 animate-spin" />
          ) : isEnabled ? (
            "Disable"
          ) : (
            "Enable"
          )}
        </Button>
      </CardHeader>

      {isEnabled && webhook && (
        <CardContent className="space-y-4">
          {/* Webhook URL */}
          <div className="space-y-1.5">
            <Label className="text-xs">Webhook URL</Label>
            <div className="flex items-center gap-2">
              <Input readOnly value={webhook.webhook_url} className="h-8 font-mono text-xs" />
              <Button
                size="icon"
                variant="outline"
                className="h-8 w-8 shrink-0"
                onClick={() => copyToClipboard(webhook.webhook_url, "Webhook URL")}
                title="Copy webhook URL"
              >
                {copiedField === "Webhook URL" ? (
                  <Check className="h-3.5 w-3.5 text-green-500" />
                ) : (
                  <Copy className="h-3.5 w-3.5" />
                )}
              </Button>
            </div>
          </div>

          {/* Secret */}
          <div className="space-y-1.5">
            <Label className="text-xs">Secret</Label>
            <div className="flex items-center gap-2">
              <Input
                readOnly
                type={showSecret ? "text" : "password"}
                value={webhook.secret}
                className="h-8 font-mono text-xs"
              />
              <Button
                size="icon"
                variant="outline"
                className="h-8 w-8 shrink-0"
                onClick={() => setShowSecret(!showSecret)}
                title={showSecret ? "Hide secret" : "Show secret"}
              >
                {showSecret ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
              </Button>
              <Button
                size="icon"
                variant="outline"
                className="h-8 w-8 shrink-0"
                onClick={() => copyToClipboard(webhook.secret, "Secret")}
                title="Copy secret"
              >
                {copiedField === "Secret" ? (
                  <Check className="h-3.5 w-3.5 text-green-500" />
                ) : (
                  <Copy className="h-3.5 w-3.5" />
                )}
              </Button>
              <Button
                size="sm"
                variant="outline"
                className="h-8 shrink-0"
                onClick={() => regenerateWebhook.mutate()}
                disabled={regenerateWebhook.isPending}
              >
                {regenerateWebhook.isPending ? (
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                ) : (
                  <RefreshCw className="h-3.5 w-3.5" />
                )}
                Regenerate
              </Button>
            </div>
          </div>

          {/* Instructions */}
          <div className="rounded-lg border border-blue-500/20 bg-blue-500/5 px-3 py-2 text-xs text-blue-600 dark:text-blue-400">
            <p className="font-medium">
              Add this webhook URL to your {provider} repository settings.
            </p>
            <ul className="mt-1 list-inside list-disc space-y-0.5 text-blue-600/80 dark:text-blue-400/80">
              {isGitLab ? (
                <>
                  <li>Go to Settings &rarr; Webhooks in your GitLab repository</li>
                  <li>Paste the Webhook URL and Secret</li>
                  <li>Select &quot;Push events&quot; as the trigger</li>
                </>
              ) : (
                <>
                  <li>Go to Settings &rarr; Webhooks in your GitHub repository</li>
                  <li>Paste the Webhook URL and Secret</li>
                  <li>Set Content type to &quot;application/json&quot;</li>
                  <li>Select &quot;Just the push event&quot;</li>
                </>
              )}
            </ul>
          </div>
        </CardContent>
      )}
    </Card>
  );
}

// ── Deployment row ─────────────────────────────────────────────────

function DeploymentRow({ deployment, appId }: { deployment: Deployment; appId: string }) {
  const [expanded, setExpanded] = useState(false);
  const { data: detail, isLoading: detailLoading } = useDeploymentDetail(
    expanded ? deployment.id : "",
  );
  const redeploy = useDeploy(appId);
  const cancelDeploy = useCancelDeploy(appId);
  const isInProgress = ["queued", "building", "deploying"].includes(deployment.status);

  const hasDuration = deployment.started_at && deployment.finished_at;
  const imageName = deployment.image
    ? deployment.image.length > 40
      ? `${deployment.image.slice(0, 37)}...`
      : deployment.image
    : null;

  return (
    <Card>
      <CardContent className="p-0">
        <button
          type="button"
          className="flex w-full items-center gap-4 p-4 text-left hover:bg-muted/50"
          onClick={() => setExpanded(!expanded)}
        >
          {expanded ? (
            <ChevronDown className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
          ) : (
            <ChevronRight className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
          )}
          {deployIcon(deployment.status)}
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2">
              <span className="font-mono text-sm">{deployment.id.slice(0, 8)}</span>
              <Badge variant={statusVariant(deployment.status)} className="text-xs">
                {deployment.status}
              </Badge>
              {hasDuration && (
                <span className="text-xs text-muted-foreground">
                  {formatDuration(deployment.started_at!, deployment.finished_at!)}
                </span>
              )}
              {imageName && (
                <Badge variant="outline" className="max-w-[200px] truncate text-xs">
                  <Container className="mr-1 h-3 w-3" />
                  {imageName}
                </Badge>
              )}
            </div>
            <p className="text-xs text-muted-foreground">
              {deployment.trigger_type} &middot; {new Date(deployment.created_at).toLocaleString()}
            </p>
          </div>
          {isInProgress && (
            <Button
              size="sm"
              variant="ghost"
              className="text-destructive hover:text-destructive"
              onClick={(e) => {
                e.stopPropagation();
                cancelDeploy.mutate(deployment.id);
              }}
              disabled={cancelDeploy.isPending}
            >
              <Square className="h-3.5 w-3.5" /> Cancel
            </Button>
          )}
          {deployment.status === "success" && (
            <Button
              size="sm"
              variant="ghost"
              onClick={(e) => {
                e.stopPropagation();
                redeploy.mutate(undefined);
              }}
            >
              <RotateCcw className="h-3.5 w-3.5" />
            </Button>
          )}
        </button>

        {/* Expanded build log */}
        {expanded && (
          <div className="border-t bg-muted p-4">
            {detailLoading ? (
              <p className="text-xs text-muted-foreground">Loading build log...</p>
            ) : detail?.build_log ? (
              <pre className="max-h-[400px] overflow-auto whitespace-pre-wrap font-mono text-xs text-foreground">
                {detail.build_log}
              </pre>
            ) : (
              <p className="text-xs text-muted-foreground">No build log available.</p>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

// ── Main component ─────────────────────────────────────────────────

export function DeploymentsTab({
  app,
  appId,
  deployments,
  deployStrategy,
}: {
  app: App;
  appId: string;
  deployments: Deployment[];
  deployStrategy: string;
}) {
  return (
    <div className="space-y-4">
      {/* Webhook configuration (only for git-based apps) */}
      <WebhookSection app={app} appId={appId} />

      {/* Deployments list */}
      {deployments.length === 0 ? (
        <EmptyState icon={Rocket} message="No deployments yet." />
      ) : (
        <div className="space-y-3">
          <div className="flex items-center gap-2">
            <span className="text-sm font-medium">Deployments</span>
            {deployStrategy && (
              <Badge variant="outline" className="text-xs">
                {deployStrategy}
              </Badge>
            )}
          </div>

          <div className="space-y-2">
            {deployments.map((d) => (
              <DeploymentRow key={d.id} deployment={d} appId={appId} />
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
