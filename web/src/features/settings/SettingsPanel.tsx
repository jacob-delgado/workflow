import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { apiErrorMessage } from '@/api/apiError.ts'
import type { Config } from '@/api/generated/types.gen.ts'
import { type AsyncState, useAsyncAction } from '@/lib/useAsyncAction.ts'
import { EmptyState } from '@/shell/EmptyState.tsx'
import {
  changedSinceRead,
  type ConfigRead,
  useConfigRead,
  useReloadConfig,
  useSaveConfig,
} from './configApi.ts'
import { BranchFieldset } from './fieldsets/BranchFieldset.tsx'
import { CommitFieldset } from './fieldsets/CommitFieldset.tsx'
import { ForgeFieldset } from './fieldsets/ForgeFieldset.tsx'
import { JiraFieldset } from './fieldsets/JiraFieldset.tsx'
import { MessagingFieldset } from './fieldsets/MessagingFieldset.tsx'
import { PullRequestFieldset, StoreFieldset } from './fieldsets/PullRequestAndStoreFieldsets.tsx'

export function SettingsPanel() {
  const query = useConfigRead()
  // A Retry is swapped for the form it loads, so the form takes the focus the
  // Retry had rather than letting it fall to the page.
  const [retried, setRetried] = useState(false)

  // Settings opens on a fresh read rather than on the cached one: a form seeded
  // from a read the file has moved on from would only learn so on its save.
  if (query.isPending || (query.isFetching && !query.isFetchedAfterMount)) {
    return <EmptyState>Loading the configuration…</EmptyState>
  }

  if (query.isError) {
    return (
      <EmptyState>
        <span className="flex flex-col items-center gap-group">
          {apiErrorMessage(query.error, 'The configuration could not be loaded.')}
          <button
            type="button"
            disabled={query.isFetching}
            onClick={() => {
              setRetried(true)
              void query.refetch()
            }}
            className={secondaryButton}
          >
            {query.isFetching ? 'Retrying…' : 'Retry'}
          </button>
        </span>
      </EmptyState>
    )
  }

  return <ConfigForm read={query.data} takesFocus={retried} />
}

// secondaryButton is how a control beside the form's own Save is drawn.
const secondaryButton =
  'rounded-md border border-input px-3 py-1.5 text-foreground hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none disabled:cursor-not-allowed disabled:bg-disabled disabled:text-disabled-foreground'

// ConfigForm edits the configuration, one fieldset per section it can edit. It
// takes focus, on its first field, only when it replaces a control that had
// it — a Retry, or a Reload — and otherwise leaves focus where the section
// change put it.
function ConfigForm({ read, takesFocus }: { read: ConfigRead; takesFocus: boolean }) {
  // The whole config seeds the form, so the sections and collections this form
  // does not edit (ui, timing, headers, views…) ride back unchanged on save
  // rather than being dropped.
  const { register, handleSubmit, reset, setFocus } = useForm<Config>({
    defaultValues: read.config,
  })
  // The revision of the file the form's values stand for: the read that seeded
  // it, then each save and each reload. A save names it, so it never writes
  // over a change the form has not seen, even once the cached read has moved on.
  const [revision, setRevision] = useState(read.revision)
  // Whether the last save was refused because the file changed since the form
  // read it: the refusal that Reload, not another save, answers.
  const [changed, setChanged] = useState(false)
  // How many times the form's first field has been asked to take focus: once
  // when the form replaces a Retry, and after each Reload, which takes its own
  // button away. The field is focused once the form has drawn it.
  const [focusRequests, setFocusRequests] = useState(takesFocus ? 1 : 0)
  const saveConfig = useSaveConfig()
  const reloadConfig = useReloadConfig()
  const seed = (next: ConfigRead) => {
    reset(next.config)
    setRevision(next.revision)
  }
  const save = useAsyncAction(
    async (values: Config) => {
      try {
        seed(await saveConfig(values, revision))
        setChanged(false)
      } catch (caught) {
        setChanged(changedSinceRead(caught))
        throw caught
      }
    },
    { fallback: 'The configuration was not saved. Try again — your edits are still in the form.' },
  )
  // Reload swaps the edits for the file as it is now and clears the refusal,
  // taking the Reload button away, so the focus it had goes to the form.
  const reload = useAsyncAction(
    async () => {
      seed(await reloadConfig())
      setChanged(false)
      save.reset()
      setFocusRequests((requests) => requests + 1)
    },
    { fallback: 'The configuration could not be read again. Try Reload again.' },
  )
  const onSubmit = handleSubmit((values) => save.run(values))

  useEffect(() => {
    if (focusRequests > 0) {
      setFocus('jira.base_url')
    }
  }, [focusRequests, setFocus])

  return (
    <form
      onSubmit={(event) => {
        void onSubmit(event)
      }}
      className="flex max-w-2xl flex-col gap-section"
    >
      <JiraFieldset register={register} />
      <MessagingFieldset register={register} />
      <ForgeFieldset register={register} />
      <CommitFieldset register={register} />
      <BranchFieldset register={register} />
      <PullRequestFieldset register={register} />
      <StoreFieldset register={register} />

      {/* A refusal ChangedSinceRead explains is not said a second time. */}
      <SaveControls state={save.state} error={changed ? '' : save.error} />
      {changed ? <ChangedSinceRead reload={reload} /> : null}
    </form>
  )
}

// SaveControls is the Save button and what the last save said: that it saved,
// or why it did not — unless the file changed since the form read it, which
// ChangedSinceRead says instead.
function SaveControls({ state, error }: { state: AsyncState; error: string }) {
  return (
    <div className="flex items-center gap-item">
      <button
        type="submit"
        disabled={state === 'running'}
        className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none disabled:cursor-not-allowed disabled:bg-disabled disabled:text-disabled-foreground"
      >
        {state === 'running' ? 'Saving…' : 'Save changes'}
      </button>
      <span role="status" className="text-sm">
        {state === 'done' ? <span className="text-success">Saved.</span> : null}
        {state === 'error' ? <span className="text-destructive">{error}</span> : null}
      </span>
    </div>
  )
}

// ReloadAction is the one-shot read Reload runs.
interface ReloadAction {
  state: AsyncState
  error: string
  run: () => Promise<void>
}

// ChangedSinceRead says a save was refused because the file changed since the
// form read it, what Reload will do to the edits, and offers it.
function ChangedSinceRead({ reload }: { reload: ReloadAction }) {
  return (
    <div className="flex flex-col items-start gap-item">
      <p role="alert" className="text-sm text-destructive">
        The configuration file changed after Settings read it — edited on disk, or saved from
        another tab — so nothing was saved. Reload reads it again in place of your edits here; then
        make your change again.
      </p>
      <button
        type="button"
        disabled={reload.state === 'running'}
        onClick={() => {
          void reload.run()
        }}
        className={secondaryButton}
      >
        {reload.state === 'running' ? 'Reloading…' : 'Reload'}
      </button>
      {reload.state === 'error' ? (
        <p role="alert" className="text-sm text-destructive">
          {reload.error}
        </p>
      ) : null}
    </div>
  )
}
