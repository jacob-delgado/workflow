import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import type { ReviewQueue } from '@/api/generated/types.gen.ts'
import { useHealthStore } from '@/api/health.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import {
  describedReviewRequest,
  gitLabWords,
  makeHealth,
  makeReviewRequest,
} from '@/test/fixtures.ts'
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
// server sends it, with nothing for the filter to offer.
function queueOf(...requests: ReturnType<typeof makeReviewRequest>[]): ReviewQueue {
  return { available: true, requests, facet_order: [] }
}

// statusSaying is the live status line that says text, if one does: the
// panel's summary and its outcome line are both status lines.
function statusSaying(text: string): HTMLElement | undefined {
  return screen.getAllByRole('status').find((line) => line.textContent === text)
}

// waitingLongest is the fixture's request, made a draft.
const waitingLongest = describedReviewRequest(
  { repository: 'acme/api', ci: 'failed', draft: true, author: 'ana' },
  [
    { kind: 'repository', value: 'acme/api', label: 'acme/api' },
    { kind: 'ci', value: 'failed', label: 'CI failed' },
    { kind: 'draft', value: 'draft', label: 'draft' },
    { kind: 'author', value: 'ana', label: 'by ana' },
  ],
)
const waitingLess = describedReviewRequest(
  {
    number: 7,
    url: 'https://github.com/acme/web/pull/7',
    title: 'feat: list the review queue on the web',
    author: 'sam',
    repository: 'acme/web',
    draft: false,
    ci: 'passed',
    opened_at: hoursAgo(5),
  },
  [
    { kind: 'repository', value: 'acme/web', label: 'acme/web' },
    { kind: 'ci', value: 'passed', label: 'CI passed' },
    { kind: 'draft', value: 'ready', label: 'ready' },
    { kind: 'author', value: 'sam', label: 'by sam' },
  ],
)

test('the Reviews section lists requests by role and name', async () => {
  // Arrange
  fakeApi({ [reviewsPath]: queueOf(waitingLongest, waitingLess) })

  // Act
  renderWithClient(<ReviewQueuePanel />)

  // Assert
  const list = await screen.findByRole('list', { name: 'Waiting on your review' })
  const rows = within(list).getAllByRole('listitem')
  expect(rows.map((row) => within(row).getByRole('link').getAttribute('href'))).toEqual([
    waitingLongest.url,
    waitingLess.url,
  ])
  expect(rows[0]?.textContent).toMatch(
    /#42.*redact the token.*acme\/api.*by ana.*3d ago.*Draft.*CI failed/,
  )
  expect(rows[1]?.textContent).toMatch(/#7.*acme\/web.*by sam.*5h ago.*CI passed/)
  const first = within(rows[0] ?? document.body)
  expect(
    ['acme/api', 'by ana', '3d ago', 'Draft'].map((fact) => first.getByText(fact).textContent),
  ).toEqual(['acme/api', 'by ana', '3d ago', 'Draft'])
})

// The words are the label the server gives the request's CI facet, the ones
// the filter offers it by. One is a label the page could not spell from the
// CI's value, so the row is seen to read the server's.
test('draws how CI stands on each request as its mark, beside the words', async () => {
  // Arrange
  const stands = [
    { ci: 'none', words: 'CI unreported', mark: 'unknown' },
    { ci: 'running', words: 'CI running', mark: 'in-flight' },
    { ci: 'passed', words: 'CI passed', mark: 'done' },
    { ci: 'failed', words: 'CI failed', mark: 'failed' },
  ] as const
  fakeApi({
    [reviewsPath]: queueOf(
      ...stands.map(({ ci, words }, index) =>
        describedReviewRequest(
          {
            repository: 'acme/api',
            ci,
            draft: false,
            author: 'ana',
            number: index + 1,
            url: `https://github.com/acme/api/pull/${String(index + 1)}`,
          },
          [
            { kind: 'repository', value: 'acme/api', label: 'acme/api' },
            { kind: 'ci', value: ci, label: words },
            { kind: 'draft', value: 'ready', label: 'ready' },
            { kind: 'author', value: 'ana', label: 'by ana' },
          ],
        ),
      ),
    ),
  })

  // Act
  renderWithClient(<ReviewQueuePanel />)

  // Assert
  await screen.findByRole('list', { name: 'Waiting on your review' })
  expect(stands.map(({ words }) => markShape(screen.getByText(words)))).toEqual(
    stands.map(({ mark }) => drawnMark(mark)),
  )
})

test('draws a draft as the not-started mark, before the word', async () => {
  // Arrange
  fakeApi({ [reviewsPath]: queueOf(waitingLongest) })

  // Act
  renderWithClient(<ReviewQueuePanel />)

  // Assert
  expect(markShape(await screen.findByText('Draft'))).toBe(drawnMark('not-started'))
})

test('a request the forge names no repository for says only who asks', async () => {
  // Arrange
  const nowhere = describedReviewRequest(
    { repository: '', ci: 'failed', draft: false, author: 'ana' },
    [
      { kind: 'repository', value: '', label: 'no repository' },
      { kind: 'ci', value: 'failed', label: 'CI failed' },
      { kind: 'draft', value: 'ready', label: 'ready' },
      { kind: 'author', value: 'ana', label: 'by ana' },
    ],
  )
  fakeApi({ [reviewsPath]: queueOf(nowhere) })

  // Act
  renderWithClient(<ReviewQueuePanel />)

  // Assert
  const list = await screen.findByRole('list', { name: 'Waiting on your review' })
  expect(within(list).getByText('by ana')).toBeTruthy()
  expect(within(list).queryByText('acme/api')).toBeNull()
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
  const list = await screen.findByRole('list', { name: 'Waiting on your review' })
  expect(within(list).getByText(said, { exact: false })).toBeTruthy()
})

test('gives the date of a request that has waited over a month', async () => {
  // Arrange
  const opened = '2025-03-14T12:00:00Z'
  fakeApi({ [reviewsPath]: queueOf(makeReviewRequest({ opened_at: opened })) })

  // Act
  renderWithClient(<ReviewQueuePanel />)

  // Assert
  const list = await screen.findByRole('list', { name: 'Waiting on your review' })
  const time = within(list).getByText('2025-03-14', { exact: false })
  expect(time.getAttribute('datetime')).toBe(opened)
})

test('says a request whose opening the forge did not give waited some time', async () => {
  // Arrange
  // The server leaves the time out when the forge did not give one.
  const undated = makeReviewRequest()
  delete undated.opened_at
  fakeApi({ [reviewsPath]: queueOf(undated) })

  // Act
  renderWithClient(<ReviewQueuePanel />)

  // Assert
  const list = await screen.findByRole('list', { name: 'Waiting on your review' })
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

test('an empty queue on GitLab says so in GitLab words', async () => {
  // Arrange
  useHealthStore.setState({ health: makeHealth(gitLabWords) })
  fakeApi({ [reviewsPath]: queueOf() })

  // Act
  renderWithClient(<ReviewQueuePanel />)

  // Assert
  expect(await screen.findAllByText('No merge requests are waiting on your review.')).toHaveLength(
    2,
  )
})

test('an empty queue says nothing is waiting', async () => {
  // Arrange
  fakeApi({ [reviewsPath]: queueOf() })

  // Act
  renderWithClient(<ReviewQueuePanel />)

  // Assert
  expect(
    await screen.findAllByText('No pull requests are waiting on your review.'),
  ).not.toHaveLength(0)
  expect(screen.queryByRole('list', { name: 'Waiting on your review' })).toBeNull()
  expect(screen.queryByText(/wait on your review/)).toBeNull()
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

test('Copy URL says it copied inside its own row, beside the button', async () => {
  // Arrange
  const user = userEvent.setup()
  fakeApi({ [reviewsPath]: queueOf(waitingLongest, waitingLess) })
  renderWithClient(<ReviewQueuePanel />)
  const copy = await screen.findByRole('button', { name: 'Copy URL to #7' })

  // Act
  await user.click(copy)

  // Assert
  const row = screen.getAllByRole('listitem').find((item) => item.contains(copy))
  expect(row).toBeDefined()
  expect(await within(row as HTMLElement).findByText('Copied the URL of #7.')).toBeTruthy()
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
