import { act, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ReactElement } from 'react'
import type { Change, Snapshot } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { mockConfig } from '@/dev/mockConfig.ts'
import { BranchPanel } from '@/features/branch/BranchPanel.tsx'
import { IssuesPanel } from '@/features/issues/IssuesPanel.tsx'
import { WorkStory } from '@/features/issues/WorkStory.tsx'
import { MessagingPanel } from '@/features/messaging/MessagingPanel.tsx'
import { ReviewPanel } from '@/features/review/ReviewPanel.tsx'
import { SettingsPanel } from '@/features/settings/SettingsPanel.tsx'
import { fakeApi } from '@/test/fakeApi.ts'
import { makeBranch, makeSnapshot } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'

// Every write the web makes, each from the control a person would use, against
// a server that accepts it: what it did is said in a live status line, which is
// still there once the snapshot that confirms the write arrives — though that
// snapshot takes some of the controls away — and focus is on that line unless
// the control that made the write is still there to hold it.

type User = ReturnType<typeof userEvent.setup>

interface Write {
  name: string
  panel: () => ReactElement
  // The snapshot the panel opens on, and the one the stream pushes once the
  // write lands; a write the snapshot does not show is confirmed by the same
  // frame again.
  before: Snapshot
  after?: Snapshot
  routes: Record<string, unknown>
  act: (user: User) => Promise<void>
  said: string
  // The control that keeps focus, for a write whose control stays put.
  keeps?: string
}

const pull = {
  number: 7,
  url: 'https://forge.example.com/pull/7',
  title: 'fix: redact tokens',
  draft: false,
  approvals: 0,
  changes_requested: false,
  mergeable: 'unknown' as const,
}

const draft = {
  title: 'fix: redact tokens',
  body: 'why',
  base: 'main',
  head: 'fix/PROJ-1',
  draft: false,
  needs_push: false,
}

const opened = {
  pull,
  follow_ups: [
    { action: 'link' as const, issue_key: 'PROJ-1' },
    { action: 'transition' as const, issue_key: 'PROJ-1', status: 'In Review' },
  ],
}

// change is a changed file: an edit the index does not hold, unless a case
// says otherwise.
function change(path: string, overrides: Partial<Change> = {}): Change {
  return {
    path,
    kind: 'modified',
    staged: false,
    has_unstaged: true,
    conflicted: false,
    ...overrides,
  }
}

const staged = { staged: true, has_unstaged: false }

function withChanges(changes: Change[]): Snapshot {
  return makeSnapshot({ changes: { changes } })
}

const committed = makeBranch({
  head: 'a1b2c3d4e5f6',
  commits: [{ hash: 'a1b2c3d', subject: 'fix: redact tokens' }],
})
const unpublished = makeBranch({ upstream: '', ahead: 0 })
const published = makeBranch({ upstream: 'origin/fix/PROJ-1', ahead: 0 })

const inFlight = makeSnapshot({
  issues: {
    total: 1,
    start_at: 0,
    issues: [
      {
        key: 'PROJ-2',
        summary: 'Document the token flow',
        status: 'In Progress',
        status_category: 'indeterminate',
        type: 'Task',
      },
    ],
  },
  branches: [{ name: 'fix/PROJ-2', issue_key: 'PROJ-2', current: false }],
})
const onHead = {
  ...inFlight,
  branches: [{ name: 'fix/PROJ-2', issue_key: 'PROJ-2', current: true }],
}
const started = makeSnapshot({
  branches: [{ name: 'feat/PROJ-3-metrics', issue_key: 'PROJ-3', current: true }],
})
const withPull = makeSnapshot({ review: { found: true, pull } })

// openThePull opens the pull request the review panel offers.
async function openThePull(user: User): Promise<void> {
  await user.click(screen.getByRole('button', { name: 'Open a pull request' }))
  await user.click(await screen.findByRole('button', { name: 'Open pull request' }))
}

const writes: Write[] = [
  {
    name: 'commit',
    panel: () => <BranchPanel />,
    before: withChanges([change('reqlog.go', staged)]),
    after: makeSnapshot({ branch: committed }),
    routes: { '/api/commit': committed },
    act: async (user) => {
      await user.type(screen.getByLabelText('Subject'), 'redact tokens')
      await user.click(screen.getByRole('button', { name: 'Commit staged changes' }))
    },
    said: 'Committed a1b2c3d fix: redact tokens.',
  },
  {
    name: 'push',
    panel: () => <BranchPanel />,
    before: makeSnapshot({ branch: unpublished }),
    after: makeSnapshot({ branch: published }),
    routes: { '/api/push': published },
    act: async (user) => {
      await user.click(screen.getByRole('button', { name: 'Push branch' }))
      await user.click(screen.getByRole('button', { name: 'Push' }))
    },
    said: 'Pushed fix/PROJ-1.',
  },
  {
    name: 'check out from the list',
    panel: () => <IssuesPanel />,
    before: inFlight,
    after: onHead,
    routes: { '/api/checkout': makeBranch({ name: 'fix/PROJ-2' }) },
    act: async (user) => {
      await user.click(screen.getByRole('button', { name: 'Check out PROJ-2' }))
    },
    said: 'Checked out fix/PROJ-2.',
  },
  {
    name: 'check out from the work story',
    panel: () => <WorkStory issueKey="PROJ-2" />,
    before: inFlight,
    after: onHead,
    routes: { '/api/checkout': makeBranch({ name: 'fix/PROJ-2' }) },
    act: async (user) => {
      await user.click(screen.getByRole('button', { name: 'Check out this branch' }))
    },
    said: 'Checked out fix/PROJ-2.',
  },
  {
    name: 'start work',
    panel: () => <WorkStory issueKey="PROJ-3" />,
    before: makeSnapshot(),
    after: started,
    routes: { '/api/branches': makeBranch({ name: 'feat/PROJ-3-metrics' }) },
    act: async (user) => {
      await user.click(screen.getByRole('button', { name: 'Start work' }))
    },
    said: 'Started work on feat/PROJ-3-metrics.',
  },
  {
    name: 'open a pull request',
    panel: () => <ReviewPanel />,
    before: makeSnapshot(),
    after: withPull,
    routes: { '/api/pull-request/draft': draft, '/api/pull-request': opened },
    act: openThePull,
    said: 'Opened pull request #7.',
  },
  {
    name: 'link it on the issue',
    panel: () => <ReviewPanel />,
    before: makeSnapshot(),
    routes: {
      '/api/pull-request/draft': draft,
      '/api/pull-request': opened,
      '/api/issues/PROJ-1/link': pull,
    },
    act: async (user) => {
      await openThePull(user)
      await user.click(await screen.findByRole('button', { name: 'Link it on PROJ-1' }))
    },
    said: 'Linked #7 on PROJ-1.',
  },
  {
    name: 'move the issue',
    panel: () => <ReviewPanel />,
    before: makeSnapshot(),
    routes: {
      '/api/pull-request/draft': draft,
      '/api/pull-request': opened,
      '/api/issues/PROJ-1/transition': { key: 'PROJ-1', status: 'In Review' },
    },
    act: async (user) => {
      await openThePull(user)
      await user.click(await screen.findByRole('button', { name: 'Move PROJ-1 to In Review' }))
    },
    said: 'Moved PROJ-1 to In Review.',
  },
  {
    name: 'announce',
    panel: () => <MessagingPanel />,
    before: withPull,
    routes: {
      '/api/announcement': { text: 'octocat opened #7', channel: '#dev' },
      '/api/announce': { text: 'octocat opened #7', channel: '#dev' },
    },
    act: async (user) => {
      await user.click(screen.getByRole('button', { name: 'Announce to Slack' }))
      await user.click(await screen.findByRole('button', { name: 'Announce now' }))
    },
    said: 'Announced to #dev.',
  },
  {
    name: 'save the configuration',
    panel: () => <SettingsPanel />,
    before: makeSnapshot(),
    routes: { '/api/config': mockConfig },
    act: async (user) => {
      await screen.findByLabelText('Base URL')
      await user.click(screen.getByRole('button', { name: 'Save changes' }))
    },
    said: 'Saved.',
    keeps: 'Save changes',
  },
  {
    name: 'stage a file',
    panel: () => <BranchPanel />,
    before: withChanges([change('notes.txt')]),
    after: withChanges([change('notes.txt', staged)]),
    routes: { '/api/stage': { changes: [change('notes.txt', staged)] } },
    act: async (user) => {
      await user.click(screen.getByRole('button', { name: 'Stage notes.txt' }))
    },
    said: 'Staged notes.txt.',
  },
  {
    name: 'unstage a file',
    panel: () => <BranchPanel />,
    before: withChanges([change('notes.txt', staged)]),
    after: withChanges([change('notes.txt')]),
    routes: { '/api/unstage': { changes: [change('notes.txt')] } },
    act: async (user) => {
      await user.click(screen.getByRole('button', { name: 'Unstage notes.txt' }))
    },
    said: 'Unstaged notes.txt.',
  },
  {
    name: 'stage all',
    panel: () => <BranchPanel />,
    before: withChanges([change('a.go'), change('b.go')]),
    after: withChanges([change('a.go', staged), change('b.go', staged)]),
    routes: { '/api/stage': { changes: [change('a.go', staged), change('b.go', staged)] } },
    act: async (user) => {
      await user.click(screen.getByRole('button', { name: 'Stage all' }))
    },
    said: 'Staged every change.',
  },
]

// statusSaying is the live status line that says text, if one does.
function statusSaying(text: string): HTMLElement | undefined {
  return screen.queryAllByRole('status').find((region) => region.textContent === text)
}

test.each(writes)(
  '$name says what it did in a line the snapshot leaves standing',
  async (write) => {
    // Arrange
    fakeApi(write.routes)
    useSnapshotStore.setState({ status: 'live', snapshot: write.before })
    const user = userEvent.setup()
    renderWithClient(write.panel())

    // Act: make the write, then the stream confirms it
    await write.act(user)
    await waitFor(() => {
      expect(statusSaying(write.said)).toBeDefined()
    })
    act(() => {
      useSnapshotStore.setState({ snapshot: structuredClone(write.after ?? write.before) })
    })

    // Assert
    const line = statusSaying(write.said)
    expect(line).toBeDefined()
    const holder =
      write.keeps === undefined ? line : screen.getByRole('button', { name: write.keeps })
    expect(document.activeElement).toBe(holder)
  },
)

test('focus the user has moved on to stays where they put it', async () => {
  // Arrange
  // The check-out lands, and the user tabs on before the stream confirms it.
  fakeApi({ '/api/checkout': makeBranch({ name: 'fix/PROJ-2' }) })
  useSnapshotStore.setState({ status: 'live', snapshot: inFlight })
  const user = userEvent.setup()
  renderWithClient(<IssuesPanel />)
  await user.click(screen.getByRole('button', { name: 'Check out PROJ-2' }))
  await waitFor(() => {
    expect(statusSaying('Checked out fix/PROJ-2.')).toBeDefined()
  })
  const filter = screen.getByRole('searchbox', { name: 'Filter' })
  filter.focus()

  // Act
  act(() => {
    useSnapshotStore.setState({ snapshot: structuredClone(onHead) })
  })

  // Assert
  expect(document.activeElement).toBe(filter)
})

// acceptingOnce is the routes each taking the first request and refusing every
// one after it, with no reason of its own.
function acceptingOnce(routes: Record<string, unknown>): Record<string, unknown> {
  return Object.fromEntries(
    Object.entries(routes).map(([path, answer]) => {
      let taken = 0

      return [
        path,
        () => {
          taken += 1

          return taken === 1 ? answer : Response.json({}, { status: 500 })
        },
      ]
    }),
  )
}

// The writes whose control stays to be pressed again while the snapshot stands
// still, each sent to its one route.
const repeatable = writes.filter((write) =>
  [
    'commit',
    'push',
    'check out from the list',
    'check out from the work story',
    'start work',
  ].includes(write.name),
)

test.each(repeatable)('$name made again and refused takes the last success away', async (write) => {
  // Arrange
  fakeApi(acceptingOnce(write.routes))
  useSnapshotStore.setState({ status: 'live', snapshot: write.before })
  const user = userEvent.setup()
  renderWithClient(write.panel())
  await write.act(user)
  await waitFor(() => {
    expect(statusSaying(write.said)).toBeDefined()
  })

  // Act
  await write.act(user)

  // Assert
  await screen.findByRole('alert')
  expect(statusSaying(write.said)).toBeUndefined()
})
