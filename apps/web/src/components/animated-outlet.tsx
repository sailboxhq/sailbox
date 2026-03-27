import { Outlet, useLocation } from "@tanstack/react-router";
import { useEffect, useRef } from "react";

/**
 * Wraps <Outlet /> with a subtle fade-in + slide-up animation
 * that re-triggers on every route change.
 */
export function AnimatedOutlet() {
  const location = useLocation();
  const ref = useRef<HTMLDivElement>(null);

  // biome-ignore lint/correctness/useExhaustiveDependencies: intentionally re-run on pathname change
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    // Remove then re-add the class to restart the animation
    el.classList.remove("animate-page-enter");
    // Force a reflow so the browser sees the class removal
    void el.offsetWidth;
    el.classList.add("animate-page-enter");
  }, [location.pathname]);

  return (
    <div ref={ref} className="animate-page-enter">
      <Outlet />
    </div>
  );
}
