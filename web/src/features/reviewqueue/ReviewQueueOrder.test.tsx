import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ReviewFacet, ReviewQueue, ReviewRequest } from '@/api/generated/types.gen.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { describedReviewRequest } from '@/test/fixtures.ts'
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
// failedReady is the facets every request below holds but its repository's,
// as the server labels them: CI failed, ready, by ana.
const failedReady: ReviewFacet[] = [
  { kind: 'ci', value: 'failed', label: 'CI failed' },
  { kind: 'draft', value: 'ready', label: 'ready' },
  { kind: 'author', value: 'ana', label: 'by ana' },
]

// inRepository is a ready request by ana whose CI failed, in repository — none
// when empty — which the server labels label, with its other facets.
function inRepository(
  repository: string,
  label: string,
  fields: Partial<Omit<ReviewRequest, 'facets'>>,
): ReviewRequest {
  return describedReviewRequest(
    { ...fields, repository, ci: 'failed', draft: false, author: 'ana' },
    [{ kind: 'repository', value: repository, label }, ...failedReady],
  )
}

const older = inRepository('example/other', 'example/other', {
  number: 12,
  url: 'https://github.com/example/other/pull/12',
  title: 'add request retries',
  opened_at: hoursAgo(26),
})
const newer = inRepository('example/repo', 'example/repo', {
  number: 7,
  url: 'https://github.com/example/repo/pull/7',
  title: 'fix flaky redaction test',
  opened_at: hoursAgo(3),
})
const oldestInRepo = inRepository('example/repo', 'example/repo', {
  number: 5,
  url: 'https://github.com/example/repo/pull/5',
  title: 'drop the old retry flag',
  opened_at: hoursAgo(40),
})
const nowhere = inRepository('', 'no repository', {
  number: 3,
  url: 'https://example.com/pull/3',
  title: 'a request with no repository',
  opened_at: hoursAgo(10),
})

// queueOf is an available queue of the requests given, oldest first as the
// server sends it, with nothing for the filter to offer.
function queueOf(...requests: ReviewRequest[]): ReviewQueue {
  return { available: true, requests, facet_order: [] }
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
  const list = await screen.findByRole('list', { name: 'Waiting on your review' })
  expect(sortChosen()).toBe('Oldest first')
  expect(linksIn(list)).toEqual([oldestInRepo.url, older.url, nowhere.url, newer.url])
})

test('Newest first lists the request opened last first, and says so', async () => {
  // Arrange
  fakeApi({ [reviewsPath]: queueOf(oldestInRepo, older, nowhere, newer) })
  renderWithClient(<ReviewQueuePanel />)
  await screen.findByRole('list', { name: 'Waiting on your review' })

  // Act
  await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Sort' }), 'Newest first')

  // Assert
  const list = screen.getByRole('list', { name: 'Waiting on your review' })
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
  await screen.findByRole('list', { name: 'Waiting on your review' })

  // Act
  await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Sort' }), 'By repository')

  // Assert
  expect(
    screen.getAllByRole('heading', { level: 3 }).map((heading) => heading.textContent),
  ).toEqual(['No repository', 'example/other', 'example/repo'])
  expect(
    linksIn(screen.getByRole('list', { name: 'Waiting on your review in example/repo' })),
  ).toEqual([oldestInRepo.url, newer.url])
  expect(
    linksIn(screen.getByRole('list', { name: 'Waiting on your review in No repository' })),
  ).toEqual([nowhere.url])
})

test('the order chosen holds across a refresh', async () => {
  // Arrange
  fakeApi({ [reviewsPath]: queueOf(older, newer) })
  renderWithClient(<ReviewQueuePanel />)
  await screen.findByRole('list', { name: 'Waiting on your review' })
  await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Sort' }), 'Newest first')

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Refresh' }))

  // Assert
  await screen.findByRole('button', { name: 'Refresh' })
  expect(sortChosen()).toBe('Newest first')
  expect(linksIn(screen.getByRole('list', { name: 'Waiting on your review' }))).toEqual([
    newer.url,
    older.url,
  ])
})

test("each repository's heading sits under the queue's own, a level up", async () => {
  // Arrange
  fakeApi({ [reviewsPath]: queueOf(oldestInRepo, older, nowhere, newer) })
  renderWithClient(<ReviewQueuePanel />)
  await screen.findByRole('list', { name: 'Waiting on your review' })

  // Act
  await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Sort' }), 'By repository')

  // Assert
  expect(screen.getByRole('heading', { level: 2, name: 'Waiting on your review' })).toBeTruthy()
  expect(screen.getAllByRole('heading').map((heading) => heading.textContent)).toEqual([
    'Waiting on your review',
    'No repository',
    'example/other',
    'example/repo',
  ])
})

test('a repository named as no repository is grouped apart from requests with none', async () => {
  // Arrange
  const named = inRepository('No repository', 'No repository', {
    number: 8,
    url: 'https://example.com/pull/8',
    opened_at: hoursAgo(5),
  })
  fakeApi({ [reviewsPath]: queueOf(nowhere, named) })
  renderWithClient(<ReviewQueuePanel />)
  await screen.findByRole('list', { name: 'Waiting on your review' })

  // Act
  await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Sort' }), 'By repository')

  // Assert
  expect(
    screen.getAllByRole('heading', { level: 3 }).map((heading) => heading.textContent),
  ).toEqual(['No repository', 'No repository'])
})
