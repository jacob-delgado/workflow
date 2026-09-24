import { onlineManager, QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { mockConfig } from '@/dev/mockConfig.ts'
import { useViews } from '@/features/issues/issueApi.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { appQueryClient } from '@/test/renderWithClient.tsx'
import { useConfig, useReloadConfig, useSaveConfig } from './configApi.ts'

// The issue views the server lists in these tests.
const listedViews = { views: [{ name: 'Assigned to me', jql: 'assignee = currentUser()' }] }

// configAt answers a read of the configuration at revision.
function configAt(revision: string, config = mockConfig): Response {
  return Response.json(config, { headers: { ETag: `"${revision}"` } })
}

// viewReads is how many times the issue views were read.
function viewReads(requests: Request[]): number {
  return requests.filter((request) => new URL(request.url).pathname === '/api/views').length
}

// appWrapper provides one client with the app's own query defaults.
function appWrapper() {
  const client = appQueryClient()

  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  )
}

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

test('reloading the configuration reads the issue views again', async () => {
  // Arrange
  // A reload can take up an edit that added, renamed or removed a view.
  const requests = fakeApi({
    '/api/views': { views: [{ name: 'Assigned to me', jql: 'assignee = currentUser()' }] },
    '/api/config': mockConfig,
  })
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  )
  const { result } = renderHook(() => ({ views: useViews(), reload: useReloadConfig() }), {
    wrapper,
  })
  await waitFor(() => {
    expect(result.current.views.isSuccess).toBe(true)
  })

  // Act
  await act(() => result.current.reload())

  // Assert
  await waitFor(() => {
    const viewReads = requests.filter((request) => new URL(request.url).pathname === '/api/views')
    expect(viewReads).toHaveLength(2)
  })
})

test('each surface that opens reads the configuration again', async () => {
  // Arrange
  // The file can change on disk while nothing shows it, so an open never
  // trusts the cached read.
  const requests = fakeApi({ '/api/config': mockConfig })
  const client = appQueryClient()
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  )
  const { result: first, unmount } = renderHook(() => useConfig(), { wrapper })
  await waitFor(() => {
    expect(first.current.isSuccess).toBe(true)
  })
  unmount()

  // Act
  const { result: second } = renderHook(() => useConfig(), { wrapper })

  // Assert
  await waitFor(() => {
    expect(second.current.isFetching).toBe(false)
  })
  expect(requests).toHaveLength(2)
})

test('a first read of the configuration reads the issue views again', async () => {
  // Arrange
  // The server takes up an edit to the file when it is read, and a first read
  // cannot tell whether it took one up since the views were listed.
  const requests = fakeApi({ '/api/views': listedViews, '/api/config': () => configAt('read-1') })
  const wrapper = appWrapper()
  const { result: listed } = renderHook(() => useViews(), { wrapper })
  await waitFor(() => {
    expect(listed.current.isSuccess).toBe(true)
  })

  // Act
  renderHook(() => useConfig(), { wrapper })

  // Assert
  await waitFor(() => {
    expect(viewReads(requests)).toBe(2)
  })
})

test('opening a surface over a file at a new revision reads the issue views again', async () => {
  // Arrange
  // The file was edited while nothing showed it; the read on open takes the
  // edit up, and it may have changed the views.
  const answers = [configAt('read-1'), configAt('read-2')]
  const requests = fakeApi({ '/api/views': listedViews, '/api/config': () => answers.shift() })
  const wrapper = appWrapper()
  const { result: first, unmount } = renderHook(() => useConfig(), { wrapper })
  await waitFor(() => {
    expect(first.current.isSuccess).toBe(true)
  })
  unmount()
  const { result: listed } = renderHook(() => useViews(), { wrapper })
  await waitFor(() => {
    expect(listed.current.isSuccess).toBe(true)
  })

  // Act
  renderHook(() => useConfig(), { wrapper })

  // Assert
  await waitFor(() => {
    expect(viewReads(requests)).toBe(2)
  })
})

test('reloading the configuration replaces the cached copy every surface reads', async () => {
  // Arrange
  const edited = {
    ...mockConfig,
    jira: { ...mockConfig.jira, base_url: 'https://edited.example.com' },
  }
  const answers = [configAt('read-1'), configAt('read-2', edited)]
  fakeApi({ '/api/config': () => answers.shift() })
  const { result } = renderHook(() => ({ config: useConfig(), reload: useReloadConfig() }), {
    wrapper: appWrapper(),
  })
  await waitFor(() => {
    expect(result.current.config.data?.jira.base_url).toBe(mockConfig.jira.base_url)
  })

  // Act
  await act(() => result.current.reload())

  // Assert
  await waitFor(() => {
    expect(result.current.config.data?.jira.base_url).toBe(edited.jira.base_url)
  })
})

test('a reconnect does not read the configuration again', async () => {
  // Arrange
  // A reconnect says nothing about the file, and a read that failed then would
  // take an open form, and the edits in it, away.
  const requests = fakeApi({ '/api/config': mockConfig })
  const { result } = renderHook(() => useConfig(), { wrapper: appWrapper() })
  await waitFor(() => {
    expect(result.current.isSuccess).toBe(true)
  })

  // Act
  act(() => {
    onlineManager.setOnline(false)
    onlineManager.setOnline(true)
  })

  // Assert
  await act(() => new Promise((resolve) => setTimeout(resolve, 0)))
  expect(requests).toHaveLength(1)
})
