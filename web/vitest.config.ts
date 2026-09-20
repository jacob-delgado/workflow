import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  // Mirrors vite.config.ts. This config is standalone — it does not extend the
  // vite one — so the alias has to be declared in both or every test fails to
  // resolve `@/`.
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test-setup.ts'],
    include: ['src/**/*.test.{ts,tsx}'],
    coverage: {
      provider: 'v8',
      // text for the console; json-summary feeds a CI coverage comment later.
      reporter: ['text', 'json-summary'],
      include: ['src/**'],
      // src/api/generated is hey-api's generated SDK/types/zod — generated code
      // is not ours to test. src/test holds test-only helpers (fakes), not
      // production code. Both are excluded like prettier/eslint/knip.
      exclude: ['src/**/*.test.{ts,tsx}', 'src/main.tsx', 'src/api/generated/**', 'src/test/**'],
      // Branches level with lines deliberately: a line threshold alone goes
      // green while error and edge arms stay unexercised.
      thresholds: {
        lines: 85,
        branches: 85,
      },
    },
  },
})
