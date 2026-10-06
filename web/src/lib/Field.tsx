import type { ChangeEvent, ComponentProps, ReactNode } from 'react'
import { cn } from './utils.ts'

// How large a field is drawn: md in a form, and sm in the dense row of
// controls over a list — a search, a sort, a view — matching a button of the
// same size, so the two line up in one row.
const sizes = {
  md: 'px-3 py-1.5',
  sm: 'px-2 py-1',
}

type Size = keyof typeof sizes

// fieldClass is how every field is drawn, whatever its element: one
// background, one border, the placeholder in the muted color, the focus ring,
// and off by color as a button is.
function fieldClass(size: Size, className: string | undefined): string {
  return cn(
    'min-w-0 rounded-md border border-input bg-background text-sm text-foreground placeholder:text-muted-foreground',
    'focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none',
    'disabled:cursor-not-allowed disabled:bg-disabled disabled:text-disabled-foreground',
    'aria-disabled:cursor-not-allowed aria-disabled:bg-disabled aria-disabled:text-disabled-foreground',
    sizes[size],
    className,
  )
}

// Held is a field whose change is saving, or whose form is sending: off, by
// aria-disabled rather than the disabled attribute, so it keeps the focus it
// was changed with, and a change while held is not heard — a text field is
// read-only, and a choice stays as it stands.
interface Held<T extends Element> {
  held?: boolean
  onChange?: (event: ChangeEvent<T>) => void
}

// heldProps are the attributes a held field takes: off to assistive
// technology, and its change heard only while it is not held.
function heldProps<T extends Element>({ held = false, onChange }: Held<T>) {
  return {
    'aria-disabled': held || undefined,
    onChange: (event: ChangeEvent<T>) => {
      if (!held) {
        onChange?.(event)
      }
    },
  }
}

type InputProps = Omit<ComponentProps<'input'>, 'size'> & Held<HTMLInputElement> & { size?: Size }

// Input is a one-line text field, or a checkbox; className places it in its
// layout.
export function Input({ size = 'md', className, held, onChange, ...props }: InputProps) {
  return (
    <input
      className={fieldClass(size, className)}
      readOnly={held}
      {...heldProps({ held, onChange })}
      {...props}
    />
  )
}

type SelectProps = Omit<ComponentProps<'select'>, 'size'> &
  Held<HTMLSelectElement> & { size?: Size }

// Select is a choice of one from a list, drawn as a field.
export function Select({ size = 'md', className, held, onChange, ...props }: SelectProps) {
  return (
    <select className={fieldClass(size, className)} {...heldProps({ held, onChange })} {...props} />
  )
}

type TextAreaProps = ComponentProps<'textarea'> & Held<HTMLTextAreaElement> & { size?: Size }

// TextArea is a field of several lines.
export function TextArea({ size = 'md', className, held, onChange, ...props }: TextAreaProps) {
  return (
    <textarea
      className={fieldClass(size, className)}
      readOnly={held}
      {...heldProps({ held, onChange })}
      {...props}
    />
  )
}

// FieldFrame is a field that holds more than its text — a comment box under its
// toolbar — drawn as one field: the frame has the border, the background and
// the focus ring while anything in it has focus, and the text area in it draws
// none of its own. It is a container of controls, so its corners are the
// container's.
export function FieldFrame({ children }: { children: ReactNode }) {
  return (
    <div className="flex flex-col overflow-hidden rounded-lg border border-input bg-background focus-within:ring-2 focus-within:ring-ring">
      {children}
    </div>
  )
}
