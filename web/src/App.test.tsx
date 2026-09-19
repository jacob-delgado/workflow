import { render, screen } from '@testing-library/react'
import App from './App.tsx'

test('names the product and how to serve it', () => {
  // Arrange
  render(<App />)

  // Act
  const heading = screen.getByRole('heading', { level: 1, name: /workflow/i })

  // Assert
  expect(heading).toBeTruthy()
  expect(screen.getByText(/workflow --web/i)).toBeTruthy()
})
