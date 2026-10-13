import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import type { PullRequestDraft } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { previewPullRequest } from './openPrApi.ts'
import { ReviewPanel } from './ReviewPanel.tsx'

// drafted is the draft composed past base: release holds one commit of the
// branch's own, and its code owner is bea.
function drafted(base = 'main'): PullRequestDraft {
  const onRelease = base === 'release'

  return {
    title: onRelease ? 'fix: only past release' : 'fix: redact tokens',
    body: onRelease ? '- fix: only past release\n' : '- fix: redact tokens\n',
    base,
    head: 'fix/PROJ-1',
    draft: false,
    needs_push: false,
    reviewers: onRelease ? ['bea'] : ['ana'],
    templates: [],
    template: '',
  }
}

vi.mock('./openPrApi.ts', () => ({
  previewPullRequest: vi.fn((_template?: string, base?: string) => Promise.resolve(drafted(base))),
  openPr: vi.fn(),
}))
const mockPreview = vi.mocked(previewPullRequest)

// composed opens the form for a branch with no pull request yet.
async function composed() {
  useSnapshotStore.setState({ status: 'live', snapshot: makeSnapshot() })
  const user = userEvent.setup()
  render(<ReviewPanel />)
  await user.click(screen.getByRole('button', { name: 'Open a pull request' }))
  const form = await screen.findByRole('form', { name: 'Open a pull request' })

  return { user, form }
}

// rebased types base into the base field, then leaves it.
async function rebased(user: ReturnType<typeof userEvent.setup>, form: HTMLElement, base: string) {
  const field = within(form).getByRole('textbox', { name: 'Base branch' })
  await user.clear(field)
  await user.type(field, base)
  await user.tab()
}

test('drafts again past the base chosen, once the base field is left', async () => {
  // Arrange
  const { user, form } = await composed()

  // Act
  await rebased(user, form, 'release')

  // Assert
  expect(mockPreview).toHaveBeenLastCalledWith(undefined, 'release')
  await vi.waitFor(() => {
    expect(within(form).getByRole('textbox', { name: 'Title' })).toHaveProperty(
      'value',
      'fix: only past release',
    )
  })
  expect(within(form).getByRole('textbox', { name: 'Reviewers' })).toHaveProperty('value', 'bea')
})

test('keeps a title typed over when the base changes', async () => {
  // Arrange
  const { user, form } = await composed()
  const title = within(form).getByRole('textbox', { name: 'Title' })
  await user.clear(title)
  await user.type(title, 'mine')

  // Act
  await rebased(user, form, 'release')

  // Assert
  await vi.waitFor(() => {
    expect(within(form).getByRole('textbox', { name: 'Description' })).toHaveProperty(
      'value',
      '- fix: only past release\n',
    )
  })
  expect(title).toHaveProperty('value', 'mine')
})

test('reads nothing again for a base left as it was', async () => {
  // Arrange
  const { user, form } = await composed()
  const calls = mockPreview.mock.calls.length

  // Act
  await user.click(within(form).getByRole('textbox', { name: 'Base branch' }))
  await user.tab()

  // Assert
  expect(mockPreview.mock.calls.length).toBe(calls)
})

test('says so, and keeps the draft, for a base whose commits cannot be read', async () => {
  // Arrange
  const { user, form } = await composed()
  mockPreview.mockRejectedValueOnce(new Error('no branch'))

  // Act
  await rebased(user, form, 'nonesuch')

  // Assert
  const said = await within(form).findByText(/could not be read/)
  expect(said.textContent).toBe('The commits past nonesuch could not be read; the draft is kept.')
  expect(within(form).getByRole('textbox', { name: 'Title' })).toHaveProperty(
    'value',
    'fix: redact tokens',
  )
})

test('a template chosen while the base is read again is not taken for an edit', async () => {
  // Arrange
  const templated = (base?: string, template = 'feature'): PullRequestDraft => ({
    ...drafted(base),
    body: `## ${template}\n${drafted(base).body}`,
    templates: ['feature', 'bugfix'],
    template,
  })
  let answer: (draft: PullRequestDraft) => void = () => undefined
  mockPreview
    .mockImplementationOnce(() => Promise.resolve(templated()))
    .mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          answer = resolve
        }),
    )
    .mockImplementationOnce((template?: string) => Promise.resolve(templated('main', template)))
  const { user, form } = await composed()
  await rebased(user, form, 'release')
  await user.selectOptions(within(form).getByRole('combobox', { name: 'Template' }), 'bugfix')
  await vi.waitFor(() => {
    expect(within(form).getByRole('textbox', { name: 'Description' })).toHaveProperty(
      'value',
      '## bugfix\n- fix: redact tokens\n',
    )
  })

  // Act
  answer(templated('release', 'bugfix'))

  // Assert
  await vi.waitFor(() => {
    expect(within(form).getByRole('textbox', { name: 'Description' })).toHaveProperty(
      'value',
      '## bugfix\n- fix: only past release\n',
    )
  })
  expect(within(form).getByRole('combobox', { name: 'Template' })).toHaveProperty('disabled', false)
})

test('leaving the base as it was drafted onto clears what a failed read said', async () => {
  // Arrange
  const { user, form } = await composed()
  mockPreview.mockRejectedValueOnce(new Error('no branch'))
  await rebased(user, form, 'nonesuch')
  await within(form).findByText(/could not be read/)

  // Act
  await rebased(user, form, 'main')

  // Assert
  expect(within(form).queryByText(/could not be read/)).toBeNull()
})
