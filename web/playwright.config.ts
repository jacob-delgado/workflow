import { defineConfig, devices } from '@playwright/test'

const isCI = Boolean(process.env.CI)

const hermeticURL = 'http://localhost:4173'
const mockURL = 'http://localhost:4174'

// The e2e run has no backend. It serves two builds of the SPA with Vite preview:
// the production build, whose specs answer the API themselves (page.route) and
// otherwise see the shell and its empty states; and a VITE_MOCK build, which
// fills every section from the mockup's fixtures, so the axe scan and the
// screenshots see what a populated cockpit draws. A spec meant for the
// populated build carries the @populated tag, and runs only there. A later
// change adds `workflow --web` to the server list so the data-driven panels can
// be exercised against a real API too.
export default defineConfig({
  testDir: 'e2e',
  fullyParallel: true,
  forbidOnly: isCI,
  retries: 0,
  // list for the console; the HTML report (with retained traces) is uploaded as
  // a CI artifact on failure, so a red run is debuggable without a re-run.
  reporter: isCI ? [['list'], ['html', { open: 'never' }]] : 'list',
  use: {
    trace: 'retain-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      grepInvert: /@populated/,
      use: { ...devices['Desktop Chrome'], baseURL: hermeticURL },
    },
    {
      name: 'mock',
      grep: /@populated/,
      use: { ...devices['Desktop Chrome'], baseURL: mockURL },
    },
  ],
  webServer: [
    {
      command: 'yarn build && yarn preview --port 4173 --strictPort',
      url: hermeticURL,
      reuseExistingServer: !isCI,
      timeout: 120_000,
    },
    {
      // web/dist, not the embed directory the production build writes, so the
      // two builds never overwrite each other and the binary never embeds a
      // mockup.
      command:
        'yarn vite build --outDir dist --emptyOutDir && yarn vite preview --outDir dist --port 4174 --strictPort',
      env: { VITE_MOCK: 'true' },
      url: mockURL,
      reuseExistingServer: !isCI,
      timeout: 120_000,
    },
  ],
})
