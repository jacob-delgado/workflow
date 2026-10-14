import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
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

// opened opens Settings over the mockup's configuration, answering a save
// with refusal when one is given, else with what it was sent, and returns
// the requests the page made.
async function opened(refusal?: object): Promise<Request[]> {
  const requests = fakeApi({
    '/api/config': async (_at: URL, asked: Request) => {
      if (asked.method !== 'PUT') {
        return Response.json(mockConfig, { headers: { ETag: '"read-1"' } })
      }

      return refusal === undefined
        ? Response.json(await asked.clone().json())
        : Response.json(refusal, { status: 422 })
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
      description: new RegExp(said.replace(/[.{}]/g, '\\$&')),
    })
    expect(field.getAttribute('aria-invalid')).toBe('true')
    expect(requests.some((request) => request.method === 'PUT')).toBe(false)
  },
)

test('the bar says how many settings to fix, and in which section', async () => {
  // Arrange
  await opened()
  await inSection('Commits and branches')
  await userEvent.type(screen.getByLabelText('Issue trailer'), ':')
  await inSection('Jira')

  // Act
  await save()

  // Assert
  await waitFor(() => {
    expect(saveBar().textContent).toContain('1 to fix in Commits and branches')
  })
})

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

test("a setting the server refuses is marked beside itself with the server's reason", async () => {
  // Arrange
  await opened(refusedSlugLimit)
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
  await opened(refusedSlugLimit)
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
  await opened({
    ...refusedSlugLimit,
    detail: 'the configuration is not valid: invalid view: each view needs a name and a jql',
    fields: [{ key: 'jira.views', detail: 'invalid view: each view needs a name and a jql' }],
  })
  await anEdit()

  // Act
  await save()

  // Assert
  expect(
    await screen.findByRole('group', { name: 'Views', description: /each view needs a name/ }),
  ).toBeTruthy()
  expect(saveBar().textContent).toContain('1 to fix in Jira')
})
