import { defineConfig, devices } from '@playwright/test'

const isCI = Boolean(process.env.CI)

// The e2e run is hermetic: it builds the SPA and serves it with Vite preview,
// with no backend. The cockpit renders its shell and empty states without one;
// a later change adds `workflow --web` to the server list so the data-driven
// panels can be exercised against a real API too.
export default defineConfig({
  testDir: 'e2e',
  fullyParallel: true,
  forbidOnly: isCI,
  retries: 0,
  // list for the console; the HTML report (with retained traces) is uploaded as
  // a CI artifact on failure, so a red run is debuggable without a re-run.
  reporter: isCI ? [['list'], ['html', { open: 'never' }]] : 'list',
  use: {
    baseURL: 'http://localhost:4173',
    trace: 'retain-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  webServer: {
    command: 'yarn build && yarn preview --port 4173 --strictPort',
    url: 'http://localhost:4173',
    reuseExistingServer: !isCI,
    timeout: 120_000,
  },
})
