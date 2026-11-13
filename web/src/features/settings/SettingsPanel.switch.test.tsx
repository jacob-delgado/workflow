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
// the server works with before, and every read after with after's: the
// configuration a moment later, and where the server works a while after
// that, long enough for a check to see it still out; and returns the page's
// client and requests once the form is drawn.
async function openSettings(after: Where, before: Files = apiFiles): Promise<Opened> {
  let switched = false
  const requests = fakeApi({
    '/api/keys': mockKeys,
    '/api/config': () => (switched ? later(answer(after.read), 20) : answer(left)),
    '/api/repositories': () => (switched ? later(placed(after.files), 250) : placed(before)),
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

// openSettingsAcross draws Settings working in api, as openSettings does, and
// answers every read after with left's file and with the files filesIn says
// apply where the server works when the read goes: where the server works a
// moment later, and the configuration a while after that.
async function openSettingsAcross(filesIn: Record<string, Files>): Promise<Opened> {
  let switched = false
  const filesHere = () => filesIn[useSnapshotStore.getState().snapshot?.here ?? ''] ?? unread
  const requests = fakeApi({
    '/api/keys': mockKeys,
    '/api/config': () => (switched ? later(answer(left), 250) : answer(left)),
    '/api/repositories': () => (switched ? later(placed(filesHere()), 5) : placed(apiFiles)),
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

// Midway is a point between the two reads after a switch: the one read again
// that has answered, and the one still out.
interface Midway {
  answered: readonly unknown[]
  out: readonly unknown[]
}

// placeAnswered is where the server works read again, with the configuration
// still out.
const placeAnswered: Midway = { answered: getRepositoriesQueryKey(), out: getConfigQueryKey() }

// configAnswered is the configuration read again, with where the server works
// still out.
const configAnswered: Midway = { answered: getConfigQueryKey(), out: getRepositoriesQueryKey() }

// switchedMidway has the stream say the server works in here now, and waits
// until one read again there has answered while the other is still out: where
// the server works, unless midway says otherwise.
async function switchedMidway(
  { client }: Opened,
  here: string,
  midway: Midway = placeAnswered,
): Promise<void> {
  const answeredAt = () => client.getQueryState(midway.answered)?.dataUpdatedAt ?? 0
  const answeredBefore = answeredAt()
  act(() => {
    useSnapshotStore.setState({ snapshot: makeSnapshot({ here }) })
  })
  await waitFor(() => {
    expect(answeredAt()).toBeGreaterThan(answeredBefore)
  })
  expect(client.getQueryState(midway.out)?.fetchStatus).toBe('fetching')
}

// saveHeld says whether Save is held, as a user's assistive technology hears
// it.
function saveHeld(): boolean {
  return (
    screen.getByRole('button', { name: 'Save changes' }).getAttribute('aria-disabled') === 'true'
  )
}

// switchedElsewhere has the stream say the server works in here now, web
// unless said, and waits until the configuration and where the server works,
// read again there, have each answered, with what is there or a refusal, and
// nothing more is being read.
async function switchedElsewhere(
  { client, requests }: Opened,
  here = '/home/ana/src/web',
): Promise<void> {
  const before = configReads(requests)
  const keys = [getConfigQueryKey(), getRepositoriesQueryKey()]
  const answered = () =>
    keys.map((queryKey) => {
      const state = client.getQueryState(queryKey)

      return Math.max(state?.dataUpdatedAt ?? 0, state?.errorUpdatedAt ?? 0)
    })
  const answeredBefore = answered()
  act(() => {
    useSnapshotStore.setState({ snapshot: makeSnapshot({ here }) })
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

test('a switch elsewhere and back before the first is read keeps the unsaved edits', async () => {
  // Arrange
  const opened = await openSettingsAcross({
    '/home/ana/src/api': apiFiles,
    '/home/ana/src/web': webFiles,
  })
  await anEdit()
  await switchedMidway(opened, '/home/ana/src/web')

  // Act
  await switchedElsewhere(opened, '/home/ana/src/api')

  // Assert
  await waitFor(() => {
    expect(screen.getByLabelText<HTMLInputElement>('User').value).toBe('x')
  })
  expect(screen.queryByText(/were for/)).toBeNull()
})

test('Save is held while where the server works after a switch elsewhere is still being read', async () => {
  // Arrange
  const opened = await openSettings({ read: left, files: homeOnly }, homeOnly)
  await anEdit()

  // Act
  await switchedMidway(opened, '/home/ana/src/web', configAnswered)

  // Assert
  expect(saveHeld(), 'Save was not held while where the server works was read').toBe(true)
})

test('Save is held while the configuration after a switch elsewhere is still being read', async () => {
  // Arrange
  const opened = await openSettingsAcross({ '/home/ana/src/web': apiFiles })
  await anEdit()

  // Act
  await switchedMidway(opened, '/home/ana/src/web')

  // Assert
  expect(saveHeld(), 'Save was not held while the configuration was read').toBe(true)
})

test('Save is no longer held once the reads after a switch elsewhere keep the unsaved edits', async () => {
  // Arrange
  const opened = await openSettings({ read: left, files: homeOnly }, homeOnly)
  await anEdit()

  // Act
  await switchedElsewhere(opened)

  // Assert
  await waitFor(() => {
    expect(screen.getByLabelText<HTMLInputElement>('User').value).toBe('x')
  })
  expect(saveHeld(), 'Save stayed held after the reads kept the edits').toBe(false)
})

test('a switch elsewhere and on to a copy before the first is read says the unsaved edits were for the directory left first', async () => {
  // Arrange
  const opened = await openSettingsAcross({
    '/home/ana/src/web': webFiles,
    '/home/ana/src/web/sub': webFiles,
  })
  await anEdit()
  await switchedMidway(opened, '/home/ana/src/web')

  // Act
  await switchedElsewhere(opened, '/home/ana/src/web/sub')

  // Assert
  expect(
    await screen.findByText(
      'Your unsaved changes were for /home/ana/src/api, which the server has left, so they were dropped.',
    ),
  ).toBeTruthy()
})
