import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { People, PersonLink, RepoGroups } from '@/api/generated/types.gen.ts'
import { useHealthStore } from '@/api/health.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { makeHealth } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { PeopleAndGroups } from './PeopleAndGroups.tsx'

const ben = { id: 'U0BEN', label: 'Ben Ito' }
const carla = { id: 'U0CARLA', label: 'Carla Diaz' }
const pod = { id: 'S0POD', label: 'control-plane-pod' }
const reviewers = { id: 'S0API', label: 'api-reviewers' }

const people: People = {
  owners: [
    { owner: 'carla', kind: 'user', state: 'linked', slack: carla },
    { owner: 'dan', kind: 'user', state: 'not_on_slack' },
    { owner: 'ben', kind: 'user', state: 'unlinked' },
    { owner: 'acme/control-plane', kind: 'team', state: 'linked', slack: pod },
  ],
}

const groups: RepoGroups = { repository: 'acme/widgets', groups: [reviewers] }

// answers answers People and groups from a kept store a write changes, and
// records each request.
function answers(): Request[] {
  let owners = people.owners
  let kept = groups.groups

  return fakeApi({
    '/api/people': async (at: URL, asked: Request) => {
      if (asked.method === 'PUT') {
        const link = (await asked.clone().json()) as PersonLink
        const slack = [ben, carla].find((member) => member.id === link.slack_id)
        owners = owners.map((owner) =>
          owner.owner !== link.owner
            ? owner
            : { ...owner, state: link.not_on_slack ? 'not_on_slack' : 'linked', slack },
        )
      }

      if (asked.method === 'DELETE') {
        owners = owners.filter((owner) => owner.owner !== at.searchParams.get('owner'))
      }

      return { owners }
    },
    '/api/slack/members': { entries: [ben, carla] },
    '/api/slack/groups': { entries: [pod, reviewers] },
    '/api/repo-groups': async (_: URL, asked: Request) => {
      if (asked.method === 'PUT') {
        const { ids } = (await asked.clone().json()) as { ids: string[] }
        kept = [pod, reviewers].filter((group) => ids.includes(group.id))
      }

      return { repository: groups.repository, groups: kept }
    },
  })
}

// sent is what the last request to path with method carried.
async function sent(requests: Request[], method: string, path: string): Promise<unknown> {
  const request = requests
    .filter((asked) => asked.method === method && new URL(asked.url).pathname === path)
    .at(-1)

  return request?.clone().json()
}

test('lists each owner with whom they are on Slack', async () => {
  // Arrange
  answers()

  // Act
  renderWithClient(<PeopleAndGroups />)

  // Assert
  const table = await screen.findByRole('table', { name: 'Code owners on Slack' })
  expect(within(table).getByRole('combobox', { name: 'Slack for carla' })).toHaveProperty(
    'value',
    'U0CARLA',
  )
  expect(within(table).getByRole('combobox', { name: 'Slack for dan' })).toHaveProperty(
    'value',
    'not-on-slack',
  )
  expect(within(table).getByRole('combobox', { name: 'Slack for ben' })).toHaveProperty('value', '')
  expect(within(table).queryByRole('button', { name: 'Forget ben…' })).toBeNull()
})

test('choosing whom an owner is saves it at once', async () => {
  // Arrange
  const requests = answers()
  const user = userEvent.setup()
  renderWithClient(<PeopleAndGroups />)
  const choice = await screen.findByRole('combobox', { name: 'Slack for ben' })
  await within(choice).findByRole('option', { name: 'Ben Ito' })

  // Act
  await user.selectOptions(choice, 'U0BEN')

  // Assert
  expect(await screen.findByText('Saved: ben is Ben Ito.')).toBeTruthy()
  expect(await sent(requests, 'PUT', '/api/people')).toEqual({
    owner: 'ben',
    slack_id: 'U0BEN',
    not_on_slack: false,
  })
})

test('choosing Not on Slack marks the owner so', async () => {
  // Arrange
  const requests = answers()
  const user = userEvent.setup()
  renderWithClient(<PeopleAndGroups />)

  // Act
  await user.selectOptions(
    await screen.findByRole('combobox', { name: 'Slack for carla' }),
    'not-on-slack',
  )

  // Assert
  expect(await screen.findByText('Saved: carla is not on Slack.')).toBeTruthy()
  expect(await sent(requests, 'PUT', '/api/people')).toEqual({ owner: 'carla', not_on_slack: true })
})

test('forgetting an owner asks first, then forgets them', async () => {
  // Arrange
  const requests = answers()
  const user = userEvent.setup()
  renderWithClient(<PeopleAndGroups />)
  await user.click(await screen.findByRole('button', { name: 'Forget dan…' }))
  const question = screen.getByRole('group', { name: 'Forget dan?' })
  expect(document.activeElement).toBe(question)

  // Act
  await user.click(within(question).getByRole('button', { name: 'Forget' }))

  // Assert
  expect(await screen.findByText('Forgot dan: they are asked about again.')).toBeTruthy()
  const forgot = requests.find((asked) => asked.method === 'DELETE')
  expect(forgot === undefined ? '' : new URL(forgot.url).search).toBe('?owner=dan')
  expect(screen.queryByRole('combobox', { name: 'Slack for dan' })).toBeNull()
})

test('Cancel forgets no one and hands focus back', async () => {
  // Arrange
  const requests = answers()
  const user = userEvent.setup()
  renderWithClient(<PeopleAndGroups />)
  await user.click(await screen.findByRole('button', { name: 'Forget dan…' }))

  // Act
  await user.click(screen.getByRole('button', { name: 'Cancel' }))

  // Assert
  expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Forget dan…' }))
  expect(requests.some((asked) => asked.method === 'DELETE')).toBe(false)
})

test('the repository’s groups are checked and saved together', async () => {
  // Arrange
  const requests = answers()
  const user = userEvent.setup()
  renderWithClient(<PeopleAndGroups />)
  const choice = await screen.findByRole('group', { name: 'Groups for acme/widgets' })
  await user.click(await within(choice).findByRole('checkbox', { name: '@control-plane-pod' }))

  // Act
  await user.click(screen.getByRole('button', { name: 'Save groups' }))

  // Assert
  expect(await screen.findByText('Saved the groups for acme/widgets.')).toBeTruthy()
  expect(await sent(requests, 'PUT', '/api/repo-groups')).toEqual({ ids: ['S0API', 'S0POD'] })
})

test('a missing scope is named where the groups are chosen', async () => {
  // Arrange
  fakeApi({
    '/api/people': { owners: [] },
    '/api/repo-groups': groups,
    '/api/slack/groups': { entries: [], missing_scope: 'usergroups:read' },
  })

  // Act
  renderWithClient(<PeopleAndGroups />)

  // Assert
  const note = await screen.findByRole('note')
  expect(note.textContent).toContain('usergroups:read')
})

test('under --dry-run the changes are explained as held back', async () => {
  // Arrange
  answers()
  useHealthStore.setState({ health: makeHealth({ dry_run: true }) })

  // Act
  renderWithClient(<PeopleAndGroups />)

  // Assert
  expect(screen.getByText(/nothing here is saved/)).toBeTruthy()
  expect(await screen.findByRole('combobox', { name: 'Slack for ben' })).toHaveProperty(
    'disabled',
    true,
  )
  expect(screen.getByRole('button', { name: 'Save groups' })).toHaveProperty('disabled', true)
})
