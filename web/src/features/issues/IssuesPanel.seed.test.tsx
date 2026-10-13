import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import type { Issue, IssuesPage } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { useUiStore } from '@/shell/uiStore.ts'
import { makeSnapshot, makeTaskBranch } from '@/test/fixtures.ts'
import { drawnMark, markShape } from '@/test/marks.tsx'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { IssuesPanel } from './IssuesPanel.tsx'

const keptIssue: Issue = {
  key: 'PROJ-1',
  tracker: 'jira',
  summary: 'Kept from last time',
  status: 'To Do',
  status_category: 'new',
  type: 'Task',
}

const liveIssue: Issue = { ...keptIssue, key: 'PROJ-2', summary: 'Found just now' }

// pageOf is a first page holding issues, all there is of the view.
function pageOf(issues: Issue[]): IssuesPage {
  return { issues, total: issues.length, start_at: 0, unavailable: [] }
}

// seeded puts the stream's seed for view on screen, before any snapshot.
function seeded(issues: Issue[], view: string | null = null) {
  useSnapshotStore.setState({ status: 'live', seed: { view, page: pageOf(issues) } })
}

test('lists the kept issues before the first update, saying Jira is being read again', () => {
  // Arrange
  seeded([keptIssue])

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  expect(screen.getByRole('button', { name: /kept from last time/i })).toBeTruthy()
  expect(screen.getByText('Reading Jira… these are the issues it listed last time.')).toBeTruthy()
  expect(screen.queryByRole('button', { name: /load more/i })).toBeNull()
  expect(screen.queryByText(/loaded\./)).toBeNull()
})

test('a view kept with no issues says none match while Jira is read again', () => {
  // Arrange
  seeded([])

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  expect(screen.getByText('No issues match this view.')).toBeTruthy()
  expect(screen.getByText(/Reading Jira…/)).toBeTruthy()
})

test("the view's first snapshot takes the kept list's place", () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    seed: null,
    snapshot: makeSnapshot({ issues: pageOf([liveIssue]) }),
  })

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  expect(screen.getByRole('button', { name: /found just now/i })).toBeTruthy()
  expect(screen.queryByText(/Reading Jira…/)).toBeNull()
})

test("a switch of view lists the view's kept issues, marked from the last snapshot", () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    view: null,
    snapshot: makeSnapshot({
      branches: [makeTaskBranch({ name: 'fix/PROJ-1-kept', issue_key: 'PROJ-1', current: true })],
    }),
    seed: { view: 'Bugs', page: pageOf([keptIssue]) },
  })
  useUiStore.setState({ view: 'Bugs' })

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  const words = screen.getByText('in flight')
  expect(markShape(words.parentElement ?? words)).toBe(drawnMark('in-flight'))
  expect(screen.queryByText('Reading the Bugs view…')).toBeNull()
})

test('an issue opened from the kept list reads its detail, its work story waiting on the first update', async () => {
  // Arrange
  const user = userEvent.setup()
  vi.stubGlobal(
    'fetch',
    vi.fn(() =>
      Promise.resolve(
        Response.json({
          ...keptIssue,
          reporter: 'Ana Lopez',
          description: '',
          comments: [],
          comment_total: 0,
          url: '',
        }),
      ),
    ),
  )
  useSnapshotStore.setState({
    status: 'connecting',
    seed: { view: null, page: pageOf([keptIssue]) },
  })
  renderWithClient(<IssuesPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: /kept from last time/i }))

  // Assert
  expect(
    await screen.findByRole('heading', { level: 2, name: /kept from last time/i }),
  ).toBeTruthy()
  expect(screen.getByText('Connecting to workflow…')).toBeTruthy()
})
