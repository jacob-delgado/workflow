import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { errorMessage, SettingsPanel } from './SettingsPanel.tsx'

test('errorMessage surfaces a thrown Error or an error body, else a fallback', () => {
  // Act & Assert
  expect(errorMessage(new Error('boom'))).toBe('boom')
  expect(errorMessage({ code: 'unprocessable', message: 'the config is not valid' })).toBe(
    'the config is not valid',
  )
  expect(errorMessage(42)).toMatch(/could not be saved/i)
})

test('shows a loading state until the configuration arrives', () => {
  // Arrange
  vi.stubEnv('VITE_MOCK', 'true')

  // Act
  renderWithClient(<SettingsPanel />)

  // Assert
  expect(screen.getByText(/loading the configuration/i)).toBeTruthy()
})

test('loads the configuration into the form', async () => {
  // Arrange
  vi.stubEnv('VITE_MOCK', 'true')
  renderWithClient(<SettingsPanel />)

  // Act
  const baseUrl = await screen.findByLabelText('Base URL')

  // Assert
  expect((baseUrl as HTMLInputElement).value).toBe('https://jira.acme.internal')
})

test('confirms when the configuration is saved', async () => {
  // Arrange
  vi.stubEnv('VITE_MOCK', 'true')
  const user = userEvent.setup()
  renderWithClient(<SettingsPanel />)
  await screen.findByLabelText('Base URL')

  // Act
  await user.click(screen.getByRole('button', { name: /save changes/i }))

  // Assert
  expect(await screen.findByText(/saved/i)).toBeTruthy()
})
