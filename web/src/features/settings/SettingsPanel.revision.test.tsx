import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { mockConfig } from '@/dev/mockConfig.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
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
