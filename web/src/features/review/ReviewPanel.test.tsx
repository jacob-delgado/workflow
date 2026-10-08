import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import type { OpenedPullRequest } from '@/api/generated/types.gen.ts'
import { useHealthStore } from '@/api/health.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { held } from '@/test/fakeApi.ts'
import { gitLabWords, makeHealth, makeSnapshot } from '@/test/fixtures.ts'
import { drawnMark, markShape } from '@/test/marks.tsx'
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
      reviewers: [],
      templates: [],
      template: '',
    }),
  ),
  openPr: vi.fn(() =>
    Promise.resolve({
      pull: {
        number: 7,
        url: 'https://forge.example.com/pull/7',
        title: 'fix: redact tokens',
        state: 'open',
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
    state: 'open',
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
  state: 'open' as const,
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
        announced: false,
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
  const title = screen.getByRole('link', { name: /^Redact tokens.*\(opens in a new tab\)$/ })
  expect(title.getAttribute('target')).toBe('_blank')
  const build = screen.getByRole('link', { name: 'build (opens in a new tab)' })
  expect(build.getAttribute('href')).toBe('https://forge.example.com/build')
  expect(build.getAttribute('target')).toBe('_blank')
  expect(screen.getByText('e2e')).toBeTruthy()
  expect(screen.queryByRole('link', { name: /e2e/ })).toBeNull()
})

// The heading counts the checks that are done, and the failed ones only when
// there are any.
const ciCounts = [
  { done: 1, failed: 1, heading: 'CI checks 1 of 2 done, 1 failed' },
  { done: 2, failed: 0, heading: 'CI checks 2 of 2 done' },
] as const

test.each(ciCounts)(
  'heads the CI checks with how many are done: $heading',
  ({ done, failed, heading }) => {
    // Arrange
    useSnapshotStore.setState({
      status: 'live',
      snapshot: makeSnapshot({
        review: {
          found: true,
          announced: false,
          pull,
          ci: {
            state: failed > 0 ? 'failed' : 'passed',
            total: 2,
            done,
            failed,
            checks: [
              { name: 'build', state: 'passed', url: '' },
              { name: 'e2e', state: failed > 0 ? 'failed' : 'passed', url: '' },
            ],
          },
        },
      }),
    })

    // Act
    render(<ReviewPanel />)

    // Assert
    // The count is an element apart, which a browser names after a space and
    // the test's DOM without one; the dot before it is never named.
    const name = new RegExp(`^CI checks\\s?${heading.slice('CI checks '.length)}$`)
    expect(screen.getByRole('heading', { level: 3, name })).toBeTruthy()
  },
)

test('draws each check as the mark of how it stands, beside the words', () => {
  // Arrange
  const checks = [
    { name: 'lint', state: 'none', mark: 'unknown' },
    { name: 'e2e', state: 'running', mark: 'in-flight' },
    { name: 'build', state: 'passed', mark: 'done' },
    { name: 'test', state: 'failed', mark: 'failed' },
  ] as const
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      review: {
        found: true,
        announced: false,
        pull,
        ci: {
          state: 'failed',
          total: 4,
          done: 3,
          failed: 1,
          checks: checks.map(({ name, state }) => ({ name, state, url: '' })),
        },
      },
    }),
  })

  // Act
  render(<ReviewPanel />)

  // Assert
  const rows = screen.getAllByRole('listitem')
  expect(rows.map(markShape)).toEqual(checks.map(({ mark }) => drawnMark(mark)))
  expect(rows.map((row) => row.textContent)).toEqual([
    'lintnone',
    'e2erunning',
    'buildpassed',
    'testfailed',
  ])
})

test('shows a pull request that has no CI', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ review: { found: true, announced: false, pull } }),
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
    snapshot: makeSnapshot({ review: { found: false, announced: false } }),
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
    snapshot: makeSnapshot({ review: { found: false, announced: false } }),
  })
  render(<ReviewPanel />)

  // Act
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
    snapshot: makeSnapshot({ review: { found: false, announced: false } }),
  })
  render(<ReviewPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: /open a pull request/i }))
  await screen.findByRole('form', { name: /open a pull request/i })
  await user.type(screen.getByLabelText(/reviewers/i), 'ana, ben')
  await user.type(screen.getByLabelText(/assignees/i), 'cass')
  await user.type(screen.getByLabelText(/labels/i), 'bug, review')
  await user.click(screen.getByRole('button', { name: 'Open pull request' }))

  // Assert
  expect(mockOpenPr).toHaveBeenCalledWith(
    expect.objectContaining({
      reviewers: ['ana', 'ben'],
      assignees: ['cass'],
      labels: ['bug', 'review'],
    }),
  )
})

test('pre-fills the reviewers with the code owners the draft proposes', async () => {
  // Arrange
  const user = userEvent.setup()
  mockPreview.mockResolvedValueOnce({
    title: 'fix: redact tokens',
    body: 'why',
    base: 'main',
    head: 'fix/PROJ-412',
    draft: false,
    needs_push: true,
    reviewers: ['ana', 'acme/control-plane'],
    templates: [],
    template: '',
  })
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ review: { found: false, announced: false } }),
  })
  render(<ReviewPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: /open a pull request/i }))
  const reviewers = await screen.findByRole('textbox', { name: /reviewers/i })
  const shown = (reviewers as HTMLInputElement).value
  await user.click(screen.getByRole('button', { name: 'Open pull request' }))

  // Assert
  expect(shown).toBe('ana, acme/control-plane')
  expect(mockOpenPr).toHaveBeenCalledWith(
    expect.objectContaining({ reviewers: ['ana', 'acme/control-plane'] }),
  )
})

test('hints that a reviewer can be a team', async () => {
  // Arrange
  const user = userEvent.setup()
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ review: { found: false, announced: false } }),
  })
  render(<ReviewPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: /open a pull request/i }))

  // Assert
  const reviewers = await screen.findByRole('textbox', { name: /reviewers/i })
  expect(reviewers.getAttribute('placeholder')).toMatch(/org\/team/)
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
    snapshot: makeSnapshot({ review: { found: false, announced: false } }),
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
    snapshot: makeSnapshot({ review: { found: false, announced: false } }),
  })
  render(<ReviewPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: /open a pull request/i }))
  await screen.findByRole('form', { name: /open a pull request/i })
  await user.click(screen.getByRole('button', { name: 'Open pull request' }))

  // Assert
  const opening = await screen.findByRole('button', { name: /opening/i })
  expect(opening.getAttribute('aria-disabled')).toBe('true')
  expect(mockOpenPr).toHaveBeenCalledTimes(1)

  releaseOpen()
  await screen.findByText('Opened pull request #7.')
})

test('keeps the form, the focus and the reason when opening is refused', async () => {
  // Arrange
  // A distinctive forge reason, not the component's generic fallback, so the
  // test fails if the reason is dropped for the fallback.
  const opened = held<never>()
  mockOpenPr.mockReturnValueOnce(opened.promise)
  const user = userEvent.setup()
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ review: { found: false, announced: false } }),
  })
  render(<ReviewPanel />)
  await user.click(screen.getByRole('button', { name: /open a pull request/i }))
  await screen.findByRole('form', { name: /open a pull request/i })
  await user.click(screen.getByRole('button', { name: 'Open pull request' }))
  await screen.findByRole('button', { name: 'Opening…' })

  // Act
  act(() => {
    opened.refuse({
      code: 'unprocessable',
      detail: 'the base branch trunk does not exist on the forge',
    })
  })

  // Assert
  expect(await screen.findByText(/base branch trunk does not exist/i)).toBeTruthy()
  expect(screen.getByRole('form', { name: /open a pull request/i })).toBeTruthy()
  expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Open pull request' }))
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
    snapshot: makeSnapshot({ review: { found: false, announced: false } }),
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
    snapshot: makeSnapshot({
      review: { found: true, announced: false, pull: { ...pull, number: 7 } },
    }),
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
    snapshot: makeSnapshot({ review: { found: false, announced: false } }),
  })

  // Act: draw the panel
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

test('says a merge request could not be composed, on GitLab, with focus still on its button', async () => {
  // Arrange
  mockPreview.mockRejectedValueOnce({})
  const user = userEvent.setup()
  useHealthStore.setState({ health: makeHealth(gitLabWords) })
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ review: { found: false, announced: false } }),
  })
  render(<ReviewPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Open a merge request' }))

  // Assert
  expect(
    await screen.findByText(
      'The merge request could not be composed. Try again, or run workflow pr from a terminal.',
    ),
  ).toBeTruthy()
  expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Open a merge request' }))
})

test('says a merge request could not be opened, on GitLab, when the forge gives no reason', async () => {
  // Arrange
  mockOpenPr.mockRejectedValueOnce({})
  const user = userEvent.setup()
  useHealthStore.setState({ health: makeHealth(gitLabWords) })
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ review: { found: false, announced: false } }),
  })
  render(<ReviewPanel />)
  await user.click(screen.getByRole('button', { name: 'Open a merge request' }))
  await screen.findByRole('form', { name: 'Open a merge request' })

  // Act
  await user.click(screen.getByRole('button', { name: 'Open merge request' }))

  // Assert
  expect(
    await screen.findByText(
      'The merge request was not opened. Try again — your edits are still in the form.',
    ),
  ).toBeTruthy()
})

test('the form opens on its title, and Cancel hands focus back', async () => {
  // Arrange
  const user = userEvent.setup()
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ review: { found: false, announced: false } }),
  })
  render(<ReviewPanel />)

  // Act: open the form
  await user.click(screen.getByRole('button', { name: 'Open a pull request' }))
  await screen.findByRole('form', { name: 'Open a pull request' })

  // Assert: focus is on its first field
  expect(document.activeElement).toBe(screen.getByLabelText('Title'))

  // Act: back out
  await user.click(screen.getByRole('button', { name: 'Cancel' }))

  // Assert: focus is back on the button that opened it
  expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Open a pull request' }))
})

test('names the issue the pull request is for, with a link to its page', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      review: {
        found: true,
        announced: false,
        pull,
        ci: null,
        issue: {
          key: '42',
          tracker: 'forge',
          url: 'https://github.com/acme/oss/issues/42',
          origin: 'pull_request',
        },
      },
    }),
  })

  // Act
  render(<ReviewPanel />)

  // Assert
  const link = screen.getByRole('link', { name: '#42 (opens in a new tab)' })
  expect(link.getAttribute('href')).toBe('https://github.com/acme/oss/issues/42')
  expect(link.getAttribute('target')).toBe('_blank')
})
