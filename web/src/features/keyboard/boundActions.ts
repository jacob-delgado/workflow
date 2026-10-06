import type { Section } from '@/shell/uiStore.ts'

// The terminal's actions this page has a control for, by the names ui.keys
// gives them: the ? sheet lists these, and only these, under the terminal's
// groups. The keys come from the server; this says only which actions a
// control here answers to. An action a section binds registers its control
// with useShortcut.
export const boundActions: ReadonlySet<string> = new Set([
  'jump-to-pane',
  'change-status',
  'comment',
  'assign',
  'log-work',
  'search-issues',
  'switch-view',
  'load-more',
  'open-link',
  'track-issue',
  'link-issue',
  'rebase',
  'push',
  'stage-all',
  'unstage-all',
  'commit',
  'amend',
  'fixup',
  'run-pre-commit',
  'set-up-lefthook',
  'open-pull-request',
  'rerun-checks',
  'merge',
  'finish-branch',
  'post',
  'sort-reviews',
  'start-stop',
  'mark-done',
  'add-task',
  'annotate-task',
  'modify-task',
  'undo-task',
  'sync-tasks',
  'search-tasks',
  'sort-tasks',
  'earlier',
  'later',
  'today',
  'copy-summary',
  'favorite-directory',
  'go-to-directory',
  'toggle-help',
])

// The two actions the keyboard layer answers itself rather than through a
// control: the pane numbers, and the sheet.
export const jumpAction = 'jump-to-pane'
export const helpAction = 'toggle-help'

// paneSections is the section each of the terminal's pane numbers opens, in
// the terminal's pane order (internal/tui/panes.go): its Commits pane, the
// third, is part of Branch here. Settings is no pane there, and has no number.
export const paneSections: readonly Section[] = [
  'issues',
  'branch',
  'branch',
  'review',
  'messaging',
  'reviews',
  'tasks',
  'summary',
  'repositories',
]
