import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { mockRepositories } from '@/dev/mockRepositories.ts'
import { useUiStore } from '@/shell/uiStore.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { HereBadge } from './HereBadge.tsx'

test('the header names the repository and the path within it, and opens Repositories', async () => {
  // Arrange
  fakeApi({ '/api/repositories': mockRepositories() })
  useUiStore.setState({ section: 'issues' })
  const user = userEvent.setup()
  renderWithClient(<HereBadge />)
  const badge = await screen.findByRole('button', {
    name: 'Working in ~/src/api/cmd: open Repositories',
  })

  // Act
  await user.click(badge)

  // Assert
  expect(badge.textContent).toBe('api/cmd')
  expect(useUiStore.getState().section).toBe('repositories')
})

test('outside a repository the header names the directory', async () => {
  // Arrange
  const plain = mockRepositories()
  plain.here = {
    ...plain.here,
    dir: '/home/ana/notes',
    shown: '~/notes',
    root: '',
    root_shown: '',
    within: '',
  }
  fakeApi({ '/api/repositories': plain })

  // Act
  renderWithClient(<HereBadge />)

  // Assert
  const badge = await screen.findByRole('button', { name: 'Working in ~/notes: open Repositories' })
  expect(badge.textContent).toBe('notes')
})
