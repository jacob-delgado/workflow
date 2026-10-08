import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import { makeBranch } from '@/test/fixtures.ts'
import { commitChanges } from './commitApi.ts'
import { CommitForm } from './CommitForm.tsx'

vi.mock('./commitApi.ts', () => ({ commitChanges: vi.fn(() => Promise.resolve(makeBranch())) }))
const mockCommit = vi.mocked(commitChanges)

// The commit types these tests offer: fix among them, so the form keeps the
// type it opens on.
const types = ['feat', 'fix', 'docs']

// The convention these tests commit under: fix among its types, so the form
// keeps the type it opens on, and the built-in limit.
const conventional = { types, subjectLimit: 72 }

// A team that commits only hotfixes and chores, and never "feat" or "fix".
const hotfixesAndChoresOnly = { types: ['hotfix', 'chore'], subjectLimit: 72 }

test('opens on the first of its types when fix is not among them', async () => {
  // Act
  render(<CommitForm canCommit suggestedScope="" convention={hotfixesAndChoresOnly} />)

  // Assert
  // The default "fix" is not among them, so the chosen type is corrected to one
  // the server will accept.
  await waitFor(() => {
    expect(screen.getByRole<HTMLSelectElement>('combobox').value).toBe('hotfix')
  })
})

test('keeps a configured type after a commit for a team that excludes fix', async () => {
  // Arrange
  // After the first commit the form must not revert its type to the built-in
  // "fix" default the server would reject.
  const user = userEvent.setup()
  render(<CommitForm canCommit suggestedScope="" convention={hotfixesAndChoresOnly} />)

  // Act
  await user.type(screen.getByLabelText('Subject'), 'first change')
  await user.click(screen.getByRole('button', { name: /commit staged changes/i }))
  await user.type(await screen.findByLabelText('Subject'), 'second change')
  await user.click(screen.getByRole('button', { name: /commit staged changes/i }))

  // Assert
  expect(mockCommit).toHaveBeenLastCalledWith(expect.objectContaining({ type: 'hotfix' }))
})

test('after a commit the form opens on the scope just used, and takes suggestions again', async () => {
  // Arrange
  const user = userEvent.setup()
  const { rerender } = render(
    <CommitForm canCommit suggestedScope="api" convention={conventional} />,
  )
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
  rerender(<CommitForm canCommit suggestedScope="web" convention={conventional} />)

  // Assert: the scope is untouched since the commit, so the suggestion applies
  expect(scope.value).toBe('web')
})

test('after a commit with no scope the form opens on the suggestion again', async () => {
  // Arrange
  // A blank scope is not recorded, so the suggestion stays as it was and no
  // later frame brings it back: the reset itself must.
  const user = userEvent.setup()
  render(<CommitForm canCommit suggestedScope="api" convention={conventional} />)
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

test('sends the breaking mark and the body with the commit', async () => {
  // Arrange
  const user = userEvent.setup()
  render(<CommitForm canCommit suggestedScope="" convention={conventional} />)
  await user.type(screen.getByLabelText('Subject'), 'drop the v1 endpoints')
  await user.type(screen.getByLabelText('Body (optional)'), 'Clients move to v2.')
  await user.click(screen.getByLabelText('Breaking change'))

  // Act
  await user.click(screen.getByRole('button', { name: 'Commit staged changes' }))

  // Assert
  expect(mockCommit).toHaveBeenLastCalledWith(
    expect.objectContaining({ breaking: true, body: 'Clients move to v2.' }),
  )
})

test('says a commit by the header it was made with when the branch does not reach it', async () => {
  // Arrange
  // A branch with no base lists no commits, so the header git recorded is not
  // there to read: the one the fields make stands in for it, type and all.
  mockCommit.mockResolvedValueOnce(makeBranch({ head: 'a1b2c3d4e5f6', commits: [] }))
  const user = userEvent.setup()
  render(<CommitForm canCommit suggestedScope="" convention={conventional} />)
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
  mockCommit.mockResolvedValueOnce(
    makeBranch({
      head: 'a1b2c3d4e5f6',
      commits: [{ hash: 'c0ffee1', subject: 'feat: older', unpushed: false }],
    }),
  )
  const user = userEvent.setup()
  render(<CommitForm canCommit suggestedScope="" convention={conventional} />)
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
  mockCommit.mockResolvedValueOnce(makeBranch({ head: '', commits: [] }))
  const user = userEvent.setup()
  render(<CommitForm canCommit suggestedScope="" convention={conventional} />)
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
  mockCommit.mockResolvedValueOnce(
    makeBranch({
      head: 'a1b2c3d4e5f6',
      commits: [{ hash: 'a1b2c3d', subject: 'fix: redact', unpushed: false }],
    }),
  )
  const user = userEvent.setup()
  const { rerender } = render(<CommitForm canCommit suggestedScope="" convention={conventional} />)
  await user.type(screen.getByLabelText('Subject'), 'redact{Enter}')
  await screen.findByText('Committed a1b2c3d fix: redact.')
  act(() => {
    ;(document.activeElement as HTMLElement).blur()
  })

  // Act
  rerender(<CommitForm canCommit={false} suggestedScope="" convention={conventional} />)

  // Assert
  expect(document.activeElement).toBe(document.body)
})

test('the subject box shows its hint in sentence case, as every placeholder does', () => {
  // Act
  render(<CommitForm canCommit suggestedScope="" convention={{ types, subjectLimit: 72 }} />)

  // Assert
  expect(screen.getByLabelText('Subject').getAttribute('placeholder')).toBe(
    'What the change does, in the imperative',
  )
})

test('counts the header against the subject limit as the subject is typed', async () => {
  // Arrange
  const user = userEvent.setup()
  render(<CommitForm canCommit suggestedScope="" convention={{ types, subjectLimit: 20 }} />)
  const subject = screen.getByLabelText('Subject')

  // Act: type a subject
  await user.type(subject, 'redact')

  // Assert: "fix: redact" is 11 of the 20
  expect(screen.getByText('11/20')).toBeTruthy()

  // Act: type further
  await user.type(subject, ' tokens')

  // Assert: "fix: redact tokens" is 18
  expect(screen.getByText('18/20')).toBeTruthy()
})
