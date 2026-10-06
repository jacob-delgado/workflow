import { vi } from 'vitest'
import { commitChanges } from '@/features/branch/commitApi.ts'
import { pushBranch } from '@/features/branch/pushApi.ts'
import { checkoutBranch } from '@/features/issues/checkoutApi.ts'
import { startWork } from '@/features/issues/startWorkApi.ts'
import { announce, announceWhenCIPasses } from '@/features/messaging/announceApi.ts'

// Under VITE_MOCK the mockup has no server behind it, so each write that says
// what it did answers with a canned value of its own shape, and asks nothing.
const writes: [string, () => Promise<unknown>, Record<string, unknown>][] = [
  [
    'a commit',
    () => commitChanges({ type: 'fix', subject: 'redact tokens' }),
    { head: 'd4e5f6a7' },
  ],
  ['a push', pushBranch, { ahead: 0 }],
  ['a check-out', () => checkoutBranch('feat/PROJ-418'), { name: 'feat/PROJ-418' }],
  ['starting work', () => startWork('PROJ-401', true), { name: 'feat/PROJ-401', upstream: '' }],
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
]

test.each(writes)('the mockup answers %s without a server', async (_, write, expected) => {
  // Arrange
  vi.stubEnv('VITE_MOCK', 'true')

  // Act
  const answered = await write()

  // Assert
  expect(answered).toMatchObject(expected)
  expect(globalThis.fetch).not.toHaveBeenCalled()
})
