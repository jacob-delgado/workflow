import type { Config, JiraView } from '@/api/generated/types.gen.ts'

// A channel as a row of the form: a field array holds objects, not strings.
interface ChannelRow {
  name: string
}

// A branch prefix rule as a row of the form: the issue type and its prefix.
interface PrefixRow {
  type: string
  prefix: string
}

// A Jira header as a row of the form. stored marks one the file holds, whose
// value arrives masked and whose name is not edited: a renamed header would
// send the mask as the new one's value.
interface HeaderRow {
  name: string
  value: string
  stored: boolean
}

// SettingsValues is the configuration as the Settings form holds it: each
// list a field array of rows, ui.keys a plain map, and every other setting as
// the file has it.
export interface SettingsValues extends Omit<Config, 'jira' | 'messaging' | 'branch' | 'ui'> {
  jira: Omit<Config['jira'], 'views' | 'headers'> & { views: JiraView[]; headers: HeaderRow[] }
  messaging: Omit<Config['messaging'], 'channels'> & { channels: ChannelRow[] }
  branch: Omit<Config['branch'], 'prefixes'> & { prefixes: PrefixRow[] }
  ui: Omit<Config['ui'], 'keys'> & { keys: Record<string, string> }
}

// formValues is the configuration as the Settings form holds it. A setting the
// file may spell as its default's name — the messaging service "slack", the
// title source "commit" — is held empty, which means the same, so its select
// shows the one choice that stands for the default.
export function formValues(config: Config): SettingsValues {
  const { jira, messaging, branch, ui, pull_request } = config

  return {
    ...config,
    jira: {
      ...jira,
      views: jira.views ?? [],
      headers: Object.entries(jira.headers ?? {}).map(([name, value]) => ({
        name,
        value,
        stored: true,
      })),
    },
    messaging: {
      ...messaging,
      kind: messaging.kind === 'slack' ? '' : messaging.kind,
      channels: (messaging.channels ?? []).map((name) => ({ name })),
    },
    branch: {
      ...branch,
      prefixes: Object.entries(branch.prefixes ?? {}).map(([type, prefix]) => ({ type, prefix })),
    },
    ui: { ...ui, keys: { ...ui.keys } },
    pull_request: {
      ...pull_request,
      title_source: pull_request.title_source === 'commit' ? '' : pull_request.title_source,
    },
  }
}

// configOf is what the form holds as the configuration a save sends: a row
// left without its name dropped, a list with no rows sent as none, and a key
// left empty, which keeps the action's default, left out of ui.keys.
export function configOf(values: SettingsValues): Config {
  const { jira, messaging, branch, ui } = values

  return {
    ...values,
    jira: {
      ...jira,
      views: noneIfEmpty(jira.views.filter((view) => view.name.trim() !== '')),
      headers: headersOf(jira.headers),
    },
    messaging: {
      ...messaging,
      channels: noneIfEmpty(
        messaging.channels.map((channel) => channel.name.trim()).filter((name) => name !== ''),
      ),
    },
    branch: {
      ...branch,
      prefixes: mapOf(branch.prefixes.map((rule) => [rule.type.trim(), rule.prefix.trim()])),
    },
    ui: {
      ...ui,
      keys: mapOf(Object.entries(ui.keys).map(([action, key]) => [action, key.trim()])),
    },
  }
}

// headersOf is the header rows as the map the file holds. A stored header
// goes back whatever its value — left masked or emptied, the server keeps the
// stored one — and a new header only once it has a value.
function headersOf(rows: HeaderRow[]): Record<string, string> | null {
  const kept = rows
    .map((row) => ({ ...row, name: row.name.trim() }))
    .filter((row) => row.name !== '' && (row.stored || row.value !== ''))

  return kept.length === 0 ? null : Object.fromEntries(kept.map((row) => [row.name, row.value]))
}

// noneIfEmpty is a list, or null — as the file writes no list — when it has
// no entries.
function noneIfEmpty<T>(list: T[]): T[] | null {
  return list.length === 0 ? null : list
}

// mapOf is the entries with both a name and a value as a map, or null when
// none has.
function mapOf(entries: [string, string][]): Record<string, string> | null {
  const kept = entries.filter(([name, value]) => name !== '' && value !== '')

  return kept.length === 0 ? null : Object.fromEntries(kept)
}
