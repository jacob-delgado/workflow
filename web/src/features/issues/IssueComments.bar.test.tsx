import { screen, within } from '@testing-library/react'
import type { IssueDetail } from '@/api/generated/types.gen.ts'
import { mockConfig } from '@/dev/mockConfig.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { IssueDetailPanel } from './IssueDetailPanel.tsx'

// The comment box of a Jira issue is Markdown or wiki markup as the
// configuration says, so it waits for that read: it draws the shared Reading
// status, then the box in its settled shape, never a box that grows its bar.

const detail: IssueDetail = {
  key: 'PROJ-1',
  tracker: 'jira',
  summary: 'Fix the token leak',
  status: 'In Progress',
  status_category: 'indeterminate',
  type: 'Bug',
  reporter: 'Ana Lopez',
  description: '',
  comments: [],
  comment_total: 0,
  url: '',
}

// configLater serves PROJ-1 at once and the configuration only once release
// is called.
function configLater(): () => void {
  let release = () => {}
  const held = new Promise((resolve) => {
    release = () => {
      resolve({ ...mockConfig, jira: { ...mockConfig.jira, markdown_comments: true } })
    }
  })
  fakeApi({ '/api/issues/PROJ-1': detail, '/api/config': () => held })

  return release
}

test('says it is reading how comments are written, and draws no box yet', async () => {
  // Arrange
  configLater()

  // Act
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)

  // Assert
  const comments = await screen.findByRole('region', { name: 'Comments' })
  expect(within(comments).getByRole('status').textContent).toMatch(/^Reading /)
  expect(within(comments).queryByRole('textbox', { name: 'Comment on PROJ-1' })).toBeNull()
})

test('draws the box with its Markdown bar at once when the read lands', async () => {
  // Arrange
  const release = configLater()
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)
  await screen.findByRole('region', { name: 'Comments' })

  // Act
  release()

  // Assert
  await screen.findByRole('textbox', { name: 'Comment on PROJ-1' })
  expect(screen.getByRole('tab', { name: 'Preview' })).toBeTruthy()
})
