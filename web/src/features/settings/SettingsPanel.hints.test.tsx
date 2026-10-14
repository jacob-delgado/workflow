import { screen, within } from '@testing-library/react'
import type { Config } from '@/api/generated/types.gen.ts'
import { mockConfig } from '@/dev/mockConfig.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { inSection } from '@/test/settings.ts'
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

test('the Jira token says where one typed is kept', async () => {
  // Arrange
  readsAs({ ...mockConfig, jira: { ...mockConfig.jira, token: '' } })

  // Act
  const token = await jiraToken()

  // Assert
  expect(description(token)).toContain(
    'One typed here is kept in your keychain for this address on macOS, or in the file elsewhere.',
  )
})

test('the Jira section says which of its settings wait for a restart or a directory switch', async () => {
  // Arrange
  readsAs(mockConfig)

  // Act
  const jira = await screen.findByRole('group', { name: 'Jira' })

  // Assert
  expect(description(jira)).toBe(
    "These wait for a restart or a directory switch: the address, token and where it is read from, user, headers, and whether this repository's issues are listed and which view they lead. The rest apply at once.",
  )
})

test('the Announcement hint names its placeholders, Slack only, and the empty default', async () => {
  // Arrange
  readsAs(mockConfig)
  await inSection('Messaging')

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
  await inSection('Messaging')

  // Act & Assert
  expect(
    await screen.findByRole('textbox', { name: 'Channel', description: /user token/ }),
  ).toBeTruthy()
})
