import { render, screen } from '@testing-library/react'
import { vi } from 'vitest'
import { Meta } from './Meta.tsx'

test('a row of facts draws each once, a fact left out taking no place', () => {
  // Arrange
  const complaints = vi.spyOn(console, 'error').mockImplementation(() => {})

  // Act
  render(
    <p>
      <Meta>
        {3}
        {null}
        <code>main</code>
        repeated
        {'repeated'}
      </Meta>
    </p>,
  )

  // Assert
  expect(screen.getByRole('paragraph').textContent).toBe('3· main· repeated· repeated')
  expect(complaints).not.toHaveBeenCalled()
})
