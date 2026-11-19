import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { Config } from '@/api/generated/types.gen.ts'
import { mockConfig } from '@/dev/mockConfig.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { anEdit, inSection } from '@/test/settings.ts'
import { SettingsPanel } from './SettingsPanel.tsx'

// refusedSlugLimit is the problem a save of a negative slug limit is refused
// with, naming the setting.
const refusedSlugLimit = {
  code: 'unprocessable',
  status: 422,
  title: 'Unprocessable content',
  detail: 'the configuration is not valid: invalid branch: slug_limit cannot be negative: -1',
  fields: [{ key: 'branch.slug_limit', detail: 'invalid branch: slug_limit cannot be negative' }],
}

// refusedViews is the problem a save is refused with while a view has no JQL,
// naming the list as a whole.
const refusedViews = {
  ...refusedSlugLimit,
  detail: 'the configuration is not valid: invalid view: each view needs a name and a jql',
  fields: [{ key: 'jira.views', detail: 'invalid view: each view needs a name and a jql' }],
}

// refusing is the problem a save is refused with, naming each setting given
// with why.
function refusing(...fields: [key: string, detail: string][]) {
  return {
    ...refusedSlugLimit,
    detail: 'the configuration is not valid',
    fields: fields.map(([key, detail]) => ({ key, detail })),
  }
}

// trailerRefused is the server's refusal of an issue trailer with a colon.
const trailerRefused: [string, string] = [
  'commit.refs_trailer',
  'invalid commit: refs_trailer is one word',
]

// intervalRefused is the server's refusal of a CI interval that is no duration.
const intervalRefused: [string, string] = [
  'timing.ci_interval',
  'invalid timing: ci_interval is no duration',
]

// A judge is the server's verdict on a save: the problem it refuses the save
// with, or null to take it.
type Judge = (sent: Config, saves: number) => object | null

// opened opens Settings over read, the mockup's configuration unless another
// is given, answering each save as judge says — the problem it refuses the
// save with, or else what it was sent — and returns the requests the page
// made.
async function opened(judge: Judge = () => null, read: Config = mockConfig): Promise<Request[]> {
  let saves = 0
  const requests = fakeApi({
    '/api/config': async (_at: URL, asked: Request) => {
      if (asked.method !== 'PUT') {
        return Response.json(read, { headers: { ETag: '"read-1"' } })
      }

      saves += 1
      const sent = (await asked.clone().json()) as Config
      const refusal = judge(sent, saves)

      return refusal === null ? Response.json(sent) : Response.json(refusal, { status: 422 })
    },
  })
  renderWithClient(<SettingsPanel />)
  await screen.findByLabelText('Base URL')

  return requests
}

// saveBar is the bar Save and its count are in.
function saveBar(): HTMLElement {
  return screen.getByRole('region', { name: 'Save' })
}

// save presses Save changes.
async function save(): Promise<void> {
  await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))
}

// saves counts the saves the page sent.
function saves(requests: Request[]): number {
  return requests.filter((request) => request.method === 'PUT').length
}

// aView adds a view named name with no JQL, and returns its row.
async function aView(name: string): Promise<HTMLElement> {
  await userEvent.click(screen.getByRole('button', { name: 'Add a view' }))
  const row = screen.getByRole('group', { name: 'View 1' })
  await userEvent.type(within(row).getByRole('textbox', { name: 'Name' }), name)

  return row
}

// prefixTaken is what the form says of a prefix row whose issue type an
// earlier row has, which it refuses before anything is sent.
const prefixTaken = 'A prefix of that issue type is already listed.'

// aTakenPrefix adds two prefixes for one issue type in Commits and branches,
// so the form refuses the second before a save is sent.
async function aTakenPrefix(): Promise<void> {
  await inSection('Commits and branches')
  for (const name of ['Prefix 1', 'Prefix 2']) {
    await userEvent.click(screen.getByRole('button', { name: 'Add a prefix' }))
    const row = screen.getByRole('group', { name })
    await userEvent.type(within(row).getByRole('textbox', { name: 'Issue type' }), 'Bug')
    await userEvent.type(within(row).getByRole('textbox', { name: 'Prefix' }), 'fix')
  }
}

// refusedList is the Views group, once it is described by the server's
// refusal of the list as a whole.
function refusedList(): Promise<HTMLElement> {
  return screen.findByRole('group', { name: 'Views', description: /each view needs a name/ })
}

test.each([
  { refused: [trailerRefused], said: '1 to fix in Commits and branches' },
  {
    refused: [trailerRefused, ['ui.comments_shown', 'invalid ui: comments_shown is negative']],
    said: '2 to fix in Commits and branches, Terminal and keys',
  },
])(
  'the bar says how many settings to fix, and in which sections: $said',
  async ({ refused, said }) => {
    // Arrange
    await opened(() => refusing(...(refused as [string, string][])))
    await anEdit()

    // Act
    await save()

    // Assert
    await waitFor(() => {
      expect(saveBar().textContent).toContain(said)
    })
  },
)

test('Show opens the section of the first setting to fix and gives that setting the focus', async () => {
  // Arrange
  await opened(() => refusing(intervalRefused))
  await anEdit()
  await save()

  // Act
  await userEvent.click(await screen.findByRole('button', { name: 'Show' }))

  // Assert
  expect(screen.getByRole('tab', { name: 'Advanced' }).getAttribute('aria-selected')).toBe('true')
  await waitFor(() => {
    expect(document.activeElement).toBe(screen.getByLabelText('CI interval'))
  })
})

test.each([
  { rows: 'no views', views: null, focused: { role: 'button', name: 'Add a view' } },
  {
    rows: 'a view',
    views: [{ name: 'Mine', jql: 'project = PROJ' }],
    focused: { role: 'textbox', name: 'Name' },
  },
])(
  'Show gives a list the server refused, with $rows, the focus on its first control',
  async ({ views, focused }) => {
    // Arrange
    await opened(() => refusedViews, { ...mockConfig, jira: { ...mockConfig.jira, views } })
    await anEdit()
    await save()
    await refusedList()
    await inSection('Advanced')

    // Act
    await userEvent.click(screen.getByRole('button', { name: 'Show' }))

    // Assert
    expect(screen.getByRole('tab', { name: 'Jira' }).getAttribute('aria-selected')).toBe('true')
    await waitFor(() => {
      expect(document.activeElement).toBe(screen.getByRole(focused.role, { name: focused.name }))
    })
  },
)

test("a setting the server refuses is marked beside itself with the server's reason", async () => {
  // Arrange
  await opened(() => refusedSlugLimit)
  await anEdit()

  // Act
  await save()

  // Assert
  await waitFor(() => {
    expect(saveBar().textContent).toContain('1 to fix in Commits and branches')
  })
  await inSection('Commits and branches')
  expect(
    screen.getByRole('spinbutton', {
      name: 'Slug limit',
      description: /slug_limit cannot be negative/,
    }),
  ).toBeTruthy()
})

test('a refusal the fields mark is not said a second time below the bar', async () => {
  // Arrange
  await opened(() => refusedSlugLimit)
  await anEdit()

  // Act
  await save()

  // Assert
  await screen.findByRole('button', { name: 'Show' })
  expect(screen.queryByText(refusedSlugLimit.detail)).toBeNull()
})

test('changing a refused setting takes its refusal away', async () => {
  // Arrange
  await opened(() => refusing(trailerRefused))
  await anEdit()
  await save()
  await inSection('Commits and branches')
  await screen.findByText(trailerRefused[1])

  // Act
  await userEvent.type(screen.getByLabelText('Issue trailer'), 'x')

  // Assert
  await waitFor(() => {
    expect(screen.queryByText(trailerRefused[1])).toBeNull()
  })
  expect(screen.getByLabelText('Issue trailer').hasAttribute('aria-invalid')).toBe(false)
})

test('a list the server refuses is marked as a whole, below its name', async () => {
  // Arrange
  await opened(() => refusedViews)
  await anEdit()

  // Act
  await save()

  // Assert
  expect(await refusedList()).toBeTruthy()
  expect(saveBar().textContent).toContain('1 to fix in Jira')
})

test('a list the server refused is sent again once it is fixed', async () => {
  // Arrange
  const requests = await opened((sent) =>
    (sent.jira.views ?? []).some((view) => view.jql === '') ? refusedViews : null,
  )
  const row = await aView('Mine')
  await save()
  await refusedList()
  await userEvent.type(
    within(row).getByRole('textbox', { name: 'JQL' }),
    'assignee = currentUser()',
  )

  // Act
  await save()

  // Assert
  await waitFor(() => {
    expect(saves(requests)).toBe(2)
  })
})

test('a list the server refused does not hold back the next save, which it judges again', async () => {
  // Arrange
  const requests = await opened((_sent, saved) => (saved === 1 ? refusedViews : null))
  await anEdit()
  await save()
  await refusedList()
  await anEdit()

  // Act
  await save()

  // Assert
  await waitFor(() => {
    expect(saves(requests)).toBe(2)
  })
})

test('a save refused before it is sent leaves the list the server refused marked', async () => {
  // Arrange
  const requests = await opened(() => refusedViews)
  await anEdit()
  await save()
  await refusedList()
  await aTakenPrefix()

  // Act
  await save()

  // Assert
  await waitFor(() => {
    expect(saveBar().textContent).toContain('2 to fix in Jira, Commits and branches')
  })
  await inSection('Jira')
  expect(await refusedList()).toBeTruthy()
  expect(saves(requests)).toBe(1)
})

test('a save refused before it is sent leaves the setting the server refused marked', async () => {
  // Arrange
  const requests = await opened(() => refusedSlugLimit)
  await anEdit()
  await save()
  await screen.findByRole('button', { name: 'Show' })
  await aTakenPrefix()

  // Act
  await save()

  // Assert
  await waitFor(() => {
    expect(saveBar().textContent).toContain('2 to fix in Commits and branches')
  })
  expect(
    screen.getByRole('spinbutton', {
      name: 'Slug limit',
      description: /slug_limit cannot be negative/,
    }),
  ).toBeTruthy()
  expect(saves(requests)).toBe(1)
})

test('a save refused before it is sent gives the setting just refused the focus', async () => {
  // Arrange
  await opened(() => refusedSlugLimit)
  await anEdit()
  await save()
  await screen.findByRole('button', { name: 'Show' })
  await aTakenPrefix()
  await inSection('Jira')

  // Act
  await save()

  // Assert
  const taken = within(screen.getByRole('group', { name: 'Prefix 2' })).getByRole('textbox', {
    name: 'Issue type',
    description: prefixTaken,
  })
  expect(
    screen.getByRole('tab', { name: /^Commits and branches/ }).getAttribute('aria-selected'),
  ).toBe('true')
  await waitFor(() => {
    expect(document.activeElement).toBe(taken)
  })
})

test('changing a list the server refused takes its refusal away', async () => {
  // Arrange
  await opened(() => refusedViews)
  const row = await aView('Mine')
  await save()
  await refusedList()

  // Act
  await userEvent.type(within(row).getByRole('textbox', { name: 'JQL' }), 'project = PROJ')

  // Assert
  await waitFor(() => {
    expect(
      screen.queryByRole('group', { name: 'Views', description: /each view needs a name/ }),
    ).toBeNull()
  })
})

test.each([
  { change: 'adding a row to', press: 'Add a view' },
  { change: 'removing a row from', press: 'Remove view 1' },
])('$change a list the server refused takes its refusal away', async ({ press }) => {
  // Arrange
  await opened(() => refusedViews)
  await aView('Mine')
  await save()
  await refusedList()

  // Act
  await userEvent.click(screen.getByRole('button', { name: press }))

  // Assert
  expect(
    await screen.findByRole('group', {
      name: 'Views',
      description: (description) => !description.includes('each view needs a name'),
    }),
  ).toBeTruthy()
})
