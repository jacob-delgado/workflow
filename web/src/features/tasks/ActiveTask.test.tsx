import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import type { TasksSummary } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { NavRail } from '@/shell/NavRail.tsx'
import { makeSnapshot, makeTask } from '@/test/fixtures.ts'
import { drawnMark, markShape } from '@/test/marks.tsx'
import { ActiveTask } from './ActiveTask.tsx'

const started = makeTask({
  description: 'PROJ-412: Redact tokens before they reach the request log',
  issue_key: 'PROJ-412',
  start: new Date(Date.now() - 72 * 60_000).toISOString(),
  state: 'started',
})

// streamTasks puts a stream frame on screen whose task summary is summary.
function streamTasks(summary: TasksSummary) {
  useSnapshotStore.setState({ status: 'live', snapshot: makeSnapshot({ tasks: summary }) })
}

test('the header shows the active task, how long it has run, and opens the Tasks section', async () => {
  // Arrange
  const user = userEvent.setup()
  streamTasks({ available: true, reason: '', active: started, linked: [started] })
  render(
    <>
      <ActiveTask />
      <NavRail />
    </>,
  )
  const chip = screen.getByRole('button', { name: /active task/i })

  // Act
  await user.click(chip)

  // Assert
  expect(chip.textContent).toContain('PROJ-412: Redact tokens before they reach the request log')
  expect(chip.textContent).toContain('1h12m')
  expect(markShape(chip)).toBe(drawnMark('in-flight'))
  expect(screen.getByRole('button', { name: 'Tasks', current: 'page' })).toBeTruthy()
})

test.each([
  ['no task is started', { available: true, reason: '', linked: [started] }],
  [
    'Taskwarrior is unavailable',
    {
      available: false,
      reason: 'Turned off by taskwarrior.disabled.',
      active: started,
      linked: [],
    },
  ],
])('nothing is drawn when %s', (_, summary: TasksSummary) => {
  // Arrange
  streamTasks(summary)

  // Act
  render(<ActiveTask />)

  // Assert
  expect(screen.queryByRole('button')).toBeNull()
})

test('nothing is drawn before the first snapshot lands', () => {
  // Act
  render(<ActiveTask />)

  // Assert
  expect(screen.queryByRole('button')).toBeNull()
})

test('the time the task has run moves on with the clock', () => {
  // Arrange
  vi.useFakeTimers({ now: Date.parse('2026-09-28T12:00:00Z') })
  const running = { ...started, start: '2026-09-28T10:48:00Z' }
  streamTasks({ available: true, reason: '', active: running, linked: [running] })
  render(<ActiveTask />)
  const chip = screen.getByRole('button', { name: /active task/i })

  // Act
  act(() => {
    vi.advanceTimersByTime(60_000)
  })

  // Assert
  expect(chip.textContent).toContain('1h13m')
})

test('the time a task has run is counted from when the chip appears, not when the page opened', () => {
  // Arrange
  // The page opens with no task started; one is started 50 seconds later, and
  // the stream says it has run 72 minutes (a task started elsewhere).
  vi.useFakeTimers({ now: Date.parse('2026-09-28T12:00:00Z') })
  streamTasks({ available: true, reason: '', linked: [] })
  render(<ActiveTask />)
  act(() => {
    vi.advanceTimersByTime(50_000)
  })
  const running = { ...started, start: '2026-09-28T10:48:50Z' }

  // Act
  act(() => {
    streamTasks({ available: true, reason: '', active: running, linked: [running] })
  })

  // Assert
  expect(screen.getByRole('button', { name: /active task/i }).textContent).toContain('1h12m')
})

test('a task started in place of another is counted from when it took its place', () => {
  // Arrange
  // Task 12 runs as the page opens; 50 seconds later task 13 has taken its
  // place, and the stream says it has run 72 minutes.
  vi.useFakeTimers({ now: Date.parse('2026-09-28T12:00:00Z') })
  const first = { ...started, start: '2026-09-28T11:00:00Z' }
  streamTasks({ available: true, reason: '', active: first, linked: [first] })
  render(<ActiveTask />)
  act(() => {
    vi.advanceTimersByTime(50_000)
  })
  const second = makeTask({
    uuid: '6a7b8c9d-0e1f-4a2b-8c3d-4e5f6a7b8c9d',
    id: 13,
    description: 'Renew the certificate',
    issue_key: '',
    start: '2026-09-28T10:48:50Z',
    state: 'started',
  })

  // Act
  act(() => {
    streamTasks({ available: true, reason: '', active: second, linked: [] })
  })

  // Assert
  expect(screen.getByRole('button', { name: /active task/i }).textContent).toContain('1h12m')
})
