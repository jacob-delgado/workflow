import { act, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import type { PullRequestDraft } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { previewPullRequest } from './openPrApi.ts'
import { ReviewPanel } from './ReviewPanel.tsx'

// drafted is the draft composed from a template, of the two the repository has.
function drafted(template: string): PullRequestDraft {
  return {
    title: 'fix: redact tokens',
    body: `## ${template}\n`,
    base: 'main',
    head: 'fix/PROJ-1',
    draft: false,
    needs_push: false,
    reviewers: [],
    templates: ['feature', 'bugfix'],
    template,
  }
}

vi.mock('./openPrApi.ts', () => ({
  previewPullRequest: vi.fn((template?: string) => Promise.resolve(drafted(template ?? 'feature'))),
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

test('starts the description from the template chosen, as ctrl+t does', async () => {
  // Arrange
  const { user, form } = await composed()

  // Act
  await user.selectOptions(within(form).getByRole('combobox', { name: 'Template' }), 'bugfix')

  // Assert
  expect(mockPreview).toHaveBeenLastCalledWith('bugfix', 'main')
  await vi.waitFor(() => {
    expect(within(form).getByRole('textbox', { name: 'Description' })).toHaveProperty(
      'value',
      '## bugfix\n',
    )
  })
})

test('keeps an edited description: the template no longer changes it', async () => {
  // Arrange
  const { user, form } = await composed()

  // Act
  await user.type(within(form).getByRole('textbox', { name: 'Description' }), 'mine')

  // Assert
  expect(within(form).getByRole('combobox', { name: 'Template' })).toHaveProperty('disabled', true)
})

test('keeps a description typed while the template chosen is read', async () => {
  // Arrange
  const { user, form } = await composed()
  let answer: (draft: PullRequestDraft) => void = () => undefined
  mockPreview.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        answer = resolve
      }),
  )
  await user.selectOptions(within(form).getByRole('combobox', { name: 'Template' }), 'bugfix')
  const description = within(form).getByRole('textbox', { name: 'Description' })
  await user.type(description, 'mine')

  // Act
  await act(async () => {
    answer(drafted('bugfix'))
    await Promise.resolve()
  })

  // Assert
  expect(description).toHaveProperty('value', '## feature\nmine')
})

test('the people and labels boxes show their hints in sentence case', async () => {
  // Act
  const { form } = await composed()

  // Assert
  const hints = within(form)
    .getAllByRole('textbox')
    .map((box) => box.getAttribute('placeholder'))
    .filter((hint) => hint !== null)
  expect(hints).toEqual([
    'Comma-separated usernames or org/team',
    'Comma-separated usernames',
    'Comma-separated labels',
  ])
})

test('a template chosen before another and read after it never fills the description', async () => {
  // Arrange
  const { user, form } = await composed()
  const answers: Record<string, () => void> = {}
  const held = (template?: string) =>
    new Promise<PullRequestDraft>((resolve) => {
      answers[template ?? ''] = () => {
        resolve(drafted(template ?? 'feature'))
      }
    })
  mockPreview.mockImplementationOnce(held).mockImplementationOnce(held)
  const choice = within(form).getByRole('combobox', { name: 'Template' })
  await user.selectOptions(choice, 'bugfix')
  await user.selectOptions(choice, 'feature')
  await act(async () => {
    answers.feature?.()
    await Promise.resolve()
  })

  // Act
  await act(async () => {
    answers.bugfix?.()
    await new Promise((resolve) => setTimeout(resolve, 0))
  })

  // Assert
  expect(choice).toHaveProperty('value', 'feature')
  expect(within(form).getByRole('textbox', { name: 'Description' })).toHaveProperty(
    'value',
    '## feature\n',
  )
})
