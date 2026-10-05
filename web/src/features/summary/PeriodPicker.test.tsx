import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import { PeriodCalendar, PeriodSteps } from './PeriodPicker.tsx'
import type { Period } from './civilDate.ts'

// today is the Wednesday the server says it is.
const today = '2026-09-16'

// tuesday is the day shown.
const tuesday: Period = { from: '2026-09-15', to: '2026-09-15' }

// showSteps draws the steps for period, and keeps each period picked.
function showSteps(period: Period) {
  const onPick = vi.fn()
  render(<PeriodSteps period={period} today={today} onPick={onPick} />)

  return onPick
}

// showCalendar draws the calendar on Tuesday, and keeps each period picked.
function showCalendar() {
  const onPick = vi.fn()
  render(<PeriodCalendar period={tuesday} today={today} onPick={onPick} />)

  return onPick
}

test.each([
  ['Earlier', { from: '2026-09-14', to: '2026-09-14' }],
  ['Later', { from: '2026-09-16', to: '2026-09-16' }],
  ['Today', { from: today, to: today }],
] as const)('%s picks the period it names', async (step, want) => {
  // Arrange
  const picked = showSteps(tuesday)

  // Act
  await userEvent.setup().click(screen.getByRole('button', { name: step }))

  // Assert
  expect(picked).toHaveBeenLastCalledWith(want)
})

test('Later is off once the period would start after today', async () => {
  // Arrange
  const picked = showSteps({ from: today, to: today })

  // Act
  await userEvent.setup().click(screen.getByRole('button', { name: 'Later' }))

  // Assert
  expect(screen.getByRole('button', { name: 'Later' }).getAttribute('aria-disabled')).toBe('true')
  expect(picked).not.toHaveBeenCalled()
})

test.each([
  ['Whole month', { from: '2025-03-01', to: '2025-03-31' }],
  ['Whole year', { from: '2025-01-01', to: '2025-12-31' }],
] as const)('%s picks it for the month the selects show', async (whole, want) => {
  // Arrange
  const picked = showCalendar()
  const user = userEvent.setup()
  await user.selectOptions(screen.getByRole('combobox', { name: 'Year' }), '2025')
  await user.selectOptions(screen.getByRole('combobox', { name: 'Month' }), 'March')

  // Act
  await user.click(screen.getByRole('button', { name: whole }))

  // Assert
  expect(picked).toHaveBeenLastCalledWith(want)
})

test('Page Down moves the calendar on a month, and the selects with it', async () => {
  // Arrange
  const picked = showCalendar()
  const user = userEvent.setup()
  screen.getByRole('gridcell', { name: 'Tuesday, September 15, 2026' }).focus()

  // Act
  await user.keyboard('{PageDown}{Enter}')

  // Assert
  expect(picked).toHaveBeenLastCalledWith({ from: '2026-10-15', to: '2026-10-15' })
  expect(screen.getByRole<HTMLSelectElement>('combobox', { name: 'Month' }).value).toBe('10')
})

test.each([
  ['Home', '2026-09-14'],
  ['End', '2026-09-20'],
  ['ArrowDown', '2026-09-22'],
  ['ArrowRight', '2026-09-16'],
  ['PageUp', '2026-08-15'],
] as const)('%s moves to the day the date grid says', async (key, want) => {
  // Arrange
  const picked = showCalendar()
  const user = userEvent.setup()
  screen.getByRole('gridcell', { name: 'Tuesday, September 15, 2026' }).focus()

  // Act
  await user.keyboard(`{${key}}{Enter}`)

  // Assert
  expect(picked).toHaveBeenLastCalledWith({ from: want, to: want })
})
