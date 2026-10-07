import { useForgeWords } from '@/api/health.ts'
import { type Control, useWatch } from 'react-hook-form'
import type { JiraConfig } from '@/api/generated/types.gen.ts'
import type { SettingsValues } from '../formValues.ts'
import { EntryList, matchedLower } from './EntryList.tsx'
import { CheckboxField, Fieldset, SecretField, TextField, type Register } from './Field.tsx'

// TokenSources are where the Jira token may come from besides the file: a
// program that prints it, or a variable that holds it. Neither is a secret.
type TokenSources = Pick<JiraConfig, 'token' | 'token_command' | 'token_env'>

// tokenHint says where the Jira token comes from: the file's own token wins,
// then token_env, then token_command, so a token typed over a source sends the
// secret into the file the source kept it out of.
function tokenHint({ token, token_command, token_env }: TokenSources): string {
  const source = tokenSource(token_command, token_env)
  if (source === '') {
    return token
      ? 'Leave as-is to keep the stored token.'
      : 'A personal access token. token_command or token_env, set in the file, keep it out of the file.'
  }

  return token
    ? `The token stored here is used over ${source}.`
    : `Taken from ${source}. A token typed here is kept in the file and used instead.`
}

// tokenSource names the source the token is read from when the file holds
// none: the variable over the command, or neither.
function tokenSource(command: string | undefined, variable: string | undefined): string {
  if (variable) {
    return `token_env: ${variable}`
  }

  return command ? `token_command: ${command}` : ''
}

interface JiraFieldsetProps {
  register: Register
  control: Control<SettingsValues>
  // storedToken is the token as read, masked: the hint says where the token in
  // effect comes from, not what is being typed.
  storedToken: string | null
}

// JiraFieldset is where the tracker is and who reads it: its address, the
// credential, the project and the status an issue moves to once in review —
// and whether the repository's own forge issues join Jira's in the list.
export function JiraFieldset({ register, control, storedToken }: JiraFieldsetProps) {
  const { noun } = useForgeWords()
  const [command, variable, headers] = useWatch({
    control,
    name: ['jira.token_command', 'jira.token_env', 'jira.headers'],
  })

  return (
    <Fieldset legend="Jira">
      <TextField register={register} name="jira.base_url" label="Base URL" type="url" />
      <SecretField
        register={register}
        name="jira.token"
        label="Token"
        hint={tokenHint({ token: storedToken, token_command: command, token_env: variable })}
      />
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
