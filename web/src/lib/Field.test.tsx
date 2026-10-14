import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { vi } from 'vitest'
import { Input, Select, TextArea } from './Field.tsx'

function SavesAsItChanges() {
  const [saving, setSaving] = useState(false)

  return (
    <Select
      aria-label="Owner"
      held={saving}
      defaultValue="alice"
      onChange={() => {
        setSaving(true)
      }}
    >
      <option value="alice">Alice</option>
      <option value="bob">Bob</option>
    </Select>
  )
}

test('a select held while its choice saves keeps the focus it was changed with', async () => {
  // Arrange
  const user = userEvent.setup()
  render(<SavesAsItChanges />)
  const owner = screen.getByRole('combobox', { name: 'Owner' })

  // Act
  await user.selectOptions(owner, 'bob')

  // Assert
  expect(owner.getAttribute('aria-disabled')).toBe('true')
  expect(document.activeElement).toBe(owner)
})

test('a held select hears no change', async () => {
  // Arrange
  const changed = vi.fn()
  const user = userEvent.setup()
  render(
    <Select aria-label="Owner" held value="alice" onChange={changed}>
      <option value="alice">Alice</option>
      <option value="bob">Bob</option>
    </Select>,
  )

  // Act
  await user.selectOptions(screen.getByRole('combobox', { name: 'Owner' }), 'bob')

  // Assert
  expect(changed).not.toHaveBeenCalled()
})

test('a held checkbox hears no change', async () => {
  // Arrange
  const changed = vi.fn()
  const user = userEvent.setup()
  render(<Input type="checkbox" aria-label="Backend" held checked={false} onChange={changed} />)

  // Act
  await user.click(screen.getByRole('checkbox', { name: 'Backend' }))

  // Assert
  expect(changed).not.toHaveBeenCalled()
})

test('a held text area takes no typing', async () => {
  // Arrange
  const changed = vi.fn()
  const user = userEvent.setup()
  render(<TextArea aria-label="Message" held value="Ready" onChange={changed} />)

  // Act
  await user.type(screen.getByRole('textbox', { name: 'Message' }), '!')

  // Assert
  expect(changed).not.toHaveBeenCalled()
})

test.each([
  {
    kind: 'a text field',
    role: 'textbox',
    field: <Input aria-label="Field" aria-describedby="hint" error="why" />,
  },
  {
    kind: 'a choice',
    role: 'combobox',
    field: (
      <Select aria-label="Field" aria-describedby="hint" error="why">
        <option>One</option>
      </Select>
    ),
  },
  {
    kind: 'a text area',
    role: 'textbox',
    field: <TextArea aria-label="Field" aria-describedby="hint" error="why" />,
  },
])('$kind refused is invalid and described by why, after its hint', ({ role, field }) => {
  // Arrange
  render(
    <>
      {field}
      <p id="hint">The hint.</p>
      <p id="why">Refused.</p>
    </>,
  )

  // Act
  const drawn = screen.getByRole(role, { name: 'Field' })

  // Assert
  expect(drawn.getAttribute('aria-invalid')).toBe('true')
  expect(drawn.getAttribute('aria-describedby')).toBe('hint why')
})

test('a field not refused is not marked invalid', () => {
  // Arrange
  render(<Input aria-label="Field" aria-describedby="hint" />)

  // Act
  const drawn = screen.getByRole('textbox', { name: 'Field' })

  // Assert
  expect(drawn.hasAttribute('aria-invalid')).toBe(false)
  expect(drawn.getAttribute('aria-describedby')).toBe('hint')
})
