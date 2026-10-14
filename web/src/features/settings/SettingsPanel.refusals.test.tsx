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

// refusedList is the Views group, once it is described by the server's
// refusal of the list as a whole.
function refusedList(): Promise<HTMLElement> {
  return screen.findByRole('group', { name: 'Views', description: /each view needs a name/ })
}

test.each([
  ['Jira', 'Base URL', 'jira.example.com', 'An address such as https://jira.example.com.'],
  ['Commits and branches', 'Name template', 'feature/slug', 'Must contain {key}.'],
  ['Commits and branches', 'Issue trailer', 'Fixed by', 'A single word, without a colon.'],
  ['Advanced', 'CI interval', 'soon', 'A duration such as 20s or 3m.'],
])(
  'in %s, %s refuses %j beside itself, and nothing is sent',
  async (section, label, typed, said) => {
    // Arrange
    const requests = await opened()
    await inSection(section)
    await userEvent.clear(screen.getByLabelText(label))
    await userEvent.type(screen.getByLabelText(label), typed)

    // Act
    await save()

    // Assert
    const field = await screen.findByRole('textbox', {
      name: label,
      description: (description) => description.includes(said),
    })
    expect(field.getAttribute('aria-invalid')).toBe('true')
    expect(requests.some((request) => request.method === 'PUT')).toBe(false)
  },
)

test.each([
  {
    edits: [['Commits and branches', 'Issue trailer', ':']],
    said: '1 to fix in Commits and branches',
  },
  {
    edits: [
      ['Commits and branches', 'Issue trailer', ':'],
      ['Terminal and keys', 'Comments shown', '-1'],
    ],
    said: '2 to fix in Commits and branches, Terminal and keys',
  },
])(
  'the bar says how many settings to fix, and in which sections: $said',
  async ({ edits, said }) => {
    // Arrange
    await opened()
    for (const [section = '', label = '', typed = ''] of edits) {
      await inSection(section)
      await userEvent.clear(screen.getByLabelText(label))
      await userEvent.type(screen.getByLabelText(label), typed)
    }
    await inSection('Jira')

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
  await opened()
  await inSection('Advanced')
  await userEvent.type(screen.getByLabelText('CI interval'), 'x')
  await inSection('Jira')
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
  await opened()
  await inSection('Commits and branches')
  await userEvent.type(screen.getByLabelText('Issue trailer'), ':')
  await save()
  await screen.findByText('A single word, without a colon.')

  // Act
  await userEvent.type(screen.getByLabelText('Issue trailer'), '{Backspace}')

  // Assert
  await waitFor(() => {
    expect(screen.queryByText('A single word, without a colon.')).toBeNull()
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
  await inSection('Advanced')
  await userEvent.type(screen.getByLabelText('CI interval'), 'x')

  // Act
  await save()

  // Assert
  await waitFor(() => {
    expect(saveBar().textContent).toContain('2 to fix in Jira, Advanced')
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
  await inSection('Advanced')
  await userEvent.type(screen.getByLabelText('CI interval'), 'x')

  // Act
  await save()

  // Assert
  await waitFor(() => {
    expect(saveBar().textContent).toContain('2 to fix in Commits and branches, Advanced')
  })
  await inSection('Commits and branches')
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
  await inSection('Advanced')
  await userEvent.type(screen.getByLabelText('CI interval'), 'x')

  // Act
  await save()

  // Assert
  expect(screen.getByRole('tab', { name: 'Advanced' }).getAttribute('aria-selected')).toBe('true')
  await waitFor(() => {
    expect(document.activeElement).toBe(screen.getByLabelText('CI interval'))
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
