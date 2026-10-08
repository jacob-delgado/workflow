import type { Page } from '@playwright/test'
import { mockDirectories, mockRepositories } from '../../src/dev/mockRepositories.ts'
import { height, openSection, pinTheme, themes, widths } from '../support/cockpit.ts'
import { expectReachableAndClean } from '../support/reachable.ts'
import { expect, problem, test } from '../support/fixtures.ts'

// The Repositories section: where the server works, the favorites, the
// directory picker, and a switch asked once more before it is made.

// opensRepositories answers the section's reads as the mockup does, keeps
// each switch asked, answering it with where the mockup shows that directory,
// and opens the section.
async function opensRepositories(page: Page): Promise<string[]> {
  const switched: string[] = []
  await page.route('**/api/repositories', (route) => route.fulfill({ json: mockRepositories() }))
  await page.route('**/api/directories**', (route) => {
    const path = new URL(route.request().url()).searchParams.get('path') ?? '/home/ana/src/api/cmd'

    return route.fulfill({ json: mockDirectories(path) })
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
  // The picker reads its directory apart from the section, so it is waited for
  // too: a walk taken before it draws would not count its controls.
  await expect(page.getByRole('button', { name: 'Up' })).toBeVisible()

  return switched
}

test('a favorite is switched to once the switch is confirmed', async ({ page }) => {
  // Arrange
  const switched = await opensRepositories(page)
  await page.getByRole('button', { name: 'Switch to ~/src/web' }).click()

  // Act
  await page
    .getByRole('region', { name: 'Switch to ~/src/web?' })
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
    .getByRole('region', { name: 'Switch to ~/src/api-review?' })
    .getByRole('button', { name: 'Switch' })
    .click()

  // Assert
  await expect(page.getByText('Switched to ~/src/api-review.')).toBeVisible()
  expect(switched).toEqual(['/home/ana/src/api-review'])
})

test('a directory browsed to is offered to switch to', async ({ page }) => {
  // Arrange
  await opensRepositories(page)
  await page.getByRole('button', { name: 'Up' }).click()
  await page.getByRole('button', { name: 'Up' }).click()
  await page.getByRole('button', { name: 'Open web' }).click()

  // Act
  await page.getByRole('button', { name: 'Switch here' }).click()

  // Assert
  await expect(page.getByRole('region', { name: 'Switch to ~/src/web?' })).toBeVisible()
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
          'Add to favorites',
          'Switch to ~/src/web',
          'Remove ~/old-site from favorites',
          'Directory',
          'Show',
          'Up',
          'Switch here',
        ],
      })
    })
  }
}
