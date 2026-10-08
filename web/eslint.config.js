import comments from '@eslint-community/eslint-plugin-eslint-comments'
import js from '@eslint/js'
import jsonc from 'eslint-plugin-jsonc'
import jsxA11y from 'eslint-plugin-jsx-a11y'
import react from 'eslint-plugin-react'
import reactHooks from 'eslint-plugin-react-hooks'
import testingLibrary from 'eslint-plugin-testing-library'
import vitest from '@vitest/eslint-plugin'
import globals from 'globals'
import tseslint from 'typescript-eslint'

// React escapes all interpolated content; never bypass it (all source + tests).
const noReactEscapeBypass = [
  {
    selector: "JSXAttribute[name.name='dangerouslySetInnerHTML']",
    message: 'Never bypass React escaping; render user content as text.',
  },
  {
    selector: "MemberExpression[property.name='innerHTML']",
    message: 'Never write innerHTML; render through React.',
  },
]

// De-emphasize with color, never opacity (all source + tests): an opacity-N
// class dims the text with its background, below the contrast floor, and axe
// skips a disabled control's contrast, so nothing else would catch it.
const opacityDimming =
  'De-emphasize with color (the --disabled tokens in index.css), not opacity, which dims text below the contrast floor (CLAUDE.md).'
const noOpacityDimming = [
  { selector: 'Literal[value=/opacity-[0-9]/]', message: opacityDimming },
  { selector: 'TemplateElement[value.raw=/opacity-[0-9]/]', message: opacityDimming },
]

// Headings in sentence case, never capitals (all source + tests), in any
// variant (md:, hover:) or important (!) form: an all-caps eyebrow is a
// template's heading, not this product's, whose type scale does the work.
const allCaps =
  'Set a heading in sentence case on the type scale (web/src/index.css), not in capitals: an uppercase eyebrow is template chrome.'
const noAllCaps = [
  { selector: 'Literal[value=/(^|[\\s:!])uppercase!?(\\s|$)/]', message: allCaps },
  { selector: 'TemplateElement[value.raw=/(^|[\\s:!])uppercase!?(\\s|$)/]', message: allCaps },
]

// A control off while its own request or run goes is held (Button's and the
// fields' held prop, in web/src/lib), never disabled: the disabled attribute
// drops its focus to the page in Chromium and WebKit, and a keyboard user who
// pressed it hears the refusal from nowhere (UX-106). disabled stays for a
// control off for a reason that names no run — nothing chosen yet. Nor does a
// Button hold itself by hand with aria-disabled and a guard in its onClick:
// held is both, in one place.
const runningNames =
  /^(busy|running|sending|opening|writing|linking|posting|going|retrying|reading|saving|isFetching|isFetchingNextPage|isPending)$/
const aRun = `:matches(Identifier[name=${String(runningNames)}], Literal[value='running'])`
const heldNotDisabled = [
  {
    selector: `JSXAttribute[name.name='disabled'] ${aRun}`,
    message:
      'Hold a control while its run goes with held (web/src/lib/Button.tsx, Field.tsx), not disabled, which drops its focus to the page.',
  },
  {
    selector: `JSXOpeningElement[name.name='Button'] > JSXAttribute[name.name='aria-disabled'] ${aRun}`,
    message:
      'Hold a Button while its run goes with held (web/src/lib/Button.tsx), which sets aria-disabled and swallows the press, not with aria-disabled and a guard of its own.',
  },
]

// Black-box test smells: assert on user-facing semantics, not implementation
// details. `no-node-access` would catch these but over-fires on the legitimate
// focus tests (`document.activeElement`) and on the state-mark test that climbs
// from a state's words to the aria-hidden mark drawn beside them, where the DOM
// *is* the behavior under test — so ban the specific patterns. The `.style` /
// `.classList` reads are scoped to `expect()` so a fixture may still SET them;
// only assertions are policed. The escape is an annotated
// `eslint-disable-next-line no-restricted-syntax -- <reason>`, kept honest by
// reportUnusedDisableDirectives below.
const noTestImplDetails = [
  {
    selector: "MemberExpression[property.name='className']",
    message: 'Assert on semantics (role, accessible name, aria-current), not CSS classes.',
  },
  {
    selector: "CallExpression[callee.name='expect'] MemberExpression[property.name='classList']",
    message: 'Class inspection inside expect() — assert semantics, not classes.',
  },
  {
    selector:
      'CallExpression[callee.property.name=/^(get|has|toHave)Attribute$/][arguments.0.value=/^data-/]',
    message: 'Assert on user-facing behavior, not internal data-* hooks.',
  },
  {
    selector: "CallExpression[callee.name='expect'] MemberExpression[object.property.name='style']",
    message: 'Reading .style.* inside expect() asserts presentation — assert the semantic state.',
  },
]

export default tseslint.config(
  { ignores: ['dist', 'coverage', 'playwright-report', 'test-results', 'src/api/generated'] },
  // A disable directive that no longer suppresses anything is a stale claim
  // about the code — fail on it rather than warn, so the annotated escapes from
  // the ban lists below stay honest.
  { linterOptions: { reportUnusedDisableDirectives: 'error' } },
  // Every disable says why on the directive itself, after `--`, and names the
  // rules it turns off: a reason in the comment above drifts from the line it
  // excuses, and a bare `eslint-disable` turns off rules nobody chose.
  {
    files: ['**/*.{js,mjs,cjs,ts,tsx}'],
    plugins: { '@eslint-community/eslint-comments': comments },
    rules: {
      '@eslint-community/eslint-comments/require-description': 'error',
      '@eslint-community/eslint-comments/no-unlimited-disable': 'error',
    },
  },
  // Plain JS (this config): recommended rules only — the type-checked set below
  // needs tsconfig coverage this file lacks.
  {
    files: ['**/*.{js,mjs,cjs}'],
    extends: [js.configs.recommended],
    languageOptions: { globals: globals.node },
  },
  // public/ is served as it is, to the browser: the pre-paint theme script.
  {
    files: ['public/**/*.js'],
    languageOptions: { globals: globals.browser, sourceType: 'script' },
  },
  {
    files: ['**/*.{ts,tsx}'],
    extends: [
      js.configs.recommended,
      ...tseslint.configs.strictTypeChecked,
      reactHooks.configs.flat['recommended-latest'],
      // Static accessibility lint (part of `yarn lint`); the runtime axe scan in
      // e2e is the belt to this suspenders.
      jsxA11y.flatConfigs.strict,
    ],
    plugins: { react },
    languageOptions: {
      globals: globals.browser,
      parserOptions: {
        projectService: true,
        tsconfigRootDir: import.meta.dirname,
      },
    },
    settings: { react: { version: 'detect' } },
    rules: {
      '@typescript-eslint/no-explicit-any': 'error',
      // A switch over a union names every member, as Go's exhaustive asks: a
      // new state then fails the lint where it is not drawn, rather than
      // drawing nothing.
      '@typescript-eslint/switch-exhaustiveness-check': 'error',
      // A list item carries a key, and one that names the item, not its place:
      // a key by index hands one row's state to the next when a row comes or
      // goes.
      'react/jsx-key': 'error',
      'react/no-array-index-key': 'error',
      complexity: ['error', 10],
      // Code-smell caps (see CLAUDE.md "Code smells"): long parameter list, deep
      // nesting, callback pyramids, nested ternaries, and parameter mutation.
      'max-params': ['error', 4],
      'max-depth': ['error', 4],
      'max-nested-callbacks': ['error', 3],
      'no-nested-ternary': 'error',
      'no-param-reassign': 'error',
      // A function that does one thing fits on a screen. JSX spends lines, so
      // the cap is looser than CLAUDE.md's ~25 for Go; a component that
      // outgrows it splits out its parts, as the settings, commit and pull
      // request forms did.
      'max-lines-per-function': ['error', { max: 80, skipBlankLines: true, skipComments: true }],
      // React escapes all interpolated content; never bypass it. Nor dim with
      // opacity, nor head a part in capitals.
      'no-restricted-syntax': [
        'error',
        ...noReactEscapeBypass,
        ...noOpacityDimming,
        ...noAllCaps,
        ...heldNotDisabled,
      ],
      // A label wrapping one of web/src/lib's fields labels the control it
      // draws, as one wrapping the native element does.
      'jsx-a11y/label-has-associated-control': [
        'error',
        { controlComponents: ['Input', 'Select', 'TextArea'], depth: 3 },
      ],
    },
  },
  // Black-box test discipline (the TS analogue of Go's external `_test`
  // package): drive components through role/text/user-event and stores through
  // their public API. testing-library catches container/screen-query smells;
  // vitest catches footguns (a committed `test.only` silently skips the file).
  {
    files: ['**/*.test.{ts,tsx}'],
    extends: [testingLibrary.configs['flat/react']],
    plugins: { vitest },
    rules: {
      ...vitest.configs.recommended.rules,
      'testing-library/no-node-access': 'off',
      'no-restricted-syntax': [
        'error',
        ...noReactEscapeBypass,
        ...noOpacityDimming,
        ...noAllCaps,
        ...noTestImplDetails,
      ],
    },
  },
  // The same black-box assertion rules for the Playwright specs and their
  // helpers: a spec asserts what a user can perceive, not a data-* hook, class
  // or style. testing-library and vitest stay off here — they police jsdom and
  // vitest, which e2e does not use.
  {
    files: ['e2e/**/*.ts'],
    rules: {
      'no-restricted-syntax': [
        'error',
        ...noReactEscapeBypass,
        ...noOpacityDimming,
        ...noAllCaps,
        ...noTestImplDetails,
      ],
    },
  },
  // JSON (package.json, tsconfig*.json): correctness rules (duplicate keys,
  // invalid values). `flat/prettier` drops the stylistic rules so prettier still
  // owns formatting.
  ...jsonc.configs['flat/recommended-with-json'],
  ...jsonc.configs['flat/prettier'],
  // TS project-config files use JSONC (block comments) despite the .json ext.
  {
    files: ['**/tsconfig*.json'],
    rules: { 'jsonc/no-comments': 'off' },
  },
)
