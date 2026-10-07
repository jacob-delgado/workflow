import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { Config } from '@/api/generated/types.gen.ts'
import { mockConfig } from '@/dev/mockConfig.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { SettingsPanel } from './SettingsPanel.tsx'

// opened opens Settings over config, answering a save with what it was sent,
// and returns the requests the page made.
async function opened(config: Config): Promise<Request[]> {
  const requests = fakeApi({
    '/api/config': async (_at: URL, asked: Request) => {
      const answer: unknown = asked.method === 'PUT' ? await asked.clone().json() : config

      return Response.json(answer, { headers: { ETag: '"read-1"' } })
    },
  })
  renderWithClient(<SettingsPanel />)
  await screen.findByLabelText('Base URL')

  return requests
}

// row is the entry of a list named name, as "Header 2".
function row(name: string) {
  return within(screen.getByRole('group', { name }))
}

// typedRow adds a row with Add, and types each of its fields, by label.
async function typedRow(add: string, name: string, typed: Record<string, string>): Promise<void> {
  await userEvent.click(screen.getByRole('button', { name: add }))
  for (const [label, text] of Object.entries(typed)) {
    await userEvent.type(row(name).getByLabelText(label), text)
  }
}

const withHeader: Config = {
  ...mockConfig,
  jira: { ...mockConfig.jira, headers: { 'CF-Access-Client-Secret': '****abcd' } },
}

const headerTaken = 'A header of that name is already listed.'

test('a header named as a stored one in another case is refused at its name, and nothing is sent', async () => {
  // Arrange
  const requests = await opened(withHeader)
  await typedRow('Add a header', 'Header 2', { Name: 'cf-access-client-secret', Value: 'x-1' })

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))

  // Assert
  const name = await row('Header 2').findByRole('textbox', {
    name: 'Name',
    description: headerTaken,
  })
  expect(document.activeElement).toBe(name)
  expect(requests.some((request) => request.method === 'PUT')).toBe(false)
})

test('two new headers of one name are refused, rather than one silently dropped', async () => {
  // Arrange
  const requests = await opened(mockConfig)
  await typedRow('Add a header', 'Header 1', { Name: 'X-Team', Value: 'one-1' })
  await typedRow('Add a header', 'Header 2', { Name: 'X-Team', Value: 'two-2' })

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))

  // Assert
  await row('Header 2').findByRole('textbox', { name: 'Name', description: headerTaken })
  expect(requests.some((request) => request.method === 'PUT')).toBe(false)
})

test('a prefix for an issue type already listed in another case is refused at its type', async () => {
  // Arrange
  const requests = await opened(mockConfig)
  await typedRow('Add a prefix', 'Prefix 1', { 'Issue type': 'Bug', Prefix: 'fix' })
  await typedRow('Add a prefix', 'Prefix 2', { 'Issue type': ' bug', Prefix: 'bugfix' })

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))

  // Assert
  await row('Prefix 2').findByRole('textbox', {
    name: 'Issue type',
    description: 'A prefix of that issue type is already listed.',
  })
  expect(requests.some((request) => request.method === 'PUT')).toBe(false)
})

test('renaming the second header clears its refusal and the save goes through', async () => {
  // Arrange
  const requests = await opened(withHeader)
  await typedRow('Add a header', 'Header 2', { Name: 'cf-access-client-secret', Value: 'x-1' })
  await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))
  const name = await row('Header 2').findByRole('textbox', {
    name: 'Name',
    description: headerTaken,
  })
  await userEvent.clear(name)
  await userEvent.type(name, 'X-Team')

  // Act
  await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))

  // Assert
  await screen.findByText('Saved.')
  const put = requests.find((request) => request.method === 'PUT')
  const sent = (await put?.json()) as Config
  expect(sent.jira.headers).toEqual({ 'CF-Access-Client-Secret': '****abcd', 'X-Team': 'x-1' })
})
