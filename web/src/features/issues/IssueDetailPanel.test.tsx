import { act, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import type { IssueDetail } from '@/api/generated/types.gen.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { IssueDetailPanel } from './IssueDetailPanel.tsx'

// detailOf is a contract-valid issue detail, with the fields a case cares about
// overridden.
function detailOf(overrides: Partial<IssueDetail> = {}): IssueDetail {
  return {
    key: 'PROJ-1',
    summary: 'Fix the token leak',
    status: 'In Progress',
    status_category: 'indeterminate',
    type: 'Bug',
    priority: 'High',
    reporter: 'Ana Lopez',
    assignee: 'octocat',
    description: 'Tokens reach the request log.',
    comments: [
      { author: 'Sam Ortiz', body: "Repro'd on main.", created: '2026-09-20T10:00:00-06:00' },
    ],
    comment_total: 1,
    url: 'https://jira.example.com/browse/PROJ-1',
    ...overrides,
  }
}

// serveIssue answers getIssue with the detail, or with the problem and status
// when one is given.
function serveIssue(body: unknown, status = 200) {
  const fetch = vi.fn(() =>
    Promise.resolve(
      Response.json(body, {
        status,
        headers: {
          'Content-Type': status === 200 ? 'application/json' : 'application/problem+json',
        },
      }),
    ),
  )
  vi.stubGlobal('fetch', fetch)

  return fetch
}

test('shows the description and comments from getIssue', async () => {
  // Arrange
  serveIssue(detailOf())

  // Act
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)

  // Assert
  expect(await screen.findByText('Tokens reach the request log.')).toBeTruthy()
  const comments = screen.getByRole('list', { name: /comments/i })
  expect(comments.textContent).toMatch(/repro'd on main/i)
  expect(comments.textContent).toMatch(/2026/)
})

test('names who reported the issue and who it is assigned to', async () => {
  // Arrange
  serveIssue(detailOf())

  // Act
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)

  // Assert
  expect(await screen.findByText('octocat')).toBeTruthy()
  expect(screen.getByText('Ana Lopez')).toBeTruthy()
})

test('reads the detail of the issue it is given', async () => {
  // Arrange
  const fetch = serveIssue(detailOf({ key: 'PROJ-7' }))

  // Act
  renderWithClient(<IssueDetailPanel issueKey="PROJ-7" />)

  // Assert
  await screen.findByText('Tokens reach the request log.')
  const [request] = fetch.mock.calls[0] as unknown as [Request]
  expect(new URL(request.url).pathname).toBe('/api/issues/PROJ-7')
})

test('links the issue in Jira', async () => {
  // Arrange
  serveIssue(detailOf())

  // Act
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)

  // Assert
  const link = await screen.findByRole('link', { name: /open in jira/i })
  expect(link.getAttribute('href')).toBe('https://jira.example.com/browse/PROJ-1')
})

test('offers no Jira link when the tracker gives none', async () => {
  // Arrange
  serveIssue(detailOf({ url: '' }))

  // Act
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)

  // Assert
  await screen.findByText('Tokens reach the request log.')
  expect(screen.queryByRole('link', { name: /open in jira/i })).toBeNull()
})

test('says when the issue is unassigned, unreported, undescribed and uncommented', async () => {
  // Arrange
  serveIssue(
    detailOf({
      reporter: '',
      assignee: undefined,
      description: '',
      comments: [],
      comment_total: 0,
    }),
  )

  // Act
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)

  // Assert
  expect(await screen.findByText(/unassigned/i)).toBeTruthy()
  expect(screen.getByText('—')).toBeTruthy()
  expect(screen.getByText(/no description/i)).toBeTruthy()
  expect(screen.getByText(/no comments/i)).toBeTruthy()
})

test('says how many comments the tracker holds beyond those shown', async () => {
  // Arrange
  serveIssue(detailOf({ comment_total: 12 }))

  // Act
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)

  // Assert
  expect(await screen.findByText(/1 of 12 comments/i)).toBeTruthy()
})

test('leaves out a comment date the tracker could not give', async () => {
  // Arrange
  // The server sends the zero time when Jira's date was unreadable.
  serveIssue(
    detailOf({
      comments: [{ author: 'Ana Lopez', body: 'Undated.', created: '0001-01-01T00:00:00Z' }],
    }),
  )

  // Act
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)

  // Assert
  // Neither the date nor the separator before it is shown.
  const comments = await screen.findByRole('list', { name: /comments/i })
  expect(comments.textContent).toMatch(/undated/i)
  expect(comments.textContent).not.toMatch(/·/)
})

test('shows the reason when the issue cannot be read', async () => {
  // Arrange
  serveIssue(
    {
      type: 'https://jacob-delgado.github.io/workflow/docs/errors/#not-found',
      title: 'Not found',
      status: 404,
      detail: 'issue PROJ-1 was not found',
      code: 'not_found',
    },
    404,
  )

  // Act
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)

  // Assert
  const alert = await screen.findByRole('alert')
  expect(alert.textContent).toMatch(/PROJ-1 was not found/)
})

test('says it is reading the issue until the detail arrives', () => {
  // Arrange
  vi.stubGlobal(
    'fetch',
    vi.fn(() => new Promise(() => undefined)),
  )

  // Act
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)

  // Assert
  expect(screen.getByText(/reading PROJ-1/i)).toBeTruthy()
})

test('reads a mock issue under VITE_MOCK', async () => {
  // Arrange
  vi.stubEnv('VITE_MOCK', 'true')

  // Act
  renderWithClient(<IssueDetailPanel issueKey="PROJ-412" />)

  // Assert
  expect(
    await screen.findByRole('heading', { level: 2, name: /redact tokens before they reach/i }),
  ).toBeTruthy()
})

test('says to press Retry when a read is refused with no reason', async () => {
  // Arrange
  serveIssue({}, 500)

  // Act
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)

  // Assert
  const alert = await screen.findByRole('alert')
  expect(alert.textContent).toBe('PROJ-1 could not be read. Press Retry to try again.')
})

test('Retry reads a refused issue again', async () => {
  // Arrange
  const answers = [Response.json({}, { status: 500 }), Response.json(detailOf())]
  const requests = fakeApi({ '/api/issues/PROJ-1': () => answers.shift() })
  const user = userEvent.setup()
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)
  await screen.findByRole('alert')

  // Act
  await user.click(screen.getByRole('button', { name: 'Retry' }))

  // Assert
  // The Retry goes once the issue is read, so its focus follows to the issue's
  // heading rather than falling to the page.
  expect(await screen.findByText('Tokens reach the request log.')).toBeTruthy()
  const reads = requests.filter((request) => new URL(request.url).pathname === '/api/issues/PROJ-1')
  expect(reads).toHaveLength(2)
  expect(screen.queryByRole('button', { name: 'Retry' })).toBeNull()
  expect(document.activeElement).toBe(
    screen.getByRole('heading', { level: 2, name: detailOf().summary }),
  )
})

test('a retried issue read again later leaves focus where the user put it', async () => {
  // Arrange
  // Only the clock is faked: staleness is judged from Date.now(), and a stale
  // issue is read again when the page is shown again.
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date('2026-09-23T12:00:00Z'))
  const answers = [
    Response.json({}, { status: 500 }),
    Response.json(detailOf()),
    Response.json(detailOf({ description: 'Tokens reach the request log and the trace.' })),
  ]
  fakeApi({ '/api/issues/PROJ-1': () => answers.shift() })
  const user = userEvent.setup()
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)
  await user.click(await screen.findByRole('button', { name: 'Retry' }))
  await screen.findByText('Tokens reach the request log.')
  const jira = screen.getByRole('link', { name: /open in jira/i })
  jira.focus()
  vi.setSystemTime(new Date('2026-09-23T12:01:01Z'))

  // Act
  act(() => {
    window.dispatchEvent(new Event('visibilitychange'))
  })

  // Assert
  await screen.findByText('Tokens reach the request log and the trace.')
  expect(document.activeElement).toBe(jira)
})

test('Retry beside an issue already read hands focus to its heading', async () => {
  // Arrange
  // The issue was read, then read again once stale and refused, so the Retry
  // stands beside what the first read showed; the retried read answers the
  // same issue.
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date('2026-09-23T12:00:00Z'))
  const answers = [
    Response.json(detailOf()),
    Response.json({}, { status: 500 }),
    Response.json(detailOf()),
  ]
  fakeApi({ '/api/issues/PROJ-1': () => answers.shift() })
  const user = userEvent.setup()
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)
  await screen.findByText('Tokens reach the request log.')
  vi.setSystemTime(new Date('2026-09-23T12:01:01Z'))
  act(() => {
    window.dispatchEvent(new Event('visibilitychange'))
  })
  const retry = await screen.findByRole('button', { name: 'Retry' })

  // Act
  await user.click(retry)

  // Assert
  await waitFor(() => {
    expect(screen.queryByRole('alert')).toBeNull()
  })
  expect(document.activeElement).toBe(
    screen.getByRole('heading', { level: 2, name: detailOf().summary }),
  )
})

test('a Retry refused again beside an issue already read keeps its focus', async () => {
  // Arrange
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date('2026-09-23T12:00:00Z'))
  const answers = [
    Response.json(detailOf()),
    Response.json({}, { status: 500 }),
    Response.json({}, { status: 500 }),
  ]
  fakeApi({ '/api/issues/PROJ-1': () => answers.shift() })
  const user = userEvent.setup()
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)
  await screen.findByText('Tokens reach the request log.')
  vi.setSystemTime(new Date('2026-09-23T12:01:01Z'))
  act(() => {
    window.dispatchEvent(new Event('visibilitychange'))
  })
  const retry = await screen.findByRole('button', { name: 'Retry' })

  // Act
  await user.click(retry)

  // Assert
  await waitFor(() => {
    expect(answers).toHaveLength(0)
  })
  expect(await screen.findByRole('button', { name: 'Retry' })).toBe(retry)
  expect(document.activeElement).toBe(retry)
})
