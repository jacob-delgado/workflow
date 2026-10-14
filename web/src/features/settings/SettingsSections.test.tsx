import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { Config } from '@/api/generated/types.gen.ts'
import { mockConfig } from '@/dev/mockConfig.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { SettingsPanel } from './SettingsPanel.tsx'

// opened opens Settings over the mockup's configuration, answering a save
// with what it was sent, and returns the requests the page made.
async function opened(): Promise<Request[]> {
  const requests = fakeApi({
    '/api/config': async (_at: URL, asked: Request) => {
      const answer: unknown = asked.method === 'PUT' ? await asked.clone().json() : mockConfig

      return Response.json(answer, { headers: { ETag: '"read-1"' } })
    },
  })
  renderWithClient(<SettingsPanel />)
  await screen.findByRole('tablist', { name: 'Settings sections' })

  return requests
}

// shown reports that the field labeled label is in the panel shown.
function shown(label: string): boolean {
  return screen.getByLabelText(label).closest<HTMLElement>('[role="tabpanel"]')?.hidden === false
}

test('Settings lists its sections, those that save on their own last', async () => {
  // Arrange
  await opened()

  // Act
  const tabs = screen.getAllByRole('tab').map((tab) => tab.textContent)

  // Assert
  expect(tabs).toEqual([
    'Jira',
    'Messaging',
    'Forge and pull requests',
    'Commits and branches',
    'Taskwarrior',
    'Terminal and keys',
    'Advanced',
    'People and groups',
    'Local data',
  ])
})

test('Settings opens on Jira, the other sections hidden', async () => {
  // Arrange
  await opened()

  // Act & Assert
  expect([shown('Base URL'), shown('Host'), shown('Request timeout')]).toEqual([true, false, false])
})

test('choosing a section shows its settings in place of the last', async () => {
  // Arrange
  await opened()

  // Act
  await userEvent.click(screen.getByRole('tab', { name: 'Forge and pull requests' }))

  // Assert
  expect([shown('Base URL'), shown('Host'), shown('Title source')]).toEqual([false, true, true])
})

test('edits in two sections are saved together', async () => {
  // Arrange
  const requests = await opened()
  await userEvent.type(screen.getByLabelText('Project'), 'X')
  await userEvent.click(screen.getByRole('tab', { name: 'Advanced' }))
  await userEvent.clear(screen.getByLabelText('Request timeout'))
  await userEvent.type(screen.getByLabelText('Request timeout'), '30s')

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))
  await screen.findByText('Saved.')

  // Assert
  const put = requests.find((request) => request.method === 'PUT')
  const sent = (await put?.json()) as Config
  expect([sent.jira.project, sent.timing.request_timeout]).toEqual([
    `${mockConfig.jira.project ?? ''}X`,
    '30s',
  ])
})

test("a section's tab counts its settings changed and not saved", async () => {
  // Arrange
  await opened()

  // Act
  await userEvent.type(screen.getByLabelText('User'), 'x')
  await userEvent.type(screen.getByLabelText('Project'), 'x')

  // Assert
  expect(screen.getByRole('tab', { name: 'Jira' }).textContent).toBe('Jira2')
  expect(screen.getByRole('tab', { name: 'Messaging' }).textContent).toBe('Messaging')
})

test("a section's tab says how many of its settings to fix", async () => {
  // Arrange
  await opened()
  await userEvent.click(screen.getByRole('tab', { name: 'Advanced' }))
  await userEvent.type(screen.getByLabelText('CI interval'), 'x')

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))

  // Assert
  await screen.findByText('A duration such as 20s or 3m.')
  expect(screen.getByRole('tab', { name: 'Advanced' }).textContent).toBe('Advanced1 to fix')
})
