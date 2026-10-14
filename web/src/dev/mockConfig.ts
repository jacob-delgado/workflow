import type { Config } from '../api/generated/types.gen.ts'

// The configuration `task web:mockup` shows in Settings, shaped like a real read:
// secrets already masked (the server never sends them in the clear), collections
// present so a save round-trips them. Dev-only, code-split out of production.
export const mockConfig: Config = {
  version: '1',
  jira: {
    base_url: 'https://jira.acme.internal',
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
    announcement: '{author} opened <{url}|{title}> for {key}: {summary}',
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
