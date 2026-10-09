import { screen, within } from '@testing-library/react'
import { fakeApi } from '@/test/fakeApi.ts'
import { makeTaskList } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { cacheTuning } from '@/test/tasks.ts'
import { TasksPanel } from './TasksPanel.tsx'

// Where the selected task's detail sends you for its issue: the page the
// server gives as the task's issue page, which is the tracker's.

const tasksPath = '/api/tasks'

test("the detail opens the tracker's page for the task's issue, in a new tab, whatever page its notes name", async () => {
  // Arrange
  // The server gives the tracker's page as the task's issue page; the note is
  // the page the task was tracked with, on a Jira since moved.
  const moved = {
    ...cacheTuning,
    annotations: [
      { entry: '2026-09-21T09:00:00Z', description: 'https://old.example.com/browse/PROJ-9' },
    ],
  }
  fakeApi({ [tasksPath]: makeTaskList([moved]) })

  // Act
  renderWithClient(<TasksPanel />)

  // Assert
  const open = await screen.findByRole('link', { name: 'Open PROJ-9 (opens in a new tab)' })
  expect(open.getAttribute('href')).toBe('https://jira.example.com/browse/PROJ-9')
  expect(open.getAttribute('target')).toBe('_blank')
  expect(open.getAttribute('rel')).toMatch(/\bnoopener\b/)
})

test('a task whose issue has no page offers no link to one', async () => {
  // Arrange
  // No tracker page, and no http(s) jiraurl on the task: the server gives none.
  fakeApi({ [tasksPath]: makeTaskList([{ ...cacheTuning, issue_url: '' }]) })

  // Act
  renderWithClient(<TasksPanel />)

  // Assert
  const detail = await screen.findByRole('article')
  expect(within(detail).getByRole('heading', { level: 2 }).textContent).toBe(
    cacheTuning.description,
  )
  expect(within(detail).queryByRole('link')).toBeNull()
  expect(detail.textContent).not.toContain('Open PROJ-9')
})
