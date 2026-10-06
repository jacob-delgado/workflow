import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { Comment, IssueDetail } from '@/api/generated/types.gen.ts'
import { mockConfig } from '@/dev/mockConfig.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { gitLabWords, makeHealth } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { useHealthStore } from '@/api/health.ts'
import { IssueDetailPanel } from './IssueDetailPanel.tsx'

// Commenting on an issue from its detail: the composer under the thread, what
// it sends, and what the thread shows once Jira has the comment.

const issuePath = '/api/issues/PROJ-1'
const commentPath = `${issuePath}/comment`

const earlier: Comment = {
  author: 'Sam Ortiz',
  body: "Repro'd on *main*.",
  created: '2026-09-20T10:00:00Z',
}

// detailWith is PROJ-1, a Jira bug, holding the comments given.
function detailWith(comments: Comment[], overrides: Partial<IssueDetail> = {}): IssueDetail {
  return {
    key: 'PROJ-1',
    tracker: 'jira',
    summary: 'Fix the token leak',
    status: 'In Progress',
    status_category: 'indeterminate',
    type: 'Bug',
    reporter: 'Ana Lopez',
    description: '',
    comments,
    comment_total: comments.length,
    url: '',
    ...overrides,
  }
}

// configWith is the configuration with jira.markdown_comments as given.
function configWith(markdown: boolean) {
  return { ...mockConfig, jira: { ...mockConfig.jira, markdown_comments: markdown } }
}

// jiraTakingComments serves PROJ-1 and keeps each comment posted to it, as
// Jira would, so a read after a post shows it.
function jiraTakingComments({ markdown = false } = {}) {
  const comments = [earlier]
  const sent: string[] = []
  const requests = fakeApi({
    [issuePath]: () => detailWith(comments),
    '/api/config': configWith(markdown),
    [commentPath]: async (_: URL, request: Request) => {
      const { text } = (await request.json()) as { text: string }
      sent.push(text)
      const posted = { author: 'Ana Lopez', body: text, created: '2026-10-05T09:00:00Z' }
      comments.push(posted)

      return posted
    },
  })

  return { requests, sent }
}

// writesTo is each write the page sent, by method and path.
function writesTo(requests: Request[]): string[] {
  return requests
    .filter((request) => request.method !== 'GET')
    .map((request) => `${request.method} ${new URL(request.url).pathname}`)
}

// commentBox is the composer's text box, once the issue is read.
async function commentBox(): Promise<HTMLElement> {
  return screen.findByRole('textbox', { name: 'Comment on PROJ-1' })
}

test('a comment posted shows in the thread, and the box is ready for another', async () => {
  // Arrange
  const { requests } = jiraTakingComments()
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)
  await userEvent.type(await commentBox(), 'Fixed on the branch.')

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Comment' }))

  // Assert
  const thread = screen.getByRole('list', { name: 'Comments' })
  await within(thread).findByText('Fixed on the branch.')
  expect(writesTo(requests)).toEqual([`POST ${commentPath}`])
  expect(
    within(screen.getByRole('region', { name: 'Comments' })).getByRole('status').textContent,
  ).toBe('Commented on PROJ-1.')
  const box = await commentBox()
  expect((box as HTMLTextAreaElement).value).toBe('')
  expect(document.activeElement).toBe(box)
})

test('what is typed is sent as it is', async () => {
  // Arrange
  const { sent } = jiraTakingComments()
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)
  await userEvent.type(await commentBox(), 'Line one{Enter}  *two*')

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Comment' }))

  // Assert
  await waitFor(() => {
    expect(sent).toEqual(['Line one\n  *two*'])
  })
})

test('a blank comment is not sent, and says why', async () => {
  // Arrange
  const { requests } = jiraTakingComments()
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)
  await userEvent.type(await commentBox(), '   ')

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Comment' }))

  // Assert
  expect(screen.getByRole('alert').textContent).toBe('Write a comment first.')
  expect(writesTo(requests)).toEqual([])
})

test('a refused comment says why and keeps what was written', async () => {
  // Arrange
  fakeApi({
    [issuePath]: detailWith([earlier]),
    '/api/config': configWith(false),
    [commentPath]: () =>
      Response.json(
        {
          type: 'about:blank',
          title: 'Unprocessable',
          status: 422,
          code: 'unprocessable',
          detail: 'Jira refused the request',
        },
        { status: 422, headers: { 'Content-Type': 'application/problem+json' } },
      ),
  })
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)
  const box = await commentBox()
  await userEvent.type(box, 'Please look.')

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Comment' }))

  // Assert
  expect((await screen.findByRole('alert')).textContent).toMatch(/Jira refused the request/)
  expect((box as HTMLTextAreaElement).value).toBe('Please look.')
})

test('the thread draws each comment written in wiki markup, with when it was written', async () => {
  // Arrange
  fakeApi({ [issuePath]: detailWith([earlier]), '/api/config': configWith(false) })

  // Act
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)

  // Assert
  const thread = await screen.findByRole('list', { name: 'Comments' })
  const comment = within(thread).getByRole('listitem')
  expect(within(comment).getByText('Sam Ortiz')).toBeTruthy()
  expect(within(comment).getByRole('strong').textContent).toBe('main')
  expect(within(comment).getByRole('time').getAttribute('datetime')).toBe(earlier.created)
})

test('with Markdown off there is no Preview, and the hint says Jira reads wiki markup', async () => {
  // Arrange
  fakeApi({ [issuePath]: detailWith([]), '/api/config': configWith(false) })

  // Act
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)

  // Assert
  await commentBox()
  expect(screen.queryByRole('tab')).toBeNull()
  expect(screen.queryByRole('group', { name: 'Formatting' })).toBeNull()
  expect(screen.getByText(/Jira reads this as wiki markup/)).toBeTruthy()
})

test('with Markdown on, Preview draws the comment as Jira will show it', async () => {
  // Arrange
  fakeApi({ [issuePath]: detailWith([]), '/api/config': configWith(true) })
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)
  await userEvent.type(
    await screen.findByRole('textbox', { name: 'Comment on PROJ-1' }),
    'Ship **it**',
  )
  const preview = await screen.findByRole('tab', { name: 'Preview' })

  // Act
  await userEvent.click(preview)

  // Assert
  expect(preview.getAttribute('aria-selected')).toBe('true')
  const shown = screen.getByRole('tabpanel', { name: 'Preview' })
  expect(within(shown).getByRole('strong').textContent).toBe('it')
})

test('the arrow keys move between Write and Preview', async () => {
  // Arrange
  fakeApi({ [issuePath]: detailWith([]), '/api/config': configWith(true) })
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)
  const write = await screen.findByRole('tab', { name: 'Write' })
  write.focus()

  // Act
  await userEvent.keyboard('{ArrowRight}')

  // Assert
  const preview = screen.getByRole('tab', { name: 'Preview' })
  expect(document.activeElement).toBe(preview)
  expect(preview.getAttribute('aria-selected')).toBe('true')
})

test('Bold wraps what is selected and keeps it selected', async () => {
  // Arrange
  fakeApi({ [issuePath]: detailWith([]), '/api/config': configWith(true) })
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)
  await screen.findByRole('tab', { name: 'Write' })
  const box = (await commentBox()) as HTMLTextAreaElement
  await userEvent.type(box, 'ship it now')
  box.setSelectionRange(5, 7)

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Bold' }))

  // Assert
  expect(box.value).toBe('ship **it** now')
  expect(box.value.slice(box.selectionStart, box.selectionEnd)).toBe('it')
  expect(document.activeElement).toBe(box)
})

test('a list marks each line selected', async () => {
  // Arrange
  fakeApi({ [issuePath]: detailWith([]), '/api/config': configWith(true) })
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)
  await screen.findByRole('tab', { name: 'Write' })
  const box = (await commentBox()) as HTMLTextAreaElement
  await userEvent.type(box, 'one{Enter}two')
  box.setSelectionRange(0, box.value.length)

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Bulleted list' }))

  // Assert
  expect(box.value).toBe('- one\n- two')
})

test('the count says how many characters the comment holds', async () => {
  // Arrange
  fakeApi({ [issuePath]: detailWith([]), '/api/config': configWith(false) })
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)

  // Act
  await userEvent.type(await commentBox(), 'hello')

  // Assert
  const box = await commentBox()
  const described = (box.getAttribute('aria-describedby') ?? '')
    .split(' ')
    .map((id) => document.getElementById(id)?.textContent)
  expect(described).toContain('5 characters')
})

// forgeIssue serves forge issue 42, holding the comments given, and keeps each
// comment posted to it, with Jira's Markdown setting off.
function forgeIssue(comments: Comment[] = []) {
  const sent: string[] = []
  fakeApi({
    '/api/issues/42': detailWith(comments, { key: '42', tracker: 'forge' }),
    '/api/config': configWith(false),
    '/api/issues/42/comment': async (_: URL, request: Request) => {
      const { text } = (await request.json()) as { text: string }
      sent.push(text)

      return { author: 'octo', body: text, created: '2026-10-05T09:00:00Z' }
    },
  })

  return sent
}

test('a forge issue takes a comment in Markdown, whatever Jira says', async () => {
  // Arrange
  const sent = forgeIssue()
  const user = userEvent.setup()
  renderWithClient(<IssueDetailPanel issueKey="42" />)
  const box = await screen.findByRole('textbox', { name: 'Comment on #42' })
  await user.type(box, 'Ship **it**')

  // Act
  await user.click(screen.getByRole('button', { name: 'Comment' }))

  // Assert
  await waitFor(() => {
    expect(sent).toEqual(['Ship **it**'])
  })
  expect(screen.getByRole('tab', { name: 'Preview' })).toBeTruthy()
  expect(await screen.findByText('Commented on #42.')).toBeTruthy()
})

test("a forge issue's thread is drawn from Markdown", async () => {
  // Arrange
  forgeIssue([{ author: 'octo', body: 'Ship **it**', created: '2026-10-01T10:00:00Z' }])

  // Act
  renderWithClient(<IssueDetailPanel issueKey="42" />)

  // Assert
  const thread = await screen.findByRole('list', { name: 'Comments' })
  expect(within(thread).getByRole('strong').textContent).toBe('it')
})

test.each([
  ['GitLab', gitLabWords, true],
  ['GitHub', {}, false],
])('on %s the box says whether a slash line is a quick action', async (_, words, said) => {
  // Arrange
  useHealthStore.setState({ health: makeHealth(words) })
  forgeIssue()

  // Act
  renderWithClient(<IssueDetailPanel issueKey="42" />)

  // Assert
  const box = await screen.findByRole('textbox', { name: 'Comment on #42' })
  const described = (box.getAttribute('aria-describedby') ?? '')
    .split(' ')
    .map((id) => document.getElementById(id)?.textContent ?? '')
    .join(' ')
  expect(described.includes('quick action')).toBe(said)
})

test('a thread past its parsing budget draws its older comments as plain text', async () => {
  // Arrange
  // Each comment is within the sizes one is parsed at, but a thread of many
  // could stall the page, so the newest are parsed and the rest are not.
  const big = (n: number) => `${'word '.repeat(10)}\n`.repeat(900) + `newer ${String(n)}`
  forgeIssue([
    { author: 'octo', body: 'Ship **old**', created: '2026-10-01T09:00:00Z' },
    { author: 'octo', body: big(1), created: '2026-10-01T10:00:00Z' },
    { author: 'octo', body: big(2), created: '2026-10-01T11:00:00Z' },
    { author: 'octo', body: 'Ship **new**', created: '2026-10-01T12:00:00Z' },
  ])

  // Act
  renderWithClient(<IssueDetailPanel issueKey="42" />)

  // Assert
  const thread = await screen.findByRole('list', { name: 'Comments' })
  expect(
    within(thread)
      .getAllByRole('strong')
      .map((strong) => strong.textContent),
  ).toEqual(['new'])
  expect(within(thread).getByText('Ship **old**')).toBeTruthy()
})
