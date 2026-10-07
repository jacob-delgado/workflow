import { screen, within } from '@testing-library/react'
import type { Config } from '@/api/generated/types.gen.ts'
import { mockConfig } from '@/dev/mockConfig.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { SettingsPanel } from './SettingsPanel.tsx'

// readsAs opens Settings over config, as the configuration a read finds.
function readsAs(config: Config): void {
  fakeApi({ '/api/config': () => Response.json(config, { headers: { ETag: '"read-1"' } }) })
  renderWithClient(<SettingsPanel />)
}

test("an empty title source is shown as the branch's oldest commit, the default", async () => {
  // Arrange
  readsAs({ ...mockConfig, pull_request: { title_source: '' } })

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

  // Act
  const service = await screen.findByRole('combobox', { name: 'Service' })

  // Assert
  expect(
    within(service).getByRole('option', { name: 'Slack (default)', selected: true }),
  ).toBeTruthy()
})
