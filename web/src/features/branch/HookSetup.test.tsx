import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeBranch, makeSnapshot } from '@/test/fixtures.ts'
import { BranchPanel } from './BranchPanel.tsx'
import { readHookSetup, writeHookSetup } from './gitRunApi.ts'

vi.mock('./gitRunApi.ts', () => ({
  startRun: vi.fn(),
  stopRun: vi.fn(),
  readHookSetup: vi.fn(() =>
    Promise.resolve({
      offered: true,
      hooks: [{ name: 'pre-commit', lines: 3 }],
      config: 'pre-commit:\n  jobs:\n    - run: go vet ./...\n',
      scripts: 0,
    }),
  ),
  writeHookSetup: vi.fn(() => Promise.resolve({ scripts: 0 })),
}))
const mockRead = vi.mocked(readHookSetup)
const mockWrite = vi.mocked(writeHookSetup)
vi.mock('./commitApi.ts', () => ({ commitChanges: vi.fn(() => Promise.resolve(makeBranch())) }))
vi.mock('./pushApi.ts', () => ({ pushBranch: vi.fn(() => Promise.resolve(makeBranch())) }))

// withHooks streams a repository with this many hooks lefthook does not manage.
function withHooks(unmanaged: number) {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ hooks_unmanaged: unmanaged }),
  })
}

test('offers to set lefthook up only while hooks lack it', () => {
  // Arrange
  withHooks(0)

  // Act
  render(<BranchPanel />)

  // Assert
  expect(screen.queryByRole('button', { name: 'Set up lefthook' })).toBeNull()
})

test('shows what it would write before it writes', async () => {
  // Arrange
  withHooks(1)
  const user = userEvent.setup()
  render(<BranchPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Set up lefthook' }))

  // Assert
  const offer = await screen.findByRole('region', { name: 'Set up lefthook' })
  expect(
    within(offer).getByRole('region', { name: 'lefthook.yml that runs them' }).textContent,
  ).toContain('go vet')
  expect(mockRead).toHaveBeenCalled()
  expect(mockWrite).not.toHaveBeenCalled()
})

test('writes lefthook.yml and says so', async () => {
  // Arrange
  withHooks(1)
  const user = userEvent.setup()
  render(<BranchPanel />)
  await user.click(screen.getByRole('button', { name: 'Set up lefthook' }))
  const offer = await screen.findByRole('region', { name: 'Set up lefthook' })

  // Act
  await user.click(within(offer).getByRole('button', { name: 'Write lefthook.yml' }))

  // Assert
  expect(mockWrite).toHaveBeenCalledWith(false)
  expect(await screen.findByText('Wrote lefthook.yml and installed lefthook.')).toBeTruthy()
})

test('writes every hook as a script when asked', async () => {
  // Arrange
  withHooks(1)
  const user = userEvent.setup()
  render(<BranchPanel />)
  await user.click(screen.getByRole('button', { name: 'Set up lefthook' }))
  const offer = await screen.findByRole('region', { name: 'Set up lefthook' })

  // Act
  await user.click(within(offer).getByRole('button', { name: 'Write every hook as a script' }))

  // Assert
  expect(mockWrite).toHaveBeenCalledWith(true)
})

test('backing out writes nothing and hands focus back', async () => {
  // Arrange
  withHooks(1)
  const user = userEvent.setup()
  render(<BranchPanel />)
  await user.click(screen.getByRole('button', { name: 'Set up lefthook' }))
  const offer = await screen.findByRole('region', { name: 'Set up lefthook' })

  // Act
  await user.click(within(offer).getByRole('button', { name: 'Cancel' }))

  // Assert
  expect(mockWrite).not.toHaveBeenCalled()
  expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Set up lefthook' }))
})

test('says when there is nothing left to set up', async () => {
  // Arrange
  mockRead.mockResolvedValueOnce({ offered: false, hooks: [], config: '', scripts: 0 })
  withHooks(1)
  const user = userEvent.setup()
  render(<BranchPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Set up lefthook' }))

  // Assert
  expect(await screen.findByText(/Nothing to set up/)).toBeTruthy()
})

test('says why the hooks could not be read', async () => {
  // Arrange
  mockRead.mockRejectedValueOnce({
    code: 'unprocessable',
    detail: 'setting up lefthook is not available here',
  })
  withHooks(1)
  const user = userEvent.setup()
  render(<BranchPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Set up lefthook' }))

  // Assert
  expect((await screen.findByRole('alert')).textContent).toContain('not available')
})

test.each(['Write lefthook.yml', 'Write every hook as a script'])(
  'says why %s wrote nothing, keeping the offer and the focus',
  async (write) => {
    // Arrange
    mockWrite.mockRejectedValueOnce({
      code: 'unprocessable',
      detail: 'lefthook.yml was not written',
    })
    withHooks(1)
    const user = userEvent.setup()
    render(<BranchPanel />)
    await user.click(screen.getByRole('button', { name: 'Set up lefthook' }))
    const offer = await screen.findByRole('region', { name: 'Set up lefthook' })

    // Act
    await user.click(within(offer).getByRole('button', { name: write }))

    // Assert
    expect((await within(offer).findByRole('alert')).textContent).toContain('was not written')
    expect(document.activeElement).toBe(within(offer).getByRole('button', { name: write }))
  },
)
