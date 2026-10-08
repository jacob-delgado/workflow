// First, so zod is configured before any schema is made.
import './api/zodConfig.ts'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClientProvider } from '@tanstack/react-query'
import { ReactQueryDevtools } from '@tanstack/react-query-devtools'
import App from './App.tsx'
import { queryClient } from './queryClient.ts'
import './api/client.ts'
import './index.css'

declare global {
  interface ImportMetaEnv {
    // Set by `task web:mockup` to run the page against the mockup's server.
    readonly VITE_MOCK?: string
  }
}

// `task web:mockup` sets VITE_MOCK, and the mockup's server stands in for
// workflow's before the page asks it anything; a production build leaves it
// out.
if (import.meta.env.VITE_MOCK === 'true') {
  const { installMockServer } = await import('./dev/mockServer.ts')
  installMockServer()
}

const root = document.getElementById('root')
if (root) {
  createRoot(root).render(
    <StrictMode>
      <QueryClientProvider client={queryClient}>
        <App />
        {import.meta.env.DEV ? <ReactQueryDevtools initialIsOpen={false} /> : null}
      </QueryClientProvider>
    </StrictMode>,
  )
}
