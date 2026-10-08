import type { ReviewFacet, ReviewRequest } from '@/api/generated/types.gen.ts'
import { makeReviewRequest } from '@/test/fixtures.ts'
import { admits, facetChoices, toggleFacet } from './reviewFacets.ts'

// The page counts, picks and admits over the facets the server describes; how
// a value is labeled and the order it is offered in are the server's, pinned by
// internal/forge/reviewfacets_test.go.

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
  }),
  makeReviewRequest({ number: 12, author: 'kwan', repository: 'example/other', ci: 'passed' }),
  makeReviewRequest({ number: 3, author: 'mira', repository: '', ci: 'none' }),
  makeReviewRequest({ number: 7, author: 'mira', repository: 'example/repo', ci: 'failed' }),
]

// held is the facet of kind holding value, as the requests above carry it.
function held(kind: ReviewFacet['kind'], value: string): ReviewFacet {
  const facet = requests
    .flatMap((request) => request.facets)
    .find((one) => one.kind === kind && one.value === value)
  if (facet === undefined) {
    throw new Error(`no request holds ${kind} ${value}`)
  }

  return facet
}

// A value the server offered once, which no request above holds.
const byAna: ReviewFacet = { kind: 'author', value: 'ana', label: 'by ana' }

// choiceLines is each choice as the filter names it: its label and count.
function choiceLines(choices: ReturnType<typeof facetChoices>): string[] {
  return choices.map((choice) => `${choice.value.label} ${String(choice.count)}`)
}

test.each([
  {
    name: 'two repositories list either',
    picked: () => [held('repository', 'example/other'), held('repository', 'example/repo')],
    want: [5, 12, 7],
  },
  {
    name: 'a repository and a CI state list both',
    picked: () => [held('repository', 'example/repo'), held('ci', 'failed')],
    want: [7],
  },
  {
    name: 'two CI states list either',
    picked: () => [held('ci', 'failed'), held('ci', 'none')],
    want: [3, 7],
  },
  {
    name: 'draft and an author list both',
    picked: () => [held('draft', 'draft'), held('author', 'kwan')],
    want: [5],
  },
])('$name', ({ picked, want }) => {
  // Act
  const got = requests.filter((request) => admits(picked(), request))

  // Assert
  expect(got.map((request) => request.number)).toEqual(want)
})

test('counts each value the server offers, in its order, leaving out what none holds', () => {
  // Arrange
  const order = [held('author', 'mira'), held('ci', 'failed'), byAna, held('repository', '')]

  // Act
  const choices = facetChoices(requests, order, [])

  // Assert
  expect(choiceLines(choices)).toEqual(['by mira 2', 'CI failed 1', 'no repository 1'])
})

test('offers a picked value the server no longer offers last, at zero', () => {
  // Act
  const choices = facetChoices(requests, [held('author', 'kwan')], [byAna])

  // Assert
  expect(choiceLines(choices)).toEqual(['by kwan 2', 'by ana 0'])
})

test('toggling a picked value unpicks it', () => {
  // Arrange
  const failed = held('ci', 'failed')
  const kwan = held('author', 'kwan')

  // Act
  const picked = toggleFacet([failed, kwan], failed)

  // Assert
  expect(picked).toEqual([kwan])
})
