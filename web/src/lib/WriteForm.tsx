import { useId, type ReactNode } from 'react'
import { Button } from './Button.tsx'
import { useFocusOnMount } from './focus.ts'
import { Failure } from './Status.tsx'

// LabeledInput is a field with its label above it and, given one, a hint
// below, the hint read as the field's description.
export function LabeledInput({
  label,
  hint,
  children,
}: {
  label: string
  hint?: string
  children: (id: string, hintId: string | undefined) => ReactNode
}) {
  const id = useId()
  const hintId = hint === undefined ? undefined : `${id}-hint`

  return (
    <div className="flex flex-col gap-tight">
      <label htmlFor={id} className="text-sm font-medium">
        {label}
      </label>
      {children(id, hintId)}
      {hint === undefined ? null : (
        <p id={hintId} className="text-xs text-muted-foreground">
          {hint}
        </p>
      )}
    </div>
  )
}

interface WriteFormProps {
  label: string
  act: string
  busy: string | null
  error: string
  disabled?: boolean
  onSend: () => void
  onCancel: () => void
  children: ReactNode
}

// WriteForm is a form that is the last look at a write: what will be sent, a
// refusal beside it, and its send and Cancel. It takes focus as it opens, and
// nothing in it can be sent twice while a send is in flight. While it sends, its fields and
// buttons are held rather than disabled: each keeps the focus it had — the send
// pressed, or the field Enter was pressed in — and a field's change is stopped
// before its own handler hears it, so what is shown stays what was sent.
export function WriteForm({
  label,
  act,
  busy,
  error,
  disabled = false,
  onSend,
  onCancel,
  children,
}: WriteFormProps) {
  const shown = useFocusOnMount<HTMLFormElement>()
  const sending = busy !== null
  return (
    <form
      ref={shown}
      aria-label={label}
      tabIndex={-1}
      onSubmit={(event) => {
        event.preventDefault()
        if (!sending) {
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
        <Button variant="primary" type="submit" held={sending} disabled={disabled}>
          {busy ?? act}
        </Button>
      </div>
    </form>
  )
}
