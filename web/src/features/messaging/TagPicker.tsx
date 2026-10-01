import { useId, useState } from 'react'
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
import { OutcomeLine, type Teller, useOutcome } from '@/lib/Outcome.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { savePerson, useSlackGroups, useSlackMembers } from './slackApi.ts'

// selectStyle is how a picker's native select is drawn, as the channel's is.
const selectStyle =
  'min-w-0 max-w-full rounded-md border border-input bg-transparent px-2 py-1 text-sm disabled:cursor-not-allowed disabled:bg-disabled disabled:text-disabled-foreground'

interface TagPickerProps {
  tagging: AnnouncementTagging
  channel: string
  posting: boolean
  checked: string[]
  onChecked: (ids: string[]) => void
}

// TagPicker is whom a ready-for-review announcement tags: the code owners of
// the branch's changes — an owner not yet linked is linked here, saved for
// next time, or marked not on Slack — and the user groups it offers, as
// checkboxes. The summary under them says who the post will tag.
export function TagPicker({ tagging, channel, posting, checked, onChecked }: TagPickerProps) {
  const outcome = useOutcome()
  const { owners, groups, link } = useOwnerLinks(tagging, outcome, () => checked, onChecked)
  const choices = useLinkChoices(owners, channel)
  const scope = tagging.missing_scope ?? choices.missingScope

  return (
    <div className="flex flex-col gap-group">
      {scope === undefined ? null : <MissingScope scope={scope} />}
      {owners.length > 0 ? (
        <OwnerList
          owners={owners}
          members={choices.members}
          groups={choices.groups}
          busy={posting || link.state === 'running'}
          onLink={(asked) => void link.run(asked)}
        />
      ) : null}
      {link.state === 'error' ? (
        <p role="alert" className="text-sm text-destructive">
          {link.error}
        </p>
      ) : null}
      <OutcomeLine said={outcome.said} />
      {groups.length > 0 ? (
        <GroupChecks groups={groups} checked={checked} disabled={posting} onChecked={onChecked} />
      ) : null}
      <p className="text-sm break-words">
        <span className="text-muted-foreground">Tags: </span>
        {tagsSummary(owners, groups, checked)}
      </p>
    </div>
  )
}

// useOwnerLinks are the owners and the groups offered as the preview links
// them: a link saved here updates its owner, and a team linked brings its
// group, checked, as the server would offer it.
function useOwnerLinks(
  tagging: AnnouncementTagging,
  tell: Teller,
  checked: () => string[],
  onChecked: (ids: string[]) => void,
) {
  const [owners, setOwners] = useState(tagging.owners)
  const [groups, setGroups] = useState(tagging.groups)
  const link = useAsyncAction((asked: PersonLink) => savePerson(asked), {
    fallback: 'That was not saved. Try again, or link them in Settings.',
    done: (answered, asked) => `Saved for next time: ${savedAs(asked, answered)}.`,
    onStart: tell.clear,
    onDone: (said, answered) => {
      tell.say(said)
      const linked = owners.map(
        (owner) => answered.owners.find((decided) => decided.owner === owner.owner) ?? owner,
      )
      const added = teamGroups(linked).filter(
        (slack) => !groups.some((group) => group.slack.id === slack.id),
      )
      setOwners(linked)
      setGroups([...groups, ...added.map((slack) => ({ slack, checked: true, from_owners: true }))])
      if (added.length > 0) {
        onChecked([...checked(), ...added.map((slack) => slack.id)])
      }
    },
  })

  return { owners, groups, link }
}

// useLinkChoices reads whom an owner not yet linked could be: the channel's
// members for a person, the user groups for a team, each read only while such
// an owner waits; and a scope the token lacks for either read.
function useLinkChoices(owners: OwnerTag[], channel: string) {
  const unlinked = owners.filter((owner) => owner.state === 'unlinked')
  const members = useSlackMembers(
    channel,
    unlinked.some((owner) => owner.kind === 'user'),
  )
  const groups = useSlackGroups(unlinked.some((owner) => owner.kind === 'team'))

  return {
    members: members.data?.entries ?? [],
    groups: groups.data?.entries ?? [],
    missingScope: members.data?.missing_scope ?? groups.data?.missing_scope,
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

// teamGroups are the user groups linked teams among owners stand for.
function teamGroups(owners: OwnerTag[]): SlackTarget[] {
  return owners.flatMap((owner) =>
    owner.kind === 'team' && owner.state === 'linked' && owner.slack ? [owner.slack] : [],
  )
}

// tagsSummary names everyone the post tags: the linked people, then the
// groups checked.
function tagsSummary(owners: OwnerTag[], groups: GroupTag[], checked: string[]): string {
  const people = owners.flatMap((owner) =>
    owner.kind === 'user' && owner.state === 'linked' && owner.slack ? [owner.slack.label] : [],
  )
  const chosen = groups.filter((group) => checked.includes(group.slack.id))
  const names = [...people, ...chosen.map((group) => group.slack.label)].map((name) => `@${name}`)

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
  busy: boolean
  onLink: (link: PersonLink) => void
}

// OwnerList is the code owners, each with whom they are on Slack, or the
// controls that decide it.
function OwnerList({ owners, members, groups, busy, onLink }: OwnerListProps) {
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
  busy: boolean
  onLink: (link: PersonLink) => void
}

// OwnerOnSlack is whom an owner is on Slack; for one not linked yet, a choice
// of the channel's members — a team's, of the user groups — saved as it is
// made, and a button for one not on Slack. Under --dry-run nothing is saved,
// so it says so instead.
function OwnerOnSlack({ owner, choices, busy, onLink }: OwnerOnSlackProps) {
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
        <select
          value=""
          disabled={busy || choices.length === 0}
          onChange={(event) => {
            onLink({ owner: owner.owner, slack_id: event.target.value, not_on_slack: false })
          }}
          className={selectStyle}
        >
          <option value="">Not linked yet</option>
          {choices.map((choice) => (
            <option key={choice.id} value={choice.id}>
              {choice.label}
            </option>
          ))}
        </select>
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
  onChecked: (ids: string[]) => void
}

// GroupChecks are the user groups the announcement offers, each a checkbox,
// noting the ones a team owning the changed paths brings.
function GroupChecks({ groups, checked, disabled, onChecked }: GroupChecksProps) {
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
              onChecked(
                event.target.checked
                  ? [...checked, group.slack.id]
                  : checked.filter((id) => id !== group.slack.id),
              )
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
