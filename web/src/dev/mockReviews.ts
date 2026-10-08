import type {
  JobLog,
  PullRequestDraft,
  PullRequestText,
  ReviewQueue,
} from '@/api/generated/types.gen.ts'

const hour = 3_600_000

// opened is the RFC 3339 time some hours before now, so the mockup's ages read
// the same whenever it is shown.
function opened(hoursAgo: number): string {
  return new Date(Date.now() - hoursAgo * hour).toISOString()
}

// mockReviewQueue is the review queue `task web:mockup` lists: pull requests
// across repositories, in every CI state, one a draft, waiting from under an
// hour to over a week — oldest first, as the server sends it. Dev-only, and
// code-split out of a production build.
export function mockReviewQueue(): ReviewQueue {
  return {
    available: true,
    requests: [
      {
        number: 377,
        url: 'https://github.com/acme/workflow/pull/377',
        title: 'Retry the forge search once on a gateway timeout',
        author: 'sam.ortiz',
        repository: 'acme/workflow',
        draft: false,
        ci: 'failed',
        opened_at: opened(9 * 24),
      },
      {
        number: 88,
        url: 'https://github.com/acme/build-images/pull/88',
        title: 'Pin the Go toolchain in the build image',
        author: 'ana.lopez',
        repository: 'acme/build-images',
        draft: false,
        ci: 'passed',
        opened_at: opened(50),
      },
      {
        number: 391,
        url: 'https://github.com/acme/workflow/pull/391',
        title: 'List the review queue on the web',
        author: 'lee.chen',
        repository: 'acme/workflow',
        draft: true,
        ci: 'running',
        opened_at: opened(3),
      },
      {
        number: 12,
        url: 'https://github.com/acme/handbook/pull/12',
        title: 'Document the release checklist',
        author: 'mia.okafor',
        repository: 'acme/handbook',
        draft: false,
        ci: 'none',
        opened_at: opened(0.5),
      },
    ],
  }
}

// mockPullDraft is the pull request the mockup would open for its branch,
// which has yet to be pushed, with no template to start from.
export const mockPullDraft: PullRequestDraft = {
  title: 'fix: redact tokens before they reach the request log',
  body: '## Commits\n\n- fix: redact tokens before they reach the request log\n\nPROJ-412',
  base: 'main',
  head: 'fix/PROJ-412',
  draft: false,
  needs_push: true,
  reviewers: ['ana', 'acme/control-plane'],
  templates: [],
  template: '',
}

// mockPullText is the open pull request's title and description, as the
// editor reads them afresh.
export const mockPullText: PullRequestText = {
  title: 'fix: redact tokens in the request log',
  body: '## Why\n\nTokens reached the log.',
}

// mockCheckLog is a failed check's log, cut short.
export const mockCheckLog: JobLog = {
  text: '--- FAIL: TestRetry (0.01s)\n    retry_test.go:41: got 4, want 3\nFAIL',
  truncated: true,
}
