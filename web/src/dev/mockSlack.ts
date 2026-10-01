import { Refusal } from '@/api/apiError.ts'
import type {
  AnnouncementTagging,
  OwnerTag,
  People,
  PersonLink,
  RepoGroups,
  SlackDirectory,
  SlackTarget,
} from '@/api/generated/types.gen.ts'

// The mockup's Slack: the announcement channel's members, the workspace's
// user groups, whom each code owner was decided to be, and the groups the
// repository tags. A link or a group saved here shows on the read after, as it
// would against a server, until the page loads again.

const ana: SlackTarget = { id: 'U0ANA', label: 'Ana Souza' }
const carla: SlackTarget = { id: 'U0CARLA', label: 'Carla Diaz' }

// configuredChannel is the channel the mockup posts to unless one is picked.
const configuredChannel = '#dev-workflow'

// membersIn are each channel's members, by name; a channel not named has
// none of the mockup's people.
const membersIn: Record<string, SlackTarget[]> = {
  [configuredChannel]: [ana, { id: 'U0BEN', label: 'Ben Ito' }, carla],
  '#releases': [ana, { id: 'U0ERIN', label: 'Erin Park' }],
}

// What the server refuses a link with, in its words.
const refusals = {
  oneOrTheOther: 'say whom the owner is on Slack, or that they are not on it — one or the other',
  wrongKind: 'a team links to a Slack user group, and a person to a Slack user',
  notListed: "that Slack ID is not among the channel's members or the workspace's user groups",
}

const pod: SlackTarget = { id: 'S0POD', label: 'control-plane-pod' }
const apiReviewers: SlackTarget = { id: 'S0API', label: 'api-reviewers' }
const groups: SlackTarget[] = [pod, apiReviewers, { id: 'S0WEB', label: 'web-guild' }]

// branchOwners are who owns the mock branch's changes: two people and a team.
const branchOwners: Pick<OwnerTag, 'owner' | 'kind'>[] = [
  { owner: 'ben', kind: 'user' },
  { owner: 'carla', kind: 'user' },
  { owner: 'acme/control-plane', kind: 'team' },
]

// held is what the mockup keeps: carla linked, dan not on Slack, the team
// linked to its pod's group, and one group for the repository.
const held: { decided: OwnerTag[]; repoGroups: SlackTarget[] } = {
  decided: [
    { owner: 'carla', kind: 'user', state: 'linked', slack: carla },
    { owner: 'dan', kind: 'user', state: 'not_on_slack' },
    { owner: 'acme/control-plane', kind: 'team', state: 'linked', slack: pod },
  ],
  repoGroups: [apiReviewers],
}

// membersOf is channel's members, the configured channel's when empty.
function membersOf(channel: string): SlackTarget[] {
  return membersIn[channel === '' ? configuredChannel : channel] ?? []
}

// mockSlackMembers is channel's members, the configured channel's when empty.
export function mockSlackMembers(channel = ''): SlackDirectory {
  return { entries: [...membersOf(channel)] }
}

// mockSlackGroups is the workspace's user groups.
export function mockSlackGroups(): SlackDirectory {
  return { entries: [...groups] }
}

// ownerAsDecided is owner as the mockup's decisions have it.
function ownerAsDecided(owner: Pick<OwnerTag, 'owner' | 'kind'>): OwnerTag {
  return (
    held.decided.find((decided) => decided.owner === owner.owner) ?? { ...owner, state: 'unlinked' }
  )
}

// mockPeople is every decided owner, then the branch's undecided ones.
export function mockPeople(): People {
  const undecided = branchOwners.map(ownerAsDecided).filter((owner) => owner.state === 'unlinked')

  return { owners: [...held.decided, ...undecided] }
}

// mockLinkPerson keeps link, labeled from the directory, and answers the
// owners after it. It refuses, as the server does, a link that names both a
// Slack ID and not on Slack or neither, an ID of the wrong kind for the
// owner — a team links to a group, a person to a user — and an ID the
// directory does not list: a person among the members of the channel named.
export function mockLinkPerson(link: PersonLink): People {
  const kind = link.owner.includes('/') ? 'team' : 'user'
  const decided: OwnerTag = link.not_on_slack
    ? { owner: link.owner, kind, state: 'not_on_slack' }
    : { owner: link.owner, kind, state: 'linked', slack: listed(link, kind) }
  if (link.not_on_slack === (link.slack_id !== undefined)) {
    throw new Refusal(refusals.oneOrTheOther)
  }
  held.decided = [...held.decided.filter((owner) => owner.owner !== link.owner), decided]

  return mockPeople()
}

// listed is the Slack user or group link names, as the directory lists it
// for an owner of kind, or the refusal the server gives.
function listed(link: PersonLink, kind: OwnerTag['kind']): SlackTarget {
  const id = link.slack_id ?? ''
  if ((kind === 'team') !== id.startsWith('S')) {
    throw new Refusal(refusals.wrongKind)
  }
  const directory = kind === 'team' ? groups : membersOf(link.channel ?? '')
  const found = directory.find((target) => target.id === id)
  if (found === undefined) {
    throw new Refusal(refusals.notListed)
  }

  return found
}

// mockForgetAll forgets every decision and the repository's groups, as a
// clean of everything removes the kept file.
export function mockForgetAll(): void {
  held.decided = []
  held.repoGroups = []
}

// mockForgetPerson forgets what was decided for owner.
export function mockForgetPerson(owner: string): People {
  held.decided = held.decided.filter((decided) => decided.owner !== owner)

  return mockPeople()
}

// mockRepoGroups is the repository's groups.
export function mockRepoGroups(): RepoGroups {
  return { repository: 'acme/workflow', groups: [...held.repoGroups] }
}

// mockSetRepoGroups keeps the groups ids name, labeled from the directory.
export function mockSetRepoGroups(ids: string[]): RepoGroups {
  held.repoGroups = groups.filter((group) => ids.includes(group.id))

  return mockRepoGroups()
}

// mockTagging is whom the mock announcement proposes to tag: the branch's
// owners as decided, the repository's groups, and the group a linked owning
// team adds, checked.
export function mockTagging(): AnnouncementTagging {
  const owners = branchOwners.map(ownerAsDecided)
  const teamGroups = owners.flatMap((owner) =>
    owner.kind === 'team' && owner.slack ? [owner.slack] : [],
  )
  const fromOwners = (slack: SlackTarget) => teamGroups.some((group) => group.id === slack.id)
  const offered = [
    ...held.repoGroups,
    ...teamGroups.filter((slack) => !held.repoGroups.some((group) => group.id === slack.id)),
  ].map((slack) => ({ slack, checked: fromOwners(slack), from_owners: fromOwners(slack) }))

  return { available: true, owners, groups: offered }
}
