import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import type { FollowUp, OpenedPullRequest } from '@/api/generated/types.gen.ts'
import { useHealthStore } from '@/api/health.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { gitLabWords, makeHealth, makeSnapshot } from '@/test/fixtures.ts'
import { ReviewPanel } from './ReviewPanel.tsx'

const pull = {
  number: 7,
  url: 'https://forge.example.com/pull/7',
  title: 'Redact tokens in the request log',
  draft: false,
  approvals: 0,
  changes_requested: false,
  mergeable: 'unknown' as const,
}

const linkOffer: FollowUp = { action: 'link', issue_key: 'PROJ-412' }
const moveOffer: FollowUp = { action: 'transition', issue_key: 'PROJ-412', status: 'In Review' }

// serveTheOpen answers the draft and the open, which offers followUps, and the
// two issue writes; the routes a case passes replace these.
function serveTheOpen(followUps: FollowUp[], routes: Record<string, unknown> = {}) {
  const opened: OpenedPullRequest = { pull, follow_ups: followUps }

  return fakeApi({
    '/api/pull-request/draft': {
      title: 'fix: redact tokens',
      body: 'why',
      base: 'main',
      head: 'fix/PROJ-412',
      draft: false,
      needs_push: false,
    },
    '/api/pull-request': opened,
    '/api/issues/PROJ-412/link': pull,
    '/api/issues/PROJ-412/transition': { key: 'PROJ-412', status: 'In Review' },
    ...routes,
  })
}

// openThePullRequest drives the panel through the open: the offer, the
// composed form, and its confirm, until the open's outcome shows. The forge's
// noun is what its buttons and outcome say.
async function openThePullRequest(user: ReturnType<typeof userEvent.setup>, noun = 'pull request') {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ review: { found: false } }),
  })
  render(<ReviewPanel />)
  await openAgain(user, noun)
}

// openAgain opens from a panel already showing the offer to open.
async function openAgain(user: ReturnType<typeof userEvent.setup>, noun = 'pull request') {
  await user.click(screen.getByRole('button', { name: `Open a ${noun}` }))
  await user.click(await screen.findByRole('button', { name: `Open ${noun}` }))
  await screen.findByText(new RegExp(`^Opened ${noun} [#!]\\d+\\.$`))
}

// heldOpen is an answer whose body arrives only once release is called, so a
// write can be caught in flight.
function heldOpen(body: unknown): { response: Response; release: () => void } {
  let release = () => {}
  const stream = new ReadableStream<Uint8Array>({
    start(controller) {
      release = () => {
        controller.enqueue(new TextEncoder().encode(JSON.stringify(body)))
        controller.close()
      }
    },
  })

  return {
    response: new Response(stream, { headers: { 'Content-Type': 'application/json' } }),
    release: () => {
      release()
    },
  }
}

// writesTo lists the writes the page sent, as method and path.
function writesTo(requests: Request[]): string[] {
  return requests
    .filter((request) => request.method !== 'GET')
    .map((request) => `${request.method} ${new URL(request.url).pathname}`)
}

test('after opening, the panel offers to link and to move the issue', async () => {
  // Arrange
  const user = userEvent.setup()
  serveTheOpen([linkOffer, moveOffer])

  // Act
  await openThePullRequest(user)

  // Assert
  expect(screen.getByRole('button', { name: 'Link it on PROJ-412' })).toBeTruthy()
  expect(screen.getByRole('button', { name: 'Move PROJ-412 to In Review' })).toBeTruthy()
})

test('offers nothing more when the open has nothing to follow it', async () => {
  // Arrange
  const user = userEvent.setup()
  serveTheOpen([])

  // Act
  await openThePullRequest(user)

  // Assert
  expect(screen.queryByRole('button', { name: /link it on/i })).toBeNull()
  expect(screen.queryByRole('button', { name: /^move /i })).toBeNull()
})

test('the link outcome survives the snapshot that shows the pull request', async () => {
  // Arrange
  const user = userEvent.setup()
  const requests = serveTheOpen([linkOffer, moveOffer])
  await openThePullRequest(user)

  // Act: link it
  await user.click(screen.getByRole('button', { name: 'Link it on PROJ-412' }))

  // Assert: the link is sent for the branch's issue, and said
  expect(await screen.findByText('Linked #7 on PROJ-412.')).toBeTruthy()
  expect(writesTo(requests)).toContain('POST /api/issues/PROJ-412/link')

  // Act: the next snapshot shows the pull request that was opened
  act(() => {
    useSnapshotStore.setState({ snapshot: makeSnapshot({ review: { found: true, pull } }) })
  })

  // Assert: the outcome and the other offer are still there, beside the pull request
  expect(screen.getByRole('heading', { level: 2, name: /redact tokens/i })).toBeTruthy()
  expect(screen.getByText('Linked #7 on PROJ-412.')).toBeTruthy()
  expect(screen.getByRole('button', { name: 'Move PROJ-412 to In Review' })).toBeTruthy()
})

test('moves the issue to the review status and says so', async () => {
  // Arrange
  const user = userEvent.setup()
  const requests = serveTheOpen([moveOffer])
  await openThePullRequest(user)

  // Act
  await user.click(screen.getByRole('button', { name: 'Move PROJ-412 to In Review' }))

  // Assert
  expect(await screen.findByText('Moved PROJ-412 to In Review.')).toBeTruthy()
  expect(writesTo(requests)).toContain('POST /api/issues/PROJ-412/transition')
})

test('hands focus to the outcome once its offer is done', async () => {
  // Arrange
  const user = userEvent.setup()
  serveTheOpen([linkOffer])
  await openThePullRequest(user)

  // Act
  await user.click(screen.getByRole('button', { name: 'Link it on PROJ-412' }))

  // Assert
  // The button that had focus is gone once the link is made, so it cannot be
  // made twice; focus lands on what it said rather than falling to the page.
  const outcome = await screen.findByText('Linked #7 on PROJ-412.')
  expect(screen.queryByRole('button', { name: 'Link it on PROJ-412' })).toBeNull()
  expect(document.activeElement).toBe(outcome)
})

test('focus moved on to the next offer while a link is made stays there', async () => {
  // Arrange
  const link = heldOpen(pull)
  const user = userEvent.setup()
  serveTheOpen([linkOffer, moveOffer], { '/api/issues/PROJ-412/link': () => link.response })
  await openThePullRequest(user)
  await user.click(screen.getByRole('button', { name: 'Link it on PROJ-412' }))
  const move = screen.getByRole('button', { name: 'Move PROJ-412 to In Review' })
  move.focus()

  // Act
  link.release()
  await screen.findByText('Linked #7 on PROJ-412.')

  // Assert
  expect(document.activeElement).toBe(move)
})

test('a link said while focus was on the next offer leaves that offer its own outcome', async () => {
  // Arrange
  // The link is said while focus is on Move, which the user then presses for
  // its own write; a stream frame lands while that move is in flight.
  const link = heldOpen(pull)
  const transition = heldOpen({ key: 'PROJ-412', status: 'In Review' })
  const user = userEvent.setup()
  serveTheOpen([linkOffer, moveOffer], {
    '/api/issues/PROJ-412/link': () => link.response,
    '/api/issues/PROJ-412/transition': () => transition.response,
  })
  await openThePullRequest(user)
  await user.click(screen.getByRole('button', { name: 'Link it on PROJ-412' }))
  screen.getByRole('button', { name: 'Move PROJ-412 to In Review' }).focus()
  link.release()
  await screen.findByText('Linked #7 on PROJ-412.')
  await user.click(screen.getByRole('button', { name: 'Move PROJ-412 to In Review' }))
  act(() => {
    useSnapshotStore.setState({ snapshot: structuredClone(useSnapshotStore.getState().snapshot) })
  })

  // Act
  transition.release()

  // Assert
  const moved = await screen.findByText('Moved PROJ-412 to In Review.')
  expect(document.activeElement).toBe(moved)
})

test('a second open offers its own link afresh', async () => {
  // Arrange
  // The first pull request is linked, then closed on the forge, and a second
  // one is opened on the same branch.
  const user = userEvent.setup()
  const second = { ...pull, number: 8, url: 'https://forge.example.com/pull/8' }
  const opens = [pull, second]
  const requests = serveTheOpen([linkOffer], {
    '/api/pull-request': () => ({ pull: opens.shift(), follow_ups: [linkOffer] }),
  })
  await openThePullRequest(user)
  await user.click(screen.getByRole('button', { name: 'Link it on PROJ-412' }))
  await screen.findByText('Linked #7 on PROJ-412.')
  act(() => {
    useSnapshotStore.setState({ snapshot: makeSnapshot({ review: { found: true, pull } }) })
  })
  act(() => {
    useSnapshotStore.setState({ snapshot: makeSnapshot({ review: { found: false } }) })
  })

  // Act
  await openAgain(user)

  // Assert
  expect(await screen.findByRole('button', { name: 'Link it on PROJ-412' })).toBeTruthy()
  expect(screen.queryByText('Linked #8 on PROJ-412.')).toBeNull()
  expect(writesTo(requests).filter((write) => write.endsWith('/link'))).toHaveLength(1)
})

test('the outcome of an open goes when another branch is checked out', async () => {
  // Arrange
  const user = userEvent.setup()
  const other = makeSnapshot({ review: { found: false } })
  other.branch = { ...other.branch, name: 'fix/PROJ-9' }
  serveTheOpen([linkOffer, moveOffer])
  await openThePullRequest(user)

  // Act: another branch is checked out before the stream shows the pull request
  act(() => {
    useSnapshotStore.setState({ snapshot: other })
  })

  // Assert: the old branch's outcome and offers go, and the new one can open
  expect(screen.queryByText('Opened pull request #7.')).toBeNull()
  expect(screen.queryByRole('button', { name: 'Link it on PROJ-412' })).toBeNull()
  expect(screen.getByRole('button', { name: 'Open a pull request' })).toBeTruthy()

  // Act: the first branch is checked out again, its pull request open
  act(() => {
    useSnapshotStore.setState({ snapshot: makeSnapshot({ review: { found: true, pull } }) })
  })

  // Assert: its offers do not come back, so nothing can be linked twice
  expect(screen.queryByRole('button', { name: 'Link it on PROJ-412' })).toBeNull()
})

test("links in GitLab's words: a merge request, marked !7", async () => {
  // Arrange
  const user = userEvent.setup()
  useHealthStore.setState({ health: makeHealth(gitLabWords) })
  serveTheOpen([linkOffer])
  await openThePullRequest(user, 'merge request')

  // Act
  await user.click(screen.getByRole('button', { name: 'Link it on PROJ-412' }))

  // Assert
  expect(await screen.findByText('Linked !7 on PROJ-412.')).toBeTruthy()
})

test("a link refused with no reason says so in GitLab's words", async () => {
  // Arrange
  const user = userEvent.setup()
  useHealthStore.setState({ health: makeHealth(gitLabWords) })
  serveTheOpen([linkOffer], {
    '/api/issues/PROJ-412/link': () => new Response('', { status: 500 }),
  })
  await openThePullRequest(user, 'merge request')

  // Act
  await user.click(screen.getByRole('button', { name: 'Link it on PROJ-412' }))

  // Assert
  expect(
    await screen.findByText(
      'The merge request was not linked on PROJ-412. Try again, or link it in Jira.',
    ),
  ).toBeTruthy()
})

test('a refused move says why and can be tried again', async () => {
  // Arrange
  const user = userEvent.setup()
  const refusal = {
    type: 'https://jacob-delgado.github.io/workflow/docs/errors/#conflict',
    title: 'Conflict',
    status: 409,
    detail: 'Jira wants fields filled to move PROJ-412 to In Review',
    code: 'conflict',
  }
  serveTheOpen([moveOffer], {
    '/api/issues/PROJ-412/transition': () =>
      Response.json(refusal, {
        status: 409,
        headers: { 'Content-Type': 'application/problem+json' },
      }),
  })
  await openThePullRequest(user)

  // Act
  await user.click(screen.getByRole('button', { name: 'Move PROJ-412 to In Review' }))

  // Assert
  expect(await screen.findByText(/wants fields filled/i)).toBeTruthy()
  expect(screen.getByRole('button', { name: 'Move PROJ-412 to In Review' })).toBeTruthy()
})

test('a server restarted under --dry-run holds the offer back and says so', async () => {
  // Arrange
  const user = userEvent.setup()
  const requests = serveTheOpen([linkOffer])
  await openThePullRequest(user)
  useHealthStore.setState({ health: makeHealth({ dry_run: true }) })

  // Act
  await user.click(screen.getByRole('button', { name: 'Link it on PROJ-412' }))

  // Assert
  expect(await screen.findByText(/held back by --dry-run/i)).toBeTruthy()
  expect(writesTo(requests)).not.toContain('POST /api/issues/PROJ-412/link')
})

test('the mockup opens and follows up with no server behind it', async () => {
  // Arrange
  vi.stubEnv('VITE_MOCK', 'true')
  const user = userEvent.setup()
  const fetch = vi.mocked(globalThis.fetch)
  await openThePullRequest(user)

  // Act
  await user.click(screen.getByRole('button', { name: /link it on/i }))
  await user.click(screen.getByRole('button', { name: /^move /i }))

  // Assert
  expect(await screen.findByText(/^Linked #\d+ on /)).toBeTruthy()
  expect(await screen.findByText(/^Moved /)).toBeTruthy()
  expect(fetch).not.toHaveBeenCalled()
})
