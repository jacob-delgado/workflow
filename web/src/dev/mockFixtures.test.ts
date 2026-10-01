import { vi } from 'vitest'
import { zAnnouncementTagging, zPeople, zSlackDirectory } from '@/api/generated/zod.gen.ts'
import { previewAnnouncement } from '@/features/messaging/announceApi.ts'
import { mockConfig } from './mockConfig.ts'
import { mockIssueDetail } from './mockIssues.ts'
import { mockSnapshot } from './mockSnapshot.ts'
import { mockPeople, mockSlackGroups, mockSlackMembers, mockTagging } from './mockSlack.ts'

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

test('the mock preview is the mock announcement filled from the mock snapshot', async () => {
  // Arrange
  vi.stubEnv('VITE_MOCK', 'true')
  const template = mockConfig.messaging.announcement ?? ''
  const current = mockSnapshot.branches.find((branch) => branch.current)
  const issue = mockSnapshot.issues.issues.find((listed) => listed.key === current?.issue_key)
  const values: [string, string | undefined][] = [
    ['{author}', mockSnapshot.messaging.author],
    ['{title}', mockSnapshot.review.pull?.title],
    ['{url}', mockSnapshot.review.pull?.url],
    ['{key}', current?.issue_key],
    ['{summary}', issue?.summary],
  ]
  const filled = values.reduce(
    (text, [placeholder, value]) => text.replaceAll(placeholder, value ?? placeholder),
    template,
  )

  // Act
  const preview = await previewAnnouncement()

  // Assert
  expect(preview.text).toBe(filled)
  expect(placeholdersIn(preview.text)).toEqual([])
})

test('the mock Slack takes the shapes the server sends', () => {
  // Act
  const answers = [
    zAnnouncementTagging.safeParse(mockTagging()),
    zSlackDirectory.safeParse(mockSlackMembers()),
    zSlackDirectory.safeParse(mockSlackGroups()),
    zPeople.safeParse(mockPeople()),
  ]

  // Assert
  expect(answers.map((answer) => answer.error?.message)).toEqual([
    undefined,
    undefined,
    undefined,
    undefined,
  ])
})
