import { act, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import type { Ci, PullRequest } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeBranch, makeSnapshot } from '@/test/fixtures.ts'
import { ReviewPanel } from './ReviewPanel.tsx'
import {
  editPull,
  finishBranch,
  mergePull,
  readMergeOffer,
  readPullText,
  rerunChecks,
} from './reviewWritesApi.ts'

// ready is #42, open, approved, clean.
const ready: PullRequest = {
  number: 42,
  url: 'https://forge.example.com/pull/42',
  title: 'Redact tokens',
  state: 'open',
  draft: false,
  approvals: 1,
  changes_requested: false,
  mergeable: 'clean',
}

vi.mock('./reviewWritesApi.ts', () => ({
  readPullText: vi.fn(() => Promise.resolve({ title: 'Redact tokens', body: 'Why: they leak.' })),
  editPull: vi.fn((title: string) => Promise.resolve({ ...ready, title })),
  readMergeOffer: vi.fn(() =>
    Promise.resolve({ pull: ready, methods: ['squash', 'rebase'], no_ci_wait_minutes: 10 }),
  ),
  mergePull: vi.fn(() => Promise.resolve({ ...ready, state: 'merged' })),
  finishBranch: vi.fn(() => Promise.resolve(makeBranch({ name: 'main' }))),
  rerunChecks: vi.fn(() => Promise.resolve({ reran: true })),
}))

// ciOf is CI in one state, with one check that keeps a log.
function ciOf(state: Ci['state']): Ci {
  return {
    state,
    no_ci: false,
    total: 1,
    done: 1,
    failed: state === 'failed' ? 1 : 0,
    checks: [{ name: 'build', state, url: '', id: '7', log_available: true }],
  }
}

// finishCommands stand for the commands the server says a finish runs, which
// the finish's last look shows as they are.
const finishCommands = ['the switch to main', 'the catch-up', 'the force delete of fix/PROJ-1']

// streamPull streams the branch's pull request and its CI, and why a merged
// one's branch cannot be finished when the server says it cannot.
function streamPull(pull: PullRequest, ci?: Ci, finishRefusal?: string) {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branch: makeBranch({ name: 'fix/PROJ-1', ahead: 0, finish_commands: finishCommands }),
      review: { found: true, announced: 'none', pull, ci, finish_refusal: finishRefusal },
    }),
  })
}

test('edits the pull request from a form that is its last look', async () => {
  // Arrange
  streamPull(ready, ciOf('passed'))
  const user = userEvent.setup()
  render(<ReviewPanel />)
  await user.click(screen.getByRole('button', { name: 'Edit pull request' }))
  const form = await screen.findByRole('form', { name: 'Edit #42' })
  const title = within(form).getByRole('textbox', { name: 'Title' })
  await user.clear(title)
  await user.type(title, 'Redact every token')

  // Act
  await user.click(within(form).getByRole('button', { name: 'Save' }))

  // Assert
  expect(vi.mocked(editPull)).toHaveBeenCalledWith('Redact every token', 'Why: they leak.')
  expect(await screen.findByText('Saved #42.')).toBeTruthy()
})

test('merges by the method chosen among those permitted, after a preview', async () => {
  // Arrange
  streamPull(ready, ciOf('passed'))
  const user = userEvent.setup()
  render(<ReviewPanel />)

  // Act: open the preview, and choose a method
  await user.click(screen.getByRole('button', { name: 'Merge' }))
  const form = await screen.findByRole('form', { name: 'Merge #42' })
  await user.click(within(form).getByRole('radio', { name: 'Rebase and merge' }))

  // Assert: nothing is merged yet
  expect(vi.mocked(mergePull)).not.toHaveBeenCalled()

  // Act: confirm
  await user.click(within(form).getByRole('button', { name: 'Merge' }))

  // Assert: it merges by that method
  expect(vi.mocked(mergePull)).toHaveBeenCalledWith('rebase')
  expect(vi.mocked(readMergeOffer)).toHaveBeenCalled()
})

test('offers no merge until the pull request can be merged', () => {
  // Arrange
  streamPull({ ...ready, approvals: 0 }, ciOf('passed'))

  // Act
  render(<ReviewPanel />)

  // Assert
  expect(screen.queryByRole('button', { name: 'Merge' })).toBeNull()
})

test('offers the merge of a pull request no CI was reported on in the wait, saying so', async () => {
  // Arrange
  // The wait is the server's, as its offer says it.
  streamPull(ready, { state: 'none', no_ci: true, total: 0, done: 0, failed: 0, checks: [] })
  vi.mocked(readMergeOffer).mockResolvedValueOnce({
    pull: ready,
    methods: ['squash'],
    no_ci_wait_minutes: 15,
  })
  const user = userEvent.setup()
  render(<ReviewPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Merge' }))

  // Assert
  const form = await screen.findByRole('form', { name: 'Merge #42' })
  expect(within(form).getByText(/No CI was reported on #42 in 15 minutes/)).toBeTruthy()
})

test('offers no merge while no check is reported yet on the head', () => {
  // Arrange
  // Q34: a forge lists a pushed head's checks a moment after the push.
  streamPull(ready, { state: 'none', no_ci: false, total: 0, done: 0, failed: 0, checks: [] })

  // Act
  render(<ReviewPanel />)

  // Assert
  expect(screen.queryByRole('button', { name: 'Merge' })).toBeNull()
})

test('offers no merge before the CI is read', () => {
  // Arrange
  streamPull(ready)

  // Act
  render(<ReviewPanel />)

  // Assert
  expect(screen.queryByRole('button', { name: 'Merge' })).toBeNull()
})

test('finishes a merged branch after a look at the commands it runs', async () => {
  // Arrange
  streamPull({ ...ready, state: 'merged' })
  const user = userEvent.setup()
  render(<ReviewPanel />)

  // Act: ask to finish the branch
  await user.click(screen.getByRole('button', { name: 'Finish the branch' }))

  // Assert: the look shows the commands it runs
  const form = screen.getByRole('form', { name: 'Finish fix/PROJ-1' })
  expect(form.textContent).toContain(finishCommands.join('\n'))

  // Act: confirm
  await user.click(within(form).getByRole('button', { name: 'Finish' }))

  // Assert: the branch is finished
  expect(vi.mocked(finishBranch)).toHaveBeenCalled()
  expect(await screen.findByText('Finished fix/PROJ-1; now on main.')).toBeTruthy()
})

test('says why a merged branch cannot be finished, offering no finish', () => {
  // Arrange
  streamPull(
    { ...ready, state: 'merged' },
    undefined,
    'it holds commits that neither merged nor are on its upstream, which deleting it would lose; push them first',
  )

  // Act
  render(<ReviewPanel />)

  // Assert
  expect(
    screen.getByText(
      'The branch cannot be finished yet: it holds commits that neither merged nor are on its upstream, which deleting it would lose; push them first.',
    ),
  ).toBeTruthy()
  expect(screen.queryByRole('button', { name: 'Finish the branch' })).toBeNull()
})

test('re-runs failed CI after a last look', async () => {
  // Arrange
  streamPull(ready, ciOf('failed'))
  const user = userEvent.setup()
  render(<ReviewPanel />)
  await user.click(screen.getByRole('button', { name: 'Re-run failed checks' }))
  const form = screen.getByRole('form', { name: 'Re-run the failed checks on #42' })

  // Act
  await user.click(within(form).getByRole('button', { name: 'Re-run' }))

  // Assert
  expect(vi.mocked(rerunChecks)).toHaveBeenCalled()
  expect(await screen.findByText('Re-ran the failed checks on #42.')).toBeTruthy()
})

test('reads the log of a check that passed, not only a failed one', () => {
  // Arrange
  streamPull(ready, ciOf('passed'))

  // Act
  render(<ReviewPanel />)

  // Assert
  expect(screen.getByRole('button', { name: 'Show log of build' })).toBeTruthy()
})

test('says why an edit was refused, keeping the form', async () => {
  // Arrange
  vi.mocked(readPullText).mockResolvedValueOnce({ title: 'x', body: '' })
  vi.mocked(editPull).mockRejectedValueOnce({
    code: 'unprocessable',
    detail: 'GitHub refused this',
  })
  streamPull(ready, ciOf('passed'))
  const user = userEvent.setup()
  render(<ReviewPanel />)
  await user.click(screen.getByRole('button', { name: 'Edit pull request' }))
  const form = await screen.findByRole('form', { name: 'Edit #42' })

  // Act
  await user.click(within(form).getByRole('button', { name: 'Save' }))

  // Assert
  expect((await within(form).findByRole('alert')).textContent).toContain('GitHub refused this')
})

test.each([
  ['Edit pull request', 'Edit #42', 'passed'],
  ['Merge', 'Merge #42', 'passed'],
  ['Re-run failed checks', 'Re-run the failed checks on #42', 'failed'],
] as const)('backing out of %s sends nothing and hands focus back', async (opener, form, ci) => {
  // Arrange
  streamPull(ready, ciOf(ci))
  const user = userEvent.setup()
  render(<ReviewPanel />)
  await user.click(screen.getByRole('button', { name: opener }))
  const look = await screen.findByRole('form', { name: form })

  // Act
  await user.click(within(look).getByRole('button', { name: 'Cancel' }))

  // Assert
  expect(document.activeElement).toBe(screen.getByRole('button', { name: opener }))
  expect(vi.mocked(editPull)).not.toHaveBeenCalled()
})

test.each([
  ['Edit pull request', readPullText],
  ['Merge', readMergeOffer],
] as const)('%s says why what it starts from could not be read', async (opener, read) => {
  // Arrange
  vi.mocked(read).mockRejectedValueOnce({
    code: 'conflict',
    detail: 'the branch has no open pull request',
  })
  streamPull(ready, ciOf('passed'))
  const user = userEvent.setup()
  render(<ReviewPanel />)
  await user.click(screen.getByRole('button', { name: opener }))

  // Act
  await user.click(await screen.findByRole('button', { name: 'Cancel' }))

  // Assert
  expect(screen.getByRole('button', { name: opener })).toBeTruthy()
})

test('says when a re-run had nothing to restart', async () => {
  // Arrange
  vi.mocked(rerunChecks).mockResolvedValueOnce({ reran: false })
  streamPull(ready, ciOf('failed'))
  const user = userEvent.setup()
  render(<ReviewPanel />)
  await user.click(screen.getByRole('button', { name: 'Re-run failed checks' }))

  // Act
  await user.click(screen.getByRole('button', { name: 'Re-run' }))

  // Assert
  expect(await screen.findByText(/Nothing was re-run/)).toBeTruthy()
})

test('says why a finish was refused, keeping its look', async () => {
  // Arrange
  vi.mocked(finishBranch).mockRejectedValueOnce({
    code: 'unprocessable',
    detail: 'git would not finish the branch',
  })
  streamPull({ ...ready, state: 'merged' })
  const user = userEvent.setup()
  render(<ReviewPanel />)
  await user.click(screen.getByRole('button', { name: 'Finish the branch' }))
  const look = screen.getByRole('form', { name: 'Finish fix/PROJ-1' })

  // Act
  await user.click(within(look).getByRole('button', { name: 'Finish' }))

  // Assert
  expect((await within(look).findByRole('alert')).textContent).toContain('git would not finish')
})

test('puts the merge form away once CI runs again', async () => {
  // Arrange
  streamPull(ready, ciOf('passed'))
  const user = userEvent.setup()
  render(<ReviewPanel />)
  await user.click(screen.getByRole('button', { name: 'Merge' }))
  await screen.findByRole('form', { name: 'Merge #42' })

  // Act
  act(() => {
    streamPull(ready, ciOf('running'))
  })

  // Assert
  expect(screen.queryByRole('form', { name: 'Merge #42' })).toBeNull()
})

test('a merge form put away by CI running again does not come back unasked', async () => {
  // Arrange
  streamPull(ready, ciOf('passed'))
  const user = userEvent.setup()
  render(<ReviewPanel />)
  await user.click(screen.getByRole('button', { name: 'Merge' }))
  await screen.findByRole('form', { name: 'Merge #42' })
  act(() => {
    streamPull(ready, ciOf('running'))
  })

  // Act
  act(() => {
    streamPull(ready, ciOf('passed'))
  })

  // Assert
  expect(screen.queryByRole('form', { name: 'Merge #42' })).toBeNull()
  expect(screen.getByRole('button', { name: 'Merge' })).toBeTruthy()
})

// holdWrite has the next call of write wait until the function it returns
// refuses it, saying detail.
function holdWrite(write: (...args: never[]) => Promise<unknown>): (detail: string) => void {
  let refuse: (reason: unknown) => void = () => {
    throw new Error('the write was not sent')
  }
  vi.mocked(write).mockImplementationOnce(
    () =>
      new Promise((_resolve, reject) => {
        refuse = reject
      }),
  )

  return (detail) => {
    refuse({ code: 'conflict', detail })
  }
}

// merged is #42 once merged.
const merged: PullRequest = { ...ready, state: 'merged' }

test.each([
  {
    opener: 'Merge',
    form: 'Merge #42',
    send: 'Merge',
    write: mergePull,
    lapse: () => {
      streamPull(ready, ciOf('running'))
    },
  },
  {
    opener: 'Finish the branch',
    form: 'Finish fix/PROJ-1',
    send: 'Finish',
    write: finishBranch,
    lapse: () => {
      streamPull(merged, undefined, 'fix/PROJ-1 has commits main lacks')
    },
  },
  {
    opener: 'Edit pull request',
    form: 'Edit #42',
    send: 'Save',
    write: editPull,
    lapse: () => {
      streamPull({ ...ready, state: 'closed' }, ciOf('passed'))
    },
  },
])(
  'says why $opener sent before its offer lapsed was refused, holding its send',
  async ({ opener, form, send, write, lapse }) => {
    // Arrange
    const refuse = holdWrite(write)
    streamPull(opener === 'Finish the branch' ? merged : ready, ciOf('passed'))
    const user = userEvent.setup()
    render(<ReviewPanel />)
    await user.click(screen.getByRole('button', { name: opener }))
    const look = await screen.findByRole('form', { name: form })
    await user.click(within(look).getByRole('button', { name: send }))
    act(lapse)

    // Act
    await act(async () => {
      refuse('the forge refused it')
      await Promise.resolve()
    })

    // Assert
    expect((await within(look).findByRole('alert')).textContent).toContain('the forge refused it')
    expect(within(look).getByRole('button', { name: send }).getAttribute('aria-disabled')).toBe(
      'true',
    )
  },
)

test('backing out of a refused form whose offer lapsed hands focus to the offers left', async () => {
  // Arrange
  const refuse = holdWrite(mergePull)
  streamPull(ready, ciOf('passed'))
  const user = userEvent.setup()
  render(<ReviewPanel />)
  await user.click(screen.getByRole('button', { name: 'Merge' }))
  const look = await screen.findByRole('form', { name: 'Merge #42' })
  await user.click(within(look).getByRole('button', { name: 'Merge' }))
  act(() => {
    streamPull(ready, ciOf('running'))
  })
  await act(async () => {
    refuse('CI is running on #42')
    await Promise.resolve()
  })

  // Act
  await user.click(within(look).getByRole('button', { name: 'Cancel' }))

  // Assert
  expect(document.activeElement).toBe(screen.getByRole('group', { name: 'Change #42' }))
})

test('a form put away by its offer lapsing hands focus to the offers left', async () => {
  // Arrange
  streamPull(ready, ciOf('passed'))
  const user = userEvent.setup()
  render(<ReviewPanel />)
  await user.click(screen.getByRole('button', { name: 'Merge' }))
  const form = await screen.findByRole('form', { name: 'Merge #42' })
  await user.click(within(form).getByRole('radio', { name: 'Rebase and merge' }))

  // Act
  act(() => {
    streamPull(ready, ciOf('running'))
  })

  // Assert
  expect(document.activeElement).toBe(screen.getByRole('group', { name: 'Change #42' }))
})
