import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, type RenderResult } from '@testing-library/react'
import type { ReactElement } from 'react'
import { queryClient } from '@/queryClient.ts'

// appQueryClient is a client of its own with the app's own query defaults — a
// query reads again only when its own staleTime says so, never on focus, and
// retries as the app does — so a test meets the reads, and the retries,
// production makes.
export function appQueryClient(): QueryClient {
  return new QueryClient({ defaultOptions: queryClient.getDefaultOptions() })
}

// renderWithClient renders a component that reads the server through TanStack
// Query inside a client of its own, kept across a rerender. The client keeps
// the app's own query defaults (appQueryClient), so a test sees the reads
// production makes; one that needs another policy passes its own client.
export function renderWithClient(
  ui: ReactElement,
  client: QueryClient = appQueryClient(),
): RenderResult {
  return render(ui, {
    wrapper: ({ children }) => (
      <QueryClientProvider client={client}>{children}</QueryClientProvider>
    ),
  })
}
