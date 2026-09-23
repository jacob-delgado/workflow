import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import type { OpenedPullRequest } from '@/api/generated/types.gen.ts'
import { useHealthStore } from '@/api/health.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { gitLabWords, makeHealth, makeSnapshot } from '@/test/fixtures.ts'
import { openPr, previewPullRequest } from './openPrApi.ts'
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
  openPr: vi.fn(() =>
    Promise.resolve({
      pull: {
        number: 7,
        url: 'https://forge.example.com/pull/7',
        title: 'fix: redact tokens',
        draft: false,
        approvals: 0,
        changes_requested: false,
        mergeable: 'unknown',
      },
      follow_ups: [],
    }),
  ),
}))
const mockOpenPr = vi.mocked(openPr)
const mockPreview = vi.mocked(previewPullRequest)

// opened is what the open answers when it offers nothing more.
const opened: OpenedPullRequest = {
  pull: {
    number: 7,
    url: 'https://forge.example.com/pull/7',
    title: 'fix: redact tokens',
    draft: false,
    approvals: 0,
    changes_requested: false,
    mergeable: 'unknown',
  },
  follow_ups: [],
}

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
  expect(await screen.findByText('Opened pull request #7.')).toBeTruthy()
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
  mockOpenPr.mockResolvedValueOnce({
    ...opened,
    warning: 'opened, but its reviewers could not all be added',
  })
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
  expect(await screen.findByText('Opened pull request #7.')).toBeTruthy()
  expect(screen.getByText(/reviewers could not all be added/i)).toBeTruthy()
})

test('locks the confirm while the pull request is opening', async () => {
  // Arrange
  // Hold the open unresolved so the in-flight state is observable; a live confirm
  // here would let a double click open two pull requests.
  let releaseOpen = () => {}
  mockOpenPr.mockImplementationOnce(
    () =>
      new Promise<OpenedPullRequest>((resolve) => {
        releaseOpen = () => {
          resolve(opened)
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
  await screen.findByText('Opened pull request #7.')
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

test('a form opened again after a refused open and a cancel starts without the old reason', async () => {
  // Arrange
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
  await user.click(screen.getByRole('button', { name: /open a pull request/i }))
  await screen.findByRole('form', { name: /open a pull request/i })
  await user.click(screen.getByRole('button', { name: 'Open pull request' }))
  await screen.findByText(/base branch trunk does not exist/i)
  await user.click(screen.getByRole('button', { name: 'Cancel' }))

  // Act
  await user.click(screen.getByRole('button', { name: /open a pull request/i }))

  // Assert
  await screen.findByRole('form', { name: /open a pull request/i })
  expect(screen.queryByText(/base branch trunk does not exist/i)).toBeNull()
})

test('names a merge request, marked with its !number, on GitLab', () => {
  // Arrange
  useHealthStore.setState({ health: makeHealth(gitLabWords) })
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ review: { found: true, pull: { ...pull, number: 7 } } }),
  })

  // Act
  render(<ReviewPanel />)

  // Assert
  expect(screen.getByRole('heading', { level: 2 }).textContent).toMatch(/^!7/)
  expect(document.body.textContent).not.toMatch(/pull request/i)
})

test('offers and opens a merge request in GitLab words throughout', async () => {
  // Arrange
  const user = userEvent.setup()
  useHealthStore.setState({ health: makeHealth(gitLabWords) })
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ review: { found: false } }),
  })
  render(<ReviewPanel />)

  // Assert: the offer names a merge request
  expect(screen.getByText('No open merge request for this branch yet.')).toBeTruthy()
  expect(document.body.textContent).not.toMatch(/pull request/i)

  // Act: open the form
  await user.click(screen.getByRole('button', { name: 'Open a merge request' }))
  await screen.findByRole('form', { name: 'Open a merge request' })

  // Assert: the form names it too
  expect(screen.getByRole('button', { name: 'Open merge request' })).toBeTruthy()
  expect(document.body.textContent).not.toMatch(/pull request/i)

  // Act: confirm it
  await user.click(screen.getByRole('button', { name: 'Open merge request' }))

  // Assert: and so does the outcome
  expect(await screen.findByText('Opened merge request !7.')).toBeTruthy()
  expect(document.body.textContent).not.toMatch(/pull request/i)
})

test('says a merge request could not be composed, on GitLab, when the forge gives no reason', async () => {
  // Arrange
  mockPreview.mockRejectedValueOnce({})
  const user = userEvent.setup()
  useHealthStore.setState({ health: makeHealth(gitLabWords) })
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ review: { found: false } }),
  })
  render(<ReviewPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Open a merge request' }))

  // Assert
  expect(await screen.findByText('A merge request could not be composed.')).toBeTruthy()
})

test('says a merge request could not be opened, on GitLab, when the forge gives no reason', async () => {
  // Arrange
  mockOpenPr.mockRejectedValueOnce({})
  const user = userEvent.setup()
  useHealthStore.setState({ health: makeHealth(gitLabWords) })
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ review: { found: false } }),
  })
  render(<ReviewPanel />)
  await user.click(screen.getByRole('button', { name: 'Open a merge request' }))
  await screen.findByRole('form', { name: 'Open a merge request' })

  // Act
  await user.click(screen.getByRole('button', { name: 'Open merge request' }))

  // Assert
  expect(await screen.findByText('The merge request could not be opened.')).toBeTruthy()
})

test('prompts to connect before any snapshot arrives', () => {
  // Act
  render(<ReviewPanel />)

  // Assert
  expect(screen.getByText(/connecting/i)).toBeTruthy()
})
