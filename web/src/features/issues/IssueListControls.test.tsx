import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import App from '@/App.tsx'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { useUiStore } from '@/shell/uiStore.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { FakeEventSource } from '@/test/fakeEventSource.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { IssuesPanel } from './IssuesPanel.tsx'

const views = {
  views: [
    { name: 'Assigned to me', jql: 'assignee = currentUser()' },
    { name: 'Team bugs', jql: 'type = Bug' },
  ],
}

const oneIssue = makeSnapshot({
  issues: {
    total: 1,
    start_at: 0,
    issues: [
      {
        key: 'PROJ-1',
        summary: 'Fix the token leak',
        status: 'In Progress',
        status_category: 'indeterminate',
        type: 'Bug',
      },
    ],
  },
})

const aBug = makeSnapshot({
  issues: {
    total: 1,
    start_at: 0,
    issues: [
      {
        key: 'PROJ-9',
        summary: 'Crash on an empty config',
        status: 'To Do',
        status_category: 'new',
        type: 'Bug',
      },
    ],
  },
})

test('offers the listed views beside the list', async () => {
  // Arrange
  fakeApi({ '/api/views': views })
  useSnapshotStore.setState({ status: 'live', snapshot: oneIssue })

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  const select = await screen.findByRole('combobox', { name: /view/i })
  const options = within(select)
    .getAllByRole('option')
    .map((option) => option.textContent)
  expect(options).toEqual(['Assigned to me', 'Team bugs'])
})

test('keeps the view select beside an empty list', async () => {
  // Arrange
  fakeApi({ '/api/views': views })
  useSnapshotStore.setState({ status: 'live', snapshot: makeSnapshot() })

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  expect(await screen.findByRole('combobox', { name: /view/i })).toBeTruthy()
  expect(screen.getByText(/no issues match this view/i)).toBeTruthy()
})

test('choosing a view changes the stream query', async () => {
  // Arrange
  const user = userEvent.setup()
  fakeApi({ '/api/views': views })
  renderWithClient(<App />)
  act(() => {
    FakeEventSource.latest().emit('snapshot', JSON.stringify(oneIssue))
  })
  const select = await screen.findByRole('combobox', { name: /view/i })

  // Act
  await user.selectOptions(select, 'Team bugs')

  // Assert
  expect(FakeEventSource.latest().url).toBe('/api/events?view=Team+bugs')
})

test('goes back to the default view when the chosen one is no longer listed', async () => {
  // Arrange
  // The view was chosen, then removed from the configuration.
  fakeApi({ '/api/views': views })
  useUiStore.setState({ view: 'Removed view' })
  useSnapshotStore.setState({ status: 'live', snapshot: oneIssue })

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  await waitFor(() => {
    expect(useUiStore.getState().view).toBeNull()
  })
})

test('offers no view select when the views cannot be read', async () => {
  // Arrange
  const requests = fakeApi({})
  useSnapshotStore.setState({ status: 'live', snapshot: oneIssue })

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  await waitFor(() => {
    expect(requests.length).toBeGreaterThan(0)
  })
  expect(screen.queryByRole('combobox', { name: /view/i })).toBeNull()
  expect(screen.getByText('Fix the token leak')).toBeTruthy()
})

test('offers the mock views under VITE_MOCK', async () => {
  // Arrange
  vi.stubEnv('VITE_MOCK', 'true')
  useSnapshotStore.setState({ status: 'live', snapshot: oneIssue })

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  const select = await screen.findByRole('combobox', { name: /view/i })
  expect(within(select).getByRole('option', { name: 'Team bugs' })).toBeTruthy()
})

test('reads the chosen view before listing its issues', async () => {
  // Arrange
  const user = userEvent.setup()
  fakeApi({ '/api/views': views })
  renderWithClient(<App />)
  act(() => {
    FakeEventSource.latest().emit('snapshot', JSON.stringify(oneIssue))
  })
  const select = await screen.findByRole('combobox', { name: /view/i })

  // Act
  await user.selectOptions(select, 'Team bugs')

  // Assert
  // The last view's issues go until the chosen view's first frame lands, and
  // the select keeps focus.
  expect(screen.queryByText('Fix the token leak')).toBeNull()
  expect(screen.getByText(/reading the team bugs view/i)).toBeTruthy()
  expect(document.activeElement).toBe(select)
})

test("lists the chosen view's issues once its first frame lands", async () => {
  // Arrange
  const user = userEvent.setup()
  fakeApi({ '/api/views': views })
  renderWithClient(<App />)
  act(() => {
    FakeEventSource.latest().emit('snapshot', JSON.stringify(oneIssue))
  })
  await user.selectOptions(await screen.findByRole('combobox', { name: /view/i }), 'Team bugs')

  // Act
  act(() => {
    FakeEventSource.latest().emit('snapshot', JSON.stringify(aBug))
  })

  // Assert
  expect(screen.getByText('Crash on an empty config')).toBeTruthy()
})

test('goes back to the default view when the server refuses the chosen one', async () => {
  // Arrange
  // The server restarts without the chosen view: its views no longer list it,
  // and the stream's reconnect is refused.
  const user = userEvent.setup()
  const routes: Record<string, unknown> = { '/api/views': views }
  fakeApi(routes)
  renderWithClient(<App />)
  act(() => {
    FakeEventSource.latest().emit('snapshot', JSON.stringify(oneIssue))
  })
  const select = await screen.findByRole('combobox', { name: /view/i })
  await user.selectOptions(select, 'Team bugs')
  routes['/api/views'] = { views: [views.views[0]] }

  // Act
  act(() => {
    FakeEventSource.latest().refuse()
  })

  // Assert
  expect(FakeEventSource.latest().url).toBe('/api/events')
  await waitFor(() => {
    expect(within(select).queryByRole('option', { name: 'Team bugs' })).toBeNull()
  })
})

test('goes back to the default view on another section too', async () => {
  // Arrange
  // The view select is not on screen, so only the stream can notice.
  const user = userEvent.setup()
  fakeApi({ '/api/views': views })
  renderWithClient(<App />)
  act(() => {
    FakeEventSource.latest().emit('snapshot', JSON.stringify(oneIssue))
  })
  await user.selectOptions(await screen.findByRole('combobox', { name: /view/i }), 'Team bugs')
  await user.click(screen.getByRole('button', { name: /branch/i }))

  // Act
  act(() => {
    FakeEventSource.latest().refuse()
  })

  // Assert
  expect(FakeEventSource.latest().url).toBe('/api/events')
})
