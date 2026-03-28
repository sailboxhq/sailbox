import { AlertTriangle, Check, Clock, Globe, Lock, Pencil, ShieldCheck, X } from "lucide-react";
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

function DomainStatusBadges({ domain }: { domain: Domain }) {
  // Protocol badge
  const proto = domain.tls ? (
    <Badge variant="success" className="text-xs">
      <Lock className="mr-1 h-2.5 w-2.5" />
      HTTPS
    </Badge>
  ) : (
    <Badge variant="warning" className="text-xs">
      HTTP
    </Badge>
  );

  // Ingress status
  const ingress = domain.ingress_ready ? (
    <Badge variant="outline" className="text-xs text-green-500">
      <Check className="mr-1 h-2.5 w-2.5" />
      Active
    </Badge>
  ) : (
    <Badge variant="outline" className="text-xs text-yellow-500">
      <Clock className="mr-1 h-2.5 w-2.5" />
      Provisioning
    </Badge>
  );

  // Certificate status (only for TLS domains)
  let cert = null;
  if (domain.tls) {
    if (domain.cert_expiry) {
      const days = daysUntil(domain.cert_expiry);
      if (days < 7) {
        cert = (
          <Badge variant="destructive" className="text-xs">
            <AlertTriangle className="mr-1 h-2.5 w-2.5" />
            Cert expires in {days}d
          </Badge>
        );
      } else if (days < 30) {
        cert = (
          <Badge variant="warning" className="text-xs">
            <ShieldCheck className="mr-1 h-2.5 w-2.5" />
            Cert: {days}d left
          </Badge>
        );
      } else {
        cert = (
          <Badge variant="outline" className="text-xs text-green-500">
            <ShieldCheck className="mr-1 h-2.5 w-2.5" />
            Cert valid ({days}d)
          </Badge>
        );
      }
    } else if (
      domain.ingress_ready &&
      !domain.host.match(/\.(sslip\.io|nip\.io|traefik\.me|localhost)$/)
    ) {
      cert = (
        <Badge variant="outline" className="text-xs text-yellow-500">
          <Clock className="mr-1 h-2.5 w-2.5" />
          Issuing cert...
        </Badge>
      );
    }
  }

  return (
    <div className="flex items-center gap-1.5">
      {proto}
      {ingress}
      {cert}
    </div>
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
            </div>
            <DomainStatusBadges domain={domain} />
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
          {newTLS && (
            <p className="mt-2 text-xs text-muted-foreground">
              HTTPS uses Let's Encrypt via Traefik. If using Cloudflare proxy (orange cloud), set
              SSL mode to "Full (strict)" and disable proxy during initial cert issuance.
            </p>
          )}
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
