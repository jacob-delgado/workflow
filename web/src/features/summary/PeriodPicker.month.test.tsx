import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import type { Activity } from '@/api/generated/types.gen.ts'
import { mockActivity } from '@/dev/mockActivity.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { PeriodCalendar } from './PeriodPicker.tsx'

// The calendar's own read of the month it shows: how each day is named once
// every source read the month whole, what it says when one could not or the
// read is refused, and which months it reads at all.

// today is the Wednesday the mockup's server says it is.
const today = '2026-09-16'

// The queries the calendar asks for the months it shows.
const september = '?from=2026-09-01&to=2026-09-30'
const august = '?from=2026-08-01&to=2026-08-31'

// readWhole is each source the mockup reads, every one read in full.
const readWhole: Activity['sources'] = mockActivity(null).sources.map((source) => ({
  ...source,
  state: 'read',
  truncated: false,
}))

// servingMonths answers each read of what was done with the mockup's for the
// period asked, from the sources given, and keeps each query asked.
function servingMonths(sources: Activity['sources']): string[] {
  const asked: string[] = []
  fakeApi({
    '/api/activity': (url: URL) => {
      asked.push(url.search)
      const from = url.searchParams.get('from') ?? today
      const to = url.searchParams.get('to') ?? from

      return { ...mockActivity({ from, to }), sources }
    },
  })

  return asked
}

// showCalendar draws the calendar on Tuesday the 15th.
function showCalendar() {
  renderWithClient(
    <PeriodCalendar
      period={{ from: '2026-09-15', to: '2026-09-15' }}
      today={today}
      onPick={vi.fn()}
    />,
  )
}

// expectNoReadPast fails the test if the calendar asks for more than the
// count of reads it has made, waiting a moment for one on its way.
async function expectNoReadPast(asked: string[], count: number): Promise<void> {
  await expect(
    waitFor(
      () => {
        expect(asked.length).toBeGreaterThan(count)
      },
      { timeout: 200 },
    ),
  ).rejects.toThrow()
}

test('a month every source read whole names each day with how much was done on it', async () => {
  // Arrange
  servingMonths(readWhole)

  // Act
  showCalendar()

  // Assert
  expect(
    await screen.findByRole('gridcell', { name: 'Tuesday, September 15, 2026, 4 things done' }),
  ).toBeTruthy()
  expect(screen.queryByRole('note')).toBeNull()
})

test.each([
  ['had more than the month shows', { state: 'read', truncated: true }],
  ['could not be read', { state: 'failed', truncated: false }],
] as const)(
  'a month in which a source %s names each day with at least what was read, and says why',
  async (_, jira) => {
    // Arrange
    servingMonths(
      readWhole.map((source) => (source.source === 'jira' ? { ...source, ...jira } : source)),
    )

    // Act
    showCalendar()

    // Assert
    expect(
      await screen.findByRole('gridcell', {
        name: 'Tuesday, September 15, 2026, at least 4 things done',
      }),
    ).toBeTruthy()
    expect(screen.getByRole('note').textContent).toBe(
      'Not every source could be read in full for September 2026, so a day may say less than was done on it.',
    )
  },
)

test('a month the server refuses to read says so, and names each day by its date alone', async () => {
  // Arrange
  fakeApi({
    '/api/activity': () =>
      Response.json(
        {
          type: 'about:blank',
          title: 'Upstream unreachable',
          status: 502,
          code: 'unreachable',
          detail: 'the request failed',
        },
        { status: 502 },
      ),
  })

  // Act
  showCalendar()

  // Assert
  expect((await screen.findByRole('note')).textContent).toBe(
    'September 2026 could not be read, so no day in it says what was done.',
  )
  expect(screen.getByRole('gridcell', { name: 'Tuesday, September 15, 2026' })).toBeTruthy()
})

test('a month after today is never read, since nothing was done in it yet', async () => {
  // Arrange
  const asked = servingMonths(readWhole)
  const user = userEvent.setup()
  showCalendar()
  const tuesday = await screen.findByRole('gridcell', {
    name: 'Tuesday, September 15, 2026, 4 things done',
  })
  tuesday.focus()

  // Act
  await user.keyboard('{PageDown}')

  // Assert
  expect(screen.getByRole('grid', { name: 'October 2026' })).toBeTruthy()
  await expectNoReadPast(asked, 1)
  expect(asked).toEqual([september])
})

test('a month shown again soon after is not read again', async () => {
  // Arrange
  const asked = servingMonths(readWhole)
  const user = userEvent.setup()
  showCalendar()
  const tuesday = await screen.findByRole('gridcell', {
    name: 'Tuesday, September 15, 2026, 4 things done',
  })
  tuesday.focus()
  await user.keyboard('{PageUp}')
  await screen.findByRole('gridcell', { name: 'Friday, August 14, 2026, 2 things done' })

  // Act
  await user.keyboard('{PageDown}')

  // Assert
  expect(
    await screen.findByRole('gridcell', { name: 'Tuesday, September 15, 2026, 4 things done' }),
  ).toBeTruthy()
  await expectNoReadPast(asked, 2)
  expect(asked).toEqual([september, august])
})
