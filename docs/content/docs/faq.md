---
title: "Questions and troubleshooting"
weight: 23
---

# Questions and troubleshooting

Frequently asked questions (FAQ), and what to do when something does not work,
grouped by the part of workflow they concern. When something fails, start with
[Something does not work. Where do I
start?](#something-does-not-work-where-do-i-start). Each answer links the page
that holds the detail.

[When something fails](#when-something-fails) ·
[Installing](#installing) ·
[First run and the configuration file](#first-run-and-the-configuration-file) ·
[When the configuration file is refused](#when-the-configuration-file-is-refused) ·
[Credentials and the keychain](#credentials-and-the-keychain) ·
[Jira](#jira) ·
[Branches, commits and hooks](#branches-commits-and-hooks) ·
[Git forge (GitHub and GitLab)](#git-forge-github-and-gitlab) ·
[Slack and other chat services](#slack-and-other-chat-services) ·
[Announcements](#announcements) ·
[The terminal interface](#the-terminal-interface) ·
[The web interface](#the-web-interface) ·
[Taskwarrior](#taskwarrior) ·
[Scripting and automation](#scripting-and-automation) ·
[Local data and privacy](#local-data-and-privacy)

## When something fails

### Something does not work. Where do I start?

Run `workflow doctor`. It makes no network calls. It reports the version, the
repository and its forge, the tools it finds (git, lefthook, `gh`, `glab`,
Taskwarrior), where the store keeps its files, the configuration files in
effect, and every required field that is empty or invalid. It ends with
`Credentials were not checked. Add --online to ask Jira, your forge and Slack; a
webhook is left unchecked.` Then run `workflow doctor --online` to ask each
service whether its credential works and where each one came from. Its output
masks every credential and never prints a webhook URL, so it is safe to paste
into a bug report; [What should I attach to a bug
report](#what-should-i-attach-to-a-bug-report-and-is---request-log-safe-to-share)
says what else to add. See [workflow
doctor]({{< relref "/docs/reference/workflow_doctor" >}}).

### What should I attach to a bug report, and is `--request-log` safe to share?

Run `workflow --request-log requests.log doctor --online` (or whatever command
failed) and attach the file. Each line is an outline of one request: the time,
the service, the method, the path, the status and how long it took, as in
`2026-09-22T18:04:11Z jira  GET  /rest/api/2/myself 200 184ms`. It never records
a header, a body, a query string or a host. A messaging webhook's path is itself
the credential, so it is redacted. The file is created readable only by you
(`0600`), and it is appended to, never truncated. The old name `--log` is
refused with exit `2` and the message
`the flag was renamed; it is --request-log now`. See [Scripting: A log for a bug
report]({{< relref "/docs/scripting#a-log-for-a-bug-report---request-log" >}}).

### A request fails with "the server took too long to answer". How do I give it longer?

Each request to Jira, the forge or the messaging service may take ten seconds.
On the command line, one that takes longer fails with
`the server took too long to answer (waited 10s)` and exit status 5; the
terminal interface says `the service was too slow`. For a host that is slow to
answer, raise `timing.request_timeout`, as in
`"timing": { "request_timeout": "30s" }`. It also bounds the calls `forge.cli`
hands to `gh` or `glab`, and how long quitting the terminal waits for writes
still in flight. A service that is limiting requests says
`rate limited; wait and try again` instead, also with status 5. See
[Configuration: Timing]({{< relref "/docs/configuration#timing" >}}).

### A request fails with "could not reach", or names a certificate. What do I check?

When workflow cannot connect, the command line says
`could not reach the server at …` for Jira, and `could not reach the forge`,
`could not reach the messaging service` or
`could not reach Slack to refresh the user token` for the others; the cause
follows, and the command exits 5. The terminal interface says
`Jira did not answer`, `could not reach the forge` or
`could not reach messaging`, and the web says
`the service could not be reached`. Check the network, a VPN, or `HTTPS_PROXY`,
which workflow's requests go through when it is set, first. A cause ending
`x509: certificate signed by unknown authority` means your
company signs its own certificates.

workflow has no setting that skips certificate checks. The requests it sends
itself go through Go's standard HTTPS transport, which trusts what your
operating system trusts, so add your company's root certificate to the system's
trust store. On Linux, Go also reads `SSL_CERT_FILE` or `SSL_CERT_DIR` if you
would rather name a bundle for workflow alone. git's pushes, and forge calls
routed through `gh`
or `glab` with `forge.cli`, use those tools' own trust settings. `jira.base_url`
must be `https` in any case; plain `http` is accepted only for `localhost` or a
loopback address. See [Configuration:
Fields]({{< relref "/docs/configuration#fields" >}}).

## Installing

### Which platforms is there a release binary for, and does workflow run on Windows?

Each [release](https://github.com/jacob-delgado/workflow/releases/latest) lists
a binary for every platform it is built for, named `workflow_<os>_<arch>` (with
`.exe` on Windows), and `brew install jacob-delgado/tap/workflow` says so on a
platform no release is built for; there, `go install
github.com/jacob-delgado/workflow/cmd/workflow@latest` builds it with Go alone,
no C toolchain or Node needed. Windows is a secondary target: download its
binary from the release (Homebrew is for macOS and Linux) and rename it
`workflow.exe` in a directory on your `PATH`. CI compiles and vets the Windows
build on every change but never runs it, so a Windows-only bug is worth
reporting. A few things differ there: as on Linux, workflow drives no keychain
([Does workflow use the keychain on Linux or
Windows?](#does-workflow-use-the-keychain-on-linux-or-windows)), so the Slack
user token's credentials stay in the configuration file, and the file-ownership
and permission checks refuse nothing, since Windows keeps no Unix owner or mode
bits. See [Install: From a
release]({{< relref "/docs/install#from-a-release" >}}) and [Install: With
Go]({{< relref "/docs/install#with-go" >}}).

### How do I upgrade, and what might break when I do?

Upgrade the way you installed: `brew upgrade workflow`, `go install
github.com/jacob-delgado/workflow/cmd/workflow@latest`, or download the new
release binary. `workflow --version` prints the version, and so does the first
line of `workflow doctor`. Before 1.0, a change in the minor version (0.12 to
0.13, say) means something you depend on changed: a configuration key, a
command, a flag, an output or an exit status. New features and fixes only bump
the patch. workflow keeps no compatibility shims for old formats, so read the
release's **BREAKING CHANGES** section in the
[changelog](https://github.com/jacob-delgado/workflow/blob/main/CHANGELOG.md)
before you upgrade across a minor version. See [Install: With
Homebrew]({{< relref "/docs/install#with-homebrew" >}}).

## First run and the configuration file

### What is the quickest way to get set up?

Run `workflow config init`. It asks for your Jira address and personal access
token and checks them with Jira (on macOS it then offers to keep the token in
the keychain). Then it asks for a Slack incoming webhook, which it saves
unchecked; leave that blank to post with your user token after
`workflow slack login` ([Should I use a Slack user token or an incoming
webhook?](#should-i-use-a-slack-user-token-or-an-incoming-webhook)). It writes
`.workflow.json` with mode `0600`, at the repository root, or in the current
directory outside one (`--global` writes `~/.workflow.json`). Then run
`workflow doctor` to see what is still missing ([Something does not work. Where
do I start?](#something-does-not-work-where-do-i-start)). You can also just run
`workflow` or `workflow --web` with no file, and each offers to set one up with
the same questions. With no terminal to answer on, `workflow config init` stops
with `no terminal to answer on; pass --template to write a file to edit by hand`
and exits 2, and `workflow config init --template` writes a blank file to fill
in yourself. See [Install: First run]({{< relref "/docs/install#first-run" >}})
and [Using workflow: Setting up]({{< relref "/docs/usage#setting-up" >}}).

### Where does workflow look for `.workflow.json`, and which file wins?

workflow looks in the current directory first, then in each directory above it
up to the repository root (the directory holding `.git`), and then in your home
directory. Outside a repository, it reads only the current directory's file and
your home file. When both a nearer file and your home file exist, the nearer one
is layered over the home one setting by setting: objects merge key by key, and
anything else it sets, such as a list, a string or an explicit `false`, replaces
the home value. So keep your Jira address, token and messaging in
`~/.workflow.json`, and keep only what a repository changes in its own file,
such as `{ "jira": { "project": "OSS" } }`. `workflow doctor` names the files in
effect on its `Configuration:` line (for example
`/repo/.workflow.json over /home/you/.workflow.json`), and
`workflow config show` names them on stderr. See [Configuration: Where it looks,
and what
wins]({{< relref "/docs/configuration#where-it-looks-and-what-wins" >}}).

### `config init` says "configuration file already exists". How do I start over?

`workflow config init` never writes over an existing file, because that file
holds credentials you cannot get back. It stops with `configuration file already
exists: <path>; pass --force to overwrite` and exits 4. Pass `--force` to
replace it. A repository's file that is a link is refused even with `--force`,
because the write would follow the link. By default the file goes at the
repository root, or in the current directory outside a repository; use
`--global` to write `~/.workflow.json` instead. Run in a repository when you
already have a home file, it starts from the home file's settings and writes
only what you change, and `--template` writes an empty layer. See [workflow
config init]({{< relref "/docs/reference/workflow_config_init" >}}).

### Can I try setup, or workflow itself, without changing anything?

Yes, with `--dry-run` (`-n`). `workflow config init --dry-run` asks the same
questions and runs the same Jira check, but writes nothing and stores nothing in
the keychain. It prints the file it would write, credentials masked, on stdout,
and `dry run: would write <path> (mode 0600)` on stderr; like a real run, it
refuses a file that already exists unless you pass `--force`. Bare
`workflow --dry-run` opens the interface with every write held back, and each
action says what it would have done. `workflow doctor --dry-run --online` holds
back the refresh of a Slack user token that is due one, since the refresh
writes. See [Using workflow: Dry run]({{< relref "/docs/usage#dry-run" >}}).

## When the configuration file is refused

### workflow refuses my configuration with "invalid .workflow.json". What is wrong?

The text after `invalid .workflow.json:` names what is wrong. workflow refuses
unknown keys instead of ignoring them, because a misspelled key that loaded
silently would look just like a credential you never set. The error names the
file and the key, as in `/home/you/.workflow.json: invalid .workflow.json: json:
unknown field "tokenn"`. Every command that reads the configuration stops, with
exit status 3. Before 1.0, a breaking release can rename or remove a
configuration key, and the old spelling is not kept. Look the key up in that
release's **BREAKING CHANGES** section of the
[changelog](https://github.com/jacob-delgado/workflow/blob/main/CHANGELOG.md)
and fix the file by hand; `workflow config init --force` also works, but it
replaces the whole file, credentials and all. The `version` field must be empty
or `"1"`; anything else is refused with `unknown configuration version`. A value
a setting cannot take is refused the same way, and its answer sits with that
setting: [the branch
template](#how-do-i-change-the-branch-names-workflow-proposes), [a user token
and a webhook both
set](#should-i-use-a-slack-user-token-or-an-incoming-webhook),
[`jira.views`](#the-issues-pane-shows-only-issues-assigned-to-me-how-do-i-see-a-sprint-a-team-filter-or-unassigned-work),
and [`ui.color` and
`ui.motion`](#the-borders-show-as-boxes-or-question-marks-or-i-want-no-colors-or-animation-what-do-i-set);
a setting only the home file may set has [its own entry in this
section](#my-repositorys-workflowjson-is-refused-with-only-the-home-directorys-file-may-set-it-why).
See [Configuration: Fields]({{< relref "/docs/configuration#fields" >}}).

### Why is my configuration file refused, or flagged by doctor, for its permissions?

These are two separate checks. First, on Linux and macOS, every command refuses
to read a configuration file someone else could have written, with `someone
other than you could have written this configuration file`. That is either one
another user owns (`another user owns it`), or one that others, or a group other
than your own, may write
(`its mode, 0664, lets others write it; run chmod go-w on it`). Fix it with
`chmod go-w <file>`; this refusal exits 1. Second, `workflow doctor` fails while
anyone but you can even *read* the file. It prints `Permissions:` with the mode,
then ``It holds credentials. Make it yours alone with `chmod 600 <file>`.``, and
exits 3. `workflow config init` writes the file with mode `0600`, so this
usually means the file was made or copied by hand. See [Configuration: Keeping
the tokens safe]({{< relref "/docs/configuration#keeping-the-tokens-safe" >}}).

### My repository's `.workflow.json` is refused with "only the home directory's file may set it". Why?

Three settings run a program or read your environment: `jira.token_command`,
`jira.token_env` and `taskwarrior.program`. A repository's file is part of a
working tree you may have cloned from anyone, so only your home file may set
them. Any other file that sets one is refused when it loads, and so is the
current directory's file outside a repository. The error names the file and the
setting, for example `/repo/.workflow.json: invalid .workflow.json:
jira.token_command: only the home directory's file may set it`, and the exit
status is 3. Move the setting to `~/.workflow.json`; the repository's file still
layers over it. See [Configuration: Where it looks, and what
wins]({{< relref "/docs/configuration#where-it-looks-and-what-wins" >}}).

## Credentials and the keychain

### Where can each token come from, and which one wins?

For **Jira**, workflow takes the first of four sources: `jira.token` in the
file, then the OS keychain (`"keychain": true`, macOS only), then the
environment variable `jira.token_env` names, then the output of
`jira.token_command`, which runs directly with no shell. For the **forge**, the
order is the environment (`$GITHUB_TOKEN` or `$GH_TOKEN` for github.com,
`$GITLAB_TOKEN` or `$GLAB_TOKEN` for gitlab.com or the host `$GITLAB_HOST` (or
`$GL_HOST`) names), then `gh auth token` for that host (GitHub only; there is no
`glab` step), then `forge.token`. So if you already use `gh auth login`, there
is nothing to set; if workflow says `no forge token found`, see [Where does
workflow get my GitHub or GitLab
token](#where-does-workflow-get-my-github-or-gitlab-token-when-it-says-no-forge-token-found).
A **Slack** user token is set up by `workflow slack login` ([Where is my Slack
token kept?](#where-is-my-slack-token-kept)); a webhook URL lives in the file.
`workflow doctor --online` reports which source each credential came from,
without printing it. If Jira refuses the token, see [What do "the credential was
not accepted" and "the credential was refused"
mean?](#what-do-the-credential-was-not-accepted-and-the-credential-was-refused-mean).
See [Configuration: Keeping tokens out of the
file]({{< relref "/docs/configuration#keeping-tokens-out-of-the-file" >}}) and
[Configuration: The forge token you probably do not
need]({{< relref "/docs/configuration#the-forge-token-you-probably-do-not-need" >}}).

### A repository's file changes the Jira, forge or messaging address, and now there is no token. Why?

A credential is inherited only by the address it was written for. A repository's
file that changes `jira.base_url` does not inherit the home file's `jira.token`,
`token_command`, `token_env` or `headers`. One that changes `forge.host` does
not inherit `forge.token`, and one that changes `messaging.kind` does not
inherit the webhook or the Slack user token. This keeps a cloned repository from
sending your token to an address of its choosing; with no token, a Jira call
fails with `no Jira token to send`. Keep a token for the new address yourself.
On macOS, `"keychain": true` reads the keychain item kept for the address in
effect, which Settings or `config init` can fill. Settings refuses to write any
other new credential into a repository's file, and a token in your home file is
not inherited at the new address either, so write the token into the
repository's file by hand and keep that file out of git. See [Configuration:
Where it looks, and what
wins]({{< relref "/docs/configuration#where-it-looks-and-what-wins" >}}).

### Does workflow use the keychain on Linux or Windows?

No. workflow drives only the macOS keychain, through the built-in `security`
tool. It keeps the Jira token there in a generic password item named
`workflow-jira <jira.base_url>`, and `config init` offers to store it there. On
Linux and Windows, `config init` makes no keychain offer, and `"keychain": true`
gives no token: Jira calls fail with `workflow drives no keychain on this
system: jira.keychain reads none here`. On Linux, store the token with your own
tool and set `jira.token_command` in `~/.workflow.json`, for example
`secret-tool lookup service workflow-jira`. On macOS, if the keychain is locked,
answer the unlock prompt within 30 seconds, or the read or save fails. The Slack
user token is kept there too, as `workflow-slack` ([Where is my Slack token
kept?](#where-is-my-slack-token-kept)). See [Configuration: The operating
system's
keychain]({{< relref "/docs/configuration#the-operating-systems-keychain" >}}).

## Jira

### Does workflow work with Jira Cloud?

No. workflow targets Jira Data Center and Server (on-premises), through REST API
v2. It searches with `/rest/api/2/search`, which Cloud replaced with
`/search/jql` in 2025, and it names users by username, while Cloud names them by
account id. Point `jira.base_url` at your on-premises instance. If your team has
no Jira, leave `jira.base_url` empty and use your forge's issues instead ([Can I
use workflow without Jira?](#can-i-use-workflow-without-jira)). See
[Configuration: Jira token (on-premises / Data
Center)]({{< relref "/docs/configuration#jira-token-on-premises--data-center" >}}).

### Can I use workflow without Jira?

Yes. Leave `jira.base_url` empty, and your forge's issues become the tracker:
the Issues pane lists the open GitHub or GitLab issues assigned to you on the
repository's project. A branch for one is named by its number, as in
`feat/42-fix-typo`. **Change status** offers only Close, and logging work,
linking the pull request and `jira.views` don't apply.
`workflow doctor --online` then says
`not configured — the forge's issues are the tracker` for Jira. To list forge
issues alongside Jira's, set `"issues": {"forge": true}` in the repository's
`.workflow.json`. See [Using workflow: Pick up an
issue]({{< relref "/docs/usage#pick-up-an-issue" >}}).

### Which Jira token do I need?

You need a personal access token (in Jira, avatar → **Profile** → **Personal
Access Tokens**), set in `jira.token` or supplied by `jira.keychain`,
`jira.token_command` or `jira.token_env`. Leave `jira.user` empty and the token
is sent as a bearer token, which is what Data Center expects; set `jira.user`
only if your instance wants HTTP Basic, and the token is then the password.
`workflow doctor` names the mode on its `Jira` line (`bearer token` or
`basic auth`), and `workflow doctor --online` asks Jira who the token is. See
[Configuration: Jira token (on-premises / Data
Center)]({{< relref "/docs/configuration#jira-token-on-premises--data-center" >}}).

### What do "the credential was not accepted" and "the credential was refused" mean?

`the credential was not accepted` means Jira did not take the token (a 401, or
an answer as the anonymous user): it is wrong, expired or revoked, so make a new
one. `the credential was refused` is a 403: Jira knows the token but will not
let its account do this, so check the account's permissions; when Jira explains
a 403 you see `jira rejected the request:` with Jira's own reason instead. If
you made a new token and it is still not accepted, `workflow doctor --online`
says
which source the one it sent came from, and a `jira.token` in the file wins over
the others ([Where can each token come from, and which one
wins?](#where-can-each-token-come-from-and-which-one-wins)). See [Configuration:
Jira token (on-premises / Data
Center)]({{< relref "/docs/configuration#jira-token-on-premises--data-center" >}}).

### workflow says "no Jira REST API answered at that address". What is wrong?

The address answered 404 without a Jira error message, so `jira.base_url`
reaches a web server but not Jira's REST API. The usual cause is a missing
context path: many on-premises instances live under one, such as
`https://jira.example.com/jira`. The error quotes the address it tried. Open
your Jira in a browser, copy the address up to (not including) `/browse/...`,
put that in `jira.base_url`, and run `workflow doctor --online` again. See
[Configuration: Fields]({{< relref "/docs/configuration#fields" >}}).

### My Jira is behind single sign-on. How do I get through?

workflow never follows a redirect, so your token can't be passed on to a sign-in
page or a plain-http hop. If you see `the server at https://jira.example.com
redirected the request: refused to follow a redirect`, or `the answer was not
JSON but "text/html" — does jira.base_url reach Jira itself rather than a
sign-in page?`, a gateway in front of Jira is answering instead of Jira. First
check whether the API takes a token directly: `curl -sik -H "Authorization:
Bearer <token>" https://your-jira/rest/api/2/myself`. If the gateway issues its
own bearer token, as Azure AD Application Proxy does, have `jira.token_command`
fetch it. If the gateway checks headers of its own, as Cloudflare Access does,
add them under `jira.headers`, next to your Jira token; their values are masked
wherever the configuration is shown. See [Configuration: Jira behind single
sign-on (Azure AD / Office 365, and
others)]({{< relref "/docs/configuration#jira-behind-single-sign-on-azure-ad--office-365-and-others" >}}).

### The Issues pane shows only issues assigned to me. How do I see a sprint, a team filter or unassigned work?

Name more lists under `jira.views`, each with a `name` and a `jql`, and press
`v` to move between them; the first view is the one shown at start. A view whose
JQL never mentions `assignee` is still limited to your own issues, so to see
anyone else's work, say whose in the query: for example `assignee is EMPTY`,
`assignee in membersOf("my-team")`, or
`(assignee = currentUser() OR assignee is EMPTY)`. A view missing its `name` or
`jql` is refused when the file loads. See [Configuration: Issue
views]({{< relref "/docs/configuration#issue-views" >}}).

### Why can't I move an issue to the status I want?

Jira's own workflow decides which statuses an issue can move to from where it
stands, and workflow offers only those: **Change status** (`t`) lists them, and
`workflow transition` matches the name you give against them, ignoring case. A
status not offered is refused, naming the ones that are, as in `no move of
PROJ-7 to "Done" is offered from where it stands; it can move to In Progress`,
so you may need to pass through another status first. A move that asks for
fields is refused on the command line with `moving PROJ-7 to Done asks for
Resolution; change its status with t in the interface or Change status on the
web, whose forms ask for them`. Both exit with status 4. See [Using workflow:
Pick up an issue]({{< relref "/docs/usage#pick-up-an-issue" >}}).

## Branches, commits and hooks

### workflow does not recognize the issue in my branch name. Why?

A Jira key is read only as Jira writes it, in uppercase: a project key of two or
more characters, a hyphen and a number, as in `fix/PROJ-412-login`.
`fix/proj-412-login` names no issue. With `jira.project` set, only keys in that
project count. Without it, common tokens shaped like keys (`UTF`, `SHA`, `CVE`,
`ISO`, `RFC`, `MD`) are skipped, so `fix/UTF-8-decoding` names none. A forge
issue number counts only at the start of the name or of a path segment, as in
`42-fix-typo`. For a branch named some other way, press `i` on the Branch pane
to link it to an issue. The link is kept in git as
`branch.<name>.workflow-issue`. Otherwise, name the issue in the pull request's
title or description. See [Using workflow:
Branch]({{< relref "/docs/usage#branch" >}}).

### How do I change the branch names workflow proposes?

Set `branch.template`, `branch.prefixes`, `branch.default_prefix` and
`branch.slug_limit` in `.workflow.json`. The default is `{prefix}/{key}-{slug}`:
`fix` for a bug, `feat` for anything else, and a slug of the summary capped at
48 characters. The template **must contain `{key}`**, because workflow reads the
issue back out of the name. A template without it is refused when the file
loads, with `invalid branch template: must contain {key}`. `prefixes` replaces
the built-in map rather than adding to it, so a type you leave out takes
`default_prefix`, not `fix`. You can still edit the proposed name before
creating the branch. See [Configuration: Branch
names]({{< relref "/docs/configuration#branch-names" >}}).

### How do I work on two issues at once, or on an issue that already has a branch?

`workflow branch KEY` refuses to recreate a branch it would name the same, exits
`4`, and tells you how to go there, as in ``a branch for this issue already
exists: fix/PROJ-7-login; switch to it with `git switch fix/PROJ-7-login` ``. To
keep your current checkout and open another issue beside it, use a worktree. Run
`workflow branch KEY --worktree` (or press `ctrl+g` in the terminal's Start work
overlay) and it creates a sibling directory named after the repository and the
branch, such as `~/src/api-fix-PROJ-7-login`. With `--worktree`, the last line
of output is that directory alone, so `cd "$(workflow branch PROJ-7 --fetch
--worktree --yes | tail -n 1)"` works. A worktree whose directory you deleted
shows as `(worktree gone)`, and `git worktree prune` frees its branch. See
[Using workflow: Branch]({{< relref "/docs/usage#branch" >}}).

### Does workflow enforce Conventional Commits?

The commit composer (`c` on the Commits pane) builds the subject from its parts,
so what it writes is always a well-formed Conventional Commit. `←`/`→` choose
the type, `tab` moves to the scope and the description, a ruler counts against
the subject limit, and `ctrl+x` marks a breaking change. It also adds a trailer
naming the issue unless the body already has one. The defaults are the built-in
types, a 72-character subject and a `Refs:` trailer. Change them with
`commit.types`, `commit.subject_limit` and `commit.refs_trailer`, and pre-fill a
scope with `commit.default_scope`. A branch whose prefix is one of the types
opens the composer on that type. A commit you make with plain `git commit` is
checked only by your repository's own hooks. See [Using workflow: Stage and
commit]({{< relref "/docs/usage#stage-and-commit" >}}) and the `commit.*` rows
of [Configuration: Fields]({{< relref "/docs/configuration#fields" >}}).

### A git hook failed when I committed. What now?

workflow runs plain `git commit`, so your repository's hooks run exactly as they
would in a terminal, and their output streams into the detail pane. workflow has
no way to skip them. When a hook fails, it lists every `file:line` the hook
reported. Pick one and press `enter` to open your editor there, fix it, then
press `r` to run the commit again. Your commit message is kept. A place that
names no file from the repository's root opens nothing and says so. `o` toggles
the full output, and `esc` leaves and reads the repository again. With lefthook
installed, you can try a hook without committing: press `h` on the Commits pane
and choose it (`pre-commit` comes first). lefthook skips a pre-commit job when
nothing is staged, so stage something first. See [Using workflow: When a hook
fails]({{< relref "/docs/usage#when-a-hook-fails" >}}).

### Why doesn't workflow offer to set up lefthook for my existing git hooks?

It offers only when all three are true: `lefthook` is installed and on your
`PATH`, `.git/hooks` holds a runnable hook that is not lefthook's own, and the
repository has no lefthook configuration yet (a `lefthook.yml`, `.lefthook.yml`
or `.config/lefthook.yml`, in any format lefthook reads). The Commits pane then
says `A hook is not managed by lefthook. Press g to set up lefthook.` In the
offer, `enter` turns plain-command hooks into lefthook jobs, `v` keeps every
hook whole as a script under `.lefthook/`, and `esc` skips it. Either choice
writes `lefthook.yml` and runs `lefthook install`, which keeps the old hooks as
`.git/hooks/*.old`. An existing `lefthook.yml` is never overwritten. If the
install fails, fix the cause and run `lefthook install` yourself. See [Using
workflow: Existing git hooks]({{< relref "/docs/usage#existing-git-hooks" >}}).

## Git forge (GitHub and GitLab)

### Where does workflow get my GitHub or GitLab token, when it says "no forge token found"?

It checks three places, in this order: an environment variable (`$GITHUB_TOKEN`
or `$GH_TOKEN` for github.com, `$GITLAB_TOKEN` or `$GLAB_TOKEN` for gitlab.com),
then `gh auth token` if `gh` is installed and signed in to that host, then
`forge.token`. If you already ran `gh auth login`, there is nothing to set.
Depending on where you meet it, the message reads `` no forge token found: set
$GITHUB_TOKEN, run `gh auth login`, or set forge.token `` (for github.com) or
`no forge token was found; for GitHub set $GITHUB_TOKEN or sign in with gh, for
GitLab set $GITLAB_TOKEN, or set forge.token`. `workflow doctor --online` shows
where the token came from (`token from the environment`,
`token from the forge CLI` or `token from forge.token`), or what it would read
for this repository's host. workflow never reads glab's login: set
`$GITLAB_TOKEN` (with `$GITLAB_HOST`, or `$GL_HOST`, on a self-managed GitLab)
or `forge.token`, or set `forge.cli: true` so requests go through glab itself.
See [Configuration: The forge token you probably do not
need]({{< relref "/docs/configuration#the-forge-token-you-probably-do-not-need" >}}).

### How do I use GitHub Enterprise Server or a self-managed GitLab?

When doctor says the host `is not github.com, a ghe.com tenant or gitlab.com`,
workflow cannot tell from the git remote which forge it is: `git.example.com`
could be either GitHub or GitLab, so tell it:
`"forge": { "kind": "github", "host": "git.example.com" }` (or `"gitlab"`).
`forge.kind` set without `forge.host` is reported as incomplete. Each token is
used only for the host it belongs to. For a GitHub Enterprise Server, set
`GH_HOST=git.example.com` with `$GH_ENTERPRISE_TOKEN`, or run
`gh auth login --hostname git.example.com`, or set `forge.token` next to
`forge.host`. A self-managed GitLab reads `$GITLAB_TOKEN` when `$GITLAB_HOST`
names it. github.com, `*.ghe.com` tenants and gitlab.com need no `forge.kind`;
on those it is ignored. For an internal certificate, see [A request fails with
"could not reach", or names a certificate. What do I
check?](#a-request-fails-with-could-not-reach-or-names-a-certificate-what-do-i-check).
See [Configuration: On-premises forges need forge.kind and
forge.host]({{< relref "/docs/configuration#on-premises-forges-need-forgekind-and-forgehost" >}}).

### What permissions does the forge token need, when opening or merging says the forge "refused this"?

A read-only token still shows issues, pull requests, reviews and CI, but every
write is refused, with a message such as `GitHub refused this: the token may
lack the repo scope or the fine-grained permission this needs, or your role may
not allow it.` followed by the forge's own reason. On GitHub, a classic token
needs `repo` (`gh auth login` grants it). A fine-grained token needs read and
write on Pull requests, Issues, Contents and Actions, plus read on Metadata,
Commit statuses and Checks. On GitLab the token needs the `api` scope, not just
`read_api`, plus a role that allows the action (Developer to open a merge
request). `workflow doctor --online` warns
`can read but not write: it has read_api, not api` when GitLab says what the
token may do. Pushing is never the token's job: git pushes with its own
credentials. See [Configuration: What the token needs to be
allowed]({{< relref "/docs/configuration#what-the-token-needs-to-be-allowed" >}}).

### I work from a fork. Why does the pull request go to the wrong repository, or to my fork?

workflow asks git which remote is the base and doesn't keep a setting of its
own. The base is the remote `gh repo set-default` marked
(`remote.<name>.gh-resolved = base`), else a remote named `upstream`, else
`origin`. So in a fork's clone, add the original repository as `upstream`, or
run `gh repo set-default`. The pull request opens on the base repository, from
your branch on the push remote, and its CI, reviews and issues are read there
too. `workflow doctor` and the Repositories pane show the base next to origin.
See [Using workflow: Working from a
fork]({{< relref "/docs/usage#working-from-a-fork" >}}).

### Why isn't my pull request template used?

workflow looks for templates where the forge does. On GitHub it reads a
`pull_request_template.md` (or `.txt`, any case) in `.github/`, the repository
root or `docs/`, or a `pull_request_template/` directory of them in those
places; on GitLab it reads `.gitlab/merge_request_templates/`, putting `Default`
first. Templates are read from your checkout, so a template only on another
branch isn't seen, and on an on-premises host workflow must know the forge
(`forge.kind` and `forge.host`) to look at all. A file larger than 64 KiB, or a
link pointing outside the repository, is skipped. With several templates, the
first form chooses between them and `ctrl+t` moves to the next; once you edit
the description no template replaces it, and with no template the description
lists the branch's commits. See [Using workflow: Open the pull
request]({{< relref "/docs/usage#open-the-pull-request" >}}).

### Why does CI show `none`?

CI is read for the branch's open pull request, so it shows `none` on the base
branch, on a branch with no pull request yet, and once the pull request has
merged. It also shows `none` when nothing reports a check: no workflows or
pipeline, or, on GitHub, a first-time contributor's run waiting for a
maintainer's approval. If the forge refuses the read, `workflow status` still
says `none` and names the failure on standard error; a fine-grained GitHub token
needs read access to Commit statuses and Checks. GitHub's CI is read at this
clone's branch head, so a fix pushed from another clone shows up once this
branch moves to it. For an announcement waiting on CI, see [Why hasn't my
announcement posted yet?](#why-hasnt-my-announcement-posted-yet); for the review
queue, see [What does workflow reviews
list](#what-does-workflow-reviews-list-and-why-are-some-reviews-missing-or-their-ci-none).
See [Using workflow: Limits]({{< relref "/docs/usage#limits" >}}).

### What does `workflow reviews` list, and why are some reviews missing or their CI `none`?

It lists the open pull or merge requests that ask for your review, across every
repository on the forge your current repository's remote points at,
longest-waiting first; `--sort newest` or `--sort repo` changes the order, and
`--json` prints data. On GitHub it searches
`is:pr is:open review-requested:@me`, so a pull request whose review request has
been met or withdrawn drops out, and a fine-grained token sees only the
repositories it was granted. GitHub's search carries no CI, so that column
always reads `none` there. On GitLab it lists open merge requests whose
reviewers include you, with each one's head pipeline. The web's **Reviews**
section and the terminal's queue show the same list. See [The web interface:
Reviews]({{< relref "/docs/web#reviews" >}}).

## Slack and other chat services

### Should I use a Slack user token or an incoming webhook?

Use a webhook if you want the quickest setup. It takes about two minutes, has no
scopes to grant and no token to keep fresh, but it only ever posts to the one
channel it was made for: `messaging.channel` and `--channel` don't apply, and it
can't tag anyone. A user token posts as you to any channel you're in
(`messaging.channel`, plus any in `messaging.channels`), and it is the only way
to tag code owners and user groups. You can set up one or the other, not both. A
file with both is refused when it loads, with `Slack posts with a user token or
a webhook_url, not both; remove one`. See [Configuration: Slack: a user token or
a
webhook]({{< relref "/docs/configuration#slack-a-user-token-or-a-webhook" >}}).

### How do I post to Teams, Discord or another chat service?

Create an incoming webhook in that service. Then set `messaging.kind` to
`teams`, `discord` or `webhook`, and put the URL in `messaging.webhook_url`. The
webhook carries its own destination, so `messaging.channel` doesn't apply.
Asking for another channel with `--channel` is refused with `it takes effect
only with a Slack user token; a webhook posts to the channel it is bound to`
(exit status 2). Treat the URL like a password: anyone who has it can post
there. `workflow doctor --online` can't check a webhook without posting to it,
so it reports the webhook as unchecked, not as failed. See [Configuration:
Teams, Discord or a plain
webhook]({{< relref "/docs/configuration#teams-discord-or-a-plain-webhook" >}}).

### What does `workflow slack login` ask for, and where do I find each answer?

It asks three questions, in this order:

```text
Slack app client ID (Basic Information):
Slack app client secret (not shown):
Refresh token, xoxe-1-… (not shown):
```

The client ID and secret are on your app's **Basic Information** page. No app
yet? See [I have an app configuration
token](#i-have-an-app-configuration-token-from-your-app-configuration-tokens-can-workflow-use-it)
for the manifest route. The refresh token (`xoxe-1-…`) appears under **OAuth &
Permissions** once the app is installed with token rotation on. workflow
refreshes the token once to check what you typed, keeps the new pair, writes
`messaging.client_id` to the file, and prints `Logged in to Slack as <you> in
<workspace>. The token is kept in … and refreshed before it expires.` It needs a
terminal, and it needs a `.workflow.json` to exist first
(`workflow config init`). See [Configuration: Slack: a user token or a
webhook]({{< relref "/docs/configuration#slack-a-user-token-or-a-webhook" >}}).

### I have an app configuration token (from "Your App Configuration Tokens"). Can workflow use it?

Not to log in or post. An app configuration token only manages apps, through
Slack's App Manifest API, and it lasts twelve hours. The refresh token generated
with it starts `xoxe-1-` too, but it isn't the one `workflow slack login` asks
for either. What the configuration token can do is create the app from a
manifest, in place of creating the app, adding its scopes and turning on token
rotation by hand. That app then gives you the client ID, client secret and
refresh token `workflow slack login` asks for. It acts as you, with no more
right than you have: if your workspace approves apps before they are installed,
installing it is still a request to your administrators. The steps are in
[Configuration: Creating the app from a
manifest]({{< relref "/docs/configuration#creating-the-app-from-a-manifest" >}}).

### `workflow slack login` refuses because of a webhook or `messaging.kind`. What do I do?

A user token can't sit alongside a webhook, and only Slack has user tokens. If
you see `slack posts through messaging.webhook_url here; remove it first, since
Slack posts with a user token or a webhook, not both`, delete
`messaging.webhook_url` from the file and run the login again. If you see
`messaging.kind is not slack; a user token is Slack's alone`, set
`messaging.kind` to `slack` (or leave it empty, which means Slack). If you see
`` no .workflow.json to log in with; run `workflow config init` first ``, create
the file first. All three exit with status 3, the configuration family. See
[Scripting: Exit status]({{< relref "/docs/scripting#exit-status" >}}).

### `workflow slack login` says it answered a "bot" token. What did I do wrong?

`` refreshing gave no Slack user token: it answered a "bot" token `` means the
refresh token you typed belongs to the app's **bot** token, not its user token.
workflow only posts with a user token. Add the `chat:write` scope under **User
Token Scopes** (not Bot Token Scopes), reinstall the app, and give
`workflow slack login` the user refresh token. See [Configuration: Slack: a user
token or a
webhook]({{< relref "/docs/configuration#slack-a-user-token-or-a-webhook" >}}).

### Does the Slack token really expire every twelve hours? Do I have to log in again?

The access token does; you don't log in again. With token rotation on, each
access token (`xoxe.xoxp-…`) lasts twelve hours, and its refresh token
(`xoxe-1-…`) swaps it for a new pair exactly once. Before every post, workflow
checks the token. If it has less than ten minutes left, workflow refreshes it,
and it refreshes once more if Slack still calls it expired. Each new pair is
kept where the first one was. `workflow doctor --online` refreshes the token if
it is due, and says whose it is, where it is kept and when it expires. You only
log in again if the chain breaks ([Posting fails with
invalid_refresh_token](#posting-fails-with-refreshing-the-slack-user-token-was-refused-invalid_refresh_token-how-do-i-recover)).
See [Configuration: Slack: a user token or a
webhook]({{< relref "/docs/configuration#slack-a-user-token-or-a-webhook" >}}).

### Posting fails with `refreshing the Slack user token was refused: invalid_refresh_token`. How do I recover?

A Slack refresh token works only once. If it was spent somewhere else, the copy
workflow holds is dead. That happens when the same `.workflow.json` or keychain
entry is used on two machines, when you tried the refresh token yourself (with
`curl`, say) before logging in, or when a new pair couldn't be saved (`` slack
gave a new token that could not be kept; run `workflow slack login` again ``).
To recover, reinstall the app under **OAuth & Permissions** to get a fresh
refresh token, then run `workflow slack login` again. Don't copy the token's
file or keychain entry to another machine, because each refresh spends the token
the other copy holds. A refusal like this exits with status 3. See
[Configuration: Slack: a user token or a
webhook]({{< relref "/docs/configuration#slack-a-user-token-or-a-webhook" >}}).

### Posting says `another workflow is refreshing the Slack token`. What do I do?

`another workflow is refreshing the Slack token; try again in a minute` means
another workflow on this machine (say, the terminal and `--web` running at once)
held the refresh lock for more than ten seconds. The lock is
`slack-refresh.lock`, next to the store. The operating system releases the lock
when the program holding it exits, so a lock left behind by a crash never blocks
you. Just try again. See [Configuration: Slack: a user token or a
webhook]({{< relref "/docs/configuration#slack-a-user-token-or-a-webhook" >}}).

### Where is my Slack token kept?

On macOS, it is kept in the login keychain as a generic password named
`workflow-slack`, so the configuration file holds no Slack secret. To check it's
there, run `security find-generic-password -s workflow-slack`. On Linux and
Windows, where workflow drives no keychain ([Does workflow use the keychain on
Linux or Windows?](#does-workflow-use-the-keychain-on-linux-or-windows)), it is
kept in the configuration file (your home `.workflow.json` unless another file
already holds it). workflow rewrites `messaging.refresh_token`,
`messaging.access_token` and `messaging.expires_at` after every refresh, and
writes the file readable only by you. If the file on macOS already holds the
token's secrets, they stay in the file. `workflow doctor --online` prints which
place is in use. See [Configuration: Slack: a user token or a
webhook]({{< relref "/docs/configuration#slack-a-user-token-or-a-webhook" >}}).

### What is `WORKFLOW_SLACK_API` for?

It points every Slack request that carries the user token at another address
than `https://slack.com/api`, such as a fake Slack in a test or a proxy in front
of Slack. It is only an environment variable, never a `.workflow.json` key, so a
repository's file can't send your token anywhere. It takes an `https://`
address, or `http://` only to `127.0.0.1`, `::1` or `localhost`. Anything else
is refused with `WORKFLOW_SLACK_API must be an https:// address, or http:// to
this machine (127.0.0.1, ::1 or localhost)`, and nothing is sent. Webhooks
ignore it, since a webhook carries its own address. See [Configuration: Slack: a
user token or a
webhook]({{< relref "/docs/configuration#slack-a-user-token-or-a-webhook" >}}).

## Announcements

### Why hasn't my announcement posted yet?

If you pressed `w` (or the pull request was armed when it opened, or you ran
`announce --when-green` or `pr --announce`), the announcement waits until CI has
**passed**. Passed means every check finished as success, neutral or skipped.
Running checks keep it waiting. A failed check keeps it waiting for a later run
that passes, and the pane says *a check failed; waiting on a fix*. With no
checks at all, it says *no checks reported yet*. After ten minutes with none, it
gives up: the terminal and the web say
`no checks reported on … within 10m; announce it by hand`, and
`workflow announce --when-green` says `no checks were reported on … within 10m;
if the repository has no CI, announce it by hand with workflow announce`.
Announcing now (`enter`, or plain `workflow announce`) is allowed while CI is
running or reports nothing, but never while a check has failed or when CI can't
be read. `x` on the messaging pane stops the wait. A draft is never armed ([How
do I stop workflow from announcing every pull request it
opens?](#how-do-i-stop-workflow-from-announcing-every-pull-request-it-opens)),
and quitting while it waits drops it ([What happens to a change I made if I
close the tab, or quit
workflow?](#what-happens-to-a-change-i-made-if-i-close-the-tab-or-quit-workflow)).
A webhook posts only to the channel it was made for, whatever
`messaging.channel` says ([Should I use a Slack user token or an incoming
webhook?](#should-i-use-a-slack-user-token-or-an-incoming-webhook)). See [Using
workflow: Announce it]({{< relref "/docs/usage#announce-it" >}}).

### `workflow announce` refuses with `a check failed on …`. How do I announce anyway?

You can't while a check has failed. workflow reads the CI again at the moment it
posts, so a stale green can't let a failure through, and a CI it can't read
blocks the post too. For a pull request that's ready for review, the refusal
ends `so it was not announced; workflow announce --when-green announces it once
a run passes`: push a fix, or re-run the checks, and let it wait. For a merged
pull request, there is no CI to wait for, so it says `re-run the failed checks
on the forge, then announce it once they pass`. The refusal exits with status 4.
See [Using workflow: Announce it]({{< relref "/docs/usage#announce-it" >}}).

### How do I stop workflow from announcing every pull request it opens?

Set `"auto_announce": false` under `messaging`. It's on by default: when the
terminal or `--web` opens a pull request that is ready for review, the
announcement is armed to post once CI passes. A draft is never armed; instead
you see *a draft is not announced; announce it once it is ready*. Turning it off
is useful in a repository with no CI. With it off, you can still ask for a wait
yourself with `w`, *Announce when CI passes* on the web,
`workflow announce --when-green` or `workflow pr --announce`. See
[Configuration: Fields]({{< relref "/docs/configuration#fields" >}}).

### Why does the merged announcement show up as a new message instead of a reply in the thread?

workflow posts each announcement as a message of its own, first the
ready-for-review one and then the merged one. It does this with a webhook and
with a user token alike, and it never replies in a thread. That's how it works,
not something wrong with your setup. See [Using workflow: Announce
it]({{< relref "/docs/usage#announce-it" >}}).

### How do I turn on tagging in announcements?

Tagging code owners and user groups needs four optional **user** token scopes on
top of `chat:write`: `users:read`, `channels:read`, `groups:read` and
`usergroups:read`. Without them, the post still goes out, just untagged.
`workflow announce` warns `Not tagging anyone: the Slack token lacks the <scope>
scope; add it to the Slack app, then run workflow slack login.` The terminal's
preview shows `tagging needs the <scope> scope; this posts untagged`. When only
`usergroups:read` is missing, it shows
`tagging groups needs the usergroups:read scope` and still tags the people it
can. A token issued before you added the scopes won't have them: reinstall the
app and run `workflow slack login` again. Only the ready-for-review announcement
tags anyone, and only with a user token. See [Using workflow: Tag the code
owners]({{< relref "/docs/usage#tag-the-code-owners" >}}).

## The terminal interface

### How do I find out what a key does?

Press `?` on any pane. The help lists every key, grouped by where it works:
Moving around, each pane under its own name, In a composer or preview, Writing a
comment, While a command runs, On every pane, and On panes and in overlays. It
shows your `ui.keys` changes, not the defaults. Press `?` again or `esc` to
close it, and `q` there quits. The bottom row names the keys for where you are.
On a narrow terminal it drops first the keys that only move, then the pane's
verbs, and `?` and the way out go last. The full table is in [Using workflow:
Keys]({{< relref "/docs/usage#keys" >}}).

### How do I rebind a key, and why does workflow refuse to start after I did?

Map an action to one key under `ui.keys`, for example
`"ui": { "keys": { "commit": "C", "comment": "ctrl+e" } }`. An action you leave
out keeps its default, and the change reaches the `--web` page too. workflow
refuses to start, and `workflow doctor` reports why, when it cannot honor the
map. The messages include `ui.keys names a key action that does not exist:
"run-pre-commit"` (that action is now `run-hook`),
`ui.keys binds two actions to one key in the same context`, and `ui.keys moves
interrupt onto a key that types or edits text`. Fix the entry it names and start
again. The list of actions and the rules are in [Configuration: Rebinding
keys]({{< relref "/docs/configuration#rebinding-keys" >}}).

### Does workflow work in a small terminal, such as 80x24?

Yes. There is no minimum size; the layout adapts. From 80 columns up, the pane
list (the rail) and the detail sit side by side. Below 80 columns they take
turns: on the Issues pane, `enter` reads the selected issue in full and `esc`
goes back to the list. The detail drops its border once the detail itself is
under 60 columns, which at 80 columns it already is beside the rail. Below 60
columns, a dialog becomes a sheet over everything between the top row and the
bottom row. Below 60 columns or 24 rows, the top row shrinks to each stage's
initial and glyph. See [Using workflow: The
screen]({{< relref "/docs/usage#the-screen" >}}).

### The borders show as boxes or question marks, or I want no colors or animation. What do I set?

Set `"ui": { "ascii": true }` for a font or terminal that cannot draw the
box-drawing borders and glyphs. workflow cannot detect this itself, so it is a
setting. To turn off the colors but keep bold, faint and reverse video, set
`"ui": { "color": "never" }` or set the `NO_COLOR` environment variable to a
non-empty value. The glyphs carry the same meaning by shape. Set
`"ui": { "motion": "reduce" }` to keep the spinner and the progress rules still.
`ui.color` accepts only empty or `never`, and `ui.motion` only empty or
`reduce`. A value such as `"off"` is refused when the file loads, with
`invalid ui setting: color is empty or never: "off"`. See [Configuration: The
interface: mouse, ASCII and
color]({{< relref "/docs/configuration#the-interface-mouse-ascii-and-color" >}}).

### I can't select text with my mouse in the terminal. How do I fix that?

While workflow captures the mouse for clicks and the wheel, your terminal's own
click-and-drag selection stops working. Press `m` to turn mouse capture off for
this session, or set `"ui": { "mouse": false }` to keep it off. The mouse is on
by default. See [Using workflow: Mouse and
ASCII]({{< relref "/docs/usage#mouse-and-ascii" >}}).

### I use a screen reader. Can the forms ask one question at a time?

Yes. Set `"ui": { "accessible": true }`, or set the `ACCESSIBLE` environment
variable to a non-empty value. Forms then take the whole terminal, the way
`$EDITOR` does, and ask one plain question at a time instead of being drawn over
the screen. This covers status changes, the first-run questions, Settings, and
opening a pull request. On the web page, the single-key shortcuts stay off until
you turn on `ui.web_shortcuts`, so a key press never acts on its own; `?` and
the `ctrl+k` palette work either way. See [Configuration: Accessible
forms]({{< relref "/docs/configuration#accessible-forms" >}}) and
[Configuration: Keys on the
web]({{< relref "/docs/configuration#keys-on-the-web" >}}).

### Why does the top row show a stage as not started?

The top row (Issue, Branch, Commits, Review, and your messaging service) is
worked out fresh every time from the repository and the services, never stored.
So it cannot drift from what git and the forge say, and it is worked out the
same way in the terminal, in `workflow status` and on `--web`. The Issue stage
is done once the branch names an issue, by a link you made with `i` or by its
name; a pull request that names the issue does not count here. Until then it
reads in flight while an issue is picked: selected in the terminal's Issues
list, or open on `--web` with no branch yet. The Branch stage is done on any
branch other than the base. Commits is done once the branch has commits, and in
flight before that while the working tree holds changes. Review follows the pull
request and its CI. The only thing remembered is whether the pull request was
already announced, which the store keeps. On the base branch with a clean tree
and no issue picked, Issue, Branch and Commits read as not started. On a branch
whose name names no issue and that has no link, Issue is never done. That is
expected, not an error. See [Using workflow: The
screen]({{< relref "/docs/usage#the-screen" >}}).

### How do I switch to another repository, or keep a list of the ones I use?

Open the Repositories pane (`9`). It lists where you are, the repository's
worktrees and your favorites. `s` switches to the row under the cursor, `g`
opens a picker to browse to any directory (`../web`, `~/src/web`), and `f` adds
or removes a favorite. Every switch asks `Switch to DIR?` first, then opens
workflow again in that directory with its own repository, forge and
configuration files. Favorites are kept in `kept.db` by path alone, so with
`store.disabled` there are none. From a script, `workflow repositories --json`
prints the same list. See [Using workflow:
Repositories]({{< relref "/docs/usage#repositories" >}}).

### Why does the wrong editor open, or my commit message come back empty because the editor returned at once?

workflow picks the editor in git's order: `$GIT_EDITOR`, then `$VISUAL`, then
`$EDITOR`, and finally `vi` (`notepad` on Windows). git's `core.editor` setting
is not read, so set one of those variables. A GUI editor that returns before you
close the file needs to be told to wait, such as `EDITOR="code --wait"`.
Anything below the scissors line (`>8`) is help text and is not kept. See [Using
workflow: Editor]({{< relref "/docs/usage#editor" >}}).

## The web interface

### How do I start the web interface, and what if the port is already in use?

Run `workflow --web` inside a repository and open the address it prints on
stderr: `workflow web: serving http://127.0.0.1:13579/#session=… — press Ctrl+C
to stop`. If another program already holds port 13579, it prints no address. It
fails with an error such as `serving the web API: listen tcp 127.0.0.1:13579:
bind: address already in use`. Pick another port with
`workflow --web --port 7001`. `--port` takes 1 to 65535 and works only with
`--web`; otherwise you get `invalid argument "7001" for "--port" flag: it takes
effect only with --web`. `ctrl+c` stops the server. See [The web interface:
Start it]({{< relref "/docs/web#start-it" >}}).

### The page says "workflow refused this page: it holds no session of the running server". What do I do?

Open the exact address `workflow --web` printed when it started, including the
`#session=…` part. Each run makes a new session, so a bookmark or tab from an
earlier run stops working once the server restarts. The page keeps the session
in this browser for that address and removes it from the address bar, so later
tabs at the same address work. `localhost` in place of `127.0.0.1`, or another
port, is a different origin and does not have the stored session. Treat the
printed address as a password while the server runs. See [The web interface:
Start it]({{< relref "/docs/web#start-it" >}}).

### Can I open the web interface from another machine?

Not directly, and this is deliberate. The server listens on `127.0.0.1` only. It
refuses any request whose `Host` does not name the loopback interface
(`the web interface serves the loopback interface only`), refuses writes from a
page served anywhere else (`cross-origin writes are refused`), and answers only
requests that present the run's session. Together these keep a website open in
another tab, or another account on the machine, from driving your repository and
tokens. If workflow runs on a remote host you reach over SSH, forward the port
(`ssh -L 13579:127.0.0.1:13579 <host>`) and open the address it printed in your
local browser. See [The web interface: Start
it]({{< relref "/docs/web#start-it" >}}).

### What happens to a change I made if I close the tab, or quit workflow?

Closing or reloading a tab stops only the reads that page was waiting on. A
write it already sent, such as a commit, a push, a post or a status change,
keeps running to its end. Pressing `ctrl+c` in the terminal stops the server,
and anything still in flight is cut short. In the terminal interface, quitting
stops reads but waits for your writes to land, showing `finishing 2 writes…`,
and gives up after `timing.request_timeout`. An announcement waiting for CI is
not sent yet, so quitting first drops it. See [The web interface: Start
it]({{< relref "/docs/web#start-it" >}}) and [Using workflow:
Limits]({{< relref "/docs/usage#limits" >}}).

### Which browsers does the web interface support?

The latest two versions of Chrome, Edge, Firefox and desktop Safari. The page is
only ever served to a browser on the same machine, so mobile Safari is not a
target. The end-to-end suite runs every spec in Chromium, plus a layout and
accessibility smoke run in Firefox and WebKit. The page fetches nothing from the
internet: its font and code come from the workflow binary. See [The web
interface: The page]({{< relref "/docs/web#the-page" >}}).

## Taskwarrior

### Which Taskwarrior do I need, and where do I get it?

workflow needs [Taskwarrior](https://taskwarrior.org) 3.5.0 or newer. It is
optional: without it, the Tasks pane and the web's Tasks section say so and
nothing else changes. On macOS, run `brew install task`. On Linux, use your
distribution's package if it has 3.5.0 or newer (many still ship 2.x). Otherwise
build it from [Taskwarrior's
source](https://github.com/GothenburgBitFactory/taskwarrior), which needs Rust
and CMake. Then run it once in a terminal so it writes its configuration. An
older one is reported, not used: the interface says "Taskwarrior is too old:
3.5.0 or newer is needed", and `workflow doctor` names the version and path it
found, such as `taskwarrior is older than workflow needs: 2.6.2 at
/usr/bin/task; 3.5.0 or newer is needed`. See [Install: What it
needs]({{< relref "/docs/install#what-it-needs" >}}).

### doctor says `task` is not Taskwarrior, or it was never run. What now?

Taskwarrior's program and go-task, the Taskfile runner, are both called `task`.
workflow asks each `task` on `PATH` for its version and keeps the first that
answers as Taskwarrior 3.5.0 or newer. If none does, doctor says
`task on PATH is not Taskwarrior (go-task?)`; set `taskwarrior.program` to
Taskwarrior's absolute path in `~/.workflow.json`, for example
`"taskwarrior": { "program": "/opt/homebrew/bin/task" }`. A repository's file
may not set it (`taskwarrior.program: only the home directory's file may set
it`), and a relative path such as `bin/task` is refused.
`installed but never run` means Taskwarrior has no taskrc yet: run it once in a
terminal, by the path doctor names, and answer its question. Either change
applies the next time workflow starts or switches directory. See [Configuration:
Finding it]({{< relref "/docs/configuration#finding-it" >}}).

### What does workflow do to my tasks, and how do I turn it off?

workflow runs `task` for every read and write and keeps no copy of your tasks.
Reads run with Taskwarrior's hooks off, so listing never fires a sync hook or
changes a task. A task changes only when you press a key or accept an offer,
such as starting the issue's task when you create its branch. Writes run your
hooks as `task` would in a shell. Tasks it creates carry `jiraid`, `jiraurl` and
`+jira`. For `task jiraid:PROJ-412` to work in your own shell, add
`uda.jiraid.type=string` and the other three UDA lines that `workflow doctor`
names to your taskrc. To turn the integration off even with Taskwarrior
installed, set `"taskwarrior": { "disabled": true }`; the Tasks pane then says
"Turned off by taskwarrior.disabled." See [Configuration: Reads, writes and
contexts]({{< relref "/docs/configuration#reads-writes-and-contexts" >}}) and
[Configuration: Tasks and
issues]({{< relref "/docs/configuration#tasks-and-issues" >}}).

## Scripting and automation

### In CI or cron, a command stopped with "no terminal to answer on". What do I do?

The full message is `no terminal to answer on; pass --yes to go ahead without
asking`. Every write (`branch`, `pr`, `finish`, `announce`, `comment`,
`transition`, `assign`, `worklog`, `summary --post`, `db-clean`) prints a
preview and then asks `[y/N]`. When standard input is piped or closed, as in CI
or cron, nothing can answer, so the command stops and exits `2`. Pass `--yes`
(`-y`) to go ahead without asking, or `--dry-run` (`-n`) to see what it would
do. `comment` reads its text from stdin, so piped it always needs `--yes`. Three
commands differ: the guided `config init` says to pass `--template`,
`slack login` must run at a terminal because it asks for secrets, and `announce`
at a moment already announced says to run it at a terminal to be asked again. In
CI, supply tokens through the environment ([Where can each token come from, and
which one wins?](#where-can-each-token-come-from-and-which-one-wins)). Don't
copy a Slack user token to a CI machine: each refresh spends the copy the other
machine holds ([Posting fails with
invalid_refresh_token](#posting-fails-with-refreshing-the-slack-user-token-was-refused-invalid_refresh_token-how-do-i-recover)),
so a webhook suits CI. See [Scripting: Writing without a
person]({{< relref "/docs/scripting#writing-without-a-person---yes-and---dry-run" >}}).

### What exit statuses does workflow use, and where are they listed?

A script should tell failures apart by the exit status, never by the message,
which is prose and may change. `0` is success, `1` any other failure, `2` a
usage mistake (a bad flag, a wrong argument, a question with no terminal to
answer it), `3` configuration (fix the file, a credential or a login), `4` a
refused precondition (a pull request already open, a branch that already exists,
a dirty tree), `5` a service that could not be reached or asked you to wait, and
`130` Ctrl+C. When a command finds several problems at once, as `doctor` does,
it exits with the first family in the order 2, 3, 4, 5, 1. So `doctor` with no
configuration and no git exits 3. The full table, with examples for each status
and the matching `--web` problem codes, is in [Scripting: Exit
status]({{< relref "/docs/scripting#exit-status" >}}).

### How do I get output a script can parse, and why is some text missing from my pipe?

Standard output carries only the result: the JSON, the preview, the rows, and
what a write created. Everything said *about* it goes to standard error: notes,
`WARN` and `ERROR` lines, dry-run lines and the questions. So
`workflow reviews | wc -l` counts reviews and nothing else, and you need `2>` to
capture the messages. `--json` (`-j`) works on `status`, `reviews`,
`repositories`, `summary`, `issues`, `issue`, `doctor` and `pr`, and
`config show` always prints JSON with credentials masked. To change what
standard error says, use `--quiet` (`-q`) for only warnings, errors and
questions, `--verbose` for a `DEBUG` line per request, or `--log-format json` or
`logfmt` for a log collector. `--quiet` with `--verbose` exits `2`, and
`NO_COLOR` turns the colored tags off. See [Scripting: Standard output and
standard
error]({{< relref "/docs/scripting#standard-output-and-standard-error" >}}) and
[Scripting: JSON]({{< relref "/docs/scripting#json" >}}).

## Local data and privacy

### Where does workflow keep its local data, and does it store my tokens?

It keeps two small SQLite files in your platform's data directory:
`~/Library/Application Support/workflow` on macOS, `$XDG_STATE_HOME/workflow` or
`~/.local/state/workflow` on Linux, and `%AppData%\workflow` on Windows.
`workflow.db` is a cache (the last commit scope, what you have announced, the
last issue list of each view) that a session rebuilds. `kept.db` holds what you
decided: whom each code owner is on Slack, the Slack user groups each repository
offers, and your favorite directories. The store **never holds a secret** and is
never keyed by a credential or by a raw remote or Jira URL. Tokens live in
`.workflow.json`, the keychain or your environment ([Where can each token come
from, and which one wins?](#where-can-each-token-come-from-and-which-one-wins)).
Where the filesystem keeps Unix modes, each file is `0600` in a `0700`
directory. `workflow doctor` names the directory on its `Store` row. Set
`"store": { "disabled": true }` to keep nothing on disk. See [Configuration:
What is kept between
sessions]({{< relref "/docs/configuration#what-is-kept-between-sessions" >}}).

### How do I clear the local data, and what does `db-clean` remove?

`workflow db-clean` prints `Local data in DIR`, each database file with its size
and what it holds, and then asks before removing anything. By default it removes
only the cache, `workflow.db`, which the next session makes again. `--all` also
removes `kept.db`, after a warning, and workflow then asks again whom each code
owner is on Slack. `--dry-run` only says what it would remove, and `--yes`
removes without asking. A file another program holds open (another workflow
session, on Windows) fails the clean with exit `4`, and a symlink or directory
where a database belongs is refused with exit `1`. If workflow says ``the kept
data was written by a build of workflow with another schema; run `workflow
db-clean --all` to start it fresh``, that is the fix. The same cleans are under
**Local data** in the web's Settings and behind `L` on the terminal's
Repositories pane (`c` removes the cache, `C` everything). See [Using workflow:
Cleaning the local data]({{< relref "/docs/usage#cleaning-the-local-data" >}}).

### Does workflow send anything anywhere else, such as telemetry or update checks?

No. workflow makes no analytics, crash-reporting or phone-home calls, and does
not check for updates: it talks only to the Jira, forge and messaging service
you configure. The `--web` page fetches nothing from the internet either, since
its font and code come from the binary. Code that sends data anywhere you did
not configure is a security bug; report it as [the security
policy](https://github.com/jacob-delgado/workflow/blob/main/SECURITY.md#no-telemetry)
says.
