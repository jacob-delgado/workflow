import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { Config } from '@/api/generated/types.gen.ts'
import { mockConfig } from '@/dev/mockConfig.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { SettingsPanel } from './SettingsPanel.tsx'

// savedConfigs serves the mockup's configuration and keeps each one saved.
function savedConfigs(): Config[] {
  const saved: Config[] = []
  fakeApi({
    '/api/config': async (_at: URL, asked: Request) => {
      if (asked.method === 'PUT') {
        saved.push((await asked.clone().json()) as Config)
      }

      return Response.json(saved.at(-1) ?? mockConfig, { headers: { ETag: '"read-1"' } })
    },
  })

  return saved
}

test('reading the Jira token from the keychain rides back through a save', async () => {
  // Arrange
  const saved = savedConfigs()
  const user = userEvent.setup()
  renderWithClient(<SettingsPanel />)
  await user.click(
    await screen.findByRole('checkbox', { name: /read the token from your keychain/i }),
  )

  // Act
  await user.click(screen.getByRole('button', { name: /save changes/i }))

  // Assert
  await screen.findByText(/saved/i)
  expect(saved.at(-1)?.jira.keychain).toBe(true)
})
