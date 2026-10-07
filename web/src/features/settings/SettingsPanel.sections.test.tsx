import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { Config } from '@/api/generated/types.gen.ts'
import { mockConfig } from '@/dev/mockConfig.ts'
import { useKeysStore } from '@/features/keyboard/keysApi.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { SettingsPanel } from './SettingsPanel.tsx'

// opened opens Settings over config, answering a save with what it was sent,
// and returns the requests the page made.
async function opened(config: Config = mockConfig): Promise<Request[]> {
  const requests = fakeApi({
    '/api/config': async (_at: URL, asked: Request) => {
      const answer: unknown = asked.method === 'PUT' ? await asked.clone().json() : config

      return Response.json(answer, { headers: { ETag: '"read-1"' } })
    },
  })
  renderWithClient(<SettingsPanel />)
  await screen.findByLabelText('Base URL')

  return requests
}

// saved saves the form and answers the configuration it sent.
async function saved(requests: Request[]): Promise<Config> {
  await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))
  await screen.findByText('Saved.')
  const put = requests.find((request) => request.method === 'PUT')
  if (put === undefined) {
    throw new Error('no save was sent')
  }

  return (await put.json()) as Config
}

// row is the entry of a list named name, as "View 1".
function row(name: string) {
  return within(screen.getByRole('group', { name }))
}

test('a view added in Settings is saved with its JQL', async () => {
  // Arrange
  const requests = await opened()
  await userEvent.click(screen.getByRole('button', { name: 'Add a view' }))
  await userEvent.type(row('View 1').getByRole('textbox', { name: 'Name' }), 'Review')
  await userEvent.type(row('View 1').getByRole('textbox', { name: 'JQL' }), 'status = Review')

  // Act
  const sent = await saved(requests)

  // Assert
  expect(sent.jira.views).toEqual([{ name: 'Review', jql: 'status = Review' }])
})

test('a view removed in Settings is left out of the save', async () => {
  // Arrange
  const requests = await opened({
    ...mockConfig,
    jira: {
      ...mockConfig.jira,
      views: [
        { name: 'Mine', jql: 'assignee = currentUser()' },
        { name: 'Team', jql: 'project = PROJ' },
      ],
    },
  })
  await userEvent.click(row('View 1').getByRole('button', { name: 'Remove view 1' }))

  // Act
  const sent = await saved(requests)

  // Assert
  expect(sent.jira.views).toEqual([{ name: 'Team', jql: 'project = PROJ' }])
})

test('a channel added in Settings is saved among the channels', async () => {
  // Arrange
  const requests = await opened()
  await userEvent.click(screen.getByRole('button', { name: 'Add a channel' }))
  await userEvent.type(row('Channel 3').getByRole('textbox', { name: 'Channel' }), '#ops')

  // Act
  const sent = await saved(requests)

  // Assert
  expect(sent.messaging.channels).toEqual(['#dev-workflow', '#releases', '#ops'])
})

test('a branch prefix added in Settings is saved as a type and its prefix', async () => {
  // Arrange
  const requests = await opened()
  await userEvent.click(screen.getByRole('button', { name: 'Add a prefix' }))
  await userEvent.type(row('Prefix 1').getByRole('textbox', { name: 'Issue type' }), 'Bug')
  await userEvent.type(row('Prefix 1').getByRole('textbox', { name: 'Prefix' }), 'bugfix')

  // Act
  const sent = await saved(requests)

  // Assert
  expect(sent.branch.prefixes).toEqual({ Bug: 'bugfix' })
})

test('a Jira header added in Settings is saved with its value', async () => {
  // Arrange
  const requests = await opened()
  await userEvent.click(screen.getByRole('button', { name: 'Add a header' }))
  await userEvent.type(row('Header 1').getByRole('textbox', { name: 'Name' }), 'X-Proxy-Auth')
  await userEvent.type(row('Header 1').getByLabelText('Value'), 'proxy-secret')

  // Act
  const sent = await saved(requests)

  // Assert
  expect(sent.jira.headers).toEqual({ 'X-Proxy-Auth': 'proxy-secret' })
})

test("a stored header's masked value goes back as it was read, so it is kept", async () => {
  // Arrange
  const requests = await opened({
    ...mockConfig,
    jira: { ...mockConfig.jira, headers: { 'CF-Access-Client-Secret': '****abcd' } },
  })

  // Act
  const sent = await saved(requests)

  // Assert
  expect(sent.jira.headers).toEqual({ 'CF-Access-Client-Secret': '****abcd' })
})

test('the timing is saved as typed', async () => {
  // Arrange
  const requests = await opened()
  const timeout = screen.getByRole('textbox', { name: 'Request timeout' })
  await userEvent.clear(timeout)
  await userEvent.type(timeout, '45s')

  // Act
  const sent = await saved(requests)

  // Assert
  expect(sent.timing.request_timeout).toBe('45s')
})

test("the terminal's drawing is saved as chosen", async () => {
  // Arrange
  const requests = await opened()
  await userEvent.click(screen.getByRole('checkbox', { name: 'Draw in plain ASCII' }))
  await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Color' }), 'Never')

  // Act
  const sent = await saved(requests)

  // Assert
  expect(sent.ui).toMatchObject({ ascii: true, color: 'never' })
})

test('a key typed for an action is saved in ui.keys, and an empty one is left out', async () => {
  // Arrange
  useKeysStore.setState({
    actions: [
      {
        action: 'comment',
        help: 'comment',
        group: 'Issues',
        shown: 'c',
        keys: ['c'],
        default: 'c',
      },
      { action: 'assign', help: 'assign', group: 'Issues', shown: 'a', keys: ['a'], default: 'a' },
    ],
  })
  const requests = await opened()
  await userEvent.click(screen.getByText('Rebind keys'))
  await userEvent.type(screen.getByRole('textbox', { name: 'Key for comment' }), 'C')

  // Act
  const sent = await saved(requests)

  // Assert
  expect(sent.ui.keys).toEqual({ comment: 'C' })
})

test('each key names the default it keeps when left empty', async () => {
  // Arrange
  useKeysStore.setState({
    actions: [
      {
        action: 'comment',
        help: 'comment',
        group: 'Issues',
        shown: 'C',
        keys: ['C'],
        default: 'c',
      },
    ],
  })

  // Act
  await opened()
  await userEvent.click(screen.getByText('Rebind keys'))

  // Assert
  expect(
    screen.getByRole('textbox', { name: 'Key for comment', description: 'Empty keeps c.' }),
  ).toBeTruthy()
})

test('the key fields are left out of the page until Rebind keys is opened', async () => {
  // Arrange
  useKeysStore.setState({
    actions: [
      {
        action: 'comment',
        help: 'comment',
        group: 'Issues',
        shown: 'c',
        keys: ['c'],
        default: 'c',
      },
    ],
  })

  // Act
  await opened()

  // Assert
  expect(screen.queryByRole('textbox', { name: 'Key for comment', hidden: true })).toBeNull()
})
