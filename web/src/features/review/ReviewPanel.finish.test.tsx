import { act, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { onTestFinished, vi } from 'vitest'
import type { PullRequest } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { fakeApi, pushFrame } from '@/test/fakeApi.ts'
import { makeBranch, makeSnapshot } from '@/test/fixtures.ts'
import { ReviewPanel } from './ReviewPanel.tsx'

// Finishing a merged branch: the server switches to the base and wakes the
// stream as it answers, and the page hands the answer on only once the frame
// it woke has landed, which already shows the base checked out and starts its
// review afresh.

const merged: PullRequest = {
  number: 12,
  url: 'https://forge.example.com/pull/12',
  title: 'Redact tokens',
  state: 'merged',
  draft: false,
  approvals: 1,
  changes_requested: false,
  mergeable: 'unknown',
}

// onBase is what the frame the finish woke shows: the base checked out, with
// no pull request of its own.
const onBase = makeSnapshot({
  branch: makeBranch({ name: 'main' }),
  review: { found: false, announced: 'none' },
})

// finishing opens the finish's last look on fix/PROJ-1, whose pull request
// merged, with the frame its finish wakes held back.
async function finishing() {
  const user = userEvent.setup()
  fakeApi({ '/api/branch/finish': makeBranch({ name: 'main' }) }, { frames: 'held' })
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branch: makeBranch({ name: 'fix/PROJ-1', ahead: 0, finish_commands: ['git switch main'] }),
      review: { found: true, announced: 'none', pull: merged },
    }),
  })
  render(<ReviewPanel />)
  await user.click(screen.getByRole('button', { name: 'Finish the branch' }))
  const form = screen.getByRole('form', { name: 'Finish fix/PROJ-1' })
  await user.click(within(form).getByRole('button', { name: 'Finish' }))
}

// onALoadedMachine has every read of the clock find a frame's time already
// spent, as on a machine under load, so React's scheduler yields to the page
// between each of its tasks rather than running the next in the same one.
function onALoadedMachine() {
  let now = performance.now()
  const clock = vi.spyOn(performance, 'now').mockImplementation(() => (now += 10))
  onTestFinished(() => {
    clock.mockRestore()
  })
}

// focusAsShown is what has focus the moment text is first shown on the page,
// before anything later has run.
function focusAsShown(text: string): Promise<{ shown: Element; focused: Element | null }> {
  return new Promise((resolve) => {
    const observer = new MutationObserver(() => {
      const shown = screen.queryByText(text)
      if (shown !== null) {
        observer.disconnect()
        resolve({ shown, focused: document.activeElement })
      }
    })
    observer.observe(document.body, { childList: true, subtree: true, characterData: true })
  })
}

test('a finish says it finished once the frame showing the base has landed', async () => {
  // Arrange
  await finishing()

  // Act
  act(() => {
    pushFrame(onBase)
  })

  // Assert
  expect(await screen.findByText('Finished fix/PROJ-1; now on main.')).toBeTruthy()
})

test('a finish hands focus to what it said as it says it, its button gone with the branch', async () => {
  // Arrange
  await finishing()
  onALoadedMachine()
  const said = focusAsShown('Finished fix/PROJ-1; now on main.')

  // Act
  act(() => {
    pushFrame(onBase)
  })

  // Assert
  const { shown, focused } = await said
  expect(focused).toBe(shown)
})

test('what a finish said goes once another branch is checked out', async () => {
  // Arrange
  await finishing()
  act(() => {
    pushFrame(onBase)
  })
  await screen.findByText('Finished fix/PROJ-1; now on main.')

  // Act
  act(() => {
    pushFrame(makeSnapshot({ branch: makeBranch({ name: 'feat/PROJ-2' }) }))
  })

  // Assert
  expect(screen.queryByText('Finished fix/PROJ-1; now on main.')).toBeNull()
})
