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
    "kind": "",
    "host": "",
    "token": ""
  },
  "ui": {
    "mouse": true,
    "ascii": false,
    "color": ""
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

## The issue cache

So the Issues pane is useful the instant it opens, workflow remembers the last
session's assigned-issue list in a small file under your user cache directory
(`~/.cache/workflow` on Linux, `~/Library/Caches/workflow` on macOS), one file
per Jira instance. It holds only what the pane shows — issue keys, summaries and
statuses, never descriptions or comments — written so that only you can read it,
and it is replaced from Jira on every start. Delete it at any time; it is
rebuilt on the next run, and until it exists the pane simply waits on Jira as it
always did.

## Fields

| Field | Required | Description |
| --- | --- | --- |
| `jira.base_url` | yes | Root URL of your Jira instance, e.g. `https://jira.example.com`. |
| `jira.token` | one of these three | Personal access token. |
| `jira.token_command` | one of these three | A program that prints the token, e.g. `pass show jira/token`. See below. |
| `jira.token_env` | one of these three | An environment variable that holds the token. |
| `jira.user` | no | Only for instances requiring HTTP Basic. See below. |
| `jira.views` | no | Named issue lists (`name` + `jql`) the pane moves between with `v`. Empty keeps the one built-in list. See below. |
| `slack.token` | for a bot | Bot token; starts with `xoxb-`. Or use `slack.token_command` / `slack.token_env`. |
| `slack.token_command` | for a bot | A program that prints the bot token. |
| `slack.token_env` | for a bot | An environment variable that holds the bot token. |
| `slack.webhook_url` | for a webhook | Incoming webhook URL. **This is a credential**, not just an address. |
| `slack.channel` | only with `slack.token` | Channel to post in, e.g. `#dev-workflow`. A webhook carries its own. |
| `slack.announcement` | no | Template for the review message, from `{author}`, `{noun}`, `{title}`, `{url}`, `{key}`, `{summary}`, `{issue_url}`. Empty uses the built-in message. |
| `forge.kind` | on-prem only | `github` or `gitlab`, for a host whose name says neither. |
| `forge.host` | with `forge.kind` | The host `forge.kind` and `forge.token` are for, e.g. `git.example.com`. |
| `forge.token` | **no** | GitHub or GitLab token. Usually leave it empty — see below. |
| `ui.mouse` | no | Capture the mouse, so a click focuses a pane or selects a row. Defaults to `true`. |
| `ui.ascii` | no | Draw borders and glyphs in plain ASCII. Defaults to `false`. |
| `ui.color` | no | `never` turns off the system hues; bold, faint and the cursor stay. Empty (the default) draws them. `NO_COLOR` also turns them off. |
| `ui.notify` | no | Ring the terminal (and raise a desktop notification where it relays one) when CI finishes. Defaults to `false`. |
| `branch.template` | no | Shape of a proposed branch name from `{prefix}`, `{key}` and `{slug}`. Must contain `{key}`. Defaults to `{prefix}/{key}-{slug}`. |
| `branch.prefixes` | no | Map from issue type to branch prefix, e.g. `{"bug": "bugfix"}`. The type is matched without regard to case, and this replaces the built-in `{"bug": "fix"}` rather than adding to it. |
| `branch.default_prefix` | no | Prefix for an issue type not named in `branch.prefixes`. Defaults to `feat`. |

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

## Issue views

By default the Issues pane shows one list: the open issues assigned to you. If
you pick work from a sprint, a team filter or the unassigned pile, name those
lists under `jira.views` and press `v` to move between them:

```json
{
  "jira": {
    "views": [
      { "name": "My work", "jql": "assignee = currentUser() AND statusCategory != done" },
      { "name": "Sprint board", "jql": "sprint in openSprints() AND statusCategory != done" },
      { "name": "Needs triage", "jql": "project = OPS AND assignee is EMPTY ORDER BY created" }
    ]
  }
}
```

The first view is shown at start, `v` switches to the next, and the pane title
names the one in use. Each view is any JQL your instance accepts. A view missing
its `name` or its `jql` is refused when the file loads, rather than showing an
empty pane with no way to tell why. With no `jira.views` at all, the built-in
"assigned to me" list is the only one, and `v` does nothing.

## Keeping tokens out of the file

So the file need hold no secret, a Jira or Slack token can instead come from a
program or an environment variable:

- `token_command` runs a program and reads the token from its output, e.g.
  `pass show jira/token`, `op read "op://vault/jira/token"`, or
  `security find-generic-password -s workflow-jira -w`. The command is split on
  spaces and run directly — no shell — so wrap a pipeline in a script if you need
  one.
- `token_env` reads the token from an environment variable, e.g. `WORKFLOW_JIRA_TOKEN`.

The file's own `token` wins when set, then `token_env`, then `token_command`.
`workflow doctor --online` reports which source each credential came from,
without ever printing the value.
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

### Your team's own words

By default the review message reads `jacob opened a pull request: <link>` and,
on the next line, the linked issue. `slack.announcement` shapes it to a house
style — an emoji, a reviewers line, a group to mention:

```json
{
  "slack": {
    "announcement": "🚀 {author} opened {noun} <{url}|{title}> — {key} {summary}"
  }
}
```

The placeholders are `{author}`, `{noun}` (pull request or merge request),
`{title}`, `{url}`, `{key}`, `{summary}` and `{issue_url}`. The rest of the
template — text, emoji, and Slack markup like `<{url}|{title}>` for a link — is
yours to write.

Every **substituted value** is escaped, always. A pull request title is anyone's
to write, and an unescaped `<!channel>` in one would ping everyone in the
channel; escaping is what stops that, and it is not optional. You preview the
message before it is posted, so you see exactly what will go out.

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

### Every token is for a host

Each of those answers for one host, the way `gh` and `glab` read the same
variables, and a token is offered to the host it is for and to no other:

| Source | Is for |
| --- | --- |
| `$GITHUB_TOKEN`, `$GH_TOKEN` | `github.com`, and an Enterprise Cloud tenant under `ghe.com` |
| `$GH_ENTERPRISE_TOKEN`, `$GITHUB_ENTERPRISE_TOKEN` | the host `$GH_HOST` names |
| `$GITLAB_TOKEN`, `$GLAB_TOKEN` | the host `$GITLAB_HOST` (or `$GL_HOST`) names, and `gitlab.com` when neither names one |
| `gh auth token` | whichever hosts you have signed `gh` in to |
| `forge.token` | `forge.host`, and with no `forge.host`, `github.com` or `gitlab.com` |

So a GitHub Enterprise Server at `git.example.com` takes
`GH_HOST=git.example.com` beside `$GH_ENTERPRISE_TOKEN`, or `gh auth login
--hostname git.example.com`, or `forge.host` beside `forge.token`. When no source
has a token for the host, `workflow doctor --online` says which of these it would
have read.

### On-premises forges need `forge.kind` and `forge.host`

workflow reads the forge from your git remote. `github.com` and `gitlab.com` name
themselves; `git.example.com` does not, and a GitHub Enterprise Server looks
exactly like a self-managed GitLab from a remote URL alone — while their APIs live
at different paths. Rather than guess and send a token to the wrong service, say
which forge it is, and which host you mean:

```json
"forge": { "kind": "github", "host": "git.example.com" }
```

`forge.kind` describes `forge.host` and says nothing about any other host, so a
repository whose remote is somewhere else is still a host workflow cannot name.
A `forge.kind` with no `forge.host` is reported as incomplete.

`forge.kind` only fills that gap. On `github.com` or `gitlab.com` it is ignored,
because the remote is the better evidence.

## The interface: mouse, ASCII and color

`ui.mouse` is on unless you turn it off. While the interface captures the mouse,
your terminal's own click-and-drag text selection stops working — which is the
whole reason it can be switched off. Press `m` to toggle it for one session
without editing the file.

`ui.ascii` swaps the box-drawing borders and the status glyphs for plain ASCII,
for a terminal or font that draws them as boxes of question marks. There is no
reliable way to detect that from inside a program, so it is a setting rather
than a guess.

`ui.color` set to `never` turns off the system hues — the blue, yellow, green
and magenta of the spine, and the red of a failure — while keeping bold, faint
and the reverse-video cursor, which carry the same meaning without color.
Setting the `NO_COLOR` environment variable to any value does the same. The
glyphs already say by shape what the colors say by hue, so nothing is lost.

`ui.notify`, off unless you turn it on, rings the terminal when CI finishes —
passes or fails — so you can open a pull request, switch to something else, and
be told rather than checking back. On a terminal that understands the OSC 9
notification sequence it also raises a desktop notification; the rest just ring.
While it is on and no `timing.ci_interval` is set, CI is polled every three
minutes rather than every twenty seconds, since a notification you stepped away
for is not in a hurry.

A setting left out of the file keeps its default, so a configuration written
before these existed behaves exactly as it did.

## Branch names

When you branch for an issue, workflow proposes a name. By default it is
`fix/PROJ-412-short-summary` for a bug and `feat/…` for anything else — the
Conventional Commit prefix, the issue key as Jira writes it, then a slug of the
summary. You can shape it to your team's convention:

```json
{
  "branch": {
    "template": "{prefix}/{key}-{slug}",
    "prefixes": { "bug": "bugfix", "story": "feature" },
    "default_prefix": "feat"
  }
}
```

- `template` places the three parts. `{prefix}` is chosen from the type, `{key}`
  is the issue key, and `{slug}` is the summary. An empty slug — a summary with
  no letters — leaves a clean name rather than a trailing hyphen.
- `template` **must contain `{key}`**. Everything the interface does with a
  branch — resuming its issue, linking its pull request, choosing a commit type —
  reads the key back out of the name, so a template that hid it is refused when
  the file loads.
- `prefixes` maps an issue type to its prefix. What you write here is the whole
  rule: a type you do not list takes `default_prefix`, not the built-in `fix`.

You can still edit the proposed name before creating the branch; this only
changes where it starts.

## Keeping the tokens safe

`.workflow.json` holds live credentials:

- `workflow config init` writes it mode `0600` — readable only by you — and
  `--force` leaves it that way whatever mode the file it replaces had.
- `workflow doctor` fails while anyone but you can read or write the file, and
  names the `chmod 600` that puts it right.
- It is listed in the repository's `.gitignore`.
- `workflow config show` masks every credential — Jira, Slack, the webhook URL
  and the forge token — printing only the last four characters so you can tell
  two apart.

**`slack.webhook_url` is masked like a token, because it is one.** Anyone holding
that URL can post to your channel; it is a password that happens to look like an
address. `workflow doctor` never prints it at all — not even masked — because
doctor's output is what the bug report template asks people to paste.

Nothing in workflow prints a credential in full.
