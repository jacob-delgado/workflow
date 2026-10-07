import { expect, test, type Page } from '@playwright/test'
import type { Comment, IssueDetail, Snapshot } from '../../src/api/generated/types.gen.ts'
import { mockConfig } from '../../src/dev/mockConfig.ts'
import { height, pinTheme, themes, widths } from '../cockpit.ts'
import { axeViolations, sidewaysScrollers, streams, walkTabOrder } from '../tabwalk.ts'

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

const snapshot = {
  issues: { total: 1, start_at: 0, unavailable: [], issues: [issue] },
  branch: {
    name: '',
    issue_link: '',
    detached: false,
    head: '',
    upstream: '',
    push_remote: '',
    ahead: 0,
    behind: 0,
    base: '',
    commits: [],
  },
  changes: { changes: [] },
  review: { found: false, announced: false },
  messaging: { service: 'Slack', configured: false, channel: '', channels: [], author: '' },
  branches: [],
  commit_types: ['feat', 'fix'],
  subject_limit: 72,
  suggested_scope: '',
  hooks_unmanaged: 0,
  tasks: { available: true, reason: '', linked: [] },
  here: '/home/ana/src/api',
} satisfies Snapshot

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
  await expect(page.getByRole('tabpanel', { name: 'Preview' }).locator('strong')).toHaveText('it')
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

      // Act: Tab once round the page.
      const { reached, missed, hidden } = await walkTabOrder(page)

      // Assert: nothing scrolls sideways; Tab reaches the tabs, the formatting
      // and the button, each in view; and axe finds nothing.
      expect(await page.evaluate(sidewaysScrollers), 'scrolls sideways').toEqual([])
      expect(reached, 'reached by Tab').toEqual(
        expect.arrayContaining(['Write', 'Bold', 'Bulleted list', 'Comment']),
      )
      expect(missed, 'never reached by Tab').toEqual([])
      expect(hidden, 'out of view with focus').toEqual([])
      expect(await axeViolations(page), 'axe').toBe('')
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
  await streams(page, { ...snapshot, issues: { ...snapshot.issues, issues: [forgeIssue] } })
  await page.route('**/api/health', (route) =>
    route.fulfill({
      json: { version: '1.2.3', dry_run: false, forge_noun: 'merge request', forge_sigil: '!' },
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

    // Act: Tab once round the page.
    const { reached, missed, hidden } = await walkTabOrder(page)

    // Assert: the thread is drawn from Markdown, the hint is on screen, Tab
    // reaches the tabs, the formatting and the button, and axe finds nothing.
    await expect(page.getByRole('list', { name: 'Comments' }).getByRole('strong')).toHaveText(
      'main',
    )
    await expect(
      page.getByText('A line starting with / runs as a GitLab quick action.'),
    ).toBeVisible()
    expect(await page.evaluate(sidewaysScrollers), 'scrolls sideways').toEqual([])
    expect(reached, 'reached by Tab').toEqual(expect.arrayContaining(['Write', 'Bold', 'Comment']))
    expect(missed, 'never reached by Tab').toEqual([])
    expect(hidden, 'out of view with focus').toEqual([])
    expect(await axeViolations(page), 'axe').toBe('')
  })
}
