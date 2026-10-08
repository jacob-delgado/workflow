import { CheckboxField, Fieldset, TextField, type Register } from './Field.tsx'

// TaskwarriorFieldset is which task program is Taskwarrior, and whether the
// integration is on at all. workflow finds Taskwarrior once, as it starts, so a
// change saved here waits for the next start. The program is shown but not
// changed here: workflow runs it as you, so the server keeps it as the file
// holds it, and refuses a save that changes it.
export function TaskwarriorFieldset({ register }: { register: Register }) {
  return (
    <Fieldset legend="Taskwarrior" hint="A change here applies when workflow restarts.">
      <TextField
        register={register}
        name="taskwarrior.program"
        label="Task program"
        readOnly
        hint="Set in the file: workflow runs it as you, so Settings keeps it as it is. Empty tries every task in an absolute PATH directory and keeps the first that is Taskwarrior 3.5.0 or newer."
      />
      <CheckboxField
        register={register}
        name="taskwarrior.disabled"
        label="Turn off the Taskwarrior integration"
      />
    </Fieldset>
  )
}
