import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import { Combobox } from './Combobox.tsx'

interface Member {
  id: string
  label: string
}

const members: Member[] = [
  { id: 'U0ANA', label: 'Ana Ruiz' },
  { id: 'U0BEN', label: 'Ben Ito' },
  { id: 'U0BEA', label: 'Bea Kim' },
]

// Members is a combobox of the members, choosing one into chosen.
function Members({ chosen, held = false }: { chosen: (member: Member) => void; held?: boolean }) {
  return (
    <Combobox
      label="Slack user for ben"
      placeholder="Not linked yet"
      items={members}
      itemKey={(member) => member.id}
      itemText={(member) => member.label}
      empty="No member matches."
      held={held}
      onChoose={chosen}
    />
  )
}

test('typing narrows the members to those that match', async () => {
  // Arrange
  const user = userEvent.setup()
  render(<Members chosen={vi.fn()} />)

  // Act
  await user.type(screen.getByRole('combobox', { name: 'Slack user for ben' }), 'be')

  // Assert
  const offered = (await screen.findAllByRole('option')).map((option) => option.textContent)
  expect(offered).toEqual(['Ben Ito', 'Bea Kim'])
})

test('choosing a member hands it on', async () => {
  // Arrange
  const chosen = vi.fn()
  const user = userEvent.setup()
  render(<Members chosen={chosen} />)
  await user.type(screen.getByRole('combobox', { name: 'Slack user for ben' }), 'ben')

  // Act
  await user.click(await screen.findByRole('option', { name: 'Ben Ito' }))

  // Assert
  expect(chosen).toHaveBeenCalledWith(members[1])
})

test('a name no member has says so', async () => {
  // Arrange
  const user = userEvent.setup()
  render(<Members chosen={vi.fn()} />)

  // Act
  await user.type(screen.getByRole('combobox', { name: 'Slack user for ben' }), 'zed')

  // Assert
  expect(await screen.findByText('No member matches.')).toBeTruthy()
})

test('a held combobox keeps the focus and takes no typing', async () => {
  // Arrange
  const user = userEvent.setup()
  render(<Members chosen={vi.fn()} held />)
  const field = screen.getByRole('combobox', { name: 'Slack user for ben' })
  field.focus()

  // Act
  await user.keyboard('be')

  // Assert
  expect(field.getAttribute('aria-disabled')).toBe('true')
  expect((field as HTMLInputElement).value).toBe('')
  expect(document.activeElement).toBe(field)
})
