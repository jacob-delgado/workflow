import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { mockConfig } from '@/dev/mockConfig.ts'
import { useViews } from '@/features/issues/issueApi.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { useSaveConfig } from './configApi.ts'

test('saving the configuration reads the issue views again', async () => {
  // Arrange
  // A save can add, rename or remove a view, and the view select offers only
  // listed views, so the list is read again rather than served from the cache.
  const requests = fakeApi({
    '/api/views': { views: [{ name: 'Assigned to me', jql: 'assignee = currentUser()' }] },
    '/api/config': mockConfig,
  })
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  )
  const { result } = renderHook(() => ({ views: useViews(), save: useSaveConfig() }), { wrapper })
  await waitFor(() => {
    expect(result.current.views.isSuccess).toBe(true)
  })

  // Act
  await act(() => result.current.save(mockConfig, '"read-1"'))

  // Assert
  await waitFor(() => {
    const viewReads = requests.filter((request) => new URL(request.url).pathname === '/api/views')
    expect(viewReads).toHaveLength(2)
  })
})
