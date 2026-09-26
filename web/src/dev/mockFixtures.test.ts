import { mockConfig } from './mockConfig.ts'
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

// placeholdersIn lists every {name} a template holds.
function placeholdersIn(template: string): string[] {
  return template.match(/\{[^{}]*\}/g) ?? []
}

// The placeholders each template's renderer substitutes: BranchNaming.Name for
// the branch, and Announcement.rendered for the announcement. Any other would
// reach the branch name or the post literally.
const templates: [string, string, string[]][] = [
  ['branch.template', mockConfig.branch.template ?? '', ['{prefix}', '{key}', '{slug}']],
  [
    'messaging.announcement',
    mockConfig.messaging.announcement ?? '',
    ['{author}', '{noun}', '{title}', '{url}', '{key}', '{summary}', '{issue_url}'],
  ],
]

test.each(templates)(
  'the mock %s uses only its documented placeholders',
  (_, template, documented) => {
    // Act
    const used = placeholdersIn(template)

    // Assert
    expect(used).not.toHaveLength(0)
    expect(used.filter((placeholder) => !documented.includes(placeholder))).toEqual([])
  },
)
