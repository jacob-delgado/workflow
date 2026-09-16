---
title: "Configuration"
weight: 20
---

# Configuration

workflow reads a single JSON file, `.workflow.json`.

```json
{
  "jira": {
    "base_url": "https://jira.example.com",
    "token": "",
    "user": ""
  },
  "slack": {
    "token": "",
    "channel": "#dev-workflow"
  }
}
```

Write a starting copy with `workflow config init`, or `workflow config init
--global` to put it in your home directory.

## Where it looks, and what wins

workflow looks for `.workflow.json` in the current directory first, then in your
home directory.

**A file in the current directory replaces the one in your home directory.** The
two are never merged. A repository-local configuration is therefore the whole
story for that repository, and the two files can never combine into a state that
neither of them describes — which is the failure mode that makes "why is it
using that project?" so hard to debug.

`workflow doctor` names the file in effect, so there is never a question about
which one was read.

## Fields

| Field | Required | Description |
| --- | --- | --- |
| `jira.base_url` | yes | Root URL of your Jira instance, e.g. `https://jira.example.com`. |
| `jira.token` | yes | Personal access token. |
| `jira.user` | no | Only for instances requiring HTTP Basic. See below. |
| `slack.token` | yes | Bot token; starts with `xoxb-`. |
| `slack.channel` | yes | Channel to post in, e.g. `#dev-workflow`. |

Unknown keys are an error rather than being ignored. A misspelled key that
loaded silently would look exactly like a credential you never set.

## Jira token (on-premises / Data Center)

1. Sign in to your Jira instance in a browser.
2. Open your avatar menu → **Profile** → **Personal Access Tokens**.
3. Create a token and copy it into `jira.token`, with your instance's URL in
   `jira.base_url`.

Leave `jira.user` empty to authenticate with that token as a bearer token, which
is what Data Center expects. Set `jira.user` only if your instance requires HTTP
Basic authentication, in which case the token is used as the password.
`workflow doctor` reports which of the two modes is in effect.

## Slack token

1. Create an app at [api.slack.com/apps](https://api.slack.com/apps) in your
   workspace.
2. Under **OAuth & Permissions**, add the `chat:write` bot token scope.
3. Install the app to the workspace and copy the **Bot User OAuth Token** — it
   starts with `xoxb-` — into `slack.token`.
4. Set `slack.channel`, and invite the bot to that channel. Without the invite it
   cannot post there.

## Keeping the tokens safe

`.workflow.json` holds live credentials:

- `workflow config init` writes it mode `0600` — readable only by you.
- It is listed in the repository's `.gitignore`.
- `workflow config show` masks both tokens, printing only the last four
  characters so you can tell two tokens apart.

Nothing in workflow prints a token in full.
