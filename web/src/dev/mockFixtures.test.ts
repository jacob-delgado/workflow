import { mockIssueDetail } from './mockIssues.ts'

// The mockup is only worth reading if its fixtures take the shapes the server
// sends: a value no real answer could hold shows a reader an inconsistency, or
// teaches a syntax, that exists only in the fake.

test('the mock issue detail names its assignee by display name, as Jira does', () => {
  // Act
  const detail = mockIssueDetail('PROJ-412')

  // Assert
  // Capitalized words, as a Jira displayName reads, and not a login.
  expect(detail.assignee).toMatch(/^\p{Lu}\p{Ll}+(?: \p{Lu}\p{Ll}+)+$/u)
})
