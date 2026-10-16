import { defineConfig, devices } from '@playwright/test'

const isCI = Boolean(process.env.CI)

// desktopChrome is the device every Chromium project runs. Where the Chromium
// build Playwright pins cannot be installed, as in a Claude Code cloud session,
// PLAYWRIGHT_CHROMIUM_EXECUTABLE names the Chromium to launch instead; unset,
// Playwright launches its own.
const desktopChrome = {
  ...devices['Desktop Chrome'],
  launchOptions: { executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE },
}

const hermeticURL = 'http://localhost:4173'
const mockURL = 'http://localhost:4174'

// This e2e run has no backend. It serves two builds of the SPA with Vite
// preview, which proxies nothing: the production build, whose specs answer the
// API themselves (page.route) and otherwise see the shell and its empty
// states, every route a spec leaves alone refused by the test that
// e2e/support/fixtures.ts makes; and a VITE_MOCK build, which fills every
// section from the mockup's fixtures, so the axe scan and the screenshots see
// what a populated cockpit draws. A spec meant for the populated build carries
// the @populated tag, and runs only there. The specs under e2e/server drive
// writes against a running `workflow --web` instead, in a run of their own
// (playwright.server.config.ts), so this one leaves them out.
export default defineConfig({
  testDir: 'e2e',
  testIgnore: 'server/**',
  fullyParallel: true,
  // Every core on CI, where nothing else runs beside the two preview servers,
  // and they only serve static files. Playwright's default, half the cores,
  // kept a four-core runner at two workers through the run's longest phase. A
  // developer's machine keeps the default.
  workers: isCI ? '100%' : undefined,
  forbidOnly: isCI,
  retries: 0,
  // list for the console; the HTML report (with retained traces) is uploaded as
  // a CI artifact on failure, so a red run is debuggable without a re-run; and
  // the JSON report is what CI counts for the pull request comment.
  reporter: isCI
    ? [['list'], ['html', { open: 'never' }], ['json', { outputFile: 'reports/e2e.json' }]]
    : 'list',
  use: {
    trace: 'retain-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      grepInvert: /@populated/,
      use: { ...desktopChrome, baseURL: hermeticURL },
    },
    {
      name: 'mock',
      grep: /@populated/,
      use: { ...desktopChrome, baseURL: mockURL },
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
