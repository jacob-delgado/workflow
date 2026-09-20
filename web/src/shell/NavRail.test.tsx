import { render, screen } from '@testing-library/react'
import { NavRail } from './NavRail.tsx'

test('marks the current section for assistive tech', () => {
  // Arrange
  render(<NavRail />)

  // Act
  const current = screen.getByRole('button', { name: /issues/i, current: 'page' })

  // Assert
  expect(current).toBeTruthy()
})
