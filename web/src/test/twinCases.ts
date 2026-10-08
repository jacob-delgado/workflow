/// <reference types="node" />
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import type { z } from 'zod'

// twinCases is a case file a rule here and its Go twin both answer to, named
// from the repository's root as the Go test names it, and read through schema.
// A strict schema refuses a field this side does not read, so a case cannot
// pin the Go copy and pass this one unread. The tests run from web/, one below
// the root.
export function twinCases<T>(file: string, schema: z.ZodType<T>): T {
  return schema.parse(JSON.parse(readFileSync(resolve(process.cwd(), '..', file), 'utf8')))
}
