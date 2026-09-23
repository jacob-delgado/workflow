import { act, renderHook, waitFor } from '@testing-library/react'
import { vi } from 'vitest'
import { useHealth, useHealthStore } from './health.ts'
import { useSnapshotStore } from './snapshot.ts'

test('reads a writable mockup under VITE_MOCK', async () => {
  // Arrange
  vi.stubEnv('VITE_MOCK', 'true')

  // Act
  renderHook(() => {
    useHealth()
  })

  // Assert
  await waitFor(() => {
    expect(useHealthStore.getState().health?.dry_run).toBe(false)
  })
})

test('stays empty when the health read fails', async () => {
  // Arrange
  // test-setup's fetch refuses every request, as an unreachable server would.
  const fetch = vi.mocked(globalThis.fetch)

  // Act
  renderHook(() => {
    useHealth()
  })

  // Assert
  await waitFor(() => {
    expect(fetch).toHaveBeenCalled()
  })
  expect(useHealthStore.getState().health).toBeNull()
})

test('does not read the health again when the stream first connects', async () => {
  // Arrange
  const fetch = vi.fn(() => Promise.resolve(Response.json({ version: '1.2.3', dry_run: false })))
  vi.stubGlobal('fetch', fetch)
  renderHook(() => {
    useHealth()
  })
  await waitFor(() => {
    expect(useHealthStore.getState().health).not.toBeNull()
  })

  // Act
  await act(async () => {
    useSnapshotStore.setState({ status: 'live' })
    // Let a read the change might start reach fetch before counting.
    await new Promise((resolve) => {
      setTimeout(resolve, 0)
    })
  })

  // Assert
  expect(fetch).toHaveBeenCalledTimes(1)
})
