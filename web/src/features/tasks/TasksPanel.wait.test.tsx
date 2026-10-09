import { act, screen } from '@testing-library/react'
import { vi } from 'vitest'
import type { Task } from '@/api/generated/types.gen.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { describedTask, makeTask, makeTaskList, taskFacet } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { TasksPanel } from './TasksPanel.tsx'

// A wait passing changes where a task stands with no write and no frame, so
// the page reads the list again once the earliest wait still ahead has passed.

const now = Date.parse('2026-10-05T12:00:00Z')
const hour = 3_600_000

// waitingUntil is a pending task with a wait still ahead, which the server
// reads as waiting, until the hours after now: task id, what it is, and the
// fields typed text matches, as the server gives them.
function waitingUntil(id: number, description: string, searchable: string[], hours: number): Task {
  return describedTask(
    {
      uuid: `waiting-${String(id)}`,
      id,
      description,
      status: 'pending',
      wait: new Date(now + hours * hour).toISOString(),
      project: '',
      priority: '',
      tags: [],
      issue_key: '',
      issue_url: '',
    },
    {
      state: 'waiting',
      facets: [
        taskFacet.waiting,
        taskFacet.noPriority,
        taskFacet.noProject,
        taskFacet.noIssue,
        taskFacet.noTag,
      ],
      searchable,
    },
  )
}

// readsOf counts the reads of the task list among the requests the page made.
function readsOf(requests: Request[]): number {
  return requests.filter((request) => new URL(request.url).pathname === '/api/tasks').length
}

test('the list is read again once the earliest wait still ahead has passed', async () => {
  // Arrange
  vi.useFakeTimers({ now, shouldAdvanceTime: true })
  const soon = waitingUntil(1, 'Book the room', ['book the room', '', '', '#1'], 1)
  const later = waitingUntil(2, 'Call the vendor', ['call the vendor', '', '', '#2'], 5)
  const requests = fakeApi({ '/api/tasks': makeTaskList([later, soon]) })
  renderWithClient(<TasksPanel />)
  await screen.findByText('2 waiting')

  // Act
  await act(async () => {
    await vi.advanceTimersByTimeAsync(hour + 2_000)
  })

  // Assert
  expect(readsOf(requests)).toBe(2)
})

test('a list with no wait ahead is not read again on its own', async () => {
  // Arrange
  vi.useFakeTimers({ now, shouldAdvanceTime: true })
  const requests = fakeApi({ '/api/tasks': makeTaskList([makeTask({ uuid: 'a' })]) })
  renderWithClient(<TasksPanel />)
  await screen.findByText(/1 task/)

  // Act
  await act(async () => {
    await vi.advanceTimersByTimeAsync(24 * hour)
  })

  // Assert
  expect(readsOf(requests)).toBe(1)
})
