import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { mockConfig } from '@/dev/mockConfig.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { editSomething, showSection } from '@/test/settings.ts'
import { SettingsPanel } from './SettingsPanel.tsx'

// A setting Settings refuses is marked beside itself: by the page's own rules
// before a save goes out, and by the server's when it refuses one.

// opened opens Settings, answering a save with what it was sent or, where
// refusal says, with it; it returns the requests the page made.
async function opened(refusal?: Response): Promise<Request[]> {
  const requests = fakeApi({
    '/api/config': async (_at: URL, asked: Request) => {
      if (asked.method !== 'PUT') {
        return Response.json(mockConfig, { headers: { ETag: '"read-1"' } })
      }

      return refusal ?? Response.json(await asked.clone().json(), { headers: { ETag: '"r2"' } })
    },
  })
  renderWithClient(<SettingsPanel />)
  await screen.findByLabelText('Base URL')

  return requests
}

// bar is the save bar.
function bar(): HTMLElement {
  return screen.getByRole('region', { name: 'Save' })
}

test('a branch template without {key} is refused beside it, and nothing is sent', async () => {
  // Arrange
  const requests = await opened()
  const user = userEvent.setup()
  await showSection('Commits and branches')
  const template = screen.getByRole('textbox', { name: 'Name template' })
  await user.clear(template)
  await user.type(template, 'feature/{{slug}')

  // Act
  await user.click(screen.getByRole('button', { name: 'Save changes' }))

  // Assert
  await waitFor(() => {
    expect(template.getAttribute('aria-invalid')).toBe('true')
  })
  expect(
    screen.getByRole('textbox', { name: 'Name template', description: /Must contain \{key\}\./ }),
  ).toBe(template)
  expect(bar().textContent).toContain('1 to fix in Commits and branches')
  expect(requests.some((request) => request.method === 'PUT')).toBe(false)
})

test('a refused save opens the section that holds the first refusal, focused on it', async () => {
  // Arrange
  await opened()
  const user = userEvent.setup()
  await showSection('Advanced')
  await user.type(screen.getByRole('textbox', { name: 'Request timeout' }), 'soon')
  await showSection('Jira')

  // Act
  await user.click(screen.getByRole('button', { name: 'Save changes' }))

  // Assert
  await waitFor(() => {
    expect(document.activeElement).toBe(screen.getByRole('textbox', { name: 'Request timeout' }))
  })
  expect(screen.getByRole('tab', { name: /^Advanced/ }).getAttribute('aria-selected')).toBe('true')
})

test('Show in the bar goes back to what is to fix', async () => {
  // Arrange
  await opened()
  const user = userEvent.setup()
  await showSection('Advanced')
  await user.type(screen.getByRole('textbox', { name: 'Request timeout' }), 'soon')
  await user.click(screen.getByRole('button', { name: 'Save changes' }))
  await showSection('Jira')

  // Act
  await user.click(screen.getByRole('button', { name: 'Show' }))

  // Assert
  await waitFor(() => {
    expect(document.activeElement).toBe(screen.getByRole('textbox', { name: 'Request timeout' }))
  })
})

test('a setting the server refuses is marked beside it, with its reason', async () => {
  // Arrange
  await opened(
    Response.json(
      {
        type: 'about:blank',
        title: 'Unprocessable content',
        status: 422,
        code: 'unprocessable',
        detail: 'invalid Jira project: lowercase',
        fields: [{ key: 'jira.project', detail: 'invalid Jira project: lowercase' }],
      },
      { status: 422, headers: { 'Content-Type': 'application/problem+json' } },
    ),
  )
  const user = userEvent.setup()
  await editSomething()

  // Act
  await user.click(screen.getByRole('button', { name: 'Save changes' }))

  // Assert
  expect(
    await screen.findByRole('textbox', {
      name: 'Project',
      description: /invalid Jira project: lowercase/,
    }),
  ).toBeTruthy()
  expect(bar().textContent).toContain('1 to fix in Jira')
})

test('a refused setting changed again is no longer marked', async () => {
  // Arrange
  await opened()
  const user = userEvent.setup()
  await showSection('Advanced')
  const timeout = screen.getByRole('textbox', { name: 'Request timeout' })
  await user.type(timeout, 'soon')
  await user.click(screen.getByRole('button', { name: 'Save changes' }))
  await waitFor(() => {
    expect(timeout.getAttribute('aria-invalid')).toBe('true')
  })

  // Act
  await user.clear(timeout)
  await user.type(timeout, '30s')

  // Assert
  await waitFor(() => {
    expect(timeout.getAttribute('aria-invalid')).toBeNull()
  })
})
