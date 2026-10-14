import { type ReactNode, type SubmitEvent, useEffect, useId, useState } from 'react'
import {
  FormProvider,
  type Path,
  type UseFormReset,
  type UseFormReturn,
  type UseFormSetError,
  type UseFormSetFocus,
  useForm,
} from 'react-hook-form'
import { apiErrorMessage, problemCode } from '@/api/apiError.ts'
import type { Config, RefusedSetting, SetupResult } from '@/api/generated/types.gen.ts'
import { useFocusOnMount } from '@/lib/focus.ts'
import { resultGround, useOutcome } from '@/lib/Outcome.tsx'
import { Reading, Unread } from '@/lib/Status.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { cn, contentMeasure } from '@/lib/utils.ts'
import { sectionHeadingId } from '@/shell/sections.ts'
import {
  changedSinceRead,
  type ConfigRead,
  refusedSettings,
  useConfigRead,
  useReloadConfig,
  useSaveConfig,
} from './configApi.ts'
import { LocalData } from './people/LocalData.tsx'
import { PeopleAndGroups } from './people/PeopleAndGroups.tsx'
import { configOf, formValues, type SettingsValues } from './formValues.ts'
import { type Removal, RemovalsContext, stored, without } from './removal.ts'
import { firstSection, sectionOf, SettingsSections } from './SettingsSections.tsx'
import { type HeldRefusal, heldRefusals, refusedPaths } from './settingsRules.ts'
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
// it. A save refused before it is sent, or by the server, marks each setting
// refused beside itself; Show opens the first one's section and focuses it.
function ConfigForm({ read, takesFocus }: { read: ConfigRead; takesFocus: boolean }) {
  // The whole config seeds the form, so every key no fieldset registers — the
  // token commands, the Slack access token and its expiry — rides back
  // unchanged on save rather than being dropped.
  const form = useForm<SettingsValues>({
    defaultValues: formValues(read.config),
    shouldFocusError: false,
  })
  const { register, handleSubmit, reset, control, formState } = form
  const file = useFormFile(read, reset)
  const view = useSectionView(form.setFocus, takesFocus)
  const formId = useId()
  const outcome = useOutcome()
  const save = useSave(file, form.setError, outcome)
  // Reload swaps the edits for the file as it is now and clears the refusal,
  // taking the Reload button away, so the focus it had goes to the form.
  const reloadConfig = useReloadConfig()
  const reload = useAsyncAction(
    async () => {
      file.seed(await reloadConfig(), { keepEdits: false })
      file.setChanged(false)
      save.reset()
      view.show('jira.base_url')
    },
    { fallback: 'The configuration could not be read. Try Reload again.' },
  )
  const refused = refusedPaths(formState.errors)
  // Only the server refuses a list as a whole, and no rule here judges one
  // again: its refusal would hold back every save, so it stands only until
  // the next save the rules here let through, which the server judges afresh.
  // A save the rules hold back is never sent, so each setting the server
  // refused is marked again, though checking the rest took its mark away.
  const submitted = async () => {
    const standing = heldRefusals(formState.errors).filter(({ type }) => type === serverRefusal)
    if (await form.trigger()) {
      form.clearErrors(listRefusals)
    }
    await handleSubmit(
      (values) => save.run(values),
      (errors) => {
        const judged = heldBack(standing, errors, form)
        view.show((judged[0] ?? standing[0])?.path ?? 'jira.base_url')
      },
    )()
  }
  const onSubmit = (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault()
    void submitted()
  }

  return (
    <RemovalsContext value={file.removals}>
      <FormProvider {...form}>
        <SettingsSections
          section={view.section}
          onSection={view.setSection}
          fields={{
            register,
            control,
            storedToken: formState.defaultValues?.jira?.token ?? null,
          }}
          form={{
            id: formId,
            noValidate: true,
            'aria-labelledby': sectionHeadingId,
            onSubmit,
          }}
          bar={
            <SaveBar
              form={formId}
              unsaved={changedSettings(formState.dirtyFields)}
              toFix={toFix(refused)}
              saving={save.state === 'running'}
              said={outcome.said}
              error={save.state === 'error' && !save.marked ? save.error : ''}
              reload={file.changed ? reload : null}
              onShow={() => {
                view.show(refused[0] ?? 'jira.base_url')
              }}
              onDiscard={() => {
                reset()
                save.reset()
              }}
            />
          }
        />
      </FormProvider>
    </RemovalsContext>
  )
}

// heldBack marks what a save held back before it was sent leaves refused:
// each setting the server refused before, as the server judged it, then each
// the rules here refuse in errors, the order the form now holds them in. It
// returns the rules' own, which the save was held back for, so the focus goes
// to what was just refused rather than to a refusal already said.
function heldBack(
  standing: HeldRefusal[],
  errors: object,
  form: Pick<UseFormReturn<SettingsValues>, 'clearErrors' | 'setError'>,
): HeldRefusal[] {
  const judged = heldRefusals(errors).filter(
    ({ path }) => !standing.some((refusal) => refusal.path === path),
  )
  const held = [...standing, ...judged]
  form.clearErrors()
  for (const { path, type, message } of held) {
    form.setError(path as Path<SettingsValues>, { type, message })
  }

  return judged
}

// toFix is how many settings are refused, and the titles of the sections
// they are in, in the order they were refused.
function toFix(refused: string[]) {
  const sections = refused.map((path) => sectionOf(path)?.title ?? '')

  return { count: refused.length, sections: [...new Set(sections)].filter((title) => title !== '') }
}

// useSectionView is the section shown, and show, which opens the section a
// setting is in and gives that setting the focus once it is drawn: Jira's
// first field, at once, when the form takes the focus as it opens.
function useSectionView(setFocus: UseFormSetFocus<SettingsValues>, takesFocus: boolean) {
  const [section, setSection] = useState(firstSection)
  // The setting last asked to take the focus, and how many times one has been,
  // so asking for the same setting again focuses it again.
  const [focusing, setFocusing] = useState({ path: 'jira.base_url', asked: takesFocus ? 1 : 0 })

  useEffect(() => {
    if (focusing.asked > 0) {
      focusSetting(focusing.path, setFocus)
    }
  }, [focusing, setFocus])

  return {
    section,
    setSection,
    show: (path: string) => {
      setSection(sectionOf(path)?.id ?? firstSection)
      setFocusing(({ asked }) => ({ path, asked: asked + 1 }))
    },
  }
}

// focusSetting gives a setting the focus: a field itself, and a list refused
// as a whole its first control — its first row's first field, or, with no
// rows, its Add — since the form registers no field at a list's own path.
function focusSetting(path: string, setFocus: UseFormSetFocus<SettingsValues>) {
  const list = path.endsWith('.root') ? path.slice(0, -'.root'.length) : null
  if (list === null) {
    setFocus(path as Path<SettingsValues>)

    return
  }

  document.getElementById(list)?.querySelector<HTMLElement>('input, button')?.focus()
}

// useSave is Save: it writes the form's values over the file, and marks each
// setting a refusal names beside itself. marked says every setting the last
// refusal named was marked, so its reason need not be said again below the
// bar.
function useSave(
  file: ReturnType<typeof useFormFile>,
  setError: UseFormSetError<SettingsValues>,
  outcome: ReturnType<typeof useOutcome>,
) {
  const [marked, setMarked] = useState(false)
  const save = useAsyncAction(
    async (values: SettingsValues) => {
      setMarked(false)
      try {
        await file.write(configOf(values), { keepEdits: false })
      } catch (caught) {
        setMarked(markRefused(refusedSettings(caught), setError))
        throw caught
      }
    },
    {
      fallback: 'The configuration was not saved. Try again — your edits are still in the form.',
      done: () => 'Saved.',
      onStart: outcome.clear,
      onDone: outcome.say,
    },
  )

  return { ...save, marked }
}

// serverRefusal is the kind of refusal the form holds for a setting the
// server refused.
const serverRefusal = 'server'

// The lists the form edits a row at a time, which a refusal marks as a whole.
const lists = new Set(['jira.views', 'jira.headers', 'branch.prefixes', 'messaging.channels'])

// listRefusal is where the form holds a refusal of a list as a whole.
function listRefusal(list: string): Path<SettingsValues> {
  return `${list}.root` as Path<SettingsValues>
}

// listRefusals are where the form holds a refusal of each list.
const listRefusals = [...lists].map(listRefusal)

// markRefused marks each refused setting a section shows beside itself, and
// reports whether there were any and every one was marked. A setting is shown
// when it is a list, or a field whose control, which takes its path as its id,
// is drawn; one no section shows, such as the access token's expiry, is left
// to the refusal's own reason.
function markRefused(settings: RefusedSetting[], setError: UseFormSetError<SettingsValues>) {
  const shown = settings.filter(
    ({ key }) =>
      sectionOf(key) !== undefined && (lists.has(key) || document.getElementById(key) !== null),
  )
  for (const { key, detail } of shown) {
    const path = lists.has(key) ? listRefusal(key) : (key as Path<SettingsValues>)
    setError(path, { type: serverRefusal, message: detail })
  }

  return settings.length > 0 && shown.length === settings.length
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
