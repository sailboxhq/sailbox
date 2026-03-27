import { Download, Play, Search, Square } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { PodInfo } from "@/types/api";

// ── Helpers ────────────────────────────────────────────────────────

/** Try to parse an RFC3339 timestamp at the start of a log line. */
function parseLogLine(line: string): { timestamp: string | null; text: string } {
  // Match ISO 8601 / RFC3339 at start: 2024-01-15T10:30:00.000Z or similar
  const match = line.match(/^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[^\s]*)\s+(.*)/);
  if (match) {
    try {
      const date = new Date(match[1]);
      if (!Number.isNaN(date.getTime())) {
        return {
          timestamp: date.toLocaleTimeString(),
          text: match[2],
        };
      }
    } catch {
      // fall through
    }
  }
  return { timestamp: null, text: line };
}

/** Highlight search matches in text with <mark> tags. */
function HighlightedText({ text, search }: { text: string; search: string }) {
  if (!search.trim()) return <>{text}</>;

  const escaped = search.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const parts = text.split(new RegExp(`(${escaped})`, "gi"));

  return (
    <>
      {parts.map((part, i) =>
        part.toLowerCase() === search.toLowerCase() ? (
          <mark key={i} className="rounded bg-yellow-500/30 px-0.5 text-yellow-200">
            {part}
          </mark>
        ) : (
          <span key={i}>{part}</span>
        ),
      )}
    </>
  );
}

// ── Web Terminal ──────────────────────────────────────────────────

function WebTerminal({ appId, pods }: { appId: string; pods: PodInfo[] }) {
  const containerRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<{ term: any; fitAddon: any } | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const [connected, setConnected] = useState(false);

  const hasPods = pods.length > 0;

  async function connect() {
    if (!containerRef.current || !hasPods) return;

    // Dynamic import to avoid bundling issues
    const { Terminal } = await import("@xterm/xterm");
    const { FitAddon } = await import("@xterm/addon-fit");
    await import("@xterm/xterm/css/xterm.css");

    // Create terminal
    const term = new Terminal({
      cursorBlink: true,
      fontSize: 13,
      fontFamily: "var(--font-mono), monospace",
      theme: {
        background: "transparent",
      },
    });
    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    term.open(containerRef.current);
    fitAddon.fit();
    termRef.current = { term, fitAddon };

    // Connect WebSocket
    const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
    const ws = new WebSocket(`${proto}//${window.location.host}/ws/terminal/${appId}`);
    wsRef.current = ws;

    ws.onopen = () => {
      setConnected(true);
      // Send initial size
      ws.send(
        JSON.stringify({
          type: "resize",
          cols: term.cols,
          rows: term.rows,
        }),
      );
    };

    ws.onmessage = (e) => {
      term.write(e.data);
    };

    ws.onclose = () => {
      setConnected(false);
      term.write("\r\n[Disconnected]\r\n");
    };

    // Terminal input -> WebSocket
    term.onData((data: string) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(data);
      }
    });

    // Terminal resize -> WebSocket
    term.onResize(({ cols, rows }: { cols: number; rows: number }) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: "resize", cols, rows }));
      }
    });

    // Window resize -> fit
    const resizeObserver = new ResizeObserver(() => fitAddon.fit());
    resizeObserver.observe(containerRef.current);
  }

  function disconnect() {
    wsRef.current?.close();
    wsRef.current = null;
    if (termRef.current) {
      termRef.current.term.dispose();
      termRef.current = null;
    }
    setConnected(false);
  }

  useEffect(() => {
    return () => {
      wsRef.current?.close();
      termRef.current?.term.dispose();
    };
  }, []);

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="text-sm">Web Terminal</CardTitle>
        <div className="flex items-center gap-2">
          <Badge variant={connected ? "success" : "secondary"} className="text-xs">
            {connected ? "Connected" : "Disconnected"}
          </Badge>
          {!connected ? (
            <Button size="sm" variant="outline" onClick={connect} disabled={!hasPods}>
              <Play className="h-3.5 w-3.5" /> Connect
            </Button>
          ) : (
            <Button size="sm" variant="outline" onClick={disconnect}>
              <Square className="h-3.5 w-3.5" /> Disconnect
            </Button>
          )}
        </div>
      </CardHeader>
      <CardContent>
        {!hasPods ? (
          <p className="py-8 text-center text-sm text-muted-foreground">
            No pods running. Deploy the application first.
          </p>
        ) : (
          <div ref={containerRef} className="h-[400px] overflow-hidden rounded-md bg-muted p-1" />
        )}
      </CardContent>
    </Card>
  );
}

// ── Main component ─────────────────────────────────────────────────

export function LogsTab({
  appId,
  appName,
  pods,
}: {
  appId: string;
  appName: string;
  pods: PodInfo[];
}) {
  const [logs, setLogs] = useState<string[]>([]);
  const [connected, setConnected] = useState(false);
  const [selectedPod, setSelectedPod] = useState<string>("");
  const [searchText, setSearchText] = useState("");
  const [subTab, setSubTab] = useState<"logs" | "terminal">("logs");
  const logsEndRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WebSocket | null>(null);

  const hasPods = pods.length > 0;

  useEffect(() => {
    return () => {
      wsRef.current?.close();
    };
  }, []);

  const logsLength = logs.length;
  // biome-ignore lint/correctness/useExhaustiveDependencies: scroll on new log entries
  useEffect(() => {
    logsEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [logsLength]);

  const connect = useCallback(() => {
    wsRef.current?.close();
    const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
    let url = `${proto}//${window.location.host}/ws/logs/${appId}`;
    if (selectedPod && selectedPod !== "__all__") {
      url += `?pod=${encodeURIComponent(selectedPod)}`;
    }
    const ws = new WebSocket(url);
    wsRef.current = ws;
    ws.onopen = () => {
      setConnected(true);
      setLogs([]);
    };
    ws.onmessage = (e) => setLogs((prev) => [...prev.slice(-499), e.data]);
    ws.onclose = () => setConnected(false);
  }, [appId, selectedPod]);

  function disconnect() {
    wsRef.current?.close();
    wsRef.current = null;
  }

  function downloadLogs() {
    const blob = new Blob([logs.join("\n")], { type: "text/plain" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `${appName}-logs.log`;
    a.click();
    URL.revokeObjectURL(url);
  }

  // Filter logs by search
  const filteredLogs = searchText.trim()
    ? logs.filter((line) => line.toLowerCase().includes(searchText.toLowerCase()))
    : logs;

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <div className="flex items-center gap-3">
          <CardTitle className="text-sm">Pod Logs</CardTitle>
          {/* Logs/Terminal sub-tabs */}
          <div className="inline-flex rounded-lg border bg-muted p-0.5">
            {(["logs", "terminal"] as const).map((t) => (
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
                {t === "logs" ? "Logs" : "Terminal"}
              </button>
            ))}
          </div>
        </div>
        <div className="flex items-center gap-2">
          {subTab === "logs" && (
            <>
              <Badge variant={connected ? "success" : "secondary"} className="text-xs">
                {connected ? "Connected" : "Disconnected"}
              </Badge>
              {!connected ? (
                <Button size="sm" variant="outline" onClick={connect} disabled={!hasPods}>
                  <Play className="h-3.5 w-3.5" /> Connect
                </Button>
              ) : (
                <Button size="sm" variant="outline" onClick={disconnect}>
                  <Square className="h-3.5 w-3.5" /> Disconnect
                </Button>
              )}
            </>
          )}
        </div>
      </CardHeader>
      <CardContent>
        {subTab === "terminal" ? (
          <WebTerminal appId={appId} pods={pods} />
        ) : (
          <>
            {/* Controls: Pod selector, Search, Download */}
            <div className="mb-3 flex flex-wrap items-center gap-2">
              {pods.length > 0 && (
                <Select value={selectedPod} onValueChange={setSelectedPod}>
                  <SelectTrigger className="h-8 w-[220px] text-xs">
                    <SelectValue placeholder="All pods" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="__all__">All pods</SelectItem>
                    {pods.map((pod) => (
                      <SelectItem key={pod.name} value={pod.name}>
                        {pod.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )}
              <div className="relative flex-1">
                <Search className="absolute left-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
                <Input
                  className="h-8 pl-7 text-xs"
                  placeholder="Filter logs..."
                  value={searchText}
                  onChange={(e) => setSearchText(e.target.value)}
                />
              </div>
              {logs.length > 0 && (
                <Button size="sm" variant="outline" className="h-8" onClick={downloadLogs}>
                  <Download className="h-3.5 w-3.5" />
                </Button>
              )}
            </div>

            {/* Log output */}
            <div className="h-[400px] overflow-auto rounded-md bg-muted p-4 font-mono text-xs text-foreground">
              {filteredLogs.length === 0 ? (
                <p className="text-muted-foreground">
                  {!hasPods
                    ? "No pods running. Deploy the application to view logs."
                    : connected
                      ? searchText
                        ? "No matching lines."
                        : "Waiting for logs..."
                      : `Click Connect to stream Pod logs.\nkubectl logs -f deployment/${appName}`}
                </p>
              ) : (
                filteredLogs.map((line, i) => {
                  const { timestamp, text } = parseLogLine(line);
                  return (
                    <div key={i} className="leading-5 hover:bg-accent">
                      {timestamp && (
                        <span className="mr-2 text-muted-foreground/70">{timestamp}</span>
                      )}
                      <HighlightedText text={text} search={searchText} />
                    </div>
                  );
                })
              )}
              <div ref={logsEndRef} />
            </div>
          </>
        )}
      </CardContent>
    </Card>
  );
}
