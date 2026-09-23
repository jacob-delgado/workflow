import type { Health, Snapshot } from '@/api/generated/types.gen.ts'

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
    messaging: {
      service: 'Slack',
      configured: true,
      channel: '#dev',
      channels: [],
      author: 'octocat',
    },
    branches: [],
    suggested_scope: '',
    ...overrides,
  }
}

// A contract-valid health answer for tests: a writable server on a forge that
// says "pull request", with the fields a case cares about overridden.
export function makeHealth(overrides: Partial<Health> = {}): Health {
  return {
    version: '1.2.3',
    dry_run: false,
    forge_noun: 'pull request',
    forge_sigil: '#',
    ...overrides,
  }
}

// What a server on a GitLab remote says in its health: GitLab's own words for a
// proposed change and the mark before its number.
export const gitLabWords: Partial<Health> = { forge_noun: 'merge request', forge_sigil: '!' }
