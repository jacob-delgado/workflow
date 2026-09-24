import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { mockConfig } from '@/dev/mockConfig.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { appQueryClient, renderWithClient } from '@/test/renderWithClient.tsx'
import { SettingsPanel } from './SettingsPanel.tsx'

// revisionedApi answers a read with the configuration at the revision "read-1",
// and each save with the configuration at the next one: "saved-1", "saved-2"…
// It returns every request it was sent.
function revisionedApi(): Request[] {
  let saves = 0

  return fakeApi({
    '/api/config': (_at: URL, asked: Request) => {
      if (asked.method === 'GET') {
        return Response.json(mockConfig, { headers: { ETag: '"read-1"' } })
      }
      saves += 1

      return Response.json(mockConfig, { headers: { ETag: `"saved-${String(saves)}"` } })
    },
  })
}

// namedRevisions is the revision each save named, in order.
function namedRevisions(requests: Request[]): (string | null)[] {
  return requests
    .filter((request) => request.method === 'PUT')
    .map((request) => request.headers.get('If-Match'))
}

test('a save is made over the revision its read returned', async () => {
  // Arrange
  const requests = revisionedApi()
  const user = userEvent.setup()
  renderWithClient(<SettingsPanel />)
  await screen.findByLabelText('Base URL')

  // Act
  await user.click(screen.getByRole('button', { name: 'Save changes' }))

  // Assert
  await screen.findByText('Saved.')
  expect(namedRevisions(requests)).toEqual(['"read-1"'])
})

test('the next save is made over the revision the last one wrote', async () => {
  // Arrange
  const requests = revisionedApi()
  const user = userEvent.setup()
  renderWithClient(<SettingsPanel />)
  await screen.findByLabelText('Base URL')
  await user.click(screen.getByRole('button', { name: 'Save changes' }))
  await screen.findByText('Saved.')

  // Act
  await user.click(screen.getByRole('button', { name: 'Save changes' }))

  // Assert
  await screen.findByText('Saved.')
  expect(namedRevisions(requests)).toEqual(['"read-1"', '"saved-1"'])
})

// The base URL an edit made outside the browser leaves in the file.
const editedURL = 'https://edited.example.com'

// configAt answers a read or a save with the configuration at revision, with
// Jira at baseURL.
function configAt(revision: string, baseURL = mockConfig.jira.base_url): Response {
  const config = { ...mockConfig, jira: { ...mockConfig.jira, base_url: baseURL } }

  return Response.json(config, { headers: { ETag: `"${revision}"` } })
}

// refusal answers with a problem details object.
function refusal(status: number, code: string, detail: string): Response {
  return Response.json({ type: 'about:blank', title: 'Refused', status, code, detail }, { status })
}

// changedOnDisk is the server's answer to a save over a revision the file has
// moved on from.
function changedOnDisk(): Response {
  return refusal(
    409,
    'conflict',
    'the configuration changed since Settings read it; reload Settings and apply your change again',
  )
}

// configAnswers answers the configuration's requests with each answer in turn,
// and returns every request it was sent.
function configAnswers(...answers: Response[]): Request[] {
  return fakeApi({ '/api/config': () => answers.shift() })
}

// saveRefusedAsChanged opens Settings over answers that refuse the first save
// as made over a changed file, saves, and waits for the offer to reload.
async function saveRefusedAsChanged(...afterwards: Response[]) {
  const requests = configAnswers(configAt('read-1'), changedOnDisk(), ...afterwards)
  const user = userEvent.setup()
  renderWithClient(<SettingsPanel />)
  await user.click(await screen.findByRole('button', { name: 'Save changes' }))
  const reload = await screen.findByRole('button', { name: 'Reload' })

  return { requests, user, reload }
}

// baseURL is the Base URL field.
function baseURL(): Promise<HTMLInputElement> {
  return screen.findByLabelText('Base URL')
}

test('a save refused because the file changed says so and offers Reload', async () => {
  // Arrange
  configAnswers(configAt('read-1'), changedOnDisk())
  const user = userEvent.setup()
  renderWithClient(<SettingsPanel />)
  await screen.findByLabelText('Base URL')

  // Act
  await user.click(screen.getByRole('button', { name: 'Save changes' }))

  // Assert
  const alert = await screen.findByRole('alert')
  expect(alert.textContent).toMatch(/changed after Settings read it/)
  expect(alert.textContent).toMatch(/Reload reads it again/)
  expect(screen.getByRole('button', { name: 'Reload' })).toBeTruthy()
  expect(screen.getByRole('status').textContent).toBe('')
})

test('Reload reads the file again into the form', async () => {
  // Arrange
  const { user, reload } = await saveRefusedAsChanged(configAt('read-2', editedURL))

  // Act
  await user.click(reload)

  // Assert
  await waitFor(async () => {
    expect((await baseURL()).value).toBe(editedURL)
  })
  expect(screen.queryByRole('button', { name: 'Reload' })).toBeNull()
  expect(screen.queryByRole('alert')).toBeNull()
  expect(screen.getByRole('status').textContent).toBe('')
})

test("Reload hands focus to the form's first field", async () => {
  // Arrange
  const { user, reload } = await saveRefusedAsChanged(configAt('read-2', editedURL))

  // Act
  await user.click(reload)

  // Assert
  await waitFor(async () => {
    expect(document.activeElement).toBe(await baseURL())
  })
})

test('a save after Reload is made over the revision Reload read', async () => {
  // Arrange
  const { requests, user, reload } = await saveRefusedAsChanged(
    configAt('read-2', editedURL),
    configAt('saved-1', editedURL),
  )
  await user.click(reload)
  await waitFor(async () => {
    expect((await baseURL()).value).toBe(editedURL)
  })

  // Act
  await user.click(screen.getByRole('button', { name: 'Save changes' }))

  // Assert
  await screen.findByText('Saved.')
  expect(namedRevisions(requests)).toEqual(['"read-1"', '"read-2"'])
})

test('a Reload that cannot read the file says why and keeps offering Reload', async () => {
  // Arrange
  const invalid =
    'the configuration file on disk is not valid, so the configuration in effect stands'
  const { user, reload } = await saveRefusedAsChanged(refusal(422, 'unprocessable', invalid))

  // Act
  await user.click(reload)

  // Assert
  expect(await screen.findByText(invalid)).toBeTruthy()
  expect(screen.getByRole('button', { name: 'Reload' })).toBeTruthy()
})

test.each([
  ['an invalid configuration', 422, 'unprocessable', 'jira.base_url is not a URL'],
  [
    'a save that named no revision',
    428,
    'precondition_required',
    'the save did not say which revision of the configuration it was made over; reload the page, then save again',
  ],
])('a save refused for %s says why and offers no Reload', async (_, status, code, detail) => {
  // Arrange
  configAnswers(configAt('read-1'), refusal(status, code, detail))
  const user = userEvent.setup()
  renderWithClient(<SettingsPanel />)
  await screen.findByLabelText('Base URL')

  // Act
  await user.click(screen.getByRole('button', { name: 'Save changes' }))

  // Assert
  expect(await screen.findByText(detail)).toBeTruthy()
  expect(screen.queryByRole('button', { name: 'Reload' })).toBeNull()
})

test('reopening Settings reads the file again before showing the form', async () => {
  // Arrange
  // The file is edited on disk while Settings is closed.
  configAnswers(configAt('read-1'), configAt('read-2', editedURL))
  const client = appQueryClient()
  const view = render(
    <QueryClientProvider client={client}>
      <SettingsPanel />
    </QueryClientProvider>,
  )
  await screen.findByLabelText('Base URL')
  view.unmount()

  // Act
  render(
    <QueryClientProvider client={client}>
      <SettingsPanel />
    </QueryClientProvider>,
  )

  // Assert
  expect((await baseURL()).value).toBe(editedURL)
})

// notValidOnDisk is the server's refusal of a read of a file on disk that is
// not valid.
const notValidOnDisk =
  'the configuration file on disk is not valid, so the configuration in effect stands; workflow doctor says what is wrong with it'

test('opening Settings over a file that is not valid says why and offers Retry', async () => {
  // Arrange
  configAnswers(refusal(422, 'unprocessable', notValidOnDisk))

  // Act
  renderWithClient(<SettingsPanel />)

  // Assert
  expect(await screen.findByText(notValidOnDisk)).toBeTruthy()
  expect(screen.getByRole('button', { name: 'Retry' })).toBeTruthy()
})

test('Settings says why a read was refused without first trying it again', async () => {
  // Arrange
  // A client with the library's own retries, which the app's client keeps.
  fakeApi({ '/api/config': () => refusal(422, 'unprocessable', notValidOnDisk) })
  const client = new QueryClient()

  // Act
  render(
    <QueryClientProvider client={client}>
      <SettingsPanel />
    </QueryClientProvider>,
  )

  // Assert
  expect(await screen.findByText(notValidOnDisk)).toBeTruthy()
})
