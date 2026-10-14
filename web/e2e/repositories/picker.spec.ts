import type { Page } from '@playwright/test'
import { mockDirectories, mockRepositories } from '../../src/dev/mockRepositories.ts'
import { height, openSection, pinTheme, themes, widths } from '../support/cockpit.ts'
import { expectReachableAndClean } from '../support/reachable.ts'
import { expect, problem, test } from '../support/fixtures.ts'

// The Repositories section: where the server works, the favorites, the
// directory dialog, and a switch asked once more in it before it is made.

// opensRepositories answers the section's reads as the mockup does, keeps
// each switch asked, answering it with where the mockup shows that directory,
// and opens the section.
async function opensRepositories(page: Page): Promise<string[]> {
  const switched: string[] = []
  await page.route('**/api/repositories', (route) => route.fulfill({ json: mockRepositories() }))
  await page.route('**/api/directories**', (route) => {
    const asked = new URL(route.request().url()).searchParams

    return route.fulfill({
      json: mockDirectories(asked.get('path') ?? '.', asked.get('prefix') ?? ''),
    })
  })
  await page.route('**/api/repositories/here', (route) => {
    const asked = route.request().postDataJSON() as { dir: string }
    switched.push(asked.dir)
    const after = mockRepositories()
    const known = [...after.favorites, ...after.worktrees].find((place) => place.dir === asked.dir)
    if (known === undefined) {
      return route.fulfill(problem('not_found', `${asked.dir} is not a directory`))
    }

    return route.fulfill({
      json: { ...after, here: { ...after.here, dir: known.dir, shown: known.shown } },
    })
  })
  await page.goto('/')
  await openSection(page, 'Repositories')
  await expect(page.getByRole('region', { name: 'Working in' })).toBeVisible()

  return switched
}

// goTo opens the directory dialog and types a path into its field.
async function goTo(page: Page, typed: string): Promise<void> {
  await page.getByRole('button', { name: 'Go to a directory…' }).click()
  const field = page.getByRole('combobox', { name: 'Directory' })
  await expect(field).toBeFocused()
  await field.fill(typed)
}

test('a favorite is switched to once the switch is confirmed', async ({ page }) => {
  // Arrange
  const switched = await opensRepositories(page)
  await page.getByRole('button', { name: 'Switch to ~/src/web' }).click()

  // Act
  await page
    .getByRole('dialog', { name: 'Go to a directory' })
    .getByRole('group', { name: 'Switch to ~/src/web?' })
    .getByRole('button', { name: 'Switch' })
    .click()

  // Assert
  await expect(page.getByText('Switched to ~/src/web.')).toBeVisible()
  expect(switched).toEqual(['/home/ana/src/web'])
})

test('another worktree is switched to once the switch is confirmed', async ({ page }) => {
  // Arrange
  const switched = await opensRepositories(page)
  await page
    .getByRole('list', { name: 'Worktrees' })
    .getByRole('button', { name: 'Switch to ~/src/api-review' })
    .click()

  // Act
  await page
    .getByRole('group', { name: 'Switch to ~/src/api-review?' })
    .getByRole('button', { name: 'Switch' })
    .click()

  // Assert
  await expect(page.getByText('Switched to ~/src/api-review.')).toBeVisible()
  expect(switched).toEqual(['/home/ana/src/api-review'])
})

test('a name typed after a path narrows its directories, and Enter goes into one', async ({
  page,
}) => {
  // Arrange
  await opensRepositories(page)
  await goTo(page, '~/src/w')
  await expect(page.getByRole('option', { name: /^web/ })).toBeVisible()

  // Act
  await page.keyboard.press('Enter')

  // Assert
  await expect(page.getByRole('combobox', { name: 'Directory' })).toHaveValue('~/src/web/')
  await expect(page.getByRole('button', { name: 'Switch to ~/src/web' })).toBeVisible()
})

test('Backspace after a slash goes up a directory', async ({ page }) => {
  // Arrange
  await opensRepositories(page)
  await goTo(page, '~/src/api/')
  await expect(page.getByRole('option', { name: /^cmd/ })).toBeVisible()

  // Act
  await page.keyboard.press('Backspace')

  // Assert
  await expect(page.getByRole('combobox', { name: 'Directory' })).toHaveValue('~/src/')
})

test('a directory browsed to is asked once more in the dialog', async ({ page }) => {
  // Arrange
  await opensRepositories(page)
  await goTo(page, '~/src/')

  // Act
  await page.getByRole('button', { name: 'Switch to ~/src', exact: true }).click()

  // Assert
  await expect(
    page.getByRole('dialog', { name: 'Go to a directory' }).getByRole('group', {
      name: 'Switch to ~/src?',
    }),
  ).toBeVisible()
})

test('Back from the question gives the focus back to the field', async ({ page }) => {
  // Arrange
  await opensRepositories(page)
  await goTo(page, '~/src/')
  const dialog = page.getByRole('dialog', { name: 'Go to a directory' })
  await dialog.getByRole('button', { name: 'Switch to ~/src', exact: true }).click()

  // Act
  await dialog.getByRole('button', { name: 'Back' }).click()

  // Assert
  await expect(page.getByRole('combobox', { name: 'Directory' })).toBeFocused()
})

test('a directory above, pressed, is typed, and the focus stays in the field', async ({ page }) => {
  // Arrange
  await opensRepositories(page)
  await goTo(page, '~/src/api/')

  // Act
  await page.getByRole('button', { name: 'Up to ~/src' }).click()

  // Assert
  const field = page.getByRole('combobox', { name: 'Directory' })
  await expect(field).toHaveValue('~/src/')
  await expect(field).toBeFocused()
})

test('no key pressed in the dialog reaches the page behind it', async ({ page }) => {
  // Arrange
  await opensRepositories(page)
  await goTo(page, '~/src/')
  const dialog = page.getByRole('dialog', { name: 'Go to a directory' })
  await dialog.getByRole('button', { name: 'Cancel' }).focus()

  // Act
  await page.keyboard.press('?')

  // Assert
  await expect(dialog).toBeVisible()
  await expect(page.getByRole('dialog', { name: 'Keyboard shortcuts' })).toHaveCount(0)
})

test('the header says where the server works and opens the section', async ({ page }) => {
  // Arrange
  await opensRepositories(page)
  await openSection(page, 'Issues')

  // Act
  await page.getByRole('button', { name: 'Working in ~/src/api/cmd: open Repositories' }).click()

  // Assert
  await expect(page.getByRole('heading', { level: 1, name: 'Repositories' })).toBeVisible()
})

for (const theme of themes) {
  for (const width of widths) {
    test(`Repositories fits ${String(width)} px in the ${theme} theme, reachable and clean`, async ({
      page,
    }) => {
      // Arrange
      await pinTheme(page, theme)
      await page.emulateMedia({ reducedMotion: 'reduce' })
      await page.setViewportSize({ width, height })
      await opensRepositories(page)

      // Act & Assert
      await expectReachableAndClean(page, {
        reaches: [
          'Go to a directory…',
          'Add to favorites',
          'Switch to ~/src/web',
          'Remove ~/old-site from favorites',
        ],
      })
    })

    test(`the directory dialog fits ${String(width)} px in the ${theme} theme, reachable and clean`, async ({
      page,
    }) => {
      // Arrange
      await pinTheme(page, theme)
      await page.emulateMedia({ reducedMotion: 'reduce' })
      await page.setViewportSize({ width, height })
      await opensRepositories(page)

      // Act
      await goTo(page, '~/src/')
      await expect(page.getByRole('option', { name: /^web/ })).toBeVisible()

      // Assert
      await expectReachableAndClean(page, {
        reaches: ['Up to ~', 'Directory', 'Cancel', 'Switch to ~/src'],
        within: page.getByRole('dialog', { name: 'Go to a directory' }),
      })
    })
  }
}
