import type { Locator, Page } from '@playwright/test'
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
      base_remote: 'origin',
      base: 'origin/main',
    }),
    review: {
      found: true,
      announced: 'none',
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
      ? route.fulfill({ status: 202, json: { id: 1, state: 'waiting', channel: '#dev', pull: 7 } })
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

// holdsAndStops answers the preview, holds a post asked to wait for CI and
// stops it as asked, the stream's next frames showing each as the server's
// would.
async function holdsAndStops(page: Page): Promise<void> {
  const waiting = {
    id: 1,
    state: 'waiting',
    waiting_on: 'ci',
    channel: '#dev',
    pull: 7,
  } satisfies QueuedAnnouncement
  const stopped = { id: 1, state: 'stopped', channel: '#dev', pull: 7 } satisfies QueuedAnnouncement

  await streams(page, snapshotHolding())
  await page.route('**/api/announcement**', (route) => route.fulfill({ json: announcement }))
  await page.route('**/api/announce', async (route) => {
    await streams(page, snapshotHolding(waiting))
    await route.fulfill({ status: 202, json: waiting })
  })
  await page.route('**/api/announce/queued', async (route) => {
    await streams(page, snapshotHolding(stopped))
    await route.fulfill({ status: 204 })
  })
}

test('once one is held, Announce now instead is offered, and once stopped, Announce to again', async ({
  page,
}) => {
  // Arrange
  await holdsAndStops(page)
  await openAnnouncementPreview(page)

  // Act: hold it until CI passes
  await page.getByRole('button', { name: 'Announce when CI passes' }).click()

  // Assert: it waits, and announcing it now instead is offered
  await expect(
    page.getByText('Waiting for CI on #7 to pass, then announcing to #dev.'),
  ).toBeVisible()
  await expect(page.getByRole('button', { name: 'Announce now instead' })).toBeVisible()

  // Act: stop waiting
  await page.getByRole('button', { name: 'Stop waiting' }).click()

  // Assert: nothing waits, and announcing is offered again
  await expect(
    page.getByRole('status').filter({ hasText: 'Stopped waiting; nothing was announced.' }),
  ).toBeVisible()
  await expect(page.getByRole('button', { name: 'Announce to Slack' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Stop waiting' })).toHaveCount(0)
})

// drawing is how a control is drawn — its fill, its words' color and its edge
// — and where its top stands on the page.
function drawing(control: Locator) {
  return control.evaluate((element) => {
    const { backgroundColor, color, borderTopColor } = getComputedStyle(element)

    return {
      fill: [backgroundColor, color, borderTopColor],
      top: element.getBoundingClientRect().top,
    }
  })
}

test('while one waits, Announce now instead is drawn as Stop waiting is, below it', async ({
  page,
}) => {
  // Arrange
  await answersAnnouncement(page, {
    id: 1,
    state: 'waiting',
    channel: '#dev',
    pull: 7,
    automatic: true,
  })
  await page.goto('/')

  // Act
  await openSection(page, 'Slack')

  // Assert
  const stop = await drawing(page.getByRole('button', { name: 'Stop waiting' }))
  const instead = await drawing(page.getByRole('button', { name: 'Announce now instead' }))
  expect(instead.fill).toEqual(stop.fill)
  expect(instead.top).toBeGreaterThan(stop.top)
  await expect(page.getByRole('button', { name: 'Announce to Slack' })).toHaveCount(0)
})

for (const theme of themes) {
  for (const width of widths) {
    test(`the edited preview fits ${String(width)} px in the ${theme} theme, reachable and clean`, async ({
      page,
    }) => {
      // Arrange
      await pinTheme(page, theme)
      await page.emulateMedia({ reducedMotion: 'reduce' })
      await page.setViewportSize({ width, height })
      await answersAnnouncement(page)
      await openAnnouncementPreview(page)
      await page.getByRole('button', { name: 'Edit' }).click()

      // Act & Assert
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
    await answersAnnouncement(page, { id: 1, state: 'waiting', channel: '#dev', pull: 7 })
    await page.goto('/')

    // Act
    await openSection(page, 'Slack')

    // Assert
    await expect(
      page.getByText('Waiting for CI on #7 to pass, then announcing to #dev.'),
    ).toBeVisible()
    await expectReachableAndClean(page, { reaches: ['Stop waiting', 'Announce now instead'] })
  })

  test(`a held announcement waiting on a fix is clean in the ${theme} theme`, async ({ page }) => {
    // Arrange
    await pinTheme(page, theme)
    await page.emulateMedia({ reducedMotion: 'reduce' })
    await page.setViewportSize({ width: widths[0], height })
    await answersAnnouncement(page, {
      id: 1,
      state: 'waiting',
      channel: '#dev',
      pull: 7,
      waiting_on: 'fix',
      ci: 'failed',
    })
    await page.goto('/')

    // Act
    await openSection(page, 'Slack')

    // Assert
    await expect(
      page.getByText(
        'A check failed on #7; waiting on a fix, then announcing to #dev once CI passes.',
      ),
    ).toBeVisible()
    await expectReachableAndClean(page, { reaches: ['Stop waiting', 'Announce now instead'] })
  })
}
