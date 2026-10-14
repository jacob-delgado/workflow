import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { mockConfig } from '@/dev/mockConfig.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { inSection } from '@/test/settings.ts'
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
  await screen.findByLabelText('Base URL')

  return requests
}

// saveBar is the bar Save and its count are in.
function saveBar(): HTMLElement {
  return screen.getByRole('region', { name: 'Save' })
}

test('with nothing changed the bar says so, and Save is held', async () => {
  // Arrange
  const requests = await opened()

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))

  // Assert
  expect(saveBar().textContent).toContain('No unsaved changes')
  expect(screen.getByRole('button', { name: 'Save changes' }).getAttribute('aria-disabled')).toBe(
    'true',
  )
  expect(requests.some((request) => request.method === 'PUT')).toBe(false)
})

test.each([
  [['User'], '1 unsaved change'],
  [['User', 'Project'], '2 unsaved changes'],
])('the bar counts the settings changed: %j', async (labels, count) => {
  // Arrange
  await opened()

  // Act
  for (const label of labels) {
    await userEvent.type(screen.getByLabelText(label), 'x')
  }

  // Assert
  expect(saveBar().textContent).toContain(count)
})

test('a list changed counts as one setting, however many of its rows changed', async () => {
  // Arrange
  await opened()
  await inSection('Messaging')

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Add a channel' }))
  await userEvent.click(screen.getByRole('button', { name: 'Add a channel' }))

  // Assert
  expect(saveBar().textContent).toContain('1 unsaved change')
})

test('Discard puts every section back as it was read', async () => {
  // Arrange
  await opened()
  await userEvent.type(screen.getByLabelText('User'), 'ana')
  await inSection('Advanced')
  await userEvent.type(screen.getByLabelText('CI interval'), '9')

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Discard' }))

  // Assert
  expect(saveBar().textContent).toContain('No unsaved changes')
  expect(screen.getByLabelText<HTMLInputElement>('User').value).toBe(mockConfig.jira.user)
  expect(screen.getByLabelText<HTMLInputElement>('CI interval').value).toBe(
    mockConfig.timing.ci_interval,
  )
})

test('a saved form says so, with nothing left unsaved', async () => {
  // Arrange
  await opened()
  await userEvent.type(screen.getByLabelText('User'), 'ana')

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))

  // Assert
  await screen.findByText('Saved.')
  expect(saveBar().textContent).not.toContain('unsaved change')
})
