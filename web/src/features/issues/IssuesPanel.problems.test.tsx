import { screen } from '@testing-library/react'
import type { Problem } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { drawnMark, markShape } from '@/test/marks.tsx'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { IssuesPanel } from './IssuesPanel.tsx'

// jiraRefused is the problem the server sends for a token Jira turned down.
const jiraRefused: Problem = {
  type: 'https://jacob-delgado.github.io/workflow/docs/errors/#unprocessable',
  title: 'Unprocessable content',
  status: 422,
  detail: 'Jira did not accept the token; check it with workflow doctor',
  code: 'unprocessable',
}

test('says the issues could not be read, rather than that none match the view', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    view: null,
    snapshot: makeSnapshot({ problems: { issues: jiraRefused } }),
  })

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  const alert = screen.getByRole('alert')
  expect(alert.textContent).toBe(`The issues could not be read: ${jiraRefused.detail}`)
  expect(markShape(alert.parentElement ?? alert)).toBe(drawnMark('failed'))
  expect(screen.queryByText('No issues match this view.')).toBeNull()
})
