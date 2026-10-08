import { act, screen } from '@testing-library/react'
import { vi } from 'vitest'
import { fakeApi } from '@/test/fakeApi.ts'
import { makeTask, makeTaskList } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { TasksPanel } from './TasksPanel.tsx'

// A wait passing changes where a task stands with no write and no frame, so
// the page reads the list again once the earliest wait still ahead has passed.

const now = Date.parse('2026-10-05T12:00:00Z')
const hour = 3_600_000

// readsOf counts the reads of the task list among the requests the page made.
function readsOf(requests: Request[]): number {
  return requests.filter((request) => new URL(request.url).pathname === '/api/tasks').length
}

test('the list is read again once the earliest wait still ahead has passed', async () => {
  // Arrange
  vi.useFakeTimers({ now, shouldAdvanceTime: true })
  const soon = makeTask({ uuid: 'a', state: 'waiting', wait: new Date(now + hour).toISOString() })
  const later = makeTask({
    uuid: 'b',
    state: 'waiting',
    wait: new Date(now + 5 * hour).toISOString(),
  })
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
