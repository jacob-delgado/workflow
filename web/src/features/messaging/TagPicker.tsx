import { type Dispatch, type SetStateAction, useId } from 'react'
import type {
  AnnouncementTagging,
  GroupTag,
  OwnerTag,
  People,
  PersonLink,
  SlackTarget,
} from '@/api/generated/types.gen.ts'
import { useHealthStore } from '@/api/health.ts'
import { Button } from '@/lib/Button.tsx'
import { Select } from '@/lib/Field.tsx'
import { OutcomeLine, useOutcome } from '@/lib/Outcome.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { savePerson, useSlackGroups, useSlackMembers } from './slackApi.ts'
import { linkedPeople, type TagPick, withGroup, withPeople } from './tagPick.ts'

interface TagPickerProps {
  tagging: AnnouncementTagging
  pick: TagPick
  channel: string
  posting: boolean
  onPick: Dispatch<SetStateAction<TagPick>>
  onLinking: (linking: boolean) => void
}

// TagPicker is whom a ready-for-review announcement tags: the code owners of
// the branch's changes — an owner not yet linked is linked here, to a member
// of the channel the preview posts to, saved for next time, or marked not on
// Slack — and the user groups it offers, as checkboxes. The summary under
// them says who the post will tag. Every change to the pick is made from the
// pick as it stands then, so a group checked while a link is saved stays
// checked; onLinking hears a link start and end, for the post to wait.
export function TagPicker({ tagging, pick, channel, posting, onPick, onLinking }: TagPickerProps) {
  const outcome = useOutcome()
  const link = useAsyncAction(saving(onLinking), {
    fallback: 'That was not saved. Try again, or link them in Settings.',
    done: (answered, asked) => `Saved for next time: ${savedAs(asked, answered)}.`,
    onStart: outcome.clear,
    onDone: (said, answered) => {
      outcome.say(said)
      onPick((current) => withPeople(current, answered))
    },
  })
  const linking = link.state === 'running'
  const choices = useLinkChoices(pick.owners, channel)
  // What the directory says for the channel picked now wins over what the
  // preview read for the channel it opened on.
  const scope = choices.settled ? choices.missingScope : tagging.missing_scope

  return (
    <div className="flex flex-col gap-group">
      {scope === undefined ? null : <MissingScope scope={scope} />}
      {pick.owners.length > 0 ? (
        <OwnerList
          owners={pick.owners}
          members={choices.members}
          groups={choices.groups}
          channel={channel}
          busy={posting || linking}
          onLink={(asked) => void link.run(asked)}
        />
      ) : null}
      {link.state === 'error' ? (
        <p role="alert" className="text-sm text-destructive">
          {link.error}
        </p>
      ) : null}
      <OutcomeLine said={outcome.said} />
      {pick.groups.length > 0 ? (
        <GroupChecks
          groups={pick.groups}
          checked={pick.checked}
          disabled={posting}
          onCheck={(id, checked) => {
            onPick((current) => withGroup(current, id, checked))
          }}
        />
      ) : null}
      <p className="text-sm break-words">
        <span className="text-muted-foreground">Tags: </span>
        {tagsSummary(pick)}
      </p>
    </div>
  )
}

// saving is the save of a link, which tells onLinking while it is under way,
// however it ends.
function saving(onLinking: (linking: boolean) => void) {
  return async (asked: PersonLink): Promise<People> => {
    onLinking(true)
    try {
      return await savePerson(asked)
    } finally {
      onLinking(false)
    }
  }
}

// useLinkChoices reads whom an owner not yet linked could be: the channel's
// members for a person, the user groups for a team, each read only while such
// an owner waits; a scope the token lacks for either read; and whether every
// read asked for has answered.
function useLinkChoices(owners: OwnerTag[], channel: string) {
  const unlinked = owners.filter((owner) => owner.state === 'unlinked')
  const wantsMembers = unlinked.some((owner) => owner.kind === 'user')
  const wantsGroups = unlinked.some((owner) => owner.kind === 'team')
  const members = useSlackMembers(channel, wantsMembers)
  const groups = useSlackGroups(wantsGroups)

  return {
    members: members.data?.entries ?? [],
    groups: groups.data?.entries ?? [],
    missingScope: members.data?.missing_scope ?? groups.data?.missing_scope,
    settled: [members, groups].every((read) => !read.isEnabled || read.isSuccess),
  }
}

// savedAs words what was saved for an owner: whom Slack's directory says
// they are, or that they are not on Slack.
function savedAs(asked: PersonLink, answered: People): string {
  if (asked.not_on_slack) {
    return `${asked.owner} is not on Slack`
  }

  const label = answered.owners.find((owner) => owner.owner === asked.owner)?.slack?.label

  return `${asked.owner} is ${label ?? asked.slack_id ?? ''}`
}

// tagsSummary names everyone the post tags: the linked people, then the
// groups checked.
function tagsSummary(pick: TagPick): string {
  const chosen = pick.groups.filter((group) => pick.checked.includes(group.slack.id))
  const names = [...linkedPeople(pick.owners), ...chosen.map((group) => group.slack)].map(
    (target) => `@${target.label}`,
  )

  return names.length === 0 ? 'no one' : names.join(', ')
}

// MissingScope says which scope the Slack token lacks to offer whom an owner
// could be linked to, and that the post goes out all the same.
function MissingScope({ scope }: { scope: string }) {
  return (
    <p role="note" className="text-sm text-muted-foreground">
      Linking owners needs the <code className="font-mono">{scope}</code> scope, which the Slack
      token lacks: add it to the Slack app, then run{' '}
      <code className="font-mono">workflow slack login</code>. The announcement still posts, tagging
      whom it can.
    </p>
  )
}

interface OwnerListProps {
  owners: OwnerTag[]
  members: SlackTarget[]
  groups: SlackTarget[]
  channel: string
  busy: boolean
  onLink: (link: PersonLink) => void
}

// OwnerList is the code owners, each with whom they are on Slack, or the
// controls that decide it.
function OwnerList({ owners, members, groups, channel, busy, onLink }: OwnerListProps) {
  const headingId = useId()

  return (
    <div className="flex flex-col gap-tight">
      <h3 id={headingId} className="text-sm font-semibold">
        Tag code owners
      </h3>
      <ul aria-labelledby={headingId} className="flex flex-col gap-tight text-sm">
        {owners.map((owner) => (
          <li key={owner.owner} className="flex flex-wrap items-center gap-item">
            <span className="font-mono break-all">{owner.owner}</span>
            <OwnerOnSlack
              owner={owner}
              choices={owner.kind === 'team' ? groups : members}
              channel={channel}
              busy={busy}
              onLink={onLink}
            />
          </li>
        ))}
      </ul>
    </div>
  )
}

interface OwnerOnSlackProps {
  owner: OwnerTag
  choices: SlackTarget[]
  channel: string
  busy: boolean
  onLink: (link: PersonLink) => void
}

// OwnerOnSlack is whom an owner is on Slack; for one not linked yet, a choice
// of the channel's members — a team's, of the user groups — saved as it is
// made — a member with the channel they were picked from, which the server
// checks them against — and a button for one not on Slack. Under --dry-run
// nothing is saved, so it says so instead.
function OwnerOnSlack({ owner, choices, channel, busy, onLink }: OwnerOnSlackProps) {
  const dryRun = useHealthStore((state) => state.health?.dry_run === true)

  if (owner.state === 'linked') {
    return <span className="text-muted-foreground">→ {owner.slack?.label}</span>
  }

  if (owner.state === 'not_on_slack') {
    return <span className="text-muted-foreground">· not on Slack</span>
  }

  if (dryRun) {
    return (
      <span className="text-muted-foreground">
        not linked — linking is held back under --dry-run
      </span>
    )
  }

  return (
    <>
      <label className="flex min-w-0 items-center">
        <span className="sr-only">
          {owner.kind === 'team' ? 'Slack group for' : 'Slack user for'} {owner.owner}
        </span>
        <Select
          size="sm"
          className="max-w-full"
          value=""
          disabled={busy || choices.length === 0}
          onChange={(event) => {
            const pickedFrom = owner.kind === 'user' && channel !== '' ? { channel } : {}
            onLink({
              owner: owner.owner,
              slack_id: event.target.value,
              not_on_slack: false,
              ...pickedFrom,
            })
          }}
        >
          <option value="">Not linked yet</option>
          {choices.map((choice) => (
            <option key={choice.id} value={choice.id}>
              {choice.label}
            </option>
          ))}
        </Select>
      </label>
      <Button
        variant="secondary"
        disabled={busy}
        aria-label={`${owner.owner} is not on Slack`}
        onClick={() => {
          onLink({ owner: owner.owner, not_on_slack: true })
        }}
      >
        Not on Slack
      </Button>
    </>
  )
}

interface GroupChecksProps {
  groups: GroupTag[]
  checked: string[]
  disabled: boolean
  onCheck: (id: string, checked: boolean) => void
}

// GroupChecks are the user groups the announcement offers, each a checkbox,
// noting the ones a team owning the changed paths brings.
function GroupChecks({ groups, checked, disabled, onCheck }: GroupChecksProps) {
  return (
    <fieldset className="flex flex-col gap-tight text-sm">
      <legend className="mb-1 text-sm font-semibold">Tag groups</legend>
      {groups.map((group) => (
        <label key={group.slack.id} className="flex flex-wrap items-center gap-item">
          <input
            type="checkbox"
            checked={checked.includes(group.slack.id)}
            disabled={disabled}
            onChange={(event) => {
              onCheck(group.slack.id, event.target.checked)
            }}
          />
          <span>@{group.slack.label}</span>
          {group.from_owners ? (
            <span className="text-muted-foreground">owns changed paths</span>
          ) : null}
        </label>
      ))}
    </fieldset>
  )
}
