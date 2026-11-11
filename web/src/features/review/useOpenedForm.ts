import { useEffect, useRef, useState } from 'react'

// FormName is a write on the pull request, by the form that sends it.
export type FormName = 'edit' | 'merge' | 'finish' | 'rerun'

// Opened is the form open, or none.
type Opened = 'none' | FormName

// OpenedForm is which of the pull request's forms is open, and what moves
// it: open, from its offer; close, backed out of; done, its write said by
// the outcome line; and sent, a write sent from it. button hands up the
// button a form backed out of hands focus back to.
export interface OpenedForm {
  opened: Opened
  open: (form: FormName) => void
  close: () => void
  done: () => void
  sent: () => void
  button: (form: FormName) => (button: HTMLButtonElement | null) => void
}

// useOpenedForm keeps one form open at a time. A form whose offer has lapsed
// — CI running again under Merge, say — is put away, as the server would
// refuse what it sends, and does not come back unasked when the offer does;
// one a write was sent from stays until that write is done or the form is
// backed out of, so it can say why the write was refused. A form backed out
// of hands focus back to the button that opened it.
export function useOpenedForm(can: (form: FormName) => boolean): OpenedForm {
  const [opened, setOpened] = useState<Opened>('none')
  const [sending, setSending] = useState(false)
  const buttons = useRef<Partial<Record<FormName, HTMLButtonElement | null>>>({})
  const returnTo = useRef<Opened>('none')

  // Set while rendering, so the frame that takes the offer away is the one
  // drawn without its form.
  if (opened !== 'none' && !sending && !can(opened)) {
    setOpened('none')
  }

  useEffect(() => {
    if (opened === 'none' && returnTo.current !== 'none') {
      buttons.current[returnTo.current]?.focus()
      returnTo.current = 'none'
    }
  }, [opened])

  const shut = () => {
    setOpened('none')
    setSending(false)
  }

  return {
    opened,
    open: setOpened,
    close: () => {
      returnTo.current = opened
      shut()
    },
    done: shut,
    sent: () => {
      setSending(true)
    },
    button: (form) => (button) => {
      buttons.current[form] = button
    },
  }
}
