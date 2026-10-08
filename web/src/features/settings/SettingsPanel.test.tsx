import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import { useHealthStore } from '@/api/health.ts'
import { mockConfig } from '@/dev/mockConfig.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { gitLabWords, makeHealth } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { AppShell } from '@/shell/AppShell.tsx'
import { useUiStore } from '@/shell/uiStore.ts'
import { SettingsPanel } from './SettingsPanel.tsx'

// servesSettings answers Settings' reads as a server keeps them: the mockup's
// configuration to start and, after a save, what was saved, at the revision
// it was saved at; a store of one file; and one owner linked on Slack.
function servesSettings(): Request[] {
  let stored: unknown = mockConfig
  let revision = 1

  return fakeApi({
    '/api/config': async (_at: URL, asked: Request) => {
      if (asked.method === 'PUT') {
        stored = await asked.clone().json()
        revision += 1
      }

      return Response.json(stored, { headers: { ETag: `"read-${String(revision)}"` } })
    },
    '/api/local-data': {
      dir: '/home/ana/.local/state/workflow',
      files: [{ name: 'workflow.db', kind: 'cache', bytes: 4096, size: '4.0 KiB', holds: [] }],
      consequences: { cache: 'The cache is made again.', all: 'Everything is asked again.' },
    },
    '/api/people': {
      owners: [
        {
          owner: 'carla',
          kind: 'user',
          state: 'linked',
          slack: { id: 'U0CARLA', label: 'Carla Diaz' },
        },
      ],
    },
    '/api/repo-groups': { repository: 'acme/workflow', groups: [] },
  })
}

test('says it is reading until the configuration arrives', () => {
  // Arrange
  servesSettings()

  // Act
  renderWithClient(<SettingsPanel />)

  // Assert
  const waits = screen.getAllByRole('status').map((wait) => wait.textContent)
  expect(waits).toContain('Reading the configuration…')
})

test('loads the configuration into the form', async () => {
  // Arrange
  servesSettings()
  renderWithClient(<SettingsPanel />)

  // Act
  const baseUrl = await screen.findByLabelText('Base URL')

  // Assert
  expect((baseUrl as HTMLInputElement).value).toBe('https://jira.acme.internal')
})

test('loads the configured messaging service into the Service select', async () => {
  // Arrange
  servesSettings()
  renderWithClient(<SettingsPanel />)

  // Act
  const service = await screen.findByLabelText('Service')

  // Assert
  // The mock names slack, the default, which the select shows as its default choice.
  expect(
    within(service).getByRole('option', { name: 'Slack (default)', selected: true }),
  ).toBeTruthy()
  expect(screen.getByRole('option', { name: 'Microsoft Teams' })).toBeTruthy()
  expect(screen.getByRole('option', { name: 'Discord' })).toBeTruthy()
})

test('surfaces the commit-convention fields', async () => {
  // Arrange
  servesSettings()
  renderWithClient(<SettingsPanel />)

  // Act
  const types = await screen.findByLabelText('Types')

  // Assert
  expect(types).toBeTruthy()
  expect(screen.getByLabelText('Subject limit')).toBeTruthy()
  expect(screen.getByLabelText('Issue trailer')).toBeTruthy()
})

test('surfaces the branch and pull-request fields', async () => {
  // Arrange
  servesSettings()
  renderWithClient(<SettingsPanel />)

  // Act
  const titleSource = await screen.findByLabelText('Title source')

  // Assert
  expect(screen.getByLabelText('Slug limit')).toBeTruthy()
  expect(titleSource).toBeTruthy()
  expect(screen.getByRole('option', { name: /the issue it names/i })).toBeTruthy()
})

test("the hints name the forge's own noun: a merge request on GitLab", async () => {
  // Arrange
  servesSettings()
  useHealthStore.setState({ health: makeHealth(gitLabWords) })

  // Act
  renderWithClient(<SettingsPanel />)

  // Assert
  await screen.findByRole('textbox', { name: 'Review status' })
  const reviewHint =
    'The status an issue moves to once its merge request is open, e.g. "In Review". Empty makes no offer.'
  expect(
    screen.getByRole('textbox', { name: 'Review status', description: reviewHint }),
  ).toBeTruthy()
  expect(
    screen.getByRole('combobox', {
      name: 'Title source',
      description: "Where a merge request's title comes from.",
    }),
  ).toBeTruthy()
})

test('editing the commit types saves them as a trimmed list', async () => {
  // Arrange
  servesSettings()
  const user = userEvent.setup()
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const view = render(
    <QueryClientProvider client={client}>
      <SettingsPanel />
    </QueryClientProvider>,
  )
  const types = await screen.findByLabelText('Types')
  await user.type(types, 'hotfix, chore')

  // Act
  // Save, then reopen against the same client.
  await user.click(screen.getByRole('button', { name: /save changes/i }))
  await screen.findByText(/saved/i)
  view.unmount()
  render(
    <QueryClientProvider client={client}>
      <SettingsPanel />
    </QueryClientProvider>,
  )

  // Assert
  // The list is parsed to ["hotfix","chore"], so it reads back without the space.
  const reopened = await screen.findByLabelText('Types')
  expect((reopened as HTMLInputElement).value).toBe('hotfix,chore')
})

test('an emptied subject limit saves as 0, which keeps the default', async () => {
  // Arrange
  servesSettings()
  const user = userEvent.setup()
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const view = render(
    <QueryClientProvider client={client}>
      <SettingsPanel />
    </QueryClientProvider>,
  )
  const limit = await screen.findByLabelText('Subject limit')
  await user.type(limit, '50')
  await user.clear(limit)

  // Act
  // Save, then reopen against the same client.
  await user.click(screen.getByRole('button', { name: /save changes/i }))
  await screen.findByText(/saved/i)
  view.unmount()
  render(
    <QueryClientProvider client={client}>
      <SettingsPanel />
    </QueryClientProvider>,
  )

  // Assert
  // A count typed empty is the number 0, not an empty string.
  const reopened = await screen.findByLabelText('Subject limit')
  expect((reopened as HTMLInputElement).value).toBe('0')
})

test('offers turning the store off and rides it back through a save', async () => {
  // Arrange
  servesSettings()
  const user = userEvent.setup()
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const view = render(
    <QueryClientProvider client={client}>
      <SettingsPanel />
    </QueryClientProvider>,
  )
  const toggle = await screen.findByRole('checkbox', { name: /nothing on disk/i })
  expect((toggle as HTMLInputElement).checked).toBe(false)
  await user.click(toggle)

  // Act: save, then reopen against the same client
  await user.click(screen.getByRole('button', { name: /save changes/i }))
  await screen.findByText(/saved/i)
  view.unmount()
  render(
    <QueryClientProvider client={client}>
      <SettingsPanel />
    </QueryClientProvider>,
  )

  // Assert
  const reopened = await screen.findByRole('checkbox', { name: /nothing on disk/i })
  expect((reopened as HTMLInputElement).checked).toBe(true)
})

test('the Taskwarrior fieldset says its changes apply when workflow restarts', async () => {
  // Arrange
  servesSettings()

  // Act
  renderWithClient(<SettingsPanel />)

  // Assert
  expect(
    await screen.findByRole('group', {
      name: 'Taskwarrior',
      description: 'A change here applies when workflow restarts.',
    }),
  ).toBeTruthy()
})

test("the Taskwarrior fieldset's switch rides back through a save, and its program as the file holds it", async () => {
  // Arrange
  servesSettings()
  const user = userEvent.setup()
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const view = render(
    <QueryClientProvider client={client}>
      <SettingsPanel />
    </QueryClientProvider>,
  )
  const program = await screen.findByRole('textbox', {
    name: 'Task program',
    description:
      'Set in the file: workflow runs it as you, so Settings keeps it as it is. Empty tries every task in an absolute PATH directory and keeps the first that is Taskwarrior 3.5.0 or newer.',
  })
  await user.type(program, '/opt/homebrew/bin/task')
  await user.click(screen.getByRole('checkbox', { name: /turn off the taskwarrior integration/i }))

  // Act
  // Save, then reopen against the same client.
  await user.click(screen.getByRole('button', { name: /save changes/i }))
  await screen.findByText(/saved/i)
  view.unmount()
  render(
    <QueryClientProvider client={client}>
      <SettingsPanel />
    </QueryClientProvider>,
  )

  // Assert
  const reopened = await screen.findByRole('textbox', { name: 'Task program' })
  expect((reopened as HTMLInputElement).value).toBe('')
  const off = screen.getByRole('checkbox', { name: /turn off the taskwarrior integration/i })
  expect((off as HTMLInputElement).checked).toBe(true)
})

test('confirms when the configuration is saved', async () => {
  // Arrange
  servesSettings()
  const user = userEvent.setup()
  renderWithClient(<SettingsPanel />)
  await screen.findByLabelText('Base URL')

  // Act
  await user.click(screen.getByRole('button', { name: /save changes/i }))

  // Assert
  expect(await screen.findByText(/saved/i)).toBeTruthy()
})

test('a clicked Save keeps the focus it was clicked with once it has saved', async () => {
  // Arrange
  servesSettings()
  const user = userEvent.setup()
  renderWithClient(<SettingsPanel />)
  await screen.findByLabelText('Base URL')
  const save = screen.getByRole('button', { name: 'Save changes' })

  // Act
  await user.click(save)

  // Assert
  await screen.findByText('Saved.')
  expect(document.activeElement).toBe(save)
})

test('a Save clicked by a pointer that gives it no focus hands focus to what it said', async () => {
  // Arrange
  servesSettings()
  renderWithClient(<SettingsPanel />)
  await screen.findByLabelText('Base URL')

  // Act
  fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))

  // Assert
  const said = await screen.findByText('Saved.')
  await waitFor(() => {
    expect(document.activeElement).toBe(said)
  })
})

test('the configuration form is named by the Settings heading', async () => {
  // Arrange
  servesSettings()
  useUiStore.setState({ section: 'settings' })

  // Act
  renderWithClient(<AppShell />)

  // Assert
  expect(await screen.findByRole('form', { name: 'Settings' })).toBeTruthy()
})

test('toggling Markdown comments rides back through a save', async () => {
  // Arrange
  servesSettings()
  const user = userEvent.setup()
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const view = render(
    <QueryClientProvider client={client}>
      <SettingsPanel />
    </QueryClientProvider>,
  )
  const toggle = await screen.findByRole('checkbox', { name: /markdown/i })
  // On by default, so the save carries it off.
  expect((toggle as HTMLInputElement).checked).toBe(true)
  await user.click(toggle)

  // Act: save, then reopen against the same client
  await user.click(screen.getByRole('button', { name: /save changes/i }))
  await screen.findByText(/saved/i)
  view.unmount()
  render(
    <QueryClientProvider client={client}>
      <SettingsPanel />
    </QueryClientProvider>,
  )

  // Assert
  const reopened = await screen.findByRole('checkbox', { name: /markdown/i })
  expect((reopened as HTMLInputElement).checked).toBe(false)
})

test('a save updates the cache so reopening Settings shows the change', async () => {
  // Arrange
  servesSettings()
  const user = userEvent.setup()
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const view = render(
    <QueryClientProvider client={client}>
      <SettingsPanel />
    </QueryClientProvider>,
  )
  const project = await screen.findByLabelText('Project')
  await user.clear(project)
  await user.type(project, 'XYZ')

  // Act
  // Save, then reopen against the same client.
  await user.click(screen.getByRole('button', { name: /save changes/i }))
  await screen.findByText(/saved/i)
  view.unmount()
  render(
    <QueryClientProvider client={client}>
      <SettingsPanel />
    </QueryClientProvider>,
  )

  // Assert
  const reopened = await screen.findByLabelText('Project')
  expect((reopened as HTMLInputElement).value).toBe('XYZ')
})

test('says why a save was refused', async () => {
  // Arrange
  // The configuration reads as stored, then the save is refused with a reason.
  const answers = [
    Response.json(mockConfig),
    Response.json(
      {
        code: 'unprocessable',
        status: 422,
        title: 'Unprocessable content',
        detail: 'jira.base_url is not a URL',
      },
      { status: 422 },
    ),
  ]
  fakeApi({ '/api/config': () => answers.shift() })
  const user = userEvent.setup()
  renderWithClient(<SettingsPanel />)
  await screen.findByLabelText('Base URL')

  // Act
  await user.click(screen.getByRole('button', { name: /save changes/i }))

  // Assert
  expect(await screen.findByText('jira.base_url is not a URL')).toBeTruthy()
})

test('locks the save while it is in flight', async () => {
  // Arrange
  // The configuration reads as stored, and the save is held open so the
  // in-flight state is observable; a live button here would let a double click
  // save twice.
  const saves: Request[] = []
  let releaseSave = () => {}
  vi.stubGlobal(
    'fetch',
    vi.fn((request: Request) => {
      if (request.method === 'GET') {
        return Promise.resolve(Response.json(mockConfig))
      }
      saves.push(request)

      return new Promise<Response>((resolve) => {
        releaseSave = () => {
          resolve(Response.json(mockConfig))
        }
      })
    }),
  )
  const user = userEvent.setup()
  renderWithClient(<SettingsPanel />)
  await screen.findByLabelText('Base URL')

  // Act
  await user.click(screen.getByRole('button', { name: 'Save changes' }))

  // Assert
  // The button reads "Saving…" and is held, and only one save went out.
  const saving = await screen.findByRole('button', { name: 'Saving…' })
  expect(saving.getAttribute('aria-disabled')).toBe('true')
  expect(saves).toHaveLength(1)

  releaseSave()
  await screen.findByText('Saved.')
})

test('a configuration that cannot be read says why, as an alert', async () => {
  // Arrange
  fakeApi({ '/api/config': () => Response.json({}, { status: 500 }) })

  // Act
  renderWithClient(<SettingsPanel />)

  // Assert
  const reason = await screen.findByText('The configuration could not be read.')
  expect(reason.getAttribute('role')).toBe('alert')
})

test('offers to try again when the configuration cannot be loaded', async () => {
  // Arrange
  // The first read fails; the one Try again asks for answers. The reads below
  // it fail too, each with a Try again of its own beside its reason.
  const answers = [Response.json({}, { status: 500 }), Response.json(mockConfig)]
  fakeApi({ '/api/config': () => answers.shift() })
  const user = userEvent.setup()
  renderWithClient(<SettingsPanel />)
  const reason = await screen.findByText('The configuration could not be read.')

  // Act
  await user.click(
    within(reason.parentElement ?? document.body).getByRole('button', { name: 'Try again' }),
  )

  // Assert
  // The form that took the Try again's place has focus, on its first field.
  expect(document.activeElement).toBe(await screen.findByLabelText('Base URL'))
})

test('the form the first read loads leaves focus where it was', async () => {
  // Arrange
  servesSettings()

  // Act
  renderWithClient(<SettingsPanel />)

  // Assert
  await screen.findByLabelText('Base URL')
  expect(document.activeElement).toBe(document.body)
})

test('sets up Slack with a user token, not a bot token', async () => {
  // Arrange
  servesSettings()
  renderWithClient(<SettingsPanel />)

  // Act
  const clientID = await screen.findByLabelText('Client ID')

  // Assert
  expect((clientID as HTMLInputElement).value).toBe('1234.5678')
  expect(screen.getByLabelText('Client secret').getAttribute('type')).toBe('password')
  expect(screen.getByLabelText('Refresh token').getAttribute('type')).toBe('password')
  expect(screen.queryByLabelText('Bot token')).toBeNull()
})

test("the switch that lists this repository's forge issues rides back through a save", async () => {
  // Arrange
  servesSettings()
  const user = userEvent.setup()
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const view = render(
    <QueryClientProvider client={client}>
      <SettingsPanel />
    </QueryClientProvider>,
  )
  const forge = await screen.findByRole('checkbox', { name: /this repository's .* issues/i })
  await user.click(forge)

  // Act
  // Save, then reopen against the same client.
  await user.click(screen.getByRole('button', { name: /save changes/i }))
  await screen.findByText(/saved/i)
  view.unmount()
  render(
    <QueryClientProvider client={client}>
      <SettingsPanel />
    </QueryClientProvider>,
  )

  // Assert
  const reopened = await screen.findByRole('checkbox', { name: /this repository's .* issues/i })
  expect((reopened as HTMLInputElement).checked).toBe(true)
})

test('shows the local data under its own heading below the configuration', async () => {
  // Arrange
  servesSettings()

  // Act
  renderWithClient(<SettingsPanel />)

  // Assert
  const heading = await screen.findByRole('heading', { level: 2, name: 'Local data' })
  const save = await screen.findByRole('button', { name: 'Save changes' })
  expect(save.compareDocumentPosition(heading) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  expect(await screen.findByRole('table', { name: 'Local data files' })).toBeTruthy()
})

test('shows people and groups between the configuration and the local data', async () => {
  // Arrange
  servesSettings()

  // Act
  renderWithClient(<SettingsPanel />)

  // Assert
  const people = await screen.findByRole('heading', { level: 2, name: 'People and groups' })
  const local = await screen.findByRole('heading', { level: 2, name: 'Local data' })
  const save = await screen.findByRole('button', { name: 'Save changes' })
  expect(save.compareDocumentPosition(people) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  expect(people.compareDocumentPosition(local) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  expect(await screen.findByRole('table', { name: 'Code owners on Slack' })).toBeTruthy()
})
