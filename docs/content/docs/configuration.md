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
  "messaging": {
    "kind": "slack",
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

Run `workflow config init` and it asks for the Jira address and token, checks
them, does the same for Slack, warns if the file would not be ignored by git,
and writes what passed — nothing is echoed as you type a token. Add `--global`
to write it to your home directory, or `--template` to write a blank file to
fill in by hand instead of being asked.

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
| `jira.base_url` | for Jira as the tracker | Root URL of your Jira instance, e.g. `https://jira.example.com`. Leave it empty to use the forge's issues instead: the Issues pane then lists the open issues assigned to you on your forge. |
| `jira.token` | one of these three, with `jira.base_url` | Personal access token. |
| `jira.token_command` | one of these three, with `jira.base_url` | A program that prints the token, e.g. `pass show jira/token`. See below. |
| `jira.token_env` | one of these three, with `jira.base_url` | An environment variable that holds the token. |
| `jira.user` | no | Only for instances requiring HTTP Basic. See below. |
| `jira.views` | no | Named issue lists (`name` + `jql`) the pane moves between with `v`. Empty keeps the one built-in list. See below. |
| `jira.headers` | no | Extra HTTP headers sent with every Jira request, for a Jira reached through an SSO proxy that checks one. Values are masked wherever the configuration is shown. See below. |
| `jira.markdown_comments` | no | Write comments in Markdown and have them posted as Jira's wiki markup. Off by default, so a comment already in wiki markup is posted unchanged. |
| `messaging.kind` | no | Service to post to: `slack` (the default when empty), `teams`, `discord`, or a plain `webhook`, spelled in lowercase; any other value is refused when the file loads. It decides the message body and link markup. |
| `messaging.token` | for a Slack bot | Bot token; starts with `xoxb-`. Or use `messaging.token_command` / `messaging.token_env`. Ignored by the webhook-only kinds. |
| `messaging.token_command` | for a Slack bot | A program that prints the bot token. |
| `messaging.token_env` | for a Slack bot | An environment variable that holds the bot token. |
| `messaging.webhook_url` | for a webhook | Incoming webhook URL. **This is a credential**, not just an address. The only transport for Teams, Discord and a plain webhook. |
| `messaging.channel` | only with a Slack bot | Channel to post in, e.g. `#dev-workflow`. A webhook carries its own. |
| `messaging.announcement` | no | Slack template for the review message, from `{author}`, `{noun}`, `{title}`, `{url}`, `{key}`, `{summary}`, `{issue_url}`. Empty, or any non-Slack kind, uses the built-in message. |
| `forge.kind` | on-prem only | `github` or `gitlab`, for a host whose name says neither. |
| `forge.host` | with `forge.kind` | The host `forge.kind` and `forge.token` are for, e.g. `git.example.com`. |
| `forge.token` | **no** | GitHub or GitLab token. Usually leave it empty — see below. |
| `ui.mouse` | no | Capture the mouse, so a click focuses a pane or selects a row. Defaults to `true`. |
| `ui.ascii` | no | Draw borders and glyphs in plain ASCII. Defaults to `false`. |
| `ui.color` | no | `never` turns off the system hues; bold, faint and the cursor stay. Empty (the default) draws them; any other value is refused when the file loads. `NO_COLOR` also turns them off. |
| `ui.notify` | no | Ring the terminal (and raise a desktop notification where it relays one) when CI finishes. Defaults to `false`. |
| `ui.keys` | no | Rebind keys: a map from an action to the single key that triggers it, e.g. `{"commit": "C"}`. The help then shows the new key. See [Rebinding keys](#rebinding-keys) for the actions. |
| `branch.template` | no | Shape of a proposed branch name from `{prefix}`, `{key}` and `{slug}`. Must contain `{key}`. Defaults to `{prefix}/{key}-{slug}`. |
| `branch.prefixes` | no | Map from issue type to branch prefix, e.g. `{"bug": "bugfix"}`. The type is matched without regard to case, and this replaces the built-in `{"bug": "fix"}` rather than adding to it. |
| `branch.default_prefix` | no | Prefix for an issue type not named in `branch.prefixes`. Defaults to `feat`. |
| `commit.default_scope` | no | Scope the commit composer — and the `--web` commit form — opens with when no kept draft has one and no commit in this repository has used one yet, e.g. `api`; the scope last used wins once there is one. Must be a valid Conventional Commit scope. Empty (the default) opens with no scope. |
| `store.disabled` | no | Keep nothing on disk between sessions. Defaults to `false` — the store remembers a few conveniences, never a secret. See [What is kept between sessions](#what-is-kept-between-sessions). |

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

## Jira behind single sign-on (Azure AD / Office 365, and others)

On-premises Jira is often reached through a single sign-on gateway. Whether
workflow needs anything special depends on where that gateway sits.

**First, find out whether the REST API accepts a token directly.** Create a
personal access token (avatar → **Profile** → **Personal Access Tokens**; you
reach this once by signing in through the usual browser SSO), then:

```sh
curl -sik -H "Authorization: Bearer <token>" https://your-jira/rest/api/2/myself
```

- **`200` with your user as JSON** — the API takes the token directly; SSO only
  guards the web UI. Put the token in `jira.token` and you are done. This is the
  common case for a SAML SSO plugin (including Azure AD / Entra ID federation).
- **A redirect to `login.microsoftonline.com` or an HTML login page** — the API
  itself is behind the gateway (for Azure AD, an Application Proxy, which is what
  the *MyApps* portal publishes). A token alone will not pass it; read on.

**A gateway that mints a bearer token (Azure AD Application Proxy, OIDC).** Use
`jira.token_command` to fetch a fresh gateway token each run — for Azure AD, the
Azure CLI does this:

```json
{
  "jira": {
    "base_url": "https://jira.example.com",
    "token_command": "az account get-access-token --resource api://<app-id> --query accessToken -o tsv"
  }
}
```

Run `az login` once; the token is sent as `Authorization: Bearer` like any other.

**A gateway that checks a header of its own (Cloudflare Access, and similar).**
Add the headers it wants under `jira.headers`; they are sent with every request,
alongside your Jira token, and their values are masked wherever the configuration
is shown (a gateway secret is a credential):

```json
{
  "jira": {
    "token": "<jira PAT>",
    "headers": {
      "CF-Access-Client-Id": "<client id>",
      "CF-Access-Client-Secret": "<client secret>"
    }
  }
}
```

A Jira token is still required — `jira.headers` adds the gateway's headers on top
of it, rather than replacing your Jira credential.

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

### The operating system's keychain

The keychain is where a token belongs, and a `token_command` reaches it without
this program linking anything. On macOS, `workflow config init` offers to do the
whole thing for you: it saves the token with `security` and writes the reading
command into the file, so the file holds a `token_command` and never the token.

On Linux, store the token once and point `token_command` at it by hand:

```sh
printf %s '<your token>' | secret-tool store --label='workflow jira' service workflow-jira
# then set, in .workflow.json:
#   "jira": { "token_command": "secret-tool lookup service workflow-jira" }
```

## Messaging: Slack, Teams, Discord or a plain webhook

`messaging.kind` picks the service. Empty is read as `slack`, so a file written
before this block was named `messaging` — it was `slack` then — needs `slack`
renamed to `messaging` and a `"kind": "slack"` added; `workflow doctor` names
the rename if you forget. Slack alone has two transports; the others post over an
incoming webhook.

| Kind | Transport | Link markup |
| --- | --- | --- |
| `slack` | bot token or incoming webhook | Slack mrkdwn `<url\|text>` |
| `teams` | incoming webhook | Markdown `[text](url)` |
| `discord` | incoming webhook | Markdown `[text](url)` |
| `webhook` | incoming webhook | bare URL, no markup |

`workflow doctor` reports the service and the transport in effect.

### Slack: a webhook or a bot token

Set either one. If you set both, the bot token is used — it is the more capable
transport, and a configuration that has both is not an error.

#### Incoming webhook — the two-minute option

1. Create an app at [api.slack.com/apps](https://api.slack.com/apps) in your
   workspace.
2. Turn on **Incoming Webhooks**, choose **Add New Webhook to Workspace**, and
   pick the channel it posts to.
3. Copy the URL into `messaging.webhook_url` (with `"kind": "slack"`).

The webhook is bound to the channel you chose, so `messaging.channel` does not
apply and is not required. There is no app review, no scope to request and no bot
to invite.

The trade-off is that a webhook posts and nothing else: it cannot tell workflow
the message's timestamp, so later events arrive as new messages rather than
replies, and it can never post anywhere but that one channel.

#### Bot token — choose the channel at runtime

1. Create an app at [api.slack.com/apps](https://api.slack.com/apps) in your
   workspace.
2. Under **OAuth & Permissions**, add the `chat:write` bot token scope.
3. Install the app to the workspace and copy the **Bot User OAuth Token** — it
   starts with `xoxb-` — into `messaging.token`.
4. Set `messaging.channel`, and invite the bot to that channel. Without the
   invite it cannot post there.

### Teams, Discord or a plain webhook

Create an incoming webhook in the service, set `messaging.kind` to `teams`,
`discord` or `webhook`, and copy the URL into `messaging.webhook_url`. A bot
token and channel do not apply — the webhook carries its own destination. The
notifier renders each message in the service's own markup: Markdown links for
Teams and Discord, and a bare URL for a plain webhook.

### Your team's own words (Slack)

By default the review message reads `jacob opened a pull request: <link>` and,
on the next line, the linked issue. `messaging.announcement` shapes it to a house
style — an emoji, a reviewers line, a group to mention. It is Slack mrkdwn, so it
applies to the Slack kind only; the other services always use the built-in text:

```json
{
  "messaging": {
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

## Rebinding keys

`ui.keys` maps an action to the one key that should trigger it. The interface
binds that key instead of the default, and the help — press `?` — shows the new
key, so what you changed and what the screen tells you never drift apart. An
action you leave out keeps its default.

```json
{
  "ui": {
    "keys": { "commit": "C", "comment": "ctrl+e" }
  }
}
```

A key can mean different things in different places — `c` comments on the Issues
pane and commits on the Commits pane — so each meaning is a separate action you
rebind on its own. workflow refuses to start, and `workflow doctor` reports the
problem, when a map names an action that does not exist or binds two actions that
are live at the same time to one key.

The actions, grouped by where they work, are:

- **Moving around:** `next-pane`, `previous-pane`, `jump-to-pane`, `up`, `down`,
  `scroll-up`, `scroll-down`.
- **Issues:** `change-status`, `comment`, `assign`, `log-work`,
  `branch-for-issue`, `filter`, `switch-view`, `load-more`, `open-link`,
  `copy-link`, `refresh`.
- **Branch and Commits:** `new-branch`, `switch-task`, `rebase`, `push`,
  `stage`, `stage-all`, `commit`, `amend`, `fixup`, `run-pre-commit`,
  `set-up-lefthook`.
- **Review and your messaging service** (named for it, Slack by default):
  `open-pull-request`, `checks`, `rerun-checks`, `merge`, `finish-branch`,
  `post`.
- **In a composer or preview:** `edit`, `edit-body`, `next-template`,
  `toggle-draft`, `toggle-breaking`, `verbatim`, `next-field`,
  `previous-field`, `cycle-type-left`, `cycle-type-right`, `toggle-option`,
  `worktree` (in the branch creator), `post-when-green` (in the announcement
  preview).
- **While a command runs:** `stop`, `run-again`, `full-output`.
- **Everywhere:** `apply`, `close`, `toggle-mouse`, `toggle-help`, `quit`,
  `interrupt`.

A key is named as its terminal name: a letter (`C`), or a combination such as
`ctrl+e` or `shift+tab`.

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

## What is kept between sessions

workflow keeps a little state on disk so it can pick up where you left off — the
commit scope you last used in a repository, in the interface or with `--web`
(which reads it once and never under `--dry-run`), which pull requests you have
announced — in the interface or with `workflow announce`, and at which moment,
so neither announces the same one twice unasked — and the last issue list it
saw, so the interface opens on it while the live one loads. It lives in a small
SQLite database under your platform's data directory:

- macOS — `~/Library/Application Support/workflow`
- Linux — `$XDG_STATE_HOME/workflow`, or `~/.local/state/workflow`
- Windows — `%AppData%\workflow`

The store **never holds a secret**. It is keyed only by a repository's host and
path and by a hash of your Jira URL — never by a credential, and never by the raw
URL — so a token embedded in a remote cannot reach it. Where the filesystem
keeps Unix modes, the database file is `0600` in a `0700` directory, readable
only by you. What it holds is disposable: delete it and the next session simply
rebuilds it.

The store is on by default. Set `store.disabled` to keep nothing on disk; with it
set, workflow behaves exactly as it did before the store existed, working
everything out afresh each time:

```json
{
  "store": { "disabled": true }
}
```

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

**`messaging.webhook_url` is masked like a token, because it is one.** Anyone holding
that URL can post to your channel; it is a password that happens to look like an
address. `workflow doctor` never prints it at all — not even masked — because
doctor's output is what the bug report template asks people to paste.

Nothing in workflow prints a credential in full.
