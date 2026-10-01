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

// The queue the terminal's facetsWorld lists, oldest first.
function requestNumbered(
  number: number,
  overrides: Parameters<typeof makeReviewRequest>[0],
): ReturnType<typeof makeReviewRequest> {
  return makeReviewRequest({
    number,
    url: `https://github.com/example/pull/${String(number)}`,
    ...overrides,
  })
}

const queue: ReviewQueue = {
  available: true,
  requests: [
    requestNumbered(5, {
      author: 'kwan',
      repository: 'example/repo',
      draft: true,
      ci: 'running',
      opened_at: hoursAgo(40),
    }),
    requestNumbered(12, {
      author: 'kwan',
      repository: 'example/other',
      ci: 'passed',
      opened_at: hoursAgo(26),
    }),
    requestNumbered(3, { author: 'mira', repository: '', ci: 'none', opened_at: hoursAgo(10) }),
    requestNumbered(7, {
      author: 'mira',
      repository: 'example/repo',
      ci: 'failed',
      opened_at: hoursAgo(3),
    }),
  ],
}

// listedNumbers is the number of each request the queue lists, in order.
function listedNumbers(): string[] {
  return within(screen.getByRole('list', { name: 'Review requests' }))
    .getAllByRole('link')
    .map((link) => link.getAttribute('href')?.split('/').at(-1) ?? '')
}

// press presses the filter's button for a value.
async function press(name: string) {
  const filter = screen.getByRole('group', { name: 'Filter' })
  await userEvent.click(within(filter).getByRole('button', { name }))
}

test('offers each value the queue holds, with its count, none pressed', async () => {
  // Arrange
  fakeApi({ [reviewsPath]: queue })

  // Act
  renderWithClient(<ReviewQueuePanel />)

  // Assert
  const filter = await screen.findByRole('group', { name: 'Filter' })
  const buttons = within(filter).getAllByRole('button')
  expect(buttons.map((button) => button.textContent)).toEqual([
    'no repository 1',
    'example/other 1',
    'example/repo 2',
    'CI failed 1',
    'CI passed 1',
    'CI running 1',
    'CI none 1',
    'draft 1',
    'ready 3',
    'by kwan 2',
    'by mira 2',
  ])
  expect(buttons.every((button) => button.getAttribute('aria-pressed') === 'false')).toBe(true)
})

test('a repository and a CI state narrow the queue together, and the summary says so', async () => {
  // Arrange
  fakeApi({ [reviewsPath]: queue })
  renderWithClient(<ReviewQueuePanel />)
  await screen.findByRole('group', { name: 'Filter' })

  // Act
  await press('example/repo 2')
  await press('CI failed 1')

  // Assert
  expect(listedNumbers()).toEqual(['7'])
  expect(
    screen
      .getAllByRole('status')
      .some(
        (line) =>
          line.textContent === '4 pull requests wait on your review, oldest first; 1 shown.',
      ),
  ).toBe(true)
})

test('two values in one facet widen the queue', async () => {
  // Arrange
  fakeApi({ [reviewsPath]: queue })
  renderWithClient(<ReviewQueuePanel />)
  await screen.findByRole('group', { name: 'Filter' })

  // Act
  await press('CI failed 1')
  await press('CI none 1')

  // Assert
  expect(listedNumbers()).toEqual(['3', '7'])
})

test('pressing a value again lets the queue out again', async () => {
  // Arrange
  fakeApi({ [reviewsPath]: queue })
  renderWithClient(<ReviewQueuePanel />)
  await screen.findByRole('group', { name: 'Filter' })
  await press('by kwan 2')

  // Act
  await press('by kwan 2')

  // Assert
  expect(listedNumbers()).toEqual(['5', '12', '3', '7'])
})

test('a filter nothing matches says so', async () => {
  // Arrange
  fakeApi({ [reviewsPath]: queue })
  renderWithClient(<ReviewQueuePanel />)
  await screen.findByRole('group', { name: 'Filter' })

  // Act
  await press('draft 1')
  await press('CI failed 1')

  // Assert
  expect(screen.getByText('No request matches the filters.')).toBeTruthy()
  expect(screen.queryByRole('list', { name: 'Review requests' })).toBeNull()
})

test('the filter holds when the order changes', async () => {
  // Arrange
  fakeApi({ [reviewsPath]: queue })
  renderWithClient(<ReviewQueuePanel />)
  await screen.findByRole('group', { name: 'Filter' })
  await press('by kwan 2')

  // Act
  await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Sort' }), 'Newest first')

  // Assert
  expect(listedNumbers()).toEqual(['12', '5'])
})
