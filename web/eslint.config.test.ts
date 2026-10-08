import { ESLint } from 'eslint'
import { expect, test } from 'vitest'

// Each rule the lint relies on, shown firing on code that breaks it, so a rule
// that stops firing — a plugin outside its supported ESLint, a selector that no
// longer matches — fails here rather than passing everything quietly. The code
// is linted as if it were the file it names, which must exist: the typed rules
// read it through the TypeScript program.
const eslint = new ESLint({ cwd: import.meta.dirname })

// rulesBroken names each rule code breaks, linted as if it were the file at path.
async function rulesBroken(code: string, path: string): Promise<string[]> {
  const results = await eslint.lintText(code, { filePath: path })

  return results.flatMap((result) => result.messages.map((message) => message.ruleId ?? 'parse'))
}

test.each([
  {
    rule: 'jsx-a11y/alt-text',
    code: 'export function Picture() {\n  return <img src="/logo.png" />\n}\n',
  },
  {
    rule: 'jsx-a11y/no-autofocus',
    code: 'export function Search() {\n  return <input aria-label="Search" autoFocus />\n}\n',
  },
  {
    rule: '@eslint-community/eslint-comments/require-description',
    code: '// eslint-disable-next-line no-console\nconsole.log("hi")\n',
  },
  {
    rule: '@eslint-community/eslint-comments/no-unlimited-disable',
    code: '/* eslint-disable -- every rule, for no rule in particular */\nconsole.log("hi")\n',
  },
])('$rule fires on the app code that breaks it', async ({ rule, code }) => {
  // Act
  const broken = await rulesBroken(code, 'src/App.tsx')

  // Assert
  expect(broken).toContain(rule)
})
