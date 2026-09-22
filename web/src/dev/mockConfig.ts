import type { Config } from '@/api/generated/types.gen.ts'

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
    user: '',
    project: 'PROJ',
    markdown_comments: false,
    headers: null,
    views: null,
  },
  messaging: {
    kind: 'slack',
    token: '••••••••',
    token_command: '',
    token_env: '',
    webhook_url: '',
    channel: '#dev-workflow',
    channels: ['#dev-workflow', '#releases'],
    announcement: 'Opened {pr} for {issue}',
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
  },
  timing: {
    request_timeout: '20s',
    ci_interval: '30s',
  },
  branch: {
    template: '{type}/{key}-{slug}',
    prefixes: null,
    default_prefix: 'feat',
  },
  commit: {
    default_scope: '',
  },
  pull_request: {
    title_source: '',
  },
}
