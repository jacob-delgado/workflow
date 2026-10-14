import { screen, waitFor, within } from '@testing-library/react'
import userEvent, { type UserEvent } from '@testing-library/user-event'
import { mockRepositories } from '@/dev/mockRepositories.ts'
import { listing, openDialog, options, serving } from '@/test/directoryDialog.ts'
import { fakeApi, held } from '@/test/fakeApi.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { RepositoriesPanel } from './RepositoriesPanel.tsx'

// The directory dialog, as a user meets it in Repositories: a field over the
// directories it names, narrowed as a name is typed, and a switch asked once
// more inside it. How it reads a directory, and what it says when a read is
// cut short or refused, is in DirectoryDialog.reads.test.tsx.

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

test('a bare name narrows the worktrees and favorites by their directories’ names', async () => {
  // Arrange
  serving()
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  const field = await openDialog(user)

  // Act
  await user.type(field, 'rev')

  // Assert
  await waitFor(() => {
    expect(options()).toEqual(['~/src/api-reviewworktree', 'switch here~/src/api/cmd'])
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

test('Backspace after your home goes on up, to the directory it is in', async () => {
  // Arrange
  serving()
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  const field = await openDialog(user)
  await user.type(field, '~/')
  await waitFor(() => {
    expect(options()).toEqual(['switch here~', 'notes', 'src'])
  })

  // Act
  await user.keyboard('{Backspace}')

  // Assert
  expect(field).toHaveProperty('value', '/home/')
  await waitFor(() => {
    expect(options()).toEqual(['switch here/home', 'ana'])
  })
})

test('your home names the directories above it, each a way up', async () => {
  // Arrange
  serving()
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  const field = await openDialog(user)

  // Act
  await user.type(field, '~/')

  // Assert
  const trail = await screen.findByRole('navigation', { name: 'Up from here' })
  expect(trail.textContent).toBe('/home/ana')
  expect(within(trail).getByRole('button', { name: 'Up to /home' })).toBeTruthy()
})

test('a path from the root names the root above it once', async () => {
  // Arrange
  serving()
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  const field = await openDialog(user)

  // Act
  await user.type(field, '/home/')

  // Assert
  const trail = await screen.findByRole('navigation', { name: 'Up from here' })
  expect(trail.textContent).toBe('/home')
  expect(
    within(trail)
      .getAllByRole('button')
      .map((up) => up.getAttribute('aria-label')),
  ).toEqual(['Up to /'])
})

test('the root has nothing above it to go up to', async () => {
  // Arrange
  serving()
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  const field = await openDialog(user)

  // Act
  await user.type(field, '/')

  // Assert
  await waitFor(() => {
    expect(options()).toEqual(['switch here/', 'home'])
  })
  expect(screen.queryByRole('navigation', { name: 'Up from here' })).toBeNull()
})

test.each([
  ['the whole path selected', 0, 10, ''],
  ['the caret inside it', 4, 4, '~/sc/api/'],
])('Backspace with %s edits the path rather than going up', async (_, start, end, edited) => {
  // Arrange
  serving()
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  const field = await openDialog(user)
  await user.type(field, '~/src/api/')
  ;(field as HTMLInputElement).setSelectionRange(start, end)

  // Act
  await user.keyboard('{Backspace}')

  // Assert
  expect(field).toHaveProperty('value', edited)
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

  // Assert: the list is back, as it was, with the focus in its field
  const back = await screen.findByRole('combobox', { name: 'Directory' })
  expect(back).toHaveProperty('value', '~/src/')
  expect(document.activeElement).toBe(back)
})

test('a directory above, pressed, is listed, and the focus stays in the field', async () => {
  // Arrange
  serving()
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  const field = await openDialog(user)
  await user.type(field, '~/src/api/')
  const up = await screen.findByRole('button', { name: 'Up to ~/src' })

  // Act
  await user.click(up)

  // Assert
  expect(field).toHaveProperty('value', '~/src/')
  expect(document.activeElement).toBe(field)
  await waitFor(() => {
    expect(options()).toEqual(['switch here~/src', 'apirepository', 'webrepository'])
  })
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
  const dialog = screen.getByRole('dialog', { name: 'Go to a directory' })
  await user.click(within(dialog).getByRole('button', { name: 'Switch to ~/src/web' }))

  // Act
  await user.click(within(dialog).getByRole('button', { name: 'Switch' }))

  // Assert
  expect(await screen.findByText('Switched to ~/src/web.')).toBeTruthy()
  expect(screen.queryByRole('dialog')).toBeNull()
  const write = asked.find((request) => new URL(request.url).pathname === '/api/repositories/here')
  expect(await write?.json()).toEqual({ dir: '/home/ana/src/web' })
})

test('the switch at the foot of the dialog names the row highlighted', async () => {
  // Arrange
  serving()
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  const field = await openDialog(user)
  await user.type(field, '~/src/')
  await waitFor(() => {
    expect(options()).toEqual(['switch here~/src', 'apirepository', 'webrepository'])
  })
  const dialog = screen.getByRole('dialog', { name: 'Go to a directory' })

  // Act
  await user.keyboard('{ArrowDown}')

  // Assert
  expect(within(dialog).getByRole('button', { name: 'Switch to ~/src/api' })).toBeTruthy()
  expect(within(dialog).queryByRole('button', { name: 'Switch to ~/src' })).toBeNull()
})

test('a switch that runs holds the dialog open, which then shows its refusal', async () => {
  // Arrange
  const switching = held<Response>()
  fakeApi({
    '/api/repositories': mockRepositories(),
    '/api/repositories/here': () => switching.promise,
    '/api/directories': listing,
  })
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  const worktrees = await screen.findByRole('list', { name: 'Worktrees' })
  await user.click(within(worktrees).getByRole('button', { name: 'Switch to ~/src/api-review' }))
  const dialog = await screen.findByRole('dialog', { name: 'Go to a directory' })
  await user.click(within(dialog).getByRole('button', { name: 'Switch' }))

  // Act: press Escape while the switch runs
  await user.keyboard('{Escape}')

  // Assert: the dialog stays
  expect(screen.getByRole('dialog', { name: 'Go to a directory' })).toBe(dialog)

  // Act: the server refuses the switch
  switching.answer(
    Response.json(
      { type: 'about:blank', title: 'Conflict', status: 409, code: 'conflict', detail: 'busy' },
      { status: 409 },
    ),
  )

  // Assert: the refusal is shown in it
  expect((await within(dialog).findByRole('alert')).textContent).toBe('busy')
})

test.each([
  ['Escape', (user: UserEvent) => user.keyboard('{Escape}')],
  ['Cancel', (user: UserEvent) => user.click(screen.getByRole('button', { name: 'Cancel' }))],
])('%s closes the dialog while no switch runs', async (_, close) => {
  // Arrange
  serving()
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  await openDialog(user)

  // Act
  await close(user)

  // Assert
  expect(screen.queryByRole('dialog')).toBeNull()
})

test.each([
  ['Escape', (user: UserEvent) => user.keyboard('{Escape}')],
  [
    'Back, then Cancel',
    async (user: UserEvent) => {
      await user.click(screen.getByRole('button', { name: 'Back' }))
      await user.click(screen.getByRole('button', { name: 'Cancel' }))
    },
  ],
])('once a switch is refused, %s closes the dialog', async (_, close) => {
  // Arrange
  fakeApi({
    '/api/repositories': mockRepositories(),
    '/api/repositories/here': () =>
      Response.json(
        { type: 'about:blank', title: 'Conflict', status: 409, code: 'conflict', detail: 'busy' },
        { status: 409 },
      ),
    '/api/directories': listing,
  })
  const user = userEvent.setup()
  renderWithClient(<RepositoriesPanel />)
  const worktrees = await screen.findByRole('list', { name: 'Worktrees' })
  await user.click(within(worktrees).getByRole('button', { name: 'Switch to ~/src/api-review' }))
  const dialog = await screen.findByRole('dialog', { name: 'Go to a directory' })
  await user.click(within(dialog).getByRole('button', { name: 'Switch' }))
  await within(dialog).findByRole('alert')

  // Act
  await close(user)

  // Assert
  expect(screen.queryByRole('dialog')).toBeNull()
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
