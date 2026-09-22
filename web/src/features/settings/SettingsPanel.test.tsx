import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
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

test('loads the configured messaging service into the Service select', async () => {
  // Arrange
  vi.stubEnv('VITE_MOCK', 'true')
  renderWithClient(<SettingsPanel />)

  // Act
  const service = await screen.findByLabelText('Service')

  // Assert
  expect((service as HTMLSelectElement).value).toBe('slack')
  expect(screen.getByRole('option', { name: 'Microsoft Teams' })).toBeTruthy()
  expect(screen.getByRole('option', { name: 'Discord' })).toBeTruthy()
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

test('toggling Markdown comments rides back through a save', async () => {
  // Arrange
  vi.stubEnv('VITE_MOCK', 'true')
  const user = userEvent.setup()
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const view = render(
    <QueryClientProvider client={client}>
      <SettingsPanel />
    </QueryClientProvider>,
  )
  const toggle = await screen.findByRole('checkbox', { name: /markdown/i })
  expect((toggle as HTMLInputElement).checked).toBe(false)
  await user.click(toggle)

  // Act: save, then reopen against the same client
  await user.click(screen.getByRole('button', { name: /save changes/i }))
  await screen.findByText(/saved/i)
  view.unmount()
  render(
    <QueryClientProvider client={client}>
      <SettingsPanel />
    </QueryClientProvider>,
  )

  // Assert
  const reopened = await screen.findByRole('checkbox', { name: /markdown/i })
  expect((reopened as HTMLInputElement).checked).toBe(true)
})

test('a save updates the cache so reopening Settings shows the change', async () => {
  // Arrange
  vi.stubEnv('VITE_MOCK', 'true')
  const user = userEvent.setup()
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const view = render(
    <QueryClientProvider client={client}>
      <SettingsPanel />
    </QueryClientProvider>,
  )
  const project = await screen.findByLabelText('Project')
  await user.clear(project)
  await user.type(project, 'XYZ')

  // Act: save, then reopen against the same client
  await user.click(screen.getByRole('button', { name: /save changes/i }))
  await screen.findByText(/saved/i)
  view.unmount()
  render(
    <QueryClientProvider client={client}>
      <SettingsPanel />
    </QueryClientProvider>,
  )

  // Assert
  const reopened = await screen.findByLabelText('Project')
  expect((reopened as HTMLInputElement).value).toBe('XYZ')
})
