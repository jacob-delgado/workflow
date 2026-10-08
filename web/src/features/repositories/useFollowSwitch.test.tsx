import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, render } from '@testing-library/react'
import { vi } from 'vitest'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { useUiStore, type IssueRef } from '@/shell/uiStore.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { useFollowSwitch } from './useFollowSwitch.ts'

// Following is a page that follows the server's switches.
function Following() {
  useFollowSwitch()

  return null
}

// jiraIssue is a Jira issue, as the server names its tracker.
const jiraIssue: IssueRef = { key: 'PROJ-1', tracker: 'jira' }

// following renders a page working in api with issue shown, and the client
// whose reads a switch resets.
function following(issue: IssueRef): QueryClient {
  const client = new QueryClient()
  useSnapshotStore.setState({ snapshot: makeSnapshot({ here: '/home/ana/src/api' }) })
  useUiStore.setState({ selectedIssue: issue })
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
  const client = following(jiraIssue)
  const reset = vi.spyOn(client, 'resetQueries')

  // Act
  switchedTo('/home/ana/src/web')

  // Assert
  expect(reset).toHaveBeenCalledTimes(1)
})

test.each<[string, IssueRef, IssueRef | null]>([
  ['a Jira issue, the same wherever you work', jiraIssue, jiraIssue],
  ["a forge issue, the repository left's", { key: '42', tracker: 'forge' }, null],
])('the issue shown stays when it is %s', (_, issue, kept) => {
  // Arrange
  following(issue)

  // Act
  switchedTo('/home/ana/src/web')

  // Assert
  expect(useUiStore.getState().selectedIssue).toBe(kept)
})

test('a snapshot of the same directory reads nothing again', () => {
  // Arrange
  const client = following(jiraIssue)
  const reset = vi.spyOn(client, 'resetQueries')

  // Act
  switchedTo('/home/ana/src/api')

  // Assert
  expect(reset).not.toHaveBeenCalled()
})
