import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type {
  Announcement,
  AnnouncementTagging,
  People,
  PullRequest,
} from '@/api/generated/types.gen.ts'
import { useHealthStore } from '@/api/health.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { makeSnapshot } from '@/test/fixtures.ts'
import { MessagingPanel } from './MessagingPanel.tsx'

// The announcement preview's tags: the code owners, linked here and saved
// for next time, the user groups checked, and the post that carries them.

const pull: PullRequest = {
  number: 7,
  url: 'https://forge.example.com/pull/7',
  title: 'Redact tokens in the request log',
  state: 'open',
  draft: false,
  approvals: 0,
  changes_requested: false,
  mergeable: 'clean',
}

const text = 'octocat opened a pull request: Redact tokens in the request log'

const ben = { id: 'U0BEN', label: 'Ben Ito' }
const carla = { id: 'U0CARLA', label: 'Carla Diaz' }
const pod = { id: 'S0POD', label: 'control-plane-pod' }
const api = { id: 'S0API', label: 'api-reviewers' }

const tagging: AnnouncementTagging = {
  available: true,
  owners: [
    { owner: 'ben', kind: 'user', state: 'unlinked' },
    { owner: 'carla', kind: 'user', state: 'linked', slack: carla },
    { owner: 'dan', kind: 'user', state: 'not_on_slack' },
    { owner: 'acme/control-plane', kind: 'team', state: 'linked', slack: pod },
  ],
  groups: [
    { slack: api, checked: false, from_owners: false },
    { slack: pod, checked: true, from_owners: true },
  ],
}

// benLinked is every owner once ben is linked.
const benLinked: People = {
  owners: [{ owner: 'ben', kind: 'user', state: 'linked', slack: ben }, ...tagging.owners.slice(1)],
}

// opensPreview answers the preview with announcement, the channel's members
// and a link, and opens the preview; it returns every request made.
async function opensPreview(announcement: Announcement, people: People = benLinked) {
  const requests = fakeApi({
    '/api/announcement': announcement,
    '/api/slack/members': { entries: [ben, carla] },
    '/api/people': people,
    '/api/announce': { text, channel: '#dev' },
  })
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ review: { found: true, pull } }),
  })
  const user = userEvent.setup()
  renderWithClient(<MessagingPanel />)
  await user.click(screen.getByRole('button', { name: 'Announce to Slack' }))
  await screen.findByRole('button', { name: 'Announce now' })

  return { requests, user }
}

// bodyOf is what the last request to path carried.
async function bodyOf(requests: Request[], path: string): Promise<unknown> {
  const sent = requests.filter((request) => new URL(request.url).pathname === path).at(-1)

  return sent?.clone().json()
}

test('the preview shows each owner, the groups and whom it tags', async () => {
  // Act
  await opensPreview({ text, channel: '#dev', tagging })

  // Assert
  const owners = screen.getByRole('list', { name: 'Tag code owners' })
  expect(within(owners).getByText('→ Carla Diaz')).toBeTruthy()
  expect(within(owners).getByText('· not on Slack')).toBeTruthy()
  expect(screen.getByRole('combobox', { name: 'Slack user for ben' })).toBeTruthy()
  const groups = screen.getByRole('group', { name: 'Tag groups' })
  expect(within(groups).getByRole('checkbox', { name: /@control-plane-pod/ })).toHaveProperty(
    'checked',
    true,
  )
  expect(screen.getByText('@Carla Diaz, @control-plane-pod')).toBeTruthy()
})

test('linking an owner saves it for next time and tags them', async () => {
  // Arrange
  const { requests, user } = await opensPreview({ text, channel: '#dev', tagging })
  await screen.findByRole('option', { name: 'Ben Ito' })

  // Act
  await user.selectOptions(screen.getByRole('combobox', { name: 'Slack user for ben' }), 'U0BEN')

  // Assert
  await screen.findByText('Saved for next time: ben is Ben Ito.')
  expect(await bodyOf(requests, '/api/people')).toEqual({
    owner: 'ben',
    slack_id: 'U0BEN',
    not_on_slack: false,
  })
  expect(screen.getByText('@Ben Ito, @Carla Diaz, @control-plane-pod')).toBeTruthy()
})

test('an owner marked not on Slack is saved as such', async () => {
  // Arrange
  const notOnSlack: People = {
    owners: [{ owner: 'ben', kind: 'user', state: 'not_on_slack' }, ...tagging.owners.slice(1)],
  }
  const { requests, user } = await opensPreview({ text, channel: '#dev', tagging }, notOnSlack)

  // Act
  await user.click(screen.getByRole('button', { name: 'ben is not on Slack' }))

  // Assert
  await screen.findByText('Saved for next time: ben is not on Slack.')
  expect(await bodyOf(requests, '/api/people')).toEqual({ owner: 'ben', not_on_slack: true })
})

test('the post carries the groups checked', async () => {
  // Arrange
  const { requests, user } = await opensPreview({ text, channel: '#dev', tagging })
  await user.click(screen.getByRole('checkbox', { name: /@api-reviewers/ }))

  // Act
  await user.click(screen.getByRole('button', { name: 'Announce now' }))

  // Assert
  await screen.findByText('Announced to #dev.')
  expect(await bodyOf(requests, '/api/announce')).toEqual({
    channel: '#dev',
    text,
    mentions: { groups: ['S0POD', 'S0API'] },
  })
})

test('a missing scope is named, and the announcement still posts', async () => {
  // Arrange
  const { user } = await opensPreview({
    text,
    channel: '#dev',
    tagging: { ...tagging, missing_scope: 'users:read' },
  })
  expect(screen.getByRole('note').textContent).toContain('users:read')

  // Act
  await user.click(screen.getByRole('button', { name: 'Announce now' }))

  // Assert
  expect(await screen.findByText('Announced to #dev.')).toBeTruthy()
})

test('an announcement that tags no one posts without mentions', async () => {
  // Arrange
  const { requests, user } = await opensPreview({
    text,
    channel: '#dev',
    tagging: { available: false, owners: [], groups: [] },
  })

  // Act
  await user.click(screen.getByRole('button', { name: 'Announce now' }))

  // Assert
  await screen.findByText('Announced to #dev.')
  expect(screen.queryByRole('group', { name: 'Tag groups' })).toBeNull()
  expect(await bodyOf(requests, '/api/announce')).toEqual({ channel: '#dev', text })
})

test('under --dry-run an owner is not offered to link', async () => {
  // Arrange
  useHealthStore.setState({
    health: { version: 'dev', dry_run: true, forge_noun: 'pull request', forge_sigil: '#' },
  })

  // Act
  await opensPreview({ text, channel: '#dev', tagging })

  // Assert
  expect(screen.queryByRole('combobox', { name: 'Slack user for ben' })).toBeNull()
  expect(screen.getByText(/linking is held back under --dry-run/)).toBeTruthy()
})
