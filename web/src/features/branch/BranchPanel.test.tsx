import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import type { Branch } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeBranch, makeSnapshot } from '@/test/fixtures.ts'
import { BranchPanel } from './BranchPanel.tsx'
import { commitChanges } from './commitApi.ts'
import { pushBranch } from './pushApi.ts'

vi.mock('./commitApi.ts', () => ({ commitChanges: vi.fn(() => Promise.resolve(makeBranch())) }))
const mockCommit = vi.mocked(commitChanges)

vi.mock('./pushApi.ts', () => ({ pushBranch: vi.fn(() => Promise.resolve(makeBranch())) }))
const mockPush = vi.mocked(pushBranch)

// pushable is a snapshot whose branch has commits the remote does not have.
function pushable() {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branch: {
        name: 'fix/PROJ-1',
        issue_link: '',
        detached: false,
        head: 'abc1234',
        upstream: '',
        push_remote: 'origin',
        ahead: 0,
        behind: 0,
        base: 'origin/main',
        commits: [{ hash: 'c0ffee1', subject: 'fix: redact tokens', unpushed: false }],
      },
    }),
  })
}

// staged is a snapshot whose working tree has one staged and one unstaged change.
function staged() {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      changes: {
        changes: [
          { path: 'a.go', kind: 'modified', staged: true, has_unstaged: false, conflicted: false },
          { path: 'b.go', kind: 'modified', staged: false, has_unstaged: true, conflicted: false },
        ],
      },
    }),
  })
}

test('shows the current branch, its commits, and its changes', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branch: {
        name: 'feat/web',
        issue_link: '',
        detached: false,
        head: 'deadbee',
        upstream: 'origin/feat/web',
        push_remote: 'origin',
        ahead: 3,
        behind: 1,
        base: 'origin/main',
        commits: [{ hash: 'deadbeef1', subject: 'feat: build the shell', unpushed: false }],
      },
      changes: {
        changes: [
          {
            path: 'web/src/App.tsx',
            kind: 'modified',
            staged: true,
            has_unstaged: false,
            conflicted: false,
          },
        ],
      },
    }),
  })

  // Act
  render(<BranchPanel />)

  // Assert
  expect(screen.getByRole('heading', { name: 'feat/web' })).toBeTruthy()
  expect(screen.getByText(/build the shell/)).toBeTruthy()
  expect(screen.getByText('web/src/App.tsx')).toBeTruthy()
})

test('offers a commit form when a change is staged', () => {
  // Arrange
  staged()

  // Act
  render(<BranchPanel />)

  // Assert
  const commit = screen.getByRole('button', { name: /commit staged changes/i })
  expect(commit.hasAttribute('disabled')).toBe(false)
  expect(screen.queryByText('Nothing staged yet — stage a file above.')).toBeNull()
})

test('keeps the commit form when nothing is staged, saying what it waits for', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      changes: {
        changes: [
          { path: 'b.go', kind: 'modified', staged: false, has_unstaged: true, conflicted: false },
        ],
      },
    }),
  })

  // Act
  render(<BranchPanel />)

  // Assert
  const commit = screen.getByRole('button', { name: /commit staged changes/i })
  expect(commit.hasAttribute('disabled')).toBe(true)
  expect(screen.getByText('Nothing staged yet — stage a file above.')).toBeTruthy()
})

test('commits the staged changes when the form is submitted', async () => {
  // Arrange
  mockCommit.mockResolvedValueOnce(makeBranch())
  const user = userEvent.setup()
  staged()
  render(<BranchPanel />)
  await user.type(screen.getByLabelText('Subject'), 'redact tokens')

  // Act
  await user.click(screen.getByRole('button', { name: /commit staged changes/i }))

  // Assert
  expect(mockCommit).toHaveBeenCalledWith(
    expect.objectContaining({ type: 'fix', subject: 'redact tokens' }),
  )
})

test('shows the reason when a commit is refused, with focus still on the commit', async () => {
  // Arrange
  mockCommit.mockRejectedValueOnce({
    code: 'unprocessable',
    detail: 'nothing is staged to commit',
  })
  const user = userEvent.setup()
  staged()
  render(<BranchPanel />)
  await user.type(screen.getByLabelText('Subject'), 'redact tokens')

  // Act
  await user.click(screen.getByRole('button', { name: /commit staged changes/i }))

  // Assert
  expect(await screen.findByText(/nothing is staged/i)).toBeTruthy()
  expect(document.activeElement).toBe(
    screen.getByRole('button', { name: /commit staged changes/i }),
  )
})

test('locks the commit while it is in flight', async () => {
  // Arrange
  // Hold the commit open so the in-flight state is observable rather than
  // transient; a live button here would let a double click commit twice.
  let releaseCommit = () => {}
  mockCommit.mockImplementationOnce(
    () =>
      new Promise<Branch>((resolve) => {
        releaseCommit = () => {
          resolve(makeBranch())
        }
      }),
  )
  const user = userEvent.setup()
  staged()
  render(<BranchPanel />)
  await user.type(screen.getByLabelText('Subject'), 'redact tokens')

  // Act
  await user.click(screen.getByRole('button', { name: 'Commit staged changes' }))

  // Assert
  const committing = await screen.findByRole('button', { name: 'Committing…' })
  expect(committing.getAttribute('aria-disabled')).toBe('true')
  expect(document.activeElement).toBe(committing)
  expect(mockCommit).toHaveBeenCalledTimes(1)

  releaseCommit()
  await screen.findByRole('button', { name: 'Commit staged changes' })
})

test('offers to push a branch with unpushed commits', () => {
  // Arrange
  pushable()

  // Act
  render(<BranchPanel />)

  // Assert
  expect(screen.getByRole('button', { name: 'Push branch' })).toBeTruthy()
})

test('offers no push for a fully published branch', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branch: {
        name: 'fix/PROJ-1',
        issue_link: '',
        detached: false,
        head: 'abc1234',
        upstream: 'origin/fix/PROJ-1',
        push_remote: 'origin',
        ahead: 0,
        behind: 0,
        base: 'origin/main',
        commits: [{ hash: 'c0ffee1', subject: 'fix: redact tokens', unpushed: false }],
      },
    }),
  })

  // Act
  render(<BranchPanel />)

  // Assert
  expect(screen.queryByRole('button', { name: 'Push branch' })).toBeNull()
})

test('offers no push in a detached HEAD', () => {
  // Arrange
  // Commits present, but not on a branch — so there is nothing to push.
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branch: {
        name: '',
        issue_link: '',
        detached: true,
        head: 'abcdef1234',
        upstream: '',
        push_remote: 'origin',
        ahead: 0,
        behind: 0,
        base: '',
        commits: [{ hash: 'c0ffee1', subject: 'fix: redact tokens', unpushed: false }],
      },
    }),
  })

  // Act
  render(<BranchPanel />)

  // Assert
  expect(screen.queryByRole('button', { name: 'Push branch' })).toBeNull()
})

test('pushes only after the confirm step', async () => {
  // Arrange
  mockPush.mockResolvedValueOnce(makeBranch())
  const user = userEvent.setup()
  pushable()
  render(<BranchPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Push branch' }))
  await user.click(screen.getByRole('button', { name: 'Push' }))

  // Assert
  expect(mockPush).toHaveBeenCalledTimes(1)
})

test('does not push if the confirm is canceled', async () => {
  // Arrange
  const user = userEvent.setup()
  pushable()
  render(<BranchPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Push branch' }))
  await user.click(screen.getByRole('button', { name: 'Cancel' }))

  // Assert
  expect(mockPush).not.toHaveBeenCalled()
})

test('shows the reason when a push fails', async () => {
  // Arrange
  mockPush.mockRejectedValueOnce({ code: 'unprocessable', detail: 'the push failed: rejected' })
  const user = userEvent.setup()
  pushable()
  render(<BranchPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Push branch' }))
  await user.click(screen.getByRole('button', { name: 'Push' }))

  // Assert
  expect(await screen.findByText(/the push failed/i)).toBeTruthy()
})

test('says what to do when a push never reaches the server', async () => {
  // Arrange
  // A stopped server: the fetch itself fails, with the browser's own words.
  mockPush.mockRejectedValueOnce(new TypeError('Failed to fetch'))
  const user = userEvent.setup()
  pushable()
  render(<BranchPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Push branch' }))
  await user.click(screen.getByRole('button', { name: 'Push' }))

  // Assert
  const alert = await screen.findByRole('alert')
  expect(alert.textContent).toBe(
    'The branch was not pushed. Try again, or push from a terminal to see why.',
  )
})

test('locks the push while it is in flight', async () => {
  // Arrange
  // Hold the push open so the in-flight state is observable rather than
  // transient; a live button here would let a second push go out behind it.
  let releasePush = () => {}
  mockPush.mockImplementationOnce(
    () =>
      new Promise<Branch>((resolve) => {
        releasePush = () => {
          resolve(makeBranch())
        }
      }),
  )
  const user = userEvent.setup()
  pushable()
  render(<BranchPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Push branch' }))
  await user.click(screen.getByRole('button', { name: 'Push' }))

  // Assert
  const pushing = await screen.findByRole('button', { name: 'Pushing…' })
  expect(pushing.getAttribute('aria-disabled')).toBe('true')
  expect(mockPush).toHaveBeenCalledTimes(1)

  releasePush()
  await screen.findByRole('button', { name: 'Push branch' })
})

test('asking to push again after a refusal shows the confirm, not the old reason', async () => {
  // Arrange
  mockPush.mockRejectedValueOnce({ code: 'conflict', detail: 'the remote refused the push' })
  const user = userEvent.setup()
  pushable()
  render(<BranchPanel />)
  await user.click(screen.getByRole('button', { name: 'Push branch' }))
  await user.click(screen.getByRole('button', { name: 'Push' }))
  await screen.findByText('the remote refused the push')

  // Act
  await user.click(screen.getByRole('button', { name: 'Push branch' }))

  // Assert
  expect(screen.getByRole('button', { name: 'Push' })).toBeTruthy()
  expect(screen.queryByText('the remote refused the push')).toBeNull()
})

test('shows placeholders for an unpublished branch with a clean tree', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branch: {
        name: 'wip',
        issue_link: '',
        detached: false,
        head: 'aaa',
        upstream: '',
        push_remote: 'origin',
        ahead: 0,
        behind: 0,
        base: 'origin/main',
        commits: [],
      },
      changes: { changes: [] },
    }),
  })

  // Act
  render(<BranchPanel />)

  // Assert
  expect(screen.getByText('None')).toBeTruthy()
  expect(screen.getByText(/no commits yet/i)).toBeTruthy()
  expect(screen.getByText(/clean/i)).toBeTruthy()
})

test('shows a detached HEAD rather than calling it not a repository', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branch: {
        name: '',
        issue_link: '',
        detached: true,
        head: 'abcdef1234',
        upstream: '',
        push_remote: 'origin',
        ahead: 0,
        behind: 0,
        base: '',
        commits: [],
      },
    }),
  })

  // Act
  render(<BranchPanel />)

  // Assert
  expect(screen.getByRole('heading', { name: /detached head at abcdef1/i })).toBeTruthy()
})

test('says so when the workspace is not a Git repository, and what one would show', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branch: {
        name: '',
        issue_link: '',
        detached: false,
        head: '',
        upstream: '',
        push_remote: '',
        ahead: 0,
        behind: 0,
        base: '',
        commits: [],
      },
    }),
  })

  // Act
  render(<BranchPanel />)

  // Assert
  expect(
    screen.getByText(
      'This directory is not a Git repository. Start workflow --web inside one to see its branch, commits and working tree here.',
    ),
  ).toBeTruthy()
})

test('the confirm takes focus as it opens, and Cancel hands it back', async () => {
  // Arrange
  const user = userEvent.setup()
  pushable()
  render(<BranchPanel />)

  // Act: ask to push
  await user.click(screen.getByRole('button', { name: 'Push branch' }))

  // Assert: focus is on the question
  expect(document.activeElement).toBe(
    screen.getByRole('group', { name: 'Push fix/PROJ-1 to origin?' }),
  )

  // Act: back out
  await user.click(screen.getByRole('button', { name: 'Cancel' }))

  // Assert: focus is back on the button that asked
  expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Push branch' }))
})

test('a refused push hands focus back to the button, beside its reason', async () => {
  // Arrange
  mockPush.mockRejectedValueOnce({ code: 'conflict', detail: 'the remote refused the push' })
  const user = userEvent.setup()
  pushable()
  render(<BranchPanel />)
  await user.click(screen.getByRole('button', { name: 'Push branch' }))

  // Act
  await user.click(screen.getByRole('button', { name: 'Push' }))

  // Assert
  await screen.findByText('the remote refused the push')
  expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Push branch' }))
})

test('focus moved on while a push runs stays put when the push is refused', async () => {
  // Arrange
  let refusePush = () => {}
  mockPush.mockImplementationOnce(
    () =>
      new Promise<Branch>((_, reject) => {
        refusePush = () => {
          reject(new Error('refused'))
        }
      }),
  )
  const user = userEvent.setup()
  pushable()
  render(<BranchPanel />)
  await user.click(screen.getByRole('button', { name: 'Push branch' }))
  await user.click(screen.getByRole('button', { name: 'Push' }))
  const subject = screen.getByLabelText('Subject')
  subject.focus()

  // Act
  refusePush()

  // Assert
  await screen.findByRole('alert')
  expect(document.activeElement).toBe(subject)
})

test('a missing base and a missing upstream read the same word', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branch: {
        name: 'wip',
        issue_link: '',
        detached: false,
        head: 'aaa',
        upstream: '',
        push_remote: 'origin',
        ahead: 0,
        behind: 0,
        base: '',
        commits: [],
      },
    }),
  })

  // Act
  render(<BranchPanel />)

  // Assert
  const terms = screen.getAllByRole('term').map((term) => term.textContent)
  const values = screen.getAllByRole('definition').map((value) => value.textContent)
  const valueOf = (term: string) => values[terms.indexOf(term)]
  expect([valueOf('Base'), valueOf('Upstream')]).toEqual(['None', 'None'])
})

test('under a detached HEAD, says where a branch is to be had: Issues', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branch: makeBranch({ name: '', detached: true, head: 'abcdef1234' }),
    }),
  })

  // Act
  render(<BranchPanel />)

  // Assert
  const heading = screen.getByRole('heading', { name: 'Detached HEAD at abcdef1' })
  const section = heading.closest('section')
  expect(section?.textContent).toMatch(/Issues, where Switch branch and Start work/)
})
