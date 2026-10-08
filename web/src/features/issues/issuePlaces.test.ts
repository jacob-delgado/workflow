import { z } from 'zod'
import type { Issue, TasksSummary } from '@/api/generated/types.gen.ts'
import { makeTask } from '@/test/fixtures.ts'
import { twinCases } from '@/test/twinCases.ts'
import {
  admits,
  marksOf,
  placeChoices,
  standingMarks,
  togglePlace,
  type Place,
} from './issuePlaces.ts'

// The cases internal/places answers to as well.
const place = z.strictObject({ kind: z.enum(['status', 'mark']), name: z.string() })
const corpus = twinCases(
  'testdata/twins/places.json',
  z.strictObject({
    about: z.string(),
    issues: z.array(
      z.strictObject({
        key: z.string(),
        tracker: z.enum(['jira', 'forge']),
        status: z.string(),
        category: z.string(),
        marks: z.array(z.string()),
      }),
    ),
    marks: z.array(
      z.strictObject({
        name: z.string(),
        standing: z.strictObject({ in_flight: z.boolean(), task: z.string(), forge: z.boolean() }),
        want: z.array(z.string()),
      }),
    ),
    admits: z.array(
      z.strictObject({ name: z.string(), picked: z.array(place), want: z.array(z.string()) }),
    ),
    choices: z.array(
      z.strictObject({
        name: z.string(),
        issues: z.array(z.string()),
        picked: z.array(place),
        want: z.array(place.extend({ count: z.number() })),
      }),
    ),
  }),
)

type CaseIssue = (typeof corpus.issues)[number]

function issueOf(issue: CaseIssue): Issue {
  return {
    key: issue.key,
    tracker: issue.tracker,
    summary: `Summary of ${issue.key}`,
    status: issue.status,
    status_category: issue.category as Issue['status_category'],
    type: 'Task',
  }
}

function caseIssue(key: string): CaseIssue {
  const found = corpus.issues.find((issue) => issue.key === key)
  if (found === undefined) {
    throw new Error(`places.json names ${key}, which its issues do not hold`)
  }

  return found
}

function caseMarks(issue: Issue): string[] {
  return caseIssue(issue.key).marks
}

test.each(corpus.marks)('$name', ({ standing, want }) => {
  // Act
  const marks = standingMarks({
    inFlight: standing.in_flight,
    task: standing.task,
    forge: standing.forge,
  })

  // Assert
  expect(marks).toEqual(want)
})

test.each(corpus.admits)('$name', ({ picked, want }) => {
  // Act
  const listed = corpus.issues
    .filter((issue) => admits(picked, issueOf(issue), issue.marks))
    .map((issue) => issue.key)

  // Assert
  expect(listed).toEqual(want)
})

test.each(corpus.choices)('$name', ({ issues, picked, want }) => {
  // Arrange
  const listed = issues.map((key) => issueOf(caseIssue(key)))

  // Act
  const choices = placeChoices(listed, caseMarks, picked)

  // Assert
  expect(choices.map(({ value, count }) => ({ ...value, count }))).toEqual(want)
})

const tasks: TasksSummary = {
  available: true,
  reason: '',
  linked: [makeTask({ issue_key: 'PROJ-503', start: '2026-09-30T09:00:00Z', state: 'started' })],
}

test('marks read the branches, the linked tasks and the tracker', () => {
  // Arrange
  const branched = issueOf({ ...caseIssue('PROJ-503'), tracker: 'forge' })

  // Act
  const marks = marksOf(branched, new Set(['PROJ-503']), tasks)

  // Assert
  expect(marks).toEqual(['in flight', 'task active', 'forge issue'])
})

test('without Taskwarrior no task places are offered', () => {
  // Arrange
  const none: TasksSummary = { ...tasks, available: false }

  // Act
  const marks = marksOf(issueOf(caseIssue('PROJ-503')), new Set(), none)

  // Assert
  expect(marks).toEqual([])
})

test('toggling a picked place unpicks it', () => {
  // Arrange
  const fixing: Place = { kind: 'status', name: 'Fixing' }

  // Act
  const picked = togglePlace(togglePlace([], fixing), fixing)

  // Assert
  expect(picked).toEqual([])
})
