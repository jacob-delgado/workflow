import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import type { Issue } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeSnapshot, makeTaskBranch } from '@/test/fixtures.ts'
import { drawnMark, markShape } from '@/test/marks.tsx'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { IssuesPanel } from './IssuesPanel.tsx'

const tokenLeak: Issue = {
  key: 'PROJ-1',
  tracker: 'jira',
  summary: 'Fix the token leak',
  status: 'In Progress',
  status_category: 'indeterminate',
  type: 'Bug',
  priority: 'High',
}

const setupDocs: Issue = {
  key: 'PROJ-2',
  tracker: 'jira',
  summary: 'Write the setup docs',
  status: 'To Do',
  status_category: 'new',
  type: 'Task',
}

// streamIssues puts a stream frame listing the issues on screen, with the
// trackers it says could not be read.
function streamIssues(issues: Issue[], unavailable: string[] = []) {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ issues: { total: issues.length, next: '', unavailable, issues } }),
  })
}

function withIssues() {
  streamIssues([tokenLeak, setupDocs])
}

// serveTokenLeak answers getIssue for PROJ-1 with its full detail, and any
// other issue with a 404. It returns the paths read, in order, so a test can
// count the reads.
function serveTokenLeak(): string[] {
  const paths: string[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn((request: Request) => {
      const path = new URL(request.url).pathname
      paths.push(path)
      if (path !== '/api/issues/PROJ-1') {
        return Promise.resolve(Response.json({ detail: 'no such issue' }, { status: 404 }))
      }

      return Promise.resolve(
        Response.json({
          ...tokenLeak,
          reporter: 'Ana Lopez',
          description: 'Tokens reach the request log.',
          comments: [{ author: 'Ana Lopez', body: "Repro'd.", created: '2026-09-20T10:00:00Z' }],
          comment_total: 1,
          url: '',
        }),
      )
    }),
  )

  return paths
}

// readsOf is how many of the paths read are PROJ-1's detail.
function readsOf(paths: string[]): number {
  return paths.filter((path) => path === '/api/issues/PROJ-1').length
}

test('lists the issues in the view', () => {
  // Arrange
  withIssues()

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  expect(screen.getByText('Fix the token leak')).toBeTruthy()
  expect(screen.getByText('Write the setup docs')).toBeTruthy()
})

test('shows an issue detail when it is selected', async () => {
  // Arrange
  const user = userEvent.setup()
  withIssues()
  renderWithClient(<IssuesPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: /fix the token leak/i }))

  // Assert
  expect(screen.getByRole('heading', { level: 2, name: /fix the token leak/i })).toBeTruthy()
})

test('reads the selected issue in full', async () => {
  // Arrange
  const user = userEvent.setup()
  withIssues()
  serveTokenLeak()
  renderWithClient(<IssuesPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: /fix the token leak/i }))

  // Assert
  expect(await screen.findByText('Tokens reach the request log.')).toBeTruthy()
  expect(screen.getByRole('list', { name: /comments/i }).textContent).toMatch(/repro'd/i)
})

test('keeps the detail heading current with the stream after the full read lands', async () => {
  // Arrange
  const user = userEvent.setup()
  withIssues()
  serveTokenLeak()
  renderWithClient(<IssuesPanel />)
  await user.click(screen.getByRole('button', { name: /fix the token leak/i }))
  await screen.findByText('Tokens reach the request log.')

  // Act
  // PROJ-1 is moved to Done elsewhere, and the stream's next frame carries it.
  act(() => {
    streamIssues([{ ...tokenLeak, status: 'Done', status_category: 'done' }, setupDocs])
  })

  // Assert
  expect(within(screen.getByRole('article')).getByText('Done')).toBeTruthy()
})

test('keeps the selected issue open when the stream drops it', async () => {
  // Arrange
  const user = userEvent.setup()
  withIssues()
  renderWithClient(<IssuesPanel />)
  await user.click(screen.getByRole('button', { name: /fix the token leak/i }))

  // Act
  // PROJ-1 leaves the view elsewhere; the next frame lists only PROJ-2.
  act(() => {
    streamIssues([setupDocs])
  })

  // Assert
  expect(screen.getByRole('heading', { level: 2, name: 'PROJ-1' })).toBeTruthy()
})

test('reads an issue again when it is reopened after 30 seconds', async () => {
  // Arrange
  // Only the clock is faked: staleness is judged from Date.now().
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date('2026-09-23T12:00:00Z'))
  const user = userEvent.setup()
  withIssues()
  const paths = serveTokenLeak()
  renderWithClient(<IssuesPanel />)
  await user.click(screen.getByRole('button', { name: /fix the token leak/i }))
  await screen.findByText('Tokens reach the request log.')
  await user.click(screen.getByRole('button', { name: /write the setup docs/i }))
  vi.setSystemTime(new Date('2026-09-23T12:00:31Z'))

  // Act
  await user.click(screen.getByRole('button', { name: /fix the token leak/i }))

  // Assert
  await waitFor(() => {
    expect(readsOf(paths)).toBe(2)
  })
})

test('does not read an issue again when it is reopened within 30 seconds', async () => {
  // Arrange
  const user = userEvent.setup()
  withIssues()
  const paths = serveTokenLeak()
  renderWithClient(<IssuesPanel />)
  await user.click(screen.getByRole('button', { name: /fix the token leak/i }))
  await screen.findByText('Tokens reach the request log.')
  await user.click(screen.getByRole('button', { name: /write the setup docs/i }))

  // Act
  await user.click(screen.getByRole('button', { name: /fix the token leak/i }))

  // Assert
  // Let a read the reopening might start reach fetch before counting.
  await act(
    () =>
      new Promise((resolve) => {
        setTimeout(resolve, 0)
      }),
  )
  expect(readsOf(paths)).toBe(1)
})

test('shows the work story for the selected issue', async () => {
  // Arrange
  const user = userEvent.setup()
  withIssues()
  renderWithClient(<IssuesPanel />)

  // Act
  // PROJ-2 has no priority, so its meta line omits the priority clause.
  await user.click(screen.getByRole('button', { name: /write the setup docs/i }))

  // Assert
  expect(screen.getByRole('heading', { name: /work story/i })).toBeTruthy()
  expect(screen.getByRole('button', { name: /^Slack/ })).toBeTruthy()
})

test("the issue's detail shows its tasks between its work story and its description", async () => {
  // Arrange
  const user = userEvent.setup()
  withIssues()
  serveTokenLeak()
  renderWithClient(<IssuesPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: /fix the token leak/i }))

  // Assert
  await screen.findByText('Tokens reach the request log.')
  expect(
    screen.getAllByRole('heading', { level: 3 }).map((heading) => heading.textContent),
  ).toEqual(['Work story', 'Tasks', 'Description', 'Comments'])
})

test('marks only the issues a local branch names as in flight', () => {
  // Arrange
  // Two issues, but only PROJ-1 has a local branch, so only it is in flight.
  withIssues()
  useSnapshotStore.setState((state) => ({
    snapshot: state.snapshot && {
      ...state.snapshot,
      branches: [makeTaskBranch({ name: 'fix/PROJ-1-leak', issue_key: 'PROJ-1', current: true })],
    },
  }))

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  expect(screen.getAllByText('in flight')).toHaveLength(1)
})

test("draws each issue's status category as its mark, beside the status", () => {
  // Arrange
  const shipped: Issue = { ...setupDocs, key: 'PROJ-3', status: 'Done', status_category: 'done' }
  streamIssues([tokenLeak, setupDocs, shipped])

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  expect(
    ['To Do', 'In Progress', 'Done'].map((status) => markShape(screen.getByText(status))),
  ).toEqual([drawnMark('not-started'), drawnMark('in-flight'), drawnMark('done')])
})

test('marks an issue in flight with the in-flight mark, beside the words', () => {
  // Arrange
  withIssues()
  useSnapshotStore.setState((state) => ({
    snapshot: state.snapshot && {
      ...state.snapshot,
      branches: [makeTaskBranch({ name: 'fix/PROJ-1-leak', issue_key: 'PROJ-1', current: true })],
    },
  }))

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  const words = screen.getByText('in flight')
  expect(markShape(words.parentElement ?? words)).toBe(drawnMark('in-flight'))
})

test('says when no issues match the view', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ issues: { total: 0, next: '', unavailable: [], issues: [] } }),
  })

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  expect(screen.getByText(/no issues match/i)).toBeTruthy()
})

// Twin of TestAForgeIssueIsListedByItsNumber.
test('a forge issue is listed by its number, as the forge writes it', () => {
  // Arrange
  streamIssues([
    { ...setupDocs, key: '57', tracker: 'forge', summary: 'Typo in the README' },
    tokenLeak,
  ])

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  expect(screen.getByRole('button', { name: /typo in the readme/i }).textContent).toContain('#57')
})

test('the filter offers forge issues without a mark, as their rows draw none', () => {
  // Arrange
  streamIssues([{ ...setupDocs, key: '57', tracker: 'forge', summary: 'Typo in the README' }])

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  const filter = screen.getByRole('group', { name: 'Filter' })
  expect(markShape(within(filter).getByRole('button', { name: 'forge issue 1' }))).toBe('')
  expect(markShape(within(filter).getByRole('button', { name: 'To Do 1' }))).toBe(
    drawnMark('not-started'),
  )
})

// Twin of TestTheIssuesListSaysWhenTheForgeCouldNotBeRead.
test("the list says which tracker's issues could not be read", () => {
  // Arrange
  streamIssues([tokenLeak], ["the forge's issues"])

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  expect(
    screen
      .getAllByRole('status')
      .some((line) => line.textContent === "Not read: the forge's issues."),
  ).toBe(true)
})
