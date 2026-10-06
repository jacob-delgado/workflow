import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { mockDirectories, mockRepositories } from '@/dev/mockRepositories.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { RepositoriesPanel } from './RepositoriesPanel.tsx'

// refusal is a problem the server answers a refused request with.
function refusal(detail: string): Response {
  return Response.json(
    { type: 'about:blank', title: 'Conflict', status: 409, code: 'conflict', detail },
    { status: 409 },
  )
}

// servingBut answers the section's reads as the mockup does, but each route
// in refused with a problem naming it.
function servingBut(...refused: string[]): void {
  const routes: Record<string, unknown> = {
    '/api/repositories': mockRepositories(),
    '/api/repositories/favorites': mockRepositories(),
    '/api/repositories/here': mockRepositories(),
    '/api/directories': (url: URL) =>
      mockDirectories(url.searchParams.get('path') ?? '/home/ana/src/api/cmd'),
  }
  for (const path of refused) {
    routes[path] = () => refusal(`${path} refused`)
  }
  fakeApi(routes)
}

test('where the server works that cannot be read is an alert, with Try again', async () => {
  // Arrange
  servingBut('/api/repositories')

  // Act
  renderWithClient(<RepositoriesPanel />)

  // Assert
  expect((await screen.findByRole('alert')).textContent).toBe('/api/repositories refused')
  expect(screen.getByRole('button', { name: 'Try again' })).toBeTruthy()
})

test('worktrees that cannot be read are an alert', async () => {
  // Arrange
  const unread = mockRepositories()
  unread.worktrees = []
  unread.worktrees_error = 'not in a git repository'
  fakeApi({ '/api/repositories': unread, '/api/directories': mockDirectories('/home/ana') })

  // Act
  renderWithClient(<RepositoriesPanel />)

  // Assert
  expect((await screen.findByRole('alert')).textContent).toBe(
    'The worktrees could not be read: not in a git repository',
  )
})

test('a favorite that cannot be changed is an alert', async () => {
  // Arrange
  servingBut('/api/repositories/favorites')
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)

  // Act
  await user.click(await screen.findByRole('button', { name: 'Add to favorites' }))

  // Assert
  expect((await screen.findByRole('alert')).textContent).toBe('/api/repositories/favorites refused')
})

test('a refused switch is an alert in its question', async () => {
  // Arrange
  servingBut('/api/repositories/here')
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  await user.click(await screen.findByRole('button', { name: 'Switch to ~/src/web' }))
  const question = screen.getByRole('region', { name: 'Switch to ~/src/web?' })

  // Act
  await user.click(within(question).getByRole('button', { name: 'Switch' }))

  // Assert
  expect((await within(question).findByRole('alert')).textContent).toBe(
    '/api/repositories/here refused',
  )
})

test('a directory that cannot be listed is an alert', async () => {
  // Arrange
  servingBut('/api/directories')

  // Act
  renderWithClient(<RepositoriesPanel />)

  // Assert
  expect((await screen.findByRole('alert')).textContent).toBe('/api/directories refused')
})
