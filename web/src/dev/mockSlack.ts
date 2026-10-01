import type {
  AnnouncementTagging,
  OwnerTag,
  People,
  PersonLink,
  SlackDirectory,
  SlackTarget,
} from '@/api/generated/types.gen.ts'

// The mockup's Slack: the announcement channel's members, the workspace's
// user groups, whom each code owner was decided to be, and the groups the
// repository tags. A link or a group saved here shows on the read after, as it
// would against a server, until the page loads again.

const carla: SlackTarget = { id: 'U0CARLA', label: 'Carla Diaz' }
const members: SlackTarget[] = [
  { id: 'U0ANA', label: 'Ana Souza' },
  { id: 'U0BEN', label: 'Ben Ito' },
  carla,
]

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

// mockSlackMembers is the channel's members.
export function mockSlackMembers(): SlackDirectory {
  return { entries: [...members] }
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
// owners after it.
export function mockLinkPerson(link: PersonLink): People {
  const kind = link.owner.includes('/') ? 'team' : 'user'
  const slack = [...members, ...groups].find((target) => target.id === link.slack_id)
  const decided: OwnerTag = link.not_on_slack
    ? { owner: link.owner, kind, state: 'not_on_slack' }
    : { owner: link.owner, kind, state: 'linked', slack }
  held.decided = [...held.decided.filter((owner) => owner.owner !== link.owner), decided]

  return mockPeople()
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
