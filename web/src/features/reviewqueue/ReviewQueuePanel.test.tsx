import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import { listReviewsQueryKey } from '@/api/generated/@tanstack/react-query.gen.ts'
import type { ReviewQueue } from '@/api/generated/types.gen.ts'
import { useHealthStore } from '@/api/health.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { gitLabWords, makeHealth, makeReviewRequest } from '@/test/fixtures.ts'
import { drawnMark, markShape } from '@/test/marks.tsx'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { ReviewQueuePanel } from './ReviewQueuePanel.tsx'

const reviewsPath = '/api/reviews'
const hour = 3_600_000

// hoursAgo is the RFC 3339 time some hours before now.
function hoursAgo(hours: number): string {
  return new Date(Date.now() - hours * hour).toISOString()
}

// queueOf is an available queue of the requests given, oldest first as the
// server sends it.
function queueOf(...requests: ReturnType<typeof makeReviewRequest>[]): ReviewQueue {
  return { available: true, requests }
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

const waitingLongest = makeReviewRequest({ draft: true })
const waitingLess = makeReviewRequest({
  number: 7,
  url: 'https://github.com/acme/web/pull/7',
  title: 'feat: list the review queue on the web',
  author: 'sam',
  repository: 'acme/web',
  ci: 'passed',
  opened_at: hoursAgo(5),
})

test('the Reviews section lists requests by role and name', async () => {
  // Arrange
  fakeApi({ [reviewsPath]: queueOf(waitingLongest, waitingLess) })

  // Act
  renderWithClient(<ReviewQueuePanel />)

  // Assert
  const list = await screen.findByRole('list', { name: 'Review requests' })
  const rows = within(list).getAllByRole('listitem')
  expect(rows.map((row) => within(row).getByRole('link').getAttribute('href'))).toEqual([
    waitingLongest.url,
    waitingLess.url,
  ])
  expect(rows[0]?.textContent).toMatch(
    /#42.*redact the token.*acme\/api · by ana · 3d ago · Draft.*CI failed/,
  )
  expect(rows[1]?.textContent).toMatch(/#7.*acme\/web · by sam · 5h ago.*CI passed/)
})

test('draws how CI stands on each request as its mark, beside the words', async () => {
  // Arrange
  const stands = [
    { ci: 'none', words: 'CI not reported', mark: 'unknown' },
    { ci: 'running', words: 'CI running', mark: 'in-flight' },
    { ci: 'passed', words: 'CI passed', mark: 'done' },
    { ci: 'failed', words: 'CI failed', mark: 'failed' },
  ] as const
  fakeApi({
    [reviewsPath]: queueOf(
      ...stands.map(({ ci }, index) =>
        makeReviewRequest({
          ci,
          number: index + 1,
          url: `https://github.com/acme/api/pull/${String(index + 1)}`,
        }),
      ),
    ),
  })

  // Act
  renderWithClient(<ReviewQueuePanel />)

  // Assert
  await screen.findByRole('list', { name: 'Review requests' })
  expect(stands.map(({ words }) => markShape(screen.getByText(words)))).toEqual(
    stands.map(({ mark }) => drawnMark(mark)),
  )
})

test('the mockup lists its own queue without a server', async () => {
  // Arrange
  vi.stubEnv('VITE_MOCK', 'true')

  // Act
  renderWithClient(<ReviewQueuePanel />)

  // Assert
  const list = await screen.findByRole('list', { name: 'Review requests' })
  expect(within(list).getAllByRole('listitem').length).toBeGreaterThan(1)
  expect(globalThis.fetch).not.toHaveBeenCalled()
})

test('a request the forge names no repository for says only who asks', async () => {
  // Arrange
  fakeApi({ [reviewsPath]: queueOf(makeReviewRequest({ repository: '' })) })

  // Act
  renderWithClient(<ReviewQueuePanel />)

  // Assert
  const list = await screen.findByRole('list', { name: 'Review requests' })
  expect(list.textContent).toContain('the logby ana · 3d ago')
})

test('opens a request in a new tab that cannot reach back to the page', async () => {
  // Arrange
  fakeApi({ [reviewsPath]: queueOf(waitingLongest) })

  // Act
  renderWithClient(<ReviewQueuePanel />)

  // Assert
  const open = await screen.findByRole('link', { name: 'Open #42 (opens in a new tab)' })
  expect(open.getAttribute('href')).toBe(waitingLongest.url)
  expect(open.getAttribute('target')).toBe('_blank')
  expect(open.getAttribute('rel')).toMatch(/\bnoopener\b/)
})

test.each([
  ['moments', 'just now', 0],
  ['minutes', '15m ago', 0.25],
  ['hours', '5h ago', 5],
  ['days', '9d ago', 9 * 24],
])("says a request opened %s ago waited %s in the interface's words", async (_, said, hours) => {
  // Arrange
  fakeApi({ [reviewsPath]: queueOf(makeReviewRequest({ opened_at: hoursAgo(hours) })) })

  // Act
  renderWithClient(<ReviewQueuePanel />)

  // Assert
  const list = await screen.findByRole('list', { name: 'Review requests' })
  expect(within(list).getByText(said, { exact: false })).toBeTruthy()
})

test('gives the date of a request that has waited over a month', async () => {
  // Arrange
  const opened = hoursAgo(40 * 24)
  fakeApi({ [reviewsPath]: queueOf(makeReviewRequest({ opened_at: opened })) })

  // Act
  renderWithClient(<ReviewQueuePanel />)

  // Assert
  const list = await screen.findByRole('list', { name: 'Review requests' })
  const time = within(list).getByText(new Date(opened).toLocaleDateString(), { exact: false })
  expect(time.getAttribute('datetime')).toBe(opened)
})

test('says a request whose opening the forge did not give waited some time', async () => {
  // Arrange
  // The server sends the zero time for a date the forge did not give.
  fakeApi({ [reviewsPath]: queueOf(makeReviewRequest({ opened_at: '0001-01-01T00:00:00Z' })) })

  // Act
  renderWithClient(<ReviewQueuePanel />)

  // Assert
  const list = await screen.findByRole('list', { name: 'Review requests' })
  expect(within(list).getByText('some time ago')).toBeTruthy()
})

test('counts and marks merge requests in GitLab words', async () => {
  // Arrange
  useHealthStore.setState({ health: makeHealth(gitLabWords) })
  fakeApi({ [reviewsPath]: queueOf(waitingLongest, waitingLess) })

  // Act
  renderWithClient(<ReviewQueuePanel />)

  // Assert
  expect(
    await screen.findByText('2 merge requests wait on your review, oldest first.'),
  ).toBeTruthy()
  expect(screen.getByRole('link', { name: 'Open !42 (opens in a new tab)' })).toBeTruthy()
})

test('an empty queue says nothing is waiting', async () => {
  // Arrange
  fakeApi({ [reviewsPath]: queueOf() })

  // Act
  renderWithClient(<ReviewQueuePanel />)

  // Assert
  expect(await screen.findAllByText('Nothing is waiting on your review.')).not.toHaveLength(0)
  expect(screen.queryByRole('list', { name: 'Review requests' })).toBeNull()
  expect(screen.queryByText(/wait on your review/)).toBeNull()
})

test('with no forge to ask, says what would give it one', async () => {
  // Arrange
  fakeApi({ [reviewsPath]: { available: false, requests: [] } })

  // Act
  renderWithClient(<ReviewQueuePanel />)

  // Assert
  const said = await screen.findByText(/no forge to ask/i)
  expect(said.textContent).toMatch(/GitHub or GitLab.*forge\.kind and forge\.host/)
  expect(screen.queryByRole('button', { name: 'Refresh' })).toBeNull()
})

test('a queue that could not be read says why', async () => {
  // Arrange
  const detail = 'no forge token was found; sign in with gh or glab, or set forge.token'
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
  // its verdict, so asking again is the user's call — Retry.
  const requests = fakeApi({ [reviewsPath]: () => refused({ detail: 'wait and try again' }, 502) })
  const client = new QueryClient()

  // Act
  render(
    <QueryClientProvider client={client}>
      <ReviewQueuePanel />
    </QueryClientProvider>,
  )

  // Assert
  expect((await screen.findByRole('alert')).textContent).toBe('wait and try again')
  expect(readsOf(requests)).toBe(1)
})

test('a refusal with no reason says to press Retry', async () => {
  // Arrange
  fakeApi({ [reviewsPath]: () => refused({}, 500) })

  // Act
  renderWithClient(<ReviewQueuePanel />)

  // Assert
  expect((await screen.findByRole('alert')).textContent).toBe(
    'The review queue could not be read. Press Retry to try again.',
  )
})

test.each([
  [
    'the queue it read',
    queueOf(waitingLongest),
    '1 pull request waits on your review, oldest first.',
  ],
  ['that nothing waits', queueOf(), 'Nothing is waiting on your review.'],
])('Retry says %s in the status line it already had', async (_, second, said) => {
  // Arrange
  // A screen reader hears a status line as it changes, not one mounted with
  // its words already in it.
  const user = userEvent.setup()
  const answers = [refused({}, 502), Response.json(second)]
  fakeApi({ [reviewsPath]: () => answers.shift() })
  renderWithClient(<ReviewQueuePanel />)
  const retry = await screen.findByRole('button', { name: 'Retry' })
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
  const retry = await screen.findByRole('button', { name: 'Retry' })

  // Act
  await user.click(retry)

  // Assert
  expect(screen.queryByText('Reading your review queue…')).toBeNull()
  expect(retry.textContent).toBe('Retrying…')
  expect(document.activeElement).toBe(retry)
})

test('Retry reads the queue again and keeps focus, as Refresh', async () => {
  // Arrange
  const user = userEvent.setup()
  const answers = [refused({}, 502), Response.json(queueOf(waitingLongest))]
  fakeApi({ [reviewsPath]: () => answers.shift() })
  renderWithClient(<ReviewQueuePanel />)
  const retry = await screen.findByRole('button', { name: 'Retry' })

  // Act
  await user.click(retry)

  // Assert
  expect(await screen.findByRole('list', { name: 'Review requests' })).toBeTruthy()
  expect(screen.queryByRole('alert')).toBeNull()
  expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Refresh' }))
})

test('Refresh reads the queue from the forge again', async () => {
  // Arrange
  const user = userEvent.setup()
  const answers = [queueOf(waitingLongest), queueOf(waitingLongest, waitingLess)]
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
  const answers = [queueOf(waitingLongest), refused({ detail: 'wait and try again' }, 502)]
  fakeApi({ [reviewsPath]: () => answers.shift() })
  renderWithClient(<ReviewQueuePanel />)
  const refresh = await screen.findByRole('button', { name: 'Refresh' })

  // Act
  await user.click(refresh)

  // Assert
  expect((await screen.findByRole('alert')).textContent).toBe('wait and try again')
  expect(screen.getByRole('list', { name: 'Review requests' })).toBeTruthy()
  expect(statusSaying('1 pull request waits on your review, oldest first.')).toBeDefined()
  expect(refresh.textContent).toBe('Retry')
})

test('a retry in flight after a failed refresh says so and keeps its focus', async () => {
  // Arrange
  // A failed refresh keeps the queue it last read, so the retry is caught
  // mid-read beside it; the third read hangs.
  const user = userEvent.setup()
  const answers = [
    Promise.resolve(Response.json(queueOf(waitingLongest))),
    Promise.resolve(refused({ detail: 'wait and try again' }, 502)),
    new Promise(() => null),
  ]
  vi.stubGlobal(
    'fetch',
    vi.fn(() => answers.shift()),
  )
  renderWithClient(<ReviewQueuePanel />)
  await user.click(await screen.findByRole('button', { name: 'Refresh' }))
  const retry = await screen.findByRole('button', { name: 'Retry' })

  // Act
  await user.click(retry)

  // Assert
  expect(retry.textContent).toBe('Retrying…')
  expect(retry.getAttribute('aria-disabled')).toBe('true')
  expect(document.activeElement).toBe(retry)
  expect(screen.getByRole('list', { name: 'Review requests' })).toBeTruthy()
})

test('a refresh in flight says so, keeps its focus, and is not asked twice', async () => {
  // Arrange
  // The second read hangs, so the control is caught mid-read. A disabled
  // control would drop the focus a keyboard user pressed it with, so it is
  // marked busy instead, and a press while busy starts nothing.
  const user = userEvent.setup()
  const answers = [Promise.resolve(Response.json(queueOf(waitingLongest))), new Promise(() => null)]
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
  const requests = fakeApi({ [reviewsPath]: queueOf(waitingLongest) })
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  client.setQueryData(listReviewsQueryKey(), queueOf(waitingLongest), { updatedAt: readAt })

  // Act
  render(
    <QueryClientProvider client={client}>
      <ReviewQueuePanel />
    </QueryClientProvider>,
  )

  // Assert
  expect(await screen.findByRole('list', { name: 'Review requests' })).toBeTruthy()
  await screen.findByRole('button', { name: 'Refresh' })
  expect(readsOf(requests)).toBe(reads)
})

test('Copy URL puts the address on the clipboard and says so', async () => {
  // Arrange
  const user = userEvent.setup()
  fakeApi({ [reviewsPath]: queueOf(waitingLongest, waitingLess) })
  renderWithClient(<ReviewQueuePanel />)

  // Act
  await user.click(await screen.findByRole('button', { name: 'Copy URL to #7' }))

  // Assert
  expect(await navigator.clipboard.readText()).toBe(waitingLess.url)
  expect(statusSaying('Copied the URL of #7.')).toBeDefined()
})

test('a copy the browser refuses says how to get the address instead', async () => {
  // Arrange
  const user = userEvent.setup()
  fakeApi({ [reviewsPath]: queueOf(waitingLongest) })
  renderWithClient(<ReviewQueuePanel />)
  const copy = await screen.findByRole('button', { name: 'Copy URL to #42' })
  vi.spyOn(navigator.clipboard, 'writeText').mockRejectedValue(
    new DOMException('Write permission denied.', 'NotAllowedError'),
  )

  // Act
  await user.click(copy)

  // Assert
  expect((await screen.findByRole('alert')).textContent).toBe(
    'The URL of #42 could not be copied; open it, and copy it from the address bar.',
  )
})
