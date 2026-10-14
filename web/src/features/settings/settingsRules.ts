// Trade-off TRADE-51: these rules say again a few of the checks the server
// makes of a save, so a setting is refused beside itself before anything is
// sent. The server stays the authority: a save it refuses names each setting,
// and Settings marks those the same way.

// A rule passes a value, or says why it is refused.
type Rule = (value: unknown) => true | string

// A Go duration, as time.ParseDuration reads one: numbers, each with its unit.
const goDuration = /^(?:(?:\d+(?:\.\d*)?|\.\d+)(?:ns|us|µs|ms|s|m|h))+$/

// durationUnits are how many seconds each unit of a Go duration stands for.
const durationUnits: Record<string, number> = {
  ns: 1e-9,
  us: 1e-6,
  µs: 1e-6,
  ms: 1e-3,
  s: 1,
  m: 60,
  h: 3600,
}

// The hosts a Jira on this machine answers on, which http may reach.
const thisMachine = new Set(['localhost', '127.0.0.1', '[::1]'])

// seconds is how long a Go duration runs.
function seconds(duration: string): number {
  let total = 0
  for (const [, amount, unit] of duration.matchAll(/(\d*\.?\d*)(ns|us|µs|ms|s|m|h)/g)) {
    total += Number(amount) * (durationUnits[unit ?? 's'] ?? 0)
  }

  return total
}

// duration passes a timing left empty, which keeps its default, or a
// positive Go duration.
function duration(value: unknown): true | string {
  const text = String(value)
  if (text === '') {
    return true
  }

  if (!goDuration.test(text)) {
    return 'A duration such as 20s or 3m.'
  }

  return seconds(text) > 0 ? true : 'Must be more than zero.'
}

// baseURL passes no address, or one Jira can be reached at with the token
// kept off the network and out of the address.
function baseURL(value: unknown): true | string {
  const text = String(value)
  if (text === '') {
    return true
  }

  const parsed = URL.canParse(text) ? new URL(text) : null
  if (parsed === null || parsed.host === '' || !['https:', 'http:'].includes(parsed.protocol)) {
    return 'An address such as https://jira.example.com.'
  }

  if (parsed.username !== '' || parsed.password !== '') {
    return 'An address carries no login: the user and the token have fields of their own.'
  }

  return parsed.protocol === 'https:' || thisMachine.has(parsed.hostname)
    ? true
    : 'Over http the token would cross the network in the clear: use https.'
}

// branchTemplate passes the default, left empty, or a template that keeps
// the issue key, so a branch named from it can be read back.
function branchTemplate(value: unknown): true | string {
  const text = String(value)

  return text === '' || text.includes('{key}') ? true : 'Must contain {key}.'
}

// trailer passes a trailer label of one word, without the colon a trailer
// line adds after it.
function trailer(value: unknown): true | string {
  return /[:\s]/.test(String(value).trim()) ? 'A single word, without a colon.' : true
}

// count passes a count of zero, which keeps the default, or more.
function count(value: unknown): true | string {
  return Number(value) < 0 ? 'Cannot be below zero.' : true
}

// rules are the settings checked before a save, by their path in the form.
const rules: Record<string, Rule> = {
  'jira.base_url': baseURL,
  'branch.template': branchTemplate,
  'branch.slug_limit': count,
  'commit.refs_trailer': trailer,
  'commit.subject_limit': count,
  'timing.request_timeout': duration,
  'timing.ci_interval': duration,
  'ui.comments_shown': count,
}

// ruleFor is the rule a setting is checked by before a save, or none.
export function ruleFor(name: string): Rule | undefined {
  return rules[name]
}

// A refusal as the form keeps one: what kind, and why.
interface Refusal {
  type?: unknown
  message?: unknown
}

// refusedPaths are the paths of every setting the form holds refused, in the
// order it refused them: a field, a list's row, or a list as a whole, whose
// path ends in root.
export function refusedPaths(errors: object, under = ''): string[] {
  return Object.entries(errors).flatMap(([key, held]: [string, unknown]) => {
    const path = under === '' ? key : `${under}.${key}`
    if (typeof held !== 'object' || held === null) {
      return []
    }

    const refusal = held as Refusal
    if (typeof refusal.message === 'string' && typeof refusal.type === 'string') {
      return [path]
    }

    return refusedPaths(held, path)
  })
}
