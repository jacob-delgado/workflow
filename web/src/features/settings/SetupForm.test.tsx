import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { SetupOffer, SetupRequest, SetupResult } from '@/api/generated/types.gen.ts'
import { useHealthStore } from '@/api/health.ts'
import { mockConfig } from '@/dev/mockConfig.ts'
import App from '@/App.tsx'
import { fakeApi } from '@/test/fakeApi.ts'
import { FakeEventSource } from '@/test/fakeEventSource.ts'
import { makeHealth } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { SettingsPanel } from './SettingsPanel.tsx'

// offer is a setup offered where no file applies, with a keychain for each
// file.
const offer: SetupOffer = {
  needed: true,
  places: [
    {
      place: 'repository',
      path: '/home/ana/src/api/.workflow.json',
      shown: '~/src/api/.workflow.json',
      keychain: true,
    },
    { place: 'home', path: '/home/ana/.workflow.json', shown: '~/.workflow.json', keychain: true },
  ],
}

// written is what the server answers a setup with.
const written: SetupResult = {
  path: '/home/ana/src/api/.workflow.json',
  shown: '~/src/api/.workflow.json',
  jira_user: 'Fred F. User (fred)',
  keychain: true,
  not_ignored: false,
  reopened: true,
}

// problem answers with a problem details object.
function problem(status: number, code: string, detail: string): Response {
  return Response.json({ type: 'about:blank', title: 'Refused', status, code, detail }, { status })
}

// noFile is the configuration's read where no file applies.
function noFile(): Response {
  return problem(404, 'not_found', 'no .workflow.json applies where the server works')
}

// firstRun answers as a server with no file: the configuration not found until
// a setup writes one, the offer, and each setup with answer, recording what
// each setup sent.
function firstRun(answer: (request: SetupRequest) => Response, offered = offer): SetupRequest[] {
  const sent: SetupRequest[] = []
  let set = false

  fakeApi({
    '/api/config': () => (set ? Response.json(mockConfig, { headers: { ETag: '"1"' } }) : noFile()),
    '/api/config/setup': async (_at: URL, asked: Request) => {
      if (asked.method === 'GET') {
        return offered
      }

      const request = (await asked.json()) as SetupRequest
      sent.push(request)
      const answered = answer(request)
      set = answered.ok

      return answered
    },
  })

  return sent
}

// answerJira fills Jira's address and token.
async function answerJira(user: ReturnType<typeof userEvent.setup>) {
  await user.type(
    await screen.findByRole('textbox', { name: 'Address' }),
    'https://jira.example.com',
  )
  await user.type(screen.getByLabelText('Personal access token'), 'typed-token')
}

test('with no file, Settings asks where one goes and what config init asks', async () => {
  // Arrange
  firstRun(() => Response.json(written))

  // Act
  renderWithClient(<SettingsPanel />)

  // Assert
  const form = await screen.findByRole('form', { name: 'Set up workflow' })
  const where = within(form).getByRole('group', { name: 'Where the file goes' })
  expect(
    within(where).getByRole('radio', { name: /~\/src\/api\/\.workflow\.json/ }),
  ).toHaveProperty('checked', true)
  expect(within(form).getByRole('textbox', { name: 'Address' })).toBeTruthy()
  expect(within(form).getByLabelText('Personal access token')).toHaveProperty('type', 'password')
  expect(within(form).getByRole('checkbox', { name: /keychain/ })).toHaveProperty('checked', true)
  expect(within(form).getByLabelText('Incoming webhook URL')).toHaveProperty('type', 'password')
  expect(screen.queryByRole('textbox', { name: 'Base URL' })).toBeNull()
})

test('setting up sends the answers, says what it wrote, and opens the file in Settings', async () => {
  // Arrange
  const user = userEvent.setup()
  const sent = firstRun(() => Response.json(written))
  renderWithClient(<SettingsPanel />)
  await answerJira(user)
  await user.click(screen.getByRole('radio', { name: /~\/\.workflow\.json/ }))

  // Act
  await user.click(screen.getByRole('button', { name: 'Write ~/.workflow.json' }))

  // Assert
  expect(await screen.findByText(/Wrote ~\/src\/api\/\.workflow\.json/)).toBeTruthy()
  expect(await screen.findByRole('textbox', { name: 'Base URL' })).toBeTruthy()
  expect(sent).toEqual([
    {
      place: 'home',
      jira_base_url: 'https://jira.example.com',
      jira_token: 'typed-token',
      webhook_url: '',
      keychain: true,
      keep_unchecked: false,
    },
  ])
})

test('the defaults in a repository keep its token in the keychain', async () => {
  // Arrange
  const user = userEvent.setup()
  const sent = firstRun(() => Response.json(written))
  renderWithClient(<SettingsPanel />)
  await answerJira(user)

  // Act
  await user.click(screen.getByRole('button', { name: 'Write ~/src/api/.workflow.json' }))

  // Assert
  expect(await screen.findByText(/Wrote ~\/src\/api\/\.workflow\.json/)).toBeTruthy()
  expect(sent.map((request) => [request.place, request.keychain])).toEqual([['repository', true]])
})

test('a file the keychain is not offered for keeps the token in it', async () => {
  // Arrange
  const user = userEvent.setup()
  const sent = firstRun(() => Response.json({ ...written, keychain: false }), {
    ...offer,
    places: offer.places.map((place) => ({ ...place, keychain: place.place === 'home' })),
  })
  renderWithClient(<SettingsPanel />)
  await answerJira(user)

  // Act
  await user.click(screen.getByRole('button', { name: 'Write ~/src/api/.workflow.json' }))

  // Assert
  expect(await screen.findByText(/Wrote ~\/src\/api\/\.workflow\.json/)).toBeTruthy()
  expect(sent.map((request) => [request.place, request.keychain])).toEqual([['repository', false]])
})

test('a check that does not pass says why and offers to write it anyway', async () => {
  // Arrange
  const user = userEvent.setup()
  const sent = firstRun((request) =>
    request.keep_unchecked
      ? Response.json({ ...written, jira_user: '' })
      : problem(422, 'check_failed', 'Jira did not accept the token; check it, or keep it anyway'),
  )
  renderWithClient(<SettingsPanel />)
  await answerJira(user)
  await user.click(screen.getByRole('button', { name: 'Write ~/src/api/.workflow.json' }))
  await within(screen.getByRole('form', { name: 'Set up workflow' })).findByRole('alert')

  // Act
  await user.click(screen.getByRole('button', { name: 'Write it anyway' }))

  // Assert
  await screen.findByRole('textbox', { name: 'Base URL' })
  expect(sent.map((request) => request.keep_unchecked)).toEqual([false, true])
})

test('the refusal of a check is said beside the write, which keeps the focus', async () => {
  // Arrange
  const user = userEvent.setup()
  firstRun(() =>
    problem(422, 'check_failed', 'Jira did not accept the token; check it, or keep it anyway'),
  )
  renderWithClient(<SettingsPanel />)
  await answerJira(user)

  // Act
  await user.click(screen.getByRole('button', { name: 'Write ~/src/api/.workflow.json' }))

  // Assert
  const form = screen.getByRole('form', { name: 'Set up workflow' })
  expect((await within(form).findByRole('alert')).textContent).toBe(
    'Jira did not accept the token; check it, or keep it anyway',
  )
  expect(document.activeElement).toBe(
    screen.getByRole('button', { name: 'Write ~/src/api/.workflow.json' }),
  )
})

test('a write anyway that is refused says why, with focus still on Write it anyway', async () => {
  // Arrange
  const user = userEvent.setup()
  firstRun((request) =>
    request.keep_unchecked
      ? problem(422, 'unprocessable', 'the file could not be written')
      : problem(422, 'check_failed', 'Jira did not accept the token; check it, or keep it anyway'),
  )
  renderWithClient(<SettingsPanel />)
  await answerJira(user)
  await user.click(screen.getByRole('button', { name: 'Write ~/src/api/.workflow.json' }))
  const anyway = await screen.findByRole('button', { name: 'Write it anyway' })

  // Act
  await user.click(anyway)

  // Assert
  await screen.findByText('the file could not be written')
  expect(document.activeElement).toBe(anyway)
})

test('an address that is no address is never offered to be written anyway', async () => {
  // Arrange
  const user = userEvent.setup()
  firstRun(() =>
    problem(
      422,
      'unprocessable',
      "Jira's address is not an https address, or http to this machine, without a username or password; type it again",
    ),
  )
  renderWithClient(<SettingsPanel />)
  await answerJira(user)

  // Act
  await user.click(screen.getByRole('button', { name: 'Write ~/src/api/.workflow.json' }))

  // Assert
  await within(screen.getByRole('form', { name: 'Set up workflow' })).findByRole('alert')
  expect(screen.queryByRole('button', { name: 'Write it anyway' })).toBeNull()
})

test('choosing the home directory offers the keychain, checked', async () => {
  // Arrange
  const user = userEvent.setup()
  firstRun(() => Response.json(written))
  renderWithClient(<SettingsPanel />)

  // Act
  await user.click(await screen.findByRole('radio', { name: /~\/\.workflow\.json/ }))

  // Assert
  expect(screen.getByRole('checkbox', { name: /keychain/ })).toHaveProperty('checked', true)
})

test('the keychain is not offered where there is none', async () => {
  // Arrange
  const user = userEvent.setup()
  firstRun(() => Response.json(written), {
    ...offer,
    places: offer.places.map((place) => ({ ...place, keychain: false })),
  })
  renderWithClient(<SettingsPanel />)

  // Act
  await user.click(await screen.findByRole('radio', { name: /~\/\.workflow\.json/ }))

  // Assert
  expect(screen.getByRole('radio', { name: /~\/\.workflow\.json/ })).toHaveProperty('checked', true)
  expect(screen.queryByRole('checkbox', { name: /keychain/ })).toBeNull()
})

test('under --dry-run writing the file is held back', async () => {
  // Arrange
  firstRun(() => Response.json(written))
  useHealthStore.setState({ health: makeHealth({ dry_run: true }) })

  // Act
  renderWithClient(<SettingsPanel />)

  // Assert
  expect(await screen.findByText(/held back while workflow runs with --dry-run/)).toBeTruthy()
  expect(screen.queryByRole('button', { name: /^Write/ })).toBeNull()
})

test('once set up, the page connects its stream to the file', async () => {
  // Arrange
  const user = userEvent.setup()
  firstRun(() => {
    // The server works with the new file and ends the stream, which the
    // browser reconnects.
    FakeEventSource.latest().emit('error', '')

    return Response.json(written)
  })
  renderWithClient(<App />)
  await user.click(screen.getByRole('button', { name: 'Settings' }))
  await answerJira(user)
  await user.click(screen.getByRole('button', { name: 'Write ~/src/api/.workflow.json' }))
  await screen.findByText(/Wrote/)

  // Act
  act(() => {
    FakeEventSource.latest().emit('open', '')
  })

  // Assert
  await waitFor(() => {
    expect(screen.getAllByRole('status').some((status) => status.textContent === 'Live')).toBe(true)
  })
})
