import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, type RenderResult } from '@testing-library/react'
import type { ReactElement } from 'react'
import { queryClient } from '@/queryClient.ts'

// Renders inside a fresh QueryClient, for components that read the server
// through TanStack Query. Retries off so a failing query surfaces at once.
export function renderWithClient(ui: ReactElement): RenderResult {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })

  return render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>)
}

// appQueryClient is a client with the app's own query defaults, retries off,
// for a test of whether a query reads again: under those defaults a query reads
// again only when its own staleTime says so.
export function appQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: { queries: { ...queryClient.getDefaultOptions().queries, retry: false } },
  })
}
