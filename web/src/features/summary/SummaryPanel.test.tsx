import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { Activity } from '@/api/generated/types.gen.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { SummaryPanel } from './SummaryPanel.tsx'

// tuesday is what the server says was done on Tuesday 15 September 2026: a
// commit at nine and a pull request at three, with the forge's reviews unread.
function tuesday(from = '2026-09-15', to = '2026-09-15'): Activity {
  return {
    from,
    to,
    today: '2026-09-16',
    sources: [
      { source: 'git', name: 'Git', failed: false, truncated: false, detail: '' },
      {
        source: 'forge',
        name: 'The forge',
        failed: true,
        truncated: false,
        detail: 'the forge could not be reached; check the network, then try again',
      },
    ],
    years: [
      {
        year: 2026,
        months: [
          {
            month: 9,
            name: 'September',
            days: [
              {
                date: '2026-09-15',
                weekday: 'Tuesday',
                hours: [
                  {
                    label: '09:00',
                    items: [
                      {
                        at: '2026-09-15T09:30:00Z',
                        source: 'git',
                        verb: 'committed',
                        ref: 'abc1234',
                        title: 'Fix the token leak',
                        url: '',
                        repository: '',
                      },
                    ],
                  },
                  {
                    label: '15:00',
                    items: [
                      {
                        at: '2026-09-15T15:10:00Z',
                        source: 'jira',
                        verb: 'moved',
                        ref: 'PROJ-412',
                        title: 'Fix token redaction, to In Review',
                        url: 'https://jira.example.com/browse/PROJ-412',
                        repository: '',
                      },
                    ],
                  },
                ],
              },
            ],
          },
        ],
      },
    ],
    text: '# 2026-09-15\n\n- committed abc1234 Fix the token leak\n',
  }
}

// servingActivity answers every read with what answer gives for its query,
// and keeps each query asked.
function servingActivity(answer: (from: string | null, to: string | null) => Activity) {
  const asked: string[] = []
  fakeApi({
    '/api/activity': (url: URL) => {
      asked.push(url.search)

      return answer(url.searchParams.get('from'), url.searchParams.get('to'))
    },
  })

  return asked
}

test('the day reads as a timeline, an hour to a line, each source named', async () => {
  // Arrange
  servingActivity(() => tuesday())

  // Act
  renderWithClient(<SummaryPanel />)

  // Assert
  const day = await screen.findByRole('list', { name: 'Tuesday, September 15, 2026' })
  expect(within(day).getByRole('heading', { name: '09:00' })).toBeTruthy()
  expect(within(day).getByText('Fix the token leak')).toBeTruthy()
  expect(within(day).getByRole('link', { name: 'PROJ-412' }).getAttribute('href')).toBe(
    'https://jira.example.com/browse/PROJ-412',
  )
})

test('a source that could not be read says so, in the server’s words', async () => {
  // Arrange
  servingActivity(() => tuesday())

  // Act
  renderWithClient(<SummaryPanel />)

  // Assert
  expect(
    await screen.findByText(
      'The forge could not be read: the forge could not be reached; check the network, then try again',
    ),
  ).toBeTruthy()
})

test('Copy as Markdown puts the summary on the clipboard and says so', async () => {
  // Arrange
  servingActivity(() => tuesday())
  const user = userEvent.setup()
  renderWithClient(<SummaryPanel />)
  const copy = await screen.findByRole('button', { name: 'Copy as Markdown' })

  // Act
  await user.click(copy)

  // Assert
  expect(await navigator.clipboard.readText()).toBe(tuesday().text)
  expect(await screen.findByText('Copied the summary of Sep 15, 2026.')).toBeTruthy()
})

test('Earlier reads the period before, a period’s length back', async () => {
  // Arrange
  const asked = servingActivity((from, to) => tuesday(from ?? '2026-09-15', to ?? '2026-09-15'))
  const user = userEvent.setup()
  renderWithClient(<SummaryPanel />)
  const earlier = await screen.findByRole('button', { name: 'Earlier' })

  // Act
  await user.click(earlier)

  // Assert
  await waitFor(() => {
    expect(asked.at(-1)).toBe('?from=2026-09-14&to=2026-09-14')
  })
})

test('Later is off once the period reaches today', async () => {
  // Arrange
  servingActivity(() => tuesday('2026-09-16', '2026-09-16'))

  // Act
  renderWithClient(<SummaryPanel />)

  // Assert
  const later = await screen.findByRole('button', { name: 'Later' })
  expect(later.getAttribute('aria-disabled')).toBe('true')
})

test('a period with nothing done invites another', async () => {
  // Arrange
  servingActivity(() => ({ ...tuesday(), years: [], sources: [] }))

  // Act
  renderWithClient(<SummaryPanel />)

  // Assert
  expect(
    await screen.findByText('Nothing done in this period. Pick another day or range.'),
  ).toBeTruthy()
  expect(
    screen.getByRole('button', { name: 'Copy as Markdown' }).getAttribute('aria-disabled'),
  ).toBe('true')
})

test('a first read that fails says why and reads again on Try again', async () => {
  // Arrange
  let answers = 0
  fakeApi({
    '/api/activity': () => {
      answers++

      return answers === 1
        ? Response.json(
            {
              type: 'about:blank',
              title: 'Internal',
              status: 500,
              code: 'internal',
              detail: 'the request failed',
            },
            { status: 500 },
          )
        : tuesday()
    },
  })
  renderWithClient(<SummaryPanel />)
  const retry = await screen.findByRole('button', { name: 'Try again' })

  // Act
  await userEvent.setup().click(retry)

  // Assert
  expect(await screen.findByRole('list', { name: 'Tuesday, September 15, 2026' })).toBeTruthy()
})

test('a period the server refuses keeps the steps and says why', async () => {
  // Arrange
  fakeApi({
    '/api/activity': (url: URL) =>
      url.searchParams.get('from') === '2026-09-14'
        ? Response.json(
            {
              type: 'about:blank',
              title: 'Unprocessable',
              status: 422,
              code: 'unprocessable',
              detail: 'the period could not be read: the period is longer than a year and a day',
            },
            { status: 422 },
          )
        : tuesday(),
  })
  renderWithClient(<SummaryPanel />)
  const earlier = await screen.findByRole('button', { name: 'Earlier' })

  // Act
  await userEvent.setup().click(earlier)

  // Assert
  expect(await screen.findByText(/longer than a year and a day/)).toBeTruthy()
  expect(screen.getByRole('heading', { level: 2, name: 'Monday, September 14, 2026' })).toBeTruthy()
  expect(screen.getByRole('button', { name: 'Later' })).toBeTruthy()
})
