import { useNavigate } from "@tanstack/react-router";
import { Database, Search } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { useDashboardApps, useDashboardDatabases } from "@/hooks/use-dashboard";
import { useProjects } from "@/hooks/use-projects";
import { statusDotColor } from "@/lib/constants";
import { cn } from "@/lib/utils";

interface SearchItem {
  id: string;
  name: string;
  status: string;
  project_id: string;
  type: "app" | "database";
}

let openFn: (() => void) | null = null;
export function openAppSearch() {
  openFn?.();
}

export function AppSearch() {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const navigate = useNavigate();
  const { data: apps } = useDashboardApps();
  const { data: databases } = useDashboardDatabases();
  const { data: projects } = useProjects();

  const projectMap = new Map(projects?.map((p) => [p.id, p.name]) ?? []);

  // Merge apps + databases into unified list
  const allItems: SearchItem[] = [
    ...(apps ?? []).map((a) => ({
      id: a.id,
      name: a.name,
      status: a.status,
      project_id: a.project_id,
      type: "app" as const,
    })),
    ...(databases ?? []).map((d) => ({
      id: d.id,
      name: d.name,
      status: d.status,
      project_id: d.project_id,
      type: "database" as const,
    })),
  ];

  const filtered = query
    ? allItems.filter(
        (item) =>
          item.name.toLowerCase().includes(query.toLowerCase()) ||
          (projectMap.get(item.project_id) || "").toLowerCase().includes(query.toLowerCase()),
      )
    : allItems;

  const go = useCallback(
    (idx: number) => {
      const item = filtered[idx];
      if (!item) return;
      setOpen(false);
      if (item.type === "database") {
        navigate({
          to: "/projects/$id/databases/$dbId",
          params: { id: item.project_id, dbId: item.id },
        });
      } else {
        navigate({
          to: "/projects/$id/apps/$appId",
          params: { id: item.project_id, appId: item.id },
        });
      }
    },
    [filtered, navigate],
  );

  useEffect(() => {
    openFn = () => setOpen(true);
    return () => {
      openFn = null;
    };
  }, []);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        setOpen((v) => !v);
      }
    };
    document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, []);

  useEffect(() => {
    if (open) {
      setQuery("");
      setSelected(0);
      setTimeout(() => inputRef.current?.focus(), 0);
    }
  }, [open]);

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setSelected((s) => Math.min(s + 1, filtered.length - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setSelected((s) => Math.max(s - 1, 0));
    } else if (e.key === "Enter") {
      e.preventDefault();
      go(selected);
    } else if (e.key === "Escape") {
      setOpen(false);
    }
  };

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center pt-[20vh]">
      {/* biome-ignore lint/a11y/useKeyWithClickEvents: backdrop */}
      {/* biome-ignore lint/a11y/noStaticElementInteractions: backdrop */}
      <div className="absolute inset-0 bg-black/50" onClick={() => setOpen(false)} />
      <div className="relative w-full max-w-md overflow-hidden rounded-xl border bg-popover shadow-2xl">
        <div className="flex items-center gap-3 border-b px-4 py-3">
          <Search className="h-4 w-4 shrink-0 text-muted-foreground/50" />
          <input
            ref={inputRef}
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setSelected(0);
            }}
            onKeyDown={onKeyDown}
            placeholder="Search services..."
            className="flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground/40"
          />
          <kbd className="hidden rounded border bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground sm:inline">
            ESC
          </kbd>
        </div>

        <div className="max-h-72 overflow-y-auto p-1.5">
          {filtered.length === 0 ? (
            <p className="px-3 py-8 text-center text-sm text-muted-foreground/50">
              {allItems.length === 0 ? "No services yet" : "No results"}
            </p>
          ) : (
            filtered.map((item, i) => (
              <button
                key={`${item.type}-${item.id}`}
                type="button"
                className={cn(
                  "flex w-full items-center gap-3 rounded-lg px-3 py-2 text-left text-sm transition-colors",
                  i === selected
                    ? "bg-accent text-foreground"
                    : "text-muted-foreground hover:bg-accent/50",
                )}
                onMouseEnter={() => setSelected(i)}
                onClick={() => go(i)}
              >
                {item.type === "database" ? (
                  <Database className="h-3.5 w-3.5 shrink-0 text-primary" />
                ) : (
                  <span
                    className={cn(
                      "inline-block h-2 w-2 shrink-0 rounded-full",
                      statusDotColor(item.status),
                    )}
                  />
                )}
                <span className="flex-1 truncate font-medium">{item.name}</span>
                <span className="truncate text-xs text-muted-foreground/50">
                  {projectMap.get(item.project_id) || ""}
                </span>
                {item.type === "database" && (
                  <span className="text-[10px] text-muted-foreground/40">DB</span>
                )}
              </button>
            ))
          )}
        </div>
      </div>
    </div>
  );
}
