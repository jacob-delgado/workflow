import type { Issue } from '../../src/api/generated/types.gen.ts'
import { openSection } from '../support/cockpit.ts'
import { expect, issuesOf, seeds, snapshotWith, test } from '../support/fixtures.ts'
import { axeViolations } from '../support/reachable.ts'

// The issues the store kept for the view, which the stream sends ahead of its
// first snapshot: the list paints from them before Jira answers, and the
// first snapshot takes their place. The hermetic stream ends after its
// messages and the browser reconnects, so these assert on the list and the
// sections, never on the stream's pill.

// keptIssue is the issue the store kept from last time.
const keptIssue = {
  key: 'PROJ-1',
  tracker: 'jira',
  summary: 'Kept from last time',
  status: 'To Do',
  status_category: 'new',
  type: 'Task',
} satisfies Issue

// liveIssue is the issue Jira answers with now.
const liveIssue = { ...keptIssue, key: 'PROJ-2', summary: 'Found just now' } satisfies Issue

// readingJira is what the list says while it shows the kept issues.
const readingJira = 'Reading Jira… these are the issues it listed last time.'

test('with a kept view, the list paints before Jira answers', async ({ page }) => {
  // Arrange
  await seeds(page, { issues: issuesOf([keptIssue]) })

  // Act
  await page.goto('/')

  // Assert
  await expect(page.getByRole('button', { name: /kept from last time/i })).toBeVisible()
  await expect(page.getByText(readingJira)).toBeVisible()
  await expect(page.getByRole('button', { name: /load more/i })).toHaveCount(0)
  expect(await axeViolations(page)).toBe('')
})

test("the first update takes the kept list's place", async ({ page }) => {
  // Arrange
  await seeds(
    page,
    { issues: issuesOf([keptIssue]) },
    snapshotWith({ issues: issuesOf([liveIssue]) }),
  )

  // Act
  await page.goto('/')

  // Assert
  await expect(page.getByRole('button', { name: /found just now/i })).toBeVisible()
  await expect(page.getByRole('button', { name: /kept from last time/i })).toHaveCount(0)
  await expect(page.getByText(readingJira)).toHaveCount(0)
})

test('every other section waits for the first update', async ({ page }) => {
  // Arrange
  await seeds(page, { issues: issuesOf([keptIssue]) })
  await page.goto('/')
  await expect(page.getByRole('button', { name: /kept from last time/i })).toBeVisible()

  // Act
  await openSection(page, 'Branch')

  // Assert
  await expect(
    page
      .getByRole('main')
      .getByText(/connecting to workflow|waiting for the first update|reconnecting to workflow/i),
  ).toBeVisible()
})
