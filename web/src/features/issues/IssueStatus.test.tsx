import { render, screen } from '@testing-library/react'
import { drawnMark, markShape } from '@/test/marks.tsx'
import { IssueStatus } from './IssueStatus.tsx'

test('draws a status Jira files under no category with the unknown mark', () => {
  // Act
  render(
    <p>
      <IssueStatus category="unknown" label="Parked" />
    </p>,
  )

  // Assert
  expect(markShape(screen.getByText('Parked'))).toBe(drawnMark('unknown'))
})
