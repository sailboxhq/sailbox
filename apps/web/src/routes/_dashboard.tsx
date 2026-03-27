import { createFileRoute, redirect, useNavigate } from "@tanstack/react-router";
import { useEffect } from "react";
import { AnimatedOutlet } from "@/components/animated-outlet";
import { AppSearch } from "@/components/app-search";
import { BrandGuard } from "@/components/brand-guard";
import { Sidebar, SidebarContext, useSidebarState } from "@/components/layout/sidebar";
import { useEventSource } from "@/hooks/use-event-source";
import { api } from "@/lib/api";
import { clearTokens, getToken, setupAuthRedirect } from "@/lib/auth";

export const Route = createFileRoute("/_dashboard")({
  beforeLoad: async () => {
    if (!getToken()) {
      throw redirect({ to: "/auth/login" });
    }
    // Verify the token is still valid — handles DB reset, expiry, etc.
    try {
      await api.get("/api/v1/auth/me");
    } catch {
      clearTokens();
      throw redirect({ to: "/auth/login" });
    }
  },
  component: DashboardLayout,
});

function DashboardLayout() {
  const navigate = useNavigate();
  const sidebar = useSidebarState();

  useEffect(() => {
    setupAuthRedirect(() => navigate({ to: "/auth/login" }));
  }, [navigate]);

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
