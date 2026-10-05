import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { vi } from 'vitest'
import { mockRepositories } from '@/dev/mockRepositories.ts'
import { useDirectories, useRepositories } from './repositoriesApi.ts'

// withClient renders a hook under a client of its own.
function withClient({ children }: { children: ReactNode }) {
  return (
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      {children}
    </QueryClientProvider>
  )
}

test('the mockup serves its own directories with no server', async () => {
  // Arrange
  vi.stubEnv('VITE_MOCK', 'true')

  // Act
  const { result: repositories } = renderHook(() => useRepositories(), { wrapper: withClient })
  const { result: listed } = renderHook(() => useDirectories(''), { wrapper: withClient })

  // Assert
  await waitFor(() => {
    expect(repositories.current.data).toEqual(mockRepositories())
  })
  await waitFor(() => {
    expect(listed.current.data?.shown).toBe('~/src/api/cmd')
  })
})

test('the mockup lists a directory it is asked for', async () => {
  // Arrange
  vi.stubEnv('VITE_MOCK', 'true')

  // Act
  const { result: listed } = renderHook(() => useDirectories('/home/ana/src'), {
    wrapper: withClient,
  })

  // Assert
  await waitFor(() => {
    expect(listed.current.data?.entries.map((entry) => entry.name)).toEqual(['api', 'web'])
  })
})
