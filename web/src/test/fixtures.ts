import type { Snapshot } from '@/api/generated/types.gen.ts'

// A complete, contract-valid snapshot for tests, with the fields a case cares
// about overridden. Kept here so every panel test starts from the same shape
// the stream actually pushes.
export function makeSnapshot(overrides: Partial<Snapshot> = {}): Snapshot {
  return {
    issues: { issues: [], total: 0, start_at: 0 },
    branch: {
      name: 'fix/PROJ-1',
      detached: false,
      head: 'abc1234',
      upstream: 'origin/fix/PROJ-1',
      ahead: 2,
      behind: 0,
      base: 'origin/main',
      commits: [],
    },
    changes: { changes: [] },
    review: { found: false },
    slack: { channel: '#dev', channels: [], author: 'octocat' },
    branches: [],
    ...overrides,
  }
}
