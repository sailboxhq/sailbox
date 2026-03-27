import { Eye, EyeOff, Plus, Save, X } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { useAppSecrets, useUpdateApp, useUpdateEnv, useUpdateSecrets } from "@/hooks/use-apps";

// ── Env helpers ────────────────────────────────────────────────────

type EnvPair = { key: string; value: string };

function parseEnvText(text: string): EnvPair[] {
  const pairs: EnvPair[] = [];
  for (const raw of text.split("\n")) {
    const line = raw.trim();
    if (!line || line.startsWith("#")) continue;
    const eqIdx = line.indexOf("=");
    if (eqIdx === -1) continue;
    const key = line.slice(0, eqIdx).trim();
    let value = line.slice(eqIdx + 1).trim();
    if (
      (value.startsWith('"') && value.endsWith('"')) ||
      (value.startsWith("'") && value.endsWith("'"))
    ) {
      value = value.slice(1, -1);
    }
    if (key) pairs.push({ key, value });
  }
  return pairs;
}

function pairsToText(pairs: EnvPair[]): string {
  return pairs
    .filter((p) => p.key.trim())
    .map((p) => {
      const needsQuote = p.value.includes(" ") || p.value.includes('"') || p.value.includes("'");
      return needsQuote ? `${p.key}="${p.value}"` : `${p.key}=${p.value}`;
    })
    .join("\n");
}

function pairsToRecord(pairs: EnvPair[]): Record<string, string> {
  const result: Record<string, string> = {};
  for (const p of pairs) {
    if (p.key.trim()) result[p.key.trim()] = p.value;
  }
  return result;
}

// ── Editor for a single set of env vars ────────────────────────────

function EnvEditor({
  pairs,
  setPairs,
  mode,
  bulkText,
  setBulkText,
  masked,
}: {
  pairs: EnvPair[];
  setPairs: (p: EnvPair[]) => void;
  mode: "editor" | "raw";
  bulkText: string;
  setBulkText: (t: string) => void;
  masked: boolean;
}) {
  if (mode === "raw") {
    return (
      <div className="space-y-3">
        <p className="text-xs text-muted-foreground">
          Paste environment variables in <code className="rounded bg-muted px-1">KEY=value</code>{" "}
          format, one per line. Lines starting with <code className="rounded bg-muted px-1">#</code>{" "}
          are ignored.
        </p>
        <textarea
          className="h-[300px] w-full rounded-md border border-input bg-muted p-4 font-mono text-sm text-foreground placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          placeholder={`# Database\nDATABASE_URL=postgres://localhost/mydb\nREDIS_URL=redis://localhost:6379`}
          value={bulkText}
          onChange={(e) => setBulkText(e.target.value)}
          spellCheck={false}
        />
      </div>
    );
  }

  if (pairs.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">
        No environment variables. Click <strong>Add</strong> to add one, or switch to{" "}
        <strong>Raw</strong> mode to paste multiple at once.
      </p>
    );
  }

  return (
    <div className="space-y-2">
      {pairs.map((pair, i) => (
        <div key={i} className="flex items-center gap-2">
          <Input
            className="w-48 font-mono text-sm"
            placeholder="KEY"
            value={pair.key}
            onChange={(e) => {
              const next = [...pairs];
              next[i] = { ...next[i], key: e.target.value };
              setPairs(next);
            }}
          />
          <span className="text-muted-foreground">=</span>
          <Input
            className="flex-1 font-mono text-sm"
            placeholder="value"
            type={masked ? "password" : "text"}
            value={pair.value}
            onChange={(e) => {
              const next = [...pairs];
              next[i] = { ...next[i], value: e.target.value };
              setPairs(next);
            }}
            onPaste={(e) => {
              const text = e.clipboardData.getData("text");
              if (text.includes("\n") && text.includes("=")) {
                e.preventDefault();
                const parsed = parseEnvText(text);
                if (parsed.length > 0) {
                  const before = pairs.filter((_, j) => j !== i || pair.key.trim());
                  setPairs([...before, ...parsed]);
                }
              }
            }}
          />
          <Button
            size="icon"
            variant="ghost"
            className="h-8 w-8 shrink-0 text-destructive"
            onClick={() => setPairs(pairs.filter((_, j) => j !== i))}
          >
            <X className="h-3.5 w-3.5" />
          </Button>
        </div>
      ))}
    </div>
  );
}

// ── Secrets section ────────────────────────────────────────────────

function SecretsSection({ appId }: { appId: string }) {
  const { data: rawSecretKeys, isLoading } = useAppSecrets(appId);
  const secretKeys = rawSecretKeys ?? [];
  const updateSecrets = useUpdateSecrets(appId);
  const [pairs, setPairs] = useState<EnvPair[]>([]);
  const [initialized, setInitialized] = useState(false);

  // Initialize pairs from server keys (values masked — never returned)
  if (!initialized && !isLoading && secretKeys) {
    setPairs(secretKeys.map((k) => ({ key: k, value: "" })));
    setInitialized(true);
  }

  function handleSave() {
    const secrets: Record<string, string> = {};
    for (const p of pairs) {
      if (p.key.trim()) {
        secrets[p.key.trim()] = p.value;
      }
    }
    updateSecrets.mutate(secrets);
  }

  const dirty = pairs.some((p) => p.key.trim() && p.value.trim());

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="text-sm">Secrets</CardTitle>
        <div className="flex items-center gap-2">
          <Button
            size="sm"
            variant="outline"
            onClick={() => setPairs([...pairs, { key: "", value: "" }])}
          >
            <Plus className="h-3.5 w-3.5" /> Add
          </Button>
          {dirty && (
            <Button size="sm" onClick={handleSave} disabled={updateSecrets.isPending}>
              <Save className="h-3.5 w-3.5" /> {updateSecrets.isPending ? "Saving..." : "Save"}
            </Button>
          )}
        </div>
      </CardHeader>
      <CardContent>
        <p className="mb-3 text-xs text-muted-foreground">
          Secrets are stored encrypted in Kubernetes. Values are never returned by the API — enter
          new values to update.
        </p>
        {isLoading ? (
          <p className="text-sm text-muted-foreground">Loading...</p>
        ) : pairs.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            No secrets configured. Click <strong>Add</strong> to create one.
          </p>
        ) : (
          <div className="space-y-2">
            {pairs.map((pair, i) => (
              <div key={i} className="flex items-center gap-2">
                <Input
                  className="w-48 font-mono text-sm"
                  placeholder="KEY"
                  value={pair.key}
                  onChange={(e) => {
                    const next = [...pairs];
                    next[i] = { ...next[i], key: e.target.value };
                    setPairs(next);
                  }}
                />
                <span className="text-muted-foreground">=</span>
                <Input
                  className="flex-1 font-mono text-sm"
                  placeholder={pair.value ? "" : "\u25CF\u25CF\u25CF\u25CF\u25CF\u25CF\u25CF\u25CF"}
                  type="password"
                  value={pair.value}
                  onChange={(e) => {
                    const next = [...pairs];
                    next[i] = { ...next[i], value: e.target.value };
                    setPairs(next);
                  }}
                />
                <Button
                  size="icon"
                  variant="ghost"
                  className="h-8 w-8 shrink-0 text-destructive"
                  onClick={() => setPairs(pairs.filter((_, j) => j !== i))}
                >
                  <X className="h-3.5 w-3.5" />
                </Button>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

// ── Main component ─────────────────────────────────────────────────

export function EnvironmentTab({
  appId,
  envVars,
  buildEnvVars,
}: {
  appId: string;
  envVars: Record<string, string>;
  buildEnvVars: Record<string, string>;
}) {
  const updateEnv = useUpdateEnv(appId);
  const updateApp = useUpdateApp(appId);

  // Sub-tab: runtime vs build
  const [subTab, setSubTab] = useState<"runtime" | "build">("runtime");
  // Mode: editor vs raw
  const [mode, setMode] = useState<"editor" | "raw">("editor");
  // Masked toggle
  const [masked, setMasked] = useState(true);

  // Runtime state
  const runtimeInitial = Object.entries(envVars || {}).map(([key, value]) => ({ key, value }));
  const [runtimePairs, setRuntimePairs] = useState<EnvPair[]>(runtimeInitial);
  const [runtimeBulk, setRuntimeBulk] = useState(() => pairsToText(runtimeInitial));

  // Build state
  const buildInitial = Object.entries(buildEnvVars || {}).map(([key, value]) => ({ key, value }));
  const [buildPairs, setBuildPairs] = useState<EnvPair[]>(buildInitial);
  const [buildBulk, setBuildBulk] = useState(() => pairsToText(buildInitial));

  const pairs = subTab === "runtime" ? runtimePairs : buildPairs;
  const setPairs = subTab === "runtime" ? setRuntimePairs : setBuildPairs;
  const bulkText = subTab === "runtime" ? runtimeBulk : buildBulk;
  const setBulkText = subTab === "runtime" ? setRuntimeBulk : setBuildBulk;

  const savedText = pairsToText(subTab === "runtime" ? runtimeInitial : buildInitial);
  const dirty =
    mode === "raw"
      ? bulkText.trim() !== savedText.trim()
      : pairsToText(pairs.filter((p) => p.key.trim())) !== savedText;

  function switchMode(to: "editor" | "raw") {
    if (to === mode) return;
    if (to === "raw") {
      setBulkText(pairsToText(pairs.filter((p) => p.key.trim())));
    } else {
      setPairs(parseEnvText(bulkText));
    }
    setMode(to);
  }

  function handleSave() {
    const activePairs = mode === "raw" ? parseEnvText(bulkText) : pairs;
    const result = pairsToRecord(activePairs);

    if (subTab === "runtime") {
      updateEnv.mutate(result);
    } else {
      updateApp.mutate({ build_env_vars: result } as Partial<{
        build_env_vars: Record<string, string>;
      }>);
    }
  }

  const isSaving = subTab === "runtime" ? updateEnv.isPending : updateApp.isPending;

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <div className="flex items-center gap-3">
            <CardTitle className="text-sm">Environment Variables</CardTitle>
            {/* Sub-tabs: Runtime | Build */}
            <div className="inline-flex rounded-lg border bg-muted p-0.5">
              {(["runtime", "build"] as const).map((t) => (
                <button
                  key={t}
                  type="button"
                  onClick={() => setSubTab(t)}
                  className={`rounded-md px-3 py-1 text-xs font-medium transition-all ${
                    subTab === t
                      ? "bg-background text-foreground shadow-sm"
                      : "text-muted-foreground hover:text-foreground"
                  }`}
                >
                  {t === "runtime" ? "Runtime" : "Build"}
                </button>
              ))}
            </div>
          </div>
          <div className="flex items-center gap-2">
            {/* Mask toggle */}
            <Button
              size="icon"
              variant="ghost"
              className="h-8 w-8"
              onClick={() => setMasked(!masked)}
              title={masked ? "Show values" : "Hide values"}
            >
              {masked ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
            </Button>
            {/* Editor/Raw toggle */}
            <div className="inline-flex rounded-lg border bg-muted p-0.5">
              {(["editor", "raw"] as const).map((m) => (
                <button
                  key={m}
                  type="button"
                  onClick={() => switchMode(m)}
                  className={`rounded-md px-3 py-1 text-xs font-medium transition-all ${
                    mode === m
                      ? "bg-background text-foreground shadow-sm"
                      : "text-muted-foreground hover:text-foreground"
                  }`}
                >
                  {m === "editor" ? "Editor" : "Raw"}
                </button>
              ))}
            </div>
          </div>
        </CardHeader>
        <CardContent>
          {/* Toolbar */}
          <div className="mb-3 flex items-center justify-end gap-2">
            {mode === "editor" && (
              <Button
                size="sm"
                variant="outline"
                onClick={() => setPairs([...pairs, { key: "", value: "" }])}
              >
                <Plus className="h-3.5 w-3.5" /> Add
              </Button>
            )}
            {dirty && (
              <Button size="sm" onClick={handleSave} disabled={isSaving}>
                <Save className="h-3.5 w-3.5" /> {isSaving ? "Saving..." : "Save"}
              </Button>
            )}
          </div>

          <EnvEditor
            pairs={pairs}
            setPairs={setPairs}
            mode={mode}
            bulkText={bulkText}
            setBulkText={setBulkText}
            masked={masked}
          />
        </CardContent>
      </Card>

      <SecretsSection appId={appId} />
    </div>
  );
}
