import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { vi } from 'vitest'
import { Input } from './Field.tsx'
import { WriteForm } from './WriteForm.tsx'

function LogWork({ onSend }: { onSend: () => void }) {
  const [sending, setSending] = useState(false)
  const [spent, setSpent] = useState('1h')

  return (
    <WriteForm
      label="Log work"
      act="Log work"
      busy={sending ? 'Logging…' : null}
      error=""
      onSend={() => {
        onSend()
        setSending(true)
      }}
      onCancel={vi.fn()}
    >
      <Input
        aria-label="Time spent"
        value={spent}
        onChange={(event) => {
          setSpent(event.target.value)
        }}
      />
    </WriteForm>
  )
}

test('a field sent from with Enter keeps its focus while the form sends', async () => {
  // Arrange
  const user = userEvent.setup()
  render(<LogWork onSend={vi.fn()} />)
  const spent = screen.getByRole('textbox', { name: 'Time spent' })
  await user.click(spent)

  // Act
  await user.keyboard('{Enter}')

  // Assert
  expect(screen.getByRole('button', { name: 'Logging…' }).getAttribute('aria-disabled')).toBe(
    'true',
  )
  expect(document.activeElement).toBe(spent)
})

test('a field takes no edit while its form sends', async () => {
  // Arrange
  const user = userEvent.setup()
  render(<LogWork onSend={vi.fn()} />)
  const spent = screen.getByRole('textbox', { name: 'Time spent' })
  await user.click(spent)
  await user.keyboard('{Enter}')

  // Act
  await user.type(spent, '30m')

  // Assert
  expect(spent).toHaveProperty('value', '1h')
})

test('the send keeps its focus while the form sends, and sends once', async () => {
  // Arrange
  const sent = vi.fn()
  const user = userEvent.setup()
  render(<LogWork onSend={sent} />)
  await user.click(screen.getByRole('button', { name: 'Log work' }))

  // Act
  await user.click(screen.getByRole('button', { name: 'Logging…' }))

  // Assert
  expect(sent).toHaveBeenCalledOnce()
  expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Logging…' }))
})
