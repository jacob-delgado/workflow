import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import { MonthGrid } from './MonthGrid.tsx'

// showGrid draws September 2026 with Tuesday the 15th picked, and keeps
// each period picked.
function showGrid() {
  const onPick = vi.fn()
  render(
    <MonthGrid
      year={2026}
      month={9}
      period={{ from: '2026-09-15', to: '2026-09-15' }}
      onPick={onPick}
    />,
  )

  return onPick
}

test('the picked day is the one Tab reaches, and is marked selected', () => {
  // Act
  showGrid()

  // Assert
  const picked = screen.getByRole('gridcell', { name: 'Tuesday, September 15, 2026' })
  expect(picked.getAttribute('aria-selected')).toBe('true')
  expect(picked.getAttribute('tabindex')).toBe('0')
  expect(
    screen.getByRole('gridcell', { name: 'Monday, September 14, 2026' }).getAttribute('tabindex'),
  ).toBe('-1')
})

test('arrows move between days and Enter picks one', async () => {
  // Arrange
  const picked = showGrid()
  const user = userEvent.setup()
  screen.getByRole('gridcell', { name: 'Tuesday, September 15, 2026' }).focus()

  // Act
  await user.keyboard('{ArrowUp}{ArrowLeft}{Enter}')

  // Assert
  // Up is a week back, left a day.
  expect(picked).toHaveBeenLastCalledWith({ from: '2026-09-07', to: '2026-09-07' })
})

test('Shift and Enter picks the range from the picked day to the one in focus', async () => {
  // Arrange
  const picked = showGrid()
  const user = userEvent.setup()
  screen.getByRole('gridcell', { name: 'Tuesday, September 15, 2026' }).focus()

  // Act
  await user.keyboard('{ArrowLeft}{ArrowLeft}{Shift>}{Enter}{/Shift}')

  // Assert
  expect(picked).toHaveBeenLastCalledWith({ from: '2026-09-13', to: '2026-09-15' })
})

test('a shift-click picks the range too', async () => {
  // Arrange
  const picked = showGrid()
  const user = userEvent.setup()

  // Act
  await user.keyboard('{Shift>}')
  await user.click(screen.getByRole('gridcell', { name: 'Friday, September 18, 2026' }))

  // Assert
  expect(picked).toHaveBeenLastCalledWith({ from: '2026-09-15', to: '2026-09-18' })
})
