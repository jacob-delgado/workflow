import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import App from './App.tsx'

test('shows the sections and opens on the Issues view', () => {
  // Arrange
  render(<App />)

  // Act
  const nav = screen.getByRole('navigation', { name: /sections/i })

  // Assert
  expect(nav).toBeTruthy()
  expect(screen.getByRole('heading', { level: 1, name: /issues/i })).toBeTruthy()
})

test('switches the view when another section is chosen', async () => {
  // Arrange
  const user = userEvent.setup()
  render(<App />)

  // Act
  await user.click(screen.getByRole('button', { name: /branch/i }))

  // Assert
  expect(screen.getByRole('heading', { level: 1, name: /branch/i })).toBeTruthy()
})
