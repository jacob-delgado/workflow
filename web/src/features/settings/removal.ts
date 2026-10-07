import { createContext, useContext } from 'react'
import type { Config } from '@/api/generated/types.gen.ts'

// CredentialPath is a credential a save removes by sending it as null.
export type CredentialPath =
  | 'jira.token'
  | 'forge.token'
  | 'messaging.client_secret'
  | 'messaging.refresh_token'
  | 'messaging.webhook_url'

// Removal is what Remove takes out of the file: a credential, or a Jira
// header, whose value is a credential too and which goes whole.
export type Removal = { credential: CredentialPath } | { header: string }

// credentialWords are how each credential is named in a removal's question
// and in what it said.
const credentialWords: Record<CredentialPath, string> = {
  'jira.token': 'the Jira token',
  'forge.token': 'the forge token',
  'messaging.client_secret': 'the client secret',
  'messaging.refresh_token': 'the refresh token',
  'messaging.webhook_url': 'the webhook URL',
}

// removalWords names what a removal takes out: "the Jira token", or "the
// header CF-Access-Client-Secret".
export function removalWords(removal: Removal): string {
  return 'header' in removal ? `the header ${removal.header}` : credentialWords[removal.credential]
}

// removalConsequence is what goes with a removal beyond what it names: a Slack
// access token was made from the client secret and the refresh token, and
// goes with either.
export function removalConsequence(removal: Removal): string {
  if (
    'credential' in removal &&
    removal.credential.startsWith('messaging.') &&
    removal.credential !== 'messaging.webhook_url'
  ) {
    return 'The access token made from it goes too. '
  }

  return ''
}

// stored reports that the configuration read holds what a removal names.
export function stored(config: Config, removal: Removal): boolean {
  if ('header' in removal) {
    return removal.header in (config.jira.headers ?? {})
  }

  const [section, field] = removal.credential.split('.') as ['jira' | 'forge' | 'messaging', string]
  const value: unknown = (config[section] as Record<string, unknown>)[field]

  return typeof value === 'string' && value !== ''
}

// without is the configuration as read with a removal made: the credential
// sent as null, which the server takes as remove, or the header left out.
export function without(config: Config, removal: Removal): Config {
  if ('header' in removal) {
    const headers = Object.fromEntries(
      Object.entries(config.jira.headers ?? {}).filter(([name]) => name !== removal.header),
    )

    return {
      ...config,
      jira: { ...config.jira, headers: Object.keys(headers).length === 0 ? null : headers },
    }
  }

  const [section, field] = removal.credential.split('.') as ['jira' | 'forge' | 'messaging', string]

  return { ...config, [section]: { ...config[section], [field]: null } }
}

// Removals are what a Remove control asks of the form it is in: whether the
// read holds what it names, and to write the file without it.
interface Removals {
  holds: (removal: Removal) => boolean
  remove: (removal: Removal) => Promise<void>
}

// RemovalsContext is the form's Removals, so a field deep in a fieldset can
// offer a removal without each fieldset passing it down.
export const RemovalsContext = createContext<Removals>({
  holds: () => false,
  remove: () => Promise.resolve(),
})

// useRemovals is the Removals of the form a control is in.
export function useRemovals(): Removals {
  return useContext(RemovalsContext)
}
