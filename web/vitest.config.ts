import { defineConfig, mergeConfig } from 'vitest/config'
import viteConfig from './vite.config.ts'

// The tests build as the app does — its plugins and its `@/` paths — so this
// adds only what testing needs over vite.config.ts.
export default mergeConfig(
  viteConfig,
  defineConfig({
    test: {
      projects: [
        {
          extends: true,
          test: {
            name: 'unit',
            environment: 'jsdom',
            // One jsdom per worker rather than per file, each file still in a
            // context of its own: making jsdom for each of a hundred files
            // cost over a minute of the run.
            pool: 'vmThreads',
            globals: true,
            setupFiles: ['./src/test-setup.ts', './src/test/dropFocusOnDisable.ts'],
            include: ['src/**/*.test.{ts,tsx}'],
            // Vitest blanks every stylesheet unless told otherwise; the token
            // test reads index.css as text (`?raw`) to check the theme's
            // contrast.
            css: { include: [/src\/index\.css/] },
          },
        },
        {
          // The lint's own rules, each shown firing on code that breaks it.
          // ESLint runs in Node, and its first typed lint builds the
          // TypeScript program, which takes seconds.
          extends: true,
          test: {
            name: 'lint',
            environment: 'node',
            include: ['eslint.config.test.ts'],
            testTimeout: 60_000,
          },
        },
      ],
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
        // untested component or handler show. Trade-off TRADE-8: v8 counts a
        // branch covered once its range has run: unlike gobco's Go floor, it
        // never asks for each condition both ways.
        thresholds: {
          lines: 96,
          statements: 96,
          branches: 92,
          functions: 96,
        },
      },
    },
  }),
)
