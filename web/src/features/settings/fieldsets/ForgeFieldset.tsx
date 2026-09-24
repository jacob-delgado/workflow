import { CheckboxField, Fieldset, SelectField, TextField, type Register } from './Field.tsx'

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
      <TextField
        register={register}
        name="forge.token"
        label="Token"
        type="password"
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
