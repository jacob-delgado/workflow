import { screen } from '@testing-library/react'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { SectionPanel } from './SectionPanel.tsx'
import { StreamStatus } from './StreamStatus.tsx'
import type { Section } from './uiStore.ts'

const readSections: [Section, string | RegExp][] = [
  ['issues', /no issues match/i],
  ['branch', /nothing to commit/i],
  ['review', /no open pull request/i],
  ['messaging', '#dev'],
]

test.each(readSections)('routes the %s section to its panel', (section, marker) => {
  // Arrange
  useSnapshotStore.setState({ status: 'live', snapshot: makeSnapshot() })

  // Act
  renderWithClient(<SectionPanel section={section} />)

  // Assert
  expect(screen.getByText(marker)).toBeTruthy()
})

test('routes the settings section to the config form', () => {
  // Act
  renderWithClient(<SectionPanel section="settings" />)

  // Assert
  expect(screen.getByText('Reading the configuration…')).toBeTruthy()
})

test.each(['issues', 'branch', 'review', 'messaging'] as const)(
  'the %s section says it is connecting in the one line every section shares',
  (section) => {
    // Act
    renderWithClient(<SectionPanel section={section} />)

    // Assert
    expect(screen.getByText('Connecting to workflow…')).toBeTruthy()
    expect(screen.getAllByText(/connecting/i)).toHaveLength(1)
  },
)

test('routes the reviews section to its queue, which reads without waiting on the stream', async () => {
  // Arrange
  fakeApi({ '/api/reviews': { available: true, requests: [] } })

  // Act
  renderWithClient(<SectionPanel section="reviews" />)

  // Assert
  expect(
    await screen.findAllByText('No pull requests are waiting on your review.'),
  ).not.toHaveLength(0)
  expect(screen.queryByText(/connecting/i)).toBeNull()
})

test('routes the tasks section to its list, which reads without waiting on the stream', async () => {
  // Arrange
  fakeApi({
    '/api/tasks': {
      available: true,
      reason: '',
      context: '',
      sync_available: false,
      said: '',
      tasks: [],
    },
  })

  // Act
  renderWithClient(<SectionPanel section="tasks" />)

  // Assert
  expect(
    await screen.findByText('No pending tasks. Add one above, or track an issue from Issues.'),
  ).toBeTruthy()
  expect(screen.queryByText(/connecting/i)).toBeNull()
})

test('a section waiting on a stream that dropped says what the header says', () => {
  // Arrange
  useSnapshotStore.setState({ status: 'reconnecting', snapshot: null })

  // Act
  renderWithClient(
    <>
      <StreamStatus />
      <SectionPanel section="issues" />
    </>,
  )

  // Assert
  expect(screen.getByRole('status').textContent).toBe('Reconnecting')
  expect(screen.getByText('Reconnecting to workflow…')).toBeTruthy()
  expect(screen.queryByText('Connecting to workflow…')).toBeNull()
})
