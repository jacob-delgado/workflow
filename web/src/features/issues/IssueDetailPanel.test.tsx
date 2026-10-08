import { screen } from '@testing-library/react'
import { vi } from 'vitest'
import type { Health } from '@/api/generated/types.gen.ts'
import { useHealthStore } from '@/api/health.ts'
import { makeHealth, makeIssueDetail } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { IssueDetailPanel } from './IssueDetailPanel.tsx'

// An issue's detail as the panel draws it: its facts, description, comments
// and link, and what it says while the issue is read or when it cannot be.
// Try again is in IssueDetailPanel.retry.test.tsx.

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
  serveIssue(makeIssueDetail())

  // Act
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)

  // Assert
  expect(await screen.findByText('Tokens reach the request log.')).toBeTruthy()
  const comments = screen.getByRole('list', { name: /comments/i })
  expect(comments.textContent).toMatch(/repro'd on main/i)
  expect(comments.textContent).toMatch(/2026/)
})

test('gives the issue’s type and priority each an element of its own', async () => {
  // Arrange
  serveIssue(makeIssueDetail())

  // Act
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)

  // Assert
  expect((await screen.findByText('High priority')).textContent).toBe('High priority')
  expect(screen.getByText('Bug').textContent).toBe('Bug')
})

test('names who reported the issue and who it is assigned to', async () => {
  // Arrange
  serveIssue(makeIssueDetail())

  // Act
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)

  // Assert
  expect(await screen.findByText('octocat')).toBeTruthy()
  expect(screen.getByText('Ana Lopez')).toBeTruthy()
})

test('reads the detail of the issue it is given', async () => {
  // Arrange
  const fetch = serveIssue(makeIssueDetail({ key: 'PROJ-7' }))

  // Act
  renderWithClient(<IssueDetailPanel issueKey="PROJ-7" />)

  // Assert
  await screen.findByText('Tokens reach the request log.')
  const [request] = fetch.mock.calls[0] as unknown as [Request]
  expect(new URL(request.url).pathname).toBe('/api/issues/PROJ-7')
})

test('links the issue in Jira', async () => {
  // Arrange
  serveIssue(makeIssueDetail())

  // Act
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)

  // Assert
  const link = await screen.findByRole('link', { name: /open in jira/i })
  expect(link.getAttribute('href')).toBe('https://jira.example.com/browse/PROJ-1')
})

test('offers no Jira link when the tracker gives none', async () => {
  // Arrange
  serveIssue(makeIssueDetail({ url: '' }))

  // Act
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)

  // Assert
  await screen.findByText('Tokens reach the request log.')
  expect(screen.queryByRole('link', { name: /open in jira/i })).toBeNull()
})

test('says when the issue is unassigned, unreported, undescribed and uncommented', async () => {
  // Arrange
  serveIssue(
    makeIssueDetail({
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
  serveIssue(makeIssueDetail({ comment_total: 12 }))

  // Act
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)

  // Assert
  expect(await screen.findByText(/1 of 12 comments/i)).toBeTruthy()
})

test('leaves out a comment date the tracker could not give', async () => {
  // Arrange
  // The server leaves the date out when Jira's could not be read.
  serveIssue(makeIssueDetail({ comments: [{ author: 'Ana Lopez', body: 'Undated.' }] }))

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

test('says to press Try again when a read is refused with no reason', async () => {
  // Arrange
  serveIssue({}, 500)

  // Act
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)

  // Assert
  const alert = await screen.findByRole('alert')
  expect(alert.textContent).toBe('PROJ-1 could not be read. Press Try again.')
})

test('a forge issue opens on its forge, not in Jira', async () => {
  // Arrange
  useHealthStore.setState({ health: makeHealth() })
  serveIssue(
    makeIssueDetail({ key: '57', tracker: 'forge', url: 'https://github.com/acme/oss/issues/57' }),
  )

  // Act
  renderWithClient(<IssueDetailPanel issueKey="57" />)

  // Assert
  const link = await screen.findByRole('link', { name: /open in github/i })
  expect(link.getAttribute('href')).toBe('https://github.com/acme/oss/issues/57')
  expect(screen.queryByRole('link', { name: /open in jira/i })).toBeNull()
})

// The forge is named by which one the server says it is, never by its words
// for a proposed change, which are only words to show.
test.each<{ kind: Health['forge_kind']; named: string }>([
  { kind: 'github', named: 'GitHub' },
  { kind: 'gitlab', named: 'GitLab' },
  { kind: 'unknown', named: 'the forge' },
])('a forge issue on $kind opens in $named', async ({ kind, named }) => {
  // Arrange
  useHealthStore.setState({ health: makeHealth({ forge_kind: kind }) })
  serveIssue(
    makeIssueDetail({ key: '57', tracker: 'forge', url: 'https://forge.example/issues/57' }),
  )

  // Act
  renderWithClient(<IssueDetailPanel issueKey="57" />)

  // Assert
  expect(await screen.findByRole('link', { name: new RegExp(`^Open in ${named}`) })).toBeTruthy()
})

test('before the forge is known, a forge issue opens in the forge, named by no guess', async () => {
  // Arrange
  serveIssue(
    makeIssueDetail({
      key: '57',
      tracker: 'forge',
      url: 'https://gitlab.com/acme/oss/-/issues/57',
    }),
  )

  // Act
  renderWithClient(<IssueDetailPanel issueKey="57" />)

  // Assert
  const link = await screen.findByRole('link', { name: /open in the forge/i })
  expect(link.getAttribute('href')).toBe('https://gitlab.com/acme/oss/-/issues/57')
  expect(screen.queryByRole('link', { name: /open in github/i })).toBeNull()
})
