import { readFileSync } from 'node:fs'
import { expect, test, type Page } from '@playwright/test'

// refusalHeard names the function the page tells a refusal to.
const refusalHeard = 'workflowE2ERefusalHeard'

// listenForRefusals runs in the page, before the page's own scripts, and
// tells each refusal the page dispatches to the function heard names, in a
// line that says what was refused and where it came from: the position the
// engine gives, in the file it names, a sample of what was refused when the
// policy asks for one with 'report-sample', and the node it was written on,
// named by what finds that node on the page and by the shadow root it sits
// in, if any. Playwright hands the page this function's source alone, so
// every helper it calls is declared within it: one moved out of it is
// undefined in the page, and the event goes untold.
function listenForRefusals(heard: string): void {
  // A form's controls shadow the form's own properties by their names, one
  // named id or getAttribute among them, so a node is read through the
  // prototypes that define what is read and never through the node itself.
  // Those are this window's, while a node adopted from another window's
  // document keeps that window's wrapper, which no instanceof against this
  // window's interfaces recognizes, so a node is told by its nodeType. A
  // getter throws on what its prototype does not describe, which tells a
  // target that is no node, and a node that is no shadow root.
  const typeOf = (target: EventTarget): number | undefined => {
    try {
      return Reflect.get(Node.prototype, 'nodeType', target)
    } catch {
      return undefined
    }
  }
  const hostOf = (node: Node): Element | null => {
    try {
      return Reflect.get(ShadowRoot.prototype, 'host', node)
    } catch {
      return null
    }
  }
  const nameOf = (node: Node): string => Reflect.get(Node.prototype, 'nodeName', node)
  const attributeOf = (element: Element, attribute: string): string | null =>
    Element.prototype.getAttribute.call(element, attribute)
  const marked = (element: Element, attributes: readonly string[]): string => {
    const marks = attributes.map((attribute) => {
      const value = attributeOf(element, attribute)

      return value === null ? '' : `[${attribute}=${JSON.stringify(value)}]`
    })

    return nameOf(element) + marks.join('')
  }
  // A shadow root is said as one, so a node at the top of a shadow tree
  // reads apart from a child of the root's host.
  const rootOf = (host: Element): string => `the shadow root of ${marked(host, ['id', 'role'])}`
  const placeOf = (node: Node): { said: string; element: Element } | null => {
    const parent = Reflect.get(Node.prototype, 'parentNode', node)
    if (parent === null) {
      return null
    }
    if (typeOf(parent) === Node.ELEMENT_NODE) {
      return { said: marked(parent as Element, ['id', 'role']), element: parent as Element }
    }
    const host = hostOf(parent)

    return host === null ? null : { said: rootOf(host), element: host }
  }
  const ancestry = (node: Node): string => {
    const nearest = placeOf(node)
    const next = nearest === null ? null : placeOf(nearest.element)

    return [nearest, next]
      .filter((place) => place !== null)
      .map((place) => ` in ${place.said}`)
      .join('')
  }
  const located = (target: EventTarget | undefined): string => {
    const type = target === undefined ? undefined : typeOf(target)
    if (type === undefined) {
      return 'no node'
    }
    const node = target as Node
    if (type === Node.DOCUMENT_NODE) {
      return 'the document'
    }
    if (type === Node.ELEMENT_NODE) {
      return marked(node as Element, ['id', 'role', 'aria-label', 'name']) + ancestry(node)
    }
    const host = hostOf(node)

    return host === null ? nameOf(node) : rootOf(host) + ancestry(host)
  }
  const said = (event: SecurityPolicyViolationEvent): string => {
    const refused = `${event.effectiveDirective} refused`
    const parts = [event.blockedURI === '' ? refused : `${refused} ${event.blockedURI}`]
    const position = `${String(event.lineNumber)}:${String(event.columnNumber)}`
    if (event.sourceFile !== '') {
      parts.push(`from ${event.sourceFile}:${position}`)
    } else if (event.lineNumber !== 0) {
      // An engine can give a position and name no file: WebKit for what the
      // HTML parser wrote, Chromium for a script with no address of its own.
      parts.push(`from line ${position} of a file the engine does not name`)
    }
    if (event.sample !== '') {
      parts.push(`sample ${JSON.stringify(event.sample)}`)
    }
    // A listener on the document is told a shadow root's host as the target
    // of a refusal inside it, while the event's path begins at the node
    // itself when the root is open, and at the host when it is closed.
    parts.push(`on ${located(event.composedPath().at(0))}`)

    return parts.join(', ')
  }

  document.addEventListener('securitypolicyviolation', (event) => {
    const tell = (window as unknown as Record<string, (refusal: string) => Promise<void>>)[heard]
    void tell(said(event))
  })
}

// heardRefusals hears what the browser reports refusing under the server's
// content policy, from the moment it is called, and returns the list it
// fills: awaited before the page opens, it hears a refusal while it loads.
// Every engine dispatches a securitypolicyviolation event for a refusal, and
// says it on the console in words of its own, Firefox's "Content-Security-
// Policy" among them, so both are heard: the console line as the engine
// words it, and the event as listenForRefusals tells it.
export async function heardRefusals(page: Page): Promise<string[]> {
  const refusals: string[] = []
  page.on('console', (message) => {
    const text = message.text()
    if (/content[- ]security[- ]policy/i.test(text)) {
      refusals.push(text)
    }
  })
  await page.exposeFunction(refusalHeard, (refusal: string) => {
    refusals.push(refusal)
  })
  await page.addInitScript(listenForRefusals, refusalHeard)

  return refusals
}

// printedAddress is the address the server printed as it started, session
// and all, read from the log the config names, as a person reads it from the
// terminal.
export function printedAddress(): string {
  const log: unknown = test.info().project.metadata.serverLog
  if (typeof log !== 'string') {
    throw new Error('the project names no server log')
  }

  const printed = /serving (\S+)/.exec(readFileSync(log, 'utf8'))?.[1]
  if (printed === undefined) {
    throw new Error(`the server has not said where it serves in ${log}`)
  }

  return printed
}

// openServed opens the page at the address the server printed, as a person
// opening it from the terminal does.
export async function openServed(page: Page): Promise<void> {
  await page.goto(printedAddress())
}

// openLive opens the served page and waits until a person could use it: the
// session taken out of the address and the stream live. A click before then
// can land on a page that is still taking its session in, and be lost.
export async function openLive(page: Page): Promise<void> {
  await openServed(page)
  await expect(page).toHaveURL(new URL('/', printedAddress()).href)
  await expect(page.getByRole('status').filter({ hasText: /^Live$/ })).toBeVisible()
}
