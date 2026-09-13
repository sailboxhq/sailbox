import path from "node:path";
import tailwindcss from "@tailwindcss/vite";
import { TanStackRouterVite } from "@tanstack/router-vite-plugin";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// Override when the API runs somewhere other than the default port, e.g. when
// 8080 is already taken: API_PROXY_TARGET=http://localhost:8090 bun dev
const apiTarget = process.env.API_PROXY_TARGET || "http://localhost:8080";

export default defineConfig({
  define: {
    __APP_VERSION__: JSON.stringify(process.env.VERSION || "dev"),
  },
  plugins: [react(), tailwindcss(), TanStackRouterVite()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    port: 3000,
    proxy: {
      "/api": apiTarget,
      "/ws": { target: apiTarget, ws: true },
    },
  },
});
