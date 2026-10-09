import { QueryClient } from '@tanstack/react-query'
import { act, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import { listReviewsQueryKey } from '@/api/generated/@tanstack/react-query.gen.ts'
import type { ReviewQueue, ReviewRequest } from '@/api/generated/types.gen.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { makeReviewRequest } from '@/test/fixtures.ts'
import { appQueryClient, renderWithClient } from '@/test/renderWithClient.tsx'
import { ReviewQueuePanel } from './ReviewQueuePanel.tsx'

// How the Reviews section reads the queue: with no forge to ask, a read that
// fails and Try again, Refresh, and reading again when the section opens.

const reviewsPath = '/api/reviews'

// waiting and waitingToo are two of ana's requests in acme/api.
const waiting = makeReviewRequest()
const waitingToo = makeReviewRequest({
  number: 7,
  url: 'https://github.com/acme/api/pull/7',
  title: 'feat: list the review queue on the web',
})

// queueOf is an available queue of the requests given, oldest first as the
// server sends it, with nothing for the filter to offer.
function queueOf(...requests: ReviewRequest[]): ReviewQueue {
  return { available: true, requests, facet_order: [] }
}

// refused is a problem answer, as the server gives a failed read.
function refused(body: object, status: number): Response {
  return Response.json(body, { status, headers: { 'Content-Type': 'application/problem+json' } })
}

// statusSaying is the live status line that says text, if one does: the
// panel's summary and its outcome line are both status lines.
function statusSaying(text: string): HTMLElement | undefined {
  return screen.getAllByRole('status').find((line) => line.textContent === text)
}

// readsOf counts the reads of the queue among the requests the page made.
function readsOf(requests: Request[]): number {
  return requests.filter((request) => new URL(request.url).pathname === reviewsPath).length
}

test('with no forge to ask, says what would give it one', async () => {
  // Arrange
  fakeApi({ [reviewsPath]: { available: false, requests: [], facet_order: [] } })

  // Act
  renderWithClient(<ReviewQueuePanel />)

  // Assert
  const said = await screen.findByText(/no forge to ask/i)
  expect(said.textContent).toMatch(/GitHub or GitLab.*forge\.kind and forge\.host/)
  expect(screen.queryByRole('button', { name: 'Refresh' })).toBeNull()
})

test('a queue that could not be read says why', async () => {
  // Arrange
  const detail =
    'no forge token was found; for GitHub set $GITHUB_TOKEN or sign in with gh, for GitLab set $GITLAB_TOKEN, or set forge.token'
  fakeApi({ [reviewsPath]: () => refused({ detail }, 422) })

  // Act
  renderWithClient(<ReviewQueuePanel />)

  // Assert
  expect((await screen.findByRole('alert')).textContent).toBe(detail)
})

test("a failed read is said at once, never retried behind the user's back", async () => {
  // Arrange
  // The app's own client, whose queries retry by default: the queue is a
  // search under the forge's rate limits, and the server's answer is already
  // its verdict, so asking again is the user's call — Try again.
  const requests = fakeApi({ [reviewsPath]: () => refused({ detail: 'wait and try again' }, 502) })
  const client = new QueryClient()

  // Act
  renderWithClient(<ReviewQueuePanel />, client)

  // Assert
  expect((await screen.findByRole('alert')).textContent).toBe('wait and try again')
  expect(readsOf(requests)).toBe(1)
})

test('a refusal with no reason says to press Try again', async () => {
  // Arrange
  fakeApi({ [reviewsPath]: () => refused({}, 500) })

  // Act
  renderWithClient(<ReviewQueuePanel />)

  // Assert
  expect((await screen.findByRole('alert')).textContent).toBe(
    'The review queue could not be read. Press Try again.',
  )
})

test.each([
  ['the queue it read', queueOf(waiting), '1 pull request waits on your review, oldest first.'],
  ['that nothing waits', queueOf(), 'No pull requests are waiting on your review.'],
])('Try again says %s in the status line it already had', async (_, second, said) => {
  // Arrange
  // A screen reader hears a status line as it changes, not one mounted with
  // its words already in it.
  const user = userEvent.setup()
  const answers = [refused({}, 502), Response.json(second)]
  fakeApi({ [reviewsPath]: () => answers.shift() })
  renderWithClient(<ReviewQueuePanel />)
  const retry = await screen.findByRole('button', { name: 'Try again' })
  const linesBefore = screen.getAllByRole('status')

  // Act
  await user.click(retry)

  // Assert
  await screen.findByRole('button', { name: 'Refresh' })
  const summary = statusSaying(said)
  expect(summary).toBeDefined()
  expect(linesBefore).toContain(summary)
})

test('a retry after a failed first read keeps its place and its focus', async () => {
  // Arrange
  // The second read hangs, as a read over a real network is in flight a while:
  // the panel must not fall back to the first read's placeholder, which would
  // take the control, and the focus it holds, away.
  const user = userEvent.setup()
  const answers = [Promise.resolve(refused({}, 502)), new Promise(() => null)]
  vi.stubGlobal(
    'fetch',
    vi.fn(() => answers.shift()),
  )
  renderWithClient(<ReviewQueuePanel />)
  const retry = await screen.findByRole('button', { name: 'Try again' })

  // Act
  await user.click(retry)

  // Assert
  expect(screen.queryByText('Reading your review queue…')).toBeNull()
  expect(retry.textContent).toBe('Trying again…')
  expect(document.activeElement).toBe(retry)
})

test('Try again reads the queue again and keeps focus, as Refresh', async () => {
  // Arrange
  const user = userEvent.setup()
  const answers = [refused({}, 502), Response.json(queueOf(waiting))]
  fakeApi({ [reviewsPath]: () => answers.shift() })
  renderWithClient(<ReviewQueuePanel />)
  const retry = await screen.findByRole('button', { name: 'Try again' })

  // Act
  await user.click(retry)

  // Assert
  expect(await screen.findByRole('list', { name: 'Waiting on your review' })).toBeTruthy()
  expect(screen.queryByRole('alert')).toBeNull()
  expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Refresh' }))
})

test('Refresh reads the queue from the forge again', async () => {
  // Arrange
  const user = userEvent.setup()
  const answers = [queueOf(waiting), queueOf(waiting, waitingToo)]
  const requests = fakeApi({ [reviewsPath]: () => answers.shift() })
  renderWithClient(<ReviewQueuePanel />)
  await screen.findByText('1 pull request waits on your review, oldest first.')
  const summary = statusSaying('1 pull request waits on your review, oldest first.')

  // Act
  await user.click(screen.getByRole('button', { name: 'Refresh' }))

  // Assert
  expect(await screen.findByText('2 pull requests wait on your review, oldest first.')).toBe(
    summary,
  )
  expect(readsOf(requests)).toBe(2)
})

test('a failed refresh says why and keeps the queue it last read', async () => {
  // Arrange
  const user = userEvent.setup()
  const answers = [queueOf(waiting), refused({ detail: 'wait and try again' }, 502)]
  fakeApi({ [reviewsPath]: () => answers.shift() })
  renderWithClient(<ReviewQueuePanel />)
  const refresh = await screen.findByRole('button', { name: 'Refresh' })

  // Act
  await user.click(refresh)

  // Assert
  expect((await screen.findByRole('alert')).textContent).toBe('wait and try again')
  expect(screen.getByRole('list', { name: 'Waiting on your review' })).toBeTruthy()
  expect(statusSaying('1 pull request waits on your review, oldest first.')).toBeDefined()
  expect(refresh.textContent).toBe('Try again')
})

test('a retry in flight after a failed refresh says so and keeps its focus', async () => {
  // Arrange
  // A failed refresh keeps the queue it last read, so the retry is caught
  // mid-read beside it; the third read hangs.
  const user = userEvent.setup()
  const answers = [
    Promise.resolve(Response.json(queueOf(waiting))),
    Promise.resolve(refused({ detail: 'wait and try again' }, 502)),
    new Promise(() => null),
  ]
  vi.stubGlobal(
    'fetch',
    vi.fn(() => answers.shift()),
  )
  renderWithClient(<ReviewQueuePanel />)
  await user.click(await screen.findByRole('button', { name: 'Refresh' }))
  const retry = await screen.findByRole('button', { name: 'Try again' })

  // Act
  await user.click(retry)

  // Assert
  expect(retry.textContent).toBe('Trying again…')
  expect(retry.getAttribute('aria-disabled')).toBe('true')
  expect(document.activeElement).toBe(retry)
  expect(screen.getByRole('list', { name: 'Waiting on your review' })).toBeTruthy()
})

test('a refresh in flight says so, keeps its focus, and is not asked twice', async () => {
  // Arrange
  // The second read hangs, so the control is caught mid-read. A disabled
  // control would drop the focus a keyboard user pressed it with, so it is
  // marked busy instead, and a press while busy starts nothing.
  const user = userEvent.setup()
  const answers = [Promise.resolve(Response.json(queueOf(waiting))), new Promise(() => null)]
  const fetch = vi.fn(() => answers.shift())
  vi.stubGlobal('fetch', fetch)
  renderWithClient(<ReviewQueuePanel />)
  const refresh = await screen.findByRole('button', { name: 'Refresh' })

  // Act
  await user.click(refresh)
  await user.click(refresh)

  // Assert
  expect(refresh.textContent).toBe('Refreshing…')
  expect(refresh.getAttribute('aria-disabled')).toBe('true')
  expect(refresh.hasAttribute('disabled')).toBe(false)
  expect(document.activeElement).toBe(refresh)
  expect(fetch).toHaveBeenCalledTimes(2)
})

test.each([
  ['reads nothing for a queue read within the minute', Date.now(), 0],
  ['reads again a queue read over a minute ago', 0, 1],
])('opening the section %s', async (_, readAt, reads) => {
  // Arrange
  const requests = fakeApi({ [reviewsPath]: queueOf(waiting) })
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  client.setQueryData(listReviewsQueryKey(), queueOf(waiting), { updatedAt: readAt })

  // Act
  renderWithClient(<ReviewQueuePanel />, client)

  // Assert
  expect(await screen.findByRole('list', { name: 'Waiting on your review' })).toBeTruthy()
  await screen.findByRole('button', { name: 'Refresh' })
  expect(readsOf(requests)).toBe(reads)
})

// openedTwice opens the Reviews section, closes it, moves the clock on by
// seconds, and opens it again, all over one client, as switching sections does.
async function openedTwice(seconds: number): Promise<Request[]> {
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date('2026-09-30T12:00:00Z'))
  const requests = fakeApi({ [reviewsPath]: () => queueOf(waiting) })
  const client = appQueryClient()
  const view = renderWithClient(<ReviewQueuePanel />, client)
  await screen.findByText('1 pull request waits on your review, oldest first.')
  view.unmount()
  vi.setSystemTime(new Date(Date.parse('2026-09-30T12:00:00Z') + seconds * 1000))
  renderWithClient(<ReviewQueuePanel />, client)
  await screen.findByText('1 pull request waits on your review, oldest first.')

  return requests
}

test('reads the queue again when the section opens after 30 seconds', async () => {
  // Act
  const requests = await openedTwice(31)

  // Assert
  await waitFor(() => {
    expect(readsOf(requests)).toBe(2)
  })
})

test('does not read the queue again when the section opens within 30 seconds', async () => {
  // Act
  const requests = await openedTwice(29)

  // Assert
  // Let a read the reopening might start reach fetch before counting.
  await act(
    () =>
      new Promise((resolve) => {
        setTimeout(resolve, 0)
      }),
  )
  expect(readsOf(requests)).toBe(1)
})
