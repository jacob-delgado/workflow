import { useRef, useState } from 'react'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Dialog, DialogClose, DialogTitle } from './Dialog.tsx'

// Opener is a page with one button that opens a dialog titled Keys, whose
// second button takes the focus as it opens.
function Opener() {
  const [open, setOpen] = useState(false)
  const takesFocus = useRef<HTMLButtonElement>(null)

  return (
    <>
      <button
        type="button"
        onClick={() => {
          setOpen(true)
        }}
      >
        Open
      </button>
      {open ? (
        <Dialog
          initialFocus={takesFocus}
          onClose={() => {
            setOpen(false)
          }}
        >
          <DialogTitle>Keys</DialogTitle>
          <button type="button">First</button>
          <button ref={takesFocus} type="button">
            Second
          </button>
          <DialogClose>Close</DialogClose>
        </Dialog>
      ) : null}
    </>
  )
}

test('a dialog is named by its title and gives the focus to the control it names', async () => {
  // Arrange
  const user = userEvent.setup()
  render(<Opener />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Open' }))

  // Assert
  const dialog = await screen.findByRole('dialog', { name: 'Keys' })
  await waitFor(() => {
    expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Second' }))
  })
  expect(dialog.contains(document.activeElement)).toBe(true)
})

test('Escape closes a dialog and hands the focus back to what opened it', async () => {
  // Arrange
  const user = userEvent.setup()
  render(<Opener />)
  await user.click(screen.getByRole('button', { name: 'Open' }))
  await screen.findByRole('dialog', { name: 'Keys' })

  // Act
  await user.keyboard('{Escape}')

  // Assert
  await waitFor(() => {
    expect(screen.queryByRole('dialog')).toBeNull()
  })
  await waitFor(() => {
    expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Open' }))
  })
})

test('its close control closes a dialog', async () => {
  // Arrange
  const user = userEvent.setup()
  render(<Opener />)
  await user.click(screen.getByRole('button', { name: 'Open' }))

  // Act
  await user.click(await screen.findByRole('button', { name: 'Close' }))

  // Assert
  await waitFor(() => {
    expect(screen.queryByRole('dialog')).toBeNull()
  })
})
