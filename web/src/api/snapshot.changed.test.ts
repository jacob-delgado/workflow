import { act, renderHook } from '@testing-library/react'
import { useChangedByStream } from './snapshot.ts'

// A value the stream changes under a mounted row is marked changed until the
// row's highlight has played; one it first draws is not.

test('a value first drawn is not marked changed', () => {
  // Act
  const { result } = renderHook(({ value }) => useChangedByStream(value), {
    initialProps: { value: 'running' },
  })

  // Assert
  expect(result.current.changed).toBe(false)
})

test('a value a later frame changes is marked changed', () => {
  // Arrange
  const { result, rerender } = renderHook(({ value }) => useChangedByStream(value), {
    initialProps: { value: 'running' },
  })

  // Act
  rerender({ value: 'passed' })

  // Assert
  expect(result.current.changed).toBe(true)
})

test('a frame that leaves the value as it was marks nothing', () => {
  // Arrange
  const { result, rerender } = renderHook(({ value }) => useChangedByStream(value), {
    initialProps: { value: 'running' },
  })

  // Act
  rerender({ value: 'running' })

  // Assert
  expect(result.current.changed).toBe(false)
})

test('a change is settled once its highlight has played', () => {
  // Arrange
  const { result, rerender } = renderHook(({ value }) => useChangedByStream(value), {
    initialProps: { value: 'running' },
  })
  rerender({ value: 'passed' })

  // Act
  act(() => {
    result.current.settle()
  })

  // Assert
  expect(result.current.changed).toBe(false)
})
