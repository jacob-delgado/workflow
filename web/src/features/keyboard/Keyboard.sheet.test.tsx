import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { action, keysWith, withIssue, openOn, commentBox, keysRead } from '@/test/keyboard.tsx'

// The page's ? sheet, as a user meets it: it lists the terminal's keys the
// page binds, under each pane's own heading, on the keys ui.keys gives them.

test('? opens a sheet listing comment under Issues on c', async () => {
  // Arrange
  const user = userEvent.setup()
  openOn(withIssue, keysWith(true))
  await keysRead('c')

  // Act
  await user.keyboard('?')

  // Assert
  const sheet = screen.getByRole('dialog', { name: 'Keyboard shortcuts' })
  const issues = within(sheet).getByRole('table', { name: 'Issues' })
  expect(within(issues).getByRole('row', { name: 'c comment' })).toBeTruthy()
  expect(within(sheet).queryByText('quit')).toBeNull()
})

test('? lists post under Summary on p', async () => {
  // Arrange
  const user = userEvent.setup()
  const keys = keysWith(true)
  openOn(withIssue, {
    ...keys,
    actions: [...keys.actions, action('post-summary', 'post', 'Summary', 'p')],
  })
  await keysRead('c')

  // Act
  await user.keyboard('?')

  // Assert
  const summary = within(screen.getByRole('dialog')).getByRole('table', { name: 'Summary' })
  expect(within(summary).getByRole('row', { name: 'p post' })).toBeTruthy()
})

test('the sheet shows a key ui.keys moved where it now is', async () => {
  // Arrange
  const user = userEvent.setup()
  openOn(withIssue, keysWith(true, { comment: 'C' }))
  await keysRead('Shift+C')

  // Act
  await user.keyboard('?')

  // Assert
  const issues = within(screen.getByRole('dialog')).getByRole('table', { name: 'Issues' })
  expect(within(issues).getByRole('row', { name: 'C comment' })).toBeTruthy()
})

test('Escape closes the sheet and hands the focus back', async () => {
  // Arrange
  const user = userEvent.setup()
  openOn(withIssue, keysWith(true))
  const box = await commentBox()
  const search = screen.getByRole('searchbox', { name: 'Search' })
  search.focus()
  await user.keyboard('{Escape}')
  screen.getByRole('button', { name: /Fix the token leak/ }).focus()

  // Act: open the sheet from the issue
  await user.keyboard('?')

  // Assert: the sheet is open
  expect(screen.getByRole('dialog')).toBeTruthy()

  // Act: close it
  await user.keyboard('{Escape}')

  // Assert: it is gone, and the focus is back on the issue
  expect(screen.queryByRole('dialog')).toBeNull()
  expect(document.activeElement).toBe(screen.getByRole('button', { name: /Fix the token leak/ }))
  expect(document.activeElement).not.toBe(box)
})

test('the sheet offers to turn single-key shortcuts on while they are off', async () => {
  // Arrange
  const user = userEvent.setup()
  openOn(withIssue, keysWith(false))
  await commentBox()
  await user.keyboard('?')

  // Act
  await user.click(screen.getByRole('button', { name: 'Turn them on in Settings' }))

  // Assert
  expect(screen.queryByRole('dialog')).toBeNull()
  expect(await screen.findByRole('heading', { level: 1, name: 'Settings' })).toBeTruthy()
})

test('an action on a key the page cannot bind is listed as the palette', async () => {
  // Arrange
  const user = userEvent.setup()
  openOn(withIssue, keysWith(true))
  await keysRead('c')

  // Act
  await user.keyboard('?')

  // Assert
  const issues = within(screen.getByRole('dialog')).getByRole('table', { name: 'Issues' })
  expect(within(issues).getByRole('row', { name: 'palette load more' })).toBeTruthy()
})

test('a press outside the sheet closes it', async () => {
  // Arrange
  const user = userEvent.setup()
  openOn(withIssue, keysWith(true))
  await keysRead('c')
  await user.keyboard('?')

  // Act
  await user.click(document.body)

  // Assert
  expect(screen.queryByRole('dialog')).toBeNull()
})
