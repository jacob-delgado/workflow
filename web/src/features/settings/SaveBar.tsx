import { Button } from '@/lib/Button.tsx'
import { OutcomeLine } from '@/lib/Outcome.tsx'
import { Failure } from '@/lib/Status.tsx'
import type { AsyncState } from '@/lib/useAsyncAction.ts'

// changedSettings is how many settings the form's dirty fields mark changed:
// each field, and each list as one, however many of its rows changed.
export function changedSettings(dirty: unknown): number {
  if (dirty === true) {
    return 1
  }

  if (Array.isArray(dirty)) {
    return dirty.some((row) => changedSettings(row) > 0) ? 1 : 0
  }

  if (typeof dirty === 'object' && dirty !== null) {
    return Object.values(dirty).reduce<number>((count, field) => count + changedSettings(field), 0)
  }

  return 0
}

// standing says where the form stands: saving, how many settings are changed
// and not saved, or, with none, that there are none — unless a save has just
// said it saved, which says so in its place.
function standing({ saving, unsaved, said }: Pick<SaveBarProps, 'saving' | 'unsaved' | 'said'>) {
  if (saving) {
    return 'Saving…'
  }

  if (unsaved === 0) {
    return said === null ? 'No unsaved changes' : ''
  }

  return unsaved === 1 ? '1 unsaved change' : `${String(unsaved)} unsaved changes`
}

// ReloadAction is the one-shot read Reload runs.
interface ReloadAction {
  state: AsyncState
  error: string
  run: () => Promise<void>
}

interface SaveBarProps {
  // form is the id of the form Save sends.
  form: string
  unsaved: number
  saving: boolean
  said: Parameters<typeof OutcomeLine>[0]['said']
  // error is why the last save failed, or empty.
  error: string
  // reload is offered when the file changed since the form read it.
  reload: ReloadAction | null
  onDiscard: () => void
}

// SaveBar stays in view below whichever section is shown: how many settings
// are changed and not saved, Discard and Save — both held while nothing is
// changed, or while a save goes — and what the last save said: that it saved,
// or why it did not. A save refused because the file changed says so, and
// offers Reload, in place of its own reason.
export function SaveBar({ form, unsaved, saving, said, error, reload, onDiscard }: SaveBarProps) {
  const held = saving || unsaved === 0

  return (
    <div
      role="region"
      aria-label="Save"
      data-save-bar
      className="sticky bottom-0 flex flex-col items-start gap-item border-t border-border bg-background py-3"
    >
      <div className="flex w-full flex-wrap items-center gap-item">
        <p className="mr-auto text-sm text-muted-foreground">
          {standing({ saving, unsaved, said })}
        </p>
        <OutcomeLine said={said} />
        <Button variant="secondary" held={held} onClick={onDiscard}>
          Discard
        </Button>
        <Button variant="primary" type="submit" form={form} held={held}>
          {saving ? 'Saving…' : 'Save changes'}
        </Button>
      </div>
      {reload === null ? null : <ChangedSinceRead reload={reload} />}
      {reload !== null || error === '' ? null : <Failure>{error}</Failure>}
    </div>
  )
}

// ChangedSinceRead says a save was refused because the file changed since the
// form read it, what Reload will do to the edits, and offers it.
function ChangedSinceRead({ reload }: { reload: ReloadAction }) {
  return (
    <>
      <Failure>
        The configuration file changed after Settings read it — edited on disk, or saved from
        another tab — so nothing was saved. Reload reads it again in place of your edits here; then
        make your change again.
      </Failure>
      <Button
        variant="secondary"
        held={reload.state === 'running'}
        onClick={() => {
          void reload.run()
        }}
      >
        {reload.state === 'running' ? 'Reloading…' : 'Reload'}
      </Button>
      {reload.state === 'error' ? <Failure>{reload.error}</Failure> : null}
    </>
  )
}
