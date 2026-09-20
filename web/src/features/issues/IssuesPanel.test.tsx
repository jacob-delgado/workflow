import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { IssuesPanel } from './IssuesPanel.tsx'

function withIssues() {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      issues: {
        total: 2,
        start_at: 0,
        issues: [
          {
            key: 'PROJ-1',
            summary: 'Fix the token leak',
            status: 'In Progress',
            status_category: 'indeterminate',
            type: 'Bug',
            priority: 'High',
          },
          {
            key: 'PROJ-2',
            summary: 'Write the setup docs',
            status: 'To Do',
            status_category: 'new',
            type: 'Task',
          },
        ],
      },
    }),
  })
}

test('lists the issues in the view', () => {
  // Arrange
  withIssues()

  // Act
  render(<IssuesPanel />)

  // Assert
  expect(screen.getByText('Fix the token leak')).toBeTruthy()
  expect(screen.getByText('Write the setup docs')).toBeTruthy()
})

test('shows an issue detail when it is selected', async () => {
  // Arrange
  const user = userEvent.setup()
  withIssues()
  render(<IssuesPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: /fix the token leak/i }))

  // Assert
  expect(screen.getByRole('heading', { level: 2, name: /fix the token leak/i })).toBeTruthy()
})

test('shows the work story for the selected issue', async () => {
  // Arrange
  const user = userEvent.setup()
  withIssues()
  render(<IssuesPanel />)

  // Act
  // PROJ-2 has no priority, so its meta line omits the priority clause.
  await user.click(screen.getByRole('button', { name: /write the setup docs/i }))

  // Assert
  expect(screen.getByRole('heading', { name: /work story/i })).toBeTruthy()
  expect(screen.getByText('Announce')).toBeTruthy()
})

test('prompts to connect before any snapshot arrives', () => {
  // Act
  render(<IssuesPanel />)

  // Assert
  expect(screen.getByText(/connecting/i)).toBeTruthy()
})

test('says when no issues match the view', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ issues: { total: 0, start_at: 0, issues: [] } }),
  })

  // Act
  render(<IssuesPanel />)

  // Assert
  expect(screen.getByText(/no issues match/i)).toBeTruthy()
})
