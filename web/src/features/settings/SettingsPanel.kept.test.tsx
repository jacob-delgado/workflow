import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
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

// credential is the password field labeled label in the group named group: a
// password field has no role of its own.
async function credential(group: string, label: string): Promise<HTMLElement> {
  return within(await screen.findByRole('group', { name: group })).getByLabelText(label)
}

// description is the text an element's aria-describedby names: what assistive
// technology reads after its name.
function description(element: HTMLElement): string {
  const ids = (element.getAttribute('aria-describedby') ?? '').split(' ').filter(Boolean)

  return ids.map((id) => document.getElementById(id)?.textContent ?? '').join(' ')
}

const jira = mockConfig.jira

test.each([
  ['the file', { token: '••••••••' }, 'Stored in the file'],
  ['the file over the keychain', { token: '••••••••', keychain: true }, 'Stored in the file'],
  [
    'the keychain over the other sources',
    { token: '', keychain: true, token_env: 'JIRA_TOKEN', token_command: 'pass show jira' },
    'Kept in your keychain, for jira.example.com',
  ],
  ['a variable', { token: '', token_env: 'JIRA_TOKEN' }, 'Read from $JIRA_TOKEN'],
  ['a command', { token: '', token_command: 'pass show jira' }, 'Printed by a command you set'],
  ['nowhere', { token: '' }, 'Not set'],
])('the Jira token kept in %s says so', async (_where, kept, said) => {
  // Arrange
  readsAs({ ...mockConfig, jira: { ...jira, ...kept } })

  // Act
  const token = await credential('Jira', 'Token')

  // Assert
  expect(description(token)).toContain(said)
})

test.each([
  {
    section: 'Forge and pull requests',
    group: 'Forge',
    label: 'Token',
    kept: { forge: { ...mockConfig.forge, token: '' } },
    said: "Not in the file; read from the environment or the forge's CLI, when either has one",
  },
  {
    section: 'Messaging',
    group: 'Messaging',
    label: 'Webhook URL',
    kept: { messaging: { ...mockConfig.messaging, webhook_url: '••••••••' } },
    said: 'Stored in the file',
  },
  {
    section: 'Messaging',
    group: 'Messaging',
    label: 'Client secret',
    kept: { messaging: { ...mockConfig.messaging, client_secret: '' } },
    said: 'Not in the file; on macOS, your keychain may keep it',
  },
])(
  'in $section, the $label says where it is kept',
  async ({ section, group, label, kept, said }) => {
    // Arrange
    readsAs({ ...mockConfig, ...kept })
    await inSection(section)

    // Act
    const field = await credential(group, label)

    // Assert
    expect(description(field)).toContain(said)
  },
)

test("the forge token in the file is said to give way to the environment's and the CLI's", async () => {
  // Arrange
  readsAs(mockConfig)
  await inSection('Forge and pull requests')

  // Act
  const token = await credential('Forge', 'Token')

  // Assert
  expect(description(token)).toContain(
    "One kept in the file is read only when neither the environment nor the forge's CLI has one.",
  )
})

test('the Jira token is said to be kept in the keychain once that is turned on', async () => {
  // Arrange
  readsAs({ ...mockConfig, jira: { ...jira, token: '' } })
  const token = await credential('Jira', 'Token')

  // Act
  await userEvent.click(
    screen.getByRole('checkbox', { name: /read the token from your keychain/i }),
  )

  // Assert
  expect(description(token)).toContain('Kept in your keychain, for jira.example.com')
})

test('a token typed over the stored one says it replaces it when saved', async () => {
  // Arrange
  readsAs(mockConfig)
  const token = await credential('Jira', 'Token')

  // Act
  await userEvent.type(token, 'x')

  // Assert
  expect(description(token)).toContain('Replaces the stored token when you save')
})
