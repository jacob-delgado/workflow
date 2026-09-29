import { CheckboxField, Fieldset, TextField, type Register } from './Field.tsx'

// TaskwarriorFieldset is which task program is Taskwarrior, and whether the
// integration is on at all. workflow finds Taskwarrior once, as it starts, so a
// change saved here waits for the next start.
export function TaskwarriorFieldset({ register }: { register: Register }) {
  return (
    <Fieldset legend="Taskwarrior" hint="A change here applies when workflow restarts.">
      <TextField
        register={register}
        name="taskwarrior.program"
        label="Task program"
        hint="Empty tries every task on PATH and keeps the first that is Taskwarrior 3.5.0 or newer."
      />
      <CheckboxField
        register={register}
        name="taskwarrior.disabled"
        label="Turn off the Taskwarrior integration"
      />
    </Fieldset>
  )
}
