import { useState } from 'react'
import { apiErrorMessage } from '@/api/apiError.ts'
import type { RepoGroups as Groups, SlackTarget } from '@/api/generated/types.gen.ts'
import { useSlackGroups } from '@/features/messaging/slackApi.ts'
import { Button } from '@/lib/Button.tsx'
import { OutcomeLine, useOutcome } from '@/lib/Outcome.tsx'
import { Reading, Unread } from '@/lib/Status.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { useRepoGroups, useSaveRepoGroups } from './peopleApi.ts'

// RepoGroups is the user groups this repository's announcements may tag:
// the workspace's groups as checkboxes, saved together.
export function RepoGroups({ dryRun }: { dryRun: boolean }) {
  const query = useRepoGroups()

  if (query.isPending) {
    return <Reading>Reading the repository’s groups…</Reading>
  }

  if (query.isError) {
    return (
      <Unread
        reason={apiErrorMessage(query.error, 'The repository’s groups could not be read.')}
        refusals={query.errorUpdateCount}
        retrying={query.isFetching}
        onRetry={() => {
          void query.refetch()
        }}
      />
    )
  }

  return <GroupChoice saved={query.data} dryRun={dryRun} />
}

// GroupChoice is the checkboxes, which start as saved, and the Save that
// keeps them.
function GroupChoice({ saved, dryRun }: { saved: Groups; dryRun: boolean }) {
  const [checked, setChecked] = useCheckedFrom(saved.groups)
  const directory = useSlackGroups(!dryRun)
  const outcome = useOutcome()
  const save = useAsyncAction(useSaveRepoGroups(), {
    fallback: 'The groups were not saved. Try again.',
    done: (answered) => `Saved the groups for ${answered.repository}.`,
    onStart: outcome.clear,
    onDone: outcome.say,
  })
  const offered = withSaved(directory.data?.entries ?? [], saved.groups)
  const scope = directory.data?.missing_scope

  return (
    <div className="flex flex-col items-start gap-item text-sm">
      <fieldset className="flex w-full flex-col gap-tight">
        <legend className="mb-1 font-semibold break-all">Groups for {saved.repository}</legend>
        {scope === undefined ? null : (
          <p role="note" className="text-muted-foreground">
            Choosing groups needs the {scope} scope, which the Slack token lacks: add it to the
            Slack app, then run workflow slack login.
          </p>
        )}
        {directory.isError ? (
          <p role="note" className="text-muted-foreground">
            {apiErrorMessage(directory.error, 'Slack’s user groups could not be read.')}
          </p>
        ) : null}
        {offered.length === 0 ? (
          <p className="text-muted-foreground">No user groups to choose from.</p>
        ) : null}
        {offered.map((group) => (
          <label key={group.id} className="flex items-center gap-item">
            <input
              type="checkbox"
              checked={checked.includes(group.id)}
              disabled={dryRun}
              onChange={(event) => {
                setChecked(
                  event.target.checked
                    ? [...checked, group.id]
                    : checked.filter((id) => id !== group.id),
                )
              }}
            />
            <span>@{group.label}</span>
          </label>
        ))}
      </fieldset>
      <Button
        variant="secondary"
        disabled={dryRun}
        held={save.state === 'running'}
        onClick={() => void save.run(checked)}
      >
        {save.state === 'running' ? 'Saving…' : 'Save groups'}
      </Button>
      {save.state === 'error' ? (
        <p role="alert" className="text-destructive">
          {save.error}
        </p>
      ) : null}
      <OutcomeLine said={outcome.said} />
    </div>
  )
}

// useCheckedFrom is the groups checked, which start as saved and start again
// whenever what is saved changes under them — a removal of everything, a save
// — so Save never writes back groups no longer kept.
function useCheckedFrom(saved: SlackTarget[]) {
  const savedIds = saved.map((group) => group.id)
  const [checked, setChecked] = useState(savedIds)
  const [startedFrom, setStartedFrom] = useState(savedIds.join(' '))
  if (startedFrom !== savedIds.join(' ')) {
    setStartedFrom(savedIds.join(' '))
    setChecked(savedIds)
  }

  return [checked, setChecked] as const
}

// withSaved is the directory's groups, then any saved one it no longer lists,
// so a saved group can still be unchecked.
function withSaved(directory: SlackTarget[], saved: SlackTarget[]): SlackTarget[] {
  return [
    ...directory,
    ...saved.filter((group) => !directory.some((listed) => listed.id === group.id)),
  ]
}
