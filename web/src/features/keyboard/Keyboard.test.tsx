import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import App from '@/App.tsx'
import type { IssueDetail, KeyAction, KeyList, Snapshot } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { mockConfig } from '@/dev/mockConfig.ts'
import { useUiStore } from '@/shell/uiStore.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'

// The page's keyboard beyond Tab, as a user meets it: ? lists the keys, a key
// does what the terminal's does where single-key shortcuts are on, and Ctrl+K
// or ⌘K finds an action by name.

// action is one action as the server lists it, on the keys given.
function action(name: string, help: string, group: string, ...keys: string[]): KeyAction {
  return { action: name, help, group, shown: keys[0] ?? '', keys, default: keys[0] ?? '' }
}

const panes = ['1', '2', '3', '4', '5', '6', '7', '8', '9']

// keysWith is the server's key list, with ui.keys' overrides applied as the
// terminal applies them, and the shortcut setting given.
function keysWith(shortcuts: boolean, overrides: Record<string, string> = {}): KeyList {
  const listed = [
    { ...action('jump-to-pane', 'jump to pane', 'Moving around', ...panes), shown: '1-9' },
    action('change-status', 'change status', 'Issues', 't'),
    action('comment', 'comment', 'Issues', 'c'),
    action('search-issues', 'search', 'Issues', '/'),
    action('load-more', 'load more', 'Issues', 'ctrl+n'),
    action('stage-all', 'stage all', 'Branch and Commits', 'a'),
    action('unstage-all', 'unstage all', 'Branch and Commits', 'U'),
    action('commit', 'commit', 'Branch and Commits', 'c'),
    action('rebase', 'rebase onto base', 'Branch and Commits', 'u'),
    action('quit', 'quit', 'Everywhere', 'q'),
    action('toggle-help', 'keys', 'Everywhere', '?'),
  ]

  return {
    single_key_shortcuts: shortcuts,
    actions: listed.map((each) => {
      const moved = overrides[each.action]

      return moved === undefined ? each : { ...each, shown: moved, keys: [moved] }
    }),
  }
}

const issue: IssueDetail = {
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

const withIssue = makeSnapshot({
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

const withChanges = makeSnapshot({
  changes: {
    changes: [
      { path: 'a.go', kind: 'modified', staged: true, has_unstaged: false, conflicted: false },
      { path: 'b.go', kind: 'untracked', staged: false, has_unstaged: true, conflicted: false },
    ],
  },
})

// openOn draws the page on a snapshot, in a section, with the server's keys,
// and an issue PROJ-1 open in Issues, and returns every request it sends.
function openOn(snapshot: Snapshot, keys: KeyList, section: 'issues' | 'branch' = 'issues') {
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
function commentBox(): Promise<HTMLElement> {
  return screen.findByRole('textbox', { name: 'Comment on PROJ-1' })
}

// keysRead waits for the server's keys to reach the page: the comment box names
// its key once they have.
async function keysRead(key: string): Promise<void> {
  await waitFor(async () => {
    expect((await commentBox()).getAttribute('aria-keyshortcuts')).toBe(key)
  })
}

test('? opens a sheet listing comment under Issues on c', async () => {
  // Arrange
  const user = userEvent.setup()
  openOn(withIssue, keysWith(true))
  await keysRead('c')

  // Act
  await user.keyboard('?')

  // Assert
  const sheet = screen.getByRole('dialog', { name: 'Keyboard shortcuts' })
  const issues = within(sheet).getByRole('table', { name: 'Issues' })
  expect(within(issues).getByRole('row', { name: 'c comment' })).toBeTruthy()
  expect(within(sheet).queryByText('quit')).toBeNull()
})

test('? lists post under Summary on p', async () => {
  // Arrange
  const user = userEvent.setup()
  const keys = keysWith(true)
  openOn(withIssue, {
    ...keys,
    actions: [...keys.actions, action('post-summary', 'post', 'Summary', 'p')],
  })
  await keysRead('c')

  // Act
  await user.keyboard('?')

  // Assert
  const summary = within(screen.getByRole('dialog')).getByRole('table', { name: 'Summary' })
  expect(within(summary).getByRole('row', { name: 'p post' })).toBeTruthy()
})

test('the sheet shows a key ui.keys moved where it now is', async () => {
  // Arrange
  const user = userEvent.setup()
  openOn(withIssue, keysWith(true, { comment: 'C' }))
  await keysRead('Shift+C')

  // Act
  await user.keyboard('?')

  // Assert
  const issues = within(screen.getByRole('dialog')).getByRole('table', { name: 'Issues' })
  expect(within(issues).getByRole('row', { name: 'C comment' })).toBeTruthy()
})

test('Escape closes the sheet and hands the focus back', async () => {
  // Arrange
  const user = userEvent.setup()
  openOn(withIssue, keysWith(true))
  const box = await commentBox()
  const search = screen.getByRole('searchbox', { name: 'Search' })
  search.focus()
  await user.keyboard('{Escape}')
  screen.getByRole('button', { name: /Fix the token leak/ }).focus()
  await user.keyboard('?')
  expect(screen.getByRole('dialog')).toBeTruthy()

  // Act
  await user.keyboard('{Escape}')

  // Assert
  expect(screen.queryByRole('dialog')).toBeNull()
  expect(document.activeElement).toBe(screen.getByRole('button', { name: /Fix the token leak/ }))
  expect(document.activeElement).not.toBe(box)
})

test('c puts the focus in the comment box', async () => {
  // Arrange
  const user = userEvent.setup()
  openOn(withIssue, keysWith(true))
  await keysRead('c')

  // Act
  await user.keyboard('c')

  // Assert
  expect(document.activeElement).toBe(await commentBox())
  expect((await commentBox()).textContent).toBe('')
})

test('typing c in the search box only types it', async () => {
  // Arrange
  const user = userEvent.setup()
  openOn(withIssue, keysWith(true))
  await keysRead('c')
  const search = screen.getByRole('searchbox', { name: 'Search' })

  // Act
  await user.type(search, 'c')

  // Assert
  expect((search as HTMLInputElement).value).toBe('c')
  expect(document.activeElement).toBe(search)
  expect(screen.queryByRole('dialog')).toBeNull()
})

test('with single-key shortcuts off, c does nothing and Ctrl+K still opens the palette', async () => {
  // Arrange
  const user = userEvent.setup()
  openOn(withIssue, keysWith(false))
  const box = await commentBox()
  await waitFor(() => {
    expect(screen.getByRole('searchbox', { name: 'Search' })).toBeTruthy()
  })

  // Act: press c
  await user.keyboard('c')

  // Assert: the box did not take the focus, and names no key
  expect(document.activeElement).not.toBe(box)
  expect(box.hasAttribute('aria-keyshortcuts')).toBe(false)

  // Act: press Ctrl+K
  await user.keyboard('{Control>}k{/Control}')

  // Assert: the palette opened
  expect(document.activeElement).toBe(screen.getByRole('combobox', { name: 'Action' }))
})

test('⌘K opens the palette too', async () => {
  // Arrange
  const user = userEvent.setup()
  openOn(withIssue, keysWith(false))
  await commentBox()

  // Act
  await user.keyboard('{Meta>}k{/Meta}')

  // Assert
  expect(screen.getByRole('dialog', { name: 'Command palette' })).toBeTruthy()
})

test('Ctrl+K, "stage all", Enter stages every change', async () => {
  // Arrange
  const user = userEvent.setup()
  const requests = openOn(withChanges, keysWith(true), 'branch')
  await waitFor(() => {
    expect(
      screen.getByRole('button', { name: 'Stage all' }).getAttribute('aria-keyshortcuts'),
    ).toBe('a')
  })

  // Act
  await user.keyboard('{Control>}k{/Control}')
  await user.keyboard('stage all')
  await user.keyboard('{Enter}')

  // Assert
  expect(await screen.findByText('Staged every change.')).toBeTruthy()
  const staged = requests.find((request) => new URL(request.url).pathname === '/api/stage')
  expect(await staged?.json()).toEqual({ all: true })
  expect(screen.queryByRole('dialog')).toBeNull()
})

test('no single key reaches a control behind an open confirm', async () => {
  // Arrange
  const user = userEvent.setup()
  const requests = openOn(withChanges, keysWith(true), 'branch')
  await waitFor(() => {
    expect(
      screen.getByRole('button', { name: 'Stage all' }).getAttribute('aria-keyshortcuts'),
    ).toBe('a')
  })
  await user.click(screen.getByRole('button', { name: 'Discard b.go…' }))
  screen.getByRole('group', { name: 'Discard the changes to b.go?' })

  // Act
  await user.keyboard('a')

  // Assert
  expect(requests.some((request) => new URL(request.url).pathname === '/api/stage')).toBe(false)
  expect(screen.queryByText('Staged every change.')).toBeNull()
})

test('the palette lists the section actions on screen by the terminal words, closest first', async () => {
  // Arrange
  const user = userEvent.setup()
  openOn(withChanges, keysWith(true), 'branch')
  await waitFor(() => {
    expect(
      screen.getByRole('button', { name: 'Stage all' }).hasAttribute('aria-keyshortcuts'),
    ).toBe(true)
  })
  await user.keyboard('{Control>}k{/Control}')

  // Act
  await user.keyboard('stage')

  // Assert
  const options = within(screen.getByRole('listbox', { name: 'Actions' }))
    .getAllByRole('option')
    .map((option) => option.textContent)
  expect(options).toEqual(['stage alla', 'unstage allU'])
})

test('a pane number opens its section', async () => {
  // Arrange
  const user = userEvent.setup()
  openOn(withIssue, keysWith(true))
  await keysRead('c')

  // Act
  await user.keyboard('2')

  // Assert
  expect(await screen.findByRole('heading', { level: 1, name: 'Branch' })).toBeTruthy()
})

test('turning single-key shortcuts on in Settings makes c work at once', async () => {
  // Arrange
  const user = userEvent.setup()
  let saved = { ...mockConfig, ui: { ...mockConfig.ui, web_shortcuts: false } }
  fakeApi({
    '/api/keys': () => keysWith(saved.ui.web_shortcuts),
    '/api/config': async (_: URL, request: Request) => {
      if (request.method === 'PUT') {
        saved = (await request.json()) as typeof saved
      }

      return saved
    },
    '/api/views': { views: [] },
    '/api/issues/PROJ-1': issue,
  })
  useSnapshotStore.setState({ status: 'live', snapshot: withIssue })
  useUiStore.setState({ section: 'settings', selectedIssue: issue })
  renderWithClient(<App />)
  await user.click(await screen.findByRole('checkbox', { name: 'Single-key shortcuts' }))
  await user.click(screen.getByRole('button', { name: 'Save changes' }))
  await screen.findByText('Saved.')
  await user.click(
    within(screen.getByRole('navigation', { name: 'Sections' })).getByRole('button', {
      name: 'Issues',
    }),
  )
  await keysRead('c')

  // Act
  await user.keyboard('c')

  // Assert
  expect(saved.ui.web_shortcuts).toBe(true)
  expect(document.activeElement).toBe(await commentBox())
})

test('the sheet offers to turn single-key shortcuts on while they are off', async () => {
  // Arrange
  const user = userEvent.setup()
  openOn(withIssue, keysWith(false))
  await commentBox()
  await user.keyboard('?')

  // Act
  await user.click(screen.getByRole('button', { name: 'Turn them on in Settings' }))

  // Assert
  expect(screen.queryByRole('dialog')).toBeNull()
  expect(await screen.findByRole('heading', { level: 1, name: 'Settings' })).toBeTruthy()
})

test('an action on a key the page cannot bind is listed as the palette', async () => {
  // Arrange
  const user = userEvent.setup()
  openOn(withIssue, keysWith(true))
  await keysRead('c')

  // Act
  await user.keyboard('?')

  // Assert
  const issues = within(screen.getByRole('dialog')).getByRole('table', { name: 'Issues' })
  expect(within(issues).getByRole('row', { name: 'palette load more' })).toBeTruthy()
})

test('a press on the backdrop closes the sheet', async () => {
  // Arrange
  const user = userEvent.setup()
  openOn(withIssue, keysWith(true))
  await keysRead('c')
  await user.keyboard('?')

  // Act
  await user.click(screen.getByRole('dialog'))

  // Assert
  expect(screen.queryByRole('dialog')).toBeNull()
})

test('the rail names the pane numbers that open each section', async () => {
  // Arrange
  openOn(withIssue, keysWith(true))
  await keysRead('c')

  // Act
  const rail = screen.getByRole('navigation', { name: 'Sections' })

  // Assert
  expect(
    within(rail).getByRole('button', { name: 'Branch' }).getAttribute('aria-keyshortcuts'),
  ).toBe('2 3')
  expect(
    within(rail).getByRole('button', { name: 'Settings' }).hasAttribute('aria-keyshortcuts'),
  ).toBe(false)
})

test('Down and Up step through the matches, round from the last to the first', async () => {
  // Arrange
  const user = userEvent.setup()
  openOn(withChanges, keysWith(true), 'branch')
  await waitFor(() => {
    expect(
      screen.getByRole('button', { name: 'Stage all' }).hasAttribute('aria-keyshortcuts'),
    ).toBe(true)
  })
  await user.keyboard('{Control>}k{/Control}')
  await user.keyboard('stage')
  const box = screen.getByRole('combobox', { name: 'Action' })
  const chosen = () =>
    document.getElementById(box.getAttribute('aria-activedescendant') ?? '')?.textContent

  // Act: step down
  await user.keyboard('{ArrowDown}')

  // Assert: the second match is chosen
  expect(chosen()).toBe('unstage allU')

  // Act: step down again
  await user.keyboard('{ArrowDown}')

  // Assert: round to the first
  expect(chosen()).toBe('stage alla')

  // Act: step up
  await user.keyboard('{ArrowUp}')

  // Assert: round to the last
  expect(chosen()).toBe('unstage allU')
})

test('a press on an option runs it', async () => {
  // Arrange
  const user = userEvent.setup()
  const requests = openOn(withChanges, keysWith(true), 'branch')
  await waitFor(() => {
    expect(
      screen.getByRole('button', { name: 'Unstage all' }).hasAttribute('aria-keyshortcuts'),
    ).toBe(true)
  })
  await user.keyboard('{Control>}k{/Control}')

  // Act
  await user.click(screen.getByRole('option', { name: /^unstage all/ }))

  // Assert
  expect(await screen.findByText('Unstaged every change.')).toBeTruthy()
  expect(requests.some((request) => new URL(request.url).pathname === '/api/unstage')).toBe(true)
})

test('the palette goes to another section by its name', async () => {
  // Arrange
  const user = userEvent.setup()
  openOn(withChanges, keysWith(true), 'branch')
  await screen.findByRole('button', { name: 'Stage all' })
  await user.keyboard('{Control>}k{/Control}')

  // Act
  await user.keyboard('issues{Enter}')

  // Assert
  expect(await screen.findByRole('heading', { level: 1, name: 'Issues' })).toBeTruthy()
})

test('the palette says when nothing is called what was typed', async () => {
  // Arrange
  const user = userEvent.setup()
  openOn(withChanges, keysWith(true), 'branch')
  await screen.findByRole('button', { name: 'Stage all' })
  await user.keyboard('{Control>}k{/Control}')

  // Act
  await user.keyboard('frobnicate{Enter}')

  // Assert
  const palette = screen.getByRole('dialog', { name: 'Command palette' })
  expect(within(palette).getByRole('status').textContent).toMatch(/Nothing here is called that/)
})
