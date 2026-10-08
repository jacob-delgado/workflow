import { test as playwright, type Page } from '@playwright/test'
import type {
  Branch,
  Issue,
  IssuesPage,
  Problem,
  Snapshot,
} from '../../src/api/generated/types.gen.ts'

// The answers the hermetic specs give the page: the event stream's frame, built
// from one empty frame so each spec names only the parts it varies; a
// refusal, built as the server builds one; and a refusal for every route a
// spec leaves unanswered.

// emptyBranch is a branch with nothing checked out: no name, no head, no
// upstream, nothing ahead or behind.
const emptyBranch = {
  name: '',
  issue_link: '',
  detached: false,
  head: '',
  upstream: '',
  push_remote: '',
  ahead: 0,
  behind: 0,
  base: '',
  commits: [],
} satisfies Branch

// emptySnapshot is the stream's frame of a repository with nothing in it: no
// issues, no branch, no changes, no pull request, and messaging not set up.
const emptySnapshot = {
  here: '/home/ana/src/api',
  issues: { total: 0, start_at: 0, unavailable: [], issues: [] },
  branch: emptyBranch,
  changes: { changes: [] },
  review: { found: false, announced: false },
  stages: [
    { step: 'issue', name: 'Issue', system: 'tracker', state: 'not_started' },
    { step: 'branch', name: 'Branch', system: 'git', state: 'not_started' },
    { step: 'commits', name: 'Commits', system: 'git', state: 'not_started' },
    { step: 'review', name: 'Review', system: 'forge', state: 'not_started' },
    { step: 'announce', name: 'Slack', system: 'messaging', state: 'not_started' },
  ],
  messaging: {
    kind: 'slack',
    service: 'Slack',
    configured: false,
    channel: '',
    channels: [],
    author: '',
  },
  branches: [],
  commit_types: ['feat', 'fix'],
  subject_limit: 72,
  suggested_scope: '',
  hooks_unmanaged: 0,
  tasks: { available: true, reason: '', linked: [] },
} satisfies Snapshot

// snapshotWith is the empty frame with the parts given in place of its own.
export function snapshotWith(parts: Partial<Snapshot> = {}): Snapshot {
  return { ...emptySnapshot, ...parts }
}

// branchWith is a branch with nothing checked out but the parts given.
export function branchWith(parts: Partial<Branch>): Branch {
  return { ...emptyBranch, ...parts }
}

// issuesOf is the stream's page of issues holding those given, of a view
// holding total in all.
export function issuesOf(issues: Issue[], total = issues.length): IssuesPage {
  return { total, start_at: 0, unavailable: [], issues }
}

// streams answers the event stream with one snapshot.
export async function streams(page: Page, snapshot: Snapshot): Promise<void> {
  await page.route('**/api/events**', (route) =>
    route.fulfill({
      contentType: 'text/event-stream',
      body: `event: snapshot\ndata: ${JSON.stringify(snapshot)}\n\n`,
    }),
  )
}

// problemBase is where a problem's type points, as the server's does: a
// section of the errors reference page for each code.
const problemBase = 'https://jacob-delgado.github.io/workflow/docs/errors/'

// meanings are the status and title the server fixes for each code
// (codeMeaning in internal/webserver/errors.go).
const meanings: Record<Problem['code'], { status: number; title: string }> = {
  bad_request: { status: 400, title: 'Bad request' },
  unauthorized: { status: 401, title: 'Unauthorized' },
  not_found: { status: 404, title: 'Not found' },
  method_not_allowed: { status: 405, title: 'Method not allowed' },
  conflict: { status: 409, title: 'Conflict' },
  unprocessable: { status: 422, title: 'Unprocessable content' },
  not_set_up: { status: 422, title: 'Not set up' },
  too_long: { status: 422, title: 'Too long' },
  precondition_required: { status: 428, title: 'Precondition required' },
  unreachable: { status: 502, title: 'Upstream unreachable' },
  rate_limited: { status: 503, title: 'Rate limited' },
  fetch_failed: { status: 502, title: 'Fetch failed' },
  check_failed: { status: 422, title: 'Check failed' },
  internal: { status: 500, title: 'Internal error' },
}

// Refusal is a route's answer that refuses: a problem, and its status.
interface Refusal {
  status: number
  contentType: string
  json: Problem
}

// problem is the server's refusal for a code, saying the detail given: an RFC
// 9457 problem whose type, title and status come from the code, as the
// server's problem does.
export function problem(code: Problem['code'], detail: string): Refusal {
  const { status, title } = meanings[code]

  return {
    status,
    contentType: 'application/problem+json',
    json: {
      type: `${problemBase}#${code.replaceAll('_', '-')}`,
      title,
      status,
      detail,
      code,
    } satisfies Problem,
  }
}

// test is Playwright's, with every /api route answered before a spec answers
// any: a read or a write the spec leaves alone is refused as a route the
// server does not have, rather than going on through Vite preview to the port
// a developer's own `workflow --web` listens on. A spec's routes come after,
// so they take precedence, and one that falls back reaches this.
export const test = playwright.extend<{ unanswered: undefined }>({
  unanswered: [
    async ({ page }, use) => {
      await page.route('**/api/**', (route) =>
        route.fulfill(problem('not_found', 'this spec does not answer this route')),
      )
      await use(undefined)
    },
    { auto: true },
  ],
})

export { expect } from '@playwright/test'
