import type { ReviewRequest } from '@/api/generated/types.gen.ts'
import { makeReviewRequest } from '@/test/fixtures.ts'
import { admits, facetChoices, facetLabel, toggleFacet, type Facet } from './reviewFacets.ts'

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

const repository = (value: string): Facet => ({ kind: 'repository', value })
const ci = (value: string): Facet => ({ kind: 'ci', value })
const draft = (value: string): Facet => ({ kind: 'draft', value })
const author = (value: string): Facet => ({ kind: 'author', value })

function listed(picked: Facet[]): number[] {
  return requests.filter((request) => admits(picked, request)).map((request) => request.number)
}

// Twins of TestFacetsWidenWithinAndNarrowTogether in
// internal/tui/reviewfacets_test.go.
test.each([
  {
    name: 'two repositories list either',
    picked: [repository('example/other'), repository('example/repo')],
    want: [5, 12, 7],
  },
  {
    name: 'a repository and a CI state list both',
    picked: [repository('example/repo'), ci('failed')],
    want: [7],
  },
  {
    name: 'two CI states list either',
    picked: [ci('failed'), ci('none')],
    want: [3, 7],
  },
  {
    name: 'draft and an author list both',
    picked: [draft('draft'), author('kwan')],
    want: [5],
  },
])('$name', ({ picked, want }) => {
  // Act
  const got = listed(picked)

  // Assert
  expect(got).toEqual(want)
})

// Twin of TestFOpensTheReviewsFilterWithCounts.
test('offers repositories, CI states, draft or ready, then authors, each with its count', () => {
  // Act
  const choices = facetChoices(requests, [])

  // Assert
  expect(choices.map((choice) => `${facetLabel(choice.facet)}  ${String(choice.count)}`)).toEqual([
    'no repository  1',
    'example/other  1',
    'example/repo  2',
    'CI failed  1',
    'CI passed  1',
    'CI running  1',
    'CI none  1',
    'draft  1',
    'ready  3',
    'by kwan  2',
    'by mira  2',
  ])
})

test('a picked value no request holds is still offered, at zero', () => {
  // Act
  const choices = facetChoices(requests, [author('ana')])

  // Assert
  expect(choices.map((choice) => `${facetLabel(choice.facet)}  ${String(choice.count)}`)).toContain(
    'by ana  0',
  )
})

test('toggling a picked value unpicks it', () => {
  // Act
  const picked = toggleFacet([ci('failed'), author('kwan')], ci('failed'))

  // Assert
  expect(picked).toEqual([author('kwan')])
})
