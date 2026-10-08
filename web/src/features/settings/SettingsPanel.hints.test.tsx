import { screen, within } from '@testing-library/react'
import type { Config } from '@/api/generated/types.gen.ts'
import { mockConfig } from '@/dev/mockConfig.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { SettingsPanel } from './SettingsPanel.tsx'

// readsAs opens Settings over config, as the configuration a read finds.
function readsAs(config: Config): void {
  fakeApi({ '/api/config': () => Response.json(config, { headers: { ETag: '"read-1"' } }) })
  renderWithClient(<SettingsPanel />)
}

// jiraToken is the Jira section's Token field. It is a password field, which
// has no role of its own, so it is found by its label within its section.
async function jiraToken(): Promise<HTMLElement> {
  return within(await screen.findByRole('group', { name: 'Jira' })).getByLabelText('Token')
}

// description is the text an element's aria-describedby names: what assistive
// technology reads after its name.
function description(element: HTMLElement): string {
  const ids = (element.getAttribute('aria-describedby') ?? '').split(' ').filter(Boolean)

  return ids.map((id) => document.getElementById(id)?.textContent ?? '').join(' ')
}

test('the Jira token is described by the token_command it is taken from', async () => {
  // Arrange
  readsAs({
    ...mockConfig,
    jira: { ...mockConfig.jira, token: '', token_command: 'pass show jira' },
  })

  // Act
  const token = await jiraToken()

  // Assert
  expect(description(token)).toMatch(/Taken from token_command: pass show jira/)
})

test('the Jira token is described by the variable it is taken from', async () => {
  // Arrange
  readsAs({ ...mockConfig, jira: { ...mockConfig.jira, token: '', token_env: 'JIRA_TOKEN' } })

  // Act
  const token = await jiraToken()

  // Assert
  expect(description(token)).toMatch(/Taken from token_env: JIRA_TOKEN/)
})

test('the Jira token is described by the keychain it is taken from, over the other sources', async () => {
  // Arrange
  readsAs({
    ...mockConfig,
    jira: {
      ...mockConfig.jira,
      token: '',
      keychain: true,
      token_command: 'pass show jira',
      token_env: 'JIRA_TOKEN',
    },
  })

  // Act
  const token = await jiraToken()

  // Assert
  expect(description(token)).toMatch(/Taken from your keychain, for this address\./)
})

test('the Jira token says where one typed is kept', async () => {
  // Arrange
  readsAs({ ...mockConfig, jira: { ...mockConfig.jira, token: '' } })

  // Act
  const token = await jiraToken()

  // Assert
  expect(description(token)).toBe(
    'A personal access token, kept in your keychain for this address on macOS or in the file elsewhere.',
  )
})

test('the Announcement hint names its placeholders, Slack only, and the empty default', async () => {
  // Arrange
  readsAs(mockConfig)

  // Act
  const announcement = await screen.findByRole('textbox', {
    name: 'Announcement',
    description: /\{author\}/,
  })

  // Assert
  expect(description(announcement)).toMatch(/Slack only/)
  expect(description(announcement)).toMatch(/empty keeps the built-in message/i)
})

test('the Channel hint says it is for a Slack user token', async () => {
  // Arrange
  readsAs(mockConfig)

  // Act & Assert
  expect(
    await screen.findByRole('textbox', { name: 'Channel', description: /user token/ }),
  ).toBeTruthy()
})
