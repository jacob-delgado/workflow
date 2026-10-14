import type { FieldErrors } from 'react-hook-form'

// A rule is what a setting must be, checked as it is typed once a save has
// been tried: true where it holds, or why it does not, in the server's words.
type Rule = (value: unknown) => true | string

// A duration as Go reads one: a run of numbers, each with its unit.
const duration = /^(\d+(\.\d+)?(ns|us|µs|ms|s|m|h))+$/

// text is a setting's value as the text it was typed as.
function text(value: unknown): string {
  return typeof value === 'string' ? value.trim() : ''
}

// notNegative refuses a count below zero.
const notNegative: Rule = (value) =>
  typeof value !== 'number' || value >= 0 ? true : 'Cannot be negative.'

// aDuration refuses what is not empty and not a duration.
const aDuration: Rule = (value) =>
  text(value) === '' || duration.test(text(value)) ? true : 'Not a duration such as 20s.'

// Trade-off TRADE-51: these rules repeat a few of the server's own checks, so
// a setting is marked before a save goes out; the server's refusal of a save,
// which names each setting it refused, stays the authority.
const rules: Partial<Record<string, Rule>> = {
  'jira.base_url': (value) =>
    text(value) === '' || /^https?:\/\/[^/\s]+/.test(text(value))
      ? true
      : 'Not an http or https address.',
  'branch.template': (value) =>
    text(value) === '' || text(value).includes('{key}') ? true : 'Must contain {key}.',
  'branch.slug_limit': notNegative,
  'commit.subject_limit': notNegative,
  'ui.comments_shown': notNegative,
  'timing.request_timeout': aDuration,
  'timing.ci_interval': aDuration,
}

// ruleFor is the rule a setting is held to by its dotted key, if it has one.
export function ruleFor(key: string): Rule | undefined {
  return rules[key]
}

// refusedKeys is the dotted key of each setting the form marks refused, in
// the order the form holds them.
export function refusedKeys(errors: FieldErrors, prefix = ''): string[] {
  return Object.entries(errors).flatMap(([key, inner]) => {
    const dotted = prefix === '' ? key : `${prefix}.${key}`
    if (typeof inner !== 'object' || inner === null) {
      return []
    }

    return 'message' in inner && typeof inner.message === 'string'
      ? [dotted]
      : refusedKeys(inner as FieldErrors, dotted)
  })
}
