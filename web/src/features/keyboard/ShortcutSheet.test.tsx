import { render, screen, within } from '@testing-library/react'
import type { KeyAction } from '@/api/generated/types.gen.ts'
import { useKeysStore } from './keysApi.ts'
import { ShortcutSheet } from './ShortcutSheet.tsx'

// The sheet lists each of the terminal's panes' actions under the pane's own
// heading, as the terminal's ? does: two panes are never live at once, so a
// ui.keys map may give both the same key, and each lists it once.

// action is one action as the server lists it, on the key given.
function action(name: string, help: string, group: string, key: string): KeyAction {
  return { action: name, help, group, shown: key, keys: [key], default: key }
}

test('a key two panes share is listed once under each pane', () => {
  // Arrange
  // ui.keys moves commit onto P, where the Branch pane's push already is.
  useKeysStore.setState({
    shortcuts: true,
    actions: [
      action('push', 'push', 'Branch', 'P'),
      action('commit', 'commit', 'Commits', 'P'),
      action('merge', 'merge', 'Review', 'M'),
    ],
  })

  // Act
  render(<ShortcutSheet helpKey="?" onClose={() => {}} onSettings={() => {}} />)

  // Assert
  const sheet = screen.getByRole('dialog', { name: 'Keyboard shortcuts' })
  const branch = within(sheet).getByRole('table', { name: 'Branch' })
  const commits = within(sheet).getByRole('table', { name: 'Commits' })
  expect(
    within(branch)
      .getAllByRole('row')
      .map((row) => row.textContent),
  ).toEqual(['Ppush'])
  expect(
    within(commits)
      .getAllByRole('row')
      .map((row) => row.textContent),
  ).toEqual(['Pcommit'])
})
