import { onlineManager } from '@tanstack/react-query'
import { act, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import { mockConfig } from '@/dev/mockConfig.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { makeIssueDetail } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { IssueDetailPanel } from './IssueDetailPanel.tsx'

// Try again beside an issue that could not be read: it reads the issue again,
// and where focus goes while it does, whether or not the issue was read before.

test('Try again reads a refused issue again', async () => {
  // Arrange
  const answers = [Response.json({}, { status: 500 }), Response.json(makeIssueDetail())]
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
    screen.getByRole('heading', { level: 2, name: makeIssueDetail().summary }),
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
    Response.json(makeIssueDetail()),
    Response.json(makeIssueDetail({ description: 'Tokens reach the request log and the trace.' })),
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
    Response.json(makeIssueDetail()),
    Response.json({}, { status: 500 }),
    Response.json(makeIssueDetail()),
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
    screen.getByRole('heading', { level: 2, name: makeIssueDetail().summary }),
  )
})

test('a Try again refused again beside an issue already read keeps its focus', async () => {
  // Arrange
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date('2026-09-23T12:00:00Z'))
  const answers = [
    Response.json(makeIssueDetail()),
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
    Promise.resolve(Response.json(makeIssueDetail())),
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
