import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef } from "react";

/**
 * Maps a DB table name from PG NOTIFY to the TanStack Query keys that should
 * be invalidated when that table changes.
 */
function queryKeysForTable(table: string): string[][] {
  switch (table) {
    case "applications":
      return [["apps"], ["projects"]];
    case "deployments":
      return [["apps"], ["deployments"]];
    case "domains":
      return [["apps"]];
    case "managed_databases":
      return [["databases"], ["projects"]];
    case "projects":
      return [["projects"]];
    case "server_nodes":
      return [["nodes"], ["cluster"]];
    case "shared_resources":
      return [["resources"]];
    case "alerts":
      return [["monitoring"]];
    case "cron_jobs":
      return [["cronjobs"], ["projects"]];
    case "cron_job_runs":
      return [["cronjobs"]];
    default:
      return [];
  }
}

const THROTTLE_MS = 1000;

/**
 * Connects to the SSE endpoint and automatically invalidates
 * TanStack Query caches when the database changes.
 *
 * Uses a 1-second throttle to batch rapid events and prevent UI thrashing.
 * Mount once in the dashboard layout — it auto-reconnects on disconnect.
 */
export function useEventSource() {
  const qc = useQueryClient();
  const retryRef = useRef(0);
  const pendingRef = useRef(new Map<string, string[]>());
  const flushTimerRef = useRef<ReturnType<typeof setTimeout>>();

  useEffect(() => {
    let es: EventSource | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;

    function flush() {
      const keys = pendingRef.current;
      if (keys.size === 0) return;
      for (const key of keys.values()) {
        qc.invalidateQueries({ queryKey: key });
      }
      keys.clear();
      flushTimerRef.current = undefined;
    }

    function scheduleFlush() {
      if (flushTimerRef.current) return;
      flushTimerRef.current = setTimeout(flush, THROTTLE_MS);
    }

    function connect() {
      const token = localStorage.getItem("sailbox_token") || "";
      const url = `${window.location.origin}/ws/events?token=${encodeURIComponent(token)}`;
      es = new EventSource(url);

      es.onopen = () => {
        const wasReconnect = retryRef.current > 0;
        retryRef.current = 0;
        console.debug("[SSE] connected");
        // After reconnect, refresh all queries to catch missed updates
        if (wasReconnect) {
          qc.invalidateQueries();
        }
      };

      es.onmessage = (e) => {
        try {
          const change = JSON.parse(e.data) as { table: string; op: string; id: string };
          const keys = queryKeysForTable(change.table);
          for (const key of keys) {
            pendingRef.current.set(key.join("/"), key);
          }
          scheduleFlush();
        } catch {
          // ignore malformed events
        }
      };

      es.onerror = () => {
        console.debug("[SSE] disconnected, reconnecting...");
        es?.close();
        es = null;
        const delay = Math.min(1000 * 2 ** retryRef.current, 30_000);
        retryRef.current++;
        reconnectTimer = setTimeout(connect, delay);
      };
    }

    connect();

    return () => {
      es?.close();
      if (reconnectTimer) clearTimeout(reconnectTimer);
      if (flushTimerRef.current) clearTimeout(flushTimerRef.current);
      flush(); // flush any pending on unmount
    };
  }, [qc]);
}
