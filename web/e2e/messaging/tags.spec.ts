import { expect, test, type Page } from '@playwright/test'
import type {
  Announcement,
  People,
  PersonLink,
  Snapshot,
} from '../../src/api/generated/types.gen.ts'
import { streams } from '../tabwalk.ts'

// The announcement preview's tags: an owner linked to a channel member and
// saved for next time, a user group checked, and the post that carries the
// groups for the server to tag. The flows run on the hermetic build, whose
// answers this spec gives and records.

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

const snapshot = {
  issues: { total: 0, start_at: 0, unavailable: [], issues: [] },
  branch: {
    name: 'fix/PROJ-7-redact',
    issue_link: '',
    detached: false,
    head: 'abc1234',
    upstream: 'origin/fix/PROJ-7-redact',
    push_remote: 'origin',
    ahead: 0,
    behind: 0,
    base: 'origin/main',
    commits: [],
  },
  changes: { changes: [] },
  review: { found: true, pull },
  messaging: {
    service: 'Slack',
    configured: true,
    channel: '#dev',
    channels: ['#dev'],
    author: 'ana',
  },
  branches: [],
  commit_types: ['feat', 'fix'],
  suggested_scope: '',
  tasks: { available: true, reason: '', linked: [] },
} satisfies Snapshot

const ben = { id: 'U0BEN', label: 'Ben Ito' }
const carla = { id: 'U0CARLA', label: 'Carla Diaz' }
const pod = { id: 'S0POD', label: 'control-plane-pod' }
const reviewers = { id: 'S0API', label: 'api-reviewers' }
const text = 'ana opened a pull request: Redact tokens in the request log'

const announcement = {
  text,
  channel: '#dev',
  tagging: {
    available: true,
    owners: [
      { owner: 'ben', kind: 'user', state: 'unlinked' },
      { owner: 'carla', kind: 'user', state: 'linked', slack: carla },
      { owner: 'acme/control-plane', kind: 'team', state: 'linked', slack: pod },
    ],
    groups: [
      { slack: reviewers, checked: false, from_owners: false },
      { slack: pod, checked: true, from_owners: true },
    ],
  },
} satisfies Announcement

// Sent is what the page sent the server: each link, and the post.
interface Sent {
  links: PersonLink[]
  posts: unknown[]
}

// answersTags answers the preview, the channel's members, a link and the
// post, and records what the page sent.
async function answersTags(page: Page): Promise<Sent> {
  const sent: Sent = { links: [], posts: [] }
  await streams(page, snapshot)
  await page.route('**/api/announcement', (route) => route.fulfill({ json: announcement }))
  await page.route('**/api/slack/members**', (route) =>
    route.fulfill({ json: { entries: [ben, carla] } }),
  )
  await page.route('**/api/people', (route) => {
    const link = route.request().postDataJSON() as PersonLink
    sent.links.push(link)
    const people: People = {
      owners: [
        { owner: 'ben', kind: 'user', state: 'linked', slack: ben },
        ...announcement.tagging.owners.slice(1),
      ],
    }

    return route.fulfill({ json: people })
  })
  await page.route('**/api/announce', (route) => {
    sent.posts.push(route.request().postDataJSON())

    return route.fulfill({ json: { text, channel: '#dev' } })
  })

  return sent
}

// opensPreview opens the Slack section and its announcement preview.
async function opensPreview(page: Page): Promise<void> {
  await page.goto('/')
  await page
    .getByRole('navigation', { name: 'Sections' })
    .getByRole('button', { name: 'Slack', exact: true })
    .click()
  await page.getByRole('button', { name: 'Announce to Slack' }).click()
  await expect(page.getByRole('group', { name: 'Announcement preview' })).toBeFocused()
}

test('links an owner, checks a group, and the post carries the groups', async ({ page }) => {
  // Arrange: the preview open, ben linked to a channel member, a group checked.
  const sent = await answersTags(page)
  await opensPreview(page)
  const ownerChoice = page.getByRole('combobox', { name: 'Slack user for ben' })
  await expect(ownerChoice.getByRole('option', { name: 'Ben Ito' })).toBeAttached()
  await ownerChoice.selectOption({ label: 'Ben Ito' })
  await expect(page.getByText('Saved for next time: ben is Ben Ito.')).toBeVisible()
  await page.getByRole('checkbox', { name: /@api-reviewers/ }).check()
  await expect(
    page.getByText('@Ben Ito, @Carla Diaz, @api-reviewers, @control-plane-pod'),
  ).toBeVisible()

  // Act
  await page.getByRole('button', { name: 'Announce now' }).click()

  // Assert: the link was kept, and the post asked for both groups.
  await expect(page.getByText('Announced to #dev.')).toBeVisible()
  expect(sent.links).toEqual([{ owner: 'ben', slack_id: 'U0BEN', not_on_slack: false }])
  expect(sent.posts).toEqual([{ channel: '#dev', text, mentions: { groups: ['S0POD', 'S0API'] } }])
})

test('an unchecked group is left out of the post', async ({ page }) => {
  // Arrange
  const sent = await answersTags(page)
  await opensPreview(page)
  await page.getByRole('checkbox', { name: /@control-plane-pod/ }).uncheck()

  // Act
  await page.getByRole('button', { name: 'Announce now' }).click()

  // Assert
  await expect(page.getByText('Announced to #dev.')).toBeVisible()
  expect(sent.posts).toEqual([{ channel: '#dev', text, mentions: { groups: [] } }])
})

test(
  'the mockup previews whom it tags and links an owner',
  { tag: '@populated' },
  async ({ page }) => {
    // Arrange
    await opensPreview(page)

    // Act
    await page.getByRole('combobox', { name: 'Slack user for ben' }).selectOption({
      label: 'Ben Ito',
    })

    // Assert
    await expect(page.getByText('Saved for next time: ben is Ben Ito.')).toBeVisible()
    await expect(page.getByText('@Ben Ito, @Carla Diaz, @control-plane-pod')).toBeVisible()
  },
)
