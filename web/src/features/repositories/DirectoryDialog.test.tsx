import { screen, waitFor, within } from '@testing-library/react'
import userEvent, { type UserEvent } from '@testing-library/user-event'
import type { DirectoryListing } from '@/api/generated/types.gen.ts'
import { mockDirectories, mockRepositories } from '@/dev/mockRepositories.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { RepositoriesPanel } from './RepositoriesPanel.tsx'

// The directory dialog, as a user meets it in Repositories: a field over the
// directories it names, narrowed as a name is typed, and a switch asked once
// more inside it.

// listing answers a directory read as the mockup does, by its path, or by
// where the mockup works, and its prefix.
function listing(url: URL): DirectoryListing {
  return mockDirectories(url.searchParams.get('path') ?? '.', url.searchParams.get('prefix') ?? '')
}

// serving answers the section's reads, the directory reads by listing unless
// directories answers them, and a switch with web as where the server works.
function serving(directories: (url: URL) => DirectoryListing = listing): Request[] {
  const repositories = mockRepositories()

  return fakeApi({
    '/api/repositories': repositories,
    '/api/repositories/here': {
      ...repositories,
      here: { ...repositories.here, dir: '/home/ana/src/web', shown: '~/src/web' },
    },
    '/api/directories': directories,
  })
}

// openDialog opens the directory dialog and waits for its field to have the
// focus.
async function openDialog(user: UserEvent): Promise<HTMLElement> {
  await user.click(await screen.findByRole('button', { name: 'Go to a directory…' }))
  const field = await screen.findByRole('combobox', { name: 'Directory' })
  await waitFor(() => {
    expect(document.activeElement).toBe(field)
  })

  return field
}

// options is the text of each option the dialog lists, in order.
function options(): (string | null)[] {
  return within(screen.getByRole('dialog', { name: 'Go to a directory' }))
    .getAllByRole('option')
    .map((option) => option.textContent)
}

test('the dialog opens on the worktrees, the favorites, then what is in here', async () => {
  // Arrange
  serving()
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)

  // Act
  await openDialog(user)

  // Assert
  await waitFor(() => {
    expect(options()).toEqual([
      '~/src/api-feat-PROJ-7-rate-limitsworktree',
      '~/src/api-reviewworktree',
      '★ ~/src/webrepository',
      '★ ~/notes',
      'switch here~/src/api/cmd',
    ])
  })
})

test('a typed path lists its directories, narrowed by the name after its last slash', async () => {
  // Arrange
  serving()
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  const field = await openDialog(user)

  // Act
  await user.type(field, '~/src/w')

  // Assert
  await waitFor(() => {
    expect(options()).toEqual(['webrepository', 'switch here~/src'])
  })
  expect(screen.getByText('1 of 2 in ~/src')).toBeTruthy()
})

test('Enter on a directory goes into it, and Backspace with nothing after the slash goes up', async () => {
  // Arrange
  serving()
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  const field = await openDialog(user)
  await user.type(field, '~/src/a')
  await waitFor(() => {
    expect(options()[0]).toBe('apirepository')
  })

  // Act: go into api
  await user.keyboard('{Enter}')

  // Assert: the field names it, and its directories are listed
  expect(field).toHaveProperty('value', '~/src/api/')
  await waitFor(() => {
    expect(options()).toEqual(['switch here~/src/api', 'cmd'])
  })

  // Act: go up
  await user.keyboard('{Backspace}')

  // Assert: the field names the directory above
  expect(field).toHaveProperty('value', '~/src/')
})

test('a name nothing in the directory holds says so, and still offers to switch here', async () => {
  // Arrange
  serving()
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  const field = await openDialog(user)

  // Act
  await user.type(field, '~/src/zz')

  // Assert
  expect(await screen.findByText('Nothing in ~/src matches zz.')).toBeTruthy()
  expect(options()).toEqual(['switch here~/src'])
})

test('switch here asks once more inside the dialog, and Back returns to the list', async () => {
  // Arrange
  serving()
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  const field = await openDialog(user)
  await user.type(field, '~/src/')
  await waitFor(() => {
    expect(options()[0]).toBe('switch here~/src')
  })

  // Act: ask to switch
  await user.keyboard('{Enter}')

  // Assert: the dialog asks
  const dialog = screen.getByRole('dialog', { name: 'Go to a directory' })
  expect(within(dialog).getByRole('group', { name: 'Switch to ~/src?' })).toBeTruthy()

  // Act: go back
  await user.click(within(dialog).getByRole('button', { name: 'Back' }))

  // Assert: the list is back, as it was
  expect(await screen.findByRole('combobox', { name: 'Directory' })).toHaveProperty(
    'value',
    '~/src/',
  )
})

test('a switch confirmed in the dialog is made, says so and closes it', async () => {
  // Arrange
  const asked = serving()
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  const field = await openDialog(user)
  await user.type(field, '~/src/w')
  await waitFor(() => {
    expect(options()[0]).toBe('webrepository')
  })
  await user.click(screen.getByRole('button', { name: 'Switch to ~/src' }))

  // Act
  await user.click(screen.getByRole('button', { name: 'Switch' }))

  // Assert
  expect(await screen.findByText('Switched to ~/src/web.')).toBeTruthy()
  expect(screen.queryByRole('dialog')).toBeNull()
  const write = asked.find((request) => new URL(request.url).pathname === '/api/repositories/here')
  expect(await write?.json()).toEqual({ dir: '/home/ana/src' })
})

test('choosing where the server already works says so rather than asking', async () => {
  // Arrange
  serving()
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  const field = await openDialog(user)
  await user.type(field, '~/src/api/cmd/')
  await waitFor(() => {
    expect(options()[0]).toBe('switch here~/src/api/cmd')
  })

  // Act
  await user.keyboard('{Enter}')

  // Assert
  expect(await screen.findByText('You already work in ~/src/api/cmd.')).toBeTruthy()
  expect(screen.queryByRole('group', { name: /^Switch to/ })).toBeNull()
})

test('a listing cut short is asked again for the names starting with what was typed', async () => {
  // Arrange
  const asked = serving((url) => ({
    ...listing(url),
    truncated: url.searchParams.get('prefix') === null,
  }))
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  const field = await openDialog(user)

  // Act
  await user.type(field, '~/src/we')

  // Assert
  await waitFor(() => {
    expect(asked.some((request) => new URL(request.url).searchParams.get('prefix') === 'we')).toBe(
      true,
    )
  })
})

test('at most two hundred matches are listed, with how many more there are', async () => {
  // Arrange
  const many = Array.from({ length: 241 }, (_, index) => ({
    name: `dir${String(index).padStart(3, '0')}`,
    path: `/home/ana/src/dir${String(index)}`,
    repository: false,
  }))
  serving((url) => ({ ...listing(url), entries: many }))
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  const field = await openDialog(user)

  // Act
  await user.type(field, '~/src/d')

  // Assert
  expect(await screen.findByText('41 more match; keep typing to narrow.')).toBeTruthy()
  expect(options()).toHaveLength(201)
})

test('a directory that cannot be listed is an alert in the dialog, which stays open', async () => {
  // Arrange
  fakeApi({
    '/api/repositories': mockRepositories(),
    '/api/directories': () => new Response('', { status: 500 }),
  })
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)

  // Act
  await openDialog(user)

  // Assert
  const dialog = screen.getByRole('dialog', { name: 'Go to a directory' })
  expect(await within(dialog).findByRole('alert')).toBeTruthy()
})

test('a worktree’s Switch opens the dialog at its question', async () => {
  // Arrange
  serving()
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  const worktrees = await screen.findByRole('list', { name: 'Worktrees' })

  // Act
  await user.click(within(worktrees).getByRole('button', { name: 'Switch to ~/src/api-review' }))

  // Assert
  const dialog = await screen.findByRole('dialog', { name: 'Go to a directory' })
  expect(within(dialog).getByRole('group', { name: 'Switch to ~/src/api-review?' })).toBeTruthy()
})
