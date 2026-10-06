import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import type { PullRequestDraft } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { previewPullRequest } from './openPrApi.ts'
import { ReviewPanel } from './ReviewPanel.tsx'

vi.mock('./openPrApi.ts', () => ({ previewPullRequest: vi.fn(), openPr: vi.fn() }))
const mockPreview = vi.mocked(previewPullRequest)

// draftFor is the pull request the server composes for a branch, pushed to its
// remote or not.
function draftFor({ pushed }: { pushed: boolean }): PullRequestDraft {
  return {
    title: 'fix: redact tokens',
    body: 'why',
    base: 'main',
    head: 'fix/PROJ-412',
    draft: false,
    needs_push: !pushed,
    reviewers: [],
    templates: [],
    template: '',
  }
}

// pushFirst is what the form says of a branch the open will push first.
const pushFirst = 'The branch is not pushed yet; opening will push it first.'

// composeThePullRequest opens the form for a branch with no pull request yet.
async function composeThePullRequest() {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ review: { found: false, announced: false } }),
  })
  render(<ReviewPanel />)
  await userEvent.setup().click(screen.getByRole('button', { name: 'Open a pull request' }))
  await screen.findByRole('form', { name: 'Open a pull request' })
}

test('says the open will push a branch that is not pushed yet', async () => {
  // Arrange
  mockPreview.mockResolvedValueOnce(draftFor({ pushed: false }))

  // Act
  await composeThePullRequest()

  // Assert
  expect(screen.getByText(pushFirst)).toBeTruthy()
})

test('says nothing of a push for a branch already pushed', async () => {
  // Arrange
  mockPreview.mockResolvedValueOnce(draftFor({ pushed: true }))

  // Act
  await composeThePullRequest()

  // Assert
  expect(screen.queryByText(pushFirst)).toBeNull()
})
