import type { Page } from '@playwright/test'
import type { Announcement, Snapshot } from '../../src/api/generated/types.gen.ts'
import { height, openAnnouncementPreview, pinTheme, themes, widths } from '../support/cockpit.ts'
import { branchWith, expect, snapshotWith, streams, test } from '../support/fixtures.ts'
import { expectReachableAndClean } from '../support/reachable.ts'

// Nothing is announced while a check has failed on the pull request: the
// preview offers only to announce it once CI passes, and says why. The flow
// runs on the hermetic build, whose answers this spec gives.

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

// snapshotFailing is the cockpit with pull request 7 open, a check on it
// failed.
function snapshotFailing(): Snapshot {
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
      ci: { state: 'failed', total: 2, done: 2, failed: 1, checks: [] },
    },
    messaging: {
      kind: 'slack',
      service: 'Slack',
      configured: true,
      channel: '#dev',
      channels: [],
      author: 'ana',
    },
  })
}

const announcement = {
  text: 'ana opened a pull request: Redact tokens in the request log',
  channel: '#dev',
  ci: 'failed',
  can_wait_for_ci: true,
} satisfies Announcement

// answersAFailedCheck answers the stream and the preview of a pull request a
// check failed on.
async function answersAFailedCheck(page: Page): Promise<void> {
  await streams(page, snapshotFailing())
  await page.route('**/api/announcement**', (route) => route.fulfill({ json: announcement }))
}

test('a preview of a pull request a check failed on offers only to wait for a fix', async ({
  page,
}) => {
  // Arrange
  await answersAFailedCheck(page)

  // Act
  await openAnnouncementPreview(page)

  // Assert
  await expect(page.getByRole('button', { name: 'Announce when CI passes' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Announce now' })).toHaveCount(0)
  await expect(
    page
      .getByRole('status')
      .filter({ hasText: 'A check failed on #7, so it is not announced now.' }),
  ).toBeVisible()
})

for (const theme of themes) {
  for (const width of widths) {
    test(`the failed-check preview fits ${String(width)} px in the ${theme} theme, reachable and clean`, async ({
      page,
    }) => {
      // Arrange
      await pinTheme(page, theme)
      await page.emulateMedia({ reducedMotion: 'reduce' })
      await page.setViewportSize({ width, height })
      await answersAFailedCheck(page)

      // Act
      await openAnnouncementPreview(page)

      // Assert
      await expectReachableAndClean(page, { reaches: ['Edit', 'Announce when CI passes'] })
    })
  }
}
