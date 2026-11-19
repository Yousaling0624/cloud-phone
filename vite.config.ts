import { defineConfig } from "vite";

export default defineConfig({
  server: {
    // The development app loads this address (`devUrl` in mygo.config.ts).
    port: 5173,
    strictPort: true,
    watch: { ignored: ["**/.mygo/**", "**/build/**", "**/.legacy/**"] },
  },
  build: {
    // The frontend is embedded in the app; a single chunk keeps startup
    // quick and avoids a waterfall of requests over the mygo scheme.
    chunkSizeWarningLimit: 2000,
  },
});
