import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import type { Issue, IssuesPage } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { useUiStore } from '@/shell/uiStore.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { IssuesPanel } from './IssuesPanel.tsx'

// issue is a slim issue with the given key, summarized after it.
function issue(key: string): Issue {
  return {
    key,
    summary: `Summary of ${key}`,
    status: 'To Do',
    status_category: 'new',
    type: 'Task',
  }
}

// streamFirstPage puts the page the stream pushes on screen: the first issues
// of the chosen view, which holds total in all.
function streamFirstPage(keys: string[], total: number) {
  useSnapshotStore.setState({
    status: 'live',
    view: useUiStore.getState().view,
    snapshot: makeSnapshot({ issues: { issues: keys.map(issue), total, start_at: 0 } }),
  })
}

// servePages answers listIssues with the page that starts where the request
// asks, cut from keys, and answers the views list.
function servePages(keys: string[], pageSize: number) {
  return fakeApi({
    '/api/views': {
      views: [
        { name: 'Assigned to me', jql: 'assignee = currentUser()' },
        { name: 'Team bugs', jql: 'type = Bug' },
      ],
    },
    '/api/issues': (url: URL): IssuesPage => {
      const startAt = Number(url.searchParams.get('start_at'))

      return {
        issues: keys.slice(startAt, startAt + pageSize).map(issue),
        total: keys.length,
        start_at: startAt,
      }
    },
  })
}

// issueReads is the listIssues requests among those the page made.
function issueReads(requests: Request[]): URL[] {
  return requests
    .map((request) => new URL(request.url))
    .filter((url) => url.pathname === '/api/issues')
}

// statusSaying is the status region whose text is exactly the given text.
function statusSaying(text: string): HTMLElement | undefined {
  return screen.getAllByRole('status').find((region) => region.textContent === text)
}

// listedKeys is the issue keys the list shows, in order.
function listedKeys(): string[] {
  const list = screen.getByRole('list', { name: 'Issues' })

  return within(list)
    .getAllByRole('listitem')
    .map((item) => /PROJ-\d+/.exec(item.textContent)?.[0] ?? '')
}

test('load more reads the page after the loaded issues and appends it', async () => {
  // Arrange
  const user = userEvent.setup()
  const requests = servePages(['PROJ-1', 'PROJ-2', 'PROJ-3', 'PROJ-4'], 2)
  streamFirstPage(['PROJ-1', 'PROJ-2'], 4)
  renderWithClient(<IssuesPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: /load more/i }))

  // Assert
  expect(await screen.findByText('Summary of PROJ-4')).toBeTruthy()
  expect(listedKeys()).toEqual(['PROJ-1', 'PROJ-2', 'PROJ-3', 'PROJ-4'])
  expect(issueReads(requests).map((url) => url.searchParams.get('start_at'))).toEqual(['2'])
})

test('load more goes on from the last page it read', async () => {
  // Arrange
  const user = userEvent.setup()
  const requests = servePages(['PROJ-1', 'PROJ-2', 'PROJ-3', 'PROJ-4', 'PROJ-5'], 2)
  streamFirstPage(['PROJ-1', 'PROJ-2'], 5)
  renderWithClient(<IssuesPanel />)
  await user.click(screen.getByRole('button', { name: /load more/i }))
  await screen.findByText('Summary of PROJ-4')

  // Act
  await user.click(screen.getByRole('button', { name: /load more/i }))

  // Assert
  expect(await screen.findByText('Summary of PROJ-5')).toBeTruthy()
  expect(issueReads(requests).map((url) => url.searchParams.get('start_at'))).toEqual(['2', '4'])
  expect(screen.queryByRole('button', { name: /load more/i })).toBeNull()
})

test('offers no load more when the stream carries every issue', () => {
  // Arrange
  servePages(['PROJ-1', 'PROJ-2'], 2)
  streamFirstPage(['PROJ-1', 'PROJ-2'], 2)

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  expect(screen.queryByRole('button', { name: /load more/i })).toBeNull()
})

test('says all are loaded when the stream carries every issue', () => {
  // Arrange
  servePages(['PROJ-1', 'PROJ-2'], 2)
  streamFirstPage(['PROJ-1', 'PROJ-2'], 2)

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  expect(statusSaying('All 2 loaded.')).toBeDefined()
})

test('load more reads the chosen view', async () => {
  // Arrange
  const user = userEvent.setup()
  const requests = servePages(['PROJ-1', 'PROJ-2', 'PROJ-3'], 2)
  useUiStore.setState({ view: 'Team bugs' })
  streamFirstPage(['PROJ-1', 'PROJ-2'], 3)
  renderWithClient(<IssuesPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: /load more/i }))

  // Assert
  await screen.findByText('Summary of PROJ-3')
  expect(issueReads(requests)[0]?.searchParams.get('view')).toBe('Team bugs')
})

test('lists an issue once when it is in both the stream page and a loaded page', async () => {
  // Arrange
  // The stream re-reads its page every few seconds while a loaded page stays as
  // read, so an issue can move from one into the other.
  const user = userEvent.setup()
  servePages(['PROJ-1', 'PROJ-2', 'PROJ-2', 'PROJ-3'], 2)
  streamFirstPage(['PROJ-1', 'PROJ-2'], 4)
  renderWithClient(<IssuesPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: /load more/i }))

  // Assert
  await screen.findByText('Summary of PROJ-3')
  expect(listedKeys()).toEqual(['PROJ-1', 'PROJ-2', 'PROJ-3'])
})

test('choosing another view drops the pages loaded for the last one', async () => {
  // Arrange
  const user = userEvent.setup()
  servePages(['PROJ-1', 'PROJ-2', 'PROJ-3'], 2)
  streamFirstPage(['PROJ-1', 'PROJ-2'], 3)
  renderWithClient(<IssuesPanel />)
  await user.click(screen.getByRole('button', { name: /load more/i }))
  await screen.findByText('Summary of PROJ-3')

  act(() => {
    useUiStore.getState().setView('Team bugs')
  })

  // Act
  act(() => {
    streamFirstPage(['PROJ-1', 'PROJ-2'], 3)
  })

  // Assert
  expect(listedKeys()).toEqual(['PROJ-1', 'PROJ-2'])
})

test("offers no load more between a view switch and the new view's first frame", () => {
  // Arrange
  servePages(['PROJ-1', 'PROJ-2', 'PROJ-3'], 2)
  streamFirstPage(['PROJ-1', 'PROJ-2'], 3)
  renderWithClient(<IssuesPanel />)

  // Act
  act(() => {
    useUiStore.getState().setView('Team bugs')
  })

  // Assert
  // The last view's page would read the new view from its own offset.
  expect(screen.queryByRole('button', { name: /load more/i })).toBeNull()
})

test('lists an issue once when two loaded pages both hold it', async () => {
  // Arrange
  // An issue added to the top of the view between two reads pushes the last
  // issue of one loaded page onto the next.
  const user = userEvent.setup()
  servePages(['PROJ-1', 'PROJ-2', 'PROJ-3', 'PROJ-4', 'PROJ-4', 'PROJ-5'], 2)
  streamFirstPage(['PROJ-1', 'PROJ-2'], 6)
  renderWithClient(<IssuesPanel />)
  await user.click(screen.getByRole('button', { name: /load more/i }))
  await screen.findByText('Summary of PROJ-4')

  // Act
  await user.click(screen.getByRole('button', { name: /load more/i }))

  // Assert
  await screen.findByText('Summary of PROJ-5')
  expect(listedKeys()).toEqual(['PROJ-1', 'PROJ-2', 'PROJ-3', 'PROJ-4', 'PROJ-5'])
})

test('holds load more while a page is being read', async () => {
  // Arrange
  // The second page never answers, so it is still in flight when pressed again.
  const user = userEvent.setup()
  const starts: string[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn((request: Request) => {
      const url = new URL(request.url)
      const startAt = url.searchParams.get('start_at') ?? ''
      starts.push(startAt)
      if (startAt !== '2') {
        return new Promise<Response>(() => undefined)
      }

      return Promise.resolve(Response.json({ issues: [issue('PROJ-3')], total: 5, start_at: 2 }))
    }),
  )
  streamFirstPage(['PROJ-1', 'PROJ-2'], 5)
  renderWithClient(<IssuesPanel />)
  await user.click(screen.getByRole('button', { name: /load more/i }))
  await screen.findByText('Summary of PROJ-3')
  await user.click(screen.getByRole('button', { name: /load more/i }))
  const button = await screen.findByRole('button', { name: /loading more/i })

  // Act
  await user.click(button)

  // Assert
  // The button keeps focus rather than going disabled, and the second press
  // waits on the read in flight instead of starting it over.
  expect(button.getAttribute('aria-disabled')).toBe('true')
  expect(document.activeElement).toBe(button)
  expect(starts.filter((start) => start === '3')).toHaveLength(1)
})

test("says how many of the view's issues are loaded", async () => {
  // Arrange
  const user = userEvent.setup()
  servePages(['PROJ-1', 'PROJ-2', 'PROJ-3', 'PROJ-4', 'PROJ-5'], 2)
  streamFirstPage(['PROJ-1', 'PROJ-2'], 5)
  renderWithClient(<IssuesPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: /load more/i }))

  // Assert
  await waitFor(() => {
    expect(statusSaying('4 of 5 loaded')).toBeDefined()
  })
})

test('hands focus to the first issue the last page adds', async () => {
  // Arrange
  const user = userEvent.setup()
  servePages(['PROJ-1', 'PROJ-2', 'PROJ-3', 'PROJ-4', 'PROJ-5'], 2)
  streamFirstPage(['PROJ-1', 'PROJ-2'], 5)
  renderWithClient(<IssuesPanel />)
  await user.click(screen.getByRole('button', { name: /load more/i }))
  await screen.findByText('Summary of PROJ-4')

  // Act
  // The last page: the button that has focus goes once it lands.
  await user.click(screen.getByRole('button', { name: /load more/i }))

  // Assert
  await screen.findByText('Summary of PROJ-5')
  await waitFor(() => {
    expect(document.activeElement).toBe(screen.getByRole('button', { name: /PROJ-5/ }))
  })
  expect(statusSaying('All 5 loaded.')).toBeDefined()
})

test('hands focus to the count when the filter hides every issue the page adds', async () => {
  // Arrange
  const user = userEvent.setup()
  servePages(['PROJ-1', 'PROJ-2', 'PROJ-3', 'PROJ-4'], 2)
  streamFirstPage(['PROJ-1', 'PROJ-2'], 4)
  renderWithClient(<IssuesPanel />)
  await user.type(screen.getByRole('searchbox', { name: /filter/i }), 'proj-1')

  // Act
  await user.click(screen.getByRole('button', { name: /load more/i }))

  // Assert
  await waitFor(() => {
    expect(document.activeElement).toBe(statusSaying('All 4 loaded.'))
  })
})

test('shows the reason when more issues cannot be loaded', async () => {
  // Arrange
  const user = userEvent.setup()
  fakeApi({})
  streamFirstPage(['PROJ-1', 'PROJ-2'], 3)
  renderWithClient(<IssuesPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: /load more/i }))

  // Assert
  const alert = await screen.findByRole('alert')
  expect(alert.textContent).toMatch(/no such route/i)
})

test('stops offering more when a page comes back empty', async () => {
  // Arrange
  // The view shrank after the stream's page was read: its total still promises
  // more, but the next page holds none.
  const user = userEvent.setup()
  const requests = fakeApi({ '/api/issues': { issues: [], total: 3, start_at: 2 } })
  streamFirstPage(['PROJ-1', 'PROJ-2'], 3)
  renderWithClient(<IssuesPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: /load more/i }))

  // Assert
  await waitFor(() => {
    expect(screen.queryByRole('button', { name: /load more/i })).toBeNull()
  })
  expect(issueReads(requests)).toHaveLength(1)
})

test('says to press Load more again when a page is refused with no reason', async () => {
  // Arrange
  const user = userEvent.setup()
  fakeApi({ '/api/issues': Response.json({}, { status: 500 }) })
  streamFirstPage(['PROJ-1', 'PROJ-2'], 3)
  renderWithClient(<IssuesPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: /load more/i }))

  // Assert
  const alert = await screen.findByRole('alert')
  expect(alert.textContent).toBe('More issues could not be loaded. Press Load more to try again.')
})
