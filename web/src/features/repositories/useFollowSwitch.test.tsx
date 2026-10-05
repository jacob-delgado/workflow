import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, render } from '@testing-library/react'
import { vi } from 'vitest'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { useUiStore } from '@/shell/uiStore.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { useFollowSwitch } from './useFollowSwitch.ts'

// Following is a page that follows the server's switches.
function Following() {
  useFollowSwitch()

  return null
}

// following renders a page working in api with key's issue shown, and the
// client whose reads a switch resets.
function following(key: string): QueryClient {
  const client = new QueryClient()
  useSnapshotStore.setState({ snapshot: makeSnapshot({ here: '/home/ana/src/api' }) })
  useUiStore.setState({ selectedIssue: key })
  render(
    <QueryClientProvider client={client}>
      <Following />
    </QueryClientProvider>,
  )

  return client
}

// switchedTo has the stream say the server works in dir now.
function switchedTo(dir: string) {
  act(() => {
    useSnapshotStore.setState({ snapshot: makeSnapshot({ here: dir }) })
  })
}

test('a switch made elsewhere reads every section again', () => {
  // Arrange
  const client = following('PROJ-1')
  const reset = vi.spyOn(client, 'resetQueries')

  // Act
  switchedTo('/home/ana/src/web')

  // Assert
  expect(reset).toHaveBeenCalledTimes(1)
})

test.each([
  ['a Jira issue, the same wherever you work', 'PROJ-1', 'PROJ-1'],
  ["a forge issue, the repository left's", '#42', null],
])('the issue shown stays when it is %s', (_, key, kept) => {
  // Arrange
  following(key)

  // Act
  switchedTo('/home/ana/src/web')

  // Assert
  expect(useUiStore.getState().selectedIssue).toBe(kept)
})

test('a snapshot of the same directory reads nothing again', () => {
  // Arrange
  const client = following('PROJ-1')
  const reset = vi.spyOn(client, 'resetQueries')

  // Act
  switchedTo('/home/ana/src/api')

  // Assert
  expect(reset).not.toHaveBeenCalled()
})
