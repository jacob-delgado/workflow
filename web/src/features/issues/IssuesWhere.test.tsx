import { act, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { makeSnapshot, makeTask } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { IssuesPanel } from './IssuesPanel.tsx'

const views = {
  views: [
    { name: 'Assigned to me', jql: 'assignee = currentUser()' },
    { name: 'Team bugs', jql: 'type = Bug' },
  ],
}

const snapshot = makeSnapshot({
  issues: {
    total: 3,
    start_at: 0,
    unavailable: [],
    issues: [
      {
        key: 'PROJ-501',
        tracker: 'jira',
        summary: 'Triage the crash',
        status: 'Intake',
        status_category: 'new',
        type: 'Bug',
      },
      {
        key: 'PROJ-502',
        tracker: 'jira',
        summary: 'Patch the leak',
        status: 'Fixing',
        status_category: 'indeterminate',
        type: 'Bug',
      },
      {
        key: 'PROJ-504',
        tracker: 'jira',
        summary: 'Speed up search',
        status: 'In development',
        status_category: 'indeterminate',
        type: 'Task',
      },
    ],
  },
  branches: [{ name: 'fix/PROJ-504-speed-up-search', issue_key: 'PROJ-504', current: false }],
  tasks: {
    available: true,
    reason: '',
    linked: [makeTask({ issue_key: 'PROJ-502', start: '2026-09-30T09:00:00Z' })],
  },
})

// listedKeys is the keys the issue list shows, in order.
function listedKeys(): string[] {
  const list = screen.getByRole('list', { name: 'Issues' })

  return within(list)
    .getAllByRole('listitem')
    .map((item) => (/PROJ-\d+/.exec(item.textContent) ?? [''])[0])
}

// where is the Where group's button for a place.
function where(name: RegExp): HTMLElement {
  return within(screen.getByRole('group', { name: 'Filter' })).getByRole('button', { name })
}

function renderPanel() {
  fakeApi({ '/api/views': views })
  useSnapshotStore.setState({ status: 'live', snapshot })
  renderWithClient(<IssuesPanel />)
}

test('offers each place the loaded issues are in, with its count', () => {
  // Arrange
  renderPanel()

  // Act
  const group = screen.getByRole('group', { name: 'Filter' })

  // Assert
  expect(
    within(group)
      .getAllByRole('button')
      .map((button) => button.textContent),
  ).toEqual(['Intake 1', 'Fixing 1', 'In development 1', 'in flight 1', 'task active 1'])
})

test('pressing a status narrows the list to it', async () => {
  // Arrange
  const user = userEvent.setup()
  renderPanel()

  // Act
  await user.click(where(/^Fixing/))

  // Assert
  expect(listedKeys()).toEqual(['PROJ-502'])
  expect(where(/^Fixing/).getAttribute('aria-pressed')).toBe('true')
  expect(screen.getAllByRole('status').map((region) => region.textContent)).toContain(
    '1 of 3 loaded issues matches.',
  )
})

test('a status and a mark narrow the list together', async () => {
  // Arrange
  const user = userEvent.setup()
  renderPanel()

  // Act
  await user.click(where(/^In development/))
  await user.click(where(/^in flight/))

  // Assert
  expect(listedKeys()).toEqual(['PROJ-504'])
})

test('a place and the filter narrow the list together', async () => {
  // Arrange
  const user = userEvent.setup()
  renderPanel()
  await user.click(where(/^in flight/))

  // Act
  await user.type(screen.getByRole('searchbox', { name: 'Search' }), 'leak')

  // Assert
  expect(screen.getAllByRole('status').map((region) => region.textContent)).toContain(
    'No loaded issue matches the filter.',
  )
})

test('pressing a place again widens the list back', async () => {
  // Arrange
  const user = userEvent.setup()
  renderPanel()
  await user.click(where(/^Intake/))

  // Act
  await user.click(where(/^Intake/))

  // Assert
  expect(listedKeys()).toEqual(['PROJ-501', 'PROJ-502', 'PROJ-504'])
  expect(where(/^Intake/).getAttribute('aria-pressed')).toBe('false')
})

// Twin of TestSwitchingViewDropsThePlaces.
test('switching view drops the places', async () => {
  // Arrange
  const user = userEvent.setup()
  renderPanel()
  await user.click(where(/^Intake/))
  await user.selectOptions(await screen.findByRole('combobox', { name: /view/i }), 'Team bugs')

  // Act
  act(() => {
    useSnapshotStore.setState({ status: 'live', snapshot, view: 'Team bugs' })
  })

  // Assert
  expect(listedKeys()).toEqual(['PROJ-501', 'PROJ-502', 'PROJ-504'])
})

test('switching view clears the search', async () => {
  // Arrange
  const user = userEvent.setup()
  renderPanel()
  await user.type(screen.getByRole('searchbox', { name: 'Search' }), 'proj-12')
  await user.selectOptions(await screen.findByRole('combobox', { name: /view/i }), 'Team bugs')

  // Act
  act(() => {
    useSnapshotStore.setState({ status: 'live', snapshot, view: 'Team bugs' })
  })

  // Assert
  expect(screen.getByRole<HTMLInputElement>('searchbox', { name: 'Search' }).value).toBe('')
  expect(listedKeys()).toEqual(['PROJ-501', 'PROJ-502', 'PROJ-504'])
})

test('unpicking a place no issue is in keeps focus in the Where group', async () => {
  // Arrange
  const user = userEvent.setup()
  renderPanel()
  await user.click(where(/^Intake/))
  act(() => {
    useSnapshotStore.setState({
      snapshot: {
        ...snapshot,
        issues: { ...snapshot.issues, issues: snapshot.issues.issues.slice(1) },
      },
    })
  })

  // Act
  await user.click(where(/^Intake 0/))

  // Assert
  expect(screen.getByRole('group', { name: 'Filter' }).contains(document.activeElement)).toBe(true)
})
