import { AxeBuilder } from '@axe-core/playwright'
import type { Locator, Page } from '@playwright/test'
import { expect } from './fixtures.ts'
import { walkTabOrder } from './tabwalk.ts'

// What every surface is held to, at each window and theme a spec opens it in
// (CLAUDE.md, "Accessibility is enforced"): nothing scrolls sideways, the page
// does not scroll, Tab reaches every drawn control and each is in view as it
// has focus, and axe finds nothing.

// Reach is what a surface's Tab order holds beyond every drawn control.
interface Reach {
  // reaches are controls Tab must reach, named as a walk names them: what the
  // step on screen adds.
  reaches?: string[]
  // passedBy are the drawn controls Tab passes by, in order: a radio group's
  // other choices, which the arrow keys reach from the one chosen.
  passedBy?: string[]
  // within is the part of the page Tab must stay inside: an open dialog.
  within?: Locator
}

// expectReachableAndClean holds the page as it stands to the layout floor:
// Tab once round it, or round the part within, then every check, each named
// in its failure.
export async function expectReachableAndClean(
  page: Page,
  { reaches = [], passedBy = [], within }: Reach = {},
): Promise<void> {
  const { reached, missed, hidden, left } = await walkTabOrder(page, { within })
  expect(await page.evaluate(sidewaysScrollers), 'scrolls sideways').toEqual([])
  expect(await page.evaluate(pageScrolls), 'the page scrolls').toBe(false)
  expect(reached, 'reached by Tab').toEqual(expect.arrayContaining(reaches))
  expect(missed, 'never reached by Tab').toEqual(passedBy)
  expect(hidden, 'out of view with focus').toEqual([])
  expect(left, 'left by Tab').toEqual([])
  expect(await axeViolations(page), 'axe').toBe('')
}

// sidewaysScrollers names what scrolls sideways: the page, or any part of it.
export function sidewaysScrollers(): string[] {
  const scrollers: string[] = []
  const page = document.scrollingElement
  if (page !== null && page.scrollWidth > window.innerWidth) {
    scrollers.push(`the page (${String(page.scrollWidth)} px in ${String(window.innerWidth)})`)
  }

  for (const element of document.querySelectorAll<HTMLElement>('body *')) {
    const { overflowX } = getComputedStyle(element)
    const scrolls = overflowX === 'auto' || overflowX === 'scroll'
    if (scrolls && element.scrollWidth > element.clientWidth) {
      const label = element.getAttribute('aria-label') ?? element.id
      scrollers.push(
        `<${element.localName}> ${label} (${String(element.scrollWidth)} px in ${String(element.clientWidth)})`,
      )
    }
  }

  return scrollers
}

// pageScrolls says whether the page itself scrolls down, which the shell never
// does: the content, or a pane in it, scrolls instead.
export function pageScrolls(): boolean {
  return (document.scrollingElement?.scrollHeight ?? 0) > window.innerHeight
}

// axeViolations names the WCAG A/AA violations axe finds on screen, or is
// empty.
export async function axeViolations(page: Page): Promise<string> {
  const { violations } = await new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
    .analyze()

  return violations.map((v) => `${v.id} (${String(v.nodes.length)})`).join(', ')
}
