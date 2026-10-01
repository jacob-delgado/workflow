import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { LocalData as Listing } from '@/api/generated/types.gen.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { LocalData } from './LocalData.tsx'
import { PeopleAndGroups } from './PeopleAndGroups.tsx'

// A clean of everything takes the people and groups with the kept file, so
// Settings shows them gone rather than writing the old ones back; and a clean
// that fails part way reads the listing again.

const api = { id: 'S0API', label: 'api-reviewers' }

const kept = {
  name: 'kept.db',
  kind: 'kept',
  bytes: 512,
  holds: [{ what: 'owner decisions', count: 1 }],
} satisfies Listing['files'][number]

// keptUntilCleaned answers People and groups and Local data as a server whose
// kept file a clean of everything removes; it returns every request made.
function keptUntilCleaned(): Request[] {
  let cleaned = false

  return fakeApi({
    '/api/local-data': (_: URL, asked: Request) => {
      cleaned ||= asked.method === 'DELETE'

      return { dir: '/d', files: cleaned ? [] : [kept] }
    },
    '/api/people': () => ({
      owners: cleaned ? [] : [{ owner: 'dan', kind: 'user', state: 'not_on_slack' }],
    }),
    '/api/repo-groups': () => ({ repository: 'acme/widgets', groups: cleaned ? [] : [api] }),
    '/api/slack/groups': { entries: [api] },
    '/api/slack/members': { entries: [] },
  })
}

// cleansEverything confirms a clean of everything and waits for its outcome.
async function cleansEverything(user: ReturnType<typeof userEvent.setup>): Promise<void> {
  await user.click(await screen.findByRole('button', { name: 'Clean everything…' }))
  await user.click(screen.getByRole('button', { name: 'Clean' }))
}

test('cleaning everything shows the people and groups gone', async () => {
  // Arrange
  keptUntilCleaned()
  const user = userEvent.setup()
  renderWithClient(
    <>
      <PeopleAndGroups />
      <LocalData />
    </>,
  )
  await screen.findByRole('button', { name: 'Forget dan…' })
  await screen.findByRole('checkbox', { name: '@api-reviewers', checked: true })

  // Act
  await cleansEverything(user)

  // Assert
  await screen.findByText('Removed kept.db.')
  await waitFor(() => {
    expect(screen.queryByRole('button', { name: 'Forget dan…' })).toBeNull()
  })
  await screen.findByRole('checkbox', { name: '@api-reviewers', checked: false })
})

test('saving groups after cleaning everything does not write the old ones back', async () => {
  // Arrange
  const requests = keptUntilCleaned()
  const user = userEvent.setup()
  renderWithClient(
    <>
      <PeopleAndGroups />
      <LocalData />
    </>,
  )
  await screen.findByRole('checkbox', { name: '@api-reviewers', checked: true })
  await cleansEverything(user)
  await screen.findByRole('checkbox', { name: '@api-reviewers', checked: false })

  // Act
  await user.click(screen.getByRole('button', { name: 'Save groups' }))

  // Assert
  await screen.findByText('Saved the groups for acme/widgets.')
  const saved = requests.filter(
    (request) => new URL(request.url).pathname === '/api/repo-groups' && request.method === 'PUT',
  )
  expect(await saved.at(-1)?.clone().json()).toEqual({ ids: [] })
})

test('a clean that fails reads the listing again, since part may be gone', async () => {
  // Arrange
  let reads = 0
  fakeApi({
    '/api/local-data': (_: URL, asked: Request) => {
      if (asked.method === 'DELETE') {
        return Response.json(
          { status: 409, code: 'conflict', title: 'Conflict', type: '', detail: 'held open' },
          { status: 409 },
        )
      }
      reads += 1

      return { dir: '/d', files: [kept] }
    },
  })
  const user = userEvent.setup()
  renderWithClient(<LocalData />)

  // Act
  await cleansEverything(user)

  // Assert
  await screen.findByText('held open')
  await waitFor(() => {
    expect(reads).toBe(2)
  })
})
