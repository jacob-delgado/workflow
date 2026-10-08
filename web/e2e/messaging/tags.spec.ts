import type { Page } from '@playwright/test'
import type { Announcement, People, PersonLink } from '../../src/api/generated/types.gen.ts'
import { openAnnouncementPreview } from '../support/cockpit.ts'
import { branchWith, expect, snapshotWith, streams, test } from '../support/fixtures.ts'

// The announcement preview's tags: an owner linked to a member of the channel
// picked and saved for next time, a user group checked, and the post that
// carries the people shown and the groups for the server to tag. The flows
// run on the hermetic build, whose answers this spec gives and records.

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

const snapshot = snapshotWith({
  branch: branchWith({
    name: 'fix/PROJ-7-redact',
    head: 'abc1234',
    upstream: 'origin/fix/PROJ-7-redact',
    push_remote: 'origin',
    base: 'origin/main',
  }),
  review: { found: true, announced: false, pull },
  messaging: {
    service: 'Slack',
    configured: true,
    channel: '#dev',
    channels: ['#dev', '#ops'],
    author: 'ana',
  },
})

const ben = { id: 'U0BEN', label: 'Ben Ito' }
const carla = { id: 'U0CARLA', label: 'Carla Diaz' }
const olive = { id: 'U0OLIVE', label: 'Olive Ops' }
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

// membersIn are each channel's members: one each alone has.
const membersIn: Partial<Record<string, (typeof ben)[]>> = {
  '#dev': [ben, carla],
  '#ops': [olive, carla],
}

// answersTags answers the preview, each channel's members, a link to one of
// them and the post, and records what the page sent.
async function answersTags(page: Page): Promise<Sent> {
  const sent: Sent = { links: [], posts: [] }
  await streams(page, snapshot)
  await page.route('**/api/announcement**', (route) => route.fulfill({ json: announcement }))
  await page.route('**/api/slack/members**', (route) => {
    const channel = new URL(route.request().url()).searchParams.get('channel') ?? '#dev'

    return route.fulfill({ json: { entries: membersIn[channel] ?? [] } })
  })
  await page.route('**/api/people', (route) => {
    const link = route.request().postDataJSON() as PersonLink
    sent.links.push(link)
    const slack = membersIn[link.channel ?? '#dev']?.find((member) => member.id === link.slack_id)
    const people: People = {
      owners: [
        { owner: 'ben', kind: 'user', state: 'linked', slack },
        ...announcement.tagging.owners.slice(1),
      ],
    }

    return route.fulfill({ json: people })
  })
  await page.route('**/api/announce', (route) => {
    const post = route.request().postDataJSON() as { channel: string }
    sent.posts.push(post)

    return route.fulfill({ json: { text, channel: post.channel } })
  })

  return sent
}

test('links an owner, checks a group, and the post carries the groups', async ({ page }) => {
  // Arrange: the preview open, ben linked to a channel member, a group checked.
  const sent = await answersTags(page)
  await openAnnouncementPreview(page)
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
  expect(sent.links).toEqual([
    { owner: 'ben', slack_id: 'U0BEN', not_on_slack: false, channel: '#dev' },
  ])
  expect(sent.posts).toEqual([
    {
      channel: '#dev',
      text,
      mentions: { users: ['U0BEN', 'U0CARLA'], groups: ['S0POD', 'S0API'] },
    },
  ])
})

test('links an owner to a member of the other channel picked', async ({ page }) => {
  // Arrange: the preview moved to #ops, whose members ben is picked from.
  const sent = await answersTags(page)
  await openAnnouncementPreview(page)
  await page.getByRole('combobox', { name: 'Channel' }).selectOption('#ops')
  const ownerChoice = page.getByRole('combobox', { name: 'Slack user for ben' })
  await expect(ownerChoice.getByRole('option', { name: 'Olive Ops' })).toBeAttached()
  await expect(ownerChoice.getByRole('option', { name: 'Ben Ito' })).not.toBeAttached()
  await ownerChoice.selectOption({ label: 'Olive Ops' })
  await expect(page.getByText('Saved for next time: ben is Olive Ops.')).toBeVisible()

  // Act
  await page.getByRole('button', { name: 'Announce now' }).click()

  // Assert: the link named #ops, and the post went there tagging olive.
  await expect(page.getByText('Announced to #ops.')).toBeVisible()
  expect(sent.links).toEqual([
    { owner: 'ben', slack_id: 'U0OLIVE', not_on_slack: false, channel: '#ops' },
  ])
  expect(sent.posts).toEqual([
    { channel: '#ops', text, mentions: { users: ['U0OLIVE', 'U0CARLA'], groups: ['S0POD'] } },
  ])
})

test('an unchecked group is left out of the post', async ({ page }) => {
  // Arrange
  const sent = await answersTags(page)
  await openAnnouncementPreview(page)
  await page.getByRole('checkbox', { name: /@control-plane-pod/ }).uncheck()

  // Act
  await page.getByRole('button', { name: 'Announce now' }).click()

  // Assert
  await expect(page.getByText('Announced to #dev.')).toBeVisible()
  expect(sent.posts).toEqual([
    { channel: '#dev', text, mentions: { users: ['U0CARLA'], groups: [] } },
  ])
})

test(
  'the mockup previews whom it tags and links an owner',
  { tag: '@populated' },
  async ({ page }) => {
    // Arrange
    await openAnnouncementPreview(page)

    // Act
    await page.getByRole('combobox', { name: 'Slack user for ben' }).selectOption({
      label: 'Ben Ito',
    })

    // Assert
    await expect(page.getByText('Saved for next time: ben is Ben Ito.')).toBeVisible()
    await expect(page.getByText('@Ben Ito, @Carla Diaz, @control-plane-pod')).toBeVisible()
  },
)

test(
  'the mockup links an owner to a member of the channel picked',
  { tag: '@populated' },
  async ({ page }) => {
    // Arrange: #releases has a member #dev-workflow has not.
    await openAnnouncementPreview(page)
    await page.getByRole('combobox', { name: 'Channel' }).selectOption('#releases')
    const ownerChoice = page.getByRole('combobox', { name: 'Slack user for ben' })
    await expect(ownerChoice.getByRole('option', { name: 'Erin Park' })).toBeAttached()

    // Act
    await ownerChoice.selectOption({ label: 'Erin Park' })

    // Assert
    await expect(page.getByText('Saved for next time: ben is Erin Park.')).toBeVisible()
  },
)
