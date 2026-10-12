import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { keysWith, withIssue, withChanges, openOn, commentBox } from '@/test/keyboard.tsx'

// The page's command palette, as a user meets it: Ctrl+K or ⌘K finds an
// action of the section on screen, or another section, by name.

test('⌘K opens the palette too', async () => {
  // Arrange
  const user = userEvent.setup()
  openOn(withIssue, keysWith(false))
  await commentBox()

  // Act
  await user.keyboard('{Meta>}k{/Meta}')

  // Assert
  expect(screen.getByRole('dialog', { name: 'Command palette' })).toBeTruthy()
})

test('Ctrl+K, "stage all", Enter stages every change', async () => {
  // Arrange
  const user = userEvent.setup()
  const requests = openOn(withChanges, keysWith(true), 'branch')
  await waitFor(() => {
    expect(
      screen.getByRole('button', { name: 'Stage all' }).getAttribute('aria-keyshortcuts'),
    ).toBe('a')
  })

  // Act
  await user.keyboard('{Control>}k{/Control}')
  await user.keyboard('stage all')
  await user.keyboard('{Enter}')

  // Assert
  expect(await screen.findByText('Staged every change.')).toBeTruthy()
  const staged = requests.find((request) => new URL(request.url).pathname === '/api/stage')
  expect(await staged?.json()).toEqual({ all: true })
  expect(screen.queryByRole('dialog')).toBeNull()
})

test('the palette lists the section actions on screen by the terminal words, closest first', async () => {
  // Arrange
  const user = userEvent.setup()
  openOn(withChanges, keysWith(true), 'branch')
  await waitFor(() => {
    expect(
      screen.getByRole('button', { name: 'Stage all' }).hasAttribute('aria-keyshortcuts'),
    ).toBe(true)
  })
  await user.keyboard('{Control>}k{/Control}')

  // Act
  await user.keyboard('stage')

  // Assert
  const options = within(screen.getByRole('listbox', { name: 'Actions' }))
    .getAllByRole('option')
    .map((option) => option.textContent)
  expect(options).toEqual(['stage alla', 'unstage allU'])
})

test('Down and Up step through the matches, round from the last to the first', async () => {
  // Arrange
  const user = userEvent.setup()
  openOn(withChanges, keysWith(true), 'branch')
  await waitFor(() => {
    expect(
      screen.getByRole('button', { name: 'Stage all' }).hasAttribute('aria-keyshortcuts'),
    ).toBe(true)
  })
  await user.keyboard('{Control>}k{/Control}')
  await user.keyboard('stage')
  const box = screen.getByRole('combobox', { name: 'Action' })
  const chosen = () =>
    document.getElementById(box.getAttribute('aria-activedescendant') ?? '')?.textContent

  // Act: step down
  await user.keyboard('{ArrowDown}')

  // Assert: the second match is chosen
  expect(chosen()).toBe('unstage allU')

  // Act: step down again
  await user.keyboard('{ArrowDown}')

  // Assert: round to the first
  expect(chosen()).toBe('stage alla')

  // Act: step up
  await user.keyboard('{ArrowUp}')

  // Assert: round to the last
  expect(chosen()).toBe('unstage allU')
})

test('a press on an option runs it', async () => {
  // Arrange
  const user = userEvent.setup()
  const requests = openOn(withChanges, keysWith(true), 'branch')
  await waitFor(() => {
    expect(
      screen.getByRole('button', { name: 'Unstage all' }).hasAttribute('aria-keyshortcuts'),
    ).toBe(true)
  })
  await user.keyboard('{Control>}k{/Control}')

  // Act
  await user.click(screen.getByRole('option', { name: /^unstage all/ }))

  // Assert
  expect(await screen.findByText('Unstaged every change.')).toBeTruthy()
  expect(requests.some((request) => new URL(request.url).pathname === '/api/unstage')).toBe(true)
})

test('the palette goes to another section by its name', async () => {
  // Arrange
  const user = userEvent.setup()
  openOn(withChanges, keysWith(true), 'branch')
  await screen.findByRole('button', { name: 'Stage all' })
  await user.keyboard('{Control>}k{/Control}')

  // Act
  await user.keyboard('issues{Enter}')

  // Assert
  expect(await screen.findByRole('heading', { level: 1, name: 'Issues' })).toBeTruthy()
})

test('the palette says when nothing is called what was typed', async () => {
  // Arrange
  const user = userEvent.setup()
  openOn(withChanges, keysWith(true), 'branch')
  await screen.findByRole('button', { name: 'Stage all' })
  await user.keyboard('{Control>}k{/Control}')

  // Act
  await user.keyboard('frobnicate{Enter}')

  // Assert
  const palette = screen.getByRole('dialog', { name: 'Command palette' })
  expect(within(palette).getByRole('status').textContent).toMatch(/Nothing here is called that/)
})
