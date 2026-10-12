import { screen, waitFor } from '@testing-library/react'
import App from '@/App.tsx'
import type { IssueDetail, KeyAction, KeyList, Snapshot } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { mockConfig } from '@/dev/mockConfig.ts'
import { mockKeys } from '@/dev/mockKeys.ts'
import { useUiStore } from '@/shell/uiStore.ts'
import { fakeApi } from './fakeApi.ts'
import { makeSnapshot } from './fixtures.ts'
import { renderWithClient } from './renderWithClient.tsx'

// The page the keyboard's cases open, beside the keys the server lists for
// it: an issue PROJ-1 open in Issues, or a work tree with changes in Branch.
// Keyboard.test.tsx and its .sheet and .palette siblings share it.

// action is one action as the server lists it, on the keys given.
export function action(name: string, help: string, group: string, ...keys: string[]): KeyAction {
  return { action: name, help, group, shown: keys[0] ?? '', keys, default: keys[0] ?? '' }
}

const panes = ['1', '2', '3', '4', '5', '6', '7', '8', '9']

// shownIn is the section each pane is shown in, as the server lists them and
// the mockup keeps them: the Commits pane, the third, in Branch.
export const shownIn = mockKeys.pane_sections

// keysWith is the server's key list, with ui.keys' overrides applied as the
// terminal applies them, and the shortcut setting given.
export function keysWith(shortcuts: boolean, overrides: Record<string, string> = {}): KeyList {
  const listed = [
    { ...action('jump-to-pane', 'jump to pane', 'Moving around', ...panes), shown: '1-9' },
    action('change-status', 'change status', 'Issues', 't'),
    action('comment', 'comment', 'Issues', 'c'),
    action('search-issues', 'search', 'Issues', '/'),
    action('load-more', 'load more', 'Issues', 'ctrl+n'),
    action('rebase', 'rebase onto base', 'Branch', 'u'),
    action('stage-all', 'stage all', 'Commits', 'a'),
    action('unstage-all', 'unstage all', 'Commits', 'U'),
    action('commit', 'commit', 'Commits', 'c'),
    action('quit', 'quit', 'On every pane', 'q'),
    action('toggle-help', 'keys', 'On every pane', '?'),
  ]

  return {
    single_key_shortcuts: shortcuts,
    pane_sections: shownIn,
    actions: listed.map((each) => {
      const moved = overrides[each.action]

      return moved === undefined ? each : { ...each, shown: moved, keys: [moved] }
    }),
  }
}

export const issue: IssueDetail = {
  key: 'PROJ-1',
  tracker: 'jira',
  summary: 'Fix the token leak',
  status: 'In Progress',
  status_category: 'indeterminate',
  type: 'Bug',
  reporter: 'Ana Lopez',
  description: '',
  comments: [],
  comment_total: 0,
  url: '',
}

export const withIssue = makeSnapshot({
  issues: {
    total: 1,
    start_at: 0,
    unavailable: [],
    issues: [
      {
        key: issue.key,
        tracker: 'jira',
        summary: issue.summary,
        status: issue.status,
        status_category: 'indeterminate',
        type: 'Bug',
      },
    ],
  },
})

export const withChanges = makeSnapshot({
  changes: {
    changes: [
      { path: 'a.go', kind: 'modified', staged: true, has_unstaged: false, conflicted: false },
      { path: 'b.go', kind: 'untracked', staged: false, has_unstaged: true, conflicted: false },
    ],
  },
})

// openOn draws the page on a snapshot, in a section, with the server's keys,
// and an issue PROJ-1 open in Issues, and returns every request it sends.
export function openOn(snapshot: Snapshot, keys: KeyList, section: 'issues' | 'branch' = 'issues') {
  const requests = fakeApi({
    '/api/keys': keys,
    '/api/config': mockConfig,
    '/api/views': { views: [] },
    '/api/issues/PROJ-1': issue,
    '/api/stage': { changes: [] },
    '/api/unstage': { changes: [] },
  })
  useSnapshotStore.setState({ status: 'live', snapshot })
  useUiStore.setState({ section, selectedIssue: issue })
  renderWithClient(<App />)

  return requests
}

// commentBox is PROJ-1's comment box, once its detail is drawn.
export function commentBox(): Promise<HTMLElement> {
  return screen.findByRole('textbox', { name: 'Comment on PROJ-1' })
}

// keysRead waits for the server's keys to reach the page: the comment box names
// its key once they have.
export async function keysRead(key: string): Promise<void> {
  await waitFor(async () => {
    expect((await commentBox()).getAttribute('aria-keyshortcuts')).toBe(key)
  })
}
