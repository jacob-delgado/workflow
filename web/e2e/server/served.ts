import { readFileSync } from 'node:fs'
import { test, type Page } from '@playwright/test'

// refusalHeard names the function the page tells a refusal to.
const refusalHeard = 'workflowE2ERefusalHeard'

// heardRefusals hears what the browser reports refusing under the server's
// content policy, from the moment it is called, and returns the list it
// fills: awaited before the page opens, it hears a refusal while it loads.
// Every engine dispatches a securitypolicyviolation event for a refusal, and
// says it on the console in words of its own, Firefox's "Content-Security-
// Policy" among them, so both are heard.
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
  await page.addInitScript((heard) => {
    document.addEventListener('securitypolicyviolation', (event) => {
      const tell = (window as unknown as Record<string, (refusal: string) => Promise<void>>)[heard]
      void tell(`${event.effectiveDirective} refused ${event.blockedURI}`)
    })
  }, refusalHeard)

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
