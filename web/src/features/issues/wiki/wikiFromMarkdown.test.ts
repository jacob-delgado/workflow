import { z } from 'zod'
import { twinCases } from '@/test/twinCases.ts'
import { wikiFromMarkdown } from './wikiFromMarkdown.ts'

// The cases jira.WikiFromMarkdown answers to as well, so Preview shows the
// wiki markup Jira will be sent.
const { cases } = twinCases(
  'internal/jira/testdata/wiki_from_markdown.json',
  z.strictObject({
    about: z.string(),
    cases: z.array(z.strictObject({ name: z.string(), markdown: z.string(), want: z.string() })),
  }),
)

test.each(cases)('$name', ({ markdown, want }) => {
  // Act
  const got = wikiFromMarkdown(markdown)

  // Assert
  expect(got).toBe(want)
})
