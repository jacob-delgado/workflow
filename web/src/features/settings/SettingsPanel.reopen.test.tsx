import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { QueuedAnnouncement } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { mockConfig } from '@/dev/mockConfig.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { anEdit } from '@/test/settings.ts'
import { SettingsPanel } from './SettingsPanel.tsx'

// A save reopens the server with the file, as a switch wires a directory, so
// every setting applies at once; Settings says when one could not, and asks
// before a save that drops the announcement waiting for CI.

// waiting is an announcement the server holds until #42's CI passes.
const waiting: QueuedAnnouncement = { id: 1, state: 'waiting', channel: '#dev', pull: 42 }

// later says a save the server could not reopen with applies later, and why:
// another write in flight, or a directory that could not be wired again.
const later =
  /could not reopen with this save: another write was in flight, or the directory could not be wired again\. It applies at the next save/

// opened opens Settings, answering each save as reopened says, and returns
// the requests the page made.
async function opened(reopened: boolean): Promise<Request[]> {
  const requests = fakeApi({
    '/api/config': (_at: URL, asked: Request) =>
      Response.json(mockConfig, {
        headers: {
          ETag: asked.method === 'GET' ? '"read-1"' : '"saved-1"',
          'Workflow-Reopened': String(reopened),
        },
      }),
  })
  renderWithClient(<SettingsPanel />)
  await screen.findByLabelText('Base URL')

  return requests
}

// saves is how many saves the page sent.
function saves(requests: Request[]): number {
  return requests.filter((request) => request.method === 'PUT').length
}

// holding puts the announcement held for CI in the stream's frame.
function holding(held: QueuedAnnouncement): void {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ queued_announcement: held }),
  })
}

test('a save the server could not reopen with says when it applies', async () => {
  // Arrange
  await opened(false)
  await anEdit()

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))

  // Assert
  expect(await screen.findByText(later)).toBeTruthy()
})

test('a save the server reopened with says only that it saved', async () => {
  // Arrange
  await opened(true)
  await anEdit()

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))

  // Assert
  await screen.findByText('Saved.')
  expect(screen.queryByText(later)).toBeNull()
})

test('a save while an announcement waits for CI asks first, and saves once asked', async () => {
  // Arrange
  holding(waiting)
  const requests = await opened(true)
  await anEdit()

  // Act: press Save
  await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))

  // Assert: it asks, naming what the save drops, and sends nothing yet
  const question = screen.getByRole('group', { name: /drop the announcement waiting/i })
  expect(within(question).getByText(/#42/)).toBeTruthy()
  expect(saves(requests)).toBe(0)

  // Act: save anyway
  await userEvent.click(within(question).getByRole('button', { name: 'Save' }))

  // Assert: the save is sent
  await screen.findByText('Saved.')
  expect(saves(requests)).toBe(1)
})

test('canceling the question over a waiting announcement sends nothing', async () => {
  // Arrange
  holding(waiting)
  const requests = await opened(true)
  await anEdit()
  await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))
  const question = screen.getByRole('group', { name: /drop the announcement waiting/i })

  // Act
  await userEvent.click(within(question).getByRole('button', { name: 'Cancel' }))

  // Assert
  await waitFor(() => {
    expect(screen.queryByRole('group', { name: /drop the announcement waiting/i })).toBeNull()
  })
  expect(saves(requests)).toBe(0)
})

test('a removal while an announcement waits for CI says it drops it', async () => {
  // Arrange
  holding(waiting)
  await opened(true)

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Remove the Jira token…' }))

  // Assert
  const question = screen.getByRole('group', { name: 'Remove the Jira token from the file?' })
  expect(within(question).getByText(/drops the announcement waiting for #42/)).toBeTruthy()
})
