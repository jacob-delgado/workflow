import { fireEvent, render, screen } from '@testing-library/react'
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
