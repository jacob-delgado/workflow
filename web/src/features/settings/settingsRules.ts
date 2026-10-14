// Trade-off TRADE-51: these rules say again a few of the checks the server
// makes of a save, so a setting is refused beside itself before anything is
// sent. The server stays the authority: a save it refuses names each setting,
// and Settings marks those the same way.

// A rule passes a value, or says why it is refused.
type Rule = (value: unknown) => true | string

// A Go duration, as time.ParseDuration reads one: a sign, then numbers, each
// with its unit — the microsecond's written with the micro sign or the Greek
// mu, which look alike.
const goDuration = /^([-+]?)((?:(?:\d+(?:\.\d*)?|\.\d+)(?:ns|us|\u00b5s|\u03bcs|ms|s|m|h))+)$/

// A number of a Go duration, and its unit.
const durationPart = /(\d*\.?\d*)(ns|us|\u00b5s|\u03bcs|ms|s|m|h)/g

// durationUnits are how many seconds each unit of a Go duration stands for.
const durationUnits: Record<string, number> = {
  ns: 1e-9,
  us: 1e-6,
  '\u00b5s': 1e-6,
  '\u03bcs': 1e-6,
  ms: 1e-3,
  s: 1,
  m: 60,
  h: 3600,
}

// An address the server reads as http or https to a host: any other it
// refuses, whatever the browser makes of it.
const schemeAndHost = /^https?:\/\/[^/?#]/i

// A host, as the browser writes it, that only this machine answers to, as the
// server tells one: localhost, or a loopback address — any in 127.0.0.0/8,
// ::1, or one of 127.0.0.0/8 in IPv6's IPv4-mapped form, which the browser
// writes in hex.
const thisMachine =
  /^(?:localhost|127(?:\.\d{1,3}){3}|\[::1\]|\[::ffff:7f[\da-f]{2}:[\da-f]{1,4}\])$/

// seconds is how long the numbers and units of a Go duration run.
function seconds(parts: string): number {
  let total = 0
  for (const [, amount, unit] of parts.matchAll(durationPart)) {
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

  const [, sign, parts] = goDuration.exec(text) ?? []
  if (parts === undefined) {
    return 'A duration such as 20s or 3m.'
  }

  return sign !== '-' && seconds(parts) > 0 ? true : 'Must be more than zero.'
}

// baseURL passes no address, or one Jira can be reached at with the token
// kept off the network and out of the address. An address the browser cannot
// read — a port past 65535, say — is left to the server, which reads more.
function baseURL(value: unknown): true | string {
  const text = String(value)
  if (text === '') {
    return true
  }

  if (!schemeAndHost.test(text)) {
    return 'An address such as https://jira.example.com.'
  }

  if (!URL.canParse(text)) {
    return true
  }

  const parsed = new URL(text)
  if (parsed.username !== '' || parsed.password !== '') {
    return 'An address carries no login: the user and the token have fields of their own.'
  }

  return parsed.protocol === 'https:' || thisMachine.test(parsed.hostname)
    ? true
    : 'Over http the token would cross the network in the clear: use https.'
}

// branchTemplate passes the default, left empty, or a template that keeps
// the issue key, so a branch named from it can be read back.
function branchTemplate(value: unknown): true | string {
  const text = String(value)

  return text === '' || text.includes('{key}') ? true : 'Must contain {key}.'
}

// The space taken from around a trailer before it is judged: what trim()
// takes, and the next-line character, which trim() keeps and Go's
// strings.TrimSpace takes.
const goSpaceAround = /^[\s\u0085]+|[\s\u0085]+$/g

// trailer passes a trailer label of one word, without the colon a trailer
// line adds after it: no space, tab or newline once trimmed, as the server
// reads one.
function trailer(value: unknown): true | string {
  const label = String(value).replace(goSpaceAround, '')

  return /[: \t\n]/.test(label) ? 'A single word, without a colon.' : true
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

// A refusal the form holds: the path of the setting refused, what kind of
// refusal it is, and why.
export interface HeldRefusal {
  path: string
  type: string
  message: string
}

// heldRefusals are every refusal the form holds, in the order it refused
// them: of a field, a list's row, or a list as a whole, whose path ends in
// root.
export function heldRefusals(errors: object, under = ''): HeldRefusal[] {
  return Object.entries(errors).flatMap(([key, held]: [string, unknown]) => {
    const path = under === '' ? key : `${under}.${key}`
    if (typeof held !== 'object' || held === null) {
      return []
    }

    const { type, message } = held as Refusal
    if (typeof message === 'string' && typeof type === 'string') {
      return [{ path, type, message }]
    }

    return heldRefusals(held, path)
  })
}

// refusedPaths are the paths of every setting the form holds refused, in the
// order it refused them.
export function refusedPaths(errors: object): string[] {
  return heldRefusals(errors).map(({ path }) => path)
}
