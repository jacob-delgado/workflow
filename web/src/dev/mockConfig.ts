import type { Config, RefusedSetting } from '../api/generated/types.gen.ts'

// The configuration `task web:mockup` shows in Settings, shaped like a real read:
// secrets already masked (the server never sends them in the clear), collections
// present so a save round-trips them. Dev-only, code-split out of production.
export const mockConfig: Config = {
  version: '1',
  jira: {
    base_url: 'https://jira.example.com',
    token: '••••••••',
    token_command: '',
    token_env: '',
    keychain: false,
    user: '',
    project: 'PROJ',
    review_status: '',
    markdown_comments: true,
    headers: null,
    views: null,
  },
  messaging: {
    kind: 'slack',
    client_id: '1234.5678',
    client_secret: '',
    refresh_token: '',
    access_token: '',
    expires_at: '',
    webhook_url: '',
    channel: '#dev-workflow',
    channels: ['#dev-workflow', '#releases'],
    announcement: '',
    auto_announce: true,
  },
  forge: {
    kind: 'github',
    host: 'github.com',
    token: '••••••••',
    cli: false,
  },
  ui: {
    mouse: true,
    ascii: false,
    color: '',
    motion: '',
    accessible: false,
    notify: true,
    comments_shown: 5,
    keys: {},
    web_shortcuts: true,
  },
  timing: {
    request_timeout: '20s',
    ci_interval: '30s',
  },
  branch: {
    template: '{prefix}/{key}-{slug}',
    prefixes: null,
    default_prefix: 'feat',
    slug_limit: 0,
  },
  commit: {
    default_scope: '',
    types: null,
    subject_limit: 0,
    refs_trailer: '',
  },
  pull_request: {
    title_source: '',
  },
  store: {
    disabled: false,
  },
  taskwarrior: {
    program: '',
    disabled: false,
  },
  issues: {
    forge: false,
  },
}

// goDuration is what Go's time.ParseDuration reads: numbers, each with its
// unit, such as "1h30m" or "1.5s", signed or not, or a bare 0. A number's
// digits match one way only: "\d+\.?\d*" could split "99" as 9 and 9, and a
// long non-duration would then try every split.
const goDuration = /^[-+]?(?:0|(?:(?:\d+(?:\.\d*)?|\.\d+)(?:ns|us|µs|μs|ms|s|m|h))+)$/

// mockTimingRefusal is the setting a save of config refuses as the server's
// configuration does, in its words: the first timing that is no duration, or
// none more than zero; undefined where none is. The mockup checks the timings
// alone, so Settings' marking of a refused setting shows with no server.
export function mockTimingRefusal(config: Config): RefusedSetting | undefined {
  const timings = [
    { key: 'timing.request_timeout', value: config.timing.request_timeout ?? '' },
    { key: 'timing.ci_interval', value: config.timing.ci_interval ?? '' },
  ]
  for (const { key, value } of timings) {
    const reason = timingRefusal(value)
    if (reason !== undefined) {
      return { key, detail: `invalid timing: ${reason}: ${JSON.stringify(value)}` }
    }
  }

  return undefined
}

// timingRefusal is why the server refuses value as a timing, or undefined: an
// empty one is the default.
function timingRefusal(value: string): string | undefined {
  if (value === '') {
    return undefined
  }
  if (!goDuration.test(value)) {
    return 'not a duration such as "20s"'
  }

  return value.startsWith('-') || !/[1-9]/.test(value) ? 'must be more than zero' : undefined
}
