import { createFileRoute, Link } from "@tanstack/react-router";
import {
  Archive,
  Check,
  Clock,
  Copy,
  Database,
  Globe,
  Hash,
  Loader2,
  Mail,
  MessageSquare,
  Play,
  Save,
  Send,
  Server,
  Shield,
  Trash2,
  UserPlus,
  Users as UsersIcon,
  X,
} from "lucide-react";
import { useEffect, useState } from "react";
import { toast } from "sonner";
import { LoadingScreen } from "@/components/loading-screen";
import type { BadgeProps } from "@/components/ui/badge";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ToggleSwitch } from "@/components/ui/toggle-switch";
import { useCurrentUser } from "@/hooks/use-auth";
import {
  useNotificationChannels,
  useSaveChannel,
  useSaveSMTPConfig,
  useSMTPConfig,
  useTestChannel,
  useTestSMTP,
} from "@/hooks/use-notifications";
import { useResources } from "@/hooks/use-resources";
import { useSettings, useUpdateSetting } from "@/hooks/use-settings";
import {
  useSaveSystemBackupConfig,
  useSystemBackupConfig,
  useSystemBackups,
  useTriggerSystemBackup,
} from "@/hooks/use-system-backup";
import {
  useCancelInvitation,
  useInviteMember,
  useRemoveMember,
  useTeamInvitations,
  useTeamMembers,
  useUpdateMemberRole,
} from "@/hooks/use-team";
import type {
  Invitation,
  NotificationChannel,
  SharedResource,
  SMTPConfig,
  SystemBackup,
  TeamMember,
} from "@/types/api";

export const Route = createFileRoute("/_dashboard/settings")({
  component: SettingsPage,
});

function SettingsPage() {
  return (
    <div>
      <h1 className="text-2xl font-bold tracking-tight">Settings</h1>
      <p className="text-sm text-muted-foreground">Manage system configuration</p>

      <Tabs defaultValue="general" className="mt-6">
        <TabsList>
          <TabsTrigger value="general">General</TabsTrigger>
          <TabsTrigger value="backup">Backup</TabsTrigger>
          <TabsTrigger value="smtp">SMTP</TabsTrigger>
          <TabsTrigger value="notifications">Notifications</TabsTrigger>
          <TabsTrigger value="team">Team</TabsTrigger>
        </TabsList>

        <TabsContent value="general">
          <GeneralTab />
        </TabsContent>
        <TabsContent value="backup">
          <BackupTab />
        </TabsContent>
        <TabsContent value="smtp">
          <SMTPTab />
        </TabsContent>
        <TabsContent value="notifications">
          <NotificationsTab />
        </TabsContent>
        <TabsContent value="team">
          <TeamTab />
        </TabsContent>
      </Tabs>
    </div>
  );
}

// ── General Tab ─────────────────────────────────────────────────────

function GeneralTab() {
  const { data: settings, isLoading } = useSettings();
  const updateSetting = useUpdateSetting();
  const [baseDomain, setBaseDomain] = useState("");
  const [panelDomain, setPanelDomain] = useState("");
  const [httpsEmail, setHttpsEmail] = useState("");

  useEffect(() => {
    if (settings?.base_domain) setBaseDomain(settings.base_domain);
    if (settings?.panel_domain) setPanelDomain(settings.panel_domain);
    if (settings?.https_email) setHttpsEmail(settings.https_email);
  }, [settings]);

  if (isLoading) return <LoadingScreen />;
  if (!settings) return null;

  return (
    <div className="mt-4 space-y-6">
      {/* Server info */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-sm font-medium">
            <Server className="h-4 w-4" /> Server
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-3 text-sm">
          <div className="flex items-center justify-between">
            <span className="text-muted-foreground">Server IP</span>
            <Badge variant="outline" className="font-mono">
              {settings.server_ip || "detecting..."}
            </Badge>
          </div>
          <div className="flex items-center justify-between">
            <span className="text-muted-foreground">Setup Status</span>
            <Badge variant={settings.setup_done === "true" ? "success" : "warning"}>
              {settings.setup_done === "true" ? "Configured" : "Pending"}
            </Badge>
          </div>
        </CardContent>
      </Card>

      {/* Wildcard Domain */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-sm font-medium">
            <Globe className="h-4 w-4" /> Wildcard Domain
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-sm text-muted-foreground">
            All services auto-generate subdomains under this domain:{" "}
            <code className="rounded bg-muted px-1.5 py-0.5 text-xs">
              myapp-xxxx.{baseDomain || "example.com"}
            </code>
          </p>
          <div className="space-y-2">
            <Label>Base Domain</Label>
            <div className="flex items-center gap-3">
              <Input
                value={baseDomain}
                onChange={(e) => setBaseDomain(e.target.value)}
                placeholder="192.168.1.100.sslip.io"
                className="max-w-md font-mono"
              />
              <Button
                onClick={() =>
                  updateSetting.mutate({
                    key: "base_domain",
                    value: baseDomain,
                  })
                }
                disabled={updateSetting.isPending || baseDomain === settings.base_domain}
              >
                <Save className="h-3.5 w-3.5" /> {updateSetting.isPending ? "Saving..." : "Save"}
              </Button>
            </div>
          </div>
          <Separator />
          <div className="space-y-2 text-xs text-muted-foreground">
            <p className="font-medium text-foreground">How it works:</p>
            <ul className="list-inside list-disc space-y-1">
              <li>
                <strong>Development:</strong> Use{" "}
                <code className="rounded bg-muted px-1">{settings.server_ip}.sslip.io</code>
              </li>
              <li>
                <strong>Production:</strong> Set your domain with wildcard DNS{" "}
                <code className="rounded bg-muted px-1">*.mysite.com &rarr; server IP</code>
              </li>
            </ul>
          </div>
        </CardContent>
      </Card>

      {/* Panel Domain */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-sm font-medium">
            <Globe className="h-4 w-4" /> Panel Domain
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-sm text-muted-foreground">
            Access the Sailbox panel via a custom domain instead of IP:port. Point an A record to
            your server IP, then enter the domain below.
          </p>
          <div className="space-y-2">
            <Label>Domain</Label>
            <div className="flex items-center gap-3">
              <Input
                value={panelDomain}
                onChange={(e) => setPanelDomain(e.target.value)}
                placeholder="panel.example.com"
                className="max-w-md font-mono"
              />
              <Button
                onClick={() =>
                  updateSetting.mutate({
                    key: "panel_domain",
                    value: panelDomain,
                  })
                }
                disabled={updateSetting.isPending || panelDomain === (settings?.panel_domain ?? "")}
              >
                <Save className="h-3.5 w-3.5" /> {updateSetting.isPending ? "Saving..." : "Save"}
              </Button>
            </div>
          </div>
          {panelDomain && (
            <p className="text-xs text-muted-foreground">
              After saving, your panel will be accessible at{" "}
              <code className="rounded bg-muted px-1.5 py-0.5">https://{panelDomain}</code>
            </p>
          )}
        </CardContent>
      </Card>

      {/* TLS / HTTPS */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-sm font-medium">
            <Shield className="h-4 w-4" /> TLS / HTTPS
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-sm text-muted-foreground">
            TLS certificates are auto-managed by Traefik via Let's Encrypt. Provide an email for
            certificate registration and renewal notifications.
          </p>
          <div className="space-y-2">
            <Label>ACME Email</Label>
            <div className="flex items-center gap-3">
              <Input
                value={httpsEmail}
                onChange={(e) => setHttpsEmail(e.target.value)}
                placeholder="admin@example.com"
                className="max-w-md"
              />
              <Button
                onClick={() =>
                  updateSetting.mutate({
                    key: "https_email",
                    value: httpsEmail,
                  })
                }
                disabled={updateSetting.isPending || httpsEmail === (settings?.https_email ?? "")}
              >
                <Save className="h-3.5 w-3.5" /> {updateSetting.isPending ? "Saving..." : "Save"}
              </Button>
            </div>
          </div>
          <p className="text-xs text-muted-foreground">
            Required for Let's Encrypt. Certificates are issued automatically for custom domains
            (panel and app domains).
          </p>
        </CardContent>
      </Card>
    </div>
  );
}

// ── Backup Tab ──────────────────────────────────────────────────────

const BACKUP_SCHEDULE_PRESETS = [
  { value: "0 */6 * * *", label: "Every 6 hours" },
  { value: "0 2 * * *", label: "Daily at 2:00 AM" },
  { value: "0 3 * * *", label: "Daily at 3:00 AM (recommended)" },
  { value: "0 2 * * 0", label: "Weekly (Sunday 2 AM)" },
  { value: "custom", label: "Custom" },
] as const;

function backupStatusVariant(status: string): "warning" | "success" | "destructive" | "secondary" {
  switch (status) {
    case "pending":
    case "running":
      return "warning";
    case "completed":
      return "success";
    case "failed":
      return "destructive";
    default:
      return "secondary";
  }
}

function formatBytes(bytes: number): string {
  if (!bytes || bytes === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / 1024 ** i).toFixed(1)} ${units[i]}`;
}

function relativeTime(dateStr: string): string {
  const now = Date.now();
  const then = new Date(dateStr).getTime();
  const diffMs = now - then;
  if (diffMs < 0) return "just now";
  const mins = Math.floor(diffMs / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

function BackupTab() {
  const { data: config, isLoading: configLoading } = useSystemBackupConfig();
  const saveConfig = useSaveSystemBackupConfig();
  const { data: rawBackups, isLoading: backupsLoading } = useSystemBackups();
  const backups = rawBackups ?? [];
  const triggerBackup = useTriggerSystemBackup();
  const { data: resources } = useResources("object_storage");
  const s3Resources = (resources ?? []).filter((r: SharedResource) => r.type === "object_storage");

  // ── Form state ──
  const [enabled, setEnabled] = useState(false);
  const [s3Id, setS3Id] = useState("");
  const [path, setPath] = useState("sailbox-backups");
  const [retention, setRetention] = useState(30);
  const [schedulePreset, setSchedulePreset] = useState("0 3 * * *");
  const [customCron, setCustomCron] = useState("");
  const [showAllBackups, setShowAllBackups] = useState(false);

  // Sync from server
  useEffect(() => {
    if (config) {
      setEnabled(config.enabled);
      setS3Id(config.s3_id || "");
      setPath(config.path || "sailbox-backups");
      setRetention(config.retention || 30);
      const match = BACKUP_SCHEDULE_PRESETS.find((p) => p.value === config.schedule);
      setSchedulePreset(match ? match.value : config.schedule ? "custom" : "0 3 * * *");
      if (!match && config.schedule) {
        setCustomCron(config.schedule);
      }
    }
  }, [config]);

  const resolvedSchedule = schedulePreset === "custom" ? customCron : schedulePreset;

  // Dirty detection
  const isDirty =
    !config ||
    enabled !== config.enabled ||
    (s3Id || "") !== (config.s3_id || "") ||
    (path || "sailbox-backups") !== (config.path || "sailbox-backups") ||
    retention !== (config.retention || 30) ||
    resolvedSchedule !== (config.schedule || "0 3 * * *");

  function handleSave() {
    saveConfig.mutate({
      enabled,
      s3_id: s3Id,
      schedule: resolvedSchedule,
      path: path || "sailbox-backups",
      retention,
    });
  }

  if (configLoading) return <LoadingScreen />;

  return (
    <div className="mt-4 space-y-6">
      {/* Backup Configuration */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-sm font-medium">
            <Archive className="h-4 w-4" /> Backup Configuration
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center gap-3">
            <ToggleSwitch checked={enabled} onChange={setEnabled} />
            <span className="text-sm font-medium">Automatic Backups</span>
          </div>

          {enabled && (
            <div className="space-y-4">
              {/* S3 Destination */}
              <div className="space-y-2">
                <Label className="text-sm">S3 Destination</Label>
                {s3Resources.length === 0 ? (
                  <p className="text-sm text-muted-foreground">
                    No S3 storage configured.{" "}
                    <Link
                      to="/resources"
                      className="text-primary underline underline-offset-4 hover:text-primary/80"
                    >
                      Add one in Resources.
                    </Link>
                  </p>
                ) : (
                  <Select value={s3Id} onValueChange={setS3Id}>
                    <SelectTrigger>
                      <SelectValue placeholder="Select S3 resource" />
                    </SelectTrigger>
                    <SelectContent>
                      {s3Resources.map((r: SharedResource) => (
                        <SelectItem key={r.id} value={r.id}>
                          {r.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              </div>

              {/* Directory */}
              <div className="space-y-2">
                <Label className="text-sm">Directory</Label>
                <Input
                  value={path}
                  onChange={(e) => setPath(e.target.value)}
                  placeholder="sailbox-backups"
                  className="max-w-md font-mono"
                />
              </div>

              {/* Schedule */}
              <div className="space-y-2">
                <Label className="text-sm">Schedule</Label>
                <Select value={schedulePreset} onValueChange={setSchedulePreset}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {BACKUP_SCHEDULE_PRESETS.map((p) => (
                      <SelectItem key={p.value} value={p.value}>
                        {p.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                {schedulePreset === "custom" && (
                  <Input
                    value={customCron}
                    onChange={(e) => setCustomCron(e.target.value)}
                    placeholder="0 */6 * * *"
                    className="font-mono"
                  />
                )}
              </div>

              {/* Retention */}
              <div className="space-y-2">
                <Label className="text-sm">Retention</Label>
                <div className="flex items-center gap-2">
                  <Input
                    type="number"
                    min={1}
                    value={retention}
                    onChange={(e) => setRetention(Number(e.target.value) || 1)}
                    className="w-24"
                  />
                  <span className="text-sm text-muted-foreground">backups</span>
                </div>
              </div>

              {/* Save */}
              <Button
                size="sm"
                onClick={handleSave}
                disabled={saveConfig.isPending || (!resolvedSchedule && enabled) || !isDirty}
              >
                {saveConfig.isPending ? (
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                ) : (
                  <Save className="h-3.5 w-3.5" />
                )}{" "}
                Save
              </Button>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Backup History */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle className="flex items-center gap-2 text-sm font-medium">
            <Database className="h-4 w-4" /> Backup History
          </CardTitle>
          <Button
            size="sm"
            onClick={() => triggerBackup.mutate()}
            disabled={triggerBackup.isPending || !config?.s3_id}
            title={!config?.s3_id ? "Configure S3 storage first" : "Run backup now"}
          >
            {triggerBackup.isPending ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <Play className="h-3.5 w-3.5" />
            )}{" "}
            Backup Now
          </Button>
        </CardHeader>
        <CardContent>
          {backupsLoading ? (
            <LoadingScreen />
          ) : backups.length === 0 ? (
            <div className="flex flex-col items-center gap-3 py-8 text-center">
              <div className="flex h-10 w-10 items-center justify-center rounded-full bg-muted">
                <Archive className="h-5 w-5 text-muted-foreground" />
              </div>
              <p className="text-sm text-muted-foreground">No backups yet</p>
            </div>
          ) : (
            <div className="space-y-2">
              {(showAllBackups ? backups : backups.slice(0, 10)).map((backup: SystemBackup) => (
                <div
                  key={backup.id}
                  className="flex items-center justify-between rounded-md border px-3 py-2 text-sm"
                >
                  <div className="flex items-center gap-3">
                    <Badge variant={backupStatusVariant(backup.status)} className="text-xs">
                      {backup.status}
                    </Badge>
                    <span className="font-mono text-xs">{backup.file_name}</span>
                  </div>
                  <div className="flex items-center gap-4 text-xs text-muted-foreground">
                    <span>{formatBytes(backup.size_bytes)}</span>
                    <span>{relativeTime(backup.created_at)}</span>
                  </div>
                  {backup.status === "failed" && backup.error && (
                    <p className="mt-1 text-xs text-destructive">{backup.error}</p>
                  )}
                </div>
              ))}
              {backups.length > 10 && (
                <button
                  type="button"
                  onClick={() => setShowAllBackups(!showAllBackups)}
                  className="w-full py-2 text-center text-xs text-primary hover:underline"
                >
                  {showAllBackups ? "Show less" : `Show all ${backups.length} backups`}
                </button>
              )}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

// ── SMTP Tab ────────────────────────────────────────────────────────

function SMTPTab() {
  const { data: smtp, isLoading } = useSMTPConfig();
  const saveSMTP = useSaveSMTPConfig();
  const testSMTP = useTestSMTP();

  const [form, setForm] = useState<SMTPConfig>({
    host: "",
    port: "587",
    user: "",
    password: "",
    from: "",
    enabled: false,
  });

  useEffect(() => {
    if (smtp) setForm(smtp);
  }, [smtp]);

  const update = (key: keyof SMTPConfig, value: string | boolean) =>
    setForm((prev) => ({ ...prev, [key]: value }));

  if (isLoading) return <LoadingScreen />;

  return (
    <div className="mt-4 space-y-6">
      <Card>
        <CardHeader className="flex flex-row items-center justify-between pb-4">
          <CardTitle className="flex items-center gap-2 text-sm font-medium">
            <Mail className="h-4 w-4" /> SMTP Configuration
          </CardTitle>
          <ToggleSwitch checked={form.enabled} onChange={(checked) => update("enabled", checked)} />
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <Label>Host</Label>
              <Input
                value={form.host}
                onChange={(e) => update("host", e.target.value)}
                placeholder="smtp.gmail.com"
              />
            </div>
            <div className="space-y-2">
              <Label>Port</Label>
              <Input
                value={form.port}
                onChange={(e) => update("port", e.target.value)}
                placeholder="587"
              />
            </div>
            <div className="space-y-2">
              <Label>Username</Label>
              <Input
                value={form.user}
                onChange={(e) => update("user", e.target.value)}
                placeholder="user@gmail.com"
              />
            </div>
            <div className="space-y-2">
              <Label>Password</Label>
              <Input
                type="password"
                value={form.password}
                onChange={(e) => update("password", e.target.value)}
                placeholder="••••••••"
              />
            </div>
          </div>
          <div className="space-y-2">
            <Label>From Address</Label>
            <Input
              value={form.from}
              onChange={(e) => update("from", e.target.value)}
              placeholder="noreply@sailbox.dev"
              className="max-w-md"
            />
          </div>
          <div className="flex gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => testSMTP.mutate()}
              disabled={testSMTP.isPending || !form.enabled}
            >
              {testSMTP.isPending ? "Sending..." : "Test"}
            </Button>
            <Button size="sm" onClick={() => saveSMTP.mutate(form)} disabled={saveSMTP.isPending}>
              <Save className="mr-1 h-3.5 w-3.5" />
              {saveSMTP.isPending ? "Saving..." : "Save"}
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

// ── Notifications Tab ───────────────────────────────────────────────

const CHANNEL_DEFS = [
  {
    type: "email" as const,
    label: "Email",
    icon: Mail,
    note: "Requires SMTP configured in the SMTP tab",
    fields: [
      {
        key: "recipients",
        label: "Recipients",
        placeholder: "user@example.com, ops@example.com",
        multiline: true,
      },
    ],
  },
  {
    type: "telegram" as const,
    label: "Telegram",
    icon: Send,
    fields: [
      { key: "bot_token", label: "Bot Token", placeholder: "123456:ABC-DEF..." },
      { key: "chat_id", label: "Chat ID", placeholder: "-1001234567890" },
    ],
  },
  {
    type: "discord" as const,
    label: "Discord",
    icon: Hash,
    fields: [
      {
        key: "webhook_url",
        label: "Webhook URL",
        placeholder: "https://discord.com/api/webhooks/...",
      },
    ],
  },
  {
    type: "slack" as const,
    label: "Slack",
    icon: MessageSquare,
    fields: [
      {
        key: "webhook_url",
        label: "Webhook URL",
        placeholder: "https://hooks.slack.com/services/...",
      },
    ],
  },
];

function NotificationsTab() {
  const { data: channels, isLoading } = useNotificationChannels();

  if (isLoading) return <LoadingScreen />;

  return (
    <div className="mt-4 grid gap-4 sm:grid-cols-2">
      {CHANNEL_DEFS.map((def) => {
        const existing = channels?.find((c) => c.type === def.type);
        return <ChannelCard key={def.type} def={def} existing={existing} />;
      })}
    </div>
  );
}

function ChannelCard({
  def,
  existing,
}: {
  def: (typeof CHANNEL_DEFS)[number];
  existing?: NotificationChannel;
}) {
  const saveChannel = useSaveChannel();
  const testChannel = useTestChannel();
  const [enabled, setEnabled] = useState(existing?.enabled ?? false);
  const [config, setConfig] = useState<Record<string, string>>(existing?.config ?? {});

  useEffect(() => {
    if (existing) {
      setEnabled(existing.enabled);
      setConfig(existing.config);
    }
  }, [existing]);

  const Icon = def.icon;

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between pb-4">
        <CardTitle className="flex items-center gap-2 text-sm font-medium">
          <Icon className="h-4 w-4" /> {def.label}
        </CardTitle>
        <ToggleSwitch checked={enabled} onChange={setEnabled} />
      </CardHeader>
      {enabled && (
        <CardContent className="space-y-4">
          {"note" in def && def.note && <p className="text-xs text-muted-foreground">{def.note}</p>}
          {def.fields.map((field) => (
            <div key={field.key} className="space-y-2">
              <Label>{field.label}</Label>
              {"multiline" in field && field.multiline ? (
                <textarea
                  className="flex min-h-[60px] w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                  value={config[field.key] ?? ""}
                  onChange={(e) => setConfig((prev) => ({ ...prev, [field.key]: e.target.value }))}
                  placeholder={field.placeholder}
                />
              ) : (
                <Input
                  value={config[field.key] ?? ""}
                  onChange={(e) => setConfig((prev) => ({ ...prev, [field.key]: e.target.value }))}
                  placeholder={field.placeholder}
                />
              )}
            </div>
          ))}
          <div className="flex gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => testChannel.mutate(def.type)}
              disabled={testChannel.isPending}
            >
              {testChannel.isPending ? "Sending..." : "Test"}
            </Button>
            <Button
              size="sm"
              onClick={() => saveChannel.mutate({ type: def.type, enabled, config })}
              disabled={saveChannel.isPending}
            >
              {saveChannel.isPending ? "Saving..." : "Save"}
            </Button>
          </div>
        </CardContent>
      )}
    </Card>
  );
}

// ── Team Tab ────────────────────────────────────────────────────────

const AVATAR_EMOJI: Record<string, string> = {
  bear: "\u{1F43B}",
  cat: "\u{1F431}",
  dog: "\u{1F436}",
  fox: "\u{1F98A}",
  koala: "\u{1F428}",
  lion: "\u{1F981}",
  monkey: "\u{1F435}",
  owl: "\u{1F989}",
  panda: "\u{1F43C}",
  penguin: "\u{1F427}",
  rabbit: "\u{1F430}",
  tiger: "\u{1F42F}",
  whale: "\u{1F433}",
  wolf: "\u{1F43A}",
};

function roleVariant(role: string): NonNullable<BadgeProps["variant"]> {
  switch (role.toLowerCase()) {
    case "owner":
      return "default";
    case "admin":
      return "outline";
    default:
      return "secondary";
  }
}

function relativeExpiry(expiresAt: string): string {
  const now = Date.now();
  const expires = new Date(expiresAt).getTime();
  const diffMs = expires - now;
  if (diffMs <= 0) return "Expired";
  const days = Math.floor(diffMs / (1000 * 60 * 60 * 24));
  const hours = Math.floor((diffMs % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60));
  if (days > 0) return `Expires in ${days}d`;
  if (hours > 0) return `Expires in ${hours}h`;
  return "Expires soon";
}

function TeamTab() {
  const { data: user, isLoading: userLoading } = useCurrentUser();
  const { data: members, isLoading: membersLoading } = useTeamMembers();
  const { data: invitations, isLoading: invitationsLoading } = useTeamInvitations();

  if (userLoading || membersLoading) return <LoadingScreen />;
  if (!user) return null;

  const isOwner = user.role?.toLowerCase() === "owner";
  const pendingInvitations = (invitations ?? []).filter((inv) => !inv.accepted_at);

  return (
    <div className="mt-4 space-y-6">
      {/* Members */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle className="flex items-center gap-2 text-sm font-medium">
            <UsersIcon className="h-4 w-4" /> Members
          </CardTitle>
          {isOwner && <InviteDialog />}
        </CardHeader>
        <CardContent>
          {!members || members.length === 0 ? (
            <div className="flex flex-col items-center gap-3 py-8 text-center">
              <div className="flex h-10 w-10 items-center justify-center rounded-full bg-muted">
                <UsersIcon className="h-5 w-5 text-muted-foreground" />
              </div>
              <p className="text-sm text-muted-foreground">No team members found</p>
            </div>
          ) : (
            <div className="space-y-1">
              {members.map((member) => (
                <MemberRow
                  key={member.id}
                  member={member}
                  isOwner={isOwner}
                  isCurrentUser={member.id === user.id}
                />
              ))}
            </div>
          )}
          {!isOwner && (
            <p className="mt-4 text-xs text-muted-foreground">
              Contact the owner to manage team members.
            </p>
          )}
        </CardContent>
      </Card>

      {/* Pending Invitations */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-sm font-medium">
            <Mail className="h-4 w-4" /> Pending Invitations
          </CardTitle>
        </CardHeader>
        <CardContent>
          {invitationsLoading ? (
            <LoadingScreen />
          ) : pendingInvitations.length === 0 ? (
            <div className="flex flex-col items-center gap-3 py-8 text-center">
              <div className="flex h-10 w-10 items-center justify-center rounded-full bg-muted">
                <Mail className="h-5 w-5 text-muted-foreground" />
              </div>
              <p className="text-sm text-muted-foreground">No pending invitations</p>
            </div>
          ) : (
            <div className="space-y-1">
              {pendingInvitations.map((inv) => (
                <InvitationRow key={inv.id} invitation={inv} isOwner={isOwner} />
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

// ── Member Row ─────────────────────────────────────────────────────

function MemberRow({
  member,
  isOwner,
  isCurrentUser,
}: {
  member: TeamMember;
  isOwner: boolean;
  isCurrentUser: boolean;
}) {
  const updateRole = useUpdateMemberRole();
  const removeMember = useRemoveMember();
  const [removeOpen, setRemoveOpen] = useState(false);
  const [removeConfirm, setRemoveConfirm] = useState("");

  const isMemberOwner = member.role.toLowerCase() === "owner";
  const showActions = isOwner && !isMemberOwner && !isCurrentUser;

  const handleRoleChange = (newRole: string) => {
    updateRole.mutate({ id: member.id, role: newRole });
  };

  const handleRemove = () => {
    if (removeConfirm !== "REMOVE") return;
    removeMember.mutate(member.id, {
      onSuccess: () => {
        setRemoveOpen(false);
        setRemoveConfirm("");
      },
    });
  };

  return (
    <div className="flex items-center gap-3 rounded-lg px-3 py-3 hover:bg-accent/50">
      {/* Avatar */}
      <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-primary/10 text-sm font-semibold text-primary">
        {member.avatar_url && AVATAR_EMOJI[member.avatar_url] ? (
          <span className="text-lg leading-none">{AVATAR_EMOJI[member.avatar_url]}</span>
        ) : (
          <span>{member.display_name?.[0]?.toUpperCase() || member.email[0]?.toUpperCase()}</span>
        )}
      </div>

      {/* Name + email */}
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium leading-tight">
          {member.display_name || `${member.first_name} ${member.last_name}`.trim() || member.email}
        </p>
        <p className="truncate text-xs text-muted-foreground">{member.email}</p>
      </div>

      {/* Role badge */}
      <Badge variant={roleVariant(member.role)} className="shrink-0 capitalize">
        {member.role}
      </Badge>

      {/* Actions */}
      {showActions && (
        <div className="flex shrink-0 items-center gap-1">
          <Select value={member.role.toLowerCase()} onValueChange={handleRoleChange}>
            <SelectTrigger className="h-7 w-[100px] text-xs">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="admin">Admin</SelectItem>
              <SelectItem value="member">Member</SelectItem>
            </SelectContent>
          </Select>

          {/* Remove dialog */}
          <Dialog
            open={removeOpen}
            onOpenChange={(open) => {
              setRemoveOpen(open);
              if (!open) setRemoveConfirm("");
            }}
          >
            <DialogTrigger asChild>
              <Button
                variant="ghost"
                size="sm"
                className="h-7 w-7 p-0 text-muted-foreground hover:text-destructive"
              >
                <Trash2 className="h-3.5 w-3.5" />
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Remove Member</DialogTitle>
                <DialogDescription>
                  Are you sure you want to remove{" "}
                  <strong>{member.display_name || member.email}</strong> from the team? Type{" "}
                  <strong>REMOVE</strong> to confirm.
                </DialogDescription>
              </DialogHeader>
              <div className="space-y-2">
                <Label>Confirmation</Label>
                <Input
                  value={removeConfirm}
                  onChange={(e) => setRemoveConfirm(e.target.value)}
                  placeholder="Type REMOVE"
                  className="font-mono"
                />
              </div>
              <DialogFooter>
                <DialogClose asChild>
                  <Button variant="outline">Cancel</Button>
                </DialogClose>
                <Button
                  variant="destructive"
                  onClick={handleRemove}
                  disabled={removeConfirm !== "REMOVE" || removeMember.isPending}
                >
                  {removeMember.isPending ? "Removing..." : "Remove"}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </div>
      )}
    </div>
  );
}

// ── Invitation Row ─────────────────────────────────────────────────

function InvitationRow({ invitation, isOwner }: { invitation: Invitation; isOwner: boolean }) {
  const cancelInvitation = useCancelInvitation();

  return (
    <div className="flex items-center gap-3 rounded-lg px-3 py-3 hover:bg-accent/50">
      <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-muted">
        <Mail className="h-4 w-4 text-muted-foreground" />
      </div>

      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium leading-tight">{invitation.email}</p>
      </div>

      <Badge variant={roleVariant(invitation.role)} className="shrink-0 capitalize">
        {invitation.role}
      </Badge>

      <span className="flex shrink-0 items-center gap-1 text-xs text-muted-foreground">
        <Clock className="h-3 w-3" />
        {relativeExpiry(invitation.expires_at)}
      </span>

      {isOwner && (
        <Button
          variant="ghost"
          size="sm"
          className="h-7 shrink-0 text-xs text-muted-foreground hover:text-destructive"
          onClick={() => cancelInvitation.mutate(invitation.id)}
          disabled={cancelInvitation.isPending}
        >
          <X className="mr-1 h-3 w-3" />
          Cancel
        </Button>
      )}
    </div>
  );
}

// ── Invite Dialog ──────────────────────────────────────────────────

function InviteDialog() {
  const [open, setOpen] = useState(false);
  const [email, setEmail] = useState("");
  const [role, setRole] = useState("member");
  const [inviteUrl, setInviteUrl] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const inviteMember = useInviteMember();

  const handleInvite = () => {
    inviteMember.mutate(
      { email, role },
      {
        onSuccess: (data) => {
          setInviteUrl(data.invite_url);
        },
      },
    );
  };

  const handleCopy = () => {
    if (!inviteUrl) return;
    navigator.clipboard.writeText(inviteUrl);
    setCopied(true);
    toast.success("Invite URL copied");
    setTimeout(() => setCopied(false), 2000);
  };

  const handleClose = (isOpen: boolean) => {
    setOpen(isOpen);
    if (!isOpen) {
      setEmail("");
      setRole("member");
      setInviteUrl(null);
      setCopied(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogTrigger asChild>
        <Button size="sm">
          <UserPlus className="mr-1 h-3.5 w-3.5" />
          Invite Member
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Invite Member</DialogTitle>
          <DialogDescription>Send an invitation to join your team.</DialogDescription>
        </DialogHeader>

        {!inviteUrl ? (
          <>
            <div className="space-y-4">
              <div className="space-y-2">
                <Label>Email</Label>
                <Input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="teammate@example.com"
                />
              </div>
              <div className="space-y-2">
                <Label>Role</Label>
                <Select value={role} onValueChange={setRole}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="admin">Admin</SelectItem>
                    <SelectItem value="member">Member</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
            <DialogFooter>
              <DialogClose asChild>
                <Button variant="outline">Cancel</Button>
              </DialogClose>
              <Button onClick={handleInvite} disabled={!email || inviteMember.isPending}>
                {inviteMember.isPending ? "Sending..." : "Send Invitation"}
              </Button>
            </DialogFooter>
          </>
        ) : (
          <>
            <div className="space-y-2">
              <Label>Invite URL</Label>
              <p className="text-xs text-muted-foreground">
                Share this link with the invitee. It will expire in 7 days.
              </p>
              <div className="flex items-center gap-2">
                <Input value={inviteUrl} readOnly className="font-mono text-xs" />
                <Button variant="outline" size="sm" onClick={handleCopy}>
                  {copied ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
                </Button>
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => handleClose(false)}>
                Done
              </Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
