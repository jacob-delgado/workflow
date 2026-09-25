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
    // Vitest blanks every stylesheet unless told otherwise; the token test reads
    // index.css as text (`?raw`) to check the theme's contrast.
    css: { include: [/src\/index\.css/] },
    coverage: {
      provider: 'v8',
      // text for the console; json-summary feeds `task test:summary`.
      reporter: ['text', 'json-summary'],
      include: ['src/**'],
      // Left out: the tests themselves; src/main.tsx, the entry point that only
      // mounts the app into the page; src/api/generated, hey-api's generated
      // SDK, types and zod schemas, which are not ours to test; src/test, the
      // test-only helpers and fakes; and src/dev, the mock data `task
      // web:mockup` loads when VITE_MOCK is set.
      exclude: [
        'src/**/*.test.{ts,tsx}',
        'src/main.tsx',
        'src/api/generated/**',
        'src/test/**',
        'src/dev/**',
      ],
      // Each metric is held at floor(measured) − 2, the ratchet CLAUDE.md sets
      // for the Go floors: raise one only once the coverage is already there.
      // Branches sits beside lines because a line threshold alone goes green
      // while error arms stay unexercised; functions and statements make a new
      // untested component or handler show. v8 counts a branch covered once its
      // range has run: unlike gobco's Go floor, it never asks for each
      // condition both ways.
      thresholds: {
        lines: 96,
        statements: 96,
        branches: 94,
        functions: 98,
      },
    },
  },
})
