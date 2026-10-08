import { vi } from 'vitest'
import { adoptSession, sessionStorageKey } from './session.ts'

// at puts the page at address, as a browser opening it would.
function at(address: string): void {
  window.history.replaceState(null, '', address)
}

afterEach(() => {
  at('/')
  vi.restoreAllMocks()
})

test('the page adopts the session its address carries, and takes it out of the address', () => {
  // Arrange
  at('/?tab=1#session=ABC234')

  // Act
  const session = adoptSession()

  // Assert
  expect(session).toBe('ABC234')
  expect(window.location.hash).toBe('')
  expect(window.location.search).toBe('?tab=1')
  expect(localStorage.getItem(sessionStorageKey)).toBe('ABC234')
})

test('a page opened at the bare address presents the session kept from before', () => {
  // Arrange
  localStorage.setItem(sessionStorageKey, 'KEPT567')
  at('/')

  // Act
  const session = adoptSession()

  // Assert
  expect(session).toBe('KEPT567')
})

test('a session the address carries replaces the one kept', () => {
  // Arrange
  localStorage.setItem(sessionStorageKey, 'OLDRUN')
  at('/#session=NEWRUN')

  // Act
  const session = adoptSession()

  // Assert
  expect(session).toBe('NEWRUN')
  expect(localStorage.getItem(sessionStorageKey)).toBe('NEWRUN')
})

test('a page with none presents none', () => {
  // Act
  const session = adoptSession()

  // Assert
  expect(session).toBe('')
})

test('with storage blocked, the session the address carries is still adopted', () => {
  // Arrange
  vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
    throw new DOMException('blocked', 'SecurityError')
  })
  at('/#session=ABC234')

  // Act
  const session = adoptSession()

  // Assert
  expect(session).toBe('ABC234')
  expect(window.location.hash).toBe('')
})
