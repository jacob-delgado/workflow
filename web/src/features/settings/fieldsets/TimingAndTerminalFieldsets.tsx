import { CheckboxField, Fieldset, SelectField, TextField, type Register } from './Field.tsx'

// TimingFieldset is how long workflow waits, for a network or a service
// slower or more rate-limited than the defaults assume.
export function TimingFieldset({ register }: { register: Register }) {
  return (
    <Fieldset
      legend="Timing"
      hint="Each is a duration, such as 20s or 3m; empty keeps the default."
    >
      <TextField
        register={register}
        name="timing.request_timeout"
        label="Request timeout"
        hint="How long each request to a service may take; empty keeps 10s."
      />
      <TextField
        register={register}
        name="timing.ci_interval"
        label="CI interval"
        hint="How often CI is asked about while it runs; empty keeps 20s."
      />
    </Fieldset>
  )
}

// TerminalFieldset is how the terminal interface draws and behaves.
export function TerminalFieldset({ register }: { register: Register }) {
  return (
    <Fieldset
      legend="Terminal"
      hint="How the terminal interface draws and behaves; a change applies when it next opens."
    >
      <CheckboxField register={register} name="ui.ascii" label="Draw in plain ASCII" />
      <CheckboxField register={register} name="ui.mouse" label="Capture the mouse" />
      <SelectField
        register={register}
        name="ui.color"
        label="Color"
        hint="Never keeps bold, faint and the reverse-video cursor, which mean the same without color."
        choices={[
          ['', 'When the terminal allows (default)'],
          ['never', 'Never'],
        ]}
      />
      <CheckboxField
        register={register}
        name="ui.notify"
        label="Ring the terminal when CI finishes"
      />
      <TextField
        register={register}
        name="ui.comments_shown"
        label="Comments shown"
        type="number"
        hint="How many of an issue's latest comments the detail shows; 0 keeps 5."
      />
    </Fieldset>
  )
}
