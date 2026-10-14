import { type ReactNode, useEffect, useId, useState } from 'react'
import { type UseFormReset, useForm } from 'react-hook-form'
import { apiErrorMessage, problemCode } from '@/api/apiError.ts'
import type { Config, SetupResult } from '@/api/generated/types.gen.ts'
import { useFocusOnMount } from '@/lib/focus.ts'
import { resultGround, useOutcome } from '@/lib/Outcome.tsx'
import { Reading, Unread } from '@/lib/Status.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { cn, contentMeasure } from '@/lib/utils.ts'
import { sectionHeadingId } from '@/shell/sections.ts'
import {
  changedSinceRead,
  type ConfigRead,
  useConfigRead,
  useReloadConfig,
  useSaveConfig,
} from './configApi.ts'
import { LocalData } from './people/LocalData.tsx'
import { PeopleAndGroups } from './people/PeopleAndGroups.tsx'
import { configOf, formValues, type SettingsValues } from './formValues.ts'
import { type Removal, RemovalsContext, stored, without } from './removal.ts'
import { firstSection, SettingsSections } from './SettingsSections.tsx'
import { changedSettings, SaveBar } from './SaveBar.tsx'
import { SetupArea } from './SetupForm.tsx'

// SettingsPanel is the configuration form a section at a time, with the areas
// that save on their own rather than with the file — people and groups, and
// the local data — as sections of their own. Until the file is read, it is
// why it is not, or the setup that writes a first one, above those two areas.
export function SettingsPanel() {
  return <ConfigArea />
}

// OwnSaves is why the configuration form is not shown, above the areas that
// save on their own, which do not wait for the file.
function OwnSaves({ children }: { children: ReactNode }) {
  return (
    <div className={cn('flex flex-col gap-section', contentMeasure)}>
      {children}
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
    return (
      <OwnSaves>
        <SetupArea onWritten={setSetUp} />
      </OwnSaves>
    )
  }

  // Settings opens on a fresh read rather than on the cached one: a form seeded
  // from a read the file has moved on from would only learn so on its save.
  if (query.isPending || (query.isFetching && !query.isFetchedAfterMount)) {
    return (
      <OwnSaves>
        <Reading>Reading the configuration…</Reading>
      </OwnSaves>
    )
  }

  if (query.isError) {
    return (
      <OwnSaves>
        <Unread
          reason={apiErrorMessage(query.error, 'The configuration could not be read.')}
          refusals={query.errorUpdateCount}
          retrying={query.isFetching}
          onRetry={() => {
            setRetried(true)
            void query.refetch()
          }}
        />
      </OwnSaves>
    )
  }

  return (
    <div className="flex flex-col gap-section">
      {setUp === null ? null : <SetUpSaid result={setUp} />}
      <ConfigForm read={query.data} takesFocus={retried} />
    </div>
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
    <p ref={said} role="status" tabIndex={-1} className={resultGround}>
      {parts.join(' ')}
    </p>
  )
}

// ConfigForm edits the configuration a section at a time. It takes focus, on
// Jira's first field, only when it replaces a control that had it — a Try
// again, or a Reload — and otherwise leaves focus where the section change put
// it.
function ConfigForm({ read, takesFocus }: { read: ConfigRead; takesFocus: boolean }) {
  // The whole config seeds the form, so every key no fieldset registers — the
  // token commands, the Slack access token and its expiry — rides back
  // unchanged on save rather than being dropped.
  const { register, handleSubmit, reset, setFocus, control, formState } = useForm<SettingsValues>({
    defaultValues: formValues(read.config),
  })
  const file = useFormFile(read, reset)
  // How many times the form's first field has been asked to take focus: once
  // when the form replaces a Try again, and after each Reload, which takes its own
  // button away. The field is focused once the form has drawn it.
  const [focusRequests, setFocusRequests] = useState(takesFocus ? 1 : 0)
  const [section, setSection] = useState(firstSection)
  const formId = useId()
  const reloadConfig = useReloadConfig()
  const outcome = useOutcome()
  const save = useAsyncAction(
    (values: SettingsValues) => file.write(configOf(values), { keepEdits: false }),
    {
      fallback: 'The configuration was not saved. Try again — your edits are still in the form.',
      done: () => 'Saved.',
      onStart: outcome.clear,
      onDone: outcome.say,
    },
  )
  // Reload swaps the edits for the file as it is now and clears the refusal,
  // taking the Reload button away, so the focus it had goes to the form.
  const reload = useAsyncAction(
    async () => {
      file.seed(await reloadConfig(), { keepEdits: false })
      file.setChanged(false)
      save.reset()
      setSection(firstSection)
      setFocusRequests((requests) => requests + 1)
    },
    { fallback: 'The configuration could not be read. Try Reload again.' },
  )
  const onSubmit = handleSubmit((values) => save.run(values))

  useEffect(() => {
    if (focusRequests > 0) {
      setFocus('jira.base_url')
    }
  }, [focusRequests, setFocus])

  return (
    <RemovalsContext value={file.removals}>
      <SettingsSections
        section={section}
        onSection={setSection}
        fields={{
          register,
          control,
          storedToken: formState.defaultValues?.jira?.token ?? null,
        }}
        form={{
          id: formId,
          'aria-labelledby': sectionHeadingId,
          onSubmit: (event) => {
            void onSubmit(event)
          },
        }}
        bar={
          <SaveBar
            form={formId}
            unsaved={changedSettings(formState.dirtyFields)}
            saving={save.state === 'running'}
            said={outcome.said}
            error={save.state === 'error' ? save.error : ''}
            reload={file.changed ? reload : null}
            onDiscard={() => {
              reset()
              save.reset()
            }}
          />
        }
      />
    </RemovalsContext>
  )
}

// Seeding is how a read or a write is taken up: in place of the form's edits,
// or under them, leaving them as they are.
interface Seeding {
  keepEdits: boolean
}

// useFormFile is the form's hold on the file it edits: the revision its
// values stand for — the read that seeded it, then each write and each reload,
// which a write names so the server refuses it over a change the form has not
// seen, unless that change lands between the server's check and its write —
// the configuration as last read or written, and whether the last write was
// refused because the file changed since: the refusal Reload, not another
// write, answers. A removal writes the file as read without the credential,
// and takes up what it wrote under the form's edits.
function useFormFile(read: ConfigRead, reset: UseFormReset<SettingsValues>) {
  const [revision, setRevision] = useState(read.revision)
  const [seeded, setSeeded] = useState(read.config)
  const [changed, setChanged] = useState(false)
  const saveConfig = useSaveConfig()
  const seed = (next: ConfigRead, { keepEdits }: Seeding) => {
    reset(formValues(next.config), { keepDirtyValues: keepEdits })
    setSeeded(next.config)
    setRevision(next.revision)
  }
  const write = async (config: Config, seeding: Seeding) => {
    try {
      seed(await saveConfig(config, revision), seeding)
      setChanged(false)
    } catch (caught) {
      setChanged(changedSinceRead(caught))
      throw caught
    }
  }

  return {
    changed,
    setChanged,
    seed,
    write,
    removals: {
      holds: (removal: Removal) => stored(seeded, removal),
      remove: (removal: Removal) => write(without(seeded, removal), { keepEdits: true }),
    },
  }
}
