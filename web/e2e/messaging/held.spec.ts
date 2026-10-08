import type { Page } from '@playwright/test'
import type {
  Announcement,
  QueuedAnnouncement,
  Snapshot,
} from '../../src/api/generated/types.gen.ts'
import {
  height,
  openAnnouncementPreview,
  openSection,
  pinTheme,
  themes,
  widths,
} from '../support/cockpit.ts'
import { branchWith, expect, snapshotWith, streams, test } from '../support/fixtures.ts'
import { expectReachableAndClean } from '../support/reachable.ts'

// The announcement edited before it goes, and held until the pull request's
// CI passes, as the terminal's e and w in the preview do. The flows run on
// the hermetic build, whose answers this spec gives and records.

const pull = {
  number: 7,
  url: 'https://forge.example.com/pull/7',
  title: 'Redact tokens in the request log',
  state: 'open',
  draft: false,
  approvals: 0,
  changes_requested: false,
  mergeable: 'clean',
} as const

// snapshotHolding is the cockpit with pull request 7 open, its CI running, and
// the held announcement given, if any.
function snapshotHolding(held?: QueuedAnnouncement): Snapshot {
  return snapshotWith({
    branch: branchWith({
      name: 'fix/PROJ-7-redact',
      head: 'abc1234',
      upstream: 'origin/fix/PROJ-7-redact',
      push_remote: 'origin',
      base: 'origin/main',
    }),
    review: {
      found: true,
      announced: false,
      pull,
      ci: { state: 'running', total: 2, done: 1, failed: 0, checks: [] },
    },
    messaging: {
      kind: 'slack',
      service: 'Slack',
      configured: true,
      channel: '#dev',
      channels: [],
      author: 'ana',
    },
    queued_announcement: held,
  })
}

const text = 'ana opened a pull request: Redact tokens in the request log'

const announcement = { text, channel: '#dev', can_wait_for_ci: true } satisfies Announcement

// answersAnnouncement answers the preview and a post, holding a post asked to
// wait for CI, and records each post the page sent.
async function answersAnnouncement(page: Page, held?: QueuedAnnouncement): Promise<unknown[]> {
  const posts: unknown[] = []
  await streams(page, snapshotHolding(held))
  await page.route('**/api/announcement**', (route) => route.fulfill({ json: announcement }))
  await page.route('**/api/announce', (route) => {
    const post = route.request().postDataJSON() as { when?: string; edited_text?: string }
    posts.push(post)

    return post.when === 'ci_passes'
      ? route.fulfill({ status: 202, json: { state: 'waiting', channel: '#dev', pull: 7 } })
      : route.fulfill({ json: { text: post.edited_text ?? text, channel: '#dev' } })
  })

  return posts
}

test('an edited announcement is held until CI passes', async ({ page }) => {
  // Arrange
  const posts = await answersAnnouncement(page)
  await openAnnouncementPreview(page)
  await page.getByRole('button', { name: 'Edit' }).click()
  await page.getByRole('textbox', { name: 'Announcement text' }).fill('Please review #7')

  // Act
  await page.getByRole('button', { name: 'Announce when CI passes' }).click()

  // Assert
  await expect(
    page.getByRole('status').filter({ hasText: 'Will announce to #dev once CI passes.' }),
  ).toBeVisible()
  expect(posts).toEqual([
    { channel: '#dev', text, edited_text: 'Please review #7', when: 'ci_passes' },
  ])
})

for (const theme of themes) {
  for (const width of widths) {
    test(`the edited preview fits ${String(width)} px in the ${theme} theme, reachable and clean`, async ({
      page,
    }) => {
      // Arrange
      // The preview's text open to edit, in this theme, at this width.
      await pinTheme(page, theme)
      await page.emulateMedia({ reducedMotion: 'reduce' })
      await page.setViewportSize({ width, height })
      await answersAnnouncement(page)
      await openAnnouncementPreview(page)
      await page.getByRole('button', { name: 'Edit' }).click()

      // Act & Assert
      // Tab reaches the text and both ways to announce it.
      await expectReachableAndClean(page, {
        reaches: ['Announcement text', 'Announce when CI passes', 'Announce now'],
      })
    })
  }

  test(`a held announcement waiting is clean in the ${theme} theme`, async ({ page }) => {
    // Arrange
    await pinTheme(page, theme)
    await page.emulateMedia({ reducedMotion: 'reduce' })
    await page.setViewportSize({ width: widths[0], height })
    await answersAnnouncement(page, { state: 'waiting', channel: '#dev', pull: 7 })
    await page.goto('/')

    // Act
    await openSection(page, 'Slack')

    // Assert
    await expect(
      page.getByText('Waiting for CI on #7 to pass, then announcing to #dev.'),
    ).toBeVisible()
    await expectReachableAndClean(page, { reaches: ['Stop waiting'] })
  })
}
