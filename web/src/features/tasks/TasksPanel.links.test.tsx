import { screen, within } from '@testing-library/react'
import { fakeApi } from '@/test/fakeApi.ts'
import { makeTask, makeTaskList } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { TasksPanel } from './TasksPanel.tsx'

// Where the selected task's detail sends you for its issue: the page the
// server gives as the task's issue page, which is the tracker's.

const tasksPath = '/api/tasks'

const cache = makeTask({
  uuid: '33333333-3333-4333-8333-333333333333',
  id: 3,
  description: 'PROJ-9: Tune the cache',
  issue_key: 'PROJ-9',
  issue_url: 'https://jira.example.com/browse/PROJ-9',
})

test("the detail opens the tracker's page for the task's issue, in a new tab, whatever page its notes name", async () => {
  // Arrange
  // The server gives the tracker's page as the task's issue page; the note is
  // the page the task was tracked with, on a Jira since moved.
  const moved = {
    ...cache,
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
  fakeApi({ [tasksPath]: makeTaskList([{ ...cache, issue_url: '' }]) })

  // Act
  renderWithClient(<TasksPanel />)

  // Assert
  const detail = await screen.findByRole('article')
  expect(within(detail).getByRole('heading', { level: 2 }).textContent).toBe(cache.description)
  expect(within(detail).queryByRole('link')).toBeNull()
  expect(detail.textContent).not.toContain('Open PROJ-9')
})
