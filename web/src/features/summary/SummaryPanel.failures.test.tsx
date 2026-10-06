import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import type { Activity } from '@/api/generated/types.gen.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { SummaryPanel } from './SummaryPanel.tsx'

// sources is one source of each kind a day can say: one read, one that could
// not be, and one that had more than it shows.
const sources: Activity['sources'] = [
  { source: 'git', name: 'Git', failed: false, truncated: false, detail: '' },
  {
    source: 'forge',
    name: 'The forge',
    failed: true,
    truncated: false,
    detail: 'the forge could not be reached',
  },
  { source: 'jira', name: 'Jira', failed: false, truncated: true, detail: '' },
]

// quietDay is a day with one commit, read from every source but the forge.
function quietDay(): Activity {
  return {
    from: '2026-09-15',
    to: '2026-09-15',
    today: '2026-09-16',
    sources,
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
                ],
              },
            ],
          },
        ],
      },
    ],
    text: '# 2026-09-15\n',
  }
}

// refusedOnce refuses the first read with a 500 and answers every later one.
function refusedOnce(): void {
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
        : quietDay()
    },
  })
}

test('a first read that fails is announced as an alert', async () => {
  // Arrange
  refusedOnce()

  // Act
  renderWithClient(<SummaryPanel />)

  // Assert
  expect((await screen.findByRole('alert')).textContent).toBe('the request failed')
})

test('a first read that fails reads again on Try again', async () => {
  // Arrange
  refusedOnce()
  const user = userEvent.setup()
  renderWithClient(<SummaryPanel />)

  // Act
  await user.click(await screen.findByRole('button', { name: 'Try again' }))

  // Assert
  expect(await screen.findByRole('list', { name: 'Tuesday, September 15, 2026' })).toBeTruthy()
})

test('a source that could not be read is an alert, and one cut short a note', async () => {
  // Arrange
  fakeApi({ '/api/activity': quietDay() })

  // Act
  renderWithClient(<SummaryPanel />)

  // Assert
  expect((await screen.findByRole('alert')).textContent).toBe(
    'The forge could not be read: the forge could not be reached',
  )
  expect(screen.getByRole('note').textContent).toBe('Jira had more than this shows.')
})

test('a copy that fails is announced as an alert', async () => {
  // Arrange
  fakeApi({ '/api/activity': { ...quietDay(), sources: [] } })
  const user = userEvent.setup()
  vi.spyOn(navigator.clipboard, 'writeText').mockRejectedValue(new Error('denied'))
  renderWithClient(<SummaryPanel />)
  const copy = await screen.findByRole('button', { name: 'Copy as Markdown' })

  // Act
  await user.click(copy)

  // Assert
  expect((await screen.findByRole('alert')).textContent).toBe('The summary could not be copied.')
})
