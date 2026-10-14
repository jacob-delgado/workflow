import { CheckboxField, Fieldset, TextField, type Register } from './Field.tsx'

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
