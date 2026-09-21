import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import App from '@/App.tsx'
import { listenerCount } from '@/test/matchMedia.ts'
import { readStoredChoice } from './themeStore.ts'

test('the theme toggle cycles system, light, dark and remembers the choice', async () => {
  // Arrange
  const user = userEvent.setup()
  render(<App />)

  // Act: it opens following the system
  // Assert
  expect(screen.getByRole('button', { name: /theme: system/i })).toBeTruthy()

  // Act: click through the cycle
  // Assert: each step advances the choice and persists it
  await user.click(screen.getByRole('button', { name: /theme: system/i }))
  expect(screen.getByRole('button', { name: /theme: light/i })).toBeTruthy()
  expect(localStorage.getItem('workflow-theme')).toBe('light')

  await user.click(screen.getByRole('button', { name: /theme: light/i }))
  expect(screen.getByRole('button', { name: /theme: dark/i })).toBeTruthy()
  expect(localStorage.getItem('workflow-theme')).toBe('dark')

  await user.click(screen.getByRole('button', { name: /theme: dark/i }))
  expect(screen.getByRole('button', { name: /theme: system/i })).toBeTruthy()
  expect(localStorage.getItem('workflow-theme')).toBe('system')
})

test('following the system registers one OS listener and drops it when set to light', async () => {
  // Arrange
  const user = userEvent.setup()
  render(<App />)

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
  localStorage.setItem('workflow-theme', 'dark')

  // Act & Assert: a valid saved choice comes back
  expect(readStoredChoice()).toBe('dark')

  // Act & Assert: an unrecognized value defaults rather than sticking
  localStorage.setItem('workflow-theme', 'chartreuse')
  expect(readStoredChoice()).toBe('system')
})
