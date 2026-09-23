import { screen } from '@testing-library/react'
import { vi } from 'vitest'
import type { IssueDetail } from '@/api/generated/types.gen.ts'
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
