import { renderHook, waitFor } from '@testing-library/react'
import { beforeEach, vi } from 'vitest'
import { apiErrorMessage } from '@/api/apiError.ts'
import {
  addTask,
  assignIssue,
  changeStatus,
  getActivity,
  getAnnouncement,
  getChangeDiff,
  getCheckLog,
  getConfig,
  getDirectories,
  getHealth,
  getHookSetup,
  getIssue,
  getKeys,
  getLocalData,
  getMergeMethods,
  getPeople,
  getPullRequestDraft,
  getPullRequestText,
  getRepoGroups,
  getRepositories,
  getSlackGroups,
  getSlackMembers,
  listReviews,
  listStatusChanges,
  listTasks,
  listViews,
  logWork,
  previewBranchIssue,
  updateConfig,
} from '@/api/generated/sdk.gen.ts'
import { useEventStream, useSnapshotStore } from '@/api/snapshot.ts'
import { linkIssue, unlinkIssue } from '@/features/branch/branchIssueApi.ts'
import { commitChanges } from '@/features/branch/commitApi.ts'
import { startRun, stopRun, writeHookSetup } from '@/features/branch/gitRunApi.ts'
import { pushBranch } from '@/features/branch/pushApi.ts'
import {
  discardFile,
  stageEverything,
  stageFile,
  unstageFile,
} from '@/features/branch/stagingApi.ts'
import { checkoutBranch } from '@/features/issues/checkoutApi.ts'
import { startWork, startWorkInWorktree } from '@/features/issues/startWorkApi.ts'
import { announce, announceWhenCIPasses, stopWaiting } from '@/features/messaging/announceApi.ts'
import { savePerson } from '@/features/messaging/slackApi.ts'
import { linkOnIssue, moveToReview } from '@/features/review/followUpApi.ts'
import { openPr } from '@/features/review/openPrApi.ts'
import {
  editPull,
  finishBranch,
  mergePull,
  rerunChecks,
} from '@/features/review/reviewWritesApi.ts'
import { postSummary } from '@/features/summary/summaryApi.ts'
import { mockConfig } from './mockConfig.ts'
import { mockSnapshot } from './mockSnapshot.ts'

// The mockup's server stands in for workflow's: it answers the API's routes,
// in the contract's shapes, from the mockup's data, and pushes the mock
// snapshot down the event stream, so every section fills with no backend.

// installed is the mockup's server as a page load installs it, over network,
// which gets every request the server does not answer.
async function installed(
  network = vi.fn<typeof fetch>(() => Promise.reject(new TypeError('no network'))),
) {
  vi.stubGlobal('fetch', network)
  vi.stubGlobal('EventSource', globalThis.EventSource)
  const { installMockServer } = await import('./mockServer.ts')
  installMockServer()

  return network
}

// Each test meets the mockup as a page load finds it, before any write.
beforeEach(() => {
  vi.resetModules()
})

test("the mockup's stream pushes its snapshot, live, for the view asked", async () => {
  // Arrange
  await installed()

  // Act
  renderHook(() => {
    useEventStream('Team bugs', vi.fn())
  })

  // Assert
  await waitFor(() => {
    expect(useSnapshotStore.getState().status).toBe('live')
  })
  const { snapshot, view } = useSnapshotStore.getState()
  expect({ branch: snapshot?.branch.name, view }).toEqual({
    branch: mockSnapshot.branch.name,
    view: 'Team bugs',
  })
})

test('the mockup is a writable build', async () => {
  // Arrange
  await installed()

  // Act
  const { data } = await getHealth({ throwOnError: true })

  // Assert
  expect(data).toEqual({
    version: 'mockup',
    dry_run: false,
    forge_noun: 'pull request',
    forge_sigil: '#',
  })
})

test.each<[string, () => Promise<{ response: Response }>]>([
  ['the configuration', () => getConfig({ throwOnError: true })],
  ['the views', () => listViews({ throwOnError: true })],
  ['an issue', () => getIssue({ path: { key: 'PROJ-412' }, throwOnError: true })],
  [
    'its status changes',
    () => listStatusChanges({ path: { key: 'PROJ-412' }, throwOnError: true }),
  ],
  [
    "a period's activity",
    () => getActivity({ query: { from: '2026-09-14', to: '2026-09-18' }, throwOnError: true }),
  ],
  ['the last working day', () => getActivity({ throwOnError: true })],
  ['the repositories', () => getRepositories({ throwOnError: true })],
  ['a directory', () => getDirectories({ query: { path: '/home/ana/src' }, throwOnError: true })],
  ['where it works', () => getDirectories({ throwOnError: true })],
  ['the review queue', () => listReviews({ throwOnError: true })],
  ['the tasks', () => listTasks({ throwOnError: true })],
  ['the local data', () => getLocalData({ throwOnError: true })],
  ['the people', () => getPeople({ throwOnError: true })],
  ["the repository's groups", () => getRepoGroups({ throwOnError: true })],
  [
    "a channel's members",
    () => getSlackMembers({ query: { channel: '#releases' }, throwOnError: true }),
  ],
  ['the user groups', () => getSlackGroups({ throwOnError: true })],
  ['the announcement', () => getAnnouncement({ throwOnError: true })],
  ['the keys', () => getKeys({ throwOnError: true })],
  ['the pull request it would open', () => getPullRequestDraft({ throwOnError: true })],
  ["the pull request's text", () => getPullRequestText({ throwOnError: true })],
  ['the merge offer', () => getMergeMethods({ throwOnError: true })],
  ["a check's log", () => getCheckLog({ path: { id: '7' }, throwOnError: true })],
  ['a diff', () => getChangeDiff({ query: { path: 'internal/a.go' }, throwOnError: true })],
  ['the lefthook offer', () => getHookSetup({ throwOnError: true })],
  ["a link's preview", () => previewBranchIssue({ query: { key: '#7' }, throwOnError: true })],
])("the mockup reads %s in the contract's shape", async (_, read) => {
  // Arrange
  await installed()

  // Act
  const { response } = await read()

  // Assert
  expect(response.status).toBe(200)
})

test.each<[string, () => Promise<unknown>, Record<string, unknown>]>([
  [
    'a commit',
    () => commitChanges({ type: 'fix', subject: 'redact tokens' }),
    { head: 'd4e5f6a7' },
  ],
  ['a push', () => pushBranch(), { ahead: 0 }],
  ['a check-out', () => checkoutBranch('feat/PROJ-418'), { name: 'feat/PROJ-418' }],
  ['starting work', () => startWork('PROJ-401', true), { name: 'feat/PROJ-401', upstream: '' }],
  [
    'starting work in a worktree',
    () => startWorkInWorktree('PROJ-401', true),
    { branch: 'feat/PROJ-401', shown: '~/src/api-feat-PROJ-401' },
  ],
  ['linking the branch', () => linkIssue('#7', false), { issue_link: '7' }],
  ['unlinking it', () => unlinkIssue(), { issue_link: '' }],
  ['writing the lefthook offer', () => writeHookSetup(true), { scripts: 1 }],
  ['an announcement', () => announce('#releases', 'octocat opened #7'), { channel: '#releases' }],
  [
    'an edited announcement',
    () => announce('#releases', 'octocat opened #7', undefined, 'Review #7, please'),
    { channel: '#releases', text: 'Review #7, please' },
  ],
  [
    'an announcement held for CI',
    () => announceWhenCIPasses('#releases', 'octocat opened #7'),
    { held: true, channel: '#releases' },
  ],
  [
    'opening the pull request',
    () => openPr({ title: 'fix: redact tokens', base: 'main', draft: true }),
    { pull: { number: 42, title: 'fix: redact tokens', draft: true } },
  ],
  [
    'an edit of the pull request',
    () => editPull('fix: mask tokens', 'Why'),
    { title: 'fix: mask tokens' },
  ],
  ['a merge', () => mergePull('squash'), { state: 'merged' }],
  ['finishing the branch', () => finishBranch(), { name: 'main', commits: [] }],
  ['a re-run', () => rerunChecks(), { reran: true }],
  [
    'posting the summary',
    () => postSummary('2026-09-14', '2026-09-18', 'Did things', ''),
    { destination: 'the channel its webhook is bound to', text: 'Did things' },
  ],
  [
    'a status change',
    async () =>
      (
        await changeStatus({
          path: { key: 'PROJ-412' },
          body: { transition_id: '11', fields: [] },
          throwOnError: true,
        })
      ).data,
    { key: 'PROJ-412', status: 'Blocked' },
  ],
  [
    'an assignment',
    async () =>
      (
        await assignIssue({
          path: { key: 'PROJ-412' },
          body: { assignee: ' Ana ' },
          throwOnError: true,
        })
      ).data,
    { assignee: 'Ana' },
  ],
  [
    'logged work',
    async () =>
      (await logWork({ path: { key: 'PROJ-412' }, body: { time_spent: '1h' }, throwOnError: true }))
        .data,
    { time_spent: '1h' },
  ],
  [
    'a task added',
    async () => (await addTask({ body: { line: 'Water the plants' }, throwOnError: true })).data,
    { available: true },
  ],
])('the mockup answers %s as the server would', async (_, write, expected) => {
  // Arrange
  await installed()

  // Act
  const answered = await write()

  // Assert
  expect(answered).toMatchObject(expected)
})

test.each<[string, () => Promise<void>]>([
  ['staging a file', () => stageFile('internal/a.go')],
  ['unstaging one', () => unstageFile('internal/a.go')],
  ['staging everything', () => stageEverything()],
  ['a discard', () => discardFile('internal/a.go')],
  ['a stop', () => stopRun()],
  ['dropping a held announcement', () => stopWaiting()],
  ['a link on the issue', () => linkOnIssue('PROJ-412')],
  ['a move to review', () => moveToReview('PROJ-412')],
])('the mockup takes %s', async (_, write) => {
  // Arrange
  await installed()

  // Act & Assert
  await expect(write()).resolves.toBeUndefined()
})

test('the mockup streams a run that passes, line by line', async () => {
  // Arrange
  await installed()
  const lines: string[] = []

  // Act
  const ended = await startRun({ kind: 'pre_commit' }, (event) => {
    if (event.line !== undefined) {
      lines.push(event.line)
    }
  })

  // Assert
  expect(ended.state).toBe('succeeded')
  expect(lines).toHaveLength(2)
})

test('a link the mockup refuses says why, as the server words it', async () => {
  // Arrange
  await installed()

  // Act
  const caught: unknown = await savePerson({
    owner: 'ben',
    slack_id: 'S0POD',
    not_on_slack: false,
  }).catch((refused: unknown) => refused)

  // Assert
  expect(apiErrorMessage(caught, 'fallback')).toBe(
    'a team links to a Slack user group, and a person to a Slack user',
  )
})

test('a link the mockup keeps reads back', async () => {
  // Arrange
  await installed()
  await savePerson({ owner: 'ben', slack_id: 'U0BEN', not_on_slack: false })

  // Act
  const { data } = await getPeople({ throwOnError: true })

  // Assert
  expect(data.owners.find((owner) => owner.owner === 'ben')).toMatchObject({ state: 'linked' })
})

test('a configuration the mockup saves reads back, at a new revision', async () => {
  // Arrange
  await installed()
  const before = await getConfig({ throwOnError: true })
  const changed = { ...mockConfig, jira: { ...mockConfig.jira, base_url: 'https://jira.example' } }
  await updateConfig({ body: changed, throwOnError: true })

  // Act
  const after = await getConfig({ throwOnError: true })

  // Assert
  expect(after.data.jira.base_url).toBe('https://jira.example')
  expect(after.response.headers.get('ETag')).not.toBe(before.response.headers.get('ETag'))
})

test('a request the mockup does not answer goes on to the network', async () => {
  // Arrange
  const network = await installed(vi.fn<typeof fetch>(() => Promise.resolve(Response.json({}))))

  // Act
  await fetch('/api/config/setup')

  // Assert
  expect(network.mock.calls.map(([asked]) => new URL((asked as Request).url).pathname)).toEqual([
    '/api/config/setup',
  ])
})
