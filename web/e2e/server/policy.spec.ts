import { expect, test, type Locator, type Page } from '@playwright/test'
import type { Announcement } from '../../src/api/generated/types.gen.ts'
import { mockConfig } from '../../src/dev/mockConfig.ts'
import { openSection, openSettingsTab } from '../support/cockpit.ts'
import { branchWith, snapshotWith, streams } from '../support/fixtures.ts'
import { heardRefusals, openServed } from './served.ts'

// The Base UI parts the page opens beyond the palette and the sheet, under the
// server's own content policy: Settings' sections, which are Tabs, the
// announcement's tag picker, whose field for an owner not yet linked is a
// Combobox, and the directory picker, an Autocomplete in a Dialog. The policy
// refuses a <style> element, and CSPProvider keeps every part from writing one
// (TRADE-50 in TECH_DEBT.md), so a Base UI release that writes one from any
// of these parts fails here. The fixture serves no configuration and reaches
// no Slack and no forge, so the Settings and tag picker specs answer the reads
// that fill their part themselves; the directory picker lists the fixture's
// own directories. The page, its scripts and the policy they run under are
// the server's.

// settingsSections are Settings' sections, in the order its list offers them.
const settingsSections = [
  'Jira',
  'Messaging',
  'Forge and pull requests',
  'Commits and branches',
  'Taskwarrior',
  'Terminal and keys',
  'Advanced',
  'People and groups',
  'Local data',
]

test('Settings’ sections open one after another under the content policy', async ({ page }) => {
  // Arrange
  const refusals = await heardRefusals(page)
  // Where no configuration file applies, Settings offers to set one up rather
  // than its sections; the mockup's configuration stands in for one.
  await page.route('**/api/config', (route) => route.fulfill({ json: mockConfig }))
  await openServed(page)
  await openSection(page, 'Settings')

  // Act
  for (const section of settingsSections) {
    await openSettingsTab(page, section)
    await expect(page.getByRole('tabpanel', { name: section })).toBeVisible()
  }

  // Assert
  await expect(
    page.getByRole('tab', { name: 'Local data', exact: true, selected: true }),
  ).toBeVisible()
  expect(refusals).toEqual([])
})

// A branch with an open pull request, announced on Slack: the stream's frame
// the tag picker's spec answers with.
const announcing = snapshotWith({
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
    announced: false,
    pull: {
      number: 7,
      url: 'https://forge.example.com/pull/7',
      title: 'Redact tokens in the request log',
      state: 'open',
      draft: false,
      approvals: 0,
      changes_requested: false,
      mergeable: 'clean',
    },
  },
  messaging: {
    kind: 'slack',
    service: 'Slack',
    configured: true,
    channel: '#dev',
    channels: ['#dev'],
    author: 'ana',
  },
})

// The announcement's preview: ben owns the changes and is not linked yet, so
// the picker offers whom of the channel's members he is.
const announcement = {
  text: 'ana opened a pull request: Redact tokens in the request log',
  channel: '#dev',
  tagging: {
    available: true,
    owners: [{ owner: 'ben', kind: 'user', state: 'unlinked' }],
    groups: [],
  },
} satisfies Announcement

test('the tag picker offers an owner the channel’s members under the content policy', async ({
  page,
}) => {
  // Arrange
  const refusals = await heardRefusals(page)
  await streams(page, announcing)
  await page.route('**/api/announcement**', (route) => route.fulfill({ json: announcement }))
  await page.route('**/api/slack/members**', (route) =>
    route.fulfill({ json: { entries: [{ id: 'U0BEN', label: 'Ben Ito' }] } }),
  )
  await openServed(page)
  await openSection(page, 'Slack')
  await page.getByRole('button', { name: 'Announce to Slack' }).click()
  await expect(page.getByRole('group', { name: 'Announcement preview' })).toBeFocused()

  // Act
  await page.getByRole('combobox', { name: 'Slack user for ben' }).fill('Ben')

  // Assert
  await expect(page.getByRole('option', { name: 'Ben Ito' })).toBeVisible()
  expect(refusals).toEqual([])
})

// openDirectoryPicker opens the page the server serves on Repositories, and
// the directory picker from it, its field taking the focus.
async function openDirectoryPicker(page: Page): Promise<Locator> {
  await openServed(page)
  await openSection(page, 'Repositories')
  await page.getByRole('button', { name: 'Go to a directory…' }).click()
  const field = page.getByRole('combobox', { name: 'Directory' })
  await expect(field).toBeFocused()

  return field
}

test('the directory picker lists a directory’s directories under the content policy', async ({
  page,
}) => {
  // Arrange
  const refusals = await heardRefusals(page)
  const field = await openDirectoryPicker(page)

  // Act
  // Above the repository served are the fixture's other directories: the
  // server's home and the bare origin.
  await field.fill('../')

  // Assert
  await expect(
    page
      .getByRole('listbox', { name: 'Directories' })
      .getByRole('option', { name: 'origin.git', exact: true }),
  ).toBeVisible()
  expect(refusals).toEqual([])
})

test('the directory picker says a directory holds none under the content policy', async ({
  page,
}) => {
  // Arrange
  const refusals = await heardRefusals(page)

  // Act
  // The fixture's repository holds no directory but .git, which is hidden.
  await openDirectoryPicker(page)

  // Assert
  await expect(page.getByText(/^No directories in \S+\/repo\.$/)).toBeVisible()
  expect(refusals).toEqual([])
})

test('the directory picker says a directory cannot be listed under the content policy', async ({
  page,
}) => {
  // Arrange
  const refusals = await heardRefusals(page)
  const field = await openDirectoryPicker(page)

  // Act
  // No directory nowhere is beside the repository, so its listing is refused
  // and the list is left with nothing in it.
  await field.fill('./nowhere/')

  // Assert
  await expect(
    page.getByRole('dialog', { name: 'Go to a directory' }).getByRole('alert'),
  ).toBeVisible()
  await expect(page.getByRole('option')).toHaveCount(0)
  expect(refusals).toEqual([])
})
