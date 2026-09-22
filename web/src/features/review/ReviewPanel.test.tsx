import { render, screen } from '@testing-library/react'
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
      body: 'why',
      base: 'main',
      head: 'fix/PROJ-412',
      draft: false,
      needs_push: true,
    }),
  ),
  openPr: vi.fn(() => Promise.resolve('')),
}))
const mockOpenPr = vi.mocked(openPr)

const pull = {
  number: 128,
  url: 'https://forge.example.com/pull/128',
  title: 'Redact tokens in the request log',
  draft: false,
  approvals: 1,
  changes_requested: false,
  mergeable: 'clean' as const,
}

test('shows the pull request and its CI checks', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      review: {
        found: true,
        pull,
        ci: {
          state: 'running',
          total: 2,
          done: 1,
          failed: 1,
          checks: [
            { name: 'build', state: 'passed', url: 'https://forge.example.com/build' },
            { name: 'e2e', state: 'failed', url: '' },
          ],
        },
      },
    }),
  })

  // Act
  render(<ReviewPanel />)

  // Assert
  expect(screen.getByRole('link', { name: /redact tokens/i })).toBeTruthy()
  expect(screen.getByText('build')).toBeTruthy()
  expect(screen.getByText('e2e')).toBeTruthy()
})

test('shows a pull request that has no CI', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ review: { found: true, pull } }),
  })

  // Act
  render(<ReviewPanel />)

  // Assert
  expect(screen.getByRole('heading', { level: 2, name: /redact tokens/i })).toBeTruthy()
})

test('says when there is no open pull request', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ review: { found: false } }),
  })

  // Act
  render(<ReviewPanel />)

  // Assert
  expect(screen.getByText(/no open pull request/i)).toBeTruthy()
})

test('opens a pull request from the composed form on confirm', async () => {
  // Arrange
  const user = userEvent.setup()
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ review: { found: false } }),
  })
  render(<ReviewPanel />)

  // Act: open the form, then confirm it
  await user.click(screen.getByRole('button', { name: /open a pull request/i }))
  await screen.findByRole('form', { name: /open a pull request/i })
  await user.click(screen.getByRole('button', { name: 'Open pull request' }))

  // Assert
  expect(mockOpenPr).toHaveBeenCalledWith(
    expect.objectContaining({ title: 'fix: redact tokens', base: 'main', draft: false }),
  )
  expect(await screen.findByText(/pull request opened/i)).toBeTruthy()
})

test('opens a pull request with reviewers, assignees and labels', async () => {
  // Arrange
  const user = userEvent.setup()
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ review: { found: false } }),
  })
  render(<ReviewPanel />)

  // Act: open the form, fill the people fields, then confirm
  await user.click(screen.getByRole('button', { name: /open a pull request/i }))
  await screen.findByRole('form', { name: /open a pull request/i })
  await user.type(screen.getByLabelText(/reviewers/i), 'ana, ben')
  await user.type(screen.getByLabelText(/assignees/i), 'cass')
  await user.type(screen.getByLabelText(/labels/i), 'bug, review')
  await user.click(screen.getByRole('button', { name: 'Open pull request' }))

  // Assert: each comma-separated field is split into a trimmed list
  expect(mockOpenPr).toHaveBeenCalledWith(
    expect.objectContaining({
      reviewers: ['ana', 'ben'],
      assignees: ['cass'],
      labels: ['bug', 'review'],
    }),
  )
})

test('shows the warning when a pull opens but its reviewers could not be added', async () => {
  // Arrange
  const user = userEvent.setup()
  mockOpenPr.mockResolvedValueOnce('opened, but its reviewers could not all be added')
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ review: { found: false } }),
  })
  render(<ReviewPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: /open a pull request/i }))
  await screen.findByRole('form', { name: /open a pull request/i })
  await user.click(screen.getByRole('button', { name: 'Open pull request' }))

  // Assert
  expect(await screen.findByText(/pull request opened/i)).toBeTruthy()
  expect(screen.getByText(/reviewers could not all be added/i)).toBeTruthy()
})

test('locks the confirm while the pull request is opening', async () => {
  // Arrange
  // Hold the open unresolved so the in-flight state is observable; a live confirm
  // here would let a double click open two pull requests.
  let releaseOpen = () => {}
  mockOpenPr.mockImplementationOnce(
    () =>
      new Promise<string>((resolve) => {
        releaseOpen = () => {
          resolve('')
        }
      }),
  )
  const user = userEvent.setup()
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ review: { found: false } }),
  })
  render(<ReviewPanel />)

  // Act: open the form and confirm, leaving the open unresolved
  await user.click(screen.getByRole('button', { name: /open a pull request/i }))
  await screen.findByRole('form', { name: /open a pull request/i })
  await user.click(screen.getByRole('button', { name: 'Open pull request' }))

  // Assert: the confirm now reads "Opening…" and is disabled, and only one open fired
  const opening = await screen.findByRole('button', { name: /opening/i })
  expect(opening.hasAttribute('disabled')).toBe(true)
  expect(mockOpenPr).toHaveBeenCalledTimes(1)

  releaseOpen()
  await screen.findByText(/pull request opened/i)
})

test('keeps the form and shows the reason when opening is refused', async () => {
  // Arrange
  // A distinctive forge reason, not the component's generic fallback, so the
  // test fails if the reason is dropped for the fallback.
  mockOpenPr.mockRejectedValueOnce({
    code: 'unprocessable',
    detail: 'the base branch trunk does not exist on the forge',
  })
  const user = userEvent.setup()
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ review: { found: false } }),
  })
  render(<ReviewPanel />)

  // Act: open the form and confirm, with the open refused
  await user.click(screen.getByRole('button', { name: /open a pull request/i }))
  await screen.findByRole('form', { name: /open a pull request/i })
  await user.click(screen.getByRole('button', { name: 'Open pull request' }))

  // Assert: the forge's own reason shows and the form is still there to retry
  expect(await screen.findByText(/base branch trunk does not exist/i)).toBeTruthy()
  expect(screen.getByRole('form', { name: /open a pull request/i })).toBeTruthy()
})

test('prompts to connect before any snapshot arrives', () => {
  // Act
  render(<ReviewPanel />)

  // Assert
  expect(screen.getByText(/connecting/i)).toBeTruthy()
})
