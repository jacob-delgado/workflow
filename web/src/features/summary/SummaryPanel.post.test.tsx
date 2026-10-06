import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { Activity, MessagingDestination } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { SummaryPanel } from './SummaryPanel.tsx'

// markdown is Tuesday's summary as the server writes it to copy and to post.
const markdown = '# 2026-09-15\n\n- committed abc1234 Fix the token leak\n'

// tuesday is what the server says was done on Tuesday 15 September 2026.
const tuesday: Activity = {
  from: '2026-09-15',
  to: '2026-09-15',
  today: '2026-09-16',
  sources: [{ source: 'git', name: 'Git', state: 'read', truncated: false, detail: '' }],
  years: [
    {
      year: 2026,
      months: [
        {
          month: 9,
          name: 'September',
          days: [
            {
              date: '2026-09-15',
              weekday: 'Tuesday',
              hours: [
                {
                  label: '09:00',
                  items: [
                    {
                      at: '2026-09-15T09:30:00Z',
                      source: 'git',
                      verb: 'committed',
                      ref: 'abc1234',
                      title: 'Fix the token leak',
                      url: '',
                      repository: '',
                    },
                  ],
                },
              ],
            },
          ],
        },
      ],
    },
  ],
  text: markdown,
}

// messagingTo puts a snapshot on the stream whose messaging is set as given.
function messagingTo(messaging: Partial<MessagingDestination>) {
  const snapshot = makeSnapshot()
  useSnapshotStore.setState({
    status: 'live',
    snapshot: { ...snapshot, messaging: { ...snapshot.messaging, ...messaging } },
  })
}

// postedTo answers a post as the server does, to destination.
function postedTo(destination: string) {
  return async (_: URL, asked: Request) => {
    const body = (await asked.clone().json()) as { channel?: string; text: string }

    return {
      from: '2026-09-15',
      to: '2026-09-15',
      channel: body.channel ?? '',
      destination,
      text: body.text,
    }
  }
}

// posts are the requests that posted the summary.
function posts(requests: Request[]): Request[] {
  return requests.filter((request) => new URL(request.url).pathname === '/api/activity/post')
}

test('Post… previews the summary and where it goes, and posts nothing yet', async () => {
  // Arrange
  messagingTo({ channel: '#dev', channels: ['#dev', '#team'] })
  const requests = fakeApi({ '/api/activity': tuesday, '/api/activity/post': postedTo('#dev') })
  const user = userEvent.setup()
  renderWithClient(<SummaryPanel />)

  // Act
  await user.click(await screen.findByRole('button', { name: 'Post…' }))

  // Assert
  const preview = screen.getByRole('group', { name: 'Summary preview' })
  expect(within(preview).getByText(/committed abc1234 Fix the token leak/)).toBeTruthy()
  expect(within(preview).getByRole('combobox', { name: 'Channel' })).toHaveProperty('value', '#dev')
  expect(document.activeElement).toBe(preview)
  expect(posts(requests)).toHaveLength(0)
})

test('Post sends the period, the text and the channel chosen, and says where it went', async () => {
  // Arrange
  messagingTo({ channel: '#dev', channels: ['#dev', '#team'] })
  const requests = fakeApi({ '/api/activity': tuesday, '/api/activity/post': postedTo('#team') })
  const user = userEvent.setup()
  renderWithClient(<SummaryPanel />)
  await user.click(await screen.findByRole('button', { name: 'Post…' }))
  await user.selectOptions(screen.getByRole('combobox', { name: 'Channel' }), '#team')

  // Act
  await user.click(screen.getByRole('button', { name: 'Post' }))

  // Assert
  expect(await screen.findByText('Posted to #team.')).toBeTruthy()
  expect(await posts(requests)[0]?.json()).toEqual({
    from: '2026-09-15',
    to: '2026-09-15',
    text: markdown,
    channel: '#team',
  })
  expect(screen.queryByRole('group', { name: 'Summary preview' })).toBeNull()
})

test('an edited summary is posted as it was edited', async () => {
  // Arrange
  messagingTo({})
  const requests = fakeApi({ '/api/activity': tuesday, '/api/activity/post': postedTo('#dev') })
  const user = userEvent.setup()
  renderWithClient(<SummaryPanel />)
  await user.click(await screen.findByRole('button', { name: 'Post…' }))
  await user.click(screen.getByRole('button', { name: 'Edit' }))
  const box = screen.getByRole('textbox', { name: 'Summary text' })
  await user.clear(box)
  await user.type(box, 'Shipped the fix')

  // Act
  await user.click(screen.getByRole('button', { name: 'Post' }))

  // Assert
  await screen.findByText('Posted to #dev.')
  expect(await posts(requests)[0]?.json()).toMatchObject({ text: 'Shipped the fix' })
})

test('Cancel closes the preview unposted and hands focus back to Post…', async () => {
  // Arrange
  messagingTo({})
  const requests = fakeApi({ '/api/activity': tuesday, '/api/activity/post': postedTo('#dev') })
  const user = userEvent.setup()
  renderWithClient(<SummaryPanel />)
  await user.click(await screen.findByRole('button', { name: 'Post…' }))

  // Act
  await user.click(screen.getByRole('button', { name: 'Cancel' }))

  // Assert
  expect(screen.queryByRole('group', { name: 'Summary preview' })).toBeNull()
  await waitFor(() => {
    expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Post…' }))
  })
  expect(posts(requests)).toHaveLength(0)
})

test('a webhook-only setup previews the channel its webhook is bound to', async () => {
  // Arrange
  messagingTo({ service: 'Teams', channel: '', channels: [], author: '' })
  fakeApi({ '/api/activity': tuesday })
  const user = userEvent.setup()
  renderWithClient(<SummaryPanel />)

  // Act
  await user.click(await screen.findByRole('button', { name: 'Post…' }))

  // Assert
  const preview = screen.getByRole('group', { name: 'Summary preview' })
  expect(within(preview).getByText('To the channel its webhook is bound to')).toBeTruthy()
  expect(within(preview).queryByRole('combobox', { name: 'Channel' })).toBeNull()
})

test('a refused post keeps the preview and the focus, and says why', async () => {
  // Arrange
  messagingTo({})
  fakeApi({
    '/api/activity': tuesday,
    '/api/activity/post': () =>
      Response.json(
        {
          type: 'about:blank',
          title: 'Unprocessable',
          status: 422,
          code: 'unprocessable',
          detail: 'the messaging service refused the post; check the token',
        },
        { status: 422 },
      ),
  })
  const user = userEvent.setup()
  renderWithClient(<SummaryPanel />)
  await user.click(await screen.findByRole('button', { name: 'Post…' }))

  // Act
  await user.click(screen.getByRole('button', { name: 'Post' }))

  // Assert
  expect((await screen.findByRole('alert')).textContent).toContain('refused the post')
  expect(screen.getByRole('group', { name: 'Summary preview' })).toBeTruthy()
  expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Post' }))
})

test('with no messaging set up there is nothing to post to', async () => {
  // Arrange
  messagingTo({ configured: false })
  fakeApi({ '/api/activity': tuesday })

  // Act
  renderWithClient(<SummaryPanel />)

  // Assert
  await screen.findByRole('button', { name: 'Copy as Markdown' })
  expect(screen.queryByRole('button', { name: 'Post…' })).toBeNull()
})

test('the preview says how long the summary is against what the service takes', async () => {
  // Arrange
  messagingTo({ channel: '', channels: [] })
  fakeApi({
    '/api/activity': {
      ...tuesday,
      post_length: { service: 'Discord', count: 1234, limit: 2000, unit: 'characters' },
    },
  })
  const user = userEvent.setup()
  renderWithClient(<SummaryPanel />)

  // Act
  await user.click(await screen.findByRole('button', { name: 'Post…' }))

  // Assert
  const preview = screen.getByRole('group', { name: 'Summary preview' })
  expect(within(preview).getByText('1,234 of 2,000 characters Discord takes')).toBeTruthy()
})

test('the preview of a summary too long for the service says so before it is posted', async () => {
  // Arrange
  messagingTo({ channel: '', channels: [] })
  fakeApi({
    '/api/activity': {
      ...tuesday,
      post_length: { service: 'Discord', count: 2517, limit: 2000, unit: 'characters' },
    },
  })
  const user = userEvent.setup()
  renderWithClient(<SummaryPanel />)

  // Act
  await user.click(await screen.findByRole('button', { name: 'Post…' }))

  // Assert
  const preview = screen.getByRole('group', { name: 'Summary preview' })
  expect(
    within(preview).getByText(
      'Too long for Discord: 2,517 of 2,000 characters. Pick a shorter period, or edit it down.',
    ),
  ).toBeTruthy()
})
