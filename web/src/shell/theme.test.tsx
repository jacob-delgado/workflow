import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import App from '@/App.tsx'
import { listenerCount } from '@/test/matchMedia.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import indexHtml from '../../index.html?raw'
import { themeStorageKey } from './themeKey.ts'
import { readStoredChoice } from './themeStore.ts'

test('the theme toggle cycles system, light, dark and remembers the choice', async () => {
  // Arrange
  const user = userEvent.setup()
  renderWithClient(<App />)

  // Act: it opens following the system
  // Assert
  expect(screen.getByRole('button', { name: /theme: system/i })).toBeTruthy()

  // Act: click through the cycle
  // Assert: each step advances the choice and persists it
  await user.click(screen.getByRole('button', { name: /theme: system/i }))
  expect(screen.getByRole('button', { name: /theme: light/i })).toBeTruthy()
  expect(localStorage.getItem(themeStorageKey)).toBe('light')

  await user.click(screen.getByRole('button', { name: /theme: light/i }))
  expect(screen.getByRole('button', { name: /theme: dark/i })).toBeTruthy()
  expect(localStorage.getItem(themeStorageKey)).toBe('dark')

  await user.click(screen.getByRole('button', { name: /theme: dark/i }))
  expect(screen.getByRole('button', { name: /theme: system/i })).toBeTruthy()
  expect(localStorage.getItem(themeStorageKey)).toBe('system')
})

test('the theme toggle shows the choice it holds on hover, and follows it', async () => {
  // Arrange
  const user = userEvent.setup()
  renderWithClient(<App />)
  const toggle = screen.getByRole('button', { name: /^Theme: System/ })
  const before = toggle.getAttribute('title')

  // Act
  await user.click(toggle)

  // Assert
  expect(before).toBe('Theme: System')
  expect(toggle.getAttribute('title')).toBe('Theme: Light')
})

test('following the system registers one OS listener and drops it when set to light', async () => {
  // Arrange
  const user = userEvent.setup()
  renderWithClient(<App />)

  // Act: it opens following the system
  // Assert: exactly one OS-change listener is registered to follow it
  expect(listenerCount()).toBe(1)

  // Act: switch off "system" to a forced choice
  await user.click(screen.getByRole('button', { name: /theme: system/i }))

  // Assert: the OS listener is removed — no leak, and no stale follow while forced
  expect(listenerCount()).toBe(0)
})

test('a saved choice is restored, and anything unexpected falls back to system', () => {
  // Arrange
  localStorage.setItem(themeStorageKey, 'dark')

  // Act & Assert: a valid saved choice comes back
  expect(readStoredChoice()).toBe('dark')

  // Act & Assert: an unrecognized value defaults rather than sticking
  localStorage.setItem(themeStorageKey, 'chartreuse')
  expect(readStoredChoice()).toBe('system')
})

test('the page reads the saved choice before paint under the key the store saves it at', () => {
  // Act: the key index.html's pre-paint script reads
  const read = /localStorage\.getItem\('([^']*)'\)/.exec(indexHtml)?.[1]

  // Assert
  expect(read).toBe(themeStorageKey)
})
