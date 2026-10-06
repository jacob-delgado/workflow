import { useRef, useState } from 'react'
import { apiErrorMessage } from '@/api/apiError.ts'
import type {
  OwnerTag,
  People,
  PersonLink,
  SlackDirectory,
  SlackTarget,
} from '@/api/generated/types.gen.ts'
import { useSlackGroups, useSlackMembers } from '@/features/messaging/slackApi.ts'
import { Button } from '@/lib/Button.tsx'
import { Select } from '@/lib/Field.tsx'
import { Reading, Unread } from '@/lib/Status.tsx'
import { useFocusOnMount } from '@/lib/focus.ts'
import { OutcomeLine, type Teller, useOutcome } from '@/lib/Outcome.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { usePeople, usePeopleWrites } from './peopleApi.ts'
import { useHoldShortcuts } from '@/features/keyboard/useShortcut.ts'

// notOnSlack is the choice that marks an owner not on Slack; no Slack ID
// reads like it.
const notOnSlack = 'not-on-slack'

// PeopleTable is every code owner decided on this forge host, then the
// branch's undecided ones, each with whom they are on Slack — changed as it
// is chosen — and, once decided, a Forget behind a confirm step.
export function PeopleTable({ dryRun }: { dryRun: boolean }) {
  const query = usePeople()

  if (query.isPending) {
    return <Reading>Reading the code owners…</Reading>
  }

  if (query.isError) {
    return (
      <Unread
        reason={apiErrorMessage(query.error, 'The code owners could not be read.')}
        refusals={query.errorUpdateCount}
        retrying={query.isFetching}
        onRetry={() => {
          void query.refetch()
        }}
      />
    )
  }

  if (query.data.owners.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">
        No code owner is decided yet: a ready-for-review announcement asks about each.
      </p>
    )
  }

  return <OwnerRows people={query.data} dryRun={dryRun} />
}

// OwnerRows are the owners' table, its directory notes, what the last change
// said, and the Forget confirm step one row opened.
function OwnerRows({ people, dryRun }: { people: People; dryRun: boolean }) {
  const outcome = useOutcome()
  const writes = usePeopleWrites()
  const [confirming, setConfirming] = useState<string | null>(null)
  const openers = useRef(new Map<string, HTMLButtonElement>())
  const choices = useChoices(people.owners, !dryRun)
  const link = useAsyncAction(writes.link, {
    fallback: 'That was not saved. Try again.',
    done: (answered, asked) => `Saved: ${linkedAs(asked, answered)}.`,
    onStart: outcome.clear,
    onDone: outcome.say,
  })

  return (
    <div className="flex flex-col gap-item text-sm">
      {choices.note === '' ? null : (
        <p role="note" className="text-muted-foreground">
          {choices.note}
        </p>
      )}
      <table aria-label="Code owners on Slack" className="w-full table-fixed text-left">
        <thead className="text-xs text-muted-foreground">
          <tr>
            <th scope="col" className="w-2/5 py-1 font-medium">
              Owner
            </th>
            <th scope="col" className="py-1 font-medium">
              On Slack
            </th>
            <th scope="col" className="w-24 py-1 font-medium">
              <span className="sr-only">Forget</span>
            </th>
          </tr>
        </thead>
        <tbody>
          {people.owners.map((owner) => (
            <OwnerRow
              key={owner.owner}
              owner={owner}
              choices={owner.kind === 'team' ? choices.groups : choices.members}
              disabled={dryRun || link.state === 'running'}
              opener={(button) => {
                setOpener(openers.current, owner.owner, button)
              }}
              onLink={(asked) => void link.run(asked)}
              onForget={() => {
                setConfirming(owner.owner)
              }}
            />
          ))}
        </tbody>
      </table>
      {link.state === 'error' ? (
        <p role="alert" className="text-destructive">
          {link.error}
        </p>
      ) : null}
      {confirming === null ? null : (
        <ForgetConfirm
          owner={confirming}
          forget={writes.forget}
          tell={outcome}
          onClose={(forgotten) => {
            setConfirming(null)
            if (!forgotten) {
              openers.current.get(confirming)?.focus()
            }
          }}
        />
      )}
      <OutcomeLine said={outcome.said} />
    </div>
  )
}

// setOpener keeps the Forget button of owner's row, so a Cancel hands focus
// back to it.
function setOpener(
  openers: Map<string, HTMLButtonElement>,
  owner: string,
  button: HTMLButtonElement | null,
) {
  if (button === null) {
    openers.delete(owner)
  } else {
    openers.set(owner, button)
  }
}

// linkedAs words what a change saved for an owner.
function linkedAs(asked: PersonLink, answered: People): string {
  if (asked.not_on_slack) {
    return `${asked.owner} is not on Slack`
  }

  const label = answered.owners.find((owner) => owner.owner === asked.owner)?.slack?.label

  return `${asked.owner} is ${label ?? asked.slack_id ?? ''}`
}

// useChoices reads whom an owner can be: the configured channel's members
// for a person, the user groups for a team, each read only while such an
// owner is listed and changes can be saved; and a note when a read cannot
// offer them.
function useChoices(owners: OwnerTag[], enabled: boolean) {
  const kinds = new Set(owners.map((owner) => owner.kind))
  const members = readOf(useSlackMembers('', enabled && kinds.has('user')))
  const groups = readOf(useSlackGroups(enabled && kinds.has('team')))

  return {
    members: members.entries,
    groups: groups.entries,
    note: directoryNote(members.scope ?? groups.scope, members.error ?? groups.error),
  }
}

// readOf is a directory read's entries, the scope it lacks, and why it
// failed.
function readOf(read: { data?: SlackDirectory; error: Error | null }) {
  return {
    entries: read.data?.entries ?? [],
    scope: read.data?.missing_scope,
    error: read.error,
  }
}

// directoryNote says why Slack's people or groups are not offered, or
// nothing when they are.
function directoryNote(scope: string | undefined, failed: Error | null): string {
  if (scope !== undefined) {
    return `Choosing from Slack needs the ${scope} scope, which the Slack token lacks: add it to the Slack app, then run workflow slack login.`
  }

  return failed === null ? '' : apiErrorMessage(failed, 'Slack’s people could not be read.')
}

interface OwnerRowProps {
  owner: OwnerTag
  choices: SlackTarget[]
  disabled: boolean
  opener: (button: HTMLButtonElement | null) => void
  onLink: (link: PersonLink) => void
  onForget: () => void
}

// OwnerRow is one owner: their name, whom they are on Slack as a choice that
// saves as it changes, and Forget once they are decided.
function OwnerRow({ owner, choices, disabled, opener, onLink, onForget }: OwnerRowProps) {
  return (
    <tr className="border-t border-border align-middle">
      <td className="py-1 pr-2 font-mono break-all">
        {owner.owner}
        {owner.kind === 'team' ? (
          <span className="ml-1 font-sans text-xs text-muted-foreground">team</span>
        ) : null}
      </td>
      <td className="py-1 pr-2">
        <SlackChoice owner={owner} choices={choices} disabled={disabled} onLink={onLink} />
      </td>
      <td className="py-1">
        {owner.state === 'unlinked' ? null : (
          <Button
            variant="secondary"
            ref={opener}
            disabled={disabled}
            aria-label={`Forget ${owner.owner}…`}
            onClick={onForget}
          >
            Forget…
          </Button>
        )}
      </td>
    </tr>
  )
}

interface SlackChoiceProps {
  owner: OwnerTag
  choices: SlackTarget[]
  disabled: boolean
  onLink: (link: PersonLink) => void
}

// SlackChoice is whom an owner is on Slack, as a native select: not decided
// yet, not on Slack, or one of the choices — the one they are linked to among
// them even when Slack no longer lists it.
function SlackChoice({ owner, choices, disabled, onLink }: SlackChoiceProps) {
  const current = owner.state === 'linked' ? owner.slack : undefined
  const listed =
    current === undefined || choices.some((choice) => choice.id === current.id)
      ? choices
      : [current, ...choices]
  const value = { linked: current?.id ?? '', not_on_slack: notOnSlack, unlinked: '' }[owner.state]

  return (
    <label className="flex min-w-0">
      <span className="sr-only">Slack for {owner.owner}</span>
      <Select
        size="sm"
        className="w-full"
        value={value}
        disabled={disabled}
        onChange={(event) => {
          const chosen = event.target.value
          onLink(
            chosen === notOnSlack
              ? { owner: owner.owner, not_on_slack: true }
              : { owner: owner.owner, slack_id: chosen, not_on_slack: false },
          )
        }}
      >
        {owner.state === 'unlinked' ? <option value="">Not decided yet</option> : null}
        <option value={notOnSlack}>Not on Slack</option>
        {listed.map((choice) => (
          <option key={choice.id} value={choice.id}>
            {choice.label}
          </option>
        ))}
      </Select>
    </label>
  )
}

interface ForgetConfirmProps {
  owner: string
  forget: (owner: string) => Promise<People>
  tell: Teller
  onClose: (forgotten: boolean) => void
}

// ForgetConfirm asks before forgetting an owner, and takes focus as it opens,
// so a screen reader hears the question.
function ForgetConfirm({ owner, forget, tell, onClose }: ForgetConfirmProps) {
  const question = useFocusOnMount<HTMLDivElement>()
  useHoldShortcuts()
  const forgetting = useAsyncAction(forget, {
    fallback: `${owner} was not forgotten. Try again.`,
    done: () => `Forgot ${owner}: they are asked about again.`,
    onStart: tell.clear,
    onDone: (said) => {
      tell.say(said)
      onClose(true)
    },
  })

  return (
    <div
      ref={question}
      role="group"
      aria-label={`Forget ${owner}?`}
      tabIndex={-1}
      className="flex flex-col items-start gap-item"
    >
      <p>Forget {owner}?</p>
      <p className="text-muted-foreground">
        The next ready-for-review announcement asks whom they are on Slack again.
      </p>
      {forgetting.state === 'error' ? (
        <p role="alert" className="text-destructive">
          {forgetting.error}
        </p>
      ) : null}
      <div className="flex items-center gap-item">
        <Button
          variant="secondary"
          disabled={forgetting.state === 'running'}
          onClick={() => {
            onClose(false)
          }}
        >
          Cancel
        </Button>
        <Button
          variant="primary"
          disabled={forgetting.state === 'running'}
          onClick={() => void forgetting.run(owner)}
        >
          {forgetting.state === 'running' ? 'Forgetting…' : 'Forget'}
        </Button>
      </div>
    </div>
  )
}
