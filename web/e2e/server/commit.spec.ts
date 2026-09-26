import { execFileSync } from 'node:child_process'
import { expect, test } from '@playwright/test'
import { openSection } from '../cockpit.ts'

// originSubject is the subject of the newest commit on docs/notes, the branch
// the served repository publishes, in the bare repository it pushes to, read by
// git itself rather than through the page. The config names that repository in
// the project's metadata.
function originSubject(): string {
  const origin: unknown = test.info().project.metadata.origin
  if (typeof origin !== 'string') {
    throw new Error('the project names no fixture origin')
  }

  return execFileSync('git', ['--git-dir', origin, 'log', '-1', '--format=%s', 'docs/notes'], {
    encoding: 'utf8',
    env: { PATH: process.env.PATH },
  }).trim()
}

// A write's answer says what it did, but the control it enables — Commit once
// a file is staged — comes with the stream's next frame of the repository, a
// few seconds on; each click waits for it. Push branch is there from the start,
// since docs/notes has no upstream, and is clicked once the commit is made, so
// the push carries it.
test('stages, commits and pushes a file through the page the server serves', async ({ page }) => {
  // Arrange: the served repository's Branch section, notes.txt untracked in it.
  await page.goto('/')
  await openSection(page, 'Branch')

  // Act: stage the untracked file.
  await page.getByRole('button', { name: 'Stage notes.txt' }).click()

  // Assert: the page says it staged it.
  await expect(page.getByRole('status').filter({ hasText: 'Staged notes.txt.' })).toBeVisible()

  // Act: commit it.
  const form = page.getByRole('form', { name: 'Commit staged changes' })
  await form.getByLabel('Type').selectOption('docs')
  await form.getByLabel('Scope (optional)').fill('fixture')
  await form.getByLabel('Subject').fill('record the notes')
  await form.getByRole('button', { name: 'Commit staged changes' }).click()

  // Assert: the page names the commit git made.
  await expect(form.getByRole('status')).toHaveText(
    /^Committed [0-9a-f]{7} docs\(fixture\): record the notes\.$/,
  )

  // Act: push it.
  await page.getByRole('button', { name: 'Push branch' }).click()
  await page
    .getByRole('group', { name: /^Push / })
    .getByRole('button', { name: 'Push', exact: true })
    .click()

  // Assert: the page says it pushed, and the origin holds the commit.
  await expect(page.getByRole('status').filter({ hasText: 'Pushed docs/notes.' })).toBeVisible()
  expect(originSubject()).toBe('docs(fixture): record the notes')
})
