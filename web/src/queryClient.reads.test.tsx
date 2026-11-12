import { renderHook, waitFor } from '@testing-library/react'
import { QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { useSlackGroups, useSlackMembers } from '@/features/messaging/slackApi.ts'
import { useReadKeys } from '@/features/keyboard/keysApi.ts'
import { useDirectories, useRepositories } from '@/features/repositories/repositoriesApi.ts'
import { useReviewQueue } from '@/features/reviewqueue/reviewQueueApi.ts'
import { useConfigRead } from '@/features/settings/configApi.ts'
import { useLocalData } from '@/features/settings/people/localDataApi.ts'
import { usePeople, useRepoGroups } from '@/features/settings/people/peopleApi.ts'
import { useSetupOffer } from '@/features/settings/setupApi.ts'
import { useActivity, useMonthRead } from '@/features/summary/summaryApi.ts'
import { useTasks } from '@/features/tasks/tasksApi.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { appQueryClient } from '@/test/renderWithClient.tsx'

// failedInPassing is a route whose first read the server fails in passing,
// with a 502 problem, and which holds every read after it, counting each.
function failedInPassing(): { route: () => Promise<Response>; reads: () => number } {
  let reads = 0

  return {
    route: () => {
      reads += 1
      if (reads > 1) {
        return new Promise<Response>(() => undefined)
      }

      return Promise.resolve(
        Response.json(
          {
            type: 'about:blank',
            title: 'Bad Gateway',
            status: 502,
            code: 'fetch_failed',
            detail: 'why',
          },
          { status: 502, headers: { 'Content-Type': 'application/problem+json' } },
        ),
      )
    },
    reads: () => reads,
  }
}

// inClient renders children inside a client with the app's own defaults.
function inClient(): ({ children }: { children: ReactNode }) => ReactNode {
  const client = appQueryClient()

  return ({ children }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>
}

test.each([
  { read: 'the tasks', path: '/api/tasks', use: () => useTasks() },
  { read: 'the activity', path: '/api/activity', use: () => useActivity(null) },
  {
    read: "a calendar month's activity",
    path: '/api/activity',
    use: () => useMonthRead({ from: '2026-08-01', to: '2026-08-31' }, '2026-08-20'),
  },
  { read: 'where the server works', path: '/api/repositories', use: () => useRepositories() },
  { read: 'a directory', path: '/api/directories', use: () => useDirectories('') },
  { read: 'the review queue', path: '/api/reviews', use: () => useReviewQueue() },
  { read: 'the configuration', path: '/api/config', use: () => useConfigRead() },
  { read: 'the setup offer', path: '/api/config/setup', use: () => useSetupOffer() },
  { read: 'the local data', path: '/api/local-data', use: () => useLocalData() },
  { read: 'the people', path: '/api/people', use: () => usePeople() },
  { read: 'the repository groups', path: '/api/repo-groups', use: () => useRepoGroups() },
  { read: 'the Slack members', path: '/api/slack/members', use: () => useSlackMembers('', true) },
  { read: 'the Slack groups', path: '/api/slack/groups', use: () => useSlackGroups(true) },
  {
    read: 'the keys',
    path: '/api/keys',
    use: () => {
      useReadKeys()
    },
  },
])('a read of $read the server failed in passing is asked once more', async (row) => {
  // Arrange
  const served = failedInPassing()
  fakeApi({ [row.path]: served.route })

  // Act
  renderHook(row.use, { wrapper: inClient() })

  // Assert
  await waitFor(
    () => {
      expect(served.reads()).toBe(2)
    },
    { timeout: 3000 },
  )
})
