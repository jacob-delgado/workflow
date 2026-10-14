import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { mockConfig } from '@/dev/mockConfig.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { showSection } from '@/test/settings.ts'
import { SettingsPanel } from './SettingsPanel.tsx'

// The save bar under Settings' sections: how many changes are not saved yet,
// Discard and Save, kept in view whichever section is shown.

// opened opens Settings over the mockup's configuration, answering a save with
// what it was sent, and returns the requests the page made.
async function opened(): Promise<Request[]> {
  const requests = fakeApi({
    '/api/config': async (_at: URL, asked: Request) => {
      const answer: unknown = asked.method === 'PUT' ? await asked.clone().json() : mockConfig

      return Response.json(answer, { headers: { ETag: '"read-1"' } })
    },
  })
  renderWithClient(<SettingsPanel />)
  await screen.findByLabelText('Base URL')

  return requests
}

// bar is the save bar, by what it says.
function bar(): HTMLElement {
  return screen.getByRole('region', { name: 'Save' })
}

test('with nothing changed the bar says so, and Save is held', async () => {
  // Act
  await opened()

  // Assert
  expect(bar().textContent).toContain('No unsaved changes')
  expect(screen.getByRole('button', { name: 'Save changes' }).getAttribute('aria-disabled')).toBe(
    'true',
  )
})

test('each change is counted in the bar, and beside its section', async () => {
  // Arrange
  await opened()
  const user = userEvent.setup()
  await user.type(screen.getByRole('textbox', { name: 'Project' }), 'X')
  await showSection('Commits and branches')

  // Act
  await user.type(screen.getByRole('spinbutton', { name: 'Subject limit' }), '50')

  // Assert
  expect(bar().textContent).toContain('2 unsaved changes')
  expect(screen.getByRole('tab', { name: 'Jira, 1 unsaved' })).toBeTruthy()
  expect(screen.getByRole('tab', { name: 'Commits and branches, 1 unsaved' })).toBeTruthy()
})

test('Discard puts every section back as it was read', async () => {
  // Arrange
  const requests = await opened()
  const user = userEvent.setup()
  const project = screen.getByRole('textbox', { name: 'Project' })
  await user.type(project, 'X')

  // Act
  await user.click(screen.getByRole('button', { name: 'Discard' }))

  // Assert
  expect(project).toHaveProperty('value', mockConfig.jira.project)
  expect(bar().textContent).toContain('No unsaved changes')
  expect(requests.some((request) => request.method === 'PUT')).toBe(false)
})

test('a save clears the count, and says it saved', async () => {
  // Arrange
  await opened()
  const user = userEvent.setup()
  await user.type(screen.getByRole('textbox', { name: 'Project' }), 'X')

  // Act
  await user.click(screen.getByRole('button', { name: 'Save changes' }))

  // Assert
  expect(await screen.findByText('Saved.')).toBeTruthy()
  expect(screen.getByRole('tab', { name: 'Jira' })).toBeTruthy()
})
