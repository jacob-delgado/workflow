import type {
  AnnounceMentions,
  AnnouncementTagging,
  GroupTag,
  OwnerTag,
  People,
  SlackTarget,
} from '@/api/generated/types.gen.ts'

// TagPick is whom an announcement's preview tags as it stands: the code
// owners, as linked so far, the user groups it offers, and the ones checked.
export interface TagPick {
  owners: OwnerTag[]
  groups: GroupTag[]
  checked: string[]
}

// pickFrom is the pick a preview starts with: the owners and groups the
// server proposed, its pre-checked groups checked.
export function pickFrom(tagging: AnnouncementTagging | undefined): TagPick {
  const groups = tagging?.groups ?? []

  return {
    owners: tagging?.owners ?? [],
    groups,
    checked: groups.filter((group) => group.checked).map((group) => group.slack.id),
  }
}

// withPeople is pick once a link is saved: each owner as people has them now,
// and the group a team linked here stands for, offered and checked, as the
// server offers it at the post. Whatever else was checked meanwhile stays.
export function withPeople(pick: TagPick, people: People): TagPick {
  const owners = pick.owners.map(
    (owner) => people.owners.find((decided) => decided.owner === owner.owner) ?? owner,
  )
  const added = teamGroups(owners).filter(
    (slack) => !pick.groups.some((group) => group.slack.id === slack.id),
  )

  return {
    owners,
    groups: [
      ...pick.groups,
      ...added.map((slack) => ({ slack, checked: true, from_owners: true })),
    ],
    checked: [
      ...pick.checked,
      ...added.map((slack) => slack.id).filter((id) => !pick.checked.includes(id)),
    ],
  }
}

// withGroup is pick with the group id checked or not.
export function withGroup(pick: TagPick, id: string, checked: boolean): TagPick {
  const others = pick.checked.filter((chosen) => chosen !== id)

  return { ...pick, checked: checked ? [...others, id] : others }
}

// linkedPeople are the Slack users the linked user owners are.
export function linkedPeople(owners: OwnerTag[]): SlackTarget[] {
  return owners.flatMap((owner) =>
    owner.kind === 'user' && owner.state === 'linked' && owner.slack ? [owner.slack] : [],
  )
}

// mentionsOf is what a post asks to tag: the people the preview shows
// tagged, which the server checks are still the ones linked, and the groups
// checked.
export function mentionsOf(pick: TagPick): AnnounceMentions {
  return { users: linkedPeople(pick.owners).map((person) => person.id), groups: pick.checked }
}

// teamGroups are the user groups linked teams among owners stand for.
function teamGroups(owners: OwnerTag[]): SlackTarget[] {
  return owners.flatMap((owner) =>
    owner.kind === 'team' && owner.state === 'linked' && owner.slack ? [owner.slack] : [],
  )
}
