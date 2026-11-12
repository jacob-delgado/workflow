import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { Config } from '@/api/generated/types.gen.ts'
import { mockConfig } from '@/dev/mockConfig.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { anEdit, inSection } from '@/test/settings.ts'
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
  await inSection('Forge and pull requests')

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
    .queryAllByRole('button', { name: /^Remove the/, hidden: true })
    .map((button) => button.textContent)

  // Assert
  expect(screen.queryByRole('button', { name: 'Remove the webhook URL…', hidden: true })).toBeNull()
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

test("a removal keeps the server's refusal of an edit it kept", async () => {
  // Arrange
  let saves = 0
  fakeApi({
    '/api/config': async (_at: URL, asked: Request) => {
      if (asked.method !== 'PUT') {
        return Response.json(mockConfig, { headers: { ETag: '"read-1"' } })
      }

      saves += 1
      if (saves > 1) {
        return Response.json(await asked.clone().json())
      }

      return Response.json(
        {
          code: 'unprocessable',
          status: 422,
          title: 'Unprocessable content',
          detail: 'the configuration is not valid: no project OSS',
          fields: [{ key: 'jira.project', detail: 'no project OSS' }],
        },
        { status: 422 },
      )
    },
  })
  renderWithClient(<SettingsPanel />)
  const project = await screen.findByRole('textbox', { name: 'Project' })
  await userEvent.clear(project)
  await userEvent.type(project, 'OSS')
  await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))
  await screen.findByRole('textbox', { name: 'Project', description: /no project OSS/ })

  // Act
  await confirmRemoving('the Jira token')

  // Assert
  await screen.findByText('Removed the Jira token.')
  expect(
    screen.getByRole('textbox', { name: 'Project', description: /no project OSS/ }),
  ).toHaveProperty('value', 'OSS')
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

// oneRevision opens Settings over a server that keeps the file at one revision,
// refusing a save over any other as changed since read, as the server does,
// and holding each save's answer until let go. It returns the requests the
// page made, and letGo, which answers the first save still held.
async function oneRevision() {
  let revision = 1
  const held: (() => void)[] = []
  const requests = fakeApi({
    '/api/config': async (_at: URL, asked: Request) => {
      if (asked.method === 'GET') {
        return Response.json(mockConfig, { headers: { ETag: `"rev-${String(revision)}"` } })
      }
      const over = asked.headers.get('If-Match')
      const body: unknown = await asked.clone().json()
      await new Promise<void>((answer) => held.push(answer))
      if (over !== `"rev-${String(revision)}"`) {
        return Response.json(
          { title: 'Conflict', status: 409, code: 'conflict', detail: 'changed since read' },
          { status: 409 },
        )
      }
      revision += 1

      return Response.json(body, { headers: { ETag: `"rev-${String(revision)}"` } })
    },
  })
  renderWithClient(<SettingsPanel />)
  await screen.findByLabelText('Base URL')
  const letGo = async () => {
    await waitFor(() => {
      expect(held.length).toBeGreaterThan(0)
    })
    held.shift()?.()
  }

  return { requests, letGo }
}

test('Save is held while a removal writes the file, so it is never refused as changed', async () => {
  // Arrange
  const { requests, letGo } = await oneRevision()
  await confirmRemoving('the Jira token')
  await anEdit()

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))

  // Assert
  await letGo()
  await screen.findByText('Removed the Jira token.')
  expect(requests.filter((request) => request.method === 'PUT')).toHaveLength(1)
  expect(screen.queryByRole('button', { name: 'Reload' })).toBeNull()
})

test('a removal asked for while a save writes waits for it, and names what it wrote', async () => {
  // Arrange
  const { requests, letGo } = await oneRevision()
  await anEdit()
  await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))

  // Act
  await confirmRemoving('the Jira token')

  // Assert
  await letGo()
  await screen.findByText('Saved.')
  await letGo()
  await screen.findByText('Removed the Jira token.')
  const [, removed] = requests.filter((request) => request.method === 'PUT')
  expect(removed?.headers.get('If-Match')).toBe('"rev-2"')
  expect(((await removed?.json()) as Config).jira.user).toBe(`${mockConfig.jira.user ?? ''}x`)
})
