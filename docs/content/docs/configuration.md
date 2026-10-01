---
title: "Configuration"
weight: 20
---

# Configuration

workflow reads a single JSON file, `.workflow.json`.

```json
{
  "version": "1",
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
  },
  "taskwarrior": { "program": "", "disabled": false }
}
```

Run `workflow config init` and it asks for the Jira address and token and
checks them, then asks for a Slack incoming webhook, which it saves unchecked,
since a webhook cannot be checked without posting; leave it blank to post with
your Slack user token, which `workflow slack login` sets up afterwards. It warns if the file would
not be ignored by git, and writes what passed — nothing is echoed as you type a
token or the webhook. Add `--global`
to write it to your home directory, or `--template` to write a blank file to
fill in by hand instead of being asked.

## Where it looks, and what wins

workflow looks for `.workflow.json` in the current directory first, then in each
directory above it up to the repository root — the directory holding `.git` —
and then in your home directory. Outside a repository the walk goes on up to
the top of the filesystem, so a subdirectory of a repository reads the
repository's own file.

**A file found in or above the current directory is layered over the one in
your home directory.** Your home file holds what every repository shares — the
Jira address and token, the messaging service — and a repository's file holds
only what that repository changes. A repository's file of just

```json
{ "jira": { "project": "OSS" } }
```

keeps everything else from home. Setting by setting, the repository's file
wins: an object in both is merged key by key, and anything else it sets — a
list such as `jira.views`, a string, an explicit `false` — replaces the home
file's. Each file's keys are checked on their own, so a misspelled one names
its file, and the two are then checked together, since two valid files can
still disagree (a webhook at home and a Slack user token in the repository,
say).

**A credential is inherited only by the address it was written for.** A
repository's file is part of a working tree you may have cloned from anyone, so
one that points a section somewhere else does not take your home file's
credentials with it:

| Section | Address | Credentials not inherited once the address changes |
| --- | --- | --- |
| `jira` | `base_url` | `token`, `token_command`, `token_env`, `headers` |
| `forge` | `host` | `token` |
| `messaging` | `kind` | `webhook_url` and the Slack user token (`client_id`, `client_secret`, `refresh_token`, `access_token`, `expires_at`) |

A repository's file that leaves the address alone, or repeats the home file's,
still inherits them, and one that moves it may set credentials of its own. A
Slack user token is only ever sent to Slack, so a repository naming another
`channel` keeps it; a webhook URL is its own address.

A save writes the repository's file when there is one: the web's Settings
writes only what differs from your home file, so a token inherited from home is
never copied into a file in a working tree, and a later change at home still
reaches the repository. `workflow config init` in a repository, over a home
file, starts from the home file's settings — a question left blank keeps the
home file's answer — and `--template` writes an empty layer rather than blanks
that would hide them. `--global` writes the home file.

`workflow doctor` names every file in effect, and `workflow config show` names
them on stderr, so there is never a question about which were read.

## Fields

| Field | Required | Description |
| --- | --- | --- |
| `version` | no | The file format's version, which `workflow config init` writes first. `"1"` is the only one this build reads; empty (the default) means the current one, and any other value is refused when the file loads rather than half-read against a format it was not written for. |
| `jira.base_url` | for Jira as the tracker | Root URL of your Jira instance, e.g. `https://jira.example.com`. Leave it empty to use the forge's issues instead: the Issues pane then lists the open issues assigned to you on your forge. |
| `jira.token` | one of these three, with `jira.base_url` | Personal access token. |
| `jira.token_command` | one of these three, with `jira.base_url` | A program that prints the token, e.g. `pass show jira/token`. See below. |
| `jira.token_env` | one of these three, with `jira.base_url` | An environment variable that holds the token. |
| `jira.user` | no | Only for instances requiring HTTP Basic. See below. |
| `jira.views` | no | Named issue lists (`name` + `jql`) the pane moves between with `v`. Empty keeps the one built-in list. See below. |
| `jira.headers` | no | Extra HTTP headers sent with every Jira request, for a Jira reached through an SSO proxy that checks one. Values are masked wherever the configuration is shown. See below. |
| `jira.project` | no | The Jira project key, e.g. `PROJ`. When set, only a branch naming a key in that project is read as an issue, so a name like `fix/UTF-8-decoding` is not mistaken for one. Empty (the default) falls back to a looser guard that rejects common technical tokens (`UTF`, `SHA`, `CVE`) by shape alone. |
| `jira.markdown_comments` | no | Write comments in Markdown and have them posted as Jira's wiki markup. Off by default, so a comment already in wiki markup is posted unchanged. |
| `jira.review_status` | no | The status an issue moves to once its pull request is open, e.g. `In Review`. Opening a pull request offers the move to this status by name. Empty (the default) makes no offer. |
| `issues.forge` | no | List the issues assigned to you on this repository's own GitHub or GitLab project beside Jira's, at the head of the first issue list. Set it in the repository's file, over your home file's default. With no `jira.base_url` the forge's issues are the whole list whatever it says. Defaults to `false`. |
| `messaging.kind` | no | Service to post to: `slack` (the default when empty), `teams`, `discord`, or a plain `webhook`, spelled in lowercase; any other value is refused when the file loads. It decides the message body and link markup. |
| `messaging.client_id` | for a Slack user token | Your Slack app's client ID, which its rotating user token is refreshed with. Not a secret; `workflow slack login` writes it. Slack only, and never together with `messaging.webhook_url`. |
| `messaging.client_secret`, `messaging.refresh_token`, `messaging.access_token`, `messaging.expires_at` | written by workflow | The user token's rotating credentials, when this file keeps them — on Linux and Windows, or on macOS once they are here — rather than the keychain. workflow rewrites the last three on every refresh. All but `expires_at` are **credentials**. |
| `messaging.webhook_url` | for a webhook | Incoming webhook URL. **This is a credential**, not just an address. The only transport for Teams, Discord and a plain webhook; for Slack, set it or a user token, never both. |
| `messaging.channel` | with a Slack user token | Channel to post in, e.g. `#dev-workflow`. You must be in it. A webhook carries its own. |
| `messaging.channels` | no | Further channels a user-token announcement can go to, for a change that concerns another team, e.g. `["#platform"]`. The announcement preview offers them after `messaging.channel`, which is always a choice; you must be in each. A webhook carries its own channel and offers none. |
| `messaging.announcement` | no | Slack template for the review message, from `{author}`, `{noun}`, `{title}`, `{url}`, `{key}`, `{summary}`, `{issue_url}`. Empty, or any non-Slack kind, uses the built-in message. |
| `forge.kind` | on-prem only | `github` or `gitlab`, for a host whose name says neither. |
| `forge.host` | with `forge.kind` | The host `forge.kind` and `forge.token` are for, e.g. `git.example.com`. |
| `forge.token` | **no** | GitHub or GitLab token. Usually leave it empty — see below. |
| `forge.cli` | no | Route forge API calls through the forge's own command-line tool — `gh` for GitHub, `glab` for GitLab — instead of over HTTP, so the login that tool already holds carries the request. This is what reaches a forge behind an SSO gateway a bare token cannot. Falls back to HTTP when the tool is not installed. Defaults to `false`. |
| `ui.mouse` | no | Capture the mouse, so a click focuses a pane or selects a row. Defaults to `true`. |
| `ui.ascii` | no | Draw borders and glyphs in plain ASCII. Defaults to `false`. |
| `ui.color` | no | `never` turns off the system hues; bold, faint and the cursor stay. Empty (the default) draws them; any other value is refused when the file loads. `NO_COLOR` also turns them off. |
| `ui.notify` | no | Ring the terminal (and raise a desktop notification where it relays one) when CI finishes. Defaults to `false`. |
| `ui.comments_shown` | no | How many of an issue's most recent comments the detail pane draws. Defaults to 5, which `0` also keeps; a negative count is refused when the file loads. |
| `ui.keys` | no | Rebind keys: a map from an action to the single key that triggers it, e.g. `{"commit": "C"}`. The help then shows the new key. See [Rebinding keys](#rebinding-keys) for the actions. |
| `timing.request_timeout` | no | How long each request to a service may take, as a Go duration such as `30s`. Defaults to ten seconds. See [Timing](#timing). |
| `timing.ci_interval` | no | How often CI is asked about while it runs, and how often the `--web` page's stream asks the forge about the branch, as a Go duration such as `1m`. Defaults to twenty seconds. See [Timing](#timing). |
| `branch.template` | no | Shape of a proposed branch name from `{prefix}`, `{key}` and `{slug}`. Must contain `{key}`. Defaults to `{prefix}/{key}-{slug}`. |
| `branch.prefixes` | no | Map from issue type to branch prefix, e.g. `{"bug": "bugfix"}`. The type is matched without regard to case, and this replaces the built-in `{"bug": "fix"}` rather than adding to it. |
| `branch.default_prefix` | no | Prefix for an issue type not named in `branch.prefixes`. Defaults to `feat`. |
| `branch.slug_limit` | no | The longest the summary's slug in a proposed branch name may be, in characters. Defaults to 48, which `0` also keeps; a negative limit is refused when the file loads. |
| `commit.default_scope` | no | Scope the commit composer — and the `--web` commit form — opens with when no kept draft has one and no commit in this repository has used one yet, e.g. `api`; the scope last used wins once there is one. Must be a valid Conventional Commit scope. Empty (the default) opens with no scope. |
| `commit.types` | no | The commit types the composer — and the `--web` commit form — offers and checks a subject against, in the order to offer them, e.g. `["feat", "fix", "chore"]`. Each is a lowercase word; a branch whose prefix is one of them opens the composer on that type. Empty keeps the built-in Conventional Commit types; a type that is not a lowercase word is refused when the file loads. |
| `commit.subject_limit` | no | The longest a commit subject may be, in characters; the composer's ruler counts against it. Defaults to 72, which `0` also keeps; a negative limit is refused when the file loads. |
| `commit.refs_trailer` | no | The label of the trailer that names the issue in a commit body, e.g. `Closes`. Defaults to `Refs`. It is a single word with no colon; anything else is refused when the file loads. |
| `pull_request.title_source` | no | Where a proposed pull request's title comes from: `commit` (the default) takes the branch's oldest commit subject, `issue` the issue's key and summary. Any other value is refused when the file loads. See [Pull requests](#pull-requests). |
| `store.disabled` | no | Keep nothing on disk between sessions. Defaults to `false` — the store remembers a few conveniences, never a secret. See [What is kept between sessions](#what-is-kept-between-sessions). |
| `taskwarrior.program` | no | The Taskwarrior program to run, and the only one tried: a path, such as `/opt/homebrew/bin/task`, or a name looked up on `PATH`. Empty (the default) tries every `task` in an absolute `PATH` directory, in order, and keeps the first that is Taskwarrior 3.5.0 or newer; a Taskwarrior that has never been run, whose taskrc has a malformed line, or that cannot start ends the search there. A value with a line break or a NUL in it is refused when the file loads. A change applies when workflow next starts. See [Taskwarrior](#taskwarrior). |
| `taskwarrior.disabled` | no | Turn the Taskwarrior integration off even where Taskwarrior is installed. Defaults to `false`. A change applies when workflow next starts. |

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
you pick work from elsewhere too — your issues in a sprint, a team filter, or
the unassigned pile — name those lists under `jira.views` and press `v` to move
between them:

```json
{
  "jira": {
    "views": [
      { "name": "My work", "jql": "assignee = currentUser() AND statusCategory != done" },
      { "name": "Sprint board", "jql": "sprint in openSprints() AND (assignee = currentUser() OR assignee is EMPTY) AND statusCategory != done" },
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

Every view is scoped to your issues unless its JQL names the assignee. A view
of `sprint in openSprints() AND statusCategory != done` alone is searched as

```text
(sprint in openSprints() AND statusCategory != done) AND assignee = currentUser()
```

— an `ORDER BY` stays at the end — while the three views above, which each
name `assignee`, are searched as written. So a view of someone else's work, or
of nobody's, says whose: `assignee is EMPTY`,
`assignee in membersOf("my-team")`, or, as the sprint board does, yours and
nobody's at once. The word is looked for anywhere in the query, in any case.
The interface and `workflow --web` scope a view alike, and the web's
`GET /api/views` lists each view with the query it searches.

## Keeping tokens out of the file

So the file need hold no secret, a Jira token can instead come from a program
or an environment variable (a Slack user token has a home of its own, below):

- `token_command` runs a program and reads the token from its output, e.g.
  `pass show jira/token`, `op read "op://vault/jira/token"`, or
  `security find-generic-password -s workflow-jira -w`. The command is split on
  spaces and run directly — no shell — so wrap a pipeline in a script if you need
  one.
- `token_env` reads the token from an environment variable, e.g. `WORKFLOW_JIRA_TOKEN`.

The file's own `token` wins when set, then `token_env`, then `token_command`.
`workflow doctor --online` reports which source each credential came from,
without ever printing the value.

The Jira token is found the first time a command needs Jira, so one that never
reaches it, such as `workflow reviews`, never runs its token command. The
interface and `--web` find it before they start, while a command that asks for a
passphrase on the terminal can still be answered. A command that fails or prints nothing, or a variable that is empty,
is reported as no token where the service was wanted, and the token is looked
for again the next time.

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
| `slack` | rotating user token or incoming webhook | Slack mrkdwn `<url\|text>` |
| `teams` | incoming webhook | Markdown `[text](url)` |
| `discord` | incoming webhook | Markdown `[text](url)` |
| `webhook` | incoming webhook | bare URL, no markup |

`workflow doctor` reports the service and the transport in effect.

### Slack: a user token or a webhook

Set up one or the other. A file that sets up both — `messaging.client_id`, or
any of the user token's secrets, beside `messaging.webhook_url` — is refused when
it loads, so it is always clear which one posts. The Slack bot token is gone:
a file still naming `messaging.token`, `token_command` or `token_env` is refused
with the way to set up a user token instead.

#### User token — posts as you, to the channel you choose

Posts go out as you, through Slack's API, with a token that **rotates**: each
access token (`xoxe.xoxp-…`) lasts twelve hours, and a refresh token
(`xoxe-1-…`), which works once, swaps it for a new pair. workflow refreshes it
for you before it runs out, and keeps each new pair.

1. Create an app at [api.slack.com/apps](https://api.slack.com/apps) in your
   workspace.
2. Under **OAuth & Permissions**, add the `chat:write` **user** token scope,
   and turn on **token rotation**. Slack does not let rotation be turned off
   again. Tagging reviewers and user groups in an announcement is optional and
   needs four more user token scopes: `users:read` and `channels:read` (and
   `groups:read` for a private channel) to match people to the channel, and
   `usergroups:read` to offer user groups. Without them the announcement posts
   untagged and names the scope to add; a token issued before you add them must
   be issued again, by reinstalling the app and running `workflow slack login`.
3. Install the app to the workspace. Copy the **refresh token** it gives, and
   the app's **Client ID** and **Client Secret** from **Basic Information**.
4. Set `messaging.channel` to a channel you are in, then run:

   ```sh
   workflow slack login
   ```

   It asks for the client ID, then the client secret and refresh token without
   echoing them, refreshes the token once to prove them, writes
   `messaging.client_id` into the file, and says whose token it is.

Where the token is kept:

- **macOS**: in the keychain, under `workflow-slack`, so the file holds no
  secret. If the file already holds the token's secrets, it stays there.
- **Linux and Windows**: in the configuration file, which workflow rewrites
  after every refresh — the file is written readable only by you, as `config
  init` writes it.

Every post asks for the token anew and refreshes it when it has less than ten
minutes left, and once more if Slack still calls it expired. Two workflows
running at once — the terminal and `--web`, say — take turns through a lock
file beside the store, so they never spend the same refresh token twice.
`workflow doctor --online` refreshes it if it is due, and says whose it is,
where it is kept and when it expires. Under `--dry-run` it uses the token as
held, and leaves one due a refresh unchecked rather than write a new one.

The web's Settings takes the same three answers. Where the keychain keeps the
token, a save refreshes it once with what you typed — a field left blank keeps
the keychain's — and keeps the new pair there, never in the file; Slack
refusing them is said at once, and nothing is written. Where the file keeps
the token, what you type is written to it, and the next post refreshes with
it, which is when a refusal shows.

**Asking somewhere other than Slack — a hook for tests and proxies.** Every
request that carries the user token goes to `https://slack.com/api`, unless the
`WORKFLOW_SLACK_API` environment variable names another address: a fake Slack
a test runs, or a proxy in front of Slack. It is an environment variable and
never a `.workflow.json` key, so a repository's file cannot send your token
anywhere. It takes an `https://` address, or `http://` only to this machine
(`127.0.0.1`, `::1` or `localhost`); anything else is refused with an error
naming the variable, and nothing is sent. A webhook carries its own address and
is not affected.

#### Incoming webhook — the two-minute option

1. Create an app at [api.slack.com/apps](https://api.slack.com/apps) in your
   workspace.
2. Turn on **Incoming Webhooks**, choose **Add New Webhook to Workspace**, and
   pick the channel it posts to.
3. Copy the URL into `messaging.webhook_url` (with `"kind": "slack"`).

The webhook is bound to the channel you chose, so `messaging.channel` does not
apply and is not required. There is no scope to request and no token to keep
fresh.

The trade-off is that a webhook posts and nothing else: it cannot tell workflow
the message's timestamp, so later events arrive as new messages rather than
replies, and it can never post anywhere but that one channel.

### Teams, Discord or a plain webhook

Create an incoming webhook in the service, set `messaging.kind` to `teams`,
`discord` or `webhook`, and copy the URL into `messaging.webhook_url`. A user
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
--hostname git.example.com`, or `forge.host` beside `forge.token`. An Enterprise
Cloud tenant such as `acme.ghe.com` reads `$GITHUB_TOKEN` as `github.com` does,
but its `forge.token` too needs `forge.host` beside it. When no source has a
token for the host, `workflow doctor --online` says which of these it would have
read.

### What the token needs to be allowed

Every call workflow makes to the forge carries this token, or the login `gh`
or `glab` holds when `forge.cli` routes the call through them, so it needs the
permissions below. Pushing is not one of those calls: `git` pushes with its own
credentials. A token that may only read still shows issues, pull requests or
merge requests, reviews and CI; a write it may not make is refused, and workflow
says which forge refused it, what that forge asks for — GitLab's `api` scope,
or GitHub's `repo` scope or, for a fine-grained token, the permission the table
below names — and the forge's own reason, in the same words in the terminal and
the browser. A token the forge did not accept at all is told apart: it may have
expired or been revoked. `workflow doctor --online` warns of a GitLab token
that can read but not write, when GitLab will say what the token may do; for
some tokens, such as an OAuth token, it will not, and doctor then says nothing.

#### GitHub

A **fine-grained** token needs these repository permissions, on every
repository you work in:

| Permission | Access | For |
| --- | --- | --- |
| Metadata | Read-only | Reading the repository's merge settings; GitHub adds it to every token |
| Pull requests | Read and write | Finding, opening and editing a pull request, requesting reviewers, reading reviews |
| Issues | Read and write | Assignees and labels on a pull request, and reading and closing a GitHub issue |
| Contents | Read and write | Merging a pull request |
| Commit statuses | Read-only | CI reported as commit statuses |
| Checks | Read-only | CI reported as check runs |
| Actions | Read and write | Listing workflow runs, and re-running failed jobs |

The review queue searches only the repositories the token was granted, so a
fine-grained token limited to one repository shows the reviews waiting in that
one alone.

A **classic** token needs the `repo` scope, or only `public_repo` when every
repository is public. `gh auth login` grants `repo` by default, so its token
already has what workflow needs.

#### GitLab

A token needs the **`api`** scope. With `read_api` alone, workflow reads
everything but every write is refused.

The scope is only half of it: GitLab also checks your role in the project.

| Role | For |
| --- | --- |
| Developer | Opening and editing a merge request, and retrying a pipeline |
| Whichever role the target branch's protection allows to merge (Maintainer by default) | Merging a merge request |
| Reporter, or the issue's author or assignee | Closing a GitLab issue |

#### Reviewers and assignees the forge will not take

Reviewers are added best effort on both forges. GitHub turns down the whole
request when it cannot ask one name, such as someone who is not a collaborator,
so workflow asks again one name at a time. GitLab sets reviewers and assignees
by id, and a username it does not know, or one it cannot look up, has none, so
the merge request opens with the reviewers and assignees it found. Either way
the pull request opens, and a note names the people left off; before, an
unknown GitLab reviewer or assignee stopped the merge request from opening.

### CODEOWNERS proposes the reviewers

When a pull request is composed — in the terminal's composer, the web form and
`workflow pr` — its reviewers start as the code owners of the paths the branch
changes. Nothing needs configuring; a repository with no CODEOWNERS proposes
nobody.

- **Which file.** The first that exists on the base branch, read as the forge
  reads it: on GitHub `.github/CODEOWNERS`, then `CODEOWNERS`, then
  `docs/CODEOWNERS`, with GitHub's rules (the last matching line wins); on
  GitLab `CODEOWNERS`, `docs/CODEOWNERS`, then `.gitlab/CODEOWNERS`, with
  GitLab's sections, default owners and `!` exclusions. On GitLab the lines
  before the first header are the section named `codeowners`, and a later
  `[codeowners]` header adds to them. The base is read as
  `origin`'s copy when one has been fetched, so a stale `origin/<base>` gives
  stale owners — `git fetch` brings them up to date.
- **Which patterns.** Each forge's patterns read as that forge reads them. On
  GitLab a pattern matches a whole path: `docs/` and `/docs/` cover
  everything under a `docs` directory, while `docs` and `/docs` name only a
  file called `docs`, and `docs/*` only a directory's direct children. A
  pattern without a leading slash, such as `README.md`, matches at any depth,
  and `*` matches dot files too. GitLab matches with Ruby's `File.fnmatch`, so
  a pattern with a doubled slash matches nothing, and a class reads as Ruby
  reads it: `[a-]` holds a `-`, and `[!]` or `[^]` is any character. A
  pattern written twice in a section keeps only its later line. A GitLab line
  starting with `[` and holding a `]` is a section header, as GitLab reads it,
  so a pattern that starts with a class is written after a slash:
  `/[Dd]ocs/` at the root, or `**/[Dd]ocs/` at any depth.
- **Which paths.** Those the branch changes since it left the base
  (`git diff <base>...HEAD`, a rename counting as both of its paths).
- **Which owners.** `@username` owners are proposed as people and
  `@org/team` owners as teams. Owners written as an email address are
  ignored, since no forge can be asked to review as one, as are GitLab's
  `@@role` owners. You are left out: nobody is asked to review their own pull
  request. On GitLab the owners are every `@name` GitLab finds in the text
  after the pattern, wherever it stands: a `#` there starts no comment, so
  `docs/ @a # @b` is owned by both, and a line whose text names nobody, such
  as `docs/ # todo`, has no owners rather than its section's defaults.
- **Teams.** GitHub is asked for a team as a team reviewer when the team is
  the repository's own organization's; a team of another organization cannot
  be asked there, and is reported as not added. On GitLab a
  `@group/subgroup` owner stands for the group's active direct members with
  the Developer role or above, who are each asked when the merge request
  opens; members inherited from a parent group are not, and neither are
  Guests, Planners or Reporters, who cannot approve. A top-level group is
  written `@group`, just as a user is: a name GitLab knows no user by is
  tried as a group and expanded the same way. For tagging on Slack, GitLab is
  asked the same of each bare owner: one it knows as a group is a team, and
  links to a Slack user group like any other.

A proposal that cannot be read — the diff fails, or the file cannot be read —
proposes nobody rather than holding the pull request back. Either way the
reviewers are only proposed: edit or clear them before opening.

### On-premises forges need `forge.kind` and `forge.host`

workflow reads the forge from your git remote. `github.com`, an Enterprise Cloud
tenant under `ghe.com`, and `gitlab.com` name themselves; `git.example.com` does
not, and a GitHub Enterprise Server looks exactly like a self-managed GitLab from
a remote URL alone — while their APIs live at different paths. Rather than guess
and send a token to the wrong service, say which forge it is, and which host you
mean:

```json
"forge": { "kind": "github", "host": "git.example.com" }
```

`forge.kind` describes `forge.host` and says nothing about any other host, so a
repository whose remote is somewhere else is still a host workflow cannot name.
A `forge.kind` with no `forge.host` is reported as incomplete.

`forge.kind` only fills that gap. On `github.com`, a `ghe.com` tenant or
`gitlab.com` it is ignored, because the remote is the better evidence.

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
and magenta of the spine, the cyan of the started task at its end, and the red
of a failure — while keeping bold, faint and the reverse-video cursor, which
carry the same meaning without color.
Setting the `NO_COLOR` environment variable to any value does the same. The
glyphs already say by shape what the colors say by hue, so nothing is lost.

`ui.notify`, off unless you turn it on, rings the terminal when CI finishes —
passes or fails — so you can open a pull request, switch to something else, and
be told rather than checking back. On a terminal that understands the OSC 9
notification sequence it also raises a desktop notification; the rest just ring.
While it is on, no [`timing.ci_interval`](#timing) is set and no announcement
is waiting for CI to pass, CI is polled every three minutes rather than every
twenty seconds, since a notification you stepped away for is not in a hurry.
An announcement waiting for CI keeps the twenty-second beat, so it goes out
soon after CI passes.

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
rebind on its own. workflow refuses to start, `workflow doctor` reports the
problem, and Settings in `workflow --web` refuses to save it, when a map names
an action that does not exist, moves `jump-to-pane` — its keys are the pane
numbers, `1`–`7`, which no one key can stand in for — or binds two actions
that are live at the same time to one key. The pane numbers work on every pane
and while a command runs, so an action live there cannot take one; `7` joined
them with the Tasks pane, so a map that moved such an action to `7` before then
is refused now.

The actions you can rebind, grouped by where they work, are:

- **Moving around:** `next-pane`, `previous-pane`, `up`, `down`, `scroll-up`,
  `scroll-down`.
- **Issues:** `change-status`, `comment`, `assign`, `log-work`,
  `branch-for-issue`, `filter`, `filter-place`, `switch-view`, `load-more`,
  `open-link`, `copy-link`, `refresh`, `track-issue`.
- **Branch and Commits:** `new-branch`, `switch-task`, `link-issue`, `rebase`,
  `push`, `stage`, `stage-all`, `commit`, `amend`, `fixup`, `run-pre-commit`,
  `set-up-lefthook`.
- **Review and your messaging service** (named for it, Slack by default):
  `open-pull-request`, `checks`, `rerun-checks`, `merge`, `finish-branch`,
  `post`, `people-and-groups`.
- **Reviews:** `sort-reviews`, `filter-reviews`.
- **Tasks:** `start-stop`, `complete-task`, `add-task`, `annotate-task`,
  `modify-task`, `undo-task`, `sync-tasks`.
- **In a composer or preview:** `edit`, `edit-body`, `next-template`,
  `toggle-draft`, `toggle-breaking`, `verbatim`, `next-field`,
  `previous-field`, `cycle-type-left`, `cycle-type-right`, `toggle-option`,
  `worktree` (in the branch creator), `post-when-green` (in the announcement
  preview), `link-to-slack` and `not-on-slack` (in the announcement preview
  and in People and groups), `forget-owner` (in People and groups),
  `show-log` (in the checks list).
- **While a command runs:** `stop`, `run-again`, `full-output`.
- **Everywhere:** `apply`, `close`, `toggle-mouse`, `toggle-help`, `quit`,
  `interrupt`.

A key is named as its terminal name: a letter (`C`), or a combination such as
`ctrl+e` or `shift+tab`.

## Timing

The waits are set for a nearby network and a forge with room in its rate limit.
When yours is slower or tighter, stretch them:

```json
{
  "timing": {
    "request_timeout": "30s",
    "ci_interval": "1m"
  }
}
```

- `request_timeout` bounds each request to a service — ten seconds unless set.
  Raise it for an on-premises host that is slow to answer.
- `ci_interval` is how often CI is asked about while it runs — twenty seconds
  unless set — and at most how often the `--web` page's stream asks the forge
  about the branch. Lengthen it to spend less of the forge's rate limit.

Each is a Go duration: a number and a unit, such as `30s`, `1m` or `1m30s`. A
value that is not a positive duration is refused when the file loads, rather
than quietly falling back to the default.

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
- `slug_limit` caps the slug at that many characters — 48 unless set.

You can still edit the proposed name before creating the branch; this only
changes where it starts.

## Pull requests

When you open a pull request, workflow proposes its title from the branch's
oldest commit subject, which on a branch of Conventional Commits already reads
as one. A team that titles its pull requests after the issue can say so:

```json
{
  "pull_request": { "title_source": "issue" }
}
```

`issue` proposes the issue's key and summary, such as
`PROJ-412: Fix token redaction`, and falls back to the oldest commit when the
branch names no issue or its summary is not known. `commit` is the default, and
any other value is refused when the file loads.

## Taskwarrior

The Tasks pane, and the Tasks section of `workflow --web`, drive
[Taskwarrior](https://taskwarrior.org) 3.5.0 or newer wherever it is
installed; there is nothing to turn on. An older Taskwarrior is reported as
too old rather than driven.

```json
{
  "taskwarrior": { "program": "/opt/homebrew/bin/task", "disabled": false }
}
```

### Finding it

Taskwarrior's program is called `task`, and so is go-task, the Taskfile runner.
So workflow does not take the first `task` on `PATH` at its word: it asks each
`task` on `PATH`, in order, for its `_version`, and keeps the first that answers
as Taskwarrior 3.5.0 or newer. A `PATH` entry that is not an absolute directory,
such as `bin` or `.`, is passed over. Each is asked from the filesystem root,
not the directory workflow runs in, so go-task finds no Taskfile to run: a
`_version` task, or a catch-all one, in your repository's Taskfile never runs.
The one that answers is kept for the session. A Taskwarrior that has never been
run, whose taskrc has a malformed line, or that cannot start at all ends the
search there: it is reported, as below, rather than passed over for a `task`
further down.

`taskwarrior.program` names the one to run instead, and then only it is tried.
Give it a path: a bare name is looked up on `PATH` like any command, so
`"task"` is whichever `task` comes first — go-task, where that is first — not
the Taskwarrior further down. `taskwarrior.disabled` turns the integration off
even where Taskwarrior is installed: the Tasks pane says so, and no mark, block
or offer appears.

workflow finds Taskwarrior once, as it starts, so a change to
`taskwarrior.program` or `taskwarrior.disabled` applies when workflow next
starts. One saved from the web's Settings waits too: until the restart, the
Tasks section says to restart workflow to apply it — never a reason that holds
only for the settings workflow started with — and the rest of the page shows
no task.

`workflow doctor` names the Taskwarrior it found, or why none is usable:

```text
  taskwarrior found — 3.5.0 at /opt/homebrew/bin/task
  taskwarrior not usable — task on PATH is not Taskwarrior (go-task?); set taskwarrior.program, e.g. /opt/homebrew/bin/task
  taskwarrior not usable — installed but never run; run /opt/homebrew/bin/task once
```

A Taskwarrior that has never been run has no taskrc, and will not make one
unless asked at a terminal, so it reads as never run; so does one whose
`TASKRC` names a file that is not there. Run it once in a terminal, by the
path `workflow doctor` names — where go-task comes first on `PATH`, a bare
`task` runs go-task — and answer its question. A taskrc with a line
Taskwarrior cannot read is reported as having a malformed line, without the
line, which may hold a secret. A Taskwarrior that cannot start for another
reason — an `include` it cannot read, a database it cannot open — is reported
in its own words: `workflow doctor` and the Tasks pane show them, and the web's
Tasks section says Taskwarrior could not start and that `workflow doctor` says
why.

### Tasks and issues

A task tracks an issue by `jiraid`, the issue's key, and carries `jiraurl`,
its page — two attributes of Taskwarrior's user-defined kind. bugwarrior's
Jira service writes the same two, so a task it made tracks its issue here too.
workflow defines both on every read and write of your tasks, as command-line
overrides, and never writes your taskrc; only `_version` and `_show`, which
read no task, run without them. For `task jiraid:PROJ-412` to work in your own
shell, add them to it — `workflow doctor` names these four lines while your
taskrc has no `uda.jiraid.type`:

```text
uda.jiraid.type=string
uda.jiraid.label=Jira
uda.jiraurl.type=string
uda.jiraurl.label=Jira URL
```

A task created from an issue — `T` in the Issues pane, or
**Track in Taskwarrior** on the web — carries:

- the issue's key in `jiraid` and its page in `jiraurl`;
- the tag `+jira`;
- the issue's priority as Taskwarrior's: `H` for Highest, High, Critical or
  Blocker, `M` for Medium or Major, `L` for Low, Lowest, Minor or Trivial, and
  none for any other;
- `PROJ-412: <summary>` as its description, after `--`, so nothing in the
  summary is read as an attribute;
- one annotation, the issue's page.

With the forge's issues as the tracker, `jiraid` holds the issue's number and
there is no page to note.

### Reads, writes and contexts

- **Reads never write.** Every read runs with Taskwarrior's hooks off, so
  listing your tasks never fires an on-launch or on-exit hook — one that syncs,
  say — and never changes a task.
- **Writes run hooks.** Adding, starting, stopping, completing, annotating,
  modifying, undoing and syncing run as `task` does in a shell, with hooks as
  your taskrc sets them: a Timewarrior hook starts and stops its timer with the
  task. Taskwarrior's confirmation prompts are off for them, since there is no
  terminal to answer.
- **Waiting tasks are counted.** The Tasks list reads your pending tasks and
  those that wait until a later date, and counts the waiting ones below the
  list rather than listing them.
- **The active context** narrows the Tasks list, as it narrows `task list`,
  and its name is shown beside the pane's title. The tasks that track issues —
  the Issues rows' marks, an issue's Tasks block, and the top row's started
  task when it tracks one — are read whatever the context. A change to a task
  workflow names — start, stop, done, annotate, modify — reaches it even where
  the context hides it; a task added with `a` takes the context's attributes,
  as `task add` does.
- **Sync** is offered only when the taskrc names a sync backend.
- **`--dry-run`** reads Taskwarrior as usual and holds every write back; the
  terminal interface says what it would have sent.

## What is kept between sessions

workflow keeps a little state on disk so it can pick up where you left off — the
commit scope you last used in a repository, in the interface or with `--web`
(which reads it once), which pull requests you have
announced — in the interface or with `workflow announce`, and at which moment,
so neither announces the same one twice unasked — and the last issue list it
saw, so the interface opens on it while the live one loads. It also keeps what
you decided and could not be seen again: whom each code owner is on Slack, or
that they are not, and the Slack user groups each repository offers. It lives
in two small SQLite databases under your platform's data directory:

- macOS — `~/Library/Application Support/workflow`
- Linux — `$XDG_STATE_HOME/workflow`, or `~/.local/state/workflow`
- Windows — `%AppData%\workflow`

`workflow.db` is the cache: the conveniences above, which a session makes
again. `kept.db`, beside it, is what you decided — the people and group
associations — and is never thrown away on its own.

Those associations are what tagging an announcement reads. Whom a code owner
is on Slack is kept per forge host (`github.com`, say), since an owner is the
same person in every repository there: a person links to a Slack user, a
team to a user group, or either is marked not on Slack and never asked again.
The user groups an announcement may tag, and the ones chosen last time, are
kept per repository. Both are kept per Slack workspace too, the one the user
token is for, which workflow asks Slack once a session: a Slack ID means
nothing in another workspace, so a link made in one is neither used nor
replaced in another, and switching back finds it again. That an owner is not
on Slack holds in every workspace. When Slack cannot say which workspace the
token is for, nobody is tagged: the announcement posts untagged and says why.
Links kept before workspaces were — by an earlier build — belong to no
workspace, so each such owner is asked about once more. The terminal's
announcement preview and the web's ask
whom an owner not yet decided is, and the web's Settings and the terminal's
`P` overlay change or forget a decision; `workflow announce` tags only whom
is already decided, and names the rest in its preview.

The store **never holds a secret**, and nothing it is keyed by is one. The
commit scope and what was announced are kept per repository: the origin remote's
host and path, or the repository's root path when there is no remote or it
cannot be parsed. The issue list is kept per Jira instance, by a hash of your
Jira URL, and per view, by the view's JQL query. The store is never keyed by a
credential, nor by the raw remote or Jira URL, so a token embedded in a remote
cannot reach it.

Where the filesystem keeps Unix modes, each database file is `0600` in a `0700`
directory, readable only by you. What the cache holds is disposable: remove it
and the next session simply rebuilds it, and a cache a build with another schema
made is discarded and rebuilt the same way. `kept.db` is never discarded: a
newer build carries what it holds forward, and one from a newer build than
yours is read as empty and left as it is. A `--dry-run` never creates it or changes
what it holds: neither the interface nor `--web` opens it at all, and a command
such as `announce` or `status` opens it read-only, and only when it is already
there. That read may leave SQLite's two owner-only companion files,
`workflow.db-wal` and `workflow.db-shm`, beside the database until the next
session's open removes them.

To see the two files, their sizes and what each holds, and to remove them, run
`workflow db-clean` (or open **Local data** in the web interface's Settings). It
removes the cache once you confirm; `--all` removes `kept.db` too, and with it
every people and group association, which workflow then asks for again. Each
file goes with its `-wal` and `-shm` companions, and a file another program holds
open — another workflow session, on Windows — fails the clean without removing
anything. A file set aside that still could not be removed is listed as
`workflow.db.cleaning` or `kept.db.cleaning`, and the next clean removes it. A
running session simply makes a fresh cache on its next write.

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
