import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { Config } from '@/api/generated/types.gen.ts'
import { mockConfig } from '@/dev/mockConfig.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { SettingsPanel } from './SettingsPanel.tsx'
import { showSection } from '@/test/settings.ts'

// readsAs opens Settings over config, as the configuration a read finds.
function readsAs(config: Config): void {
  fakeApi({ '/api/config': () => Response.json(config, { headers: { ETag: '"read-1"' } }) })
  renderWithClient(<SettingsPanel />)
}

test("an empty title source is shown as the branch's oldest commit, the default", async () => {
  // Arrange
  readsAs({ ...mockConfig, pull_request: { title_source: '' } })
  await showSection('Forge and pull requests')

  // Act
  const titleSource = await screen.findByRole('combobox', { name: 'Title source' })

  // Assert
  expect(
    within(titleSource).getByRole('option', {
      name: "The branch's oldest commit (default)",
      selected: true,
    }),
  ).toBeTruthy()
})

test('an empty messaging service is shown as Slack, the default', async () => {
  // Arrange
  readsAs({ ...mockConfig, messaging: { ...mockConfig.messaging, kind: '' } })
  await showSection('Messaging')

  // Act
  const service = await screen.findByRole('combobox', { name: 'Service' })

  // Assert
  expect(
    within(service).getByRole('option', { name: 'Slack (default)', selected: true }),
  ).toBeTruthy()
})

test('a service written as slack is shown as the default it means', async () => {
  // Arrange
  readsAs({ ...mockConfig, messaging: { ...mockConfig.messaging, kind: 'slack' } })
  await showSection('Messaging')

  // Act
  const service = await screen.findByRole('combobox', { name: 'Service' })

  // Assert
  expect(
    within(service).getByRole('option', { name: 'Slack (default)', selected: true }),
  ).toBeTruthy()
})

test('announcing a pull request opened here once its CI passes is on, and saves off', async () => {
  // Arrange
  const saved: Config[] = []
  fakeApi({
    '/api/config': async (_at: URL, asked: Request) => {
      if (asked.method === 'PUT') {
        saved.push((await asked.clone().json()) as Config)
      }

      return Response.json(saved.at(-1) ?? mockConfig, { headers: { ETag: '"read-1"' } })
    },
  })
  const user = userEvent.setup()
  renderWithClient(<SettingsPanel />)
  await showSection('Messaging')
  const armed = await screen.findByRole('checkbox', {
    name: 'Announce a pull request opened here once its CI passes',
  })
  const wasOn = (armed as HTMLInputElement).checked
  await user.click(armed)

  // Act
  await user.click(screen.getByRole('button', { name: /save changes/i }))

  // Assert
  await screen.findByText(/saved/i)
  expect(wasOn).toBe(true)
  expect(saved.at(-1)?.messaging.auto_announce).toBe(false)
})
