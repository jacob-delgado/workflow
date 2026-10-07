import { Fieldset, SelectField, TextField, type Register } from './Field.tsx'

// MessagingFieldset is where announcements go: the service, its credential —
// a Slack user token or a webhook, one or the other — the channel, and the
// announcement's template. The user token's secrets are kept where workflow
// keeps them: the macOS keychain, unless the file already holds them.
export function MessagingFieldset({ register }: { register: Register }) {
  return (
    <Fieldset legend="Messaging">
      <SelectField
        register={register}
        name="messaging.kind"
        label="Service"
        hint="Slack posts with a user token or a webhook, not both; the others post over a webhook."
        choices={[
          ['', 'Slack (default)'],
          ['teams', 'Microsoft Teams'],
          ['discord', 'Discord'],
          ['webhook', 'Plain webhook'],
        ]}
      />
      <TextField
        register={register}
        name="messaging.client_id"
        label="Client ID"
        hint="Slack user token: your app's client ID, from its Basic Information page."
      />
      <TextField
        register={register}
        name="messaging.client_secret"
        label="Client secret"
        type="password"
        hint="Slack user token: your app's client secret; leave as-is to keep the stored one."
      />
      <TextField
        register={register}
        name="messaging.refresh_token"
        label="Refresh token"
        type="password"
        hint="Slack user token: a refresh token (xoxe-1-…); workflow refreshes it and keeps each new one."
      />
      <TextField
        register={register}
        name="messaging.webhook_url"
        label="Webhook URL"
        type="password"
        hint="A credential; leave as-is to keep it. For Slack, set this or the user token, not both."
      />
      <TextField
        register={register}
        name="messaging.channel"
        label="Channel"
        hint="With a Slack user token; a webhook posts to its own channel."
      />
      <TextField
        register={register}
        name="messaging.announcement"
        label="Announcement"
        hint="Slack only: the review message, from {author}, {noun}, {title}, {url}, {key}, {summary} and {issue_url}. Empty keeps the built-in message."
      />
    </Fieldset>
  )
}
