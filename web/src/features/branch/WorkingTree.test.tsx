import { act, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import type { Change } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import type { MarkState } from '@/lib/StateMark.tsx'
import { makeBranch, makeSnapshot } from '@/test/fixtures.ts'
import { drawnMark, markShape } from '@/test/marks.tsx'
import { BranchPanel } from './BranchPanel.tsx'
import { stageEverything, stageFile, unstageFile } from './stagingApi.ts'

vi.mock('./stagingApi.ts', () => ({
  stageFile: vi.fn(() => Promise.resolve()),
  unstageFile: vi.fn(() => Promise.resolve()),
  stageEverything: vi.fn(() => Promise.resolve()),
  unstageEverything: vi.fn(() => Promise.resolve()),
  discardFile: vi.fn(() => Promise.resolve()),
  readDiff: vi.fn(() => Promise.resolve({ path: '', lines: [] })),
}))
const mockStageFile = vi.mocked(stageFile)
const mockUnstageFile = vi.mocked(unstageFile)
const mockStageEverything = vi.mocked(stageEverything)

vi.mock('./commitApi.ts', () => ({ commitChanges: vi.fn(() => Promise.resolve(makeBranch())) }))
vi.mock('./pushApi.ts', () => ({ pushBranch: vi.fn(() => Promise.resolve(makeBranch())) }))

// change is a changed file as the stream lists it: an edit the index does not
// hold, unless a case says otherwise.
function change(path: string, overrides: Partial<Change> = {}): Change {
  return {
    path,
    kind: 'modified',
    staged: false,
    has_unstaged: true,
    conflicted: false,
    ...overrides,
  }
}

const wholly = { staged: true, has_unstaged: false }

// streamTree has the stream push a working tree of these changes, as it does
// on connect and after every write.
function streamTree(changes: Change[]) {
  useSnapshotStore.setState({ status: 'live', snapshot: makeSnapshot({ changes: { changes } }) })
}

// commitButton is the commit form's submit.
function commitButton() {
  return screen.getByRole('button', { name: /commit staged changes/i })
}

test('offers to stage each unstaged file and to unstage each staged one', () => {
  // Arrange
  streamTree([change('a.go', wholly), change('b.go')])

  // Act
  render(<BranchPanel />)

  // Assert
  expect(screen.getByRole('button', { name: 'Unstage a.go' })).toBeTruthy()
  expect(screen.getByRole('button', { name: 'Stage b.go' })).toBeTruthy()
  expect(screen.queryByRole('button', { name: 'Unstage b.go' })).toBeNull()
})

test('offers to stage the rest of a partly staged file', () => {
  // Arrange
  streamTree([change('c.go', { staged: true, has_unstaged: true })])

  // Act
  render(<BranchPanel />)

  // Assert
  expect(screen.getByRole('button', { name: 'Stage c.go' })).toBeTruthy()
  expect(screen.getByText('partly staged')).toBeTruthy()
})

test('offers to stage a conflict, which marks it resolved', () => {
  // Arrange
  streamTree([
    change('d.go', { kind: 'conflicted', staged: false, has_unstaged: false, conflicted: true }),
  ])

  // Act
  render(<BranchPanel />)

  // Assert
  expect(screen.getByRole('button', { name: 'Stage d.go' })).toBeTruthy()
  expect(screen.getByRole<HTMLButtonElement>('button', { name: 'Stage all' }).disabled).toBe(false)
})

test.each<{ staging: Partial<Change>; word: string; state: MarkState }>([
  { staging: {}, word: 'unstaged', state: 'not-started' },
  {
    staging: { staged: true, has_unstaged: true },
    word: 'partly staged',
    state: 'in-flight',
  },
  { staging: wholly, word: 'staged', state: 'done' },
  {
    staging: { kind: 'conflicted', staged: false, has_unstaged: false, conflicted: true },
    word: 'conflicted',
    state: 'failed',
  },
])(
  'marks how much of a file is staged by shape beside the word $word',
  ({ staging, word, state }) => {
    // Arrange
    streamTree([change('e.go', staging)])

    // Act
    render(<BranchPanel />)

    // Assert
    const row = screen.getByRole('listitem')
    expect(within(row).getAllByText(word).length).toBeGreaterThan(0)
    expect(markShape(row)).toBe(drawnMark(state))
  },
)

test('stages a file by its path and says so', async () => {
  // Arrange
  const user = userEvent.setup()
  streamTree([change('b.go')])
  render(<BranchPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Stage b.go' }))

  // Assert
  expect(mockStageFile).toHaveBeenCalledWith('b.go')
  expect((await screen.findByText('Staged b.go.')).getAttribute('role')).toBe('status')
})

test('unstages a wholly staged file and says so', async () => {
  // Arrange
  const user = userEvent.setup()
  streamTree([change('a.go', wholly)])
  render(<BranchPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Unstage a.go' }))

  // Assert
  expect(mockUnstageFile).toHaveBeenCalledWith('a.go')
  expect(mockStageFile).not.toHaveBeenCalled()
  expect(await screen.findByText('Unstaged a.go.')).toBeTruthy()
})

test('what a stage said survives the snapshot that shows the file staged', async () => {
  // Arrange
  const user = userEvent.setup()
  streamTree([change('b.go')])
  render(<BranchPanel />)
  await user.click(screen.getByRole('button', { name: 'Stage b.go' }))
  await screen.findByText('Staged b.go.')

  // Act
  // The stream brings the file back staged.
  act(() => {
    streamTree([change('b.go', wholly)])
  })

  // Assert
  // The file now offers to unstage, and the line still says what was done.
  expect(screen.getByRole('button', { name: 'Unstage b.go' })).toBeTruthy()
  expect(screen.getByText('Staged b.go.')).toBeTruthy()
})

test('focus stays on the button that staged, which is still there to hold it', async () => {
  // Arrange
  const user = userEvent.setup()
  streamTree([change('b.go')])
  render(<BranchPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Stage b.go' }))
  await screen.findByText('Staged b.go.')

  // Assert
  expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Stage b.go' }))
})

test('focus that fell to the page while staging lands on what the stage said', async () => {
  // Arrange
  // Focus can fall to the page while a stage runs — a click on nothing — so it
  // is dropped there through another control.
  let finishStaging = () => {}
  mockStageFile.mockImplementationOnce(
    () =>
      new Promise<void>((resolve) => {
        finishStaging = resolve
      }),
  )
  const user = userEvent.setup()
  streamTree([change('b.go')])
  render(<BranchPanel />)
  await user.click(screen.getByRole('button', { name: 'Stage b.go' }))
  const elsewhere = screen.getByRole('button', { name: 'Stage all' })
  elsewhere.focus()
  elsewhere.blur()

  // Act
  act(() => {
    finishStaging()
  })
  const said = await screen.findByText('Staged b.go.')

  // Assert
  // Focus moves in an effect after the outcome renders, which a loaded
  // machine can run a moment after the text appears.
  await waitFor(() => {
    expect(document.activeElement).toBe(said)
  })
})

test('staging leaves focus in a subject being typed', async () => {
  // Arrange
  // Stage all runs one git process per file, so its answer can come while
  // the user is already writing the message in the form below.
  let finishStaging = () => {}
  mockStageEverything.mockImplementationOnce(
    () =>
      new Promise<void>((resolve) => {
        finishStaging = resolve
      }),
  )
  const user = userEvent.setup()
  streamTree([change('a.go')])
  render(<BranchPanel />)
  await user.click(screen.getByRole('button', { name: 'Stage all' }))
  const subject = screen.getByRole('textbox', { name: /subject/i })
  await user.click(subject)

  // Act
  act(() => {
    finishStaging()
  })
  await screen.findByText('Staged every change.')

  // Assert
  expect(document.activeElement).toBe(subject)
})

test('says why a file could not be staged, with focus still on its Stage', async () => {
  // Arrange
  mockStageFile.mockRejectedValueOnce({
    code: 'unprocessable',
    detail: 'git would not stage b.go; stage from a terminal to see git’s reason',
  })
  const user = userEvent.setup()
  streamTree([change('b.go')])
  render(<BranchPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Stage b.go' }))

  // Assert
  expect(await screen.findByText(/git would not stage b\.go/)).toBeTruthy()
  expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Stage b.go' }))
})

test('Stage all makes the commit form live', async () => {
  // Arrange
  const user = userEvent.setup()
  const unstaged = [change('a.go'), change('notes.txt', { kind: 'untracked' })]
  streamTree(unstaged)

  // Act: draw the panel
  render(<BranchPanel />)

  // Assert: nothing is staged, so the form waits and says what to do, and
  // Stage all has said nothing yet
  expect(commitButton().hasAttribute('disabled')).toBe(true)
  expect(screen.getByText('Nothing staged yet — stage a file above.')).toBeTruthy()
  expect(screen.queryByText('Staged every change.')).toBeNull()

  // Act: stage all, and the stream brings the index back full
  await user.click(screen.getByRole('button', { name: 'Stage all' }))
  await screen.findByText('Staged every change.')
  act(() => {
    streamTree(unstaged.map((each) => ({ ...each, ...wholly })))
  })

  // Assert: the form is live
  expect(mockStageEverything).toHaveBeenCalledTimes(1)
  expect(commitButton().hasAttribute('disabled')).toBe(false)
  expect(screen.queryByText('Nothing staged yet — stage a file above.')).toBeNull()
})

test('Stage all is off when the index holds everything', () => {
  // Arrange
  streamTree([change('a.go', wholly)])

  // Act
  render(<BranchPanel />)

  // Assert
  expect(screen.getByRole('button', { name: 'Stage all' }).hasAttribute('disabled')).toBe(true)
})

test('says why the changes could not all be staged, with focus still on Stage all', async () => {
  // Arrange
  mockStageEverything.mockRejectedValueOnce({ code: 'unprocessable', detail: 'git refused' })
  const user = userEvent.setup()
  streamTree([change('a.go')])
  render(<BranchPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Stage all' }))

  // Assert
  expect(await screen.findByText('git refused')).toBeTruthy()
  expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Stage all' }))
})

test('a clean tree keeps the commit form, with nothing to stage', () => {
  // Arrange
  streamTree([])

  // Act
  render(<BranchPanel />)

  // Assert
  expect(screen.getByText('Clean — nothing to commit.')).toBeTruthy()
  expect(commitButton().hasAttribute('disabled')).toBe(true)
  expect(screen.queryByRole('button', { name: 'Stage all' })).toBeNull()
})

test('a clean tree says so under its heading, before the commit form', () => {
  // Arrange
  streamTree([])

  // Act
  render(<BranchPanel />)

  // Assert
  // Said once, and before the form, not at its foot.
  const clean = screen.getByText('Clean — nothing to commit.')
  const form = screen.getByRole('form', { name: 'Commit staged changes' })
  expect(clean.compareDocumentPosition(form) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  expect(form.contains(clean)).toBe(false)
})

// streamSuggestion has the stream push a frame suggesting scope for the next
// commit, with one file staged so the form is live.
function streamSuggestion(scope: string) {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      changes: { changes: [change('a.go', wholly)] },
      suggested_scope: scope,
    }),
  })
}

// scopeField is the commit form's scope input.
function scopeField() {
  return screen.getByLabelText<HTMLInputElement>('Scope (optional)')
}

test('the commit form offers the commit types the stream carries, in its order', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ commit_types: ['wip', 'deps'] }),
  })

  // Act
  render(<BranchPanel />)

  // Assert
  const types = within(screen.getByRole('combobox', { name: 'Type' })).getAllByRole('option')
  expect(types.map((option) => option.textContent)).toEqual(['wip', 'deps'])
})

test('the commit form opens on the suggested scope', () => {
  // Arrange
  streamSuggestion('api')

  // Act
  render(<BranchPanel />)

  // Assert
  expect(scopeField().value).toBe('api')
})

test('a new frame brings its suggestion to an untouched scope', () => {
  // Arrange
  // Nothing learned and no default yet.
  streamSuggestion('')
  render(<BranchPanel />)

  // Act
  // default_scope is saved in Settings, and the next frame suggests it.
  act(() => {
    streamSuggestion('web')
  })

  // Assert
  expect(scopeField().value).toBe('web')
})

test('a new frame does not overwrite a typed scope', async () => {
  // Arrange
  const user = userEvent.setup()
  streamSuggestion('api')
  render(<BranchPanel />)
  await user.clear(scopeField())
  await user.type(scopeField(), 'cli')

  // Act
  act(() => {
    streamSuggestion('web')
  })

  // Assert
  expect(scopeField().value).toBe('cli')
})

test('draws a renamed file by the path it had and the path it has', () => {
  // Arrange
  streamTree([change('cmd/new.go', { kind: 'renamed', original_path: 'cmd/old.go' })])

  // Act
  render(<BranchPanel />)

  // Assert
  const row = screen.getByRole('button', { name: 'Stage cmd/new.go' }).closest('li')
  expect(row?.textContent).toMatch(/cmd\/old\.go.*cmd\/new\.go/)
})
