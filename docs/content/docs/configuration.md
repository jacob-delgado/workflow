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
    "webhook_url": "",
    "channel": "#dev-workflow"
  },
  "forge": {
    "token": ""
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
| `slack.token` | one of these two | Bot token; starts with `xoxb-`. |
| `slack.webhook_url` | one of these two | Incoming webhook URL. **This is a credential**, not just an address. |
| `slack.channel` | only with `slack.token` | Channel to post in, e.g. `#dev-workflow`. A webhook carries its own. |
| `forge.token` | **no** | GitHub or GitLab token. Usually leave it empty — see below. |

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

## Slack: a webhook or a bot token

Set either one. If you set both, the bot token is used — it is the more capable
transport, and a configuration that has both is not an error.

`workflow doctor` reports which of the two is in effect.

### Incoming webhook — the two-minute option

1. Create an app at [api.slack.com/apps](https://api.slack.com/apps) in your
   workspace.
2. Turn on **Incoming Webhooks**, choose **Add New Webhook to Workspace**, and
   pick the channel it posts to.
3. Copy the URL into `slack.webhook_url`.

The webhook is bound to the channel you chose, so `slack.channel` does not apply
and is not required. There is no app review, no scope to request and no bot to
invite.

The trade-off is that a webhook posts and nothing else: it cannot tell workflow
the message's timestamp, so later events arrive as new messages rather than
replies, and it can never post anywhere but that one channel.

### Bot token — choose the channel at runtime

1. Create an app at [api.slack.com/apps](https://api.slack.com/apps) in your
   workspace.
2. Under **OAuth & Permissions**, add the `chat:write` bot token scope.
3. Install the app to the workspace and copy the **Bot User OAuth Token** — it
   starts with `xoxb-` — into `slack.token`.
4. Set `slack.channel`, and invite the bot to that channel. Without the invite it
   cannot post there.

## The forge token you probably do not need

`forge.token` is consulted **last**, and most people never set it. workflow looks
for a GitHub or GitLab credential in this order:

1. `$GITHUB_TOKEN` or `$GH_TOKEN` (`$GITLAB_TOKEN` or `$GLAB_TOKEN` for GitLab)
2. `gh auth token`, if `gh` is installed and signed in to that host
3. `forge.token`

So if you already use `gh auth login`, there is nothing to configure and no
second copy of a credential to keep safe. `workflow doctor --online` reports
which of the three it used, which is what answers "why is it using that one?".

There is no `glab` step. `glab` reports its token through `auth status`, whose
output is prose on standard error, and parsing prose is not something to put a
credential behind — GitLab users set `$GITLAB_TOKEN` or `forge.token` instead.

`forge.token` is never reported as missing, because failing `doctor` for everyone
correctly relying on `gh auth login` would be wrong.

## Keeping the tokens safe

`.workflow.json` holds live credentials:

- `workflow config init` writes it mode `0600` — readable only by you.
- It is listed in the repository's `.gitignore`.
- `workflow config show` masks every credential — Jira, Slack, the webhook URL
  and the forge token — printing only the last four characters so you can tell
  two apart.

**`slack.webhook_url` is masked like a token, because it is one.** Anyone holding
that URL can post to your channel; it is a password that happens to look like an
address. `workflow doctor` never prints it at all — not even masked — because
doctor's output is what the bug report template asks people to paste.

Nothing in workflow prints a credential in full.
