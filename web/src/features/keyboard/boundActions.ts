// The terminal's actions this page has a control for, by the names ui.keys
// gives them: the ? sheet lists these, and only these, under the terminal's
// groups. The keys come from the server; this says only which actions a
// control here answers to. An action a section binds registers its control
// with useShortcut. Left out on purpose: discard-change, the terminal's x on
// the change its cursor is on, since the page selects no change row — each
// change carries its own Discard… button instead. The server keeps the same
// list as tui.WebBoundActions, and a Go test holds the two to one: the key
// check refuses a key two panes shown in one section share only when the page
// binds both actions.
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
  'post-summary',
  'favorite-directory',
  'go-to-directory',
  'toggle-help',
])

// The two actions the keyboard layer answers itself rather than through a
// control: the pane numbers, and the sheet.
export const jumpAction = 'jump-to-pane'
export const helpAction = 'toggle-help'
