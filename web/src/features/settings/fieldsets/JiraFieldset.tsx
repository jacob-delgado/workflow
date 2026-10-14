import { useForgeWords } from '@/api/health.ts'
import { type Control, useWatch } from 'react-hook-form'
import type { JiraConfig } from '@/api/generated/types.gen.ts'
import type { SettingsValues } from '../formValues.ts'
import { EntryList, matchedLower } from './EntryList.tsx'
import {
  CheckboxField,
  Fieldset,
  type Kept,
  SecretField,
  TextField,
  type Register,
} from './Field.tsx'

// tokenHint says where a Jira token typed here goes.
const tokenHint =
  'A personal access token. One typed here is kept in your keychain for this address on macOS, or in the file elsewhere.'

// hostOf is the host of Jira's address, or the words for it while the
// address is not one.
function hostOf(baseURL: string): string {
  return URL.canParse(baseURL) ? new URL(baseURL).host : 'this address'
}

// tokenKept is where the Jira token is read from when the file holds none:
// the keychain over the variable over the command, or none of them.
function tokenKept(
  sources: Pick<JiraConfig, 'base_url' | 'keychain' | 'token_command' | 'token_env'>,
): Kept {
  if (sources.keychain) {
    return { from: 'keychain', host: hostOf(sources.base_url ?? '') }
  }

  if (sources.token_env) {
    return { from: 'variable', name: sources.token_env }
  }

  return sources.token_command ? { from: 'command' } : { from: 'nowhere' }
}

interface JiraFieldsetProps {
  register: Register
  control: Control<SettingsValues>
}

// JiraToken is the credential: the token, said to be kept where the token in
// effect comes from, and whether the keychain item for the address keeps it.
function JiraToken({ register, control }: JiraFieldsetProps) {
  const [baseURL, keychain, command, variable] = useWatch({
    control,
    name: ['jira.base_url', 'jira.keychain', 'jira.token_command', 'jira.token_env'],
  })
  const sources = { base_url: baseURL, keychain, token_command: command, token_env: variable }

  return (
    <>
      <SecretField
        register={register}
        name="jira.token"
        label="Token"
        hint={tokenHint}
        elsewhere={tokenKept(sources)}
      />
      <CheckboxField
        register={register}
        name="jira.keychain"
        label="Read the token from your keychain, kept there for this address (macOS)"
      />
    </>
  )
}

// JiraFieldset is where the tracker is and who reads it: its address, the
// credential, the project and the status an issue moves to once in review —
// and whether the repository's own forge issues join Jira's in the list.
export function JiraFieldset({ register, control }: JiraFieldsetProps) {
  const { noun } = useForgeWords()
  const headers = useWatch({ control, name: 'jira.headers' })

  return (
    <Fieldset legend="Jira">
      <TextField register={register} name="jira.base_url" label="Base URL" type="url" />
      <JiraToken register={register} control={control} />
      <TextField
        register={register}
        name="jira.user"
        label="User"
        hint="Empty authenticates with the token as a bearer."
      />
      <TextField register={register} name="jira.project" label="Project" />
      <TextField
        register={register}
        name="jira.review_status"
        label="Review status"
        hint={`The status an issue moves to once its ${noun} is open, e.g. "In Review". Empty makes no offer.`}
      />
      <EntryList
        control={control}
        register={register}
        name="jira.views"
        legend="Views"
        entry="view"
        hint="The issue lists v moves between, each a name and its JQL. None keeps the one built-in list: open issues assigned to you."
        columns={[
          { field: 'name', label: 'Name' },
          { field: 'jql', label: 'JQL', wide: true },
        ]}
        blank={{ name: '', jql: '' }}
      />
      <EntryList
        control={control}
        register={register}
        name="jira.headers"
        legend="Headers"
        entry="header"
        hint="Sent with every Jira request, for a proxy that wants one. Each value is a credential: leave it as-is to keep it."
        columns={[
          { field: 'name', label: 'Name' },
          { field: 'value', label: 'Value', secret: true, wide: true },
        ]}
        blank={{ name: '', value: '', stored: false }}
        unique={{
          namesOf: (values) => values.jira.headers.map((header) => header.name),
          key: matchedLower,
          taken: 'A header of that name is already listed.',
        }}
        storedAs={(index) => {
          const header = headers[index]

          return header?.stored ? { header: header.name } : null
        }}
      />
      <CheckboxField
        register={register}
        name="jira.markdown_comments"
        label="Write comments in Markdown, posted as Jira wiki markup"
      />
      <CheckboxField
        register={register}
        name="issues.forge"
        label="List this repository's GitHub or GitLab issues beside Jira's"
      />
    </Fieldset>
  )
}
