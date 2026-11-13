import { act, screen, waitFor } from '@testing-library/react'
import type { QueryClient } from '@tanstack/react-query'
import App from '@/App.tsx'
import { getConfigQueryKey } from '@/api/generated/@tanstack/react-query.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import type { Config } from '@/api/generated/types.gen.ts'
import { mockConfig } from '@/dev/mockConfig.ts'
import { mockKeys } from '@/dev/mockKeys.ts'
import { useUiStore } from '@/shell/uiStore.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { appQueryClient, renderWithClient } from '@/test/renderWithClient.tsx'
import { anEdit } from '@/test/settings.ts'

// Settings open while the server switches directory from another tab or the
// terminal, as a user meets it: the form's unsaved edits stay when the
// configuration read back is the same file, and otherwise the page says they
// were for the directory left.

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

// answer is a read of the configuration as the server answers it.
function answer(read: Read | typeof noFile): Response {
  if (read === noFile) {
    const problem = { type: 'about:blank', title: 'Refused', status: 404, code: 'not_found' }

    return Response.json({ ...problem, detail: 'no .workflow.json applies' }, { status: 404 })
  }

  return Response.json(read.config, { headers: { ETag: read.revision } })
}

// Opened is Settings drawn: the page's query client, and the requests the
// page made.
interface Opened {
  client: QueryClient
  requests: Request[]
}

// openSettings draws Settings working in api, answering the first read of
// the configuration with left's file and every read after with after, a
// moment later, as a read over the network answers; and returns the page's
// client and requests once the form is drawn.
async function openSettings(after: Read | typeof noFile): Promise<Opened> {
  let reads = 0
  const requests = fakeApi({
    '/api/keys': mockKeys,
    '/api/config': async () => {
      reads += 1
      if (reads === 1) {
        return answer(left)
      }
      await new Promise((resolve) => setTimeout(resolve, 20))

      return answer(after)
    },
  })
  const client = appQueryClient()
  useSnapshotStore.setState({ status: 'live', snapshot: makeSnapshot() })
  useUiStore.setState({ section: 'settings' })
  renderWithClient(<App />, client)
  await screen.findByLabelText('User')

  return { client, requests }
}

// switchedElsewhere has the stream say the server works in web now, and waits
// until the configuration read again there has answered, with the file or a
// refusal, and nothing more is being read.
async function switchedElsewhere({ client, requests }: Opened): Promise<void> {
  const before = configReads(requests)
  const answered = () => {
    const state = client.getQueryState(getConfigQueryKey())

    return Math.max(state?.dataUpdatedAt ?? 0, state?.errorUpdatedAt ?? 0)
  }
  const answeredBefore = answered()
  act(() => {
    useSnapshotStore.setState({ snapshot: makeSnapshot({ here: '/home/ana/src/web' }) })
  })
  await waitFor(() => {
    expect(configReads(requests)).toBeGreaterThan(before)
    expect(answered()).toBeGreaterThan(answeredBefore)
    expect(client.getQueryState(getConfigQueryKey())?.fetchStatus).toBe('idle')
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
  const opened = await openSettings(left)
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

test('a switch elsewhere to another file says the unsaved edits were for the directory left', async () => {
  // Arrange
  const opened = await openSettings(elsewhere)
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
  const opened = await openSettings(elsewhere)

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
  const opened = await openSettings(noFile)
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
