import { fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { vi } from 'vitest'
import { Button } from './Button.tsx'

test('a button in a form does not send the form unless it submits', () => {
  // Arrange
  const sent = vi.fn()
  render(
    <form
      aria-label="Message"
      onSubmit={(event) => {
        event.preventDefault()
        sent()
      }}
    >
      <Button variant="secondary">Cancel</Button>
    </form>,
  )

  // Act
  fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))

  // Assert
  expect(sent).not.toHaveBeenCalled()
})

test('a submitting button sends its form', () => {
  // Arrange
  const sent = vi.fn()
  render(
    <form
      aria-label="Message"
      onSubmit={(event) => {
        event.preventDefault()
        sent()
      }}
    >
      <Button variant="primary" type="submit">
        Send
      </Button>
    </form>,
  )

  // Act
  fireEvent.click(screen.getByRole('button', { name: 'Send' }))

  // Assert
  expect(sent).toHaveBeenCalledOnce()
})

test('a disabled button is off to assistive technology', () => {
  // Act
  render(
    <Button variant="primary" disabled>
      Send
    </Button>,
  )

  // Assert
  expect(screen.getByRole('button', { name: 'Send' }).hasAttribute('disabled')).toBe(true)
})

function HeldWhilePressed({ onPress }: { onPress: () => void }) {
  const [running, setRunning] = useState(false)

  return (
    <Button
      variant="primary"
      held={running}
      onClick={() => {
        onPress()
        setRunning(true)
      }}
    >
      Send
    </Button>
  )
}

test('a button held while its run goes keeps the focus it was pressed with', async () => {
  // Arrange
  const user = userEvent.setup()
  render(<HeldWhilePressed onPress={vi.fn()} />)
  const send = screen.getByRole('button', { name: 'Send' })

  // Act
  await user.click(send)

  // Assert
  expect(send.getAttribute('aria-disabled')).toBe('true')
  expect(document.activeElement).toBe(send)
})

test('a held button starts nothing when pressed again', async () => {
  // Arrange
  const pressed = vi.fn()
  const user = userEvent.setup()
  render(<HeldWhilePressed onPress={pressed} />)
  await user.click(screen.getByRole('button', { name: 'Send' }))

  // Act
  await user.click(screen.getByRole('button', { name: 'Send' }))

  // Assert
  expect(pressed).toHaveBeenCalledOnce()
})

test('a held submitting button does not send its form', async () => {
  // Arrange
  const sent = vi.fn()
  const user = userEvent.setup()
  render(
    <form
      aria-label="Message"
      onSubmit={(event) => {
        event.preventDefault()
        sent()
      }}
    >
      <Button variant="primary" type="submit" held>
        Send
      </Button>
    </form>,
  )

  // Act
  await user.click(screen.getByRole('button', { name: 'Send' }))

  // Assert
  expect(sent).not.toHaveBeenCalled()
})

test('a button not held says nothing of being off', () => {
  // Act
  render(
    <Button variant="primary" held={false}>
      Send
    </Button>,
  )

  // Assert
  expect(screen.getByRole('button', { name: 'Send' }).hasAttribute('aria-disabled')).toBe(false)
})
