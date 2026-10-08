import { readFileSync } from 'node:fs'
import { test, type Page } from '@playwright/test'

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
