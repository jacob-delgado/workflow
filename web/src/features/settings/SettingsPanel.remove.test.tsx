import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { Config } from '@/api/generated/types.gen.ts'
import { mockConfig } from '@/dev/mockConfig.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { SettingsPanel } from './SettingsPanel.tsx'

// withHeader is the mock configuration with a Jira header stored.
const withHeader: Config = {
  ...mockConfig,
  jira: { ...mockConfig.jira, headers: { 'CF-Access-Client-Secret': '****abcd' } },
}

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

// sent is the body of the one save the page made.
async function sent(requests: Request[]): Promise<Config> {
  const [put, ...more] = requests.filter((request) => request.method === 'PUT')
  if (put === undefined || more.length > 0) {
    throw new Error(`${String(more.length + (put ? 1 : 0))} saves were sent, not one`)
  }

  return (await put.json()) as Config
}

// confirmRemoving opens the removal of what and confirms it.
async function confirmRemoving(what: string): Promise<void> {
  await userEvent.click(screen.getByRole('button', { name: `Remove ${what}…` }))
  const question = screen.getByRole('group', { name: `Remove ${what} from the file?` })
  await userEvent.click(within(question).getByRole('button', { name: 'Remove' }))
}

test('removing the Jira token asks first, then saves it as null', async () => {
  // Arrange
  const requests = await opened()

  // Act
  await confirmRemoving('the Jira token')

  // Assert
  await screen.findByText('Removed the Jira token.')
  const body = await sent(requests)
  expect(body.jira.token).toBeNull()
  expect(body.forge.token).toBe(mockConfig.forge.token)
})

test('the question says a removal cannot be undone', async () => {
  // Arrange
  await opened()

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Remove the forge token…' }))

  // Assert
  const question = screen.getByRole('group', { name: 'Remove the forge token from the file?' })
  expect(within(question).getByText(/cannot be undone/)).toBeTruthy()
})

test('cancelling a removal sends nothing', async () => {
  // Arrange
  const requests = await opened()
  await userEvent.click(screen.getByRole('button', { name: 'Remove the Jira token…' }))

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))

  // Assert
  expect(requests.filter((request) => request.method === 'PUT')).toHaveLength(0)
  expect(screen.getByRole('button', { name: 'Remove the Jira token…' })).toBeTruthy()
})

test('a credential with nothing stored offers no removal', async () => {
  // Arrange
  await opened()

  // Act
  const removals = screen
    .queryAllByRole('button', { name: /^Remove the/ })
    .map((button) => button.textContent)

  // Assert
  expect(screen.queryByRole('button', { name: 'Remove the webhook URL…' })).toBeNull()
  expect(removals).toHaveLength(2)
})

test('removing a stored header asks first, then saves without it', async () => {
  // Arrange
  const requests = await opened(withHeader)

  // Act
  await confirmRemoving('the header CF-Access-Client-Secret')

  // Assert
  await screen.findByText('Removed the header CF-Access-Client-Secret.')
  expect((await sent(requests)).jira.headers).toBeNull()
})

test("a removal saves the file as read, and keeps the form's other edits", async () => {
  // Arrange
  const requests = await opened()
  const project = screen.getByRole('textbox', { name: 'Project' })
  await userEvent.clear(project)
  await userEvent.type(project, 'OSS')

  // Act
  await confirmRemoving('the Jira token')

  // Assert
  await screen.findByText('Removed the Jira token.')
  expect((await sent(requests)).jira.project).toBe(mockConfig.jira.project)
  await waitFor(() => {
    expect(screen.getByRole('textbox', { name: 'Project' })).toHaveProperty('value', 'OSS')
  })
})
