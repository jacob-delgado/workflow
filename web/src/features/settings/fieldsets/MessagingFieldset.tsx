import { Fieldset, SelectField, TextField, type Register } from './Field.tsx'

// MessagingFieldset is where announcements go: the service, its credential —
// a bot token or a webhook — the channel, and the announcement's template.
export function MessagingFieldset({ register }: { register: Register }) {
  return (
    <Fieldset legend="Messaging">
      <SelectField
        register={register}
        name="messaging.kind"
        label="Service"
        hint="Slack posts over a bot token or a webhook; the others post over a webhook."
        choices={[
          ['slack', 'Slack'],
          ['teams', 'Microsoft Teams'],
          ['discord', 'Discord'],
          ['webhook', 'Plain webhook'],
        ]}
      />
      <TextField
        register={register}
        name="messaging.token"
        label="Bot token"
        type="password"
        hint="Slack only; leave as-is to keep the stored token."
      />
      <TextField
        register={register}
        name="messaging.webhook_url"
        label="Webhook URL"
        type="password"
        hint="A credential; leave as-is to keep it."
      />
      <TextField register={register} name="messaging.channel" label="Channel" />
      <TextField register={register} name="messaging.announcement" label="Announcement" />
    </Fieldset>
  )
}
