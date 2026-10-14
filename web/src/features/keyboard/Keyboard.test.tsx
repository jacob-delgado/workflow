import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import App from '@/App.tsx'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { mockConfig } from '@/dev/mockConfig.ts'
import { useUiStore } from '@/shell/uiStore.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import {
  shownIn,
  keysWith,
  issue,
  withIssue,
  withChanges,
  openOn,
  commentBox,
  keysRead,
} from '@/test/keyboard.tsx'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { inSection } from '@/test/settings.ts'

// The page's keyboard beyond Tab, as a user meets it: where single-key
// shortcuts are on, a key does what the terminal's does, and a pane number
// opens the section that pane is shown in. The ? sheet's cases are in
// Keyboard.sheet.test.tsx, and Ctrl+K's palette's in Keyboard.palette.test.tsx.

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
  await waitFor(() => {
    expect(document.activeElement).toBe(screen.getByRole('combobox', { name: 'Action' }))
  })
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
  await inSection('Terminal and keys')
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

test('the pane numbers open the sections the server says the panes are shown in', async () => {
  // Arrange
  // A server that showed the Review pane in the Branch section too.
  const user = userEvent.setup()
  const reviewInBranch = shownIn.map((section) => (section === 'review' ? 'branch' : section))
  openOn(withIssue, { ...keysWith(true), pane_sections: reviewInBranch })
  await keysRead('c')

  // Act
  await user.keyboard('4')

  // Assert
  expect(await screen.findByRole('heading', { level: 1, name: 'Branch' })).toBeTruthy()
  const rail = screen.getByRole('navigation', { name: 'Sections' })
  expect(
    within(rail).getByRole('button', { name: 'Branch' }).getAttribute('aria-keyshortcuts'),
  ).toBe('2 3 4')
})
