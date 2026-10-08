import { act, screen, waitFor } from '@testing-library/react'
import type { CiState, Config } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { mockConfig } from '@/dev/mockConfig.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { CiSettled } from './CiSettled.tsx'

// When CI on the branch's pull request settles, the page says so, as the
// terminal rings when ui.notify asks it to.

// configNotifying is the configuration with ui.notify as given.
function configNotifying(notify: boolean): Config {
  return { ...mockConfig, ui: { ...mockConfig.ui, notify } }
}

// streamCi has the stream push #128 with its CI in the state given.
function streamCi(state: CiState, number = 128) {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      review: {
        found: true,
        announced: false,
        pull: {
          number,
          url: `https://forge.example.com/pull/${String(number)}`,
          title: 'Redact tokens in the request log',
          state: 'open',
          draft: false,
          approvals: 0,
          changes_requested: false,
          mergeable: 'clean',
        },
        ci: { state, total: 0, done: 0, failed: 0, checks: [] },
      },
    }),
  })
}

// configRead waits for the configuration to be read, and what it decides to be
// drawn.
async function configRead(requests: Request[]) {
  await waitFor(() => {
    expect(requests.some((request) => request.url.endsWith('/api/config'))).toBe(true)
  })
  await act(async () => {
    await Promise.resolve()
  })
}

test.each([
  ['passed', 'CI passed on #128.'],
  ['failed', 'CI failed on #128.'],
] as const)('a snapshot that flips CI from running to %s says so', async (settled, words) => {
  // Arrange
  fakeApi({ '/api/config': configNotifying(true) })
  streamCi('running')
  renderWithClient(<CiSettled />)

  // Act
  act(() => {
    streamCi(settled)
  })

  // Assert
  await waitFor(() => {
    expect(screen.getByRole('status').textContent).toBe(words)
  })
})

test('says nothing when ui.notify is off', async () => {
  // Arrange
  const requests = fakeApi({ '/api/config': configNotifying(false) })
  streamCi('running')
  renderWithClient(<CiSettled />)

  // Act
  act(() => {
    streamCi('passed')
  })

  // Assert
  await configRead(requests)
  expect(screen.getByRole('status').textContent).toBe('')
})

test('says nothing of CI the page first sees already settled', async () => {
  // Arrange
  const requests = fakeApi({ '/api/config': configNotifying(true) })
  streamCi('passed')

  // Act
  renderWithClient(<CiSettled />)

  // Assert
  await configRead(requests)
  expect(screen.getByRole('status').textContent).toBe('')
})

test('says nothing of another pull request whose CI is settled', async () => {
  // Arrange
  const requests = fakeApi({ '/api/config': configNotifying(true) })
  streamCi('running', 128)
  renderWithClient(<CiSettled />)

  // Act
  act(() => {
    streamCi('passed', 129)
  })

  // Assert
  await configRead(requests)
  expect(screen.getByRole('status').textContent).toBe('')
})
