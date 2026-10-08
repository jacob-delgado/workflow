import { onlineManager } from '@tanstack/react-query'
import { act, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import type { Health, IssueDetail } from '@/api/generated/types.gen.ts'
import { useHealthStore } from '@/api/health.ts'
import { mockConfig } from '@/dev/mockConfig.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { makeHealth } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { IssueDetailPanel } from './IssueDetailPanel.tsx'

// detailOf is a contract-valid issue detail, with the fields a case cares about
// overridden.
function detailOf(overrides: Partial<IssueDetail> = {}): IssueDetail {
  return {
    key: 'PROJ-1',
    tracker: 'jira',
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

test('gives the issue’s type and priority each an element of its own', async () => {
  // Arrange
  serveIssue(detailOf())

  // Act
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)

  // Assert
  expect((await screen.findByText('High priority')).textContent).toBe('High priority')
  expect(screen.getByText('Bug').textContent).toBe('Bug')
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
  // The server leaves the date out when Jira's could not be read.
  serveIssue(detailOf({ comments: [{ author: 'Ana Lopez', body: 'Undated.' }] }))

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

test('Try again reads a refused issue again', async () => {
  // Arrange
  const answers = [Response.json({}, { status: 500 }), Response.json(detailOf())]
  const requests = fakeApi({ '/api/issues/PROJ-1': () => answers.shift() })
  const user = userEvent.setup()
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)
  await screen.findByRole('alert')

  // Act
  await user.click(screen.getByRole('button', { name: 'Try again' }))

  // Assert
  // The Try again goes once the issue is read, so its focus follows to the issue's
  // heading rather than falling to the page.
  expect(await screen.findByText('Tokens reach the request log.')).toBeTruthy()
  const reads = requests.filter((request) => new URL(request.url).pathname === '/api/issues/PROJ-1')
  expect(reads).toHaveLength(2)
  expect(screen.queryByRole('button', { name: 'Try again' })).toBeNull()
  expect(document.activeElement).toBe(
    screen.getByRole('heading', { level: 2, name: detailOf().summary }),
  )
})

test('a Try again beside an issue never read keeps its focus through a read refused again', async () => {
  // Arrange
  // The issue was never read: its first read was refused, so the Try again stands
  // beside the reason alone. The read the Try again starts is held until the test
  // answers it, so the Try again is caught mid-read.
  let answer: (response: Response) => void = () => undefined
  const held = new Promise<Response>((resolve) => {
    answer = resolve
  })
  const answers = [Promise.resolve(Response.json({}, { status: 500 })), held]
  vi.stubGlobal(
    'fetch',
    vi.fn(() => answers.shift() ?? new Promise<Response>(() => undefined)),
  )
  const user = userEvent.setup()
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)
  const retry = await screen.findByRole('button', { name: 'Try again' })

  // Act: press Try again, and the read it starts is held
  await user.click(retry)

  // Assert: the same Try again stays beside the reason, busy, with its focus
  expect(await screen.findByRole('button', { name: 'Trying again…' })).toBe(retry)
  expect(retry.getAttribute('aria-disabled')).toBe('true')
  expect(document.activeElement).toBe(retry)
  const keptAlert = screen.getByRole('alert')
  expect(keptAlert.textContent).toBe('PROJ-1 could not be read. Press Try again.')
  expect(screen.queryByText(/reading PROJ-1/i)).toBeNull()

  // Act: the held read is refused too
  answer(Response.json({}, { status: 500 }))

  // Assert: the same Try again, ready again, still has focus, and the refusal is
  // told again in an alert of its own, since an alert that keeps its words is
  // not spoken again
  expect(await screen.findByRole('button', { name: 'Try again' })).toBe(retry)
  expect(retry.getAttribute('aria-disabled')).not.toBe('true')
  expect(document.activeElement).toBe(retry)
  const refusedAgain = screen.getByRole('alert')
  expect(refusedAgain).not.toBe(keptAlert)
  expect(refusedAgain.textContent).toBe('PROJ-1 could not be read. Press Try again.')
})

test('a read nobody pressed Try again for keeps the latest refusal beside an issue never read', async () => {
  // Arrange
  // The issue was never read: its first read was refused, and so was the read
  // its Try again started. A reconnect then reads it again on its own, and that
  // read is held, so the panel is caught mid-read.
  const refusal = (detail: string) =>
    Promise.resolve(
      Response.json(
        { title: 'Bad gateway', status: 502, detail, code: 'unreachable' },
        { status: 502, headers: { 'Content-Type': 'application/problem+json' } },
      ),
    )
  const answers = [refusal('first refusal'), refusal('second refusal')]
  vi.stubGlobal(
    'fetch',
    vi.fn(() => answers.shift() ?? new Promise<Response>(() => undefined)),
  )
  const user = userEvent.setup()
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)
  await user.click(await screen.findByRole('button', { name: 'Try again' }))
  await waitFor(() => {
    expect(screen.getByRole('alert').textContent).toBe('second refusal')
  })

  // Act
  act(() => {
    onlineManager.setOnline(false)
    onlineManager.setOnline(true)
  })

  // Assert
  expect(await screen.findByRole('button', { name: 'Trying again…' })).toBeTruthy()
  expect(screen.getByRole('alert').textContent).toBe('second refusal')
})

test('a retried issue read again later leaves focus where the user put it', async () => {
  // Arrange
  // Only the clock is faked: staleness is judged from Date.now(), and a stale
  // issue is read again when the connection comes back.
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
  await user.click(await screen.findByRole('button', { name: 'Try again' }))
  await screen.findByText('Tokens reach the request log.')
  const jira = screen.getByRole('link', { name: /open in jira/i })
  jira.focus()
  vi.setSystemTime(new Date('2026-09-23T12:01:01Z'))

  // Act
  act(() => {
    onlineManager.setOnline(false)
    onlineManager.setOnline(true)
  })

  // Assert
  await screen.findByText('Tokens reach the request log and the trace.')
  expect(document.activeElement).toBe(jira)
})

test('Try again beside an issue already read hands focus to its heading', async () => {
  // Arrange
  // The issue was read, then read again once stale and refused, so the Try again
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
    onlineManager.setOnline(false)
    onlineManager.setOnline(true)
  })
  const retry = await screen.findByRole('button', { name: 'Try again' })

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

test('a Try again refused again beside an issue already read keeps its focus', async () => {
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
    onlineManager.setOnline(false)
    onlineManager.setOnline(true)
  })
  const retry = await screen.findByRole('button', { name: 'Try again' })

  // Act
  await user.click(retry)

  // Assert
  await waitFor(() => {
    expect(answers).toHaveLength(0)
  })
  expect(await screen.findByRole('button', { name: 'Try again' })).toBe(retry)
  expect(document.activeElement).toBe(retry)
})

test('a Try again in flight beside an issue already read keeps its focus and is not asked twice', async () => {
  // Arrange
  // The issue was read, then read again once stale and refused, so the Try again
  // stands beside what the first read showed. The retried read hangs, so the
  // Try again is caught mid-read: a disabled control would drop the focus a
  // keyboard user pressed it with, so it is marked busy instead, and a press
  // while busy starts nothing.
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date('2026-09-23T12:00:00Z'))
  const answers = [
    Promise.resolve(Response.json(detailOf())),
    Promise.resolve(Response.json({}, { status: 500 })),
  ]
  // The comment composer reads the configuration beside the issue; only the
  // issue's reads are scripted.
  const fetch = vi.fn((request: Request) =>
    new URL(request.url).pathname === '/api/config'
      ? Promise.resolve(Response.json(mockConfig))
      : (answers.shift() ?? new Promise<Response>(() => undefined)),
  )
  vi.stubGlobal('fetch', fetch)
  const user = userEvent.setup()
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)
  await screen.findByText('Tokens reach the request log.')
  vi.setSystemTime(new Date('2026-09-23T12:01:01Z'))
  act(() => {
    onlineManager.setOnline(false)
    onlineManager.setOnline(true)
  })
  const retry = await screen.findByRole('button', { name: 'Try again' })
  await user.click(retry)
  await screen.findByRole('button', { name: 'Trying again…' })

  // Act
  await user.click(retry)

  // Assert
  expect(retry.getAttribute('aria-disabled')).toBe('true')
  expect(retry.hasAttribute('disabled')).toBe(false)
  expect(document.activeElement).toBe(retry)
  const issueReads = fetch.mock.calls.filter(
    ([request]) => new URL(request.url).pathname === '/api/issues/PROJ-1',
  )
  expect(issueReads).toHaveLength(3)
})

test('a forge issue opens on its forge, not in Jira', async () => {
  // Arrange
  useHealthStore.setState({ health: makeHealth() })
  serveIssue(
    detailOf({ key: '57', tracker: 'forge', url: 'https://github.com/acme/oss/issues/57' }),
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
  serveIssue(detailOf({ key: '57', tracker: 'forge', url: 'https://forge.example/issues/57' }))

  // Act
  renderWithClient(<IssueDetailPanel issueKey="57" />)

  // Assert
  expect(await screen.findByRole('link', { name: new RegExp(`^Open in ${named}`) })).toBeTruthy()
})

test('before the forge is known, a forge issue opens in the forge, named by no guess', async () => {
  // Arrange
  serveIssue(
    detailOf({ key: '57', tracker: 'forge', url: 'https://gitlab.com/acme/oss/-/issues/57' }),
  )

  // Act
  renderWithClient(<IssueDetailPanel issueKey="57" />)

  // Assert
  const link = await screen.findByRole('link', { name: /open in the forge/i })
  expect(link.getAttribute('href')).toBe('https://gitlab.com/acme/oss/-/issues/57')
  expect(screen.queryByRole('link', { name: /open in github/i })).toBeNull()
})
