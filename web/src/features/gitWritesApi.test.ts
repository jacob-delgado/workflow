import type { Run, RunEvent } from '@/api/generated/types.gen.ts'
import { readHookSetup, startRun, stopRun, writeHookSetup } from '@/features/branch/gitRunApi.ts'
import { discardFile, readDiff, unstageEverything } from '@/features/branch/stagingApi.ts'
import { startWorkInWorktree } from '@/features/issues/startWorkApi.ts'
import { previewPullRequest } from '@/features/review/openPrApi.ts'
import {
  editPull,
  finishBranch,
  mergePull,
  readMergeOffer,
  readPullText,
  rerunChecks,
} from '@/features/review/reviewWritesApi.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { makeBranch } from '@/test/fixtures.ts'

// The Branch and Review sections' reads and writes: what each asks the server
// and what it answers.

const ended: Run = {
  kind: 'pre_commit',
  title: 'pre-commit',
  state: 'succeeded',
  outcome: 'The pre-commit hook passed.',
  lines: ['ok'],
}

const pull = {
  number: 42,
  url: 'https://forge.example.com/pull/42',
  title: 'Redact',
  state: 'open',
  draft: false,
  approvals: 1,
  changes_requested: false,
  mergeable: 'clean',
} as const

// streamed answers a run's events, one JSON object a line, cut where chunks
// would cut it.
function streamed(lines: string[]): Response {
  const text = lines.join('\n') + '\n'
  const encoder = new TextEncoder()
  const body = new ReadableStream<Uint8Array>({
    start(controller) {
      controller.enqueue(encoder.encode(text.slice(0, 10)))
      controller.enqueue(encoder.encode(text.slice(10)))
      controller.close()
    },
  })

  return new Response(body, { headers: { 'Content-Type': 'application/x-ndjson' } })
}

test('a run hands each event over as it lands and answers how it ended', async () => {
  // Arrange
  const requests = fakeApi({
    '/api/runs': () =>
      streamed([
        JSON.stringify({ run: { ...ended, state: 'in_progress', outcome: '', lines: [] } }),
        '',
        'not json',
        JSON.stringify({ line: 'ok' }),
        JSON.stringify({ run: ended }),
      ]),
  })
  const events: RunEvent[] = []

  // Act
  const answered = await startRun({ kind: 'pre_commit' }, (event) => events.push(event))

  // Assert
  expect(answered).toEqual(ended)
  expect(events.map((event) => event.line ?? event.run?.state)).toEqual([
    'in_progress',
    'ok',
    'succeeded',
  ])
  expect(requests[0]?.method).toBe('POST')
})

test('a run whose stream never says how it ended is refused', async () => {
  // Arrange
  fakeApi({ '/api/runs': () => streamed([JSON.stringify({ line: 'ok' })]) })

  // Act
  const started = startRun({ kind: 'rebase' }, () => {})

  // Assert
  await expect(started).rejects.toThrow('without saying how')
})

test('a run refused before it ran throws the problem', async () => {
  // Arrange
  fakeApi({
    '/api/runs': () =>
      Response.json({ code: 'conflict', detail: 'a run is already going' }, { status: 409 }),
  })

  // Act
  const started = startRun({ kind: 'amend' }, () => {})

  // Assert
  await expect(started).rejects.toMatchObject({ detail: 'a run is already going' })
})

test.each<[string, () => Promise<unknown>, string, object]>([
  ['stopping the run', () => stopRun(), 'DELETE /api/runs/current', {}],
  ['reading the lefthook offer', () => readHookSetup(), 'GET /api/hooks/setup', { offered: false }],
  ['writing lefthook.yml', () => writeHookSetup(true), 'POST /api/hooks/setup', { scripts: 1 }],
  ['reading a diff', () => readDiff('a.go'), 'GET /api/changes/diff?path=a.go', { path: 'a.go' }],
  ['unstaging everything', () => unstageEverything(), 'POST /api/unstage', {}],
  ['discarding a file', () => discardFile('a.go'), 'POST /api/discard', {}],
  ['reading the text', () => readPullText(), 'GET /api/pull-request', { title: 'Redact' }],
  ['editing it', () => editPull('Redact', 'Why'), 'PATCH /api/pull-request', { number: 42 }],
  [
    'reading the merge offer',
    () => readMergeOffer(),
    'GET /api/pull-request/merge',
    { methods: ['squash'] },
  ],
  ['merging', () => mergePull('squash'), 'POST /api/pull-request/merge', { number: 42 }],
  ['finishing', () => finishBranch(), 'POST /api/branch/finish', { name: 'main' }],
  ['re-running', () => rerunChecks(), 'POST /api/review/rerun', { reran: true }],
  [
    'starting work in a worktree from what you have',
    () => startWorkInWorktree('PROJ-7', false),
    'POST /api/worktrees',
    { branch: 'feat/PROJ-7' },
  ],
  [
    'a draft from a template',
    () => previewPullRequest('bugfix'),
    'GET /api/pull-request/draft?template=bugfix',
    { template: 'bugfix' },
  ],
])('%s asks the server and answers what it said', async (_, call, asked, expected) => {
  // Arrange
  const requests = fakeApi({
    '/api/runs/current': () => new Response(null, { status: 204 }),
    '/api/hooks/setup': (_: URL, request: Request) =>
      request.method === 'GET'
        ? { offered: false, hooks: [], config: '', scripts: 0 }
        : { scripts: 1 },
    '/api/changes/diff': { path: 'a.go', lines: [] },
    '/api/unstage': { changes: [] },
    '/api/discard': { changes: [] },
    '/api/pull-request': (_: URL, request: Request) =>
      request.method === 'GET' ? { title: 'Redact', body: '' } : pull,
    '/api/pull-request/merge': (_: URL, request: Request) =>
      request.method === 'GET' ? { pull, methods: ['squash'] } : pull,
    '/api/branch/finish': makeBranch({ name: 'main' }),
    '/api/review/rerun': { reran: true },
    '/api/worktrees': { dir: '/home/ana/src/api-x', shown: '~/src/api-x', branch: 'feat/PROJ-7' },
    '/api/pull-request/draft': {
      title: 't',
      body: '',
      base: 'main',
      head: 'h',
      draft: false,
      needs_push: false,
      reviewers: [],
      templates: ['feature', 'bugfix'],
      template: 'bugfix',
    },
  })

  // Act
  const answered = await call()

  // Assert
  const url = new URL(requests[0]?.url ?? '')
  expect(`${requests[0]?.method ?? ''} ${url.pathname}${url.search}`).toBe(asked)
  expect(answered ?? {}).toMatchObject(expected)
})
