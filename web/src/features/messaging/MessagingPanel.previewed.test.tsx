import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { PullRequest } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { MessagingPanel } from './MessagingPanel.tsx'

// The server posts only the text a preview showed, refusing one that changed
// since, so the post carries it.

const pull: PullRequest = {
  number: 7,
  url: 'https://forge.example.com/pull/7',
  title: 'Redact tokens in the request log',
  state: 'open',
  draft: false,
  approvals: 0,
  changes_requested: false,
  mergeable: 'clean',
}

const previewed =
  'octocat opened a pull request: Redact tokens in the request log\nhttps://forge.example.com/pull/7'

test('the post carries the text its preview showed', async () => {
  // Arrange
  const requests = fakeApi({
    '/api/announcement': { text: previewed, channel: '#dev' },
    '/api/announce': { text: previewed, channel: '#dev' },
  })
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ review: { found: true, pull } }),
  })
  const user = userEvent.setup()
  render(<MessagingPanel />)
  await user.click(screen.getByRole('button', { name: 'Announce to Slack' }))

  // Act
  await user.click(await screen.findByRole('button', { name: 'Announce now' }))

  // Assert
  await screen.findByText('Announced to #dev.')
  const posted = requests.find((request) => new URL(request.url).pathname === '/api/announce')
  expect(await posted?.json()).toEqual({ channel: '#dev', text: previewed })
})
