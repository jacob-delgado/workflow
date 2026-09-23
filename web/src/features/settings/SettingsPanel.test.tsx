import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import { useHealthStore } from '@/api/health.ts'
import { mockConfig } from '@/dev/mockConfig.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { gitLabWords, makeHealth } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { SettingsPanel } from './SettingsPanel.tsx'

test('shows a loading state until the configuration arrives', () => {
  // Arrange
  vi.stubEnv('VITE_MOCK', 'true')

  // Act
  renderWithClient(<SettingsPanel />)

  // Assert
  expect(screen.getByText(/loading the configuration/i)).toBeTruthy()
})

test('loads the configuration into the form', async () => {
  // Arrange
  vi.stubEnv('VITE_MOCK', 'true')
  renderWithClient(<SettingsPanel />)

  // Act
  const baseUrl = await screen.findByLabelText('Base URL')

  // Assert
  expect((baseUrl as HTMLInputElement).value).toBe('https://jira.acme.internal')
})

test('loads the configured messaging service into the Service select', async () => {
  // Arrange
  vi.stubEnv('VITE_MOCK', 'true')
  renderWithClient(<SettingsPanel />)

  // Act
  const service = await screen.findByLabelText('Service')

  // Assert
  expect((service as HTMLSelectElement).value).toBe('slack')
  expect(screen.getByRole('option', { name: 'Microsoft Teams' })).toBeTruthy()
  expect(screen.getByRole('option', { name: 'Discord' })).toBeTruthy()
})

test('surfaces the commit-convention fields', async () => {
  // Arrange
  vi.stubEnv('VITE_MOCK', 'true')
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
  vi.stubEnv('VITE_MOCK', 'true')
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
  vi.stubEnv('VITE_MOCK', 'true')
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
  vi.stubEnv('VITE_MOCK', 'true')
  const user = userEvent.setup()
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const view = render(
    <QueryClientProvider client={client}>
      <SettingsPanel />
    </QueryClientProvider>,
  )
  const types = await screen.findByLabelText('Types')
  await user.type(types, 'hotfix, chore')

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
  // The list is parsed to ["hotfix","chore"], so it reads back without the space.
  const reopened = await screen.findByLabelText('Types')
  expect((reopened as HTMLInputElement).value).toBe('hotfix,chore')
})

test('offers turning the store off and rides it back through a save', async () => {
  // Arrange
  vi.stubEnv('VITE_MOCK', 'true')
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

test('confirms when the configuration is saved', async () => {
  // Arrange
  vi.stubEnv('VITE_MOCK', 'true')
  const user = userEvent.setup()
  renderWithClient(<SettingsPanel />)
  await screen.findByLabelText('Base URL')

  // Act
  await user.click(screen.getByRole('button', { name: /save changes/i }))

  // Assert
  expect(await screen.findByText(/saved/i)).toBeTruthy()
})

test('toggling Markdown comments rides back through a save', async () => {
  // Arrange
  vi.stubEnv('VITE_MOCK', 'true')
  const user = userEvent.setup()
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const view = render(
    <QueryClientProvider client={client}>
      <SettingsPanel />
    </QueryClientProvider>,
  )
  const toggle = await screen.findByRole('checkbox', { name: /markdown/i })
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
  const reopened = await screen.findByRole('checkbox', { name: /markdown/i })
  expect((reopened as HTMLInputElement).checked).toBe(true)
})

test('a save updates the cache so reopening Settings shows the change', async () => {
  // Arrange
  vi.stubEnv('VITE_MOCK', 'true')
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

  // Assert: the button reads "Saving…" and is off, and only one save went out
  const saving = await screen.findByRole('button', { name: 'Saving…' })
  expect(saving.hasAttribute('disabled')).toBe(true)
  expect(saves).toHaveLength(1)

  releaseSave()
  await screen.findByText('Saved.')
})
