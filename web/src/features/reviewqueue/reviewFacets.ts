import type { ReviewFacet, ReviewRequest } from '@/api/generated/types.gen.ts'
import type { FilterChoice } from '@/lib/FilterChips.tsx'

// A facet is one value a request holds in one of the four the queue is
// narrowed by, as the server describes it: each request carries its own, each
// labeled, and the queue the order the filter offers them in. Values picked in
// one facet widen the queue, and the facets narrow it together; the page only
// counts, picks and admits.
export type Facet = ReviewFacet

// FacetChoice is a facet value on offer, with how many requests hold it.
export type FacetChoice = FilterChoice<Facet>

function sameFacet(a: Facet, b: Facet): boolean {
  return a.kind === b.kind && a.value === b.value
}

// admits reports whether a request holds a picked value in every facet
// something is picked in.
export function admits(picked: Facet[], request: ReviewRequest): boolean {
  return request.facets.every(
    (held) =>
      !picked.some((chosen) => chosen.kind === held.kind) ||
      picked.some((chosen) => sameFacet(chosen, held)),
  )
}

// facetChoices is each value the server offers, in its order, that a request
// holds or that is picked, with how many requests hold it, then each picked
// value it no longer offers, at zero, in the order it was picked, so it can
// still be unpicked.
export function facetChoices(
  requests: ReviewRequest[],
  order: Facet[],
  picked: Facet[],
): FacetChoice[] {
  const held = requests.flatMap((request) => request.facets)
  const countOf = (facet: Facet) => held.filter((one) => sameFacet(one, facet)).length
  const offered = order
    .map((facet) => ({ value: facet, count: countOf(facet) }))
    .filter((choice) => choice.count > 0 || isPicked(picked, choice.value))
  const unoffered = picked
    .filter((facet) => !offered.some((choice) => sameFacet(choice.value, facet)))
    .map((facet) => ({ value: facet, count: countOf(facet) }))

  return [...offered, ...unoffered]
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
