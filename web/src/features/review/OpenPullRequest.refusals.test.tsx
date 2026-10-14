import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { openPr } from './openPrApi.ts'
import { ReviewPanel } from './ReviewPanel.tsx'

vi.mock('./openPrApi.ts', () => ({
  previewPullRequest: vi.fn(() =>
    Promise.resolve({
      title: 'fix: redact tokens',
      body: '',
      base: 'main',
      head: 'fix/PROJ-1',
      draft: false,
      needs_push: false,
      reviewers: [],
      templates: [],
      template: '',
    }),
  ),
  openPr: vi.fn(),
}))
const mockOpen = vi.mocked(openPr)

test.each([
  ['Title', 'Type a title first.'],
  ['Base branch', 'Name the branch to merge into first.'],
])('an emptied %s is refused beside it, and nothing is opened', async (label, said) => {
  // Arrange
  useSnapshotStore.setState({ status: 'live', snapshot: makeSnapshot() })
  const user = userEvent.setup()
  render(<ReviewPanel />)
  await user.click(screen.getByRole('button', { name: 'Open a pull request' }))
  const form = await screen.findByRole('form', { name: 'Open a pull request' })
  await user.clear(within(form).getByRole('textbox', { name: label }))

  // Act
  await user.click(within(form).getByRole('button', { name: /^Open/ }))

  // Assert
  const field = await within(form).findByRole('textbox', { name: label, description: said })
  expect(field.getAttribute('aria-invalid')).toBe('true')
  expect(mockOpen).not.toHaveBeenCalled()
})
