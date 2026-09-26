import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import type { Config } from '@/api/generated/types.gen.ts'
import { useConfig } from '@/features/settings/configApi.ts'
import { makeBranch } from '@/test/fixtures.ts'
import { commitChanges } from './commitApi.ts'
import { CommitForm } from './CommitForm.tsx'

vi.mock('./commitApi.ts', () => ({ commitChanges: vi.fn(() => Promise.resolve(makeBranch())) }))
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
  render(<CommitForm blocked={null} suggestedScope="" />)

  // Assert
  expect(screen.getByRole('option', { name: 'feat' })).toBeTruthy()
  expect(screen.getByRole('option', { name: 'fix' })).toBeTruthy()
})

test("offers a team's configured commit types instead of the built-in set", async () => {
  // Arrange
  // The team commits only hotfixes and chores, and never "feat" or "fix".
  mockConfig.mockReturnValue(configResult({ commit: { types: ['hotfix', 'chore'] } }))

  // Act
  render(<CommitForm blocked={null} suggestedScope="" />)

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
  render(<CommitForm blocked={null} suggestedScope="" />)

  // Act: commit once, then commit again without touching the Type dropdown
  await user.type(screen.getByLabelText('Subject'), 'first change')
  await user.click(screen.getByRole('button', { name: /commit staged changes/i }))
  await user.type(await screen.findByLabelText('Subject'), 'second change')
  await user.click(screen.getByRole('button', { name: /commit staged changes/i }))

  // Assert
  expect(mockCommit).toHaveBeenLastCalledWith(expect.objectContaining({ type: 'hotfix' }))
})

test('after a commit the form opens on the scope just used, and takes suggestions again', async () => {
  // Arrange
  mockConfig.mockReturnValue(configResult(undefined))
  const user = userEvent.setup()
  const { rerender } = render(<CommitForm blocked={null} suggestedScope="api" />)
  const scope = screen.getByLabelText<HTMLInputElement>('Scope (optional)')
  await user.clear(scope)
  await user.type(scope, 'cli')
  await user.type(screen.getByLabelText('Subject'), 'redact tokens')

  // Act: commit, then a frame suggests another scope
  await user.click(screen.getByRole('button', { name: /commit staged changes/i }))

  // Assert: the form keeps the scope it committed with
  await waitFor(() => {
    expect(screen.getByLabelText<HTMLInputElement>('Subject').value).toBe('')
  })
  expect(scope.value).toBe('cli')

  // Act: a later frame suggests another scope
  rerender(<CommitForm blocked={null} suggestedScope="web" />)

  // Assert: the scope is untouched since the commit, so the suggestion applies
  expect(scope.value).toBe('web')
})

test('after a commit with no scope the form opens on the suggestion again', async () => {
  // Arrange
  // A blank scope is not recorded, so the suggestion stays as it was and no
  // later frame brings it back: the reset itself must.
  mockConfig.mockReturnValue(configResult(undefined))
  const user = userEvent.setup()
  render(<CommitForm blocked={null} suggestedScope="api" />)
  const scope = screen.getByLabelText<HTMLInputElement>('Scope (optional)')
  await user.clear(scope)
  await user.type(screen.getByLabelText('Subject'), 'redact tokens')

  // Act
  await user.click(screen.getByRole('button', { name: /commit staged changes/i }))

  // Assert
  await waitFor(() => {
    expect(screen.getByLabelText<HTMLInputElement>('Subject').value).toBe('')
  })
  expect(mockCommit).toHaveBeenLastCalledWith(expect.objectContaining({ scope: '' }))
  expect(scope.value).toBe('api')
})

test('says a commit by the header it was made with when the branch does not reach it', async () => {
  // Arrange
  // A branch with no base lists no commits, so the header git recorded is not
  // there to read: the one the fields make stands in for it, type and all.
  mockConfig.mockReturnValue(configResult(undefined))
  mockCommit.mockResolvedValueOnce(makeBranch({ head: 'a1b2c3d4e5f6', commits: [] }))
  const user = userEvent.setup()
  render(<CommitForm blocked={null} suggestedScope="" />)
  await user.type(screen.getByLabelText('Scope (optional)'), 'api')
  await user.type(screen.getByLabelText('Subject'), 'redact tokens')
  await user.click(screen.getByLabelText('Breaking change'))

  // Act
  await user.click(screen.getByRole('button', { name: 'Commit staged changes' }))

  // Assert
  expect(await screen.findByText('Committed a1b2c3d fix(api)!: redact tokens.')).toBeTruthy()
})

test('says a commit by what was typed when the newest listed commit is not HEAD', async () => {
  // Arrange
  // A long branch's list stops short of HEAD: its last entry is an older commit.
  mockConfig.mockReturnValue(configResult(undefined))
  mockCommit.mockResolvedValueOnce(
    makeBranch({ head: 'a1b2c3d4e5f6', commits: [{ hash: 'c0ffee1', subject: 'feat: older' }] }),
  )
  const user = userEvent.setup()
  render(<CommitForm blocked={null} suggestedScope="" />)
  await user.type(screen.getByLabelText('Subject'), 'redact tokens')

  // Act
  await user.click(screen.getByRole('button', { name: 'Commit staged changes' }))

  // Assert
  expect(await screen.findByText('Committed a1b2c3d fix: redact tokens.')).toBeTruthy()
})

test('says a commit by its header alone when the server could not read its hash back', async () => {
  // Arrange
  // The commit landed but the branch read after it failed, so the server
  // answers the branch as it stood, with no head: there is no hash to name.
  mockConfig.mockReturnValue(configResult(undefined))
  mockCommit.mockResolvedValueOnce(makeBranch({ head: '', commits: [] }))
  const user = userEvent.setup()
  render(<CommitForm blocked={null} suggestedScope="" />)
  await user.type(screen.getByLabelText('Subject'), 'redact tokens')

  // Act
  await user.click(screen.getByRole('button', { name: 'Commit staged changes' }))

  // Assert
  await waitFor(() => {
    expect(screen.getByRole('status').textContent).toBe('Committed fix: redact tokens.')
  })
})

test('focus dropped to the page after a commit from the keyboard stays on the page', async () => {
  // Arrange
  // A commit made with Enter in the subject: the field stays to hold focus, so
  // the line that says what the commit did does not take it — then or when the
  // form is next drawn, after the user has clicked away.
  mockConfig.mockReturnValue(configResult(undefined))
  mockCommit.mockResolvedValueOnce(
    makeBranch({ head: 'a1b2c3d4e5f6', commits: [{ hash: 'a1b2c3d', subject: 'fix: redact' }] }),
  )
  const user = userEvent.setup()
  const { rerender } = render(<CommitForm blocked={null} suggestedScope="" />)
  await user.type(screen.getByLabelText('Subject'), 'redact{Enter}')
  await screen.findByText('Committed a1b2c3d fix: redact.')
  act(() => {
    ;(document.activeElement as HTMLElement).blur()
  })

  // Act: the next frame draws the form again
  rerender(<CommitForm blocked="Clean — nothing to commit." suggestedScope="" />)

  // Assert
  expect(document.activeElement).toBe(document.body)
})
