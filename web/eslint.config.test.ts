import { readFile } from 'node:fs/promises'
import { join } from 'node:path'
import { ESLint } from 'eslint'
import { expect, test } from 'vitest'

// Each rule the lint relies on, shown firing on code that breaks it, so a rule
// that stops firing — a plugin outside its supported ESLint, a selector that no
// longer matches — fails here rather than passing everything quietly. The code
// is linted as if it were the file it names, which must exist: the typed rules
// read it through the TypeScript program.
const eslint = new ESLint({ cwd: import.meta.dirname })

// Broken is one complaint: the rule that made it, and what it said.
interface Broken {
  rule: string
  says: string
}

// brokenBy is each complaint code draws, linted as if it were the file at path.
async function brokenBy(code: string, path: string): Promise<Broken[]> {
  const results = await eslint.lintText(code, { filePath: path })

  return results.flatMap((result) =>
    result.messages.map((message) => ({ rule: message.ruleId ?? 'parse', says: message.message })),
  )
}

// A rule restricted-syntax holds is told apart by the words of its message.
const restricted = 'no-restricted-syntax'

test.each([
  {
    rule: 'jsx-a11y/alt-text',
    says: '',
    code: 'export function Picture() {\n  return <img src="/logo.png" />\n}\n',
  },
  {
    rule: 'jsx-a11y/no-autofocus',
    says: '',
    code: 'export function Search() {\n  return <input aria-label="Search" autoFocus />\n}\n',
  },
  {
    rule: '@eslint-community/eslint-comments/require-description',
    says: '',
    code: '// eslint-disable-next-line no-console\nconsole.log("hi")\n',
  },
  {
    rule: '@eslint-community/eslint-comments/no-unlimited-disable',
    says: '',
    code: '/* eslint-disable -- every rule, for no rule in particular */\nconsole.log("hi")\n',
  },
  {
    rule: 'react/jsx-key',
    says: '',
    code: 'export function Names({ names }: { names: string[] }) {\n  return <ul>{names.map((name) => <li>{name}</li>)}</ul>\n}\n',
  },
  {
    rule: 'react/no-array-index-key',
    says: '',
    code: 'export function Names({ names }: { names: string[] }) {\n  return <ul>{names.map((name, at) => <li key={at}>{name}</li>)}</ul>\n}\n',
  },
  {
    rule: restricted,
    says: 'with held',
    code: 'export function Send({ busy }: { busy: boolean }) {\n  return <Button variant="primary" aria-disabled={busy}>Send</Button>\n}\n',
  },
  {
    rule: restricted,
    says: 'with held',
    code: 'export function Send({ send }: { send: { state: string } }) {\n  return <Button variant="primary" aria-disabled={send.state === \'running\'}>Send</Button>\n}\n',
  },
])('$rule fires on the app code that breaks it: "$says"', async ({ rule, says, code }) => {
  // Act
  const broken = await brokenBy(code, 'src/App.tsx')

  // Assert
  expect(broken).toContainEqual({ rule, says: expect.stringContaining(says) as string })
})

test('a state left out of Shape fails the lint', async () => {
  // Arrange
  const path = 'src/shell/StateMark.tsx'
  const drawn = await readFile(join(import.meta.dirname, path), 'utf8')
  const unknownLeftOut = drawn.replace(
    '    case \'unknown\':\n      return <circle cx="8" cy="8" r="2" fill="currentColor" />\n',
    '',
  )

  // Act
  const broken = await brokenBy(unknownLeftOut, path)

  // Assert
  expect(unknownLeftOut).not.toBe(drawn)
  expect(broken).toContainEqual({
    rule: '@typescript-eslint/switch-exhaustiveness-check',
    says: expect.any(String) as string,
  })
})
