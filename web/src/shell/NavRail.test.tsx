import { render, screen } from '@testing-library/react'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { NavRail } from './NavRail.tsx'

test('marks the current section for assistive tech', () => {
  // Arrange
  render(<NavRail />)

  // Act
  const current = screen.getByRole('button', { name: /issues/i, current: 'page' })

  // Assert
  expect(current).toBeTruthy()
})

test('names the messaging section after the configured service', () => {
  // Arrange
  const snapshot = makeSnapshot()
  useSnapshotStore.setState({
    status: 'live',
    snapshot: { ...snapshot, messaging: { ...snapshot.messaging, service: 'Teams' } },
  })

  // Act
  render(<NavRail />)

  // Assert
  expect(screen.getByRole('button', { name: 'Teams' })).toBeTruthy()
  expect(screen.queryByRole('button', { name: 'Messaging' })).toBeNull()
})

test('calls the messaging section Messaging until the stream names the service', () => {
  // Act
  render(<NavRail />)

  // Assert
  expect(screen.getByRole('button', { name: 'Messaging' })).toBeTruthy()
})
