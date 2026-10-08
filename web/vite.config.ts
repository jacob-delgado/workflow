import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  // The Go binary embeds the built app from internal/web/dist, which is
  // committed so `go install` carries it; build straight into it rather than
  // web/dist. emptyOutDir is explicit because the directory sits outside this
  // project root.
  build: {
    outDir: '../internal/web/dist',
    emptyOutDir: true,
  },
  // `@/` is the src root. Cross-directory imports use it so a file that moves
  // between directories keeps its own imports intact; same-directory siblings
  // stay relative. It is declared once, in tsconfig.app.json, and read from
  // there — here and, through this config, by Vitest.
  resolve: { tsconfigPaths: true },
  // In dev the API and its event stream are served by `workflow --web` on the
  // loopback interface; proxy to it so the SPA's same-origin /api fetches (and
  // the /api/events stream) resolve without CORS.
  server: {
    proxy: {
      '/api': { target: 'http://127.0.0.1:13579', changeOrigin: true },
    },
  },
})
