import { splitList } from '@/lib/utils.ts'
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
