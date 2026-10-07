import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { ReviewPanel } from './ReviewPanel.tsx'

// The branch's pull request offers its URL to copy, as the review queue's rows
// and the terminal's link keys do.

const url = 'https://forge.example.com/pull/128'

// showingTheBranchPull puts #128, open, on the branch's review.
function showingTheBranchPull() {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      review: {
        found: true,
        announced: false,
        pull: {
          number: 128,
          url,
          title: 'Redact tokens in the request log',
          state: 'open',
          draft: false,
          approvals: 1,
          changes_requested: false,
          mergeable: 'clean',
        },
      },
    }),
  })
}

test('Copy URL puts the pull request on the clipboard and says so beside its title', async () => {
  // Arrange
  const user = userEvent.setup()
  showingTheBranchPull()
  render(<ReviewPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Copy URL to #128' }))

  // Assert
  expect(await navigator.clipboard.readText()).toBe(url)
  const pullRequest = screen.getByRole('region', { name: /Redact tokens in the request log/ })
  expect(await within(pullRequest).findByText('Copied the URL of #128.')).toBeTruthy()
})
