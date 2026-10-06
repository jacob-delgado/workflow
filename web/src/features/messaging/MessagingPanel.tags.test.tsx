import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type {
  Announcement,
  AnnouncementTagging,
  People,
  PullRequest,
  SlackDirectory,
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

// Answers are what the server answers besides the preview: a link, and the
// members of each channel, by name.
interface Answers {
  people?: People | (() => Promise<People>)
  members?: Record<string, SlackDirectory>
  channels?: string[]
}

// opensPreview answers the preview with announcement, the channels' members
// and a link, and opens the preview; it returns every request made.
async function opensPreview(announcement: Announcement, answers: Answers = {}) {
  const members = answers.members ?? { '#dev': { entries: [ben, carla] } }
  const requests = fakeApi({
    '/api/announcement': announcement,
    '/api/slack/members': (at: URL) => members[at.searchParams.get('channel') ?? ''],
    '/api/slack/groups': { entries: [pod, api] },
    '/api/people': answers.people ?? benLinked,
    '/api/announce': { text, channel: '#dev' },
  })
  const snapshot = makeSnapshot({ review: { found: true, announced: false, pull } })
  snapshot.messaging.channels = answers.channels ?? []
  useSnapshotStore.setState({ status: 'live', snapshot })
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
    channel: '#dev',
  })
  expect(screen.getByText('@Ben Ito, @Carla Diaz, @control-plane-pod')).toBeTruthy()
})

test('an owner marked not on Slack is saved as such', async () => {
  // Arrange
  const notOnSlack: People = {
    owners: [{ owner: 'ben', kind: 'user', state: 'not_on_slack' }, ...tagging.owners.slice(1)],
  }
  const { requests, user } = await opensPreview(
    { text, channel: '#dev', tagging },
    { people: notOnSlack },
  )

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
    mentions: { users: ['U0CARLA'], groups: ['S0POD', 'S0API'] },
  })
})

test('a missing scope is named, and the announcement still posts', async () => {
  // Arrange
  const { user } = await opensPreview(
    { text, channel: '#dev', tagging: { ...tagging, missing_scope: 'users:read' } },
    { members: { '#dev': { entries: [], missing_scope: 'users:read' } } },
  )
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

test('an announcement whose Slack workspace is unknown says why and posts untagged', async () => {
  // Arrange
  const reason = "can't tell which Slack workspace this token is for"
  const { requests, user } = await opensPreview({
    text,
    channel: '#dev',
    tagging: { available: false, unavailable_reason: reason, owners: [], groups: [] },
  })
  expect(screen.getByRole('note').textContent).toContain(reason)

  // Act
  await user.click(screen.getByRole('button', { name: 'Announce now' }))

  // Assert
  await screen.findByText('Announced to #dev.')
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

// twoChannels are the channels a preview can post to, with a member each
// alone has.
const olive = { id: 'U0OLIVE', label: 'Olive Ops' }
const twoChannels: Answers = {
  channels: ['#dev', '#ops'],
  members: { '#dev': { entries: [ben, carla] }, '#ops': { entries: [olive, carla] } },
}

test('the preview is composed for the channel it opens on', async () => {
  // Act
  const { requests } = await opensPreview({ text, channel: '#dev', tagging }, twoChannels)

  // Assert
  const composed = requests.find((request) => new URL(request.url).pathname === '/api/announcement')
  expect(new URL(composed?.url ?? 'http://x').searchParams.get('channel')).toBe('#dev')
})

test('an owner is linked to a member of the channel picked, named with it', async () => {
  // Arrange
  const oliveLinked: People = {
    owners: [
      { owner: 'ben', kind: 'user', state: 'linked', slack: olive },
      ...tagging.owners.slice(1),
    ],
  }
  const { requests, user } = await opensPreview(
    { text, channel: '#dev', tagging },
    { ...twoChannels, people: oliveLinked },
  )
  await user.selectOptions(screen.getByRole('combobox', { name: /^Channel/ }), '#ops')
  await screen.findByRole('option', { name: 'Olive Ops' })

  // Act
  await user.selectOptions(screen.getByRole('combobox', { name: 'Slack user for ben' }), 'U0OLIVE')

  // Assert
  await screen.findByText('Saved for next time: ben is Olive Ops.')
  expect(await bodyOf(requests, '/api/people')).toEqual({
    owner: 'ben',
    slack_id: 'U0OLIVE',
    not_on_slack: false,
    channel: '#ops',
  })
})

test('a scope the opening channel lacks gives way to the channel picked', async () => {
  // Arrange
  const { user } = await opensPreview(
    { text, channel: '#dev', tagging: { ...tagging, missing_scope: 'groups:read' } },
    {
      ...twoChannels,
      members: {
        '#dev': { entries: [], missing_scope: 'groups:read' },
        '#ops': { entries: [olive] },
      },
    },
  )

  // Act
  await user.selectOptions(screen.getByRole('combobox', { name: /^Channel/ }), '#ops')

  // Assert
  await screen.findByRole('option', { name: 'Olive Ops' })
  expect(screen.queryByRole('note')).toBeNull()
})

// teamTagging is an announcement whose team owner waits to be linked, with
// one group offered.
const teamTagging: AnnouncementTagging = {
  available: true,
  owners: [{ owner: 'acme/control-plane', kind: 'team', state: 'unlinked' }],
  groups: [{ slack: api, checked: false, from_owners: false }],
}

// teamLinkHeld answers a link only once release is called.
function teamLinkHeld() {
  const held = { release: () => {} }
  const people = () =>
    new Promise<People>((resolve) => {
      held.release = () => {
        resolve({
          owners: [{ owner: 'acme/control-plane', kind: 'team', state: 'linked', slack: pod }],
        })
      }
    })

  return { held, people }
}

test('a group checked while a team link is saved stays checked, and is posted', async () => {
  // Arrange
  const { held, people } = teamLinkHeld()
  const { requests, user } = await opensPreview(
    { text, channel: '#dev', tagging: teamTagging },
    { people },
  )
  await screen.findByRole('option', { name: 'control-plane-pod' })
  await user.selectOptions(
    screen.getByRole('combobox', { name: 'Slack group for acme/control-plane' }),
    'S0POD',
  )
  await user.click(screen.getByRole('checkbox', { name: /@api-reviewers/ }))
  held.release()
  await screen.findByText('Saved for next time: acme/control-plane is control-plane-pod.')

  // Act
  await user.click(screen.getByRole('button', { name: 'Announce now' }))

  // Assert
  await screen.findByText('Announced to #dev.')
  expect(await bodyOf(requests, '/api/announce')).toEqual({
    channel: '#dev',
    text,
    mentions: { users: [], groups: ['S0API', 'S0POD'] },
  })
})

test('the post waits while a link is being saved', async () => {
  // Arrange
  const { held, people } = teamLinkHeld()
  const { user } = await opensPreview({ text, channel: '#dev', tagging: teamTagging }, { people })
  await screen.findByRole('option', { name: 'control-plane-pod' })

  // Act
  await user.selectOptions(
    screen.getByRole('combobox', { name: 'Slack group for acme/control-plane' }),
    'S0POD',
  )

  // Assert
  expect(screen.getByRole('button', { name: 'Announce now' }).getAttribute('aria-disabled')).toBe(
    'true',
  )
  held.release()
  await screen.findByText(/Saved for next time/)
  expect(screen.getByRole('button', { name: 'Announce now' }).hasAttribute('aria-disabled')).toBe(
    false,
  )
})

// refusedLink answers a link with the server's refusal.
function refusedLink(): Promise<People> {
  return Promise.reject(new Error('refused'))
}

test('a refused link from the owner select says why, with focus still on the select', async () => {
  // Arrange
  const { user } = await opensPreview(
    { text, channel: '#dev', tagging },
    { people: () => refusedLink() },
  )
  await screen.findByRole('option', { name: 'Ben Ito' })
  const owner = screen.getByRole('combobox', { name: 'Slack user for ben' })

  // Act
  await user.selectOptions(owner, 'U0BEN')

  // Assert
  expect(await screen.findByRole('alert')).toBeTruthy()
  expect(document.activeElement).toBe(owner)
})

test('a refused Not on Slack says why, with focus still on it', async () => {
  // Arrange
  const { user } = await opensPreview(
    { text, channel: '#dev', tagging },
    { people: () => refusedLink() },
  )
  const notOnSlack = screen.getByRole('button', { name: 'ben is not on Slack' })

  // Act
  await user.click(notOnSlack)

  // Assert
  expect(await screen.findByRole('alert')).toBeTruthy()
  expect(document.activeElement).toBe(notOnSlack)
})
