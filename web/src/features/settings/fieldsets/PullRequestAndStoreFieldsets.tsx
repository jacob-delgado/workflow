import { useForgeWords } from '@/api/health.ts'
import { CheckboxField, Fieldset, SelectField, type Register } from './Field.tsx'

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
          ['commit', "The branch's oldest commit"],
          ['issue', 'The issue it names'],
        ]}
      />
    </Fieldset>
  )
}

// StoreFieldset is whether anything is kept on disk between sessions.
export function StoreFieldset({ register }: { register: Register }) {
  return (
    <Fieldset legend="Store">
      <CheckboxField
        register={register}
        name="store.disabled"
        label="Keep nothing on disk between sessions"
      />
    </Fieldset>
  )
}
