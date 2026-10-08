import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { LocalData as Listing } from '@/api/generated/types.gen.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { LocalData } from './LocalData.tsx'
import { PeopleAndGroups } from './PeopleAndGroups.tsx'

// Removing everything takes the people and groups with the kept file, so
// Settings shows them gone rather than writing the old ones back; and a removal
// that fails part way reads the listing again.

const api = { id: 'S0API', label: 'api-reviewers' }

const kept = {
  name: 'kept.db',
  kind: 'kept',
  bytes: 512,
  size: '512 B',
  holds: [{ what: 'owner decisions', count: 1 }],
} satisfies Listing['files'][number]

// consequences are the warnings the server gives with the listing.
const consequences = { cache: 'cache words', all: 'everything words' }

// keptUntilRemoved answers People and groups and Local data as a server whose
// kept file removing everything removes; it returns every request made.
function keptUntilRemoved(): Request[] {
  let removed = false

  return fakeApi({
    '/api/local-data': (_: URL, asked: Request) => {
      removed ||= asked.method === 'DELETE'

      return { dir: '/d', files: removed ? [] : [kept], consequences }
    },
    '/api/people': () => ({
      owners: removed ? [] : [{ owner: 'dan', kind: 'user', state: 'not_on_slack' }],
    }),
    '/api/repo-groups': () => ({ repository: 'acme/widgets', groups: removed ? [] : [api] }),
    '/api/slack/groups': { entries: [api] },
    '/api/slack/members': { entries: [] },
  })
}

// removesEverything confirms removing everything and waits for its outcome.
async function removesEverything(user: ReturnType<typeof userEvent.setup>): Promise<void> {
  await user.click(await screen.findByRole('button', { name: 'Remove everything…' }))
  await user.click(screen.getByRole('button', { name: 'Remove' }))
}

test('removing everything shows the people and groups gone', async () => {
  // Arrange
  keptUntilRemoved()
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
  await removesEverything(user)

  // Assert
  await screen.findByText('Removed kept.db.')
  await waitFor(() => {
    expect(screen.queryByRole('button', { name: 'Forget dan…' })).toBeNull()
  })
  await screen.findByRole('checkbox', { name: '@api-reviewers', checked: false })
})

test('saving groups after removing everything does not write the old ones back', async () => {
  // Arrange
  const requests = keptUntilRemoved()
  const user = userEvent.setup()
  renderWithClient(
    <>
      <PeopleAndGroups />
      <LocalData />
    </>,
  )
  await screen.findByRole('checkbox', { name: '@api-reviewers', checked: true })
  await removesEverything(user)
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

test('a removal that fails reads the listing again, since part may be gone', async () => {
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

      return { dir: '/d', files: [kept], consequences }
    },
  })
  const user = userEvent.setup()
  renderWithClient(<LocalData />)

  // Act
  await removesEverything(user)

  // Assert
  await screen.findByText('held open')
  await waitFor(() => {
    expect(reads).toBe(2)
  })
})
