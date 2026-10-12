import { render, screen, within } from '@testing-library/react'
import type { KeyAction } from '@/api/generated/types.gen.ts'
import { useKeysStore } from './keysApi.ts'
import { ShortcutSheet } from './ShortcutSheet.tsx'

// The sheet lists each of the terminal's panes' actions under the pane's own
// heading, as the terminal's ? does. Two panes no surface shows together are
// never live at once, so a ui.keys map may give both the same key, and each
// lists it once; the terminal refuses a map that gives one key to two actions
// this page binds on panes a section here shows together, as Branch shows
// Commits.

// action is one action as the server lists it, on the key given.
function action(name: string, help: string, group: string, key: string): KeyAction {
  return { action: name, help, group, shown: key, keys: [key], default: key }
}

test('a key two panes share is listed once under each pane', () => {
  // Arrange
  // ui.keys moves sort-tasks onto P, where the Branch pane's push already is.
  useKeysStore.setState({
    shortcuts: true,
    actions: [
      action('push', 'push', 'Branch', 'P'),
      action('sort-tasks', 'sort', 'Tasks', 'P'),
      action('merge', 'merge', 'Review', 'M'),
    ],
  })

  // Act
  render(<ShortcutSheet helpKey="?" onClose={() => {}} onSettings={() => {}} />)

  // Assert
  const sheet = screen.getByRole('dialog', { name: 'Keyboard shortcuts' })
  const branch = within(sheet).getByRole('table', { name: 'Branch' })
  const tasks = within(sheet).getByRole('table', { name: 'Tasks' })
  expect(
    within(branch)
      .getAllByRole('row')
      .map((row) => row.textContent),
  ).toEqual(['Ppush'])
  expect(
    within(tasks)
      .getAllByRole('row')
      .map((row) => row.textContent),
  ).toEqual(['Psort'])
})
