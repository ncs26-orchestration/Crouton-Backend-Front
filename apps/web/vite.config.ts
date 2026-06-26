import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "node:path";

// Both the dev server and `vite preview` route /api and /agent to the Go API
// and Python agent. Targets come from env so the same build works locally and
// in containers; share one proxy map so the two stay in sync.
const proxy = {
  "/api": {
    target: process.env.API_URL ?? "http://localhost:8080",
    changeOrigin: true,
    rewrite: (p: string) => p.replace(/^\/api/, ""),
  },
  "/agent": {
    target: process.env.AGENT_URL ?? "http://localhost:8000",
    changeOrigin: true,
    rewrite: (p: string) => p.replace(/^\/agent/, ""),
  },
};

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    port: 5173,
    host: true,
    allowedHosts: ["croutonworkflow.up.railway.app"],
    proxy,
  },
  preview: {
    port: 5174,
    host: true,
    proxy,
  },
});
