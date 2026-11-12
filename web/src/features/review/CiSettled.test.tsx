import { act, screen, waitFor } from '@testing-library/react'
import type { CiState, Config } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { mockConfig } from '@/dev/mockConfig.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { appQueryClient, renderWithClient } from '@/test/renderWithClient.tsx'
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
        announced: 'none',
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

// streamCiUnread has the stream push #128 with its CI unread: the read
// failed, with why, and no CI held for it.
function streamCiUnread() {
  streamCi('running')
  useSnapshotStore.setState((state) =>
    state.snapshot === null
      ? state
      : {
          snapshot: {
            ...state.snapshot,
            review: {
              ...state.snapshot.review,
              ci: null,
              ci_error: {
                type: 'about:blank',
                title: 'Bad Gateway',
                status: 502,
                detail: 'the forge did not answer',
                code: 'unreachable',
              },
            },
          },
        },
  )
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

test('says nothing of CI that settled while ui.notify was off once it is turned on', async () => {
  // Arrange
  let notify = false
  const requests = fakeApi({ '/api/config': () => configNotifying(notify) })
  const client = appQueryClient()
  streamCi('running')
  renderWithClient(<CiSettled />, client)
  act(() => {
    streamCi('passed')
  })
  await configRead(requests)
  notify = true

  // Act
  await act(async () => {
    await client.invalidateQueries()
    await new Promise((resolve) => setTimeout(resolve, 0))
  })

  // Assert
  expect(
    client
      .getQueryCache()
      .getAll()
      .map((query) => query.state.data),
  ).toContainEqual(expect.objectContaining({ config: configNotifying(true) }))
  expect(screen.getByRole('status').textContent).toBe('')
})

test('a frame whose CI could not be read between running and passed keeps the notice', async () => {
  // Arrange
  fakeApi({ '/api/config': configNotifying(true) })
  streamCi('running')
  renderWithClient(<CiSettled />)
  act(() => {
    streamCiUnread()
  })

  // Act
  act(() => {
    streamCi('passed')
  })

  // Assert
  await waitFor(() => {
    expect(screen.getByRole('status').textContent).toBe('CI passed on #128.')
  })
})
