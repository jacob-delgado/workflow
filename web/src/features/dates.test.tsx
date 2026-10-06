import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { Activity, IssueDetail } from '@/api/generated/types.gen.ts'
import { IssueDetailPanel } from '@/features/issues/IssueDetailPanel.tsx'
import { SummaryPanel } from '@/features/summary/SummaryPanel.tsx'
import { TasksPanel } from '@/features/tasks/TasksPanel.tsx'
import { fakeApi } from '@/test/fakeApi.ts'
import { makeTask, makeTaskList } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'

// One day is written one way wherever the web writes it: a comment, a task's
// due date and its notes, and the Summary.

const sameDay = '2025-03-14T12:00:00Z'

// commented is PROJ-1 with one comment written on sameDay.
function commented(): IssueDetail {
  return {
    key: 'PROJ-1',
    tracker: 'jira',
    summary: 'Fix the token leak',
    status: 'In Progress',
    status_category: 'indeterminate',
    type: 'Bug',
    reporter: 'Ana Lopez',
    description: '',
    comments: [{ author: 'Sam Ortiz', body: 'Repro’d.', created: sameDay }],
    comment_total: 1,
    url: '',
  }
}

// oneDay is a Summary of Tuesday 15 September 2026 with one commit.
function oneDay(): Activity {
  return {
    from: '2026-09-15',
    to: '2026-09-15',
    today: '2026-09-16',
    sources: [],
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

test('an old comment gives the date in running text', async () => {
  // Arrange
  fakeApi({ '/api/issues/PROJ-1': commented() })

  // Act
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)

  // Assert
  const thread = await screen.findByRole('list', { name: 'Comments' })
  expect(within(thread).getByRole('time').textContent).toBe('Mar 14, 2025')
})

test('a task’s due date and its note give the date as a comment does', async () => {
  // Arrange
  const task = makeTask({
    due: sameDay,
    annotations: [{ entry: sameDay, description: 'Ana can review it' }],
  })
  fakeApi({ '/api/tasks': makeTaskList([task]) })

  // Act
  renderWithClient(<TasksPanel />)

  // Assert
  const detail = await screen.findByRole('article')
  const dates = within(detail)
    .getAllByRole('time')
    .map((time) => time.textContent)
  expect(dates).toEqual(['Mar 14, 2025', 'Mar 14, 2025'])
})

test('a one-day Summary heads the day once', async () => {
  // Arrange
  fakeApi({ '/api/activity': oneDay() })

  // Act
  renderWithClient(<SummaryPanel />)

  // Assert
  await screen.findByRole('list', { name: 'Tuesday, September 15, 2026' })
  const headings = screen
    .getAllByRole('heading')
    .map((heading) => heading.textContent)
    .filter((text) => /September|Tuesday/.test(text))
  expect(headings).toEqual(['Tuesday, September 15, 2026'])
})

test('the Summary’s copy names its day as running text does', async () => {
  // Arrange
  fakeApi({ '/api/activity': oneDay() })
  const user = userEvent.setup()
  renderWithClient(<SummaryPanel />)

  // Act
  await user.click(await screen.findByRole('button', { name: 'Copy as Markdown' }))

  // Assert
  expect(await screen.findByText('Copied the summary of Sep 15, 2026.')).toBeTruthy()
})
