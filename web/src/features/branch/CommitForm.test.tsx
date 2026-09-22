import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import type { Config } from '@/api/generated/types.gen.ts'
import { useConfig } from '@/features/settings/configApi.ts'
import { commitChanges } from './commitApi.ts'
import { CommitForm } from './CommitForm.tsx'

vi.mock('./commitApi.ts', () => ({ commitChanges: vi.fn(() => Promise.resolve()) }))
vi.mock('@/features/settings/configApi.ts', () => ({ useConfig: vi.fn() }))
const mockConfig = vi.mocked(useConfig)
const mockCommit = vi.mocked(commitChanges)

// configResult stubs useConfig's query result down to the data the form reads.
function configResult(data: Partial<Config> | undefined) {
  return { data } as unknown as ReturnType<typeof useConfig>
}

test('offers the built-in commit types when none are configured', () => {
  // Arrange
  mockConfig.mockReturnValue(configResult(undefined))

  // Act
  render(<CommitForm />)

  // Assert
  expect(screen.getByRole('option', { name: 'feat' })).toBeTruthy()
  expect(screen.getByRole('option', { name: 'fix' })).toBeTruthy()
})

test("offers a team's configured commit types instead of the built-in set", async () => {
  // Arrange
  // The team commits only hotfixes and chores, and never "feat" or "fix".
  mockConfig.mockReturnValue(configResult({ commit: { types: ['hotfix', 'chore'] } }))

  // Act
  render(<CommitForm />)

  // Assert
  expect(screen.getByRole('option', { name: 'hotfix' })).toBeTruthy()
  expect(screen.queryByRole('option', { name: 'feat' })).toBeNull()

  // The default "fix" is not among them, so the chosen type is corrected to one
  // the server will accept.
  await waitFor(() => {
    expect(screen.getByRole<HTMLSelectElement>('combobox').value).toBe('hotfix')
  })
})

test('keeps a configured type after a commit for a team that excludes fix', async () => {
  // Arrange
  // The team's types are hotfix/chore; after the first commit the form must not
  // revert its type to the built-in "fix" default the server would reject.
  mockConfig.mockReturnValue(configResult({ commit: { types: ['hotfix', 'chore'] } }))
  const user = userEvent.setup()
  render(<CommitForm />)

  // Act: commit once, then commit again without touching the Type dropdown
  await user.type(screen.getByLabelText('Subject'), 'first change')
  await user.click(screen.getByRole('button', { name: /commit staged changes/i }))
  await user.type(await screen.findByLabelText('Subject'), 'second change')
  await user.click(screen.getByRole('button', { name: /commit staged changes/i }))

  // Assert
  expect(mockCommit).toHaveBeenLastCalledWith(expect.objectContaining({ type: 'hotfix' }))
})
