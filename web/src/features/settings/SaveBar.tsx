import type { ReactNode } from 'react'
import { Button } from '@/lib/Button.tsx'
import { OutcomeLine } from '@/lib/Outcome.tsx'
import { Failure } from '@/lib/Status.tsx'
import { sectionOfKey } from './SettingsSections.tsx'

interface SaveBarProps {
  // unsaved is how many settings differ from the file as last read or saved,
  // and refused the dotted key of each the page's rules or the server refused.
  unsaved: number
  refused: string[]
  saving: boolean
  said: Parameters<typeof OutcomeLine>[0]['said']
  // error is why the last save failed, unless what follows says it.
  error: string
  onDiscard: () => void
  // onShow opens the section of the first setting refused, focused on it.
  onShow: () => void
  // children follow what the bar says: what a refusal asks of you.
  children?: ReactNode
}

// SaveBar is the configuration's save, kept at the foot of the window
// whichever section is shown: how many changes are not saved yet and how many
// settings are to fix, with Show, which goes to the first; Discard, which puts
// every section back as last read; and Save, held while there is nothing to
// save or a save runs.
export function SaveBar({
  unsaved,
  refused,
  saving,
  said,
  error,
  onDiscard,
  onShow,
  children,
}: SaveBarProps) {
  return (
    <section
      aria-label="Save"
      data-save-bar
      className="sticky bottom-0 z-10 -mx-block flex flex-col gap-item border-t border-border bg-background px-block py-3"
    >
      <div className="flex flex-wrap items-center gap-item">
        <p className="mr-auto text-sm text-muted-foreground tabular-nums">
          {[unsaved === 0 ? 'No unsaved changes' : changesWords(unsaved), fixWords(refused)]
            .filter((words) => words !== '')
            .join(', ')}
        </p>
        <OutcomeLine said={said} />
        {refused.length === 0 ? null : (
          <Button variant="secondary" onClick={onShow}>
            Show
          </Button>
        )}
        {unsaved === 0 ? null : (
          <Button variant="secondary" held={saving} onClick={onDiscard}>
            Discard
          </Button>
        )}
        <Button variant="primary" type="submit" held={saving || unsaved === 0}>
          {saving ? 'Saving…' : 'Save changes'}
        </Button>
      </div>
      {error === '' ? null : <Failure>{error}</Failure>}
      {children}
    </section>
  )
}

// changesWords counts unsaved changes.
function changesWords(count: number): string {
  return `${String(count)} unsaved ${count === 1 ? 'change' : 'changes'}`
}

// fixWords says how many settings are to fix, and in which section the first
// is, or nothing while none is.
function fixWords(refused: string[]): string {
  const [first] = refused
  if (first === undefined) {
    return ''
  }

  const section = sectionOfKey(first)

  return `${String(refused.length)} to fix${section === undefined ? '' : ` in ${section.name}`}`
}
