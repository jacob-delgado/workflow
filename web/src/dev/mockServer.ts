import type {
  Branch,
  Config,
  PullRequest,
  Run,
  RunEvent,
  RunRequest,
} from '@/api/generated/types.gen.ts'
import {
  zActivityPostRequest,
  zAnnounceRequest,
  zAssignRequest,
  zBranchIssueRequest,
  zCheckoutRequest,
  zCommitRequest,
  zConfig,
  zCreateBranchRequest,
  zCreateWorktreeRequest,
  zHookSetupRequest,
  zOpenPullRequestRequest,
  zPersonLink,
  zPullRequestText,
  zRepoGroupsRequest,
  zRunRequest,
  zStatusChangeRequest,
  zWorklogRequest,
} from '@/api/generated/zod.gen.ts'
import { mockActivity } from './mockActivity.ts'
import { mockAnnouncement } from './mockAnnouncement.ts'
import { mockConfig } from './mockConfig.ts'
import { mockIssueDetail, mockStatusChanges, mockViews } from './mockIssues.ts'
import { mockKeys } from './mockKeys.ts'
import { mockLocalData, mockRemoveLocalData } from './mockLocalData.ts'
import { mockDirectories, mockRepositories } from './mockRepositories.ts'
import { mockCheckLog, mockPullDraft, mockPullText, mockReviewQueue } from './mockReviews.ts'
import {
  MockRefusal,
  mockForgetPerson,
  mockLinkPerson,
  mockPeople,
  mockRepoGroups,
  mockSetRepoGroups,
  mockSlackGroups,
  mockSlackMembers,
  mockTagging,
} from './mockSlack.ts'
import { mockSnapshot } from './mockSnapshot.ts'
import { mockDone, mockTaskList } from './mockTasks.ts'

// The mockup's server: `task web:mockup` installs it as the page loads, in
// place of the network and the event stream, so the page asks it as it asks
// workflow and every section fills with no backend. It answers the API's
// routes in the contract's shapes from the mockup's data — the generated
// client validates each answer as it would the server's — and refuses as the
// server does, with a problem; a route it does not answer goes on to the
// network.

// Asked is a request as a route reads it: its address, the named parts of its
// path, and its JSON body, undefined for none.
interface Asked {
  url: URL
  params: Record<string, string>
  body: unknown
}

// A route answers what was asked: a body the server would send as JSON, or a
// Response of its own for an answer that is not one (nothing, a stream).
type Route = (asked: Asked) => unknown

// Routes are the routes by method and path, as the contract names them:
// "GET /api/issues/{key}".
type Routes = Record<string, Route>

// Where a problem's type points, as the server's do: one anchor per code.
const problemBase = 'https://jacob-delgado.github.io/workflow/docs/errors/'

// installMockServer stands the mockup's server in for the network and the
// event stream.
export function installMockServer(): void {
  const network = globalThis.fetch.bind(globalThis)
  const routes = Object.entries(allRoutes())
  globalThis.fetch = (input, init) => answer(routes, new Request(input, init), network)
  globalThis.EventSource = MockEventSource as unknown as typeof EventSource
}

// answer is the mockup's answer to request, or the network's where no route
// matches it.
async function answer(
  routes: [string, Route][],
  request: Request,
  network: typeof fetch,
): Promise<Response> {
  const url = new URL(request.url)
  const found = matched(routes, request.method, url.pathname)
  if (found === undefined) {
    return network(request)
  }

  const body: unknown = request.headers.get('Content-Type')?.includes('json')
    ? await request.json()
    : undefined
  try {
    const answered = await found.route({ url, params: found.params, body })

    return answered instanceof Response ? answered : Response.json(answered)
  } catch (refused) {
    if (refused instanceof MockRefusal) {
      return unprocessable(refused.message)
    }

    throw refused
  }
}

// matched is the route for method on path, with the named parts of its path.
function matched(
  routes: [string, Route][],
  method: string,
  path: string,
): { route: Route; params: Record<string, string> } | undefined {
  const parts = path.split('/')
  for (const [name, route] of routes) {
    const [routeMethod, routePath = ''] = name.split(' ')
    const params = routeMethod === method ? paramsOf(routePath.split('/'), parts) : undefined
    if (params !== undefined) {
      return { route, params }
    }
  }

  return undefined
}

// paramsOf is the named parts of a path its route's parts match, or undefined
// where they do not.
function paramsOf(pattern: string[], parts: string[]): Record<string, string> | undefined {
  if (pattern.length !== parts.length) {
    return undefined
  }

  const params: Record<string, string> = {}
  for (const [index, part] of pattern.entries()) {
    const given = parts[index] ?? ''
    if (part.startsWith('{')) {
      params[part.slice(1, -1)] = decodeURIComponent(given)
    } else if (part !== given) {
      return undefined
    }
  }

  return params
}

// unprocessable is the server's refusal of a request it understood but will
// not carry out, worded as it words one.
function unprocessable(detail: string): Response {
  const status = 422

  return Response.json(
    {
      type: `${problemBase}#unprocessable`,
      title: 'Unprocessable Entity',
      status,
      detail,
      code: 'unprocessable',
    },
    { status, headers: { 'Content-Type': 'application/problem+json' } },
  )
}

// nothing is an answer with no body, as the server gives one for a write that
// has nothing to say.
function nothing(): Response {
  return new Response(null, { status: 204 })
}

// MockEventSource is the mockup's event stream: it opens, then pushes the mock
// snapshot, for whichever view it is asked for.
class MockEventSource extends EventTarget {
  static readonly CONNECTING = 0
  static readonly OPEN = 1
  static readonly CLOSED = 2

  readonly url: string
  readyState = MockEventSource.CONNECTING

  constructor(url: string | URL) {
    super()
    this.url = String(url)
    queueMicrotask(() => {
      this.push()
    })
  }

  close(): void {
    this.readyState = MockEventSource.CLOSED
  }

  private push(): void {
    if (this.readyState === MockEventSource.CLOSED) {
      return
    }

    this.readyState = MockEventSource.OPEN
    this.dispatchEvent(new Event('open'))
    this.dispatchEvent(new MessageEvent('snapshot', { data: JSON.stringify(mockSnapshot) }))
  }
}

function allRoutes(): Routes {
  return {
    'GET /api/health': () => ({
      version: 'mockup',
      dry_run: false,
      forge_kind: 'github',
      forge_noun: 'pull request',
      forge_sigil: '#',
    }),
    'GET /api/keys': () => mockKeys,
    ...issueRoutes(),
    ...branchRoutes(),
    ...runRoutes(),
    ...reviewRoutes(),
    ...messagingRoutes(),
    ...peopleRoutes(),
    ...configRoutes(),
    ...taskRoutes(),
    'GET /api/activity': ({ url }) => {
      const from = url.searchParams.get('from')
      const to = url.searchParams.get('to')

      return mockActivity(from === null || to === null ? null : { from, to })
    },
    'POST /api/activity/post': ({ body }) => {
      const { from, to, text, channel = '' } = zActivityPostRequest.parse(body)
      const destination = channel === '' ? 'the channel its webhook is bound to' : channel

      return { from, to, channel, destination, text }
    },
    'GET /api/repositories': () => mockRepositories(),
    'GET /api/directories': ({ url }) =>
      mockDirectories(url.searchParams.get('path') ?? '/home/ana/src/api/cmd'),
    'GET /api/reviews': () => mockReviewQueue(),
  }
}

// issueRoutes are an issue's reads and the tracker's writes, each answered as
// the tracker would.
function issueRoutes(): Routes {
  return {
    'GET /api/views': () => mockViews,
    'GET /api/issues/{key}': ({ params }) => mockIssueDetail(params.key ?? ''),
    'GET /api/issues/{key}/transitions': () => mockStatusChanges,
    'POST /api/issues/{key}/transitions': ({ params, body }) => {
      const { transition_id: id } = zStatusChangeRequest.parse(body)
      const change = mockStatusChanges.find((offered) => offered.id === id)

      return { key: params.key, status: change?.to_status ?? '' }
    },
    'PUT /api/issues/{key}/assignee': ({ params, body }) => ({
      key: params.key,
      assignee: zAssignRequest.parse(body).assignee.trim(),
    }),
    'POST /api/issues/{key}/worklog': ({ params, body }) => ({
      key: params.key,
      time_spent: zWorklogRequest.parse(body).time_spent.trim(),
    }),
    'POST /api/issues/{key}/comment': () => ({
      author: 'Ana Souza',
      body: 'Commented from the mockup.',
      created: new Date().toISOString(),
    }),
    'POST /api/issues/{key}/link': () => mockPull(),
    'POST /api/issues/{key}/transition': ({ params }) => ({ key: params.key, status: 'In Review' }),
    'POST /api/checkout': ({ body }) => ({
      ...mockSnapshot.branch,
      name: zCheckoutRequest.parse(body).branch,
    }),
    'POST /api/branches': ({ body }) => ({
      ...mockSnapshot.branch,
      name: `feat/${zCreateBranchRequest.parse(body).issue_key}`,
      upstream: '',
      ahead: 0,
      behind: 0,
      commits: [],
    }),
    'POST /api/worktrees': ({ body }) => {
      const key = zCreateWorktreeRequest.parse(body).issue_key

      return {
        dir: `/home/ana/src/api-feat-${key}`,
        shown: `~/src/api-feat-${key}`,
        branch: `feat/${key}`,
      }
    },
  }
}

// branchRoutes are the working tree's and the branch's writes, each answering
// the mockup's tree or branch as it would stand after.
function branchRoutes(): Routes {
  const changes = () => mockSnapshot.changes

  return {
    'POST /api/stage': changes,
    'POST /api/unstage': changes,
    'POST /api/discard': changes,
    'GET /api/changes/diff': ({ url }) => {
      const path = url.searchParams.get('path') ?? ''

      return {
        path,
        lines: [
          `--- a/${path}`,
          `+++ b/${path}`,
          '@@ -1,3 +1,3 @@',
          ' package redact',
          '-const mask = "***"',
          '+const mask = "[redacted]"',
        ],
      }
    },
    'POST /api/commit': ({ body }) => committed(zCommitRequest.parse(body)),
    'POST /api/push': () => ({ ...mockSnapshot.branch, ahead: 0 }),
    'GET /api/branch/issue/preview': ({ url }) => ({
      key: (url.searchParams.get('key') ?? '').replace(/^#/, ''),
      pull: 0,
      body: '',
      changes: false,
    }),
    'PUT /api/branch/issue': ({ body }) => ({
      ...mockSnapshot.branch,
      issue_link: zBranchIssueRequest.parse(body).key.replace(/^#/, ''),
    }),
    'DELETE /api/branch/issue': () => ({ ...mockSnapshot.branch, issue_link: '' }),
  }
}

// committed is the mockup's branch with a commit of message on top.
function committed(message: { type: string; subject: string }): Branch {
  const hash = 'd4e5f6a7'
  const subject = `${message.type}: ${message.subject}`

  return {
    ...mockSnapshot.branch,
    head: hash,
    commits: [...mockSnapshot.branch.commits, { hash, subject, unpushed: true }],
  }
}

// runRoutes are the git runs and the hooks: a run streams two lines and
// passes, and the lefthook offer is for an old pre-commit hook.
function runRoutes(): Routes {
  return {
    'POST /api/runs': ({ body }) => streamedRun(zRunRequest.parse(body).kind),
    'DELETE /api/runs/current': nothing,
    'GET /api/hooks/setup': () => ({
      offered: true,
      hooks: [{ name: 'pre-commit', lines: 3 }],
      config: 'pre-commit:\n  jobs:\n    - name: go-vet\n      run: go vet ./...\n',
      scripts: 0,
    }),
    'POST /api/hooks/setup': ({ body }) => ({
      scripts: zHookSetupRequest.parse(body).verbatim ? 1 : 0,
    }),
  }
}

// streamedRun is a run of kind as the server streams one, an event a line:
// the run as it starts, each line of its output, and the run as it passed.
function streamedRun(kind: RunRequest['kind']): Response {
  const lines = ['lefthook v1.11.0  hook: pre-commit', '✔️ golangci-lint (2.31 seconds)']
  const run: Run = {
    kind,
    title: kind === 'pre_commit' ? 'pre-commit' : `git ${kind}`,
    state: 'in_progress',
    outcome: '',
    lines: [],
  }
  const events: RunEvent[] = [
    { run },
    ...lines.map((line) => ({ line })),
    { run: { ...run, state: 'succeeded', outcome: 'It went through.', lines } },
  ]

  return new Response(events.map((event) => `${JSON.stringify(event)}\n`).join(''), {
    headers: { 'Content-Type': 'application/x-ndjson' },
  })
}

// mockPull is the mockup's open pull request.
function mockPull(): PullRequest {
  return mockSnapshot.review.pull as PullRequest
}

// reviewRoutes are the pull request's reads and writes, and its CI's.
function reviewRoutes(): Routes {
  return {
    'GET /api/pull-request/draft': () => mockPullDraft,
    'POST /api/pull-request': ({ body }) => {
      const { title, draft = false } = zOpenPullRequestRequest.parse(body)

      return {
        pull: { ...mockPull(), number: 42, url: 'https://example.com/pull/42', title, draft },
        follow_ups: [
          { action: 'link', issue_key: 'PROJ-412' },
          { action: 'transition', issue_key: 'PROJ-412', status: 'In Review' },
        ],
      }
    },
    'GET /api/pull-request': () => mockPullText,
    'PATCH /api/pull-request': ({ body }) => ({
      ...mockPull(),
      title: zPullRequestText.parse(body).title,
    }),
    'GET /api/pull-request/merge': () => ({ pull: mockPull(), methods: ['squash', 'merge'] }),
    'POST /api/pull-request/merge': () => ({ ...mockPull(), state: 'merged' }),
    'POST /api/branch/finish': () => ({ ...mockSnapshot.branch, name: 'main', commits: [] }),
    'POST /api/review/rerun': () => ({ reran: true }),
    'GET /api/review/checks/{id}/log': () => mockCheckLog,
  }
}

// messagingRoutes are the announcement's preview and its post: posted at
// once, or held while the pull request's CI runs.
function messagingRoutes(): Routes {
  const previewed = () => ({ ...mockAnnouncement, tagging: mockTagging() })

  return {
    'GET /api/announcement': previewed,
    'POST /api/announce': ({ body }) => {
      const { channel, edited_text: edited, when } = zAnnounceRequest.parse(body)
      if (when === 'ci_passes') {
        return Response.json(
          { state: 'waiting', channel, pull: mockPull().number },
          { status: 202 },
        )
      }

      const preview = previewed()

      return { ...preview, text: edited ?? preview.text, channel }
    },
    'DELETE /api/announce/queued': nothing,
  }
}

// peopleRoutes are whom each code owner is on Slack, the repository's groups,
// Slack's directory and the local store, each keeping what a write changed
// until the page loads again.
function peopleRoutes(): Routes {
  return {
    'GET /api/people': () => mockPeople(),
    'PUT /api/people': ({ body }) => mockLinkPerson(zPersonLink.parse(body)),
    'DELETE /api/people': ({ url }) => mockForgetPerson(url.searchParams.get('owner') ?? ''),
    'GET /api/repo-groups': () => mockRepoGroups(),
    'PUT /api/repo-groups': ({ body }) => mockSetRepoGroups(zRepoGroupsRequest.parse(body).ids),
    'GET /api/slack/members': ({ url }) => mockSlackMembers(url.searchParams.get('channel') ?? ''),
    'GET /api/slack/groups': () => mockSlackGroups(),
    'GET /api/local-data': () => mockLocalData(),
    'DELETE /api/local-data': ({ url }) =>
      mockRemoveLocalData(url.searchParams.get('scope') === 'all' ? 'all' : 'cache'),
  }
}

// configRoutes are the configuration's read and its save, which the read
// after it shows, at a revision of its own, until the page loads again.
function configRoutes(): Routes {
  let config: Config = mockConfig
  let revision = 1
  const read = () => Response.json(config, { headers: { ETag: `"mock-${String(revision)}"` } })

  return {
    'GET /api/config': read,
    'PUT /api/config': ({ body }) => {
      config = zConfig.parse(body)
      revision += 1

      return read()
    },
  }
}

// taskRoutes are your tasks: the list, and every write on it, which answers
// the list as it stands; a done answers it without the task, and the task as
// it stands done.
function taskRoutes(): Routes {
  const listed = () => mockTaskList()

  return {
    ...Object.fromEntries(
      [
        'GET /api/tasks',
        'POST /api/tasks',
        'POST /api/tasks/track',
        'POST /api/tasks/undo',
        'POST /api/tasks/sync',
        ...['start', 'stop', 'annotations', 'modify'].map(
          (write) => `POST /api/tasks/{uuid}/${write}`,
        ),
      ].map((name) => [name, listed]),
    ),
    'POST /api/tasks/{uuid}/done': ({ params }) => mockDone(params.uuid ?? ''),
  }
}
