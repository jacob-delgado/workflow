import type { LocalData, LocalDataFile } from '@/api/generated/types.gen.ts'
import { mockForgetAll } from './mockSlack.ts'

// The mockup's store: a cache a few sessions have filled, and the kept
// associations of a repository whose owners were asked about.

const dir = '/home/ana/.local/state/workflow'

const cache: LocalDataFile = {
  name: 'workflow.db',
  kind: 'cache',
  bytes: 94_208,
  holds: [
    { what: 'scopes', count: 3 },
    { what: 'announcements', count: 5 },
    { what: 'cached views', count: 2 },
    { what: 'cached issues', count: 41 },
  ],
}

const kept: LocalDataFile = {
  name: 'kept.db',
  kind: 'kept',
  bytes: 24_576,
  holds: [
    { what: 'owner decisions', count: 4 },
    { what: 'owners on Slack', count: 3 },
    { what: 'Slack users and groups', count: 5 },
    { what: 'repository groups', count: 2 },
    { what: 'chosen groups', count: 1 },
  ],
}

// held is what the mockup's store still holds, so a removal shows on the read
// after it, as it would against a server, until the page loads again.
const held = { files: [cache, kept] }

// mockLocalData is the mockup's store as it stands.
export function mockLocalData(): LocalData {
  return { dir, files: [...held.files] }
}

// mockRemoveLocalData forgets what a removal of scope reaches — with the kept
// file, the mockup's people and groups — and answers what is left.
export function mockRemoveLocalData(scope: 'cache' | 'all'): LocalData {
  held.files = scope === 'all' ? [] : held.files.filter((file) => file.kind === 'kept')
  if (scope === 'all') {
    mockForgetAll()
  }

  return mockLocalData()
}
