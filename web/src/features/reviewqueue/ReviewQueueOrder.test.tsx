import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ReviewQueue } from '@/api/generated/types.gen.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { makeReviewRequest } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { ReviewQueuePanel } from './ReviewQueuePanel.tsx'

const reviewsPath = '/api/reviews'
const hour = 3_600_000

// hoursAgo is the RFC 3339 time some hours before now.
function hoursAgo(hours: number): string {
  return new Date(Date.now() - hours * hour).toISOString()
}

// The queue the terminal's withReviews lists, plus a second request in
// example/repo older than #7 and one the forge names no repository for, so a
// group holds two and the oldest-first order within it shows.
const older = makeReviewRequest({
  number: 12,
  url: 'https://github.com/example/other/pull/12',
  title: 'add request retries',
  repository: 'example/other',
  opened_at: hoursAgo(26),
})
const newer = makeReviewRequest({
  number: 7,
  url: 'https://github.com/example/repo/pull/7',
  title: 'fix flaky redaction test',
  repository: 'example/repo',
  opened_at: hoursAgo(3),
})
const oldestInRepo = makeReviewRequest({
  number: 5,
  url: 'https://github.com/example/repo/pull/5',
  title: 'drop the old retry flag',
  repository: 'example/repo',
  opened_at: hoursAgo(40),
})
const nowhere = makeReviewRequest({
  number: 3,
  url: 'https://example.com/pull/3',
  title: 'a request with no repository',
  repository: '',
  opened_at: hoursAgo(10),
})

// queueOf is an available queue of the requests given, oldest first as the
// server sends it.
function queueOf(...requests: ReturnType<typeof makeReviewRequest>[]): ReviewQueue {
  return { available: true, requests }
}

// linksIn is the address each request listed in container opens, in order.
function linksIn(container: HTMLElement): (string | null)[] {
  return within(container)
    .getAllByRole('link')
    .map((link) => link.getAttribute('href'))
}

// sortChosen is the order the Sort control shows chosen.
function sortChosen(): string | undefined {
  const sort = screen.getByRole<HTMLSelectElement>('combobox', { name: 'Sort' })

  return sort.selectedOptions[0]?.textContent ?? undefined
}

test('lists the queue oldest first until another order is chosen', async () => {
  // Arrange
  fakeApi({ [reviewsPath]: queueOf(oldestInRepo, older, nowhere, newer) })

  // Act
  renderWithClient(<ReviewQueuePanel />)

  // Assert
  const list = await screen.findByRole('list', { name: 'Review requests' })
  expect(sortChosen()).toBe('Oldest first')
  expect(linksIn(list)).toEqual([oldestInRepo.url, older.url, nowhere.url, newer.url])
})

test('Newest first lists the request opened last first, and says so', async () => {
  // Arrange
  fakeApi({ [reviewsPath]: queueOf(oldestInRepo, older, nowhere, newer) })
  renderWithClient(<ReviewQueuePanel />)
  await screen.findByRole('list', { name: 'Review requests' })

  // Act
  await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Sort' }), 'Newest first')

  // Assert
  const list = screen.getByRole('list', { name: 'Review requests' })
  expect(linksIn(list)).toEqual([newer.url, nowhere.url, older.url, oldestInRepo.url])
  expect(
    screen
      .getAllByRole('status')
      .some((line) => line.textContent === '4 pull requests wait on your review, newest first.'),
  ).toBe(true)
})

test('By repository heads each repository, oldest first within it', async () => {
  // Arrange
  fakeApi({ [reviewsPath]: queueOf(oldestInRepo, older, nowhere, newer) })
  renderWithClient(<ReviewQueuePanel />)
  await screen.findByRole('list', { name: 'Review requests' })

  // Act
  await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Sort' }), 'By repository')

  // Assert
  expect(
    screen.getAllByRole('heading', { level: 3 }).map((heading) => heading.textContent),
  ).toEqual(['No repository', 'example/other', 'example/repo'])
  expect(linksIn(screen.getByRole('list', { name: 'Review requests in example/repo' }))).toEqual([
    oldestInRepo.url,
    newer.url,
  ])
  expect(linksIn(screen.getByRole('list', { name: 'Review requests in No repository' }))).toEqual([
    nowhere.url,
  ])
})

test('the order chosen holds across a refresh', async () => {
  // Arrange
  fakeApi({ [reviewsPath]: queueOf(older, newer) })
  renderWithClient(<ReviewQueuePanel />)
  await screen.findByRole('list', { name: 'Review requests' })
  await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Sort' }), 'Newest first')

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Refresh' }))

  // Assert
  await screen.findByRole('button', { name: 'Refresh' })
  expect(sortChosen()).toBe('Newest first')
  expect(linksIn(screen.getByRole('list', { name: 'Review requests' }))).toEqual([
    newer.url,
    older.url,
  ])
})
