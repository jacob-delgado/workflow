import { onlineManager } from '@tanstack/react-query'
import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { IssueDetail } from '@/api/generated/types.gen.ts'
import { mockStatusChanges } from '@/dev/mockIssues.ts'
import { fakeApi, keptBeside, refusedThenHeld, tryAgainShown } from '@/test/fakeApi.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { IssueDetailPanel } from '../IssueDetailPanel.tsx'

// Changing an issue from its detail: its status, with the fields the change
// needs, its assignee, and the work logged on it — each a form that shows what
// it will send, sent only from the form.

const issuePath = '/api/issues/PROJ-1'
const changesPath = `${issuePath}/transitions`

// detailOf is PROJ-1, a Jira bug, or a forge issue when tracker says so.
function detailOf(tracker: IssueDetail['tracker'] = 'jira'): IssueDetail {
  return {
    key: tracker === 'jira' ? 'PROJ-1' : '42',
    tracker,
    summary: 'Fix the token leak',
    status: 'In Progress',
    status_category: 'indeterminate',
    type: 'Bug',
    reporter: 'Ana Lopez',
    description: '',
    comments: [],
    comment_total: 0,
    url: '',
  }
}

// trackerTakingWrites serves PROJ-1, offers the mock's status changes, and
// keeps the body of each write sent, by method and path; a status change
// answers as made to changedTo.
function trackerTakingWrites(answers: Record<string, unknown> = {}, changedTo = 'Resolved') {
  const sent: { to: string; body: unknown }[] = []
  const keep = (answer: unknown) => async (_: URL, request: Request) => {
    if (request.method === 'GET') {
      return mockStatusChanges
    }

    sent.push({
      to: `${request.method} ${new URL(request.url).pathname}`,
      body: await request.json(),
    })

    return answer
  }

  fakeApi({
    [issuePath]: detailOf(),
    [changesPath]: keep({ key: 'PROJ-1', status: changedTo }),
    [`${issuePath}/assignee`]: keep({ key: 'PROJ-1', assignee: 'sam.ortiz' }),
    [`${issuePath}/worklog`]: keep({ key: 'PROJ-1', time_spent: '2h' }),
    ...answers,
  })

  return sent
}

// open presses the issue's action named, once the issue is read.
async function open(action: string) {
  await userEvent.click(await screen.findByRole('button', { name: action }))
}

// choose picks the status change of this id — the mock's 11 to Blocked, 21
// to Resolved with a field form, 31 to Split, which only Jira can make — once
// the changes are read.
async function choose(changeID: string) {
  await userEvent.selectOptions(
    await screen.findByRole('combobox', { name: 'New status' }),
    changeID,
  )
}

// filledForResolved fills every field the change to Resolved needs.
async function filledForResolved() {
  const fields = form('Change the status of PROJ-1')
  await userEvent.type(within(fields).getByRole('textbox', { name: 'Due date' }), '2026-10-09')
  await userEvent.click(within(fields).getByRole('checkbox', { name: '1.5.0' }))
  await userEvent.selectOptions(
    within(fields).getByRole('combobox', { name: 'Resolution' }),
    "Won't Fix",
  )
}

// form is the form an action opened.
function form(name: string): HTMLElement {
  return screen.getByRole('form', { name })
}

test('a status change that needs nothing is made from the form', async () => {
  // Arrange
  const sent = trackerTakingWrites({}, 'Blocked')
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)
  await open('Change status')
  await choose('11')

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Change to Blocked' }))

  // Assert
  expect(await screen.findByText('Changed PROJ-1 to Blocked.')).toBeTruthy()
  expect(sent).toEqual([{ to: `POST ${changesPath}`, body: { transition_id: '11', fields: [] } }])
  expect(screen.queryByRole('form', { name: 'Change the status of PROJ-1' })).toBeNull()
})

test('a status change no longer offered once the changes are read again is no longer chosen', async () => {
  // Arrange
  // The changes are read again as the browser comes back online, and by then
  // the issue has moved on: Block is no longer among them.
  let reads = 0
  fakeApi({
    [issuePath]: detailOf(),
    [changesPath]: () => {
      reads += 1

      return Response.json(reads === 1 ? mockStatusChanges : mockStatusChanges.slice(1))
    },
  })
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)
  await open('Change status')
  await choose('11')

  // Act
  act(() => {
    onlineManager.setOnline(false)
    onlineManager.setOnline(true)
  })

  // Assert
  await waitFor(() => {
    expect(reads).toBe(2)
  })
  const fields = form('Change the status of PROJ-1')
  expect(await within(fields).findByRole('button', { name: 'Change status' })).toBeTruthy()
  expect(within(fields).queryByRole('button', { name: 'Change to Blocked' })).toBeNull()
})

test('a status change with fields shows its form and sends what is filled', async () => {
  // Arrange
  const sent = trackerTakingWrites()
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)
  await open('Change status')
  await choose('21')
  await filledForResolved()

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Change to Resolved' }))

  // Assert
  await waitFor(() => {
    expect(sent).toEqual([
      {
        to: `POST ${changesPath}`,
        body: {
          transition_id: '21',
          fields: [
            { id: 'duedate', text: '2026-10-09' },
            { id: 'fixVersions', option_ids: ['11'] },
            { id: 'resolution', option_id: '2' },
          ],
        },
      },
    ])
  })
})

test('a status change sent with a field it needs left empty is refused beside it', async () => {
  // Arrange
  const sent = trackerTakingWrites()
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)
  await open('Change status')
  await choose('21')
  const fields = form('Change the status of PROJ-1')
  await userEvent.type(within(fields).getByRole('textbox', { name: 'Due date' }), '2026-10-09')
  await userEvent.click(within(fields).getByRole('checkbox', { name: '1.5.0' }))

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Change to Resolved' }))

  // Assert
  const resolution = screen.getByRole('combobox', {
    name: 'Resolution',
    description: 'Resolution needs a value.',
  })
  expect(resolution.getAttribute('aria-invalid')).toBe('true')
  expect(sent).toEqual([])
})

test('a list field sent with nothing chosen is refused in the server’s words', async () => {
  // Arrange
  const sent = trackerTakingWrites()
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)
  await open('Change status')
  await choose('21')
  const fields = form('Change the status of PROJ-1')
  await userEvent.type(within(fields).getByRole('textbox', { name: 'Due date' }), '2026-10-09')
  await userEvent.selectOptions(
    within(fields).getByRole('combobox', { name: 'Resolution' }),
    "Won't Fix",
  )

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Change to Resolved' }))

  // Assert
  expect(
    screen.getByRole('group', {
      name: 'Fix versions',
      description: 'Fix versions needs at least one.',
    }),
  ).toBeTruthy()
  expect(sent).toEqual([])
})

test('a date field says the shape it takes, as the terminal and the refusal do', async () => {
  // Arrange
  trackerTakingWrites()
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)
  await open('Change status')

  // Act
  await choose('21')

  // Assert
  expect(
    within(form('Change the status of PROJ-1')).getByText('A date, written YYYY-MM-DD.'),
  ).toBeTruthy()
})

test('a status change only Jira can make says so and cannot be sent', async () => {
  // Arrange
  const sent = trackerTakingWrites()
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)
  await open('Change status')

  // Act
  await choose('31')

  // Assert
  expect(
    screen.getByText(
      "Split needs Component tree, which only Jira's own screen can fill; make this change in Jira.",
    ),
  ).toBeTruthy()
  expect(screen.getByRole('button', { name: 'Change to Split' }).hasAttribute('disabled')).toBe(
    true,
  )
  expect(sent).toEqual([])
})

test('a refused status change keeps the form and the focus, beside why', async () => {
  // Arrange
  trackerTakingWrites({
    [changesPath]: (_: URL, request: Request) =>
      request.method === 'GET'
        ? mockStatusChanges
        : Response.json(
            {
              title: 'Unprocessable',
              status: 422,
              detail: 'Due date is not a date the tracker takes',
              code: 'unprocessable',
            },
            { status: 422 },
          ),
  })
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)
  await open('Change status')
  await choose('21')
  await filledForResolved()

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Change to Resolved' }))

  // Assert
  expect((await screen.findByRole('alert')).textContent).toBe(
    'Due date is not a date the tracker takes',
  )
  expect(form('Change the status of PROJ-1')).toBeTruthy()
  expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Change to Resolved' }))
})

test('canceling a form sends nothing and hands focus back to its button', async () => {
  // Arrange
  const sent = trackerTakingWrites()
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)
  await open('Assign')
  await userEvent.type(screen.getByRole('textbox', { name: 'Assignee' }), 'sam.ortiz')

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))

  // Assert
  expect(sent).toEqual([])
  expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Assign' }))
})

test('an issue is assigned to the username typed', async () => {
  // Arrange
  const sent = trackerTakingWrites()
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)
  await open('Assign')
  await userEvent.type(screen.getByRole('textbox', { name: 'Assignee' }), 'sam.ortiz')

  // Act
  await userEvent.click(within(form('Assign PROJ-1')).getByRole('button', { name: 'Assign' }))

  // Assert
  expect(await screen.findByText('Assigned PROJ-1 to sam.ortiz.')).toBeTruthy()
  expect(sent).toEqual([{ to: `PUT ${issuePath}/assignee`, body: { assignee: 'sam.ortiz' } }])
})

test('work is logged with its note', async () => {
  // Arrange
  const sent = trackerTakingWrites()
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)
  await open('Log work')
  await userEvent.type(screen.getByRole('textbox', { name: 'Time spent' }), '2h')
  await userEvent.type(screen.getByRole('textbox', { name: 'Note' }), 'pairing on the redaction')

  // Act
  await userEvent.click(
    within(form('Log work on PROJ-1')).getByRole('button', { name: 'Log work' }),
  )

  // Assert
  expect(await screen.findByText('Logged 2h on PROJ-1.')).toBeTruthy()
  expect(sent).toEqual([
    {
      to: `POST ${issuePath}/worklog`,
      body: { time_spent: '2h', comment: 'pairing on the redaction' },
    },
  ])
})

test('a forge issue offers no worklog', async () => {
  // Arrange
  fakeApi({ '/api/issues/42': detailOf('forge') })

  // Act
  renderWithClient(<IssueDetailPanel issueKey="42" />)

  // Assert
  expect(await screen.findByRole('button', { name: 'Assign' })).toBeTruthy()
  expect(screen.getByRole('button', { name: 'Change status' })).toBeTruthy()
  expect(screen.queryByRole('button', { name: 'Log work' })).toBeNull()
})

test('status changes the server fails to read are asked for once more, then say why', async () => {
  // Arrange
  const requests = fakeApi({
    [issuePath]: detailOf(),
    [changesPath]: () =>
      Response.json(
        {
          title: 'Bad Gateway',
          status: 502,
          detail: 'the service could not be reached',
          code: 'unreachable',
        },
        { status: 502 },
      ),
  })
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)

  // Act
  await open('Change status')

  // Assert
  const alert = await screen.findByRole('alert', {}, { timeout: 3000 })
  expect(alert.textContent).toBe('the service could not be reached')
  expect(screen.getByRole('button', { name: 'Try again' })).toBeTruthy()
  const reads = requests.filter((request) => new URL(request.url).pathname === changesPath)
  expect(reads).toHaveLength(2)
})

test('status changes the tracker refuses say why at once, and are asked for no more', async () => {
  // Arrange
  const requests = fakeApi({
    [issuePath]: detailOf(),
    [changesPath]: () =>
      Response.json(
        {
          title: 'Forbidden',
          status: 403,
          detail: 'Jira refused: no permission',
          code: 'forbidden',
        },
        { status: 403, headers: { 'Content-Type': 'application/problem+json' } },
      ),
  })
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)

  // Act
  await open('Change status')

  // Assert
  expect((await screen.findByRole('alert')).textContent).toBe('Jira refused: no permission')
  const reads = requests.filter((request) => new URL(request.url).pathname === changesPath)
  expect(reads).toHaveLength(1)
})

test('Try again beside status changes never read keeps its place and focus while it reads', async () => {
  // Arrange
  const user = userEvent.setup()
  fakeApi({ [issuePath]: detailOf(), [changesPath]: refusedThenHeld('Jira refused the read') })
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)
  await open('Change status')
  const retry = await screen.findByRole('button', { name: 'Try again' })

  // Act
  await user.click(retry)

  // Assert
  expect(await tryAgainShown(retry)).toEqual(keptBeside('Jira refused the read'))
})

test('an issue the tracker offers no status change says so', async () => {
  // Arrange
  fakeApi({ [issuePath]: detailOf(), [changesPath]: [] })
  renderWithClient(<IssueDetailPanel issueKey="PROJ-1" />)

  // Act
  await open('Change status')

  // Assert
  expect(await screen.findByText('The tracker offers no status change for PROJ-1.')).toBeTruthy()
})
