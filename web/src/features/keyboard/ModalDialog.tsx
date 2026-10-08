import { useLayoutEffect, useRef, type ReactNode, type RefObject } from 'react'
import { cn } from '@/lib/utils.ts'

interface ModalDialogProps {
  // namedBy is the id of what names the dialog, or label names it outright.
  namedBy?: string
  label?: string
  // takesFocus is the control focus goes to as the dialog opens.
  takesFocus: RefObject<HTMLElement | null>
  onClose: () => void
  className?: string
  children: ReactNode
}

// ModalDialog is a native modal dialog, drawn while it is mounted: the page
// behind it is inert, so Tab stays inside it; Escape, or a press on the
// backdrop around it, closes it; and focus goes back to where it was when it
// opened. It opens and closes in layout effects, so focus is back as the
// commit that unmounts it ends: what closed it can send the focus elsewhere
// on purpose straight after.
export function ModalDialog({
  namedBy,
  label,
  takesFocus,
  onClose,
  className,
  children,
}: ModalDialogProps) {
  const dialog = useRef<HTMLDialogElement>(null)

  useLayoutEffect(() => {
    const opener = document.activeElement
    const shown = dialog.current
    shown?.showModal()
    takesFocus.current?.focus()

    return () => {
      shown?.close()
      if (opener instanceof HTMLElement) {
        opener.focus()
      }
    }
  }, [takesFocus])

  return (
    // eslint-disable-next-line jsx-a11y/click-events-have-key-events, jsx-a11y/no-noninteractive-element-interactions -- a press on the backdrop lands on the dialog itself, which closes it; Escape, through onCancel, is the keyboard's way out
    <dialog
      ref={dialog}
      aria-labelledby={namedBy}
      aria-label={label}
      onCancel={(event) => {
        event.preventDefault()
        onClose()
      }}
      onClose={onClose}
      onClick={(event) => {
        if (event.target === event.currentTarget) {
          onClose()
        }
      }}
      className={cn(
        'max-w-[calc(100%-2rem)] rounded-lg border border-border bg-popover p-0 text-popover-foreground backdrop:bg-background/75',
        className,
      )}
    >
      {children}
    </dialog>
  )
}
