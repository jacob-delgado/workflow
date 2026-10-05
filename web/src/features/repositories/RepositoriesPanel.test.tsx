import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { DirectoryListing, Repositories } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { mockDirectories, mockRepositories } from '@/dev/mockRepositories.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { RepositoriesPanel } from './RepositoriesPanel.tsx'

// serving answers the section's reads as the mockup does, keeps each write
// asked, and answers a switch with web as where the server works.
function serving(repositories: Repositories = mockRepositories()): Request[] {
  return fakeApi({
    '/api/repositories': repositories,
    '/api/repositories/favorites': repositories,
    '/api/repositories/here': {
      ...repositories,
      here: {
        ...repositories.here,
        dir: '/home/ana/src/web',
        shown: '~/src/web',
        root_shown: '~/src/web',
      },
    },
    '/api/directories': (url: URL): DirectoryListing =>
      mockDirectories(url.searchParams.get('path') ?? '/home/ana/src/api/cmd'),
  })
}

test('where the server works reads as the repository, then the path within it', async () => {
  // Arrange
  serving()

  // Act
  renderWithClient(<RepositoriesPanel />)

  // Assert
  const workingIn = await screen.findByRole('region', { name: 'Working in' })
  expect(within(workingIn).getByText('/cmd').parentElement?.textContent).toBe('~/src/api/cmd')
  expect(within(workingIn).getByText('github.com/acme/api')).toBeTruthy()
})

test('a directory in no repository says so', async () => {
  // Arrange
  const plain = mockRepositories()
  plain.here = { ...plain.here, root: '', root_shown: '', within: '', origin: '', config: [] }
  serving(plain)

  // Act
  renderWithClient(<RepositoriesPanel />)

  // Assert
  expect(await screen.findByText('None: this directory is in no repository')).toBeTruthy()
  expect(screen.getByText('None, so the defaults apply')).toBeTruthy()
})

test('each favorite says what is there now, and one not there cannot be switched to', async () => {
  // Arrange
  serving()

  // Act
  renderWithClient(<RepositoriesPanel />)

  // Assert
  const favorites = await screen.findByRole('list', { name: 'Favorites' })
  expect(within(favorites).getByText('Not there any more')).toBeTruthy()
  expect(within(favorites).getByRole('button', { name: 'Switch to ~/src/web' })).toBeTruthy()
  expect(within(favorites).queryByRole('button', { name: 'Switch to ~/old-site' })).toBeNull()
})

test('each worktree says what it has checked out, and only another one there can be switched to', async () => {
  // Arrange
  serving()

  // Act
  renderWithClient(<RepositoriesPanel />)

  // Assert
  const worktrees = await screen.findByRole('list', { name: 'Worktrees' })
  const said = within(worktrees)
    .getAllByRole('listitem')
    .map((row) => row.textContent)
  expect(said).toEqual([
    '~/src/apiWhere you work, on main',
    '~/src/api-feat-PROJ-7-rate-limitsOn feat/PROJ-7-rate-limits, lockedSwitch',
    '~/src/api-reviewAt 5e0b7aa, detachedSwitch',
    '~/src/api-spikeNot there any more',
  ])
  expect(within(worktrees).getByRole('button', { name: 'Switch to ~/src/api-review' })).toBeTruthy()
  expect(within(worktrees).queryByRole('button', { name: 'Switch to ~/src/api' })).toBeNull()
  expect(within(worktrees).queryByRole('button', { name: 'Switch to ~/src/api-spike' })).toBeNull()
})

test('switching to a worktree asks first, then switches', async () => {
  // Arrange
  const asked = serving()
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  const worktrees = await screen.findByRole('list', { name: 'Worktrees' })
  await user.click(within(worktrees).getByRole('button', { name: 'Switch to ~/src/api-review' }))

  // Act
  await user.click(screen.getByRole('button', { name: 'Switch' }))

  // Assert
  expect(await screen.findByText('Switched to ~/src/web.')).toBeTruthy()
  const switched = asked.find((request) => request.url.endsWith('/api/repositories/here'))
  expect(await switched?.json()).toEqual({ dir: '/home/ana/src/api-review' })
})

test('worktrees that cannot be read say why, and the favorites still list', async () => {
  // Arrange
  const unread = mockRepositories()
  unread.worktrees = []
  unread.worktrees_error = 'the server is not running in a git repository'
  serving(unread)

  // Act
  renderWithClient(<RepositoriesPanel />)

  // Assert
  expect(
    await screen.findByText(
      'The worktrees could not be read: the server is not running in a git repository',
    ),
  ).toBeTruthy()
  expect(screen.getByRole('list', { name: 'Favorites' })).toBeTruthy()
})

test('with no worktrees to list there is no Worktrees heading', async () => {
  // Arrange
  const none = mockRepositories()
  none.worktrees = []
  serving(none)

  // Act
  renderWithClient(<RepositoriesPanel />)

  // Assert
  await screen.findByRole('list', { name: 'Favorites' })
  expect(screen.queryByRole('heading', { name: 'Worktrees' })).toBeNull()
})

test('adding where you work to the favorites says so', async () => {
  // Arrange
  const asked = serving()
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)

  // Act
  await user.click(await screen.findByRole('button', { name: 'Add to favorites' }))

  // Assert
  expect(await screen.findByText('Added ~/src/api/cmd to favorites.')).toBeTruthy()
  const write = asked.find((request) => request.method === 'PUT')
  expect(await write?.json()).toEqual({ dir: '/home/ana/src/api/cmd' })
})

test('a switch is asked once more, then made', async () => {
  // Arrange
  const asked = serving()
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  await user.click(await screen.findByRole('button', { name: 'Switch to ~/src/web' }))

  // Act
  const question = screen.getByRole('region', { name: 'Switch to ~/src/web?' })
  await user.click(within(question).getByRole('button', { name: 'Switch' }))

  // Assert
  expect(await screen.findByText('Switched to ~/src/web.')).toBeTruthy()
  const write = asked.find((request) => new URL(request.url).pathname === '/api/repositories/here')
  expect(await write?.json()).toEqual({ dir: '/home/ana/src/web' })
})

test('a switch canceled is not made', async () => {
  // Arrange
  const asked = serving()
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  await user.click(await screen.findByRole('button', { name: 'Switch to ~/src/web' }))

  // Act
  await user.click(screen.getByRole('button', { name: 'Cancel' }))

  // Assert
  expect(screen.queryByRole('region', { name: 'Switch to ~/src/web?' })).toBeNull()
  expect(asked.some((request) => request.method === 'PUT')).toBe(false)
})

test('the picker goes up, and opens a directory by its name', async () => {
  // Arrange
  serving()
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  await user.click(await screen.findByRole('button', { name: 'Up' }))
  await screen.findByRole('list', { name: 'Directories in ~/src/api' })
  await user.click(screen.getByRole('button', { name: 'Up' }))

  // Act
  await user.click(await screen.findByRole('button', { name: 'Open api' }))

  // Assert
  expect(await screen.findByRole('list', { name: 'Directories in ~/src/api' })).toBeTruthy()
})

test('a typed path is shown, and its directories listed', async () => {
  // Arrange
  serving()
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)

  // Act
  await user.type(await screen.findByRole('textbox', { name: 'Directory' }), '/home/ana/src')
  await user.click(screen.getByRole('button', { name: 'Show' }))

  // Assert
  const listed = await screen.findByRole('list', { name: 'Directories in ~/src' })
  expect(within(listed).getByRole('button', { name: 'Open web' })).toBeTruthy()
  expect(within(listed).getAllByText('Repository')).toHaveLength(2)
})

test('a switch asked from the picker takes the focus to its question', async () => {
  // Arrange
  // The picker is at the foot of the section, the question at its head.
  serving()
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)

  // Act
  await user.click(await screen.findByRole('button', { name: 'Switch here' }))

  // Assert
  expect(document.activeElement).toBe(
    screen.getByRole('heading', { name: 'Switch to ~/src/api/cmd?' }),
  )
})

test('removing where you work forgets the favorite by the name it was kept under', async () => {
  // Arrange
  // A favorite kept through a link to where the server works is that
  // directory under another name; that name is the one to forget.
  const linked = mockRepositories()
  linked.favorites = [
    { dir: '/home/ana/linked/cmd', shown: '~/linked/cmd', state: 'here', origin: '' },
  ]
  const asked = serving(linked)
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)

  // Act
  await user.click(await screen.findByRole('button', { name: 'Remove from favorites' }))

  // Assert
  await screen.findByText('Removed ~/linked/cmd from favorites.')
  const write = asked.find((request) => request.method === 'DELETE')
  expect(new URL(write?.url ?? '').searchParams.get('dir')).toBe('/home/ana/linked/cmd')
})

test('a switch made here tells the page where it works at once', async () => {
  // Arrange
  // Until it does, the page's writes name the directory left, and the server
  // refuses them.
  serving()
  useSnapshotStore.setState({ snapshot: makeSnapshot({ here: '/home/ana/src/api/cmd' }) })
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  await user.click(await screen.findByRole('button', { name: 'Switch to ~/src/web' }))

  // Act
  await user.click(
    within(screen.getByRole('region', { name: 'Switch to ~/src/web?' })).getByRole('button', {
      name: 'Switch',
    }),
  )

  // Assert
  await screen.findByText('Switched to ~/src/web.')
  expect(useSnapshotStore.getState().snapshot?.here).toBe('/home/ana/src/web')
})

test('where the server works that cannot be read says why, and reads again on Retry', async () => {
  // Arrange
  let answers = 0
  fakeApi({
    '/api/repositories': () => {
      answers++

      return answers === 1
        ? Response.json(
            {
              type: 'about:blank',
              title: 'Internal',
              status: 500,
              code: 'internal',
              detail: 'the request failed',
            },
            { status: 500 },
          )
        : mockRepositories()
    },
    '/api/directories': mockDirectories('/home/ana/src/api/cmd'),
  })
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)

  // Act
  await user.click(await screen.findByRole('button', { name: 'Retry' }))

  // Assert
  expect(await screen.findByRole('region', { name: 'Working in' })).toBeTruthy()
})

test('favorites the server cannot keep offer no change', async () => {
  // Arrange
  const unkept = mockRepositories()
  unkept.favorites_kept = false
  serving(unkept)

  // Act
  renderWithClient(<RepositoriesPanel />)

  // Assert
  expect(await screen.findByText(/Favorites are not kept here/)).toBeTruthy()
  expect(screen.queryByRole('button', { name: /Remove .* from favorites/ })).toBeNull()
  expect(screen.queryByRole('button', { name: 'Add to favorites' })).toBeNull()
})

test('no favorites yet invites adding one', async () => {
  // Arrange
  const none = mockRepositories()
  none.favorites = []
  serving(none)

  // Act
  renderWithClient(<RepositoriesPanel />)

  // Assert
  expect(await screen.findByText(/No favorites yet/)).toBeTruthy()
})
