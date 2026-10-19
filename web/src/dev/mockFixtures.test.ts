import type { ActivityDay } from '@/api/generated/types.gen.ts'
import {
  zActivity,
  zAnnouncementTagging,
  zPeople,
  zSlackDirectory,
} from '@/api/generated/zod.gen.ts'
import { mockActivity } from './mockActivity.ts'
import { mockAnnouncement } from './mockAnnouncement.ts'
import { mockConfig } from './mockConfig.ts'
import { mockIssueDetail } from './mockIssues.ts'
import { mockSnapshot } from './mockSnapshot.ts'
import { mockPeople, mockSlackGroups, mockSlackMembers, mockTagging } from './mockSlack.ts'
import { mockTasksSummary } from './mockTasks.ts'

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

// The placeholders BranchNaming.Name substitutes in the branch template; any
// other would reach the branch name literally.
const branchPlaceholders = ['{prefix}', '{key}', '{slug}']

test('the mock branch.template uses only its documented placeholders', () => {
  // Act
  const used = placeholdersIn(mockConfig.branch.template ?? '')

  // Assert
  expect(used).not.toHaveLength(0)
  expect(used.filter((placeholder) => !branchPlaceholders.includes(placeholder))).toEqual([])
})

test('the mock announcement is the built-in Slack message, as the terminal demo posts it', () => {
  // Act
  const { text } = mockAnnouncement

  // Assert
  // internal/messaging's defaultText, for the pull request and issue the
  // terminal's demo world (internal/demo) announces too.
  expect(text).toBe(
    'ana.lopez opened a pull request: ' +
      '<https://github.com/acme/api/pull/128|fix(config): redact tokens in the request log>\n' +
      '<https://jira.example.com/browse/PROJ-412|PROJ-412> Redact tokens in the request log',
  )
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

// amountOf is how many things were done on a day.
function amountOf(day: ActivityDay): number {
  return day.hours.reduce((sum, hour) => sum + hour.items.length, 0)
}

test('the mock month spreads what was done over its weekdays up to today, busy and quiet', () => {
  // Act
  const month = mockActivity({ from: '2026-09-01', to: '2026-09-30' })

  // Assert
  expect(zActivity.safeParse(month).error?.message).toBeUndefined()
  const days = month.years.flatMap((year) => year.months.flatMap((each) => each.days))
  expect(new Set(days.map(amountOf)).size).toBeGreaterThanOrEqual(3)
  expect(days.filter((day) => day.date > month.today)).toEqual([])
  expect(days.filter((day) => ['Saturday', 'Sunday'].includes(day.weekday))).toEqual([])
})

test('a mock day is named for the weekday its date falls on', () => {
  // Act
  const monday = mockActivity({ from: '2026-09-14', to: '2026-09-14' })

  // Assert
  expect(monday.years[0]?.months[0]?.days[0]?.weekday).toBe('Monday')
})

// workedIssue is the issue the mock branch is for, as the mock issues list
// shows it.
function workedIssue() {
  const current = mockSnapshot.branches.find((branch) => branch.current)

  return mockSnapshot.issues.issues.find((listed) => listed.key === current?.issue_key)
}

test('the mock task tracking an issue is named as workflow names one, for its key and summary', () => {
  // Arrange
  const issue = workedIssue()

  // Act
  const task = mockTasksSummary.active

  // Assert
  expect(task?.description).toBe(`${issue?.key ?? ''}: ${issue?.summary ?? ''}`)
  expect(task?.searchable[0]).toBe(task?.description.toLowerCase())
})

// lastWorkingDay is every item of the mock Summary's default day.
function lastWorkingDay() {
  const days = mockActivity(null).years.flatMap((year) =>
    year.months.flatMap((month) => month.days),
  )

  return days.flatMap((day) => day.hours.flatMap((hour) => hour.items))
}

test('the commit the mock Summary tells of is merged, so not one the mock branch holds', () => {
  // Act
  const committed = lastWorkingDay().find((item) => item.source === 'git')

  // Assert
  // internal/demo's activity.go: yesterday's refactor, merged into main.
  expect(committed?.title).toBe('refactor(config): route every write through Redact')
  const held = mockSnapshot.branch.commits.filter((commit) =>
    commit.hash.startsWith(committed?.ref ?? '-'),
  )
  expect(held).toEqual([])
})

test('the mock branch is named for its issue, as the branch template names it', () => {
  // Arrange
  const issue = workedIssue()
  const slug = (issue?.summary ?? '').toLowerCase().replace(/[^a-z0-9]+/g, '-')

  // Act
  const { name } = mockSnapshot.branch

  // Assert
  expect(name).toBe(
    (mockConfig.branch.template ?? '')
      .replace('{prefix}', 'fix')
      .replace('{key}', issue?.key ?? '')
      .replace('{slug}', slug),
  )
  expect(mockSnapshot.branch.upstream).toBe(`origin/${name}`)
})

test('the mock branch starts from the commit the terminal demo pushes, and is ahead by those after it', () => {
  // Act
  const { commits, ahead, head } = mockSnapshot.branch

  // Assert
  // internal/demo's first commit, which its recording pushes.
  expect(commits.at(-1)).toEqual({
    hash: expect.stringMatching(/^4f2c9e1/) as string,
    subject: 'fix(config): redact tokens in the request log',
    unpushed: false,
  })
  expect(ahead).toBe(commits.filter((commit) => commit.unpushed).length)
  expect(commits[0]?.hash.startsWith(head)).toBe(true)
})

test('the mock branch is behind nothing on its own upstream, so Push branch sends it as it is', () => {
  // Act
  const { behind } = mockSnapshot.branch

  // Assert
  expect(behind).toBe(0)
})

test('the mock Summary moves the worked issue into the status the mock issues list shows', () => {
  // Arrange
  const issue = workedIssue()

  // Act
  const moved = lastWorkingDay().find((item) => item.source === 'jira' && item.ref === issue?.key)

  // Assert
  expect(moved?.title).toBe(`${issue?.summary ?? ''}, to ${issue?.status ?? ''}`)
})

test('the mock issues are those of the default view, so none is done', () => {
  // Act
  const listed = mockSnapshot.issues.issues

  // Assert
  // The default view is the issues assigned to you and not done
  // (internal/jira's AssignedToMe), as the terminal demo's list is too.
  expect(listed).not.toHaveLength(0)
  expect(listed.filter((issue) => issue.status_category === 'done')).toEqual([])
})

test("the mock working tree is edits since the branch's commits, so adds no file of theirs again", () => {
  // Act
  const { changes } = mockSnapshot.changes

  // Assert
  // The terminal demo commits the morning's new file, the request log's
  // test, so what the branch holds since is edits to files already there.
  expect(changes).not.toHaveLength(0)
  expect(changes.filter((change) => change.kind === 'new')).toEqual([])
})
