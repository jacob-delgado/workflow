import type { ReviewFacet, ReviewRequest } from '@/api/generated/types.gen.ts'
import { makeReviewRequest } from '@/test/fixtures.ts'
import { admits, facetChoices, toggleFacet } from './reviewFacets.ts'

// The page counts, picks and admits over the facets the server describes; how
// a value is labeled and the order it is offered in are the server's, pinned by
// internal/forge/reviewfacets_test.go.

// The values the requests below hold, each as the server labels it.
const exampleRepo: ReviewFacet = {
  kind: 'repository',
  value: 'example/repo',
  label: 'example/repo',
}
const exampleOther: ReviewFacet = {
  kind: 'repository',
  value: 'example/other',
  label: 'example/other',
}
const noRepository: ReviewFacet = { kind: 'repository', value: '', label: 'no repository' }
const ciRunning: ReviewFacet = { kind: 'ci', value: 'running', label: 'CI running' }
const ciPassed: ReviewFacet = { kind: 'ci', value: 'passed', label: 'CI passed' }
const ciNone: ReviewFacet = { kind: 'ci', value: 'none', label: 'CI none' }
const ciFailed: ReviewFacet = { kind: 'ci', value: 'failed', label: 'CI failed' }
const draft: ReviewFacet = { kind: 'draft', value: 'draft', label: 'draft' }
const ready: ReviewFacet = { kind: 'draft', value: 'ready', label: 'ready' }
const byKwan: ReviewFacet = { kind: 'author', value: 'kwan', label: 'by kwan' }
const byMira: ReviewFacet = { kind: 'author', value: 'mira', label: 'by mira' }

// The requests the terminal's facetsWorld queues, oldest first: #5, a draft
// by kwan in example/repo with CI running; #12 by kwan in example/other,
// passed; #3 by mira in no repository, with no CI; and #7 by mira in
// example/repo, failed.
const requests: ReviewRequest[] = [
  makeReviewRequest({
    number: 5,
    author: 'kwan',
    repository: 'example/repo',
    draft: true,
    ci: 'running',
    facets: [exampleRepo, ciRunning, draft, byKwan],
  }),
  makeReviewRequest({
    number: 12,
    author: 'kwan',
    repository: 'example/other',
    ci: 'passed',
    facets: [exampleOther, ciPassed, ready, byKwan],
  }),
  makeReviewRequest({
    number: 3,
    author: 'mira',
    repository: '',
    ci: 'none',
    facets: [noRepository, ciNone, ready, byMira],
  }),
  makeReviewRequest({
    number: 7,
    author: 'mira',
    repository: 'example/repo',
    ci: 'failed',
    facets: [exampleRepo, ciFailed, ready, byMira],
  }),
]

// A value the server offered once, which no request above holds.
const byAna: ReviewFacet = { kind: 'author', value: 'ana', label: 'by ana' }

// choiceLines is each choice as the filter names it: its label and count.
function choiceLines(choices: ReturnType<typeof facetChoices>): string[] {
  return choices.map((choice) => `${choice.value.label} ${String(choice.count)}`)
}

test.each([
  {
    name: 'two repositories list either',
    picked: [exampleOther, exampleRepo],
    want: [5, 12, 7],
  },
  {
    name: 'a repository and a CI state list both',
    picked: [exampleRepo, ciFailed],
    want: [7],
  },
  {
    name: 'two CI states list either',
    picked: [ciFailed, ciNone],
    want: [3, 7],
  },
  {
    name: 'draft and an author list both',
    picked: [draft, byKwan],
    want: [5],
  },
])('$name', ({ picked, want }) => {
  // Act
  const got = requests.filter((request) => admits(picked, request))

  // Assert
  expect(got.map((request) => request.number)).toEqual(want)
})

test('counts each value the server offers, in its order, leaving out what none holds', () => {
  // Arrange
  const order = [byMira, ciFailed, byAna, noRepository]

  // Act
  const choices = facetChoices(requests, order, [])

  // Assert
  expect(choiceLines(choices)).toEqual(['by mira 2', 'CI failed 1', 'no repository 1'])
})

test('offers a picked value the server no longer offers last, at zero', () => {
  // Act
  const choices = facetChoices(requests, [byKwan], [byAna])

  // Assert
  expect(choiceLines(choices)).toEqual(['by kwan 2', 'by ana 0'])
})

test('toggling a picked value unpicks it', () => {
  // Act
  const picked = toggleFacet([ciFailed, byKwan], ciFailed)

  // Assert
  expect(picked).toEqual([byKwan])
})
