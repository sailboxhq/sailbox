import { useQueryClient } from "@tanstack/react-query";
import { createFileRoute, redirect, useNavigate } from "@tanstack/react-router";
import { useEffect } from "react";
import { AnimatedOutlet } from "@/components/animated-outlet";
import { AppSearch } from "@/components/app-search";
import { BrandGuard } from "@/components/brand-guard";
import { Sidebar, SidebarContext, useSidebarState } from "@/components/layout/sidebar";
import { useEventSource } from "@/hooks/use-event-source";
import { api } from "@/lib/api";
import { clearTokens, getToken, setupAuthRedirect } from "@/lib/auth";

// Cache the verification promise so it only runs once per page load,
// not on every route change. This prevents SSE reconnection on navigation.
let verified: Promise<void> | null = null;

function verifyToken(): Promise<void> {
  if (!verified) {
    // api.get resolves with the /auth/me payload, which this guard ignores —
    // map it to void so the cached promise matches the declared return type.
    verified = api
      .get("/api/v1/auth/me")
      .then(() => undefined)
      .catch(() => {
        verified = null;
        clearTokens();
        throw redirect({ to: "/auth/login" });
      });
  }
  return verified;
}

// Reset on logout so next login re-verifies
export function resetVerification() {
  verified = null;
}

export const Route = createFileRoute("/_dashboard")({
  beforeLoad: async () => {
    if (!getToken()) {
      throw redirect({ to: "/auth/login" });
    }
    await verifyToken();
  },
  component: DashboardLayout,
});

function DashboardLayout() {
  const navigate = useNavigate();
  const sidebar = useSidebarState();
  const qc = useQueryClient();

  useEffect(() => {
    setupAuthRedirect(() => {
      resetVerification();
      qc.removeQueries({ queryKey: ["auth", "setup-status"] });
      navigate({ to: "/auth/login" });
    });
  }, [navigate, qc]);

  useEventSource();

  return (
    <SidebarContext.Provider value={sidebar}>
      <div className="flex h-screen bg-background">
        <Sidebar />
        <main className="flex-1 overflow-auto bg-muted/30">
          <div className="mx-auto max-w-6xl px-6 py-6">
            <AnimatedOutlet />
          </div>
        </main>
      </div>
      <AppSearch />
      <BrandGuard />
    </SidebarContext.Provider>
  );
}
