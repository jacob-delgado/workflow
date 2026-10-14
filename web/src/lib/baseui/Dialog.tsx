import { useEffect, type ComponentProps, type ReactNode, type RefObject } from 'react'
import { Dialog as Base } from '@base-ui/react/dialog'
import { create } from 'zustand'
import { cn } from '@/lib/utils.ts'

// How many dialogs are open now, each modal.
const useOpenDialogs = create<{ open: number }>(() => ({ open: 0 }))

// dialogOpen reports a dialog open now: the page behind it is inert, so no
// key the page binds may act there.
export function dialogOpen(): boolean {
  return useOpenDialogs.getState().open > 0
}

interface DialogProps {
  // label names the dialog outright; without it, its DialogTitle does.
  label?: string
  // initialFocus is the control focus goes to as the dialog opens, or false
  // for what the dialog opens on to take the focus itself.
  initialFocus: RefObject<HTMLElement | null> | false
  onClose: () => void
  className?: string
  children: ReactNode
}

// Dialog is a modal dialog, open while it is mounted: the page behind it is
// inert and dimmed, so Tab stays inside it and no key the page binds reaches
// it; Escape, or a press outside it, closes it; and focus goes back to where
// it was when it opened.
export function Dialog({ label, initialFocus, onClose, className, children }: DialogProps) {
  useEffect(() => {
    useOpenDialogs.setState(({ open }) => ({ open: open + 1 }))

    return () => {
      useOpenDialogs.setState(({ open }) => ({ open: open - 1 }))
    }
  }, [])

  return (
    <Base.Root
      open
      onOpenChange={(open) => {
        if (!open) {
          onClose()
        }
      }}
    >
      <Base.Portal>
        <Base.Backdrop className="fixed inset-0 bg-background/75" />
        <Base.Viewport className="fixed inset-0 flex items-center justify-center overflow-y-auto p-4">
          <Base.Popup
            aria-label={label}
            initialFocus={initialFocus}
            className={cn(
              'max-w-full rounded-lg border border-border bg-popover text-popover-foreground focus-visible:outline-none',
              className,
            )}
          >
            {children}
          </Base.Popup>
        </Base.Viewport>
      </Base.Portal>
    </Base.Root>
  )
}

// DialogTitle names the dialog it is drawn in: a heading, h2 unless told.
export function DialogTitle(props: ComponentProps<typeof Base.Title>) {
  return <Base.Title {...props} />
}

// DialogClose is a button that closes the dialog it is drawn in.
export function DialogClose(props: ComponentProps<typeof Base.Close>) {
  return <Base.Close {...props} />
}
