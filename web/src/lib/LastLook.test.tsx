import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import { LastLook, shortcutsHeld } from './LastLook.tsx'

test('a last look takes the focus, named by its question, and holds the single keys', () => {
  // Act
  render(<LastLook question="Remove the cache?" act="Remove" onAct={vi.fn()} onCancel={vi.fn()} />)

  // Assert
  expect(document.activeElement).toBe(screen.getByRole('group', { name: 'Remove the cache?' }))
  expect(shortcutsHeld()).toBe(true)
})

test('a last look let go of holds the single keys no longer', () => {
  // Arrange
  const view = render(
    <LastLook question="Remove the cache?" act="Remove" onAct={vi.fn()} onCancel={vi.fn()} />,
  )

  // Act
  view.unmount()

  // Assert
  expect(shortcutsHeld()).toBe(false)
})

test('while its write runs, both buttons are held and the act says so', async () => {
  // Arrange
  const user = userEvent.setup()
  const onAct = vi.fn()
  const onCancel = vi.fn()
  render(
    <LastLook
      question="Forget dan?"
      act="Forget"
      acting="Forgetting…"
      write={{ state: 'running', error: '' }}
      onAct={onAct}
      onCancel={onCancel}
    />,
  )

  // Act
  await user.click(screen.getByRole('button', { name: 'Forgetting…' }))
  await user.click(screen.getByRole('button', { name: 'Cancel' }))

  // Assert
  expect(onAct).not.toHaveBeenCalled()
  expect(onCancel).not.toHaveBeenCalled()
})

test('a refused write is said beside the buttons, which answer again', async () => {
  // Arrange
  const user = userEvent.setup()
  const onAct = vi.fn()
  render(
    <LastLook
      question="Forget dan?"
      cost="They are asked about again."
      act="Forget"
      write={{ state: 'error', error: 'dan was not forgotten.' }}
      onAct={onAct}
      onCancel={vi.fn()}
    />,
  )

  // Act
  await user.click(screen.getByRole('button', { name: 'Forget' }))

  // Assert
  expect(screen.getByRole('alert').textContent).toBe('dan was not forgotten.')
  expect(screen.getByText('They are asked about again.')).toBeTruthy()
  expect(onAct).toHaveBeenCalledOnce()
})

test('a last look drawn as a section is named by its heading, which takes the focus', () => {
  // Act
  render(
    <LastLook
      section
      question="Switch to ~/src/web?"
      act="Switch"
      onAct={vi.fn()}
      onCancel={vi.fn()}
    />,
  )

  // Assert
  expect(screen.getByRole('region', { name: 'Switch to ~/src/web?' })).toBeTruthy()
  expect(document.activeElement).toBe(screen.getByRole('heading', { name: 'Switch to ~/src/web?' }))
})

test('a preview with no question is named by its label', () => {
  // Act
  render(
    <LastLook label="Summary preview" act="Post" onAct={vi.fn()} onCancel={vi.fn()}>
      <p>What was done this week.</p>
    </LastLook>,
  )

  // Assert
  expect(document.activeElement).toBe(screen.getByRole('group', { name: 'Summary preview' }))
  expect(screen.getByText('What was done this week.')).toBeTruthy()
})
