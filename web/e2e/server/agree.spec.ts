import { writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { expect, test, type Locator } from '@playwright/test'
import { openSection } from '../support/cockpit.ts'
import { openLive } from './served.ts'

// agreed is the file this spec writes into the served repository and
// stages, beside the notes.txt commit.spec.ts stages: the specs share the
// run's one fixture, so each writes a file of its own.
const agreed = 'agreed.txt'

// writeAgreed writes agreed, untracked, into the repository the server
// serves, as an editor would. The config names the bare repository the
// served one pushes to; the served one sits beside it.
function writeAgreed(): void {
  const origin: unknown = test.info().project.metadata.origin
  if (typeof origin !== 'string') {
    throw new Error('the project names no fixture origin')
  }

  writeFileSync(join(dirname(origin), 'repo', agreed), 'Said only once the page shows it.\n')
}

// shownNow reports whether target is visible at this moment, asked once and
// never again: a page that agrees with a notice already shows what the
// notice says, so a check that waited would let a page that lags pass.
async function shownNow(target: Locator): Promise<boolean> {
  return target.isVisible()
}

// enabledNow reports whether target takes a click at this moment, asked once
// and never again, as shownNow asks.
async function enabledNow(target: Locator): Promise<boolean> {
  return target.isEnabled()
}

// The binary's stream re-pushes every five seconds; a write wakes it at once,
// and the page says a write is done only once a frame read after it landed
// is on the page: the first whose count of writes reaches the number the
// write's answer carries. So each notice below is checked against the page
// the moment it shows, with no wait for the page to catch up.
test('each write is said only once the page served shows it', async ({ page }) => {
  // Arrange
  writeAgreed()
  await openLive(page)
  await openSection(page, 'Branch')
  await expect(page.getByRole('button', { name: `Stage ${agreed}` })).toBeVisible()
  const form = page.getByRole('form', { name: 'Commit staged changes' })

  // Act: stage the file
  await page.getByRole('button', { name: `Stage ${agreed}` }).click()
  await expect(page.getByRole('status').filter({ hasText: `Staged ${agreed}.` })).toBeVisible()

  // Assert: its row and the commit form already agree
  expect(await shownNow(page.getByRole('button', { name: `Unstage ${agreed}` }))).toBe(true)
  expect(await enabledNow(form.getByRole('button', { name: 'Commit staged changes' }))).toBe(true)

  // Act: commit it
  await form.getByLabel('Type').selectOption('docs')
  await form.getByLabel('Scope (optional)').fill('fixture')
  await form.getByLabel('Subject').fill('agree with the page')
  await form.getByRole('button', { name: 'Commit staged changes' }).click()
  await expect(form.getByRole('status')).toHaveText(
    /^Committed [0-9a-f]{7} docs\(fixture\): agree with the page\.$/,
  )

  // Assert: the branch already lists the commit
  const commits = page.getByRole('region', { name: 'Commits', exact: true })
  expect(await shownNow(commits.getByText('docs(fixture): agree with the page'))).toBe(true)

  // Act: push it
  await page.getByRole('button', { name: 'Push branch' }).click()
  await page
    .getByRole('group', { name: /^Push / })
    .getByRole('button', { name: 'Push', exact: true })
    .click()
  await expect(page.getByRole('status').filter({ hasText: 'Pushed docs/notes.' })).toBeVisible()

  // Assert: the branch already reads level with its upstream
  const branch = page.getByRole('region', { name: 'docs/notes', exact: true })
  expect(await shownNow(branch.getByText('0 ahead, 0 behind'))).toBe(true)
})
