import { useRef, useState } from 'react'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import { Autocomplete, type AutocompleteGroup } from './Autocomplete.tsx'

// Fruit is an item the tests list, under a group by its kind.
interface Fruit {
  id: string
  name: string
}

const groups: AutocompleteGroup<Fruit>[] = [
  { name: 'Citrus', items: [{ id: 'lemon', name: 'lemon' }] },
  {
    name: 'Berries',
    items: [
      { id: 'raspberry', name: 'raspberry' },
      { id: 'blueberry', name: 'blueberry' },
    ],
  },
]

// Picker is an autocomplete over the fruit, shown as the caller filters them:
// those whose names hold what was typed.
function Picker({ onChoose }: { onChoose: (fruit: Fruit) => void }) {
  const [typed, setTyped] = useState('')
  const input = useRef<HTMLInputElement>(null)
  const shown = groups
    .map((group) => ({
      ...group,
      items: group.items.filter((fruit) => fruit.name.includes(typed)),
    }))
    .filter((group) => group.items.length > 0)

  return (
    <Autocomplete
      groups={shown}
      typed={typed}
      onType={setTyped}
      label="Fruit"
      listLabel="Fruits"
      placeholder="Find a fruit"
      inputRef={input}
      itemKey={(fruit) => fruit.id}
      itemText={(fruit) => fruit.name}
      renderItem={(fruit) => fruit.name}
      onChoose={onChoose}
      empty="No fruit is called that."
    />
  )
}

// options is the text of each option listed, in order.
function options(): (string | null)[] {
  return within(screen.getByRole('listbox', { name: 'Fruits' }))
    .getAllByRole('option')
    .map((option) => option.textContent)
}

test('an autocomplete lists what the caller passes, under each group', () => {
  // Act
  render(<Picker onChoose={vi.fn()} />)

  // Assert
  const list = screen.getByRole('listbox', { name: 'Fruits' })
  expect(within(list).getByRole('group', { name: 'Berries' })).toBeTruthy()
  expect(options()).toEqual(['lemon', 'raspberry', 'blueberry'])
})

test('typing narrows the list as the caller filters it', async () => {
  // Arrange
  const user = userEvent.setup()
  render(<Picker onChoose={vi.fn()} />)

  // Act
  await user.type(screen.getByRole('combobox', { name: 'Fruit' }), 'berry')

  // Assert
  expect(options()).toEqual(['raspberry', 'blueberry'])
})

test('Down then Enter chooses the second item', async () => {
  // Arrange
  const user = userEvent.setup()
  const onChoose = vi.fn()
  render(<Picker onChoose={onChoose} />)
  await user.click(screen.getByRole('combobox', { name: 'Fruit' }))

  // Act
  await user.keyboard('{ArrowDown}{Enter}')

  // Assert
  expect(onChoose).toHaveBeenCalledWith({ id: 'raspberry', name: 'raspberry' })
})

test('a press on an item chooses it', async () => {
  // Arrange
  const user = userEvent.setup()
  const onChoose = vi.fn()
  render(<Picker onChoose={onChoose} />)

  // Act
  await user.click(screen.getByRole('option', { name: 'blueberry' }))

  // Assert
  expect(onChoose).toHaveBeenCalledWith({ id: 'blueberry', name: 'blueberry' })
})

test('an autocomplete says so when nothing matches', async () => {
  // Arrange
  const user = userEvent.setup()
  render(<Picker onChoose={vi.fn()} />)

  // Act
  await user.type(screen.getByRole('combobox', { name: 'Fruit' }), 'kiwi')

  // Assert
  expect(screen.getByRole('status').textContent).toBe('No fruit is called that.')
})
