import { screen } from '@testing-library/react'
import type { ReactElement } from 'react'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { IssueDetailPanel } from '@/features/issues/IssueDetailPanel.tsx'
import { IssuesPanel } from '@/features/issues/IssuesPanel.tsx'
import { RepositoriesPanel } from '@/features/repositories/RepositoriesPanel.tsx'
import { ReviewQueuePanel } from '@/features/reviewqueue/ReviewQueuePanel.tsx'
import { SettingsPanel } from '@/features/settings/SettingsPanel.tsx'
import { LocalData } from '@/features/settings/people/LocalData.tsx'
import { PeopleAndGroups } from '@/features/settings/people/PeopleAndGroups.tsx'
import { SummaryPanel } from '@/features/summary/SummaryPanel.tsx'
import { TasksPanel } from '@/features/tasks/TasksPanel.tsx'
import { useUiStore } from '@/shell/uiStore.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { vi } from 'vitest'

// Every read in flight is said one way: a status line that begins "Reading",
// whatever the section, so a screen reader hears each wait as it starts.

// neverAnswering holds every request open, so each read stays in flight.
function neverAnswering(): void {
  vi.stubGlobal(
    'fetch',
    vi.fn(() => new Promise(() => undefined)),
  )
}

const panels: [string, () => ReactElement][] = [
  ['Tasks', () => <TasksPanel />],
  ['Reviews', () => <ReviewQueuePanel />],
  ['Summary', () => <SummaryPanel />],
  ['Repositories', () => <RepositoriesPanel />],
  ['Settings', () => <SettingsPanel />],
  ['People and groups', () => <PeopleAndGroups />],
  ['Local data', () => <LocalData />],
  ['an issue', () => <IssueDetailPanel issueKey="PROJ-1" />],
]

test.each(panels)('%s says its read in flight as a Reading status', async (_, panel) => {
  // Arrange
  neverAnswering()

  // Act
  renderWithClient(panel())

  // Assert
  const waits = await screen.findAllByRole('status')
  expect(waits.map((wait) => wait.textContent)).toContainEqual(expect.stringMatching(/^Reading /))
})

test('Issues says another view in flight as a Reading status', async () => {
  // Arrange
  neverAnswering()
  useSnapshotStore.setState({ status: 'live', snapshot: makeSnapshot(), view: null })
  useUiStore.setState({ view: 'Sprint' })

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  const waits = await screen.findAllByRole('status')
  expect(waits.map((wait) => wait.textContent)).toContain('Reading the Sprint view…')
})
