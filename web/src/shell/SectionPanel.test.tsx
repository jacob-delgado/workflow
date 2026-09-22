import { render, screen } from '@testing-library/react'
import { vi } from 'vitest'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { SectionPanel } from './SectionPanel.tsx'
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
  render(<SectionPanel section={section} />)

  // Assert
  expect(screen.getByText(marker)).toBeTruthy()
})

test('routes the settings section to the config form', () => {
  // Arrange
  vi.stubEnv('VITE_MOCK', 'true')

  // Act
  renderWithClient(<SectionPanel section="settings" />)

  // Assert
  expect(screen.getByText(/loading the configuration/i)).toBeTruthy()
})
