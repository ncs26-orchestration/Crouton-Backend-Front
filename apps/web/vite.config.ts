import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "node:path";

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
    allowedHosts: ["pabloworkflow.up.railway.app"],
    proxy: {
      "/api": {
        target: process.env.API_URL ?? "http://localhost:8080",
        changeOrigin: true,
        rewrite: (p) => p.replace(/^\/api/, ""),
      },
      "/agent": {
        target: process.env.AGENT_URL ?? "http://localhost:8000",
        changeOrigin: true,
        rewrite: (p) => p.replace(/^\/agent/, ""),
      },
    },
  },
});
