import { useForgeWords } from '@/api/health.ts'
import {
  CheckboxField,
  Fieldset,
  SecretField,
  SelectField,
  TextField,
  type Register,
} from './Field.tsx'

// ForgeFieldset is which forge the pull requests live on and how to reach it:
// its kind, its host, and a token or the forge's own command line.
export function ForgeFieldset({ register }: { register: Register }) {
  return (
    <Fieldset legend="Forge">
      <SelectField
        register={register}
        name="forge.kind"
        label="Kind"
        choices={[
          ['', 'Auto-detect'],
          ['github', 'GitHub'],
          ['gitlab', 'GitLab'],
        ]}
      />
      <TextField register={register} name="forge.host" label="Host" />
      <SecretField
        register={register}
        name="forge.token"
        label="Token"
        hint="Leave as-is to keep the stored token."
      />
      <CheckboxField
        register={register}
        name="forge.cli"
        label="Use the forge CLI for authentication"
      />
    </Fieldset>
  )
}

// PullRequestFieldset is where a pull request's title comes from.
export function PullRequestFieldset({ register }: { register: Register }) {
  const { noun } = useForgeWords()

  return (
    <Fieldset legend="Pull request">
      <SelectField
        register={register}
        name="pull_request.title_source"
        label="Title source"
        hint={`Where a ${noun}'s title comes from.`}
        choices={[
          ['', "The branch's oldest commit (default)"],
          ['issue', 'The issue it names'],
        ]}
      />
    </Fieldset>
  )
}
