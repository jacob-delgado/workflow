import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { ReviewPanel } from './ReviewPanel.tsx'

// A failed check says why it failed and the stage it ran in, and offers its
// log to read in the panel.

const pull = {
  number: 128,
  url: 'https://forge.example.com/pull/128',
  title: 'Redact tokens in the request log',
  state: 'open' as const,
  draft: false,
  approvals: 1,
  changes_requested: false,
  mergeable: 'clean' as const,
}

test('says why each failed check failed, and the stage it ran in', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      review: {
        found: true,
        announced: false,
        pull,
        ci: {
          state: 'failed',
          total: 0,
          done: 0,
          failed: 0,
          checks: [
            {
              name: 'unit-race',
              state: 'failed',
              url: 'https://gl/jobs/501',
              id: '501',
              stage: 'test',
              reason: 'script failure',
            },
          ],
        },
      },
    }),
  })

  // Act
  render(<ReviewPanel />)

  // Assert
  const row = screen.getByRole('listitem')
  expect(within(row).getByText('test')).toBeTruthy()
  expect(within(row).getByRole('link', { name: 'unit-race (opens in a new tab)' })).toBeTruthy()
  expect(row.textContent).toContain('script failure')
})

test("shows a failed check's log on demand", async () => {
  // Arrange
  const requests = fakeApi({
    '/api/review/checks/501/log': { text: '--- FAIL: TestRetry\n    got 4', truncated: true },
  })
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      review: {
        found: true,
        announced: false,
        pull,
        ci: {
          state: 'failed',
          total: 0,
          done: 0,
          failed: 0,
          checks: [{ name: 'unit-race', state: 'failed', url: '', id: '501', log_available: true }],
        },
      },
    }),
  })
  const user = userEvent.setup()

  // Act: draw the panel
  render(<ReviewPanel />)

  // Assert: no log is read before it is asked for
  expect(requests).toHaveLength(0)

  // Act: ask for the log
  await user.click(screen.getByRole('button', { name: 'Show log of unit-race' }))

  // Assert: it shows, and says its earlier lines are cut
  expect(await screen.findByText(/--- FAIL: TestRetry/)).toBeTruthy()
  expect(screen.getByText(/earlier lines are not shown/)).toBeTruthy()
})

// onFailedJob streams a pull request whose one check is a failed GitLab job,
// in the test stage, with a log to read.
function onFailedJob() {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      review: {
        found: true,
        announced: false,
        pull,
        ci: {
          state: 'failed',
          total: 1,
          done: 1,
          failed: 1,
          checks: [
            {
              name: 'unit-race',
              stage: 'test',
              state: 'failed',
              url: '',
              id: '501',
              log_available: true,
            },
          ],
        },
      },
    }),
  })
}

test('a log, once read, takes focus from the control that asked for it', async () => {
  // Arrange
  fakeApi({ '/api/review/checks/501/log': { text: '--- FAIL: TestRetry', truncated: false } })
  onFailedJob()
  const user = userEvent.setup()
  render(<ReviewPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Show log of unit-race' }))

  // Assert
  const log = await screen.findByRole('region', { name: 'Log of unit-race' })
  expect(document.activeElement).toBe(log)
  expect(log.textContent).toContain('--- FAIL: TestRetry')
})

test('while the log is read, the control is named by what it says', async () => {
  // Arrange
  fakeApi({ '/api/review/checks/501/log': () => new Promise(() => undefined) })
  onFailedJob()
  const user = userEvent.setup()
  render(<ReviewPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Show log of unit-race' }))

  // Assert
  expect(await screen.findByRole('button', { name: 'Reading the log of unit-race…' })).toBeTruthy()
})
