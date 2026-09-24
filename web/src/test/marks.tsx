import { render } from '@testing-library/react'
import { StateMark, type MarkState } from '@/shell/StateMark.tsx'

// The attributes that make a mark's shape — which parts it draws, where, and
// whether each is filled or outlined — and not its color: a state must be
// told apart by shape alone.
const geometry = ['cx', 'cy', 'r', 'd', 'fill', 'stroke', 'stroke-width']

// markShape is the shape of the first state mark drawn inside element — a mark
// is hidden from assistive tech, its words beside it — or '' when there is none.
export function markShape(element: Element): string {
  const mark = element.querySelector('svg[aria-hidden="true"]')
  if (mark === null) {
    return ''
  }

  return [...mark.children]
    .map((part) => [part.tagName, ...geometry.map((name) => part.getAttribute(name))].join(' '))
    .join(' | ')
}

// drawnMark is the shape StateMark draws for a state, for a component's mark
// to be compared against.
export function drawnMark(state: MarkState): string {
  const { container, unmount } = render(<StateMark state={state} />)
  const shape = markShape(container)
  unmount()

  return shape
}
