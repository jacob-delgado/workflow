import { useForgeWords } from '@/api/health.ts'
import { type Control, useWatch } from 'react-hook-form'
import type { JiraConfig } from '@/api/generated/types.gen.ts'
import type { SettingsValues } from '../formValues.ts'
import { EntryList, matchedLower } from './EntryList.tsx'
import { CheckboxField, Fieldset, SecretField, TextField, type Register } from './Field.tsx'

// TokenSources are where the Jira token may come from besides the file: the
// keychain, a program that prints it, or a variable that holds it. None is a
// secret.
type TokenSources = Pick<JiraConfig, 'token' | 'keychain' | 'token_command' | 'token_env'>

// typedTokenKept says where a Jira token typed here goes.
const typedTokenKept = 'kept in your keychain for this address on macOS or in the file elsewhere'

// tokenHint says where the Jira token comes from: the file's own token wins,
// then the keychain, then token_env, then token_command, so a token typed over
// a source is used instead of it.
function tokenHint({ token, ...sources }: TokenSources): string {
  const source = tokenSource(sources)
  if (source === '') {
    return token
      ? 'Leave as-is to keep the stored token.'
      : `A personal access token, ${typedTokenKept}.`
  }

  return token
    ? `The token stored here is used over ${source}.`
    : `Taken from ${source}. One typed here is used instead, ${typedTokenKept}.`
}

// tokenSource names the source the token is read from when the file holds
// none: the keychain over the variable over the command, or none of them.
function tokenSource({ keychain, token_command, token_env }: Omit<TokenSources, 'token'>): string {
  if (keychain) {
    return 'your keychain, for this address'
  }

  if (token_env) {
    return `token_env: ${token_env}`
  }

  return token_command ? `token_command: ${token_command}` : ''
}

interface JiraFieldsetProps {
  register: Register
  control: Control<SettingsValues>
  // storedToken is the token as read, masked: the hint says where the token in
  // effect comes from, not what is being typed.
  storedToken: string | null
}

// JiraToken is the credential: the token, said to come from where the token
// in effect comes from, and whether the keychain item for the address keeps it.
function JiraToken({ register, control, storedToken }: JiraFieldsetProps) {
  const [keychain, command, variable] = useWatch({
    control,
    name: ['jira.keychain', 'jira.token_command', 'jira.token_env'],
  })
  const sources = { keychain, token_command: command, token_env: variable }

  return (
    <>
      <SecretField
        register={register}
        name="jira.token"
        label="Token"
        hint={tokenHint({ token: storedToken, ...sources })}
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
export function JiraFieldset({ register, control, storedToken }: JiraFieldsetProps) {
  const { noun } = useForgeWords()
  const headers = useWatch({ control, name: 'jira.headers' })

  return (
    <Fieldset legend="Jira">
      <TextField register={register} name="jira.base_url" label="Base URL" type="url" />
      <JiraToken register={register} control={control} storedToken={storedToken} />
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
