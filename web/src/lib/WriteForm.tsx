import { useId, type ReactNode } from 'react'
import { Button } from './Button.tsx'
import { useFocusOnMount } from './focus.ts'
import { useHoldShortcuts } from './LastLook.tsx'
import { Failure } from './Status.tsx'

// LabeledInput is a field with its label above it and, given one, a hint
// below, the hint read as the field's description; and, while the field is
// refused, why, below the hint, for the field's error.
export function LabeledInput({
  label,
  hint,
  error,
  children,
}: {
  label: string
  hint?: string
  error?: string
  children: (id: string, hintId: string | undefined, errorId: string | undefined) => ReactNode
}) {
  const id = useId()
  const hintId = hint === undefined ? undefined : `${id}-hint`
  const errorId = error === undefined ? undefined : `${id}-error`

  return (
    <div className="flex flex-col gap-tight">
      <label htmlFor={id} className="text-sm font-medium">
        {label}
      </label>
      {children(id, hintId, errorId)}
      {hint === undefined ? null : (
        <p id={hintId} className="text-xs text-muted-foreground">
          {hint}
        </p>
      )}
      {error === undefined ? null : <Failure id={errorId}>{error}</Failure>}
    </div>
  )
}

interface WriteFormProps {
  label: string
  act: string
  busy: string | null
  error: string
  disabled?: boolean
  held?: boolean
  onSend: () => void
  onCancel: () => void
  children: ReactNode
}

// WriteForm is a form that is the last look at a write: what will be sent, a
// refusal beside it, and its send and Cancel. It takes focus as it opens, and
// holds the single-key shortcuts while it is drawn, as every last look does,
// so a key pressed in it never presses a control behind it or takes it away;
// nothing in it can be sent twice while a send is in flight. While it sends, its fields and
// buttons are held rather than disabled: each keeps the focus it had — the send
// pressed, or the field Enter was pressed in — and a field's change is stopped
// before its own handler hears it, so what is shown stays what was sent. Held,
// the send waits on something going elsewhere — a git run — and is held as
// while it sends, its fields and Cancel left free.
export function WriteForm({
  label,
  act,
  busy,
  error,
  disabled = false,
  held = false,
  onSend,
  onCancel,
  children,
}: WriteFormProps) {
  const shown = useFocusOnMount<HTMLFormElement>()
  useHoldShortcuts()
  const sending = busy !== null
  const sendHeld = sending || held

  return (
    <form
      ref={shown}
      aria-label={label}
      tabIndex={-1}
      onSubmit={(event) => {
        event.preventDefault()
        if (!sendHeld) {
          onSend()
        }
      }}
      className="flex flex-col gap-group rounded-lg border border-border p-4"
    >
      <fieldset
        aria-disabled={sending || undefined}
        onChangeCapture={(event) => {
          if (sending) {
            event.stopPropagation()
          }
        }}
        className="flex min-w-0 flex-col gap-group"
      >
        {children}
      </fieldset>
      {error === '' ? null : <Failure>{error}</Failure>}
      <div className="flex items-center gap-item">
        <Button variant="secondary" held={sending} onClick={onCancel}>
          Cancel
        </Button>
        <Button variant="primary" type="submit" held={sendHeld} disabled={disabled}>
          {busy ?? act}
        </Button>
      </div>
    </form>
  )
}
