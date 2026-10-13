import { screen, within } from '@testing-library/react'
import { mockRepositories } from '@/dev/mockRepositories.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { RepositoriesPanel } from './RepositoriesPanel.tsx'

// workingInTerms is what each term of the Working in facts says, by term.
async function workingInTerms(): Promise<Map<string, string>> {
  const workingIn = await screen.findByRole('region', { name: 'Working in' })

  return new Map(
    within(workingIn)
      .getAllByRole('term')
      .map((term) => [term.textContent, term.nextElementSibling?.textContent ?? '']),
  )
}

test("names the base beside origin in a fork's clone", async () => {
  // Arrange
  const fork = mockRepositories()
  fork.here = { ...fork.here, base: 'github.com/upstream/api' }
  fakeApi({ '/api/repositories': fork, '/api/repositories/favorites': fork })

  // Act
  renderWithClient(<RepositoriesPanel />)

  // Assert
  expect((await workingInTerms()).get('Base')).toBe('github.com/upstream/api')
})

test('names no base outside a fork', async () => {
  // Arrange
  const plain = mockRepositories()
  fakeApi({ '/api/repositories': plain, '/api/repositories/favorites': plain })

  // Act
  renderWithClient(<RepositoriesPanel />)

  // Assert
  expect((await workingInTerms()).has('Base')).toBe(false)
})
