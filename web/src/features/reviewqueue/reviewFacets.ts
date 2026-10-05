import type { CiState, ReviewRequest } from '@/api/generated/types.gen.ts'
import type { FilterChoice } from '@/lib/FilterChips.tsx'

// A facet is one value a request can hold in one of four: its repository, how
// its CI stands, whether it is a draft, or who asks. Values picked in one
// facet widen the queue, and the facets narrow it together.
//
// Trade-off TRADE-23: these rules are written again in
// internal/tui/reviewfacets.go, and twin-named tests pin the two.
type FacetKind = 'repository' | 'ci' | 'draft' | 'author'

export interface Facet {
  kind: FacetKind
  value: string
}

// FacetChoice is a facet value on offer, with how many requests hold it.
export type FacetChoice = FilterChoice<Facet>

// The CI states and draft or ready, in the order they are offered.
const ciOrder: CiState[] = ['failed', 'passed', 'running', 'none']
const draftOrder = ['draft', 'ready']

function sameFacet(a: Facet, b: Facet): boolean {
  return a.kind === b.kind && a.value === b.value
}

// facetsOf is the value a request holds in each facet.
function facetsOf(request: ReviewRequest): Facet[] {
  return [
    { kind: 'repository', value: request.repository },
    { kind: 'ci', value: request.ci },
    { kind: 'draft', value: request.draft ? 'draft' : 'ready' },
    { kind: 'author', value: request.author },
  ]
}

// facetLabel is how the filter names a facet value.
export function facetLabel(facet: Facet): string {
  switch (facet.kind) {
    case 'repository':
      return facet.value === '' ? 'no repository' : facet.value
    case 'ci':
      return `CI ${facet.value}`
    case 'author':
      return `by ${facet.value}`
    case 'draft':
      return facet.value
  }
}

// admits reports whether a request holds a picked value in every facet
// something is picked in.
export function admits(picked: Facet[], request: ReviewRequest): boolean {
  return facetsOf(request).every(
    (held) =>
      !picked.some((chosen) => chosen.kind === held.kind) ||
      picked.some((chosen) => sameFacet(chosen, held)),
  )
}

// facetChoices is every value the requests hold, with how many hold each —
// repositories by name, the CI states and draft or ready in a fixed order,
// then authors by name — and every picked value none holds, at zero, so it
// can still be unpicked.
export function facetChoices(requests: ReviewRequest[], picked: Facet[]): FacetChoice[] {
  const held = requests.flatMap(facetsOf)
  const countOf = (facet: Facet) => held.filter((one) => sameFacet(one, facet)).length
  const named = (kind: FacetKind): Facet[] =>
    [...new Set([...held, ...picked].filter((one) => one.kind === kind).map((one) => one.value))]
      .sort()
      .map((value) => ({ kind, value }))
  const offered: Facet[] = [
    ...named('repository'),
    ...ciOrder.map((value): Facet => ({ kind: 'ci', value })),
    ...draftOrder.map((value): Facet => ({ kind: 'draft', value })),
    ...named('author'),
  ]

  return offered
    .map((facet) => ({ value: facet, count: countOf(facet) }))
    .filter((choice) => choice.count > 0 || isPicked(picked, choice.value))
}

// isPicked reports whether facet is among the picked.
export function isPicked(picked: Facet[], facet: Facet): boolean {
  return picked.some((chosen) => sameFacet(chosen, facet))
}

// toggleFacet picks a value, or unpicks it when it is already picked.
export function toggleFacet(picked: Facet[], facet: Facet): Facet[] {
  return isPicked(picked, facet)
    ? picked.filter((chosen) => !sameFacet(chosen, facet))
    : [...picked, facet]
}
