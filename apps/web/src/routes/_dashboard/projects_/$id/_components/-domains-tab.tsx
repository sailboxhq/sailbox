import { Check, Globe, Lock, Pencil, ShieldCheck, X } from "lucide-react";
import { useState } from "react";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ToggleSwitch } from "@/components/ui/toggle-switch";
import {
  useAddDomain,
  useDeleteDomain,
  useGenerateDomain,
  useUpdateDomain,
} from "@/hooks/use-apps";
import type { Domain } from "@/types/api";

// ── Helpers ────────────────────────────────────────────────────────

function daysUntil(dateStr: string): number {
  const diff = new Date(dateStr).getTime() - Date.now();
  return Math.floor(diff / (1000 * 60 * 60 * 24));
}

function CertExpiry({ certExpiry }: { certExpiry?: string }) {
  if (!certExpiry) return null;
  const days = daysUntil(certExpiry);
  let color = "text-muted-foreground";
  if (days < 7) color = "text-red-500";
  else if (days < 30) color = "text-amber-500";

  return (
    <span className={`flex items-center gap-1 text-xs ${color}`}>
      <ShieldCheck className="h-3 w-3" />
      Cert expires {new Date(certExpiry).toLocaleDateString()}
      {days < 30 && ` (${days}d)`}
    </span>
  );
}

// ── Domain row ─────────────────────────────────────────────────────

function DomainRow({
  domain,
  appId,
  onRequestDelete,
}: {
  domain: Domain;
  appId: string;
  onRequestDelete: (id: string) => void;
}) {
  const updateDomain = useUpdateDomain(appId);
  const [editing, setEditing] = useState(false);
  // Split host into prefix and base domain suffix (e.g., "test-fb68" + "192.168.2.3.sslip.io")
  const dotIdx = domain.host.indexOf(".");
  const hasBaseDomain = dotIdx > 0;
  const prefix = hasBaseDomain ? domain.host.slice(0, dotIdx) : domain.host;
  const baseDomain = hasBaseDomain ? domain.host.slice(dotIdx + 1) : "";
  const [editPrefix, setEditPrefix] = useState(prefix);

  function toggleForceHttps() {
    updateDomain.mutate({ id: domain.id, force_https: !domain.force_https });
  }

  function handleRename() {
    const trimmed = editPrefix
      .trim()
      .toLowerCase()
      .replace(/[^a-z0-9-]/g, "");
    if (!trimmed || trimmed === prefix) {
      setEditing(false);
      return;
    }
    const newHost = hasBaseDomain ? `${trimmed}.${baseDomain}` : trimmed;
    updateDomain.mutate({ id: domain.id, host: newHost }, { onSuccess: () => setEditing(false) });
  }

  return (
    <Card>
      <CardContent className="flex items-center justify-between p-4">
        <div className="flex items-center gap-3">
          <Globe className="h-4 w-4 text-primary" />
          <div className="space-y-0.5">
            <div className="flex items-center gap-2">
              {editing ? (
                <form
                  onSubmit={(e) => {
                    e.preventDefault();
                    handleRename();
                  }}
                  className="flex items-center gap-1"
                >
                  <div className="flex items-center">
                    <Input
                      value={editPrefix}
                      onChange={(e) => setEditPrefix(e.target.value)}
                      className="h-7 w-40 rounded-r-none text-sm"
                      autoFocus
                      onKeyDown={(e) => {
                        if (e.key === "Escape") {
                          setEditing(false);
                          setEditPrefix(prefix);
                        }
                      }}
                    />
                    {hasBaseDomain && (
                      <span className="flex h-7 items-center rounded-r-md border border-l-0 bg-muted px-2 text-xs text-muted-foreground">
                        .{baseDomain}
                      </span>
                    )}
                  </div>
                  <Button
                    type="submit"
                    size="icon"
                    variant="ghost"
                    className="h-7 w-7"
                    disabled={updateDomain.isPending}
                  >
                    <Check className="h-3 w-3" />
                  </Button>
                  <Button
                    type="button"
                    size="icon"
                    variant="ghost"
                    className="h-7 w-7"
                    onClick={() => {
                      setEditing(false);
                      setEditPrefix(prefix);
                    }}
                  >
                    <X className="h-3 w-3" />
                  </Button>
                </form>
              ) : (
                <>
                  <a
                    href={`http${domain.tls ? "s" : ""}://${domain.host}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="font-medium hover:underline"
                  >
                    {domain.host}
                  </a>
                  <Button
                    size="icon"
                    variant="ghost"
                    className="h-6 w-6 text-muted-foreground"
                    onClick={() => setEditing(true)}
                    title="Rename domain"
                  >
                    <Pencil className="h-3 w-3" />
                  </Button>
                </>
              )}
              {/* Ingress ready dot */}
              {!editing && (
                <span
                  className={`inline-block h-2 w-2 rounded-full ${domain.ingress_ready ? "bg-green-500" : "bg-yellow-500"}`}
                  title={domain.ingress_ready ? "Ingress ready" : "Ingress pending"}
                />
              )}
            </div>
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <Badge variant={domain.tls ? "success" : "warning"} className="text-xs">
                {domain.tls ? "HTTPS" : "HTTP"}
              </Badge>
              <CertExpiry certExpiry={domain.cert_expiry} />
            </div>
          </div>
        </div>

        <div className="flex items-center gap-2">
          {/* Force HTTPS toggle */}
          {domain.tls && (
            <ToggleSwitch
              checked={domain.force_https}
              onChange={() => toggleForceHttps()}
              title={domain.force_https ? "Force HTTPS enabled" : "Force HTTPS disabled"}
            />
          )}
          {domain.tls && <span className="text-xs text-muted-foreground">Force HTTPS</span>}

          <Button
            size="icon"
            variant="ghost"
            className="h-8 w-8 text-destructive"
            aria-label="Delete domain"
            onClick={() => onRequestDelete(domain.id)}
          >
            <X className="h-3.5 w-3.5" />
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

// ── Main component ─────────────────────────────────────────────────

export function DomainsTab({ appId, domains }: { appId: string; domains: Domain[] }) {
  const addDomain = useAddDomain(appId);
  const deleteDomain = useDeleteDomain(appId);
  const generateDomain = useGenerateDomain(appId);
  const [newHost, setNewHost] = useState("");
  const [newTLS, setNewTLS] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null);

  const targetDomain = domains.find((d) => d.id === deleteTarget);

  function handleAdd(e: React.FormEvent) {
    e.preventDefault();
    addDomain.mutate(
      { host: newHost, tls: newTLS, auto_cert: newTLS },
      { onSuccess: () => setNewHost("") },
    );
  }

  return (
    <>
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle className="text-sm">Custom Domains</CardTitle>
          <Button
            size="sm"
            variant="outline"
            onClick={() => generateDomain.mutate()}
            disabled={generateDomain.isPending}
          >
            Generate Domain
          </Button>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleAdd} className="flex items-end gap-3">
            <div className="flex-1 space-y-1">
              <Label className="text-xs">Hostname</Label>
              <Input
                value={newHost}
                onChange={(e) => setNewHost(e.target.value)}
                placeholder="app.example.com"
                required
              />
            </div>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={newTLS}
                onChange={(e) => setNewTLS(e.target.checked)}
                className="rounded"
              />
              <Lock className="h-3.5 w-3.5" /> HTTPS
            </label>
            <Button type="submit" size="sm" disabled={addDomain.isPending}>
              {addDomain.isPending ? "..." : "Add"}
            </Button>
          </form>
        </CardContent>
      </Card>

      {domains.length === 0 ? (
        <p className="py-6 text-center text-sm text-muted-foreground">
          No custom domains configured yet.
        </p>
      ) : (
        <div className="space-y-2">
          {domains.map((d) => (
            <DomainRow key={d.id} domain={d} appId={appId} onRequestDelete={setDeleteTarget} />
          ))}
        </div>
      )}

      <ConfirmDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null);
        }}
        title="Delete domain"
        description={
          targetDomain
            ? `Are you sure you want to delete "${targetDomain.host}"? This action cannot be undone.`
            : ""
        }
        confirmLabel="Delete"
        variant="destructive"
        loading={deleteDomain.isPending}
        onConfirm={() => {
          if (targetDomain) {
            deleteDomain.mutate(
              { id: targetDomain.id, host: targetDomain.host },
              { onSuccess: () => setDeleteTarget(null) },
            );
          }
        }}
      />
    </>
  );
}
