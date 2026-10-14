import type { Control } from 'react-hook-form'
import { splitList } from '@/lib/utils.ts'
import type { SettingsValues } from '../formValues.ts'
import { EntryList, matchedLower } from './EntryList.tsx'
import { Fieldset, TextField, type Register } from './Field.tsx'

// CommitFieldset is the commit convention: the scope a commit opens on, the
// types offered, how long a subject may be, and the trailer naming the issue.
export function CommitFieldset({ register }: { register: Register }) {
  return (
    <Fieldset legend="Commit">
      <TextField
        register={register}
        name="commit.default_scope"
        label="Default scope"
        hint="Pre-fills the scope field until a commit here uses a scope of its own, e.g. an area you scope commits to."
      />
      <TextField
        register={register}
        name="commit.types"
        label="Types"
        hint="Comma-separated, in the order to offer them; empty keeps the Conventional Commit types."
        readAs={(value) => (typeof value === 'string' ? splitList(value) : value)}
      />
      <TextField
        register={register}
        name="commit.subject_limit"
        label="Subject limit"
        type="number"
        hint="The longest a subject may be, in characters; 0 keeps 72."
      />
      <TextField
        register={register}
        name="commit.refs_trailer"
        label="Issue trailer"
        hint='The trailer label added to a commit body; empty keeps "Refs".'
      />
    </Fieldset>
  )
}

// BranchFieldset is how a branch is named for an issue: the template, the
// prefix for a type no rule maps, and how long the summary's slug may run.
export function BranchFieldset({
  register,
  control,
}: {
  register: Register
  control: Control<SettingsValues>
}) {
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
      <EntryList
        control={control}
        register={register}
        name="branch.prefixes"
        legend="Prefixes"
        entry="prefix"
        hint="The prefix for each issue type, matched without regard to case. Any here replace the built-in ones."
        columns={[
          { field: 'type', label: 'Issue type' },
          { field: 'prefix', label: 'Prefix' },
        ]}
        blank={{ type: '', prefix: '' }}
        unique={{
          namesOf: (values) => values.branch.prefixes.map((rule) => rule.type),
          key: matchedLower,
          taken: 'A prefix of that issue type is already listed.',
        }}
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
