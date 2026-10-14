import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { Config } from '@/api/generated/types.gen.ts'
import { mockConfig } from '@/dev/mockConfig.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { SettingsPanel } from './SettingsPanel.tsx'

// Settings a section at a time: a list of its sections beside the one shown,
// the configuration's first, then the two that save as they change.

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

test('Settings lists its sections, and opens on Jira alone', async () => {
  // Act
  await opened()

  // Assert
  const list = screen.getByRole('tablist', { name: 'Settings sections' })
  expect(
    within(list)
      .getAllByRole('tab')
      .map((tab) => tab.textContent),
  ).toEqual([
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
  expect(screen.getByRole('tab', { name: 'Jira' }).getAttribute('aria-selected')).toBe('true')
  expect(screen.queryByRole('spinbutton', { name: 'Subject limit' })).toBeNull()
})

test('a section chosen from the list is shown in place of the one before', async () => {
  // Arrange
  await opened()
  const user = userEvent.setup()

  // Act
  await user.click(screen.getByRole('tab', { name: 'Commits and branches' }))

  // Assert
  expect(screen.getByRole('spinbutton', { name: 'Subject limit' })).toBeTruthy()
  expect(screen.queryByRole('textbox', { name: 'Base URL' })).toBeNull()
})

test('a save sends what was changed in every section, shown or not', async () => {
  // Arrange
  const requests = await opened()
  const user = userEvent.setup()
  await user.clear(screen.getByLabelText('Base URL'))
  await user.type(screen.getByLabelText('Base URL'), 'https://jira.example.org')
  await user.click(screen.getByRole('tab', { name: 'Commits and branches' }))
  await user.clear(screen.getByLabelText('Subject limit'))
  await user.type(screen.getByLabelText('Subject limit'), '60')

  // Act
  await user.click(screen.getByRole('button', { name: 'Save changes' }))

  // Assert
  await screen.findByText('Saved.')
  const sent = (await requests.find((request) => request.method === 'PUT')?.json()) as Config
  expect(sent.jira.base_url).toBe('https://jira.example.org')
  expect(sent.commit.subject_limit).toBe(60)
})
