import type { Issue, TasksSummary } from '@/api/generated/types.gen.ts'
import { makeTask } from '@/test/fixtures.ts'
import { admits, marksOf, placeChoices, togglePlace, type Place } from './issuePlaces.ts'

// The issues the terminal's placesWorld lists: one Intake, two Fixing and two
// In development; a branch names PROJ-504 and a started task PROJ-503.
function issueIn(key: string, status: string, category: Issue['status_category']): Issue {
  return {
    key,
    tracker: 'jira',
    summary: `Summary of ${key}`,
    status,
    status_category: category,
    type: 'Task',
  }
}

const issues = [
  issueIn('PROJ-501', 'Intake', 'new'),
  issueIn('PROJ-502', 'Fixing', 'indeterminate'),
  issueIn('PROJ-503', 'Fixing', 'indeterminate'),
  issueIn('PROJ-504', 'In development', 'indeterminate'),
  issueIn('PROJ-505', 'In development', 'indeterminate'),
]

const branchKeys = new Set(['PROJ-504'])

const tasks: TasksSummary = {
  available: true,
  reason: '',
  linked: [makeTask({ issue_key: 'PROJ-503', start: '2026-09-30T09:00:00Z' })],
}

const status = (name: string): Place => ({ kind: 'status', name })
const mark = (name: string): Place => ({ kind: 'mark', name })

function listed(picked: Place[]): string[] {
  return issues
    .filter((issue) => admits(picked, issue, marksOf(issue, branchKeys, tasks)))
    .map((issue) => issue.key)
}

// Twins of TestStatusesOrAndMarksAnd in internal/tui/issueplaces_test.go.
test.each([
  {
    name: 'two statuses list either',
    picked: [status('Intake'), status('Fixing')],
    want: ['PROJ-501', 'PROJ-502', 'PROJ-503'],
  },
  {
    name: 'a status and a mark list both',
    picked: [status('In development'), mark('in flight')],
    want: ['PROJ-504'],
  },
  {
    name: 'two marks list either',
    picked: [mark('in flight'), mark('task active')],
    want: ['PROJ-503', 'PROJ-504'],
  },
])('$name', ({ picked, want }) => {
  // Act
  const got = listed(picked)

  // Assert
  expect(got).toEqual(want)
})

test('offers statuses by category, then the marks, each with its count', () => {
  // Act
  const choices = placeChoices(issues, (issue) => marksOf(issue, branchKeys, tasks), [])

  // Assert
  expect(choices.map((choice) => `${choice.value.name} ${String(choice.count)}`)).toEqual([
    'Intake 1',
    'Fixing 2',
    'In development 2',
    'in flight 1',
    'task active 1',
  ])
})

// Twin of TestAPickedPlaceWithNoIssuesStaysOfferedAtZero.
test('a picked place with no issues stays offered at zero', () => {
  // Act
  const choices = placeChoices(issues.slice(0, 1), (issue) => marksOf(issue, branchKeys, tasks), [
    status('Fixing'),
  ])

  // Assert
  expect(choices.map((choice) => `${choice.value.name} ${String(choice.count)}`)).toContain(
    'Fixing 0',
  )
})

// Twin of TestWithoutTaskwarriorNoTaskPlacesAreOffered.
test('without Taskwarrior no task places are offered', () => {
  // Arrange
  const none: TasksSummary = { ...tasks, available: false }

  // Act
  const choices = placeChoices(issues, (issue) => marksOf(issue, branchKeys, none), [])

  // Assert
  expect(choices.map((choice) => choice.value.name)).not.toContain('task active')
})

test('toggling a picked place unpicks it', () => {
  // Act
  const picked = togglePlace(togglePlace([], status('Fixing')), status('Fixing'))

  // Assert
  expect(picked).toEqual([])
})

// Twin of TestAStatusOutsideTheThreeCategoriesIsStillOffered.
test('a status outside the three categories is still offered', () => {
  // Arrange
  const parked = {
    ...issueIn('PROJ-506', 'Parked', 'new'),
    status_category: 'undefined' as Issue['status_category'],
  }

  // Act
  const choices = placeChoices(
    [...issues, parked],
    (issue) => marksOf(issue, branchKeys, tasks),
    [],
  )

  // Assert
  expect(choices.map((choice) => `${choice.value.name} ${String(choice.count)}`)).toContain(
    'Parked 1',
  )
})

// Twin of TestTheWherePickerOffersTheForgesIssues.
test("the Where picker offers the forge's issues", () => {
  // Arrange
  const forgeIssue = { ...issueIn('57', 'Open', 'new'), tracker: 'forge' as const }

  // Act
  const choices = placeChoices([forgeIssue], (issue) => marksOf(issue, new Set(), tasks), [])

  // Assert
  expect(choices.map((choice) => `${choice.value.name} ${String(choice.count)}`)).toContain(
    'forge issue 1',
  )
})
