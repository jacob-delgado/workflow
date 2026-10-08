import type { Page } from '@playwright/test'
import type { Comment, IssueDetail } from '../../src/api/generated/types.gen.ts'
import { mockConfig } from '../../src/dev/mockConfig.ts'
import { height, pinTheme, themes, widths } from '../support/cockpit.ts'
import { expect, issuesOf, snapshotWith, streams, test } from '../support/fixtures.ts'
import { expectReachableAndClean } from '../support/reachable.ts'

// Commenting on a Jira issue from its detail: the thread, the composer under
// it, and what a posted comment does.

const issue = {
  key: 'PROJ-1',
  tracker: 'jira',
  summary: 'Redact tokens before they reach the request log',
  status: 'In Progress',
  status_category: 'indeterminate',
  type: 'Bug',
  priority: 'High',
} as const

const snapshot = snapshotWith({ issues: issuesOf([issue]) })

const earlier: Comment = {
  author: 'Sam Ortiz',
  body: "Repro'd on *main*: the header lands in {{request.log}}.\n* staging\n* prod",
  created: '2026-09-18T15:04:00Z',
}

// detailWith is PROJ-1 in full, holding the comments given.
function detailWith(comments: Comment[]): IssueDetail {
  return {
    ...issue,
    reporter: 'Ana Lopez',
    assignee: 'octocat',
    description: 'The request log records every header.',
    comments,
    comment_total: comments.length,
    url: 'https://jira.example.com/browse/PROJ-1',
  }
}

// opensIssue serves PROJ-1, keeping each comment posted on it as Jira would,
// with jira.markdown_comments as given, and opens its detail. It answers what
// was sent.
async function opensIssue(page: Page, markdown: boolean): Promise<string[]> {
  const comments = [earlier]
  const sent: string[] = []
  await streams(page, snapshot)
  await page.route('**/api/config', (route) =>
    route.fulfill({
      json: { ...mockConfig, jira: { ...mockConfig.jira, markdown_comments: markdown } },
    }),
  )
  await page.route('**/api/issues/PROJ-1', (route) => route.fulfill({ json: detailWith(comments) }))
  await page.route('**/api/issues/PROJ-1/comment', (route) => {
    const { text } = route.request().postDataJSON() as { text: string }
    sent.push(text)
    const posted = { author: 'Ana Lopez', body: text, created: new Date().toISOString() }
    comments.push(posted)

    return route.fulfill({ json: posted })
  })
  await page.goto('/')
  await page.getByRole('button', { name: /redact tokens/i }).click()
  await expect(page.getByRole('list', { name: 'Comments' })).toBeVisible()

  return sent
}

test('a comment posted from the detail joins the thread', async ({ page }) => {
  // Arrange
  const sent = await opensIssue(page, false)
  const box = page.getByRole('textbox', { name: 'Comment on PROJ-1' })
  await box.fill('Fixed on the branch.')

  // Act
  await page.getByRole('button', { name: 'Comment' }).click()

  // Assert
  const thread = page.getByRole('list', { name: 'Comments' })
  await expect(thread).toContainText('Fixed on the branch.')
  expect(sent).toEqual(['Fixed on the branch.'])
  await expect(page.getByRole('status').filter({ hasText: 'Commented on PROJ-1.' })).toBeVisible()
  await expect(box).toHaveValue('')
  await expect(box).toBeFocused()
})

test('Preview shows a Markdown comment as Jira will', async ({ page }) => {
  // Arrange
  await opensIssue(page, true)
  await page.getByRole('textbox', { name: 'Comment on PROJ-1' }).fill('Ship **it**')

  // Act
  await page.getByRole('tab', { name: 'Preview' }).click()

  // Assert
  await expect(page.getByRole('tabpanel', { name: 'Preview' }).getByRole('strong')).toHaveText('it')
})

for (const theme of themes) {
  for (const width of widths) {
    test(`the thread and a Markdown composer fit ${String(width)} px in the ${theme} theme, reachable and clean`, async ({
      page,
    }) => {
      // Arrange: a comment written, in this theme, at this width.
      await pinTheme(page, theme)
      await page.emulateMedia({ reducedMotion: 'reduce' })
      await page.setViewportSize({ width, height })
      await opensIssue(page, true)
      await page.getByRole('textbox', { name: 'Comment on PROJ-1' }).fill('Ship **it** with `care`')

      // Act & Assert: Tab reaches the tabs, the formatting and the button.
      await expectReachableAndClean(page, {
        reaches: ['Write', 'Bold', 'Bulleted list', 'Comment'],
      })
    })
  }
}

const forgeIssue = {
  key: '57',
  tracker: 'forge',
  summary: 'Typo in the README',
  status: 'Open',
  status_category: 'new',
  type: '',
} as const

// opensForgeIssue serves forge issue 57 on a GitLab remote, its thread written
// in Markdown, and opens its detail.
async function opensForgeIssue(page: Page): Promise<void> {
  await streams(page, snapshotWith({ issues: issuesOf([forgeIssue]) }))
  await page.route('**/api/health', (route) =>
    route.fulfill({
      json: {
        version: '1.2.3',
        dry_run: false,
        forge_kind: 'gitlab',
        forge_noun: 'merge request',
        forge_sigil: '!',
      },
    }),
  )
  await page.route('**/api/config', (route) =>
    route.fulfill({
      json: { ...mockConfig, jira: { ...mockConfig.jira, markdown_comments: false } },
    }),
  )
  await page.route('**/api/issues/57', (route) =>
    route.fulfill({
      json: {
        ...forgeIssue,
        reporter: 'octo',
        description: 'The install line is wrong.',
        comments: [
          {
            author: 'octo',
            body: 'Fixed in **main**, see `README.md`.',
            created: '2026-09-18T15:04:00Z',
          },
        ],
        comment_total: 1,
        url: 'https://gitlab.com/group/repo/-/issues/57',
      } satisfies IssueDetail,
    }),
  )
  await page.goto('/')
  await page.getByRole('button', { name: /typo in the readme/i }).click()
  await expect(page.getByRole('list', { name: 'Comments' })).toBeVisible()
}

for (const theme of themes) {
  test(`a GitLab issue's thread and composer are reachable and clean in the ${theme} theme`, async ({
    page,
  }) => {
    // Arrange: a forge issue on GitLab, whose composer is always Markdown and
    // says a slash line is a quick action.
    await pinTheme(page, theme)
    await page.emulateMedia({ reducedMotion: 'reduce' })
    await page.setViewportSize({ width: widths[0], height })
    await opensForgeIssue(page)
    await page.getByRole('textbox', { name: 'Comment on #57' }).fill('/close')

    // Act & Assert: the thread is drawn from Markdown, the hint is on screen,
    // and Tab reaches the tabs, the formatting and the button.
    await expect(page.getByRole('list', { name: 'Comments' }).getByRole('strong')).toHaveText(
      'main',
    )
    await expect(
      page.getByText('A line starting with / runs as a GitLab quick action.'),
    ).toBeVisible()
    await expectReachableAndClean(page, { reaches: ['Write', 'Bold', 'Comment'] })
  })
}
