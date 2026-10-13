import { render, screen } from '@testing-library/react'
import { vi } from 'vitest'
import { FilterChips } from './FilterChips.tsx'

test("a chip draws its value's mark before its name, hidden, so it is named as before", () => {
  // Arrange
  const choices = [
    { value: 'To Do', count: 2 },
    { value: 'Done', count: 1 },
  ]

  // Act
  render(
    <FilterChips
      label="Filter"
      choices={choices}
      isPicked={(value) => value === 'Done'}
      nameOf={(value) => value}
      keyOf={(value) => value}
      markOf={(value) => (value === 'To Do' ? <span>○</span> : undefined)}
      onToggle={vi.fn()}
    />,
  )

  // Assert
  expect(screen.getByRole('button', { name: 'To Do 2' }).textContent).toBe('○To Do 2')
  expect(screen.getByRole('button', { name: 'Done 1', pressed: true }).textContent).toBe('Done 1')
})
