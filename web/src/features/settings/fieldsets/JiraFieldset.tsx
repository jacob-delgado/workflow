import { useForgeWords } from '@/api/health.ts'
import { CheckboxField, Fieldset, TextField, type Register } from './Field.tsx'

// JiraFieldset is where the tracker is and who reads it: its address, the
// credential, the project and the status an issue moves to once in review.
export function JiraFieldset({ register }: { register: Register }) {
  const { noun } = useForgeWords()

  return (
    <Fieldset legend="Jira">
      <TextField register={register} name="jira.base_url" label="Base URL" type="url" />
      <TextField
        register={register}
        name="jira.token"
        label="Token"
        type="password"
        hint="Leave as-is to keep the stored token."
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
      <CheckboxField
        register={register}
        name="jira.markdown_comments"
        label="Write comments in Markdown, posted as Jira wiki markup"
      />
    </Fieldset>
  )
}
