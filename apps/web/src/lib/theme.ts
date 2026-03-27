/**
 * Theme management — follows Tailwind CSS v4 official dark mode guide.
 * https://tailwindcss.com/docs/dark-mode
 *
 * Uses .dark class on <html> + localStorage for persistence.
 * Three modes: light (explicit), dark (explicit), system (OS preference).
 */

export type Theme = "light" | "dark" | "system";

const KEY = "theme";

export function getTheme(): Theme {
  if (typeof window === "undefined") return "dark";
  const stored = localStorage.getItem(KEY);
  if (stored === "light" || stored === "dark" || stored === "system") return stored;
  return "dark"; // default to dark when no preference is stored
}

/** Apply theme immediately — call on every toggle and on page load. */
export function setTheme(theme: Theme) {
  if (theme === "light") {
    localStorage.setItem(KEY, "light");
    document.documentElement.classList.remove("dark");
  } else if (theme === "dark") {
    localStorage.setItem(KEY, "dark");
    document.documentElement.classList.add("dark");
  } else {
    // System: store explicitly so getTheme() can distinguish from "no preference"
    localStorage.setItem(KEY, "system");
    document.documentElement.classList.toggle(
      "dark",
      window.matchMedia("(prefers-color-scheme: dark)").matches,
    );
  }
  // Also set color-scheme for native elements (scrollbars, inputs)
  document.documentElement.style.colorScheme = document.documentElement.classList.contains("dark")
    ? "dark"
    : "light";
}

/** Call once on app startup (before React renders). */
export function initTheme() {
  // Migrate from old storage key
  const old = localStorage.getItem("sailbox_theme");
  if (old) {
    localStorage.setItem(KEY, old);
    localStorage.removeItem("sailbox_theme");
  }

  const theme = getTheme();
  setTheme(theme);

  // Listen for OS theme changes when in system mode
  window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", () => {
    if (localStorage.getItem(KEY) === "system") {
      setTheme("system");
    }
  });
}
