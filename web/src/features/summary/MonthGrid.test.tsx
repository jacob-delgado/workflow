import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import { markShape } from '@/test/marks.tsx'
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

test('a day is named with how much was done on it, and one with nothing by its date alone', () => {
  // Arrange
  const amounts = new Map([
    ['2026-09-15', 1],
    ['2026-09-16', 3],
  ])

  // Act
  render(
    <MonthGrid
      year={2026}
      month={9}
      period={{ from: '2026-09-15', to: '2026-09-15' }}
      done={{ amounts, whole: true }}
      onPick={vi.fn()}
    />,
  )

  // Assert
  const picked = screen.getByRole('gridcell', { name: 'Tuesday, September 15, 2026, 1 thing done' })
  expect(picked.getAttribute('aria-selected')).toBe('true')
  expect(
    screen.getByRole('gridcell', { name: 'Wednesday, September 16, 2026, 3 things done' }),
  ).toBeTruthy()
  expect(screen.getByRole('gridcell', { name: 'Monday, September 14, 2026' })).toBeTruthy()
})

// dotsOn is how many dots a day's cell draws under its number.
function dotsOn(name: string): number {
  const shape = markShape(screen.getByRole('gridcell', { name }))

  return shape === '' ? 0 : shape.split(' | ').length
}

test("a day's dots step with its share of the month's busiest, and a day with nothing has none", () => {
  // Arrange
  const amounts = new Map([
    ['2026-09-14', 1],
    ['2026-09-15', 2],
    ['2026-09-16', 3],
  ])

  // Act
  render(
    <MonthGrid
      year={2026}
      month={9}
      period={{ from: '2026-09-01', to: '2026-09-01' }}
      done={{ amounts, whole: true }}
      onPick={vi.fn()}
    />,
  )

  // Assert
  expect(
    [
      'Monday, September 14, 2026, 1 thing done',
      'Tuesday, September 15, 2026, 2 things done',
      'Wednesday, September 16, 2026, 3 things done',
      'Thursday, September 17, 2026',
    ].map(dotsOn),
  ).toEqual([1, 2, 3, 0])
})
