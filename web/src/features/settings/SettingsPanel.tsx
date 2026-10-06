import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { apiErrorMessage, problemCode } from '@/api/apiError.ts'
import type { Config, SetupResult } from '@/api/generated/types.gen.ts'
import { Button } from '@/lib/Button.tsx'
import { useFocusOnMount } from '@/lib/focus.ts'
import { Reading } from '@/lib/Status.tsx'
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
import { KeyboardFieldset } from './fieldsets/KeyboardFieldset.tsx'
import { MessagingFieldset } from './fieldsets/MessagingFieldset.tsx'
import { PullRequestFieldset, StoreFieldset } from './fieldsets/PullRequestAndStoreFieldsets.tsx'
import { TaskwarriorFieldset } from './fieldsets/TaskwarriorFieldset.tsx'
import { LocalData } from './people/LocalData.tsx'
import { PeopleAndGroups } from './people/PeopleAndGroups.tsx'
import { SetupArea } from './SetupForm.tsx'

// SettingsPanel is the configuration form, and below it the areas that save on
// their own rather than with the file: people and groups, and the local data.
export function SettingsPanel() {
  return (
    <div className="flex flex-col gap-section">
      <ConfigArea />
      <PeopleAndGroups />
      <LocalData />
    </div>
  )
}

// ConfigArea is the configuration form, once the file is read, or why it is
// not — or, where no file applies, the setup that writes a first one, and once
// it has, what it wrote above the form.
function ConfigArea() {
  const query = useConfigRead()
  // A Try again is swapped for the form it loads, so the form takes the focus the
  // Try again had rather than letting it fall to the page.
  const [retried, setRetried] = useState(false)
  const [setUp, setSetUp] = useState<SetupResult | null>(null)

  if (query.isError && problemCode(query.error) === 'not_found') {
    return <SetupArea onWritten={setSetUp} />
  }

  // Settings opens on a fresh read rather than on the cached one: a form seeded
  // from a read the file has moved on from would only learn so on its save.
  if (query.isPending || (query.isFetching && !query.isFetchedAfterMount)) {
    return <Reading>Reading the configuration…</Reading>
  }

  if (query.isError) {
    return (
      <EmptyState>
        <span className="flex flex-col items-center gap-group">
          {apiErrorMessage(query.error, 'The configuration could not be loaded.')}
          <Button
            variant="secondary"
            disabled={query.isFetching}
            onClick={() => {
              setRetried(true)
              void query.refetch()
            }}
          >
            {query.isFetching ? 'Trying again…' : 'Try again'}
          </Button>
        </span>
      </EmptyState>
    )
  }

  return (
    <>
      {setUp === null ? null : <SetUpSaid result={setUp} />}
      <ConfigForm read={query.data} takesFocus={retried} />
    </>
  )
}

// SetUpSaid says what a setup wrote, and what is left to do about it. It takes
// the focus the setup's write had, which went with the setup.
function SetUpSaid({ result }: { result: SetupResult }) {
  const said = useFocusOnMount<HTMLParagraphElement>()
  const parts = [`Wrote ${result.shown}.`]
  if (result.jira_user !== '') {
    parts.push(`Jira knows the token as ${result.jira_user}.`)
  }
  if (result.keychain) {
    parts.push('Your keychain keeps the token.')
  }
  if (result.not_ignored) {
    parts.push('git does not ignore it, and it holds credentials: add it to .gitignore.')
  }
  if (!result.reopened) {
    parts.push('Restart workflow --web to work with it.')
  }

  return (
    <p ref={said} role="status" tabIndex={-1} className="max-w-2xl text-sm text-success">
      {parts.join(' ')}
    </p>
  )
}

// ConfigForm edits the configuration, one fieldset per section it can edit. It
// takes focus, on its first field, only when it replaces a control that had
// it — a Try again, or a Reload — and otherwise leaves focus where the section
// change put it.
function ConfigForm({ read, takesFocus }: { read: ConfigRead; takesFocus: boolean }) {
  // The whole config seeds the form, so every key no fieldset registers — ui,
  // timing, jira.views and the token commands among them — rides back unchanged
  // on save rather than being dropped.
  const { register, handleSubmit, reset, setFocus } = useForm<Config>({
    defaultValues: read.config,
  })
  // The revision of the file the form's values stand for: the read that seeded
  // it, then each save and each reload. A save names it, so the server refuses
  // it over a change the form has not seen, even once the cached read has moved
  // on — unless that change lands between the server's check and its write.
  const [revision, setRevision] = useState(read.revision)
  // Whether the last save was refused because the file changed since the form
  // read it: the refusal that Reload, not another save, answers.
  const [changed, setChanged] = useState(false)
  // How many times the form's first field has been asked to take focus: once
  // when the form replaces a Try again, and after each Reload, which takes its own
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
      <TaskwarriorFieldset register={register} />
      <KeyboardFieldset register={register} />

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
      <Button variant="primary" type="submit" disabled={state === 'running'}>
        {state === 'running' ? 'Saving…' : 'Save changes'}
      </Button>
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
      <Button
        variant="secondary"
        disabled={reload.state === 'running'}
        onClick={() => {
          void reload.run()
        }}
      >
        {reload.state === 'running' ? 'Reloading…' : 'Reload'}
      </Button>
      {reload.state === 'error' ? (
        <p role="alert" className="text-sm text-destructive">
          {reload.error}
        </p>
      ) : null}
    </div>
  )
}
