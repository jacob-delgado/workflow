import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { defineConfig, devices } from '@playwright/test'

const isCI = Boolean(process.env.CI)

// Beside the default port, 13579, and not on it: the run exercises --port,
// and never meets a developer's own `workflow --web` or the /api proxy of the
// Vite preview playwright.config.ts serves, which both use the default.
const port = '13580'

const serverURL = `http://127.0.0.1:${port}`

// The fixture scripts/e2e-server.sh rebuilds on every run: a throwaway home,
// a repository with one untracked file, and the bare repository it pushes to.
// A fixed path, not a fresh temporary one, because each worker evaluates this
// file again and must name the same directory the server was started in.
const fixture = join(tmpdir(), 'workflow-e2e-server')

// The server-backed run: `workflow --web`, the binary `task build` makes with
// the app embedded, serves the page and its API from that repository, so a
// spec here drives a write through to git. It runs apart from
// playwright.config.ts because the server takes a write only from its own
// origin, so the page must come from the binary and not from Vite preview. The
// specs share one repository, so they run one at a time.
export default defineConfig({
  testDir: 'e2e/server',
  workers: 1,
  forbidOnly: isCI,
  retries: 0,
  // As playwright.config.ts: the console, the HTML report CI uploads on
  // failure, and the JSON report CI counts for the pull request comment.
  reporter: isCI
    ? [['list'], ['html', { open: 'never' }], ['json', { outputFile: 'reports/e2e-server.json' }]]
    : 'list',
  use: {
    trace: 'retain-on-failure',
  },
  projects: [
    {
      name: 'server',
      // The spec reads what a push landed from here, with git.
      metadata: { origin: join(fixture, 'origin.git') },
      use: { ...devices['Desktop Chrome'], baseURL: serverURL },
    },
  ],
  webServer: {
    command: `../scripts/e2e-server.sh '${fixture}' ${port}`,
    url: `${serverURL}/api/health`,
    // Never a server already listening on the run's port: whatever answers
    // there, a developer's `workflow --web --port 13580` say, would serve
    // from their own repository, and a push there is real.
    reuseExistingServer: false,
    timeout: 60_000,
  },
})
