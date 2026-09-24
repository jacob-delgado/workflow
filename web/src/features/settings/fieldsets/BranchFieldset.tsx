import { Fieldset, TextField, type Register } from './Field.tsx'

// BranchFieldset is how a branch is named for an issue: the template, the
// prefix for a type no rule maps, and how long the summary's slug may run.
export function BranchFieldset({ register }: { register: Register }) {
  return (
    <Fieldset legend="Branch">
      <TextField
        register={register}
        name="branch.template"
        label="Name template"
        hint="Uses {prefix}, {key} and {slug}; must contain {key}."
      />
      <TextField
        register={register}
        name="branch.default_prefix"
        label="Default prefix"
        hint='The prefix for an unmapped type; empty keeps "feat".'
      />
      <TextField
        register={register}
        name="branch.slug_limit"
        label="Slug limit"
        type="number"
        hint="Caps the summary slug's length; 0 keeps 48."
      />
    </Fieldset>
  )
}
