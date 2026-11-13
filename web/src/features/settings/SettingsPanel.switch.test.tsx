import { act, screen, waitFor } from '@testing-library/react'
import type { QueryClient } from '@tanstack/react-query'
import App from '@/App.tsx'
import {
  getConfigQueryKey,
  getRepositoriesQueryKey,
} from '@/api/generated/@tanstack/react-query.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import type { Config } from '@/api/generated/types.gen.ts'
import { mockConfig } from '@/dev/mockConfig.ts'
import { mockKeys } from '@/dev/mockKeys.ts'
import { mockRepositories } from '@/dev/mockRepositories.ts'
import { useUiStore } from '@/shell/uiStore.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { appQueryClient, renderWithClient } from '@/test/renderWithClient.tsx'
import { anEdit } from '@/test/settings.ts'

// Settings open while the server switches directory from another tab or the
// terminal, as a user meets it: the form's unsaved edits stay when the
// configuration read back has the same revision and the same files apply where
// the server works now, and otherwise the page says they were for the
// directory left.

// Read is a configuration as the server answers it, with its revision.
interface Read {
  config: Config
  revision: string
}

const left: Read = { config: mockConfig, revision: '"left"' }

// elsewhere is another directory's file, whose Jira user is bea.
const elsewhere: Read = {
  config: { ...mockConfig, jira: { ...mockConfig.jira, user: 'bea' } },
  revision: '"elsewhere"',
}

// noFile is the read where no file applies.
const noFile = 'no file'

// unread is where the server works when the read of it is refused.
const unread = 'unread'

// Files are the configuration files that apply where the server works, as
// the read of where it works lists them, or unread.
type Files = string[] | typeof unread

// homeOnly is the home file alone, which applies wherever the server works.
const homeOnly = ['~/.workflow.json']

// apiFiles are the files that apply in api, where Settings opens.
const apiFiles = ['~/src/api/.workflow.json', '~/.workflow.json']

// webFiles are the files that apply in web, a copy of api's among them.
const webFiles = ['~/src/web/.workflow.json', '~/.workflow.json']

// Where is a directory as the server reads it: its configuration, and the
// files that apply there.
interface Where {
  read: Read | typeof noFile
  files: Files
}

// refused is a problem the server answers a read with.
function refused(detail: string): Response {
  const problem = { type: 'about:blank', title: 'Refused', status: 404, code: 'not_found' }

  return Response.json({ ...problem, detail }, { status: 404 })
}

// answer is a read of the configuration as the server answers it.
function answer(read: Read | typeof noFile): Response {
  if (read === noFile) {
    return refused('no .workflow.json applies')
  }

  return Response.json(read.config, { headers: { ETag: read.revision } })
}

// placed is a read of where the server works as the server answers it.
function placed(files: Files): Response {
  if (files === unread) {
    return refused('the directory is not there')
  }

  const repositories = mockRepositories()

  return Response.json({ ...repositories, here: { ...repositories.here, config: files } })
}

// later answers a read some milliseconds later, as a read over the network
// does.
async function later(answered: Response, milliseconds: number): Promise<Response> {
  await new Promise((resolve) => setTimeout(resolve, milliseconds))

  return answered
}

// Opened is Settings drawn: the page's query client, and the requests the
// page made.
interface Opened {
  client: QueryClient
  requests: Request[]
}

// openSettings draws Settings working in api, where the files before apply,
// answering the first read of the configuration with left's file and of where
// the server works with before, and every read after with after's, a moment
// later, where the server works the later of the two; and returns the page's
// client and requests once the form is drawn.
async function openSettings(after: Where, before: Files = apiFiles): Promise<Opened> {
  let switched = false
  const requests = fakeApi({
    '/api/keys': mockKeys,
    '/api/config': () => (switched ? later(answer(after.read), 20) : answer(left)),
    '/api/repositories': () => (switched ? later(placed(after.files), 40) : placed(before)),
  })
  const client = appQueryClient()
  useSnapshotStore.setState({ status: 'live', snapshot: makeSnapshot() })
  useUiStore.setState({ section: 'settings' })
  renderWithClient(<App />, client)
  await screen.findByLabelText('User')
  await waitFor(() => {
    expect(client.isFetching()).toBe(0)
  })
  switched = true

  return { client, requests }
}

// switchedElsewhere has the stream say the server works in web now, and waits
// until the configuration and where the server works, read again there, have
// each answered, with what is there or a refusal, and nothing more is being
// read.
async function switchedElsewhere({ client, requests }: Opened): Promise<void> {
  const before = configReads(requests)
  const keys = [getConfigQueryKey(), getRepositoriesQueryKey()]
  const answered = () =>
    keys.map((queryKey) => {
      const state = client.getQueryState(queryKey)

      return Math.max(state?.dataUpdatedAt ?? 0, state?.errorUpdatedAt ?? 0)
    })
  const answeredBefore = answered()
  act(() => {
    useSnapshotStore.setState({ snapshot: makeSnapshot({ here: '/home/ana/src/web' }) })
  })
  await waitFor(() => {
    expect(configReads(requests)).toBeGreaterThan(before)
    answered().forEach((time, index) => {
      expect(time).toBeGreaterThan(answeredBefore[index] ?? 0)
    })
    expect(client.isFetching()).toBe(0)
  })
}

// configReads counts the reads of the configuration among requests.
function configReads(requests: Request[]): number {
  return requests.filter(
    (request) => request.method === 'GET' && new URL(request.url).pathname === '/api/config',
  ).length
}

test('a switch elsewhere keeps the unsaved edits when the file read back is the same', async () => {
  // Arrange
  const opened = await openSettings({ read: left, files: homeOnly }, homeOnly)
  await anEdit()

  // Act
  await switchedElsewhere(opened)

  // Assert
  await waitFor(() => {
    expect(screen.getByLabelText<HTMLInputElement>('User').value).toBe('x')
  })
  expect(screen.getByRole('region', { name: 'Save' }).textContent).toContain('1 unsaved change')
  expect(screen.queryByText(/were for/)).toBeNull()
})

test('a switch elsewhere to a copy of the file says the unsaved edits were for the directory left', async () => {
  // Arrange
  const opened = await openSettings({ read: left, files: webFiles })
  await anEdit()

  // Act
  await switchedElsewhere(opened)

  // Assert
  expect(
    await screen.findByText(
      'Your unsaved changes were for /home/ana/src/api, which the server has left, so they were dropped.',
    ),
  ).toBeTruthy()
  expect(screen.getByLabelText<HTMLInputElement>('User').value).toBe('')
})

test('a switch elsewhere to where the files that apply are unread says the unsaved edits were for the directory left', async () => {
  // Arrange
  const opened = await openSettings({ read: left, files: unread })
  await anEdit()

  // Act
  await switchedElsewhere(opened)

  // Assert
  expect(
    await screen.findByText(
      'Your unsaved changes were for /home/ana/src/api, which the server has left, so they were dropped.',
    ),
  ).toBeTruthy()
})

test('a switch elsewhere where the files that apply are never read says the unsaved edits were for the directory left', async () => {
  // Arrange
  const opened = await openSettings({ read: left, files: unread }, unread)
  await anEdit()

  // Act
  await switchedElsewhere(opened)

  // Assert
  expect(
    await screen.findByText(
      'Your unsaved changes were for /home/ana/src/api, which the server has left, so they were dropped.',
    ),
  ).toBeTruthy()
})

test('a switch elsewhere to another file says the unsaved edits were for the directory left', async () => {
  // Arrange
  const opened = await openSettings({ read: elsewhere, files: webFiles })
  await anEdit()

  // Act
  await switchedElsewhere(opened)

  // Assert
  expect(
    await screen.findByText(
      'Your unsaved changes were for /home/ana/src/api, which the server has left, so they were dropped.',
    ),
  ).toBeTruthy()
  expect(screen.getByLabelText<HTMLInputElement>('User').value).toBe('bea')
})

test('a switch elsewhere to another file shows that file in a form with nothing unsaved', async () => {
  // Arrange
  const opened = await openSettings({ read: elsewhere, files: webFiles })

  // Act
  await switchedElsewhere(opened)

  // Assert
  await waitFor(() => {
    expect(screen.getByLabelText<HTMLInputElement>('User').value).toBe('bea')
  })
  expect(screen.queryByText(/were for/)).toBeNull()
})

test('a switch elsewhere to where no file applies says the unsaved edits were for the directory left', async () => {
  // Arrange
  const opened = await openSettings({ read: noFile, files: [] })
  await anEdit()

  // Act
  await switchedElsewhere(opened)

  // Assert
  expect(
    await screen.findByText(
      'Your unsaved changes were for /home/ana/src/api, which the server has left, so they were dropped.',
    ),
  ).toBeTruthy()
  expect(screen.queryByLabelText('User')).toBeNull()
})
