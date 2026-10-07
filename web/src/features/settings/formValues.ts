import type { Config } from '@/api/generated/types.gen.ts'

// formValues is the configuration as the Settings form holds it. A setting the
// file may spell as its default's name — the messaging service "slack", the
// title source "commit" — is held empty, which means the same, so its select
// shows the one choice that stands for the default.
export function formValues(config: Config): Config {
  return {
    ...config,
    messaging: {
      ...config.messaging,
      kind: config.messaging.kind === 'slack' ? '' : config.messaging.kind,
    },
    pull_request: {
      ...config.pull_request,
      title_source:
        config.pull_request.title_source === 'commit' ? '' : config.pull_request.title_source,
    },
  }
}
