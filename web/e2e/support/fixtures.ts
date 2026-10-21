import { test as playwright, type Page, type Request, type Route } from '@playwright/test'
import type {
  Branch,
  Issue,
  IssuesPage,
  Problem,
  Seed,
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
  base_remote: '',
  ahead: 0,
  behind: 0,
  base: '',
  commits: [],
} satisfies Branch

// emptySnapshot is the stream's frame of a repository with nothing in it: no
// issues, no branch, no changes, no pull request, and messaging not set up.
const emptySnapshot = {
  writes: 0,
  here: '/home/ana/src/api',
  issues: { total: 0, start_at: 0, unavailable: [], issues: [] },
  branch: emptyBranch,
  changes: { changes: [] },
  review: { found: false, announced: false },
  stages: [
    { step: 'issue', name: 'Issue', state: 'not_started' },
    { step: 'branch', name: 'Branch', state: 'not_started' },
    { step: 'commits', name: 'Commits', state: 'not_started' },
    { step: 'review', name: 'Review', state: 'not_started' },
    { step: 'announce', name: 'Slack', state: 'not_started' },
  ],
  unstarted_stages: [
    { step: 'issue', name: 'Issue', state: 'in_flight' },
    { step: 'branch', name: 'Branch', state: 'not_started' },
    { step: 'commits', name: 'Commits', state: 'not_started' },
    { step: 'review', name: 'Review', state: 'not_started' },
    { step: 'announce', name: 'Slack', state: 'not_started' },
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

// reconnectSoon is how long the browser waits to connect the event stream
// again once a body ends: a fulfilled route's body ends at once, so each
// reconnection is the next frame, which a write waits on before the page
// says it is done; the browser's own wait would be seconds.
const reconnectSoon = 'retry: 50\n\n'

// streams answers the event stream with one snapshot, again on each
// reconnection, each time counting the writes the page has been answered.
export async function streams(page: Page, snapshot: Snapshot): Promise<void> {
  await page.route('**/api/events**', (route) =>
    route.fulfill({
      contentType: 'text/event-stream',
      body: `${reconnectSoon}event: snapshot\ndata: ${JSON.stringify(counted(page, snapshot))}\n\n`,
    }),
  )
}

// seeds answers the event stream with the view's kept issues, as the server
// sends them ahead of its first snapshot, then the snapshot when one is given.
export async function seeds(page: Page, seed: Seed, snapshot?: Snapshot): Promise<void> {
  const first = `${reconnectSoon}id: 1\nevent: seed\ndata: ${JSON.stringify(seed)}\n\n`
  const then = (frame: Snapshot) =>
    `id: 2\nevent: snapshot\ndata: ${JSON.stringify(counted(page, frame))}\n\n`

  await page.route('**/api/events**', (route) =>
    route.fulfill({
      contentType: 'text/event-stream',
      body: first + (snapshot === undefined ? '' : then(snapshot)),
    }),
  )
}

// writesAnswered is how many writes each page has been answered, as the
// server counts them: what the frame streams sends it next counts.
const writesAnswered = new WeakMap<Page, number>()

// counted is snapshot as a frame read now, counting the writes page has been
// answered.
function counted(page: Page, snapshot: Snapshot): Snapshot {
  return { ...snapshot, writes: writesAnswered.get(page) ?? 0 }
}

// refusedByTheContract are the statuses the contract's validator refuses a
// request with before the server takes it, which no write lands with: a
// spec's refusal with one is taken for the validator's.
const refusedByTheContract = new Set([400, 401, 404, 405])

// numberingWrites is page with the answer to every write a spec's route
// fulfills numbered, in its Workflow-Write header, as the server numbers every
// write it takes, so the frames streams sends after it count it.
function numberingWrites(page: Page): Page {
  const route = page.route.bind(page)
  page.route = (url, handler, options) =>
    route(
      url,
      (answering, request): unknown =>
        handler(numbered(page, answering, request), request) as unknown,
      options,
    )

  return page
}

// numbered is answering, for a write, with a fulfill that numbers the answer;
// for a read, answering itself.
function numbered(page: Page, answering: Route, request: Request): Route {
  if (['GET', 'HEAD', 'OPTIONS'].includes(request.method())) {
    return answering
  }

  return new Proxy(answering, {
    get(target, key) {
      if (key === 'fulfill') {
        return (answer: Parameters<Route['fulfill']>[0]) =>
          target.fulfill(numberedAnswer(page, answer))
      }

      const value: unknown = Reflect.get(target, key)

      return typeof value === 'function' ? (value as () => unknown).bind(target) : value
    },
  })
}

// writeHeader is the header the server numbers a write's answer in.
const writeHeader = 'Workflow-Write'

// landed counts a write page was answered, and is its number: for a route
// that numbers its own answer, as a run numbers its start and its end.
export function landed(page: Page): number {
  const number = (writesAnswered.get(page) ?? 0) + 1
  writesAnswered.set(page, number)

  return number
}

// numberedAnswer is answer with the number the write it answers lands as,
// counted on page; one the route numbered itself, or a refusal the
// contract's validator would make, it leaves as it is.
function numberedAnswer(
  page: Page,
  answer: Parameters<Route['fulfill']>[0],
): Parameters<Route['fulfill']>[0] {
  if (
    refusedByTheContract.has(answer?.status ?? 200) ||
    answer?.headers?.[writeHeader] !== undefined
  ) {
    return answer
  }

  return { ...answer, headers: { ...answer?.headers, [writeHeader]: String(landed(page)) } }
}

// problemBase is where a problem's type points, as the server's does: a
// section of the errors reference page for each code.
const problemBase = 'https://jacob-delgado.github.io/workflow/docs/errors/'

// meanings are the status and title the server fixes for each code
// (codeMeaning in internal/webserver/problem/problem.go).
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
  ci_failed: { status: 409, title: 'CI failed' },
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
// server's problem.New does.
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
// so they take precedence, and one that falls back reaches this. Its page
// numbers every write a spec answers, as the server does.
export const test = playwright.extend<{ unanswered: undefined }>({
  page: async ({ page }, provide) => {
    await provide(numberingWrites(page))
  },
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
