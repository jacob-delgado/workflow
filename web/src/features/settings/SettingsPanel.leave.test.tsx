import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import App from '@/App.tsx'
import { mockConfig } from '@/dev/mockConfig.ts'
import { mockKeys } from '@/dev/mockKeys.ts'
import { useUiStore } from '@/shell/uiStore.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { openPalette } from '@/test/keyboard.tsx'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { anEdit } from '@/test/settings.ts'

// Leaving Settings while the form holds unsaved edits, as a user meets it:
// the page asks before a change of section drops them.

// openSettings draws the page on Settings, once the form has read the file.
async function openSettings(): Promise<void> {
  fakeApi({ '/api/keys': mockKeys, '/api/config': mockConfig })
  useUiStore.setState({ section: 'settings' })
  renderWithClient(<App />)
  await screen.findByLabelText('User')
}

// goTo presses a section's button on the rail.
async function goTo(name: string): Promise<void> {
  const rail = screen.getByRole('navigation', { name: 'Sections' })
  await userEvent.click(within(rail).getByRole('button', { name }))
}

test('a section change with unsaved edits asks first, and Stay keeps them', async () => {
  // Arrange
  await openSettings()
  await anEdit()

  // Act: go to Issues
  await goTo('Issues')

  // Assert: Settings stays, asking
  expect(screen.getByRole('heading', { level: 1, name: 'Settings' })).toBeTruthy()
  expect(screen.getByRole('group', { name: 'Leave without saving?' })).toBeTruthy()

  // Act: stay
  await userEvent.click(screen.getByRole('button', { name: 'Stay' }))

  // Assert: the question goes, handing the focus back, and the edit is still
  // in the form
  expect(screen.queryByRole('group', { name: 'Leave without saving?' })).toBeNull()
  expect(document.activeElement).toBe(
    within(screen.getByRole('navigation', { name: 'Sections' })).getByRole('button', {
      name: 'Issues',
    }),
  )
  expect(screen.getByRole('heading', { level: 1, name: 'Settings' })).toBeTruthy()
  expect(screen.getByLabelText<HTMLInputElement>('User').value).toBe('x')
})

test('a section change from the palette asks with the focus, and Stay hands it to the edit', async () => {
  // Arrange
  const user = userEvent.setup()
  await openSettings()
  await anEdit()
  await openPalette(user)
  await user.keyboard('Issues')

  // Act: go to Issues through the palette
  await user.keyboard('{Enter}')

  // Assert: the question has the focus, not the field the palette handed it
  // back to
  const question = await screen.findByRole('group', { name: 'Leave without saving?' })
  expect(document.activeElement).toBe(question)

  // Act: stay
  await user.click(screen.getByRole('button', { name: 'Stay' }))

  // Assert: the focus goes back to the setting edited, as the palette left it
  expect(document.activeElement).toBe(screen.getByLabelText('User'))
})

test('Leave goes to the section asked for, dropping the edits', async () => {
  // Arrange
  await openSettings()
  await anEdit()
  await goTo('Issues')

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Leave' }))

  // Assert
  expect(await screen.findByRole('heading', { level: 1, name: 'Issues' })).toBeTruthy()
  expect(screen.queryByRole('group', { name: 'Leave without saving?' })).toBeNull()
})

test('a section change with nothing unsaved goes at once', async () => {
  // Arrange
  await openSettings()

  // Act
  await goTo('Issues')

  // Assert
  expect(await screen.findByRole('heading', { level: 1, name: 'Issues' })).toBeTruthy()
  expect(screen.queryByRole('group', { name: 'Leave without saving?' })).toBeNull()
})
