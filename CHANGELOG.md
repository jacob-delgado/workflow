# Changelog

## [0.6.0](https://github.com/jacob-delgado/workflow/compare/v0.5.1...v0.6.0) (2026-10-05)


### ⚠ BREAKING CHANGES

* **store:** the kept schema's version goes from 1 to 2, with no migration. A kept.db an earlier build made reads as empty, and refuses writes, until `workflow db-clean --all` starts it fresh; its owners and groups are then asked about again once.
* **config:** a configuration that leaves out jira.markdown_comments now converts comments from Markdown; set it to false to keep posting them as typed.

### Features

* **activity:** group what was done by year, month, day and hour ([0011cc3](https://github.com/jacob-delgado/workflow/commit/0011cc3b5b44ff8d41eab9ee1dd477de8482a07c))
* **activity:** move a date by months, and take its month or year ([79a9bec](https://github.com/jacob-delgado/workflow/commit/79a9becdb4c4fe766a71a506c2351274ab061313))
* **activity:** say which source each kind of thing done is from ([0b5a6f8](https://github.com/jacob-delgado/workflow/commit/0b5a6f8b9998c5a0c1c7d5967c44a7ca1fd6335a))
* **api:** comment on a forge issue from the web ([94a0019](https://github.com/jacob-delgado/workflow/commit/94a001986f559ef693341071fd995ad1ba96262d))
* **api:** list worktrees and start work in a new one from the web ([d9336c0](https://github.com/jacob-delgado/workflow/commit/d9336c043d060a92699b12868de394b10fa61f9b))
* **api:** read back what you did over a period from the web ([6c89346](https://github.com/jacob-delgado/workflow/commit/6c89346874c597997f664fd3ccd2c7e3ccb9ec01))
* **api:** say which worktree has an issue's branch checked out ([e40ab72](https://github.com/jacob-delgado/workflow/commit/e40ab723404078ba7ea8156ad95cfecf78fe4d93))
* **api:** switch directory and keep favorites from the web ([d8d7e9b](https://github.com/jacob-delgado/workflow/commit/d8d7e9bfff44a4d147263d923584a72bfaca8c4f))
* **config:** write comments in Markdown by default ([f1893a0](https://github.com/jacob-delgado/workflow/commit/f1893a0a1fef7b6254d34ca167c3c6bffc47ad6a))
* **forge:** read and post a forge issue's comments ([01c7d43](https://github.com/jacob-delgado/workflow/commit/01c7d4340472b0968249cc9337417b52972da693))
* **forge:** read what you did on the forge over a period ([1b33567](https://github.com/jacob-delgado/workflow/commit/1b33567b2d91af38e2d4f57e2c060c20fb5bc437))
* **gitrepo:** read a repository's worktrees ([364bce7](https://github.com/jacob-delgado/workflow/commit/364bce735a92dba84b0359f304415eb60491e65a))
* **gitrepo:** read the git directory a repository's worktrees share ([ab43c85](https://github.com/jacob-delgado/workflow/commit/ab43c8546fef1d4befd702c3f70caba7e721910b))
* **gitrepo:** read your commits by when you wrote them ([7277b54](https://github.com/jacob-delgado/workflow/commit/7277b54c74c5f13d86f37a5802a800c865584074))
* **jira:** read what you did to issues over a period ([19c64f1](https://github.com/jacob-delgado/workflow/commit/19c64f148838764361408c1bb9181e8e9105c5f2))
* **loop:** turn each source's answer into the Summary's items ([c11663a](https://github.com/jacob-delgado/workflow/commit/c11663a9b47b84bc3047a3107c8fc442bc7569da))
* read the Summary's commits across your favorite repositories ([3858426](https://github.com/jacob-delgado/workflow/commit/3858426a492f3d9b50766a16a7b8cffb5e726d84))
* **store:** keep the directories you mark as favorites ([978cb83](https://github.com/jacob-delgado/workflow/commit/978cb836b1f7e7b85c85f00573c7dd8420771728))
* **taskwarrior:** narrow tasks by facet and typed text ([571baef](https://github.com/jacob-delgado/workflow/commit/571baef90ba26a5800c788c3b702f423118a8a7e))
* **taskwarrior:** order tasks by state, id, tag, issue and priority ([e87147e](https://github.com/jacob-delgado/workflow/commit/e87147ec6778a6ce3aafb1798494e7e2bac033d4))
* **taskwarrior:** read the tasks you touched since a time ([ff96be0](https://github.com/jacob-delgado/workflow/commit/ff96be091d8731bc75833b22f0bf620af9b42c34))
* **tui:** comment on a forge issue ([428b56e](https://github.com/jacob-delgado/workflow/commit/428b56e561dd38e693c1f6b85b619dae3cc7b7dd))
* **tui:** filter and narrow the Tasks pane ([c55c40e](https://github.com/jacob-delgado/workflow/commit/c55c40eaabe0b1e0550976b6343275577b7aaad1))
* **tui:** keep the focused pane readable with more panes than fit ([88f4588](https://github.com/jacob-delgado/workflow/commit/88f4588f2d385abafd5cab2f15bf7c3da72fb565))
* **tui:** lead the top row with where you work ([c9c5298](https://github.com/jacob-delgado/workflow/commit/c9c52982af6403f553a8d22a307543a3dd7d1a52))
* **tui:** list the repository's worktrees in the Repositories pane ([5c8f2cf](https://github.com/jacob-delgado/workflow/commit/5c8f2cff5e0a6d34ee42eaaab82e05753cc26d7e))
* **tui:** offer to switch into a worktree just made ([df06b29](https://github.com/jacob-delgado/workflow/commit/df06b29acf64f7c12c7de5f412610e8f3d282688))
* **tui:** pick the Summary's period in a calendar ([5d6a828](https://github.com/jacob-delgado/workflow/commit/5d6a8284b8b34813077ee3fb3a8f2ba1b30ab51e))
* **tui:** say where you work and keep favorites in a Repositories pane ([6c0db9c](https://github.com/jacob-delgado/workflow/commit/6c0db9cd02c79de9fb9a6c04c370de9a3f85085f))
* **tui:** show what you did in a Summary pane ([3118545](https://github.com/jacob-delgado/workflow/commit/311854507f80604a0492cfb80ec58857e1dccdd2))
* **tui:** sort the Tasks pane with O ([dbf9595](https://github.com/jacob-delgado/workflow/commit/dbf95953d5e60157c7d3c5319c8f72448e45d860))
* **tui:** switch the directory you work in from the Repositories pane ([d47c143](https://github.com/jacob-delgado/workflow/commit/d47c143d5e8f84de52cb8797d2de3a8ec3a305d9))
* **tui:** switch to a branch's worktree from the task switcher ([fd67665](https://github.com/jacob-delgado/workflow/commit/fd67665ee37100d8fd095e5c76751eae663e5525))
* **tui:** write a comment in a box with vim's normal and insert modes ([1359dec](https://github.com/jacob-delgado/workflow/commit/1359decba6691c16fe39ee7e99bbe4df9dcbf950))
* **web:** comment on a forge issue from its detail ([ec2009b](https://github.com/jacob-delgado/workflow/commit/ec2009b1c5a818ed8a457e00dcd376d033417fef))
* **web:** filter and narrow the Tasks section ([eabe11a](https://github.com/jacob-delgado/workflow/commit/eabe11a9ef2a4867576c2a5612988e14cb6f9627))
* **web:** list worktrees, start work in one, and switch to it ([0b45ee1](https://github.com/jacob-delgado/workflow/commit/0b45ee14bc3c80553633db6a841b196683952eca))
* **web:** say where you work and switch from a Repositories section ([7eecae3](https://github.com/jacob-delgado/workflow/commit/7eecae36a2aa29c6dcd52bae62c083b3a887d3d6))
* **web:** show what you did in a Summary section ([056778d](https://github.com/jacob-delgado/workflow/commit/056778dbb6ca2d969c3ba99b1704cc8269dd3aa8))
* **web:** sort the Tasks section ([6233a18](https://github.com/jacob-delgado/workflow/commit/6233a18ca9df1a2fbcc181df7fc63eafdb433e48))
* **wiring:** comment on forge issues and read their threads ([974fedd](https://github.com/jacob-delgado/workflow/commit/974fedd9b1c036ea17df663a38b8e949b1abe382))
* **wiring:** list the worktrees of the repository you work in ([f64f79d](https://github.com/jacob-delgado/workflow/commit/f64f79d5ad555b2b10e36ca74ba662242b17400b))
* **wiring:** reach git, Taskwarrior, Jira and the forge for the Summary ([3965858](https://github.com/jacob-delgado/workflow/commit/396585883c5491ffe414a1a517cf8699f4ae8eb6))
* **wiring:** read where you work and the directories you can switch to ([1d591a7](https://github.com/jacob-delgado/workflow/commit/1d591a7fe49a7aa7e2a339567e339ad298494529))
* **workdirs:** read the directories workflow can work in ([1cbd4c7](https://github.com/jacob-delgado/workflow/commit/1cbd4c7955435889c828bda5a28f8fc65d56cd06))


### Bug Fixes

* convert a Markdown comment by the setting in effect ([5e5c524](https://github.com/jacob-delgado/workflow/commit/5e5c5241d99786eac40394b03f75246a1cf4af15))
* harden worktrees against gone ones, twin starts and repeats ([6d8d31f](https://github.com/jacob-delgado/workflow/commit/6d8d31f188ef3be26ca91f08e8a59e4cf086c91d))
* name the repository whose commits could not be read ([52d731d](https://github.com/jacob-delgado/workflow/commit/52d731dfb3122eea85f1ccb263d0842625829d8e))
* **tui:** count editing keys and whole characters against interrupt ([bfb0159](https://github.com/jacob-delgado/workflow/commit/bfb0159b52b25854a455ccde51f60fcb6be3fe2c))
* **tui:** narrow the Tasks list as it says, wherever the cursor goes ([6faada8](https://github.com/jacob-delgado/workflow/commit/6faada8890100d184892ab4963f14a9ca62799ff))
* **tui:** paste into the focused text field ([78f05fc](https://github.com/jacob-delgado/workflow/commit/78f05fc121dad7678de67f7a1495492aa8962f82))
* **tui:** paste into the owner picker's filter ([d33bc67](https://github.com/jacob-delgado/workflow/commit/d33bc67fec6239fdc93e4c30eac23fb9666fab0a))
* **tui:** post a comment as written; neutralize controls on screen only ([b70e372](https://github.com/jacob-delgado/workflow/commit/b70e37271c6c8e2198699ea8323cf47b033f39ab))
* **tui:** read the Repositories pane when a switch opens on it ([7e48747](https://github.com/jacob-delgado/workflow/commit/7e4874725fc0ed0b1e8e2108b80c1f6fba5d8372))
* **tui:** refuse a keymap that puts interrupt on a key that types ([352d436](https://github.com/jacob-delgado/workflow/commit/352d43658f7eaa734a0f42fcfe3096ef4372cfa9))
* **web:** close the worktree offer once switched, and say what fails ([990f333](https://github.com/jacob-delgado/workflow/commit/990f333ad104b4ea5f0cdf9258ce5d9bbd197592))
* **web:** count a narrowed Tasks list out of what it shows, keep focus ([539f823](https://github.com/jacob-delgado/workflow/commit/539f8234fd6f57d172090280f2fddc18a961d177))
* **web:** draw a comment too large to parse safely as plain text ([997c668](https://github.com/jacob-delgado/workflow/commit/997c66830a300a541559e6621916529217438de6))
* **web:** order issue keys equal but for leading zeros as Go does ([1fa69fb](https://github.com/jacob-delgado/workflow/commit/1fa69fbb1ed91bde5232139249785c45c37908e5))
* **wiring:** keep forge issues out of the issue cache ([535fdd7](https://github.com/jacob-delgado/workflow/commit/535fdd7dca618528012f541dd72aded20b0c6d88))
* **wiring:** leave a view of only forge issues uncached ([03bc705](https://github.com/jacob-delgado/workflow/commit/03bc705874d2fdc6e7e00f8816fe722b0679274d))
* **wiring:** tell apart repositories the Summary would name alike ([bb51709](https://github.com/jacob-delgado/workflow/commit/bb517091bfb3b58c81617afaf1ff6c71581a30d7))


### Performance

* **wiring:** read the Summary's repositories side by side ([de47f6c](https://github.com/jacob-delgado/workflow/commit/de47f6c38346a31ad43a708d74c5888c5314a821))


### Refactors

* **cli:** wire a command to its directory in connect.go ([3fa719e](https://github.com/jacob-delgado/workflow/commit/3fa719e15862ea6c383b9f5649378ff30c2a0057))
* **loop:** drop a comment markup arm that repeated its last case ([f537e93](https://github.com/jacob-delgado/workflow/commit/f537e939b78a96990a0cb7d8e232cf3ca4e9b4ba))
* **loop:** post a forge issue's comment as Markdown, as written ([faf395e](https://github.com/jacob-delgado/workflow/commit/faf395e829072e1a3f87d0544d12e5f96791a5f8))
* **tui:** move the keymap check out of keys.go and balance the help ([67ffe53](https://github.com/jacob-delgado/workflow/commit/67ffe53737364b6095159776a47134d5624cd4d9))
* **tui:** move the Tasks pane's listing into tasklist.go ([f4eb145](https://github.com/jacob-delgado/workflow/commit/f4eb145729113e4fa0033667ebb38d987eb4c2fe))
* **tui:** share one checklist overlay for every list narrowing ([2ba3e1b](https://github.com/jacob-delgado/workflow/commit/2ba3e1b62ea0ec0ec26d261735d946dfe2a69ec0))
* **web:** share one chip group for every list narrowing ([2b12a36](https://github.com/jacob-delgado/workflow/commit/2b12a3683956a2c1fbb9370bc9ffcc500ec95d70))
* **wiring:** read where the interface runs in repositories.go ([fe7b318](https://github.com/jacob-delgado/workflow/commit/fe7b3188543f508ae5aede1202ae6a6ef3f5f74a))


### Documentation

* name the Taskwarrior client in the architecture's map ([13df9e2](https://github.com/jacob-delgado/workflow/commit/13df9e280899f99a822fc4273d7f111368ebbf3f))
* point UX.md at where the moved and grown code now sits ([9b573fd](https://github.com/jacob-delgado/workflow/commit/9b573fd96f0c24f392087b59709e3ebf1c8aa306))
* say what the Tasks orders and facet counts leave out ([30f9666](https://github.com/jacob-delgado/workflow/commit/30f966652296ed4864ad9cf7c8a86a4cc27dfd45))


### Tests

* **web:** sort, filter and narrow tasks end to end ([d125a3a](https://github.com/jacob-delgado/workflow/commit/d125a3aaf6106aa3f19dae01d578fa8869bc64ac))

## [0.5.1](https://github.com/jacob-delgado/workflow/compare/v0.5.0...v0.5.1) (2026-10-05)


### Features

* **web:** comment on a Jira issue from its detail ([ba30014](https://github.com/jacob-delgado/workflow/commit/ba300140ab01d04ea616004cb3d50d0b0e6168f8))
* **web:** render Jira wiki markup in comments ([5511d7a](https://github.com/jacob-delgado/workflow/commit/5511d7a1cc630cce8e488025943ab5fb3c693c66))
* **webserver:** post a comment on a Jira issue ([d762861](https://github.com/jacob-delgado/workflow/commit/d762861825102b0753cc170c0bc89324571e9294))
* **web:** turn Markdown into wiki markup as Jira will ([7d74380](https://github.com/jacob-delgado/workflow/commit/7d743806d887279e42e84dd4cdcea47b85119374))


### Bug Fixes

* **web:** keep hostile comment markup linear and read Jira's CRLF ([17dec73](https://github.com/jacob-delgado/workflow/commit/17dec73b350ea9cb3bbe60e1c40216856938d363))


### Documentation

* comment on Jira issues from the web ([416a779](https://github.com/jacob-delgado/workflow/commit/416a779b71e5b44c71283abfaca8914ad54eafbe))


### Tests

* **web:** drive commenting end to end ([7146b48](https://github.com/jacob-delgado/workflow/commit/7146b482e1239cea45e3aced27a0b2795f98b4f9))

## [0.5.0](https://github.com/jacob-delgado/workflow/compare/v0.4.3...v0.5.0) (2026-10-01)


### ⚠ BREAKING CHANGES

* **messaging:** messaging.token, token_command and token_env are removed; a file naming them is refused with the way to run `workflow slack login` instead.

### Features

* **api:** carry proposed reviewers on the pull request draft ([0bc6aad](https://github.com/jacob-delgado/workflow/commit/0bc6aad12f665d6cbe9aacdf78018ddf0e7fa837))
* **cli:** clean the local databases with db-clean ([75dc6d6](https://github.com/jacob-delgado/workflow/commit/75dc6d6717f5a955862dc4732178417d4fa7b94b))
* **cli:** request code owners' review on workflow pr ([6754ac5](https://github.com/jacob-delgado/workflow/commit/6754ac50aac3a3f73c7d738932d9011e32986c27))
* **cli:** tag known owners and groups on workflow announce ([f9d875b](https://github.com/jacob-delgado/workflow/commit/f9d875b95d72df5efe1026391d14e7159358ad78))
* **codeowners:** read CODEOWNERS as GitHub and GitLab do ([437ffff](https://github.com/jacob-delgado/workflow/commit/437ffffadd4dac26911acbe265bd7d4d9fee9d59))
* **config:** a repository can opt into its forge's issues ([fd831e9](https://github.com/jacob-delgado/workflow/commit/fd831e918c10b2c9bace1545fa7935aa991fd2b2))
* **config:** layer the repository file over the home file ([b39ed45](https://github.com/jacob-delgado/workflow/commit/b39ed45292ae230a16859dc47b937b88c269b9f3))
* **forge:** link to an issue, and assign one ([eeedbc9](https://github.com/jacob-delgado/workflow/commit/eeedbc9f8027d811682927dbefe0313995426321))
* **forge:** list a GitLab group's direct members ([0e377fb](https://github.com/jacob-delgado/workflow/commit/0e377fbfbfcccf7757df21d78e448715c46e57a1))
* **forge:** read a failed job's log ([185d629](https://github.com/jacob-delgado/workflow/commit/185d6292e87f15854d0888920e69706f4357fc55))
* **forge:** request team reviewers on GitHub ([7898d94](https://github.com/jacob-delgado/workflow/commit/7898d94bb2272953b2b5a9216471132bb1c88762))
* **forge:** say why each failed check failed ([00e43a8](https://github.com/jacob-delgado/workflow/commit/00e43a843fe090863e3cd9d1c518be0ced15804b))
* **gitrepo:** keep a branch's issue in git's configuration ([4fe63c2](https://github.com/jacob-delgado/workflow/commit/4fe63c2153e7ab9a09fb8e55f81165339ebff2e3))
* **gitrepo:** read changed paths and a file at a ref ([ab416ff](https://github.com/jacob-delgado/workflow/commit/ab416ff5a80002253903d1088e31817242b0f5e4))
* **keychain:** read a secret back, under a service of its own ([bf0cb43](https://github.com/jacob-delgado/workflow/commit/bf0cb4316b1a313c4b9ff972f17694c5cd4bb65f))
* link a top-level GitLab group to a Slack user group ([d659ae4](https://github.com/jacob-delgado/workflow/commit/d659ae4afc86b34578b17d060b05754c8bc16b27))
* link the branch to an issue, and its pull request ([51b6e1b](https://github.com/jacob-delgado/workflow/commit/51b6e1b8671d39de848e83e4dd1462c1be89bed4))
* **loop:** propose code owners as reviewers ([5f9d86e](https://github.com/jacob-delgado/workflow/commit/5f9d86ed9a8571d7400b310f11d451af0f3cdf0c))
* **loop:** propose who to tag in an announcement ([03351d2](https://github.com/jacob-delgado/workflow/commit/03351d2b60e760eb7928d41501195f02534a028a))
* **messaging:** label a large channel's members one by one ([2e2ef6a](https://github.com/jacob-delgado/workflow/commit/2e2ef6a04014651f73795193af57f92919da848a))
* **messaging:** post to Slack with a rotating user token ([fa6e241](https://github.com/jacob-delgado/workflow/commit/fa6e241711a1fa4415f54283f7160d6f6a22ff4f))
* **messaging:** read channel members and user groups ([11bc336](https://github.com/jacob-delgado/workflow/commit/11bc3367b4910983f018866c2e9972b2345d65d0))
* **messaging:** read the Slack workspace a token is for ([d6b30c8](https://github.com/jacob-delgado/workflow/commit/d6b30c81a0198147537498a8f745f82f8e238143))
* **messaging:** render validated Slack mentions ([50e3892](https://github.com/jacob-delgado/workflow/commit/50e389296b869468077d91b6852f26c56202667c))
* **reviews:** filter the queue by repository, CI, draft and author ([f15751b](https://github.com/jacob-delgado/workflow/commit/f15751ba162b42824eb22225b0a6fcb68388b433))
* **review:** show the issue a pull request is for ([75cf426](https://github.com/jacob-delgado/workflow/commit/75cf426a64c59a1b556c6ffbe7018c657ac7d94c))
* **reviews:** sort by age or repository ([4519f0c](https://github.com/jacob-delgado/workflow/commit/4519f0ce50108a66697d9a66d8f9606350f9ff71))
* say each issue's tracker in the API, terminal and web ([d387334](https://github.com/jacob-delgado/workflow/commit/d387334d4cc9797026f6ffe0837b00f31731cfe3))
* show a failed check's log on demand ([fed71a6](https://github.com/jacob-delgado/workflow/commit/fed71a65e56f97a4acb472f3a558f26d2b9fec69))
* **store:** add a kept database that migrates forward ([ffd6eb6](https://github.com/jacob-delgado/workflow/commit/ffd6eb6931b47a83bc6fc50360c0909beb0ae5d8))
* **store:** keep each repository's Slack groups and last choice ([d222312](https://github.com/jacob-delgado/workflow/commit/d22231283f891a4765a850fd727e80326611b8aa))
* **store:** keep forge owners' Slack identities ([967f103](https://github.com/jacob-delgado/workflow/commit/967f1037c6feb381616533cb47ef50beac5e8ccb))
* **store:** list and clean the local databases ([23c30a0](https://github.com/jacob-delgado/workflow/commit/23c30a0a4b356f712e417b7901515bfa961a0fc2))
* tag only within the Slack workspace in use ([915b716](https://github.com/jacob-delgado/workflow/commit/915b716a07df754e4da156a207ecf86a983a7de7))
* **tui:** manage people and groups from the messaging pane ([83bb303](https://github.com/jacob-delgado/workflow/commit/83bb3033bd6da8d0aba8b6d719c34d9252586df7))
* **tui:** pre-fill reviewers from code owners ([d11cc25](https://github.com/jacob-delgado/workflow/commit/d11cc2507b61f47af91693e3d8e15f9b6e64a9d6))
* **tui:** refresh a pane on switch when stale ([51d6d10](https://github.com/jacob-delgado/workflow/commit/51d6d10d4681d0d6ddbd66394bad450dba28e0a2))
* **tui:** tag code owners and groups in the announcement preview ([187acc4](https://github.com/jacob-delgado/workflow/commit/187acc47d38ea2a2585e0fc63503b9935c7fda61))
* **web:** manage people and groups in Settings ([4615313](https://github.com/jacob-delgado/workflow/commit/4615313875689a667703f9132cf8c43156d5797e))
* **web:** pre-fill the pull request form's reviewers ([10a1f84](https://github.com/jacob-delgado/workflow/commit/10a1f8489af90b0e5c197b8156a57afd51e75577))
* **web:** read Reviews and an issue again after 30 seconds ([17d47a7](https://github.com/jacob-delgado/workflow/commit/17d47a71cda909fd4a7e8ffbcb9120646e553fbe))
* **webserver:** list and clean the local databases ([fe983dc](https://github.com/jacob-delgado/workflow/commit/fe983dc66fb92ea5d9d1acc172100b443685fb08))
* **webserver:** serve tags, people and repository groups ([4046aae](https://github.com/jacob-delgado/workflow/commit/4046aae3b1f3ac917f3ef3a4084b7cd60316837f))
* **web:** show and clean local data in Settings ([9056276](https://github.com/jacob-delgado/workflow/commit/9056276494a132813c57f9e6f864ae0fe284a653))
* **web:** sort the review queue by age or repository ([5013059](https://github.com/jacob-delgado/workflow/commit/50130591ca5bc8444f0c675cd40a002e2f9085fa))
* **web:** tag owners and groups in the announcement preview ([c48ca70](https://github.com/jacob-delgado/workflow/commit/c48ca7055a31a33084df3f8046285238100b945d))
* **wiring:** bind kept associations to the forge host ([c94424f](https://github.com/jacob-delgado/workflow/commit/c94424fd4e2a741050219d10c94fac3d5b703409))
* **wiring:** cache the Slack directory per session ([61efa99](https://github.com/jacob-delgado/workflow/commit/61efa9993bc8ea3f5945f6764064bb00b3392576))
* **wiring:** let tests point the Slack client at a fake ([ae80ef8](https://github.com/jacob-delgado/workflow/commit/ae80ef8c7b429187f47b6be31d3c61984da3cc8f))
* **wiring:** list Jira's and the forge's issues together ([e4f59ec](https://github.com/jacob-delgado/workflow/commit/e4f59ec332fa69ef56e73261b54eb668d16ca56b))


### Bug Fixes

* ask again about a bare name decided before GitLab told groups ([7456ea3](https://github.com/jacob-delgado/workflow/commit/7456ea3049de253475c84a5296a531b4487e31e5))
* **cli:** let config init --force replace a repository layer ([763f890](https://github.com/jacob-delgado/workflow/commit/763f8905e38b4e15f088977f0ac3501a3eec812d))
* **cli:** read announce's owners against the pull request's base ([54eb10f](https://github.com/jacob-delgado/workflow/commit/54eb10fa41daa14c1ede48acda75a0f85976be42))
* **cli:** say pr opened when a reviewer could not be added ([92d6077](https://github.com/jacob-delgado/workflow/commit/92d6077847760e1abce9e58d8523bc28ef5bb22e))
* **codeowners:** match GitLab patterns with Ruby's fnmatch ([4a12e75](https://github.com/jacob-delgado/workflow/commit/4a12e757c0257ac3d70e8a0ad9ff958beee1abb4))
* **codeowners:** name GitLab's top-level section codeowners ([30ef3f6](https://github.com/jacob-delgado/workflow/commit/30ef3f614e5bf58d91f11963dcea132e6e340ba2))
* **codeowners:** read GitLab directory patterns as GitLab does ([41dc8c1](https://github.com/jacob-delgado/workflow/commit/41dc8c1d32e80caeeaf9e6486017f537baa6d9c7))
* **codeowners:** read GitLab section headers by GitLab's regex ([c9c811b](https://github.com/jacob-delgado/workflow/commit/c9c811ba6b98a079e939b941b4367846bf1759ee))
* **codeowners:** take GitLab owners as GitLab's extractor does ([ee11d4d](https://github.com/jacob-delgado/workflow/commit/ee11d4de19b56a78862488a9026e9e5b9db95055))
* **config:** lend a home credential only to its own address ([fd8c141](https://github.com/jacob-delgado/workflow/commit/fd8c14184d19bf81c6c8bf41495f684033b318d5))
* **config:** save over a home file valid only with its layer ([a58e1e2](https://github.com/jacob-delgado/workflow/commit/a58e1e24e52ec10c7d896a171328341effc8776a))
* **convention:** count only [#42](https://github.com/jacob-delgado/workflow/issues/42) or its URL as naming forge issue 42 ([49e488b](https://github.com/jacob-delgado/workflow/commit/49e488be8b0e2157e33169ac79b791c4d5a9dbb6))
* **convention:** take an issue key only in its tracker's shape ([bfda9d6](https://github.com/jacob-delgado/workflow/commit/bfda9d684f19e5278cbf6456616263de12ac0188))
* **forge:** add GitLab assignees best effort ([9b39d18](https://github.com/jacob-delgado/workflow/commit/9b39d18884e7ea39a96cdccaf33cb913cfe36ba7))
* **forge:** add reviewers best effort on both forges ([03595bb](https://github.com/jacob-delgado/workflow/commit/03595bb44c5e0ff44d935cc5d6b8db376020feeb))
* **forge:** ask only GitLab members who can approve to review ([97af20e](https://github.com/jacob-delgado/workflow/commit/97af20e2cdf85cb18083238a31ce69383820cb23))
* **forge:** expand a GitLab name no user has as a group ([5e05bfd](https://github.com/jacob-delgado/workflow/commit/5e05bfd790e15c37116875ff4d472e9d6ea61b4d))
* **forge:** keep the real end of a job log past 32 MiB ([3c08460](https://github.com/jacob-delgado/workflow/commit/3c084601d7c7d503d2c22b89724a517938c1a01a))
* **forge:** leave the author out of a GitLab team's reviewers ([1263e20](https://github.com/jacob-delgado/workflow/commit/1263e20dffa5a5695111ead553b60ea42c5bf9b9))
* **forge:** request only the repository's own org's teams ([28466a2](https://github.com/jacob-delgado/workflow/commit/28466a220cfdda4120cfe2b03d436bb340aeb505))
* **forge:** resolve GitLab reviewers best effort, members by id ([84410cf](https://github.com/jacob-delgado/workflow/commit/84410cf967ff35b768645c3d1bbed408cc1657e8))
* **forge:** tell a job log's storage failure from the forge's ([d332615](https://github.com/jacob-delgado/workflow/commit/d3326150bbe1dccc01bf4a283ff80c2edb192c93))
* **gitrepo:** clear a stored issue link even when it cannot be shown ([aa34009](https://github.com/jacob-delgado/workflow/commit/aa34009d5fdb478401e04a91e1a25c46dc5969d1))
* **gitrepo:** keep changed paths that only draw differently ([b03485c](https://github.com/jacob-delgado/workflow/commit/b03485c42ecd7ade8761d63096d9f4967db9dada))
* **loop:** keep forge issue numbers out of Jira's assigned-key query ([0bd15ea](https://github.com/jacob-delgado/workflow/commit/0bd15ead0f5edb9691518e86bff44742c7648de9))
* **loop:** keep the owners when the forge can't classify one ([b50eb42](https://github.com/jacob-delgado/workflow/commit/b50eb422b358454211948673ba4db8d708b02284))
* **messaging:** fold in the review of the Slack user token ([ce5ef0c](https://github.com/jacob-delgado/workflow/commit/ce5ef0c022806857fa37fa66dddf9037e0db4f99))
* **store:** finish what an interrupted clean set aside ([4da216e](https://github.com/jacob-delgado/workflow/commit/4da216efdb004e4d2ebb0a8f2dc626e47fc4f276))
* **store:** keep the Slack workspace with each link, not the ID ([4af7aa4](https://github.com/jacob-delgado/workflow/commit/4af7aa4484b20b6ce313bf679517079ba9c63a07))
* **store:** match forge owners without regard to case ([94db065](https://github.com/jacob-delgado/workflow/commit/94db065529a9f43df7e50ce518013e4a24f91b49))
* **store:** remember the listed groups of a mixed choice ([60e5fdb](https://github.com/jacob-delgado/workflow/commit/60e5fdb78416d893fd57d7d1ace34c48f7b12199))
* **store:** wait out a lost WAL switch when opening a fresh file ([763e439](https://github.com/jacob-delgado/workflow/commit/763e4392a68679bbfd23ccf0fd6256913c122f97))
* **tui:** click the issue drawn above the not-read line ([c8ed19a](https://github.com/jacob-delgado/workflow/commit/c8ed19af80bda67a556ad131a752bb3c6ddd2349))
* **tui:** keep People's selection on the owner it was on ([d54465e](https://github.com/jacob-delgado/workflow/commit/d54465e8b988ed57e7c216b6eb22d71f6b6b3865))
* **tui:** key the owners' read to the composer that asked ([b05e49a](https://github.com/jacob-delgado/workflow/commit/b05e49a6ff63489ce4f39743e8629f16add740d4))
* **tui:** know your forge name before a pull request is found ([2709519](https://github.com/jacob-delgado/workflow/commit/2709519d89efd33407961b44547e70131678523f))
* **tui:** land directory reads beneath the owner picker ([939b588](https://github.com/jacob-delgado/workflow/commit/939b588db10b028e530305295abe97f309e71fe9))
* **tui:** land each tag read only in the overlay that asked ([cc30a4b](https://github.com/jacob-delgado/workflow/commit/cc30a4b7129d88be9b2c90dea6afb0e515d8799d))
* **tui:** leave tagging out when the directory has no credential ([4b36e09](https://github.com/jacob-delgado/workflow/commit/4b36e094dfa0333297e8da0c7e7a4d9ee331d35e))
* **tui:** link people from the channel a post goes to ([9604e4b](https://github.com/jacob-delgado/workflow/commit/9604e4b3052a417f342429e73d52b0e2b495f73a))
* **tui:** offer "when CI passes" once whom to tag is read ([eace98b](https://github.com/jacob-delgado/workflow/commit/eace98bb59e1513192dd5a12299278356937c024))
* **tui:** offer linking only an open pull request on the issue ([722c593](https://github.com/jacob-delgado/workflow/commit/722c593da35ab19808c44e3edfd64fad0208871b))
* **tui:** offer sort and filter on the Reviews pane only when they act ([19478ce](https://github.com/jacob-delgado/workflow/commit/19478ce581c7f1568485b3f379acbd887b620188))
* **tui:** read the branch once for every pane it feeds ([850085d](https://github.com/jacob-delgado/workflow/commit/850085d7ed4b6402e5f3330f09b315fb57d2e20e))
* **tui:** save each group change at once, only once read ([4e16ac9](https://github.com/jacob-delgado/workflow/commit/4e16ac96b2c055e4f886b26aa8176b66f1393a61))
* **tui:** say whom a dry run's announcement would tag ([cfad2cb](https://github.com/jacob-delgado/workflow/commit/cfad2cb57b4409bdf0878ba5f4fce16b64cbf20d))
* **tui:** separate a failed check's parts with the glyph set's mark ([84c5554](https://github.com/jacob-delgado/workflow/commit/84c555419f0f30d229c051491b0b767981367940))
* **tui:** show a link's description change only where it can be made ([4b4646a](https://github.com/jacob-delgado/workflow/commit/4b4646ae248eb206088e0479bf9985a9fde3621c))
* **tui:** stop a job log's scroll with its first line at the top ([5a687e5](https://github.com/jacob-delgado/workflow/commit/5a687e59f7299d1192dd11e5441b3137a24344c6))
* **tui:** tag the people a token can when it cannot read groups ([378234e](https://github.com/jacob-delgado/workflow/commit/378234e97b746d375fbd77e84714eabb281f790b))
* **web:** give the review queue an h2 and group by the raw repository ([06b8a68](https://github.com/jacob-delgado/workflow/commit/06b8a68982dbbd5fb7fe19c7ace99a8cc98a8147))
* **web:** hand focus to a failed check's log once it is read ([cb95dc1](https://github.com/jacob-delgado/workflow/commit/cb95dc17e74deacd0912cbb712ef3b138b26ae9d))
* **web:** keep a focused control clear of the content edge ([742503c](https://github.com/jacob-delgado/workflow/commit/742503c205ad30adb2f7948a15f958059259be28))
* **web:** keep focus and the new link through linking an issue ([829bf2d](https://github.com/jacob-delgado/workflow/commit/829bf2df0339c6c85e83118b5a9eda44542ac25d))
* **web:** keep focus when unpicking the review filter's last value ([91f8f94](https://github.com/jacob-delgado/workflow/commit/91f8f9431f9dcea69e97f7dcff035302d4d3d0ad))
* **web:** keep the review queue's sort and filter across sections ([f28e7f9](https://github.com/jacob-delgado/workflow/commit/f28e7f9f4d7705d320171e4a35dbe4e184b55081))
* **web:** link the issue that was previewed ([5e1da93](https://github.com/jacob-delgado/workflow/commit/5e1da9301149c54e98a47c1312df503335404ef3))
* **web:** mark the pull request by the forge's sigil when linking ([81590ea](https://github.com/jacob-delgado/workflow/commit/81590eaf27f421a57d581013f7c9215e35f1c6a6))
* **web:** open a forge issue "in the forge" until the forge is known ([aac8011](https://github.com/jacob-delgado/workflow/commit/aac8011fc261a8778110c1019fe9d968840ca76c))
* **web:** say an unread tracker as a status line, where the docs say ([84cde3c](https://github.com/jacob-delgado/workflow/commit/84cde3c4a329e4733d1066e73268a1b966c0d5fb))
* **webserver:** answer a check log's forge failure by its class ([92117f8](https://github.com/jacob-delgado/workflow/commit/92117f817cc51c6f98ca9f80d014cd358d4f0975))
* **webserver:** answer a link git cannot keep with 422, not 500 ([cf9aca9](https://github.com/jacob-delgado/workflow/commit/cf9aca98d07421d873a8382c3c49955663b6a8e2))
* **webserver:** document the link and log answers, unlink like link ([f0f27a3](https://github.com/jacob-delgado/workflow/commit/f0f27a33ace04995ad46f564fe6f8831491be09e))
* **webserver:** edit the pull request before keeping the issue link ([0ec87cf](https://github.com/jacob-delgado/workflow/commit/0ec87cfd9442bb7b1534d600af02fffee9904a4e))
* **webserver:** keep saved groups, and queue kept writes with a clean ([490ea58](https://github.com/jacob-delgado/workflow/commit/490ea58feec1abf12135fed2f5015ee37c86649c))
* **webserver:** tag by the live Slack mode, pinned to the preview ([2b067df](https://github.com/jacob-delgado/workflow/commit/2b067df657c68fa1c8374ded449a0288f48763c7))
* **web:** show a branch's forge issue link as [#42](https://github.com/jacob-delgado/workflow/issues/42) ([4709afa](https://github.com/jacob-delgado/workflow/commit/4709afa1927175e452d7d63c564e7d568a2f6e62))
* **web:** show people and groups gone after cleaning everything ([932fb09](https://github.com/jacob-delgado/workflow/commit/932fb09f72bec1baad0f75e7b9b7a9d4b194b393))
* **wiring:** bind no kept seams that would save nothing ([aef3884](https://github.com/jacob-delgado/workflow/commit/aef388473dd41dd75fb492ddf611fbb05f711755))
* **wiring:** follow messaging settings saved while running ([211b239](https://github.com/jacob-delgado/workflow/commit/211b239f4241f370052ed3e231e98ab44ad48192))
* **wiring:** hold each directory read ten minutes from its answer ([371ff22](https://github.com/jacob-delgado/workflow/commit/371ff222cf24dd114d98121dde9b2d3f44c2ead6))
* **wiring:** keep what GitLab said a bare name is for the session ([29b3dee](https://github.com/jacob-delgado/workflow/commit/29b3dee49ee40bb712c6bf9ee7ad2f21776b2fc9))
* **wiring:** wait as Slack asks while labeling members one by one ([394036b](https://github.com/jacob-delgado/workflow/commit/394036bf3c1370904918aaeee2126a6f7823266d))


### Performance

* **cli:** tell a missing tagging scope from auth.test, not users.list ([c112508](https://github.com/jacob-delgado/workflow/commit/c11250864ac60d21f89791fc77c704f477076748))
* **wiring:** read the Slack directory without a lock across calls ([09adb0b](https://github.com/jacob-delgado/workflow/commit/09adb0b1e5881db16f2ee7a2a208088bb570554a))


### Refactors

* **config:** drop the hints for configuration from older builds ([5dc0a10](https://github.com/jacob-delgado/workflow/commit/5dc0a10040f082957f5fa84ff5867715e1c2e85a))
* **convention:** an issue key knows its tracker ([89553c6](https://github.com/jacob-delgado/workflow/commit/89553c69ee9e74bfd7a2919049682834f1dc7c93))
* **loop:** find a branch's issue in one place ([08e762a](https://github.com/jacob-delgado/workflow/commit/08e762afa97f510f0e8b1da658a8ca706e3ed2bc))
* **seams:** read code owners at the base, of the changed paths ([1b0ef29](https://github.com/jacob-delgado/workflow/commit/1b0ef29814252f32c4a7bce8886ded17a914a8f2))
* **store:** give kept.db one schema and no migrations ([31754bc](https://github.com/jacob-delgado/workflow/commit/31754bc1dc945491345b42e2f616b917498fbcd2))
* **tui:** move owner linking out of people.go ([5088758](https://github.com/jacob-delgado/workflow/commit/5088758d1e97cdb3f0558bcfacdbd56fceffcb75))


### Documentation

* describe db-clean and the Local data area ([286296f](https://github.com/jacob-delgado/workflow/commit/286296ff02da44c0e54f1c01ccd7d0478281c970))
* describe reviewers proposed from CODEOWNERS ([e0d0e73](https://github.com/jacob-delgado/workflow/commit/e0d0e73455155ea436c42b67d4594a727b479cc0))
* describe tagging in the preview and People and groups ([53c74a5](https://github.com/jacob-delgado/workflow/commit/53c74a58e7863a7d9a5f13a8b4e424ca9d46617f))
* describe tags pinned to the preview and a partial clean ([69ebc16](https://github.com/jacob-delgado/workflow/commit/69ebc160a20213ccd997b0e7254d453a7947f1f5))
* name the Slack scopes tagging needs ([8ae35f6](https://github.com/jacob-delgado/workflow/commit/8ae35f6bffd9e2e97c3e304601140e6b771ab13b))
* narrow TRADE-17 to the keychain Settings fills ([f032f76](https://github.com/jacob-delgado/workflow/commit/f032f76b8f557b971f638808b96a007cc31ecee2))
* record the combined tracker's read-once settings as TRADE-24 ([9541f92](https://github.com/jacob-delgado/workflow/commit/9541f92fb09fd6f04090d9dd2034665fa615503b))


### Build & Packaging

* lower the condition-coverage floor to 91 ([5d3ad2d](https://github.com/jacob-delgado/workflow/commit/5d3ad2db198b4d67f74738042b4cac0a5727d224))
* **web:** bump brace-expansion to 5.0.12 for GHSA-q2hr-2g5m-vwhr ([89d34e6](https://github.com/jacob-delgado/workflow/commit/89d34e6ce0f2a15192035fe32ef6fa51f1f4a614))


### Tests

* **cli:** pin announce posting untagged when a scope is missing ([73e5e39](https://github.com/jacob-delgado/workflow/commit/73e5e391c01b3d2f671647ea11f93af85a822dc9))
* **cli:** pin announce posting untagged when the workspace is unknown ([39ff38d](https://github.com/jacob-delgado/workflow/commit/39ff38d5cdf089c7357168e794bd0ad1dd3f5e72))
* **cli:** pin what doctor --online says of an accepted Slack token ([4947767](https://github.com/jacob-delgado/workflow/commit/49477678c96c51e30fa6a2b9191ea24cfa3257bf))
* **cli:** pin what slack login keeps once Slack accepts it ([8a0711f](https://github.com/jacob-delgado/workflow/commit/8a0711f9f0d89035a4d7e587c7b50d9d4f1979f7))
* **cli:** pin workflow announce tagging in the workspace in use ([bb57a06](https://github.com/jacob-delgado/workflow/commit/bb57a063a46c49b9fbc7524536a59929668edc3b))
* **cli:** restore reading announce's owners against the pull's base ([ed8ebce](https://github.com/jacob-delgado/workflow/commit/ed8ebcea64fb43e76c0cbadb48ae0b50ce6e2a00))
* pin that nothing is tagged or kept without a Slack workspace ([401ab6c](https://github.com/jacob-delgado/workflow/commit/401ab6c126bc591dad71b26e81013f4a4fbdb2b5))
* **store:** pin a kept.db that is not a database ([e2a8e0b](https://github.com/jacob-delgado/workflow/commit/e2a8e0baa0edd8341a5d45671d4e095a46542cf1))
* **tui:** answer a failed CI read with no CI, as the real seam does ([4ac621a](https://github.com/jacob-delgado/workflow/commit/4ac621a5e8206458dc2188a699212ff738409aa3))
* **tui:** cover a team linked from People and a moved cursor ([e80c851](https://github.com/jacob-delgado/workflow/commit/e80c8510aa4fb06c207628f57717d5cea97ef336))
* **tui:** name the one clock step the tests take ([659b504](https://github.com/jacob-delgado/workflow/commit/659b50439ba97c97763cda06fe7c0d23ece63431))
* **tui:** pin the order the reviews filter offers its values in ([24c8c4b](https://github.com/jacob-delgado/workflow/commit/24c8c4b18d37e36bc21d17edd874f047e7519e00))
* **tui:** prove a dry run turns on no write that is not there ([85695d3](https://github.com/jacob-delgado/workflow/commit/85695d3f5a0328c7aaa90de613501e424fcdff38))
* **tui:** see every review queue condition both ways ([284cd82](https://github.com/jacob-delgado/workflow/commit/284cd8224b6e33d46c5021a87253c4d1a170c15d))
* **web:** drive refresh on switch, the reviews filter and issue linking ([3f1f1d6](https://github.com/jacob-delgado/workflow/commit/3f1f1d613da78ac4a8c490ef502f67f5ddc7eca5))
* **web:** let the 30-second staleness e2e fail when a read goes out ([e3923f1](https://github.com/jacob-delgado/workflow/commit/e3923f16b4b1e76f1d4b1965525e54379954dab2))
* **web:** wait for focus to land on what staging said ([242ca82](https://github.com/jacob-delgado/workflow/commit/242ca82fd6e44171dea6cb1b91943f4de587c953))
* **wiring:** pin an https Slack address and a cleartext name refused ([bf53c92](https://github.com/jacob-delgado/workflow/commit/bf53c92dd62c5b2238c9069a4d5e496fa4ce22ec))

## [0.4.3](https://github.com/jacob-delgado/workflow/compare/v0.4.2...v0.4.3) (2026-09-30)


### Features

* **doctor:** report glab beside gh ([f78fb5b](https://github.com/jacob-delgado/workflow/commit/f78fb5bcb6f34001725c219e1e67a95c3bcd961c))
* **tui:** filter the issue list by status and mark ([8112f53](https://github.com/jacob-delgado/workflow/commit/8112f538819d77c5f8238de1cb6a58cbf1c61e98))
* **web:** filter the issue list by status and mark ([b6393d6](https://github.com/jacob-delgado/workflow/commit/b6393d6762e7817acac119a9b4df8de62fe55b58))


### Bug Fixes

* **forge:** tell a refused token the same way in terminal and web ([a353637](https://github.com/jacob-delgado/workflow/commit/a35363761dd3605d152f205b0d0c5ed41985a481))
* **web:** apply forge settings saved in Settings at once ([6ad9e85](https://github.com/jacob-delgado/workflow/commit/6ad9e854bb21a67ad67d54161c41f1a0e9ddabd4))


### Documentation

* say what a refused GitHub token lacks and when doctor can tell ([89d687c](https://github.com/jacob-delgado/workflow/commit/89d687cfb9ac239647f98ad8f3652911d51b3111))


### CI

* lint the Markdown only when a pull request changes it ([a1c3faa](https://github.com/jacob-delgado/workflow/commit/a1c3faa2b32d4f63289294479a07306cd45dd36e))
* show each suite's test counts in the PR comment ([9d3a0a5](https://github.com/jacob-delgado/workflow/commit/9d3a0a5ba49372f74cdd778e7657eb93654f86ea))

## [0.4.2](https://github.com/jacob-delgado/workflow/compare/v0.4.1...v0.4.2) (2026-09-30)


### Bug Fixes

* **tui:** say where to set a token for the repository's own forge ([2707dc2](https://github.com/jacob-delgado/workflow/commit/2707dc20efa053d3067eb8968e7f7592af98d3f3))
* **web:** embed the built web app in every build ([79d34a2](https://github.com/jacob-delgado/workflow/commit/79d34a2d06edddc926f2f47c97867796918805b6))
* **web:** tell GitHub and GitLab users how to supply a forge token ([35f9308](https://github.com/jacob-delgado/workflow/commit/35f93085ff531c9c1fd45f7de3e19e0b5163501b))


### Documentation

* say go install carries the web interface ([e74d9e1](https://github.com/jacob-delgado/workflow/commit/e74d9e1644f587a69ddbef59bf8e0bf4725bf732))
* say what the forge token must be allowed on GitHub and GitLab ([480cf84](https://github.com/jacob-delgado/workflow/commit/480cf84dc90bb201fdcafa23d557598440bfaf68))


### Build & Packaging

* Bump brace-expansion from 1.1.18 to 1.1.21 in /web ([e57c716](https://github.com/jacob-delgado/workflow/commit/e57c716c23863d0eda94f632fa1eeff20a6be328))
* Bump the web group in /web with 7 updates ([df4d130](https://github.com/jacob-delgado/workflow/commit/df4d130b19411242333dd189a9b65b6a7354b8ca))
* **web:** fail when the committed web bundle is stale ([aa15a4d](https://github.com/jacob-delgado/workflow/commit/aa15a4d068da9e76129e6ae5db01e965890d5efb))

## [0.4.1](https://github.com/jacob-delgado/workflow/compare/v0.4.0...v0.4.1) (2026-09-29)


### Features

* **api:** read and change tasks over the loopback API ([6beb8b4](https://github.com/jacob-delgado/workflow/commit/6beb8b40c591daa85b5a5080a92b69dd1a69793e))
* **cli:** report Taskwarrior in doctor's tooling ([0509e76](https://github.com/jacob-delgado/workflow/commit/0509e7674aa2012a885f4fb38e9421223f53fad9))
* **config:** add the taskwarrior section ([e536dac](https://github.com/jacob-delgado/workflow/commit/e536dac8c26f118ee82410c95c8a950051140de0))
* **jira:** scope a view and a key set to the credential's user ([10fa26a](https://github.com/jacob-delgado/workflow/commit/10fa26a059c60a35aa412231b002ec9164ca9589))
* **proc:** bound Capture and read a program's exit ([da4cb3b](https://github.com/jacob-delgado/workflow/commit/da4cb3bc1ec259b6bd6512508a0a069fc28b6e61))
* **seams:** declare the Taskwarrior bundle ([53ab3b3](https://github.com/jacob-delgado/workflow/commit/53ab3b3750967e6569bd2b588d622d0919103c1c))
* **store:** discard a store whose schema version is not this build's ([255da9f](https://github.com/jacob-delgado/workflow/commit/255da9fc7bbf6eb35fda5ce1653050cd802eb407))
* **taskwarrior:** drive the task program ([7a71dc9](https://github.com/jacob-delgado/workflow/commit/7a71dc94935c62f087dc31699bdbbda07f50a4c9))
* **tui:** act on a task in Taskwarrior's own words ([1caa1b5](https://github.com/jacob-delgado/workflow/commit/1caa1b507d4eeeb87056a811334ff9c4439ddc2b))
* **tui:** hold every Taskwarrior write back under dry run ([183a951](https://github.com/jacob-delgado/workflow/commit/183a95142a9c2f1d5ef797494176cd063c8fd611))
* **tui:** list your Taskwarrior tasks in a seventh pane ([f39a18e](https://github.com/jacob-delgado/workflow/commit/f39a18e304a9fd78bcec26f1658efd4a750ca4b0))
* **tui:** offer the task change at each moment of the loop ([75288c4](https://github.com/jacob-delgado/workflow/commit/75288c4a36131bab4be34b7d27ad91fe00789ae8))
* **tui:** scope views and the task switcher to your issues ([052cc76](https://github.com/jacob-delgado/workflow/commit/052cc76977a4c29efc4778119e69818ec7494433))
* **web:** a Tasks section, the issue's tasks and the active task ([7c8aa23](https://github.com/jacob-delgado/workflow/commit/7c8aa2323957c16bfd1dcd948f12e17c4dfcbc1d))
* **wiring:** find Taskwarrior and offer its seams ([aabf248](https://github.com/jacob-delgado/workflow/commit/aabf248ca8c52a5ed2a3c6301b17273d46d65533))


### Bug Fixes

* **config:** say which PATH entries the taskwarrior default tries ([162413f](https://github.com/jacob-delgado/workflow/commit/162413fc2b9d11e66ab8c618b88f98f0ab417a8e))
* **taskwarrior:** pass over relative PATH entries when finding task ([9d664aa](https://github.com/jacob-delgado/workflow/commit/9d664aa36f1d7ae0f051db9c0cca0ecc5e19d392))
* **taskwarrior:** read a quoted Windows PATH entry when finding task ([cf60178](https://github.com/jacob-delgado/workflow/commit/cf60178eb1860dd51945f2457b10f9ad784cd34c))


### Documentation

* describe the Tasks pane, section and configuration ([22c8ac6](https://github.com/jacob-delgado/workflow/commit/22c8ac6ae82fb62a9784a7a9e4f0473a6e71f999))
* replace the store migration rule with the schema-version rule ([9719ac9](https://github.com/jacob-delgado/workflow/commit/9719ac9f55a41a1c36c8ebbe3b2a8711867ce6a1))


### Tests

* **taskwarrior:** give the PATH walk's tests a file of their own ([ef511cc](https://github.com/jacob-delgado/workflow/commit/ef511cc75e49a11984126a9dd30ef9bb0dca08f0))

## [0.4.0](https://github.com/jacob-delgado/workflow/compare/v0.3.1...v0.4.0) (2026-09-28)


### ⚠ BREAKING CHANGES

* **cli:** workflow --web now serves http://127.0.0.1:13579 by default instead of port 7000; pass --port 7000 to keep the old address.

### Features

* **api:** carry the effective commit types in the snapshot ([1987889](https://github.com/jacob-delgado/workflow/commit/198788904954c8b02b8083df28a6021830944b85))
* **api:** carry the pull request's state on the wire ([192de60](https://github.com/jacob-delgado/workflow/commit/192de603ade326d24b7f505f20ca61fe20d7a3eb))
* **cli:** choose the web interface's port with --port ([5eff9a5](https://github.com/jacob-delgado/workflow/commit/5eff9a527c6931b798027ba8fd9f2434ddabfab0))
* **cli:** serve the web interface on port 13579 by default ([464e23e](https://github.com/jacob-delgado/workflow/commit/464e23ea46678afe05669343164e9cf43375d470))


### Bug Fixes

* **api:** bound ui.comments_shown and ui.color in the spec ([599991b](https://github.com/jacob-delgado/workflow/commit/599991b1695d0f576a9c9e028f14cd5192db99fc))
* **cli:** doctor counts a rate limit as unreachable ([3891c9e](https://github.com/jacob-delgado/workflow/commit/3891c9e383208aa6ec83305f58f094aa13ee8c5b))
* **cli:** doctor counts a refused redirect as unreachable ([ccc6875](https://github.com/jacob-delgado/workflow/commit/ccc68755ae7865b0702805337c3bd0795dfbfb37))
* **cli:** exit 3 when no remote names the forge ([1a01a0a](https://github.com/jacob-delgado/workflow/commit/1a01a0a637bd07da3ab7fd979b82ff42b3b03c42))
* **cli:** exit 3 when the interface refuses its ui.keys map ([37011cf](https://github.com/jacob-delgado/workflow/commit/37011cfbbb8ede695c068c0626836d32dafab6d9))
* **cli:** exit 3 when the origin's forge cannot be identified ([72db184](https://github.com/jacob-delgado/workflow/commit/72db184b7ae938825489e43027a620aa13d42484))
* **cli:** give a refused redirect the unreachable exit status ([d1e40f7](https://github.com/jacob-delgado/workflow/commit/d1e40f7f31b03b3b0939384eba9af961a63e0ff2))
* **cli:** hand a dry-run interface no store ([78ad6e4](https://github.com/jacob-delgado/workflow/commit/78ad6e41f3b73824ec5c4f16439bb5280018d437))
* **cli:** name a missing git on doctor's repository line ([be5c762](https://github.com/jacob-delgado/workflow/commit/be5c762b495afd99b709dc3da2a80e9c3f2dcc2d))
* **cli:** offer no review move for a forge issue number ([85eaa90](https://github.com/jacob-delgado/workflow/commit/85eaa90eec99f10d61196a6f901737c73e8d134a))
* **cli:** preview an announced moment under announce --yes --dry-run ([f0e238a](https://github.com/jacob-delgado/workflow/commit/f0e238a23f1e7dff94e979f92944cc24a61f8ca0))
* **cli:** say when doctor cannot read the configuration ([ed25290](https://github.com/jacob-delgado/workflow/commit/ed25290fcdb901844fcbd17cbfb1d4d8c5a7490b))
* **cli:** say which offers pr --dry-run would make once open ([319ab05](https://github.com/jacob-delgado/workflow/commit/319ab055d335ec1b5357a80c0ffedcc21ab402da))
* **cli:** show a remembered announcement in status's last stage ([4596438](https://github.com/jacob-delgado/workflow/commit/45964384bd6387be011dd392d0e15baef7d9f859))
* **cli:** skip doctor --json --online checks when the file did not load ([28a19ae](https://github.com/jacob-delgado/workflow/commit/28a19ae3aaee9084c3f57ed4c4ebfc2abe77de64))
* **cli:** warn when the request log could not be written ([bf8b287](https://github.com/jacob-delgado/workflow/commit/bf8b28787bd0d5130bb9aa1086bffe1618d55bcb))
* **cli:** write the web server's unexpected failures to stderr ([2b0e49f](https://github.com/jacob-delgado/workflow/commit/2b0e49f8153088b575ec79fed7ba6ae6abd8a435))
* **config:** hold jira.headers values as Secrets ([f594f9c](https://github.com/jacob-delgado/workflow/commit/f594f9ccf4d10eae03014090bfda7e9f502a0a95))
* **config:** leave Jira out of Missing when the forge is the tracker ([b4fd025](https://github.com/jacob-delgado/workflow/commit/b4fd025970e7853e1e8fae6d3f493d1a5deffdf5))
* **config:** mask a Secret under %#v ([777c178](https://github.com/jacob-delgado/workflow/commit/777c1784c218269f9350b91d6ff2b0878e149482))
* **config:** refuse a Jira base URL with a login, as the client does ([6e6606b](https://github.com/jacob-delgado/workflow/commit/6e6606b57bae259a987739f837e3f38fefcd871e))
* **config:** refuse a negative ui.comments_shown ([384cbfe](https://github.com/jacob-delgado/workflow/commit/384cbfeca4e4651cb818ad8dff9f72dfbdb66f84))
* **config:** refuse an unknown messaging.kind ([a28e3b9](https://github.com/jacob-delgado/workflow/commit/a28e3b92a86a95e6a8b01c3f4d5e4a95def3885e))
* **config:** refuse an unknown ui.color ([cac1aca](https://github.com/jacob-delgado/workflow/commit/cac1acaabf0b5c1b71a9772c4b6a298efd94bf05))
* **config:** replace the config file by rename, never truncate it ([66c7b29](https://github.com/jacob-delgado/workflow/commit/66c7b294b5e303a2519a36883949dca0c1023160))
* **convention:** find a key one separator after a rejected token ([f0f5603](https://github.com/jacob-delgado/workflow/commit/f0f5603e0365bbf4a41842d94d539f9984306fbc))
* **convention:** read the scope after a commit type with a digit ([989681c](https://github.com/jacob-delgado/workflow/commit/989681c012d4663131750162958ff665c576b240))
* **convention:** refuse a commit description that spans lines ([bc8bc99](https://github.com/jacob-delgado/workflow/commit/bc8bc9912dd53322c677f85422427fb840411403))
* **doctor:** ask the forge through gh or glab when forge.cli is set ([ddaf56d](https://github.com/jacob-delgado/workflow/commit/ddaf56d9d078937183d58f487ef33b243228dc29))
* **doctor:** fail --json on set-but-invalid values as the prose does ([955f101](https://github.com/jacob-delgado/workflow/commit/955f101b3219c451dc67254ee7d3a37af0fccb55))
* **doctor:** leave a Jira address with a login unchecked, as invalid ([44ed611](https://github.com/jacob-delgado/workflow/commit/44ed611654f61f30e88e2fd1ae44bc7a8efc6e8b))
* **doctor:** name the tracker in effect, Jira or the forge's issues ([6194af8](https://github.com/jacob-delgado/workflow/commit/6194af8acd963036b30293a07b097fa08c23ac79))
* **doctor:** report a check doctor could not make as unchecked ([be8f6a0](https://github.com/jacob-delgado/workflow/commit/be8f6a0c5dad80b9f349bb92ee8f8eec0d377422))
* **doctor:** report a missing credential as missing, not rejected ([8e06564](https://github.com/jacob-delgado/workflow/commit/8e065645d7e37a3d39bddf667b4a681df6cc5a1c))
* **doctor:** say the offline report leaves a webhook unchecked ([450d86e](https://github.com/jacob-delgado/workflow/commit/450d86e8d16fca6bf2a81adba90135d18623dae9))
* **doctor:** skip the Jira check when the forge is the tracker ([6374076](https://github.com/jacob-delgado/workflow/commit/6374076b5fdafd658d730a3dc1f2e47e0c8b21da))
* **forge:** keep a 403's own reason and tell a rate limit apart ([f919f95](https://github.com/jacob-delgado/workflow/commit/f919f9567c952edf23cd6bdd819853fbc04bcaaf))
* **forge:** list every assigned issue, not the first hundred ([16910c9](https://github.com/jacob-delgado/workflow/commit/16910c97a7743694afb01aeaaaf8175788fedee7))
* **forge:** list every review request, not the first hundred ([f9e4315](https://github.com/jacob-delgado/workflow/commit/f9e431599ce57b1795acb5b1feea4a0b2be7c4ba))
* **forge:** mask a Token under %#v ([422734d](https://github.com/jacob-delgado/workflow/commit/422734d079b6690688478b2cfc29bcded43114b9))
* **forge:** name the tenant's own sources for a ghe.com host ([71fe6c6](https://github.com/jacob-delgado/workflow/commit/71fe6c68803257191fd5dd6ddf61c8a5c27bebed))
* **forge:** re-run failed runs past the first page ([27b49a2](https://github.com/jacob-delgado/workflow/commit/27b49a2795d54fe09387d27b3562e8b5771a3485))
* **forge:** read a ghe.com remote as GitHub ([b3b70e8](https://github.com/jacob-delgado/workflow/commit/b3b70e84211be90648af13a12fdfbb37bf7504bd))
* **forge:** read every review on a pull request, not the first page ([7985eeb](https://github.com/jacob-delgado/workflow/commit/7985eeb3de0c0b8fa64a8f6667da3a7b4468a066))
* **forge:** read whether an issue is closed on both forges ([9520a80](https://github.com/jacob-delgado/workflow/commit/9520a80019ba309b37bee149e3676b96abe9bb56))
* **gitrepo:** count a branch on its push remote as pushed ([0edcb22](https://github.com/jacob-delgado/workflow/commit/0edcb22021cbfd9277f9477120eee1ef4c1a17f3))
* **gitrepo:** report a git read that timed out as a timeout ([8477893](https://github.com/jacob-delgado/workflow/commit/847789340fd3e4cf30c75aa065e1b7bdcea40c3d))
* **gitrepo:** report a missing git rather than no repository ([40d4e1b](https://github.com/jacob-delgado/workflow/commit/40d4e1bd436f5011aa82450b495caff9f8de6fdb))
* **gitrepo:** report a non-repository when listing branches ([69301d9](https://github.com/jacob-delgado/workflow/commit/69301d9f8fac30ea5191bb82c9caf6d463ce7e91))
* **hooks:** remove a hook file that cannot be written whole ([b6a31cb](https://github.com/jacob-delgado/workflow/commit/b6a31cb47effcb9bb96b1250cce6871935644171))
* **hooks:** stop Verbatim's header claiming hooks became jobs ([12344b0](https://github.com/jacob-delgado/workflow/commit/12344b0539dc9b8885b821e8e0bae5e6779b5963))
* **jira:** answer an explained 403 as forbidden too ([42912fb](https://github.com/jacob-delgado/workflow/commit/42912fb67a753343d9646a6f6dc61534c41c4c9a))
* **keychain:** hand security the secret on its input, not its arguments ([0029153](https://github.com/jacob-delgado/workflow/commit/002915347192a73975108cc59084a5284c9f8c64))
* **keychain:** name the account when storing the Jira token ([aa3cd30](https://github.com/jacob-delgado/workflow/commit/aa3cd30d095d36c05ab5763e580e34411b00c23e))
* **loop:** remember a commit scope through one trimmed rule ([86eac5b](https://github.com/jacob-delgado/workflow/commit/86eac5b3ebac13083e1538591b1129c01ad92598))
* **messaging:** count any 2xx answer as a delivered post ([173c259](https://github.com/jacob-delgado/workflow/commit/173c2591b6089b28a6fd8e096f69dc977efe29d0))
* **messaging:** treat Slack's auth errors on a post as rejected ([713db72](https://github.com/jacob-delgado/workflow/commit/713db72363f8645ce2b6d554b257571675e3f077))
* **proc:** report a run past its bound as a timeout ([c79469a](https://github.com/jacob-delgado/workflow/commit/c79469a4dbfa1041a9dd2043acd662d778504f31))
* **progress:** read a merged pull request's review as done ([b4872e3](https://github.com/jacob-delgado/workflow/commit/b4872e310f88032a432a5b0d8e7ab95740b437af))
* **scripts:** claim only a go statement in the goroutine gate ([d952e1b](https://github.com/jacob-delgado/workflow/commit/d952e1b8d9d6ae8b30f005afb284cf79e5173f4c))
* **scripts:** stop the gobco report claiming it runs in short mode ([2449dc0](https://github.com/jacob-delgado/workflow/commit/2449dc03ad5b3c08ec5654b4f36b86900e128240))
* **store:** narrow an existing store directory to owner-only ([50855ce](https://github.com/jacob-delgado/workflow/commit/50855ce7339439775a847b77be9baa451a9c71f1))
* **store:** read an existing store under a dry run, never create one ([ceaeddf](https://github.com/jacob-delgado/workflow/commit/ceaeddfa888c2829d1da33180a5beb6ed6a18b7e))
* **tui:** balance the help's two columns ([41d8c13](https://github.com/jacob-delgado/workflow/commit/41d8c13240f7594a0632d9a08f05904ae9911d68))
* **tui:** clamp the help's scroll to its key list ([864d790](https://github.com/jacob-delgado/workflow/commit/864d790a35147aaeb9f2ace8252b9ef40f74ffb8))
* **tui:** close the lefthook offer when the install fails after a write ([8610184](https://github.com/jacob-delgado/workflow/commit/86101848c2ec6698a3b92c81371689378ba4b24b))
* **tui:** drop a search answer for a view no longer shown ([e0e38ed](https://github.com/jacob-delgado/workflow/commit/e0e38ed30d837441fb76eeb8bfe2edf6ef620c0f))
* **tui:** hold back the store under --dry-run ([ffdece1](https://github.com/jacob-delgado/workflow/commit/ffdece143e568cbd0bde3676de169e58f26fc678))
* **tui:** ignore a click on the issue being read below 80 columns ([5712b04](https://github.com/jacob-delgado/workflow/commit/5712b04a89fcda7380404227f46c4d2f2dbff7a6))
* **tui:** keep the listed issues when a refresh fails ([1794897](https://github.com/jacob-delgado/workflow/commit/1794897c050b8211bfbaa31586bfae2da46fb8ba))
* **tui:** keep the review and its CI poll through a failed find ([9aa0fa8](https://github.com/jacob-delgado/workflow/commit/9aa0fa8e5a3abe9ab38bdf4faa36bd469de319bd))
* **tui:** let the wheel move an overlay's list under rebound keys ([255ed95](https://github.com/jacob-delgado/workflow/commit/255ed957c25c963c2a28a4f5f5f9f36b185ec04e))
* **tui:** make w do nothing where the preview does not offer it ([8bb3322](https://github.com/jacob-delgado/workflow/commit/8bb3322fc0a9de1d24a85a51e7a1ac740b43e9cf))
* **tui:** name no key in the nothing-staged sentence ([27971a1](https://github.com/jacob-delgado/workflow/commit/27971a107fe01f5988acd001af6d14dbd7912fa3))
* **tui:** name the offers a dry-run pull request would make ([61ea8e0](https://github.com/jacob-delgado/workflow/commit/61ea8e0e40dc30616c04dc96e4d51a909324596a))
* **tui:** name the remote a push goes to in its preview ([5f8a257](https://github.com/jacob-delgado/workflow/commit/5f8a257a5d84d6f0cb47d588e4ac2acbc5041e1d))
* **tui:** offer a failure printed before the run's kept tail ([4321edc](https://github.com/jacob-delgado/workflow/commit/4321edc8176502670687cbf6d6f1d2137725eab8))
* **tui:** offer a new pull request once the last one merged ([376530e](https://github.com/jacob-delgado/workflow/commit/376530eb156b4d60161d20f166bd60549f04ab47))
* **tui:** offer b only in a repository and let r re-read the branch ([4693b81](https://github.com/jacob-delgado/workflow/commit/4693b814a621fc456b9bfc0b7920eda2c8d63040))
* **tui:** offer n when the pull request find fails ([7a5d26d](https://github.com/jacob-delgado/workflow/commit/7a5d26d623794d5690b9e85727d997de706b1694))
* **tui:** offer no Jira link for a forge issue number ([68c09a9](https://github.com/jacob-delgado/workflow/commit/68c09a9ad40810d6c8b17be66f7156dc92fc37a4))
* **tui:** offer stage all only when a file is left to stage ([4005b3a](https://github.com/jacob-delgado/workflow/commit/4005b3ac8f267705d3da96493e4e957598104c4a))
* **tui:** offer the help's scroll keys when it is taller than the pane ([0373552](https://github.com/jacob-delgado/workflow/commit/0373552fc28fd18290ec82ba181f3a144805019e))
* **tui:** open the merge picker before its methods are read ([cebe05a](https://github.com/jacob-delgado/workflow/commit/cebe05abb92df2f2ff3678ef27059c0470c9d0a7))
* **tui:** re-read the git hooks when the Commits pane refreshes ([4f89c10](https://github.com/jacob-delgado/workflow/commit/4f89c10e4e5fbabc2e335ed0347aa4937940e76e))
* **tui:** read an unlisted issue for the pull request's title ([870b79c](https://github.com/jacob-delgado/workflow/commit/870b79c3840695805ac7eb8d2fac213ab7f29acc))
* **tui:** read the Branch pane's pushed state against the push remote ([a58a1ae](https://github.com/jacob-delgado/workflow/commit/a58a1ae92ac6787ae27a0b5c6677ed0d3726ea65))
* **tui:** read the working tree when a task switch is chosen ([ac08365](https://github.com/jacob-delgado/workflow/commit/ac08365feee4dd33955f600eb974ac56be686e55))
* **tui:** refuse a ui.keys override of jump-to-pane ([2eceeb7](https://github.com/jacob-delgado/workflow/commit/2eceeb78354a805c7a0b4b6f924088bbc33ce755))
* **tui:** say a dry-run branch would also be switched to ([8776ea4](https://github.com/jacob-delgado/workflow/commit/8776ea44984aca546977359eae18c656dd51910c))
* **tui:** say what doctor checks in the messaging setup hint ([498dd31](https://github.com/jacob-delgado/workflow/commit/498dd3191a908adc7a212a21bf6b7d7e33be1d5e))
* **tui:** show a failed listing in the review-status offer ([044591b](https://github.com/jacob-delgado/workflow/commit/044591b072ce4eb11fef33ebf5461d4e0f91c5d2))
* **tui:** show git's reason when the fetch before branching fails ([f5ea004](https://github.com/jacob-delgado/workflow/commit/f5ea004f5374759743fde3520413ed284be42589))
* **tui:** stop offering lefthook once lefthook.yml is written ([4fdd338](https://github.com/jacob-delgado/workflow/commit/4fdd338f1b86c341064a7ac373598f0d6ef9044d))
* **tui:** word a forge refusal as a permission answer ([db06e73](https://github.com/jacob-delgado/workflow/commit/db06e73b571f84f9c63912c4f7635225d9977d1d))
* **tui:** word a program that timed out ([0c3cc9d](https://github.com/jacob-delgado/workflow/commit/0c3cc9d0e80e7f7f56cf3d676b2fb0a1c4bbe9db))
* **tui:** word the outside-a-repository panes through the failure table ([56ae5b2](https://github.com/jacob-delgado/workflow/commit/56ae5b2097884d22250d19c84d6e8ba9a6a4e8aa))
* **web:** give every primary button one size ([4e7e4f4](https://github.com/jacob-delgado/workflow/commit/4e7e4f4711c2ce9dbae0b5bc61f8704cb2616e51))
* **web:** hold the issue detail's Retry busy so it keeps its focus ([89b8fe9](https://github.com/jacob-delgado/workflow/commit/89b8fe9c6911ddb6d7243bcabae7501f659c4896))
* **web:** keep an unread issue's Retry in place while it reads again ([da4bfb0](https://github.com/jacob-delgado/workflow/commit/da4bfb098dbd9ae86f3fb8874d2864e37a358543))
* **web:** name no commit the server could not confirm ([5ce525a](https://github.com/jacob-delgado/workflow/commit/5ce525ae857f1ed1139ffb26b994df274b426b42))
* **web:** offer a push for a branch tracked off its push remote ([aa4bd1d](https://github.com/jacob-delgado/workflow/commit/aa4bd1d638cd1c9d471851bebe7c6bf97881d721))
* **web:** read changes requested as failed and no CI as in flight ([90a01ce](https://github.com/jacob-delgado/workflow/commit/90a01ced65e92243140e6258f97417d6ec05e739))
* **web:** read the changes stage done on a commit, as progress does ([986be30](https://github.com/jacob-delgado/workflow/commit/986be3074ed9eb0feef9a0c774e5a4a3d1fde429))
* **web:** send the previewed announcement with the post ([632fdf6](https://github.com/jacob-delgado/workflow/commit/632fdf6ed784c33356d10c73812be7a645f77b96))
* **webserver:** answer a checkout that landed when its re-read fails ([539037d](https://github.com/jacob-delgado/workflow/commit/539037d347ca676d49dd1404ed35e5fca6ebae08))
* **webserver:** answer a commit that landed when its re-read fails ([a86ad62](https://github.com/jacob-delgado/workflow/commit/a86ad6215bf9e1f3697224f2fcc3d2d340792465))
* **webserver:** answer a created branch when its re-read fails ([9fc3d54](https://github.com/jacob-delgado/workflow/commit/9fc3d54c633bc0f92194c50c3c916419cecd7ef7))
* **webserver:** answer a failed git read through fault on every write ([d2b2156](https://github.com/jacob-delgado/workflow/commit/d2b215631fa6f7ee15830806d55a1388058e01e5))
* **webserver:** answer a forge refusal as a permission problem ([e04baf6](https://github.com/jacob-delgado/workflow/commit/e04baf6bbbae000cf55136456bb839426a6ee4f1))
* **webserver:** answer a server outside a repository as a conflict ([99b8c87](https://github.com/jacob-delgado/workflow/commit/99b8c8750e4c917af9d9bf229c3a8a2672bcd77c))
* **webserver:** answer a wrong method on a known path with 405 ([d600dc8](https://github.com/jacob-delgado/workflow/commit/d600dc8fc316e7b2550168211aaa4337ef4647d9))
* **webserver:** ask no CI about a pull request that is not open ([4b8de7d](https://github.com/jacob-delgado/workflow/commit/4b8de7d6cab7c5237dff0413e851a3b908b0182a))
* **webserver:** ask the forge who the author is once per server ([1527247](https://github.com/jacob-delgado/workflow/commit/15272479bd41110952ff6b65d001713b4d112620))
* **webserver:** classify a failed pull request open through fault ([d951132](https://github.com/jacob-delgado/workflow/commit/d9511322369586686d17e564afd41d7b25a2de71))
* **webserver:** keep a commit's start failure off the wire ([e776eeb](https://github.com/jacob-delgado/workflow/commit/e776eebcd5c0709e8f40f0c78703d2dad1085a80))
* **webserver:** keep a push's start failure off the wire ([8d6759f](https://github.com/jacob-delgado/workflow/commit/8d6759ff55c99eb05ba05851d17cbf5f8fe80b20))
* **webserver:** keep the remote's address out of a failed push ([31ec1d6](https://github.com/jacob-delgado/workflow/commit/31ec1d669bd9ccb690eec0d1da51a1f7079f30a0))
* **webserver:** name the open pull request when one is already open ([a2f6173](https://github.com/jacob-delgado/workflow/commit/a2f61730be1163b7e4690ec263c2beec404d2000))
* **webserver:** name what is invalid in a refused configuration ([8e7e9b9](https://github.com/jacob-delgado/workflow/commit/8e7e9b959fc79219c3b577ee3a47ccefa2b358c9))
* **webserver:** push a branch whose upstream is off its push remote ([8df9d48](https://github.com/jacob-delgado/workflow/commit/8df9d485fc6b5e6b186d8a51b3d362255089c0ac))
* **webserver:** read the branch once per stream frame ([e6ffd5d](https://github.com/jacob-delgado/workflow/commit/e6ffd5d6f8ad031ba27fa2edb049caee1f058047))
* **webserver:** read the forge once per CI interval for every stream ([19e6329](https://github.com/jacob-delgado/workflow/commit/19e63291874c72a6c9cc5420d5f00e28458f44e4))
* **webserver:** refuse a keymap the terminal would refuse ([e8408ae](https://github.com/jacob-delgado/workflow/commit/e8408aea7a2f73f261399ac53f9aac63dd14fa9c))
* **webserver:** refuse an announcement that changed since its preview ([3c6bd18](https://github.com/jacob-delgado/workflow/commit/3c6bd188d6b06a263a52c85ca63e9186dd315443))
* **webserver:** report an unexpected failure's cause to a seam ([a6f0a33](https://github.com/jacob-delgado/workflow/commit/a6f0a332625d62f50ab11ca3bdd1e3aa282ea89f))
* **webserver:** tell a failed branch read apart from nothing to open ([7f128fd](https://github.com/jacob-delgado/workflow/commit/7f128fd8c05788414cc8f4f98d56285b706b4e1e))
* **webserver:** tell a failed read apart from nothing to announce ([7e5f035](https://github.com/jacob-delgado/workflow/commit/7e5f035461929e18e0d34cc7425f4586f5270701))
* **web:** show a merged pull request as merged, not ready ([a16dadb](https://github.com/jacob-delgado/workflow/commit/a16dadbdb3ac63085d003ba3e199b669eb3e26e6))
* **wiring:** answer unconfigured when the hooks dir is unreadable ([09c4a65](https://github.com/jacob-delgado/workflow/commit/09c4a65b0a0ee03e44f4b82fa82f73a896b1f047))
* **wiring:** connect to the forge once for the tracker and forge seams ([62a1cce](https://github.com/jacob-delgado/workflow/commit/62a1cce21ea127e8565cc1d124d19b2c3c254c25))
* **wiring:** keep the request log's first write error ([06bb4cf](https://github.com/jacob-delgado/workflow/commit/06bb4cff503d272f26d34b390a8ee1f826556afa))
* **wiring:** open a browser on Windows without going through cmd ([61c36b5](https://github.com/jacob-delgado/workflow/commit/61c36b5d1c962739fb6f06ae587cd67b8b866809))
* **wiring:** pull for a finish without the quick-read bound ([d625a4a](https://github.com/jacob-delgado/workflow/commit/d625a4a3fee9355f299481bf119200d3077e13d1))
* **wiring:** read no Jira issue for a branch named by a forge number ([e153fd3](https://github.com/jacob-delgado/workflow/commit/e153fd3def4d1bf8ae9b09031fd314a20a0a327b))
* **wiring:** reject a non-numeric issue key before connecting ([883460c](https://github.com/jacob-delgado/workflow/commit/883460c5ce49072650ad5dc13d1553644df795eb))
* **wiring:** resolve the Jira token on first use ([d687d77](https://github.com/jacob-delgado/workflow/commit/d687d77b44316c46717cd9928ac719a3ecc9bf60))
* **wiring:** resolve the messaging token on first use ([d968b65](https://github.com/jacob-delgado/workflow/commit/d968b650a254f0d4672d4fd19818d067f4b0ea0a))
* **wiring:** show a closed forge issue as closed ([1458c0b](https://github.com/jacob-delgado/workflow/commit/1458c0b6284a8d7975de8b10e7c687a2657af437))


### Performance

* **editor:** resolve every printed place in one walk ([23c3707](https://github.com/jacob-delgado/workflow/commit/23c37078f0d5cf263eb0ddd0eb8748103610106c))
* **tui:** resolve a failed run's places off the update loop ([22adb9f](https://github.com/jacob-delgado/workflow/commit/22adb9f9c395007808960a6e3b4291bcc5bfb98a))


### Refactors

* **cli:** decide NO_COLOR before the interface runs ([263b58b](https://github.com/jacob-delgado/workflow/commit/263b58b7020ee0fd7f91435a5b2c83eab1a26253))
* **cli:** look up stage and CI words in maps, not switches ([be8d187](https://github.com/jacob-delgado/workflow/commit/be8d187c03c265bb0ef590880867b19ee77b572a))
* **cli:** read the home directory for configuration in one place ([79ab337](https://github.com/jacob-delgado/workflow/commit/79ab33776a59ca08e44a1095bb2915be66526ce1))
* **config:** move the ui section into ui.go ([9d71b88](https://github.com/jacob-delgado/workflow/commit/9d71b88b2df5fff04005cd0c0afdbd8b046839f7))
* **config:** read an empty messaging kind in the default arm ([1938d88](https://github.com/jacob-delgado/workflow/commit/1938d88f3cc36efe7b6d9a8329c58f70df229141))
* **convention:** drop the built-in wrappers nothing calls ([005cc6a](https://github.com/jacob-delgado/workflow/commit/005cc6a62e756586450b6e595e0184eac61ae5c0))
* **doctor:** move the credential checks into their own file ([4df6e71](https://github.com/jacob-delgado/workflow/commit/4df6e71b34f396ad368b4e78eb6803c7bad24004))
* **doctor:** range the external tools in one function ([96c779c](https://github.com/jacob-delgado/workflow/commit/96c779c546403faedcd5eebb9f870b9aef495d7b))
* drop the clients' aliases of httpx's sentinels ([bc4c845](https://github.com/jacob-delgado/workflow/commit/bc4c8452df29171824504473ed415289112cf862))
* **editor:** move the standup compose path out of main ([81b20dc](https://github.com/jacob-delgado/workflow/commit/81b20dcb131e3f489acbf54af1caabd47e17879b))
* **forge:** build pull and merge request paths in one helper each ([f3d92a9](https://github.com/jacob-delgado/workflow/commit/f3d92a91192d680637968bf130d9bc2059769502))
* **forge:** read a paged listing through one reader ([166ef50](https://github.com/jacob-delgado/workflow/commit/166ef50f674b3139348eb06cf092abd814f9ac56))
* **gitrepo:** take a finish's pull as a seam ([1b72ae2](https://github.com/jacob-delgado/workflow/commit/1b72ae2a451fed43c44da7f41d36e0d5c52c448b))
* **hooks:** drop Jobs and fold test lines through NextJob ([60741b5](https://github.com/jacob-delgado/workflow/commit/60741b58e76aace728446ebf25e73378dac560ae))
* **httpx:** unexport cause and test it through Unreachable ([36102a0](https://github.com/jacob-delgado/workflow/commit/36102a0b14fa102114aa11f5cbc9adecc42fc9a3))
* **jira:** build a JSON write request in one helper ([636c7f2](https://github.com/jacob-delgado/workflow/commit/636c7f2f46a64a5d1d2a043205d7cdc6bb88d39e))
* **keychain:** move storing a secret out of main ([1f86901](https://github.com/jacob-delgado/workflow/commit/1f86901ee5b4edb89c918755cb29630fbe82a039))
* look up the remaining dead-default switches in maps ([2a538fa](https://github.com/jacob-delgado/workflow/commit/2a538fa3ea722087023ca708844a4808a549dfa2))
* **loop:** compose an announcement from values already held ([8647685](https://github.com/jacob-delgado/workflow/commit/864768549a845a0593f216609024ec0531024f3d))
* **loop:** own the rule that a Jira key carries a dash ([a1328c3](https://github.com/jacob-delgado/workflow/commit/a1328c3654dae8e7007723c1efb067bd5d12ae4f))
* **loop:** propose a pull request's title and body once ([997a569](https://github.com/jacob-delgado/workflow/commit/997a5696567d7221d2c76277d1797279d4241577))
* **messaging:** answer checkable from a map keyed by mode ([182362e](https://github.com/jacob-delgado/workflow/commit/182362eaf77b952902f263ba7321790c9c2316ef))
* **progress:** name each stage's system for the spine's hue ([37e7496](https://github.com/jacob-delgado/workflow/commit/37e749651c25019a9c19cbbf42005241d34b8e42))
* **seams:** declare the shared seam bundles outside the terminal ([6198f6f](https://github.com/jacob-delgado/workflow/commit/6198f6f0fb036cf26ce07b5a88fb011403bca0f9))
* **store:** ask only whether a view was cached ([775e20b](https://github.com/jacob-delgado/workflow/commit/775e20b2dcdd0f6732e58c45de5486954561b402))
* **test:** lower a resource limit through one shared helper ([5353f99](https://github.com/jacob-delgado/workflow/commit/5353f99b6b6ef1931abaa0251a1e862666660597))
* **tui:** carry each run's title and headline in one run kind ([6685eb9](https://github.com/jacob-delgado/workflow/commit/6685eb939e1f3baec8b89e4fa5820c59a625aef5))
* **tui:** hold back EditPullRequest with the other forge writes ([6a7b16b](https://github.com/jacob-delgado/workflow/commit/6a7b16b298d43d0d48aaf3f48d9df26e18c367b4))
* **tui:** remember announcements as loop.Announced ([49d2831](https://github.com/jacob-delgado/workflow/commit/49d28317f93262e35aa5d0c18bab02ac13344921))
* **web:** draw every button through one Button ([423b427](https://github.com/jacob-delgado/workflow/commit/423b427c1005cdddf1396e13b1f25536c29d92f1))
* **web:** offer the snapshot's commit types in the commit form ([78d7cd1](https://github.com/jacob-delgado/workflow/commit/78d7cd117a8db28ab587c8e83fc3ae2d05917f30))
* **web:** read the snapshot through one hook that expects it ([d204252](https://github.com/jacob-delgado/workflow/commit/d2042526e2933372c8daea8de5daa0eb5ab4bfe1))
* **webserver:** build codeMeaning as a map keyed by problem code ([1395fd6](https://github.com/jacob-delgado/workflow/commit/1395fd67825ec052abe2cc1bc4f7dd4e35cc688e))
* **webserver:** classify seam failures through the server ([bd81dbc](https://github.com/jacob-delgado/workflow/commit/bd81dbcbf51939c8c7e539f2613374185caf6522))
* **webserver:** drop nil-body guards the strict handler rules out ([ff5dc02](https://github.com/jacob-delgado/workflow/commit/ff5dc02d53b0fea91804126f9f9088837d3ce405))
* **webserver:** read the branch, changes and review once for both ([3bbfd75](https://github.com/jacob-delgado/workflow/commit/3bbfd758f2566cbe9d2524c4d384ec3a8dfb00a0))
* **webserver:** share the re-read that follows a write ([887ce92](https://github.com/jacob-delgado/workflow/commit/887ce92937e5d64324cef2a846446deecc35752d))
* **web:** share the e2e Tab walk and stream helpers ([28ee7d2](https://github.com/jacob-delgado/workflow/commit/28ee7d23b0c29d0ee360e1174a50f0a1fe5f20c0))
* **web:** spell the theme storage key in one file ([2ba9cc4](https://github.com/jacob-delgado/workflow/commit/2ba9cc4d2e46d0e6a3ebb511a55acc804d79116e))
* **wiring:** bundle what the forge connection needs in one struct ([6901f31](https://github.com/jacob-delgado/workflow/commit/6901f31d3c844841ab51ab2da8fd5561a16a7e52))
* **wiring:** export how a forge request travels and signs in ([76b0f0c](https://github.com/jacob-delgado/workflow/commit/76b0f0c075ddc4bbb5d96b4120b83721fae1e61e))
* **wiring:** look forgeProgram up in a map ([5ac3933](https://github.com/jacob-delgado/workflow/commit/5ac39336e747994a03e8ad6ed22d93b84a55db05))
* **wiring:** make onceConnected generic over its connection ([9c4cbc1](https://github.com/jacob-delgado/workflow/commit/9c4cbc1373e450e64386fadc843aeab334ef47c2))
* **wiring:** share one HTTP client across the services ([28509af](https://github.com/jacob-delgado/workflow/commit/28509af8215626aa707fa8e336526f09ef96a625))
* **wiring:** stream any git command to its end, not only a fetch ([11dcf5c](https://github.com/jacob-delgado/workflow/commit/11dcf5c2485237aafe7396b63644f3434c036020))


### Documentation

* **api:** call an omitted optional field absent ([5611712](https://github.com/jacob-delgado/workflow/commit/5611712ba14d6738118c75cc82a889aaa9bae7c1))
* **api:** name the two cases push refuses as nothing to push ([079161e](https://github.com/jacob-delgado/workflow/commit/079161e7984d2406b547816fdb02fd424e53da2f))
* **api:** name web:gen:check as the web client's drift check ([8893151](https://github.com/jacob-delgado/workflow/commit/889315192142ba2a5ca6b46a0d412c59c5fc9e8e))
* **api:** say the server keeps the configuration in effect ([3f347e8](https://github.com/jacob-delgado/workflow/commit/3f347e882283a127a38b28dc5298115f9f8d005e))
* **api:** say the stream pushes on connect and every few seconds ([4977027](https://github.com/jacob-delgado/workflow/commit/4977027526b504dbb1b4881cbbe51040a920b2fc))
* **api:** tag the branch writes with a tag the spec declares ([ea6df89](https://github.com/jacob-delgado/workflow/commit/ea6df89d115e720f1e2013df0cc28fa960e02348))
* **backlog:** close the audit edition and re-pin all three backlogs ([880fc1d](https://github.com/jacob-delgado/workflow/commit/880fc1d85a5826cb1be957b3510574261f68bdab))
* **backlog:** keep the rest of the condition report as its worklist ([83ab1fc](https://github.com/jacob-delgado/workflow/commit/83ab1fc6d5d9a73feb4d22ed51ca7868bd9ed8a4))
* **backlog:** name the pull request that finished the paydown ([fbd901c](https://github.com/jacob-delgado/workflow/commit/fbd901cc0f1a26b0126c396865d398b4b1406411))
* **backlog:** point citations at the lines this series moved ([29fad1b](https://github.com/jacob-delgado/workflow/commit/29fad1b7246040583f83966f53844188edbaaf82))
* **backlog:** point citations at the lines this series moved ([6f7423b](https://github.com/jacob-delgado/workflow/commit/6f7423b73b2daa13f02ce2928900a9d9d84e1b1b))
* **backlog:** point every citation at the line it names ([2455e1a](https://github.com/jacob-delgado/workflow/commit/2455e1a7bec7b5920758fab7949443e162de5875))
* **backlog:** point the citations past a file's end at their lines ([3349cbc](https://github.com/jacob-delgado/workflow/commit/3349cbc93e9048d43ade1fa5773f5fffd8fa0869))
* **backlog:** point the rest of the scripting page citations at lines ([430599c](https://github.com/jacob-delgado/workflow/commit/430599c677159debc44d6694dcef312b99d1cf5a))
* **backlog:** point the scripting page citations at their lines ([c1300be](https://github.com/jacob-delgado/workflow/commit/c1300be446147b1a7387a822571c2e21adaf2454))
* **backlog:** register every kept trade-off with a reopen trigger ([5072a57](https://github.com/jacob-delgado/workflow/commit/5072a5796494d6ea02e5b8ffb6f386ed9dc4633c))
* **backlog:** register the conditions no black-box test reaches ([db5b4ce](https://github.com/jacob-delgado/workflow/commit/db5b4ceb6ed3f6e27c8711a2c2f563f9938207cf))
* **budgets:** name the pull request in every history row ([a8b90b7](https://github.com/jacob-delgado/workflow/commit/a8b90b79c8b8cb25b40c606234b686329cd18f91))
* **claude:** name every messaging service in the opening line ([95ec83b](https://github.com/jacob-delgado/workflow/commit/95ec83ba27f0da78f8fad04c4e06629252bcd15b))
* **claude:** name the credential fields in the never-print rule ([868f6f8](https://github.com/jacob-delgado/workflow/commit/868f6f854174a59915f38bdda1d767ceb94508f6))
* **cli:** say config init saves a webhook without checking it ([651747b](https://github.com/jacob-delgado/workflow/commit/651747b0ceaa8b95ff2ade73142f92bf22f51e9e))
* **cli:** say in the root help that Jira is optional ([934df56](https://github.com/jacob-delgado/workflow/commit/934df5665562bb6ff8b2f74c7a455c539a0ea257))
* **config:** add the thirteen keys the Fields table leaves out ([04657d3](https://github.com/jacob-delgado/workflow/commit/04657d39bde94a4553240a09d2344378cfac0c55))
* **config:** describe the walk up to the repository root ([a316087](https://github.com/jacob-delgado/workflow/commit/a316087051722af62ffb006fa30d6e140e670b3b))
* **config:** name the waiting announcement in the notify poll ([a0a82ad](https://github.com/jacob-delgado/workflow/commit/a0a82ad8f9e9afe0b2f5244b0b71e269ea5468a6))
* **config:** point ui.keys' comment at the documented action list ([8f6065b](https://github.com/jacob-delgado/workflow/commit/8f6065b38455cb3d98c88b777234400bee2dfe48))
* **config:** say the package saves the configuration as well ([1d49a6b](https://github.com/jacob-delgado/workflow/commit/1d49a6b6c0e34c013ba1e3405249a3682de09f8a))
* **convention:** name pull request text in the package doc ([a46cc46](https://github.com/jacob-delgado/workflow/commit/a46cc46fba4c903dd07e469e9bff5f31f3cbd4d4))
* **editor:** state git's editor order, GIT_EDITOR first ([85b088d](https://github.com/jacob-delgado/workflow/commit/85b088dda58cd492332e69a54146e12e2f66be87))
* **features:** let forge.cli reach the forge through gh or glab ([412fb3e](https://github.com/jacob-delgado/workflow/commit/412fb3efc7d03555277588428ee6a2d57c937dda))
* **forge:** send FindPullRequest callers to IsOpen, not Opened ([25996da](https://github.com/jacob-delgado/workflow/commit/25996daab43c209f1ab024d25fff745bd60f27ed))
* **gitrepo:** say the package changes the repository too ([b52428f](https://github.com/jacob-delgado/workflow/commit/b52428f36b4397df63d9f3c895d1ab12912b02e3))
* **install:** keep the pinned install version current on release ([cc1f926](https://github.com/jacob-delgado/workflow/commit/cc1f926f45953fe6f71364a3da4acf2fa9ea531e))
* **install:** list glab beside gh for forge.cli on GitLab ([96c9848](https://github.com/jacob-delgado/workflow/commit/96c9848a2b089e1b35a510801aaa701c39a78ee8))
* **jira:** describe the client's whole surface in its comments ([4f8ba81](https://github.com/jacob-delgado/workflow/commit/4f8ba8152988709189c8bec07f47c71df1d9e733))
* match an audit's findings against the trade-off register ([7bffd79](https://github.com/jacob-delgado/workflow/commit/7bffd79e0b628ef88d25c4fc1c2c37db0a8e5d21))
* **messaging:** say escaping and errors are per kind, not Slack's ([9d743b2](https://github.com/jacob-delgado/workflow/commit/9d743b2e58b295f966a97f7fde2b119cffeadfc4))
* move each String sentence from its assertion to the method ([bdeaddc](https://github.com/jacob-delgado/workflow/commit/bdeaddc2a022ce7690f62bf2890b9578c21a9515))
* name the store's three keys and its root-path fallback ([ce59f29](https://github.com/jacob-delgado/workflow/commit/ce59f29557275b395901eeaca4e982bf0643c69a))
* name the terminal prompt in main's layout row ([814a52f](https://github.com/jacob-delgado/workflow/commit/814a52f21a320da4b870bd109300e05eb0f3882e))
* point both task lint rows at the tasks lint runs ([bab845a](https://github.com/jacob-delgado/workflow/commit/bab845af439c98c9e82024ac50e98512c957fa77))
* point the platform count at RELEASE_PLATFORMS ([ea01be9](https://github.com/jacob-delgado/workflow/commit/ea01be9c8f8819aa2a5700fa26d563e0d972732d))
* **progress:** name the web's work story as a second derivation ([dadddaf](https://github.com/jacob-delgado/workflow/commit/dadddaf06fdb0613e0bd1acfd7478619b12da427))
* **reference:** name help as the one command without a page ([c108ae7](https://github.com/jacob-delgado/workflow/commit/c108ae70be905bbd3deaeba2bfd98c8b139e03c7))
* say condition coverage wherever gobco's metric is described ([deb918f](https://github.com/jacob-delgado/workflow/commit/deb918f304ab70bf430433e0fa9bdd7fe249c56f))
* say the forge's issues stand in when jira.base_url is empty ([c38d7b0](https://github.com/jacob-delgado/workflow/commit/c38d7b06bf522fbf398fb12f09d3999a4c8d530f))
* say the subject limit, trailer, title and CI poll are defaults ([fbabf49](https://github.com/jacob-delgado/workflow/commit/fbabf4959d024806f41b9fc4330a00f96525f97c))
* say what doctor --online checks, a webhook left unchecked ([6b818a7](https://github.com/jacob-delgado/workflow/commit/6b818a7553bc5bf452639769339062cc6580c8b5))
* **scripting:** name ghe.com tenants among the hosts that name a forge ([768b400](https://github.com/jacob-delgado/workflow/commit/768b400589f75ec4a965f5a113b627e00eadc5af))
* **security:** bring web/ and api/ into the reporting scope ([8883e59](https://github.com/jacob-delgado/workflow/commit/8883e59b1c3f4c0e68b3d9b61fe4eefe9387ef45))
* **tui:** make pane, key and overlay comments match the handlers ([0438b07](https://github.com/jacob-delgado/workflow/commit/0438b076254e063ccedf96c4c7bc3f8847e2df85))
* **usage:** describe field kinds, the base fetch and the diff ([416c68b](https://github.com/jacob-delgado/workflow/commit/416c68bde7e46c34e2049aec55a656df25d17e4c))
* **web:** lead the web README with task dev and web:mockup ([7e78a05](https://github.com/jacob-delgado/workflow/commit/7e78a0561fb611ca7f115e4c6c5a15bd4097d96f))
* **web:** list Settings in the form's order with every carried key ([575e60d](https://github.com/jacob-delgado/workflow/commit/575e60dca044ccfaf02a12cfd746dd8ee7ba63f6))
* **web:** name only what the coverage and lint lists hold ([2fc39c0](https://github.com/jacob-delgado/workflow/commit/2fc39c08f310049adb06f751153e0966074a7418))
* **web:** name the four GET reads a script may call ([942caf0](https://github.com/jacob-delgado/workflow/commit/942caf096d4ef46c278591d06aaaf25d515a1164))
* **web:** point the queryClient comment at useSnapshotStore ([3629773](https://github.com/jacob-delgado/workflow/commit/362977323d22fe69663dede676ed35dc39c65c7f))
* **web:** qualify the Settings save's stale-write guard ([c12dea7](https://github.com/jacob-delgado/workflow/commit/c12dea7f05abb8bffc1b96fac5a63c43a6b7c79d))
* **webserver:** name the answers a nil or failing seam gets ([62e55a3](https://github.com/jacob-delgado/workflow/commit/62e55a3fc6034cbb1d6ba1281a46989a0018722b))
* **wiring:** name every surface wiring connects ([a6a47ff](https://github.com/jacob-delgado/workflow/commit/a6a47ffd0c0f133b11d698595ab59555eab06fe1))


### Build & Packaging

* **air:** build under tmp/air so stopping task dev spares tmp/ ([68bdec3](https://github.com/jacob-delgado/workflow/commit/68bdec3dd3494f1daa2357eb0d04e9aa839bdce0))
* Bump knip from 6.36.0 to 6.37.0 in /web in the web group ([75635cc](https://github.com/jacob-delgado/workflow/commit/75635cceb1d591b7fbddb69d31b74be6bfb82d5c))
* clear git's hook variables before the web client check ([f34deb0](https://github.com/jacob-delgado/workflow/commit/f34deb0d9356dbe5e46996d85d1e8656a08a5be5))
* **cloc:** exclude every generated command reference page ([4b2d5f1](https://github.com/jacob-delgado/workflow/commit/4b2d5f16fd7a9ae9a85bf8ec13b61fbb701500eb))
* **cloc:** leave the generated web client out of the web count ([932559c](https://github.com/jacob-delgado/workflow/commit/932559cf17122105c08db22318b8e992b3e271ae))
* fail the container build when its Go is not mise.toml's pin ([eced846](https://github.com/jacob-delgado/workflow/commit/eced8463bd74b4fc23f2d1445ed307d5b1cf7a65))
* gate the mise version on every workflow and the devcontainer ([1297cb0](https://github.com/jacob-delgado/workflow/commit/1297cb09a860d538881eaeadc01b0657f8230400))
* **gobco:** measure conditions, not branches, and reset the floor ([e77c75c](https://github.com/jacob-delgado/workflow/commit/e77c75cde1a2c8bd7f5639d57cef944ad75d04ac))
* **lefthook:** run the web gates on a push that changes the web ([b1a6eeb](https://github.com/jacob-delgado/workflow/commit/b1a6eeb5485cebb36438199ec45d4db8f5b0749e))
* move the two-shell comment to the web pair it describes ([677f4cc](https://github.com/jacob-delgado/workflow/commit/677f4cc05a928ea230a198d61913aa7b52b6471f))
* root every Go tool at api/ as well as cmd/ and internal/ ([83d2c5c](https://github.com/jacob-delgado/workflow/commit/83d2c5c46d5553ddbfc9302b0bde3d5e66f4529b))
* **scripts:** gate the shape of the trade-off register ([fb5725a](https://github.com/jacob-delgado/workflow/commit/fb5725a6ebb765caeb32c27cea2f519aed7d2f88))
* **testshape:** count a stored func literal only when called ([6b5c378](https://github.com/jacob-delgado/workflow/commit/6b5c3788c1504bf0ade4a150b3bf23682d9b75e8))
* **testshape:** resolve a helper method by its receiver's type ([d71a056](https://github.com/jacob-delgado/workflow/commit/d71a0568c5b3e502729fcadfececade6940a8690))
* **testshape:** scope each subtest's closures to its t.Run ([1e7c0b3](https://github.com/jacob-delgado/workflow/commit/1e7c0b30cdeda7e3269b7582492deedc0acd4ae9))
* **web:** hold the e2e specs to the black-box assertion rules ([20722b5](https://github.com/jacob-delgado/workflow/commit/20722b5b7de15db13f5c5ec260533ea7edc3a719))


### CI

* **dependabot:** shorten the group names to fit the subject limit ([251bff5](https://github.com/jacob-delgado/workflow/commit/251bff524643c2d2deb2e455085c53bc26714e94))
* hold ci-gate on the cross-compile job ([2761ed2](https://github.com/jacob-delgado/workflow/commit/2761ed2486e893644e12b0964cc84818fe2e18d7))
* name the coverage comment job as the one that writes ([33b102a](https://github.com/jacob-delgado/workflow/commit/33b102af2a2d067a3b7cd40cf590685b688f8ed0))
* **release:** build in its own job, apart from attesting and publishing ([f432d49](https://github.com/jacob-delgado/workflow/commit/f432d49421e79c9859647471b653317be528f04d))
* **release:** let the tag script alone decide what a release commit is ([d864ec5](https://github.com/jacob-delgado/workflow/commit/d864ec5d2a9159e84999ed3436b1532b4a32bb87))
* **release:** republish the docs site once a release is published ([bd8dc80](https://github.com/jacob-delgado/workflow/commit/bd8dc801b8aec8f0de018fe8c27eee21dba50bab))
* **release:** run the gate in a job that holds no write permission ([306a242](https://github.com/jacob-delgado/workflow/commit/306a24294d261d2848493da74ebe7f51d8730a02))
* run test:summary's Go row over GO_PKGS with -race ([eb88d51](https://github.com/jacob-delgado/workflow/commit/eb88d51c16996549d99228b9cb46e59b26a3f4e9))
* run the server-backed e2e spec and hold ci-gate on it ([0b5e6cf](https://github.com/jacob-delgado/workflow/commit/0b5e6cf4e8606db6d17000456fb958a76dd76105))
* take test:summary's Go coverage from the coverage gate ([b28fbaf](https://github.com/jacob-delgado/workflow/commit/b28fbafe4ff7a548595ede5c95bfe49e13ef96a5))


### Tests

* **cli:** call a refused credential rejected in doctor --json ([39cee7d](https://github.com/jacob-delgado/workflow/commit/39cee7d66fd3e03d668421c09a588a9f3c1aea7c))
* **cli:** fail doctor --json on a refused output, name a path remote ([696193e](https://github.com/jacob-delgado/workflow/commit/696193e93f0e2f5deff0d3208556fe70092bbea8))
* **cli:** list no pull request when the branches cannot be read ([4d89358](https://github.com/jacob-delgado/workflow/commit/4d89358da0320ef127f8ae49dd3ceeb8110f603a))
* **cli:** pass over a branch or an index git cannot give ([3769a04](https://github.com/jacob-delgado/workflow/commit/3769a044683de3ee7b0397b519f4e3e1d6841b9a))
* **cli:** pin the exit status of each bare error check ([f08111f](https://github.com/jacob-delgado/workflow/commit/f08111f66239738369e56b990b578c6c564a1630))
* **cli:** report a working directory that was removed ([52c2bda](https://github.com/jacob-delgado/workflow/commit/52c2bdafb786e24c9dda3c6b5931280a2813f5a0))
* **cli:** see an announcement and a standup post delivered ([7a5d7e1](https://github.com/jacob-delgado/workflow/commit/7a5d7e13c205da80ce5d40441ada16149f1cded0))
* **cli:** stop config init where it cannot write or read ([a144896](https://github.com/jacob-delgado/workflow/commit/a144896c0613ff72d9adaefaea8cfac91f40c877))
* **cli:** stop pr at a review move nothing can answer ([fd283d4](https://github.com/jacob-delgado/workflow/commit/fd283d42ccbeb37c500fc20b8a03eb2c07f2f985))
* **cli:** stop standup at a failed edit or an unanswered post ([5df00c2](https://github.com/jacob-delgado/workflow/commit/5df00c2e0dfa270db041f2e042bcdb1dddcc2b9c))
* **config:** check every credential in the redaction fuzz ([96ea03d](https://github.com/jacob-delgado/workflow/commit/96ea03d22abf4005ff20e39be56766071548c2f0))
* **config:** hand Discover an empty home, as its comment says ([df99b0e](https://github.com/jacob-delgado/workflow/commit/df99b0e872027b69570786bd67844cb78df97987))
* **config:** keep the configuration when a save is cut short ([2de4d96](https://github.com/jacob-delgado/workflow/commit/2de4d96fc08a9118b9a77deee97719877295ebae))
* **config:** mask a token beside a Jira address that does not parse ([8e84bf9](https://github.com/jacob-delgado/workflow/commit/8e84bf90aa4198dd0f7d61304b81594c92670d11))
* **config:** name the messaging mode and target tests for Messaging ([d54b0eb](https://github.com/jacob-delgado/workflow/commit/d54b0eb1560c8fc128aa55a4d8fe1768a2e2ad9b))
* **config:** name the overwrite test for Save, check SaveOver's mode ([202d4a0](https://github.com/jacob-delgado/workflow/commit/202d4a0c2c64e5c72c8783c2463cf389d8acc870))
* **config:** refuse a save through more links than are followed ([b81b6dc](https://github.com/jacob-delgado/workflow/commit/b81b6dce243f06d533dcde4ad6f51ab56bd1125d))
* **config:** split the version test and assert ErrUnknownVersion ([4576bcf](https://github.com/jacob-delgado/workflow/commit/4576bcfd52ebd3fc7e3c496349910a9bc81645bf))
* **editor:** report a draft that cannot be written whole ([cb96925](https://github.com/jacob-delgado/workflow/commit/cb96925f5e3b568ace28667f26ebd0f5767c15c1))
* **forge:** leave out a template the repository cannot read ([a11bf28](https://github.com/jacob-delgado/workflow/commit/a11bf28f533ec4b3805eb796ff13992d6d234df8))
* **forge:** name ErrUnknownForge for an unknown forge ([4e27fce](https://github.com/jacob-delgado/workflow/commit/4e27fcea18b1c55ffef5d77f7e4ba30ef7b7ad6a))
* **forge:** report a read or re-run the forge refuses ([93fda1a](https://github.com/jacob-delgado/workflow/commit/93fda1a7b9736165a15e82307948096c7ee867ad))
* **forge:** report a write that never lands or an untyped answer ([8d0146b](https://github.com/jacob-delgado/workflow/commit/8d0146ba725b246e94587b009c15725f49211c1f))
* **forge:** stop opening a pull request at a person it cannot add ([b4004be](https://github.com/jacob-delgado/workflow/commit/b4004be4b59dc2d99658b2d1ab84353bd0bff647))
* **gitrepo:** leave a base undated when git gives no readable date ([37a0624](https://github.com/jacob-delgado/workflow/commit/37a0624c9309d31ba22ed53f34481b4adba324b0))
* **gitrepo:** name no method in the shared fake runner's failure ([5290f80](https://github.com/jacob-delgado/workflow/commit/5290f801ebb776ea8289e2e3c3c11091c4e3b5d4))
* **hooks:** leave out a hook that cannot be read ([62f953d](https://github.com/jacob-delgado/workflow/commit/62f953da1b36d28b1f3bc872a0450906a436b0e4))
* **jira:** send no write without a credential, keep refused comments ([13c2278](https://github.com/jacob-delgado/workflow/commit/13c2278dd9a9331d7e59841b5ee77531c3f039c4))
* judge the webhook address and the template per service ([f4be4f0](https://github.com/jacob-delgado/workflow/commit/f4be4f05f93db4e12500c09a7b47d653b9fa1599))
* **layout:** describe the six-pane rail the interface draws ([d33ea88](https://github.com/jacob-delgado/workflow/commit/d33ea88a6003be8a60cdd141523c869415a45da2))
* **messaging:** put each fixture's comment on the fixture it names ([845bc93](https://github.com/jacob-delgado/workflow/commit/845bc9365df8e5fbdb5067f147ea1fb5e849a3b2))
* **proc:** report a run with no descriptor left for its pipe ([8de680e](https://github.com/jacob-delgado/workflow/commit/8de680ef59c252906272d2485578da8fb7078c1e))
* **progress:** pin the stage rules the web's work story ports ([593253e](https://github.com/jacob-delgado/workflow/commit/593253e74327878ca9b4f282b30e0386e07014ab))
* ratchet BRANCH_COVERAGE_MIN to 92 ([59b6737](https://github.com/jacob-delgado/workflow/commit/59b673790257b0ecbbf89ca8d0a1074ad8e53c55))
* **release:** run the script's own --jq filter in the gh stub ([21f834b](https://github.com/jacob-delgado/workflow/commit/21f834bb9a91751d4aeeb16bc7cd7c14a9ef3f15))
* scripts. mise.toml's comment now says the gate holds the copies. ([1297cb0](https://github.com/jacob-delgado/workflow/commit/1297cb09a860d538881eaeadc01b0657f8230400))
* **scripts:** compare coverage-summary's numbers, not its keys ([2e14a34](https://github.com/jacob-delgado/workflow/commit/2e14a346d62e426ac98649012278565db073d473))
* **scripts:** drive gobco-report's two refusals with a stub gobco ([d9db0a3](https://github.com/jacob-delgado/workflow/commit/d9db0a3450c74487cecd70aaede70850e497154b))
* **store:** drop a read that fails partway and a refused commit ([a269606](https://github.com/jacob-delgado/workflow/commit/a2696062d2ab195430d3cc06cad9cc89d4472acc))
* **store:** fail on Arrange errors in the per-key isolation tests ([07554d7](https://github.com/jacob-delgado/workflow/commit/07554d7201430fc68623c264a66be0cf7d988759))
* **store:** parse every _at column read raw from the file ([3980f71](https://github.com/jacob-delgado/workflow/commit/3980f7151cd9db6540dc45b0b3c397e777c86298))
* **store:** prove the store's connection enforces foreign keys ([a895bcc](https://github.com/jacob-delgado/workflow/commit/a895bcc528f7e6b945f2a18b6b143df2f9cef029))
* **tui:** add merge, finish and edit to the in-flight write table ([f2111c6](https://github.com/jacob-delgado/workflow/commit/f2111c6fe8d3d59243e5c0d909062a0e05760ffb))
* **tui:** assert the commits detail itself says it is loading ([23ce036](https://github.com/jacob-delgado/workflow/commit/23ce036eb75045cb5da6a283ad427c0d5cd44d85))
* **tui:** click the collapsed issue list below 80 columns ([8b8c33b](https://github.com/jacob-delgado/workflow/commit/8b8c33bc84a053c58d2d032334f6f8de4a678379))
* **tui:** cover the quit guard, a spaceless filter key and no link ([fa67e70](https://github.com/jacob-delgado/workflow/commit/fa67e7091e73bf840562daf11871bc872f39ca7d))
* **tui:** give fake run output a Stop, as proc.Start does ([957031e](https://github.com/jacob-delgado/workflow/commit/957031ec370d8314809abbc12cd7ba6d6b3c3314))
* **tui:** hold the documented key actions to the bound ones ([0a86252](https://github.com/jacob-delgado/workflow/commit/0a862523b1d7a8a4902c797dce75ed680dd78a88))
* **tui:** pin the notify-only CI beat on a recording timer ([8c4829a](https://github.com/jacob-delgado/workflow/commit/8c4829a24fa79d262a632cda5e2c1df2d600180b))
* **tui:** press g where the lefthook offer is withheld ([efc5c5b](https://github.com/jacob-delgado/workflow/commit/efc5c5b21969748c91bb726064fee4caf0fe6553))
* **tui:** prove the fixup and pre-commit failure headlines ([cd079d3](https://github.com/jacob-delgado/workflow/commit/cd079d3b80ac3896adf00f16fc44513eb3e10805))
* **tui:** run the color tests in parallel ([c4d57c5](https://github.com/jacob-delgado/workflow/commit/c4d57c550f81b139246fcc250f69f6c9d493409b))
* **tui:** send a run just past its kept tail, not twice it ([26d62f9](https://github.com/jacob-delgado/workflow/commit/26d62f93bc42d6fcda96add2f56a5b8d5f247cb4))
* **tui:** suggest no reviewers when CODEOWNERS cannot be read ([36955a9](https://github.com/jacob-delgado/workflow/commit/36955a9177ee2ba342a624160a40fce221e65014))
* **web:** assert a commit sends its breaking mark and body ([94dade8](https://github.com/jacob-delgado/workflow/commit/94dade8391494551d181d7ddd764d9f0fe0d4e99))
* **web:** assert the push-first note both ways ([9c04a08](https://github.com/jacob-delgado/workflow/commit/9c04a08e093d1228ec3660dca6c3b6371892cd7c))
* **web:** await the refused views read before asserting no select ([25522b0](https://github.com/jacob-delgado/workflow/commit/25522b053c384bec212fb44d8f44a32f5f64674e))
* **web:** check the CI check's link by role and href ([3e23f8c](https://github.com/jacob-delgado/workflow/commit/3e23f8cf0104131bbe984baf4bc24338a5f0c126))
* **web:** drive a commit and a push against a running workflow --web ([aed7eda](https://github.com/jacob-delgado/workflow/commit/aed7eda74b866b48c2802cc30d9af38c08f6840c))
* **web:** hold index.html's theme key to the theme store's ([f596c61](https://github.com/jacob-delgado/workflow/commit/f596c61c9605ff6545d3380f48be8118406920e5))
* **web:** name the mock assignee by display name, as Jira does ([da4d6c7](https://github.com/jacob-delgado/workflow/commit/da4d6c7022c0340935e08c43558df452289fe9dc))
* **web:** preview the mock announcement from its own template ([c8d5561](https://github.com/jacob-delgado/workflow/commit/c8d5561c82a12f5aa385288c5592c99777fcf53a))
* **web:** scan the pull request form and a refused write ([ad6fefd](https://github.com/jacob-delgado/workflow/commit/ad6fefd3092c6bb41a9496b5ae52d0ccd17cb103))
* **web:** scan the push confirmation and announcement preview ([11409a2](https://github.com/jacob-delgado/workflow/commit/11409a27173b27cad28508ee4fc66e73eeb4a80b))
* **webserver:** file coverage_test.go's tests beside their handlers ([a22181e](https://github.com/jacob-delgado/workflow/commit/a22181e2801e3c59d987ebd35b4eb7bc46a0d87c))
* **webserver:** give the branch, changes and messaging reads own files ([4890213](https://github.com/jacob-delgado/workflow/commit/48902139cbe3ddaba268444a513bc4ba79a9f597))
* **webserver:** require an errors-page section for every problem code ([f9c2302](https://github.com/jacob-delgado/workflow/commit/f9c2302b68a6b19159c315fdcb75ee6292ac5cfd))
* **webserver:** wait for the re-push instead of sleeping ([a8ae224](https://github.com/jacob-delgado/workflow/commit/a8ae22433af6cf8ede3a119a249cea6fdf386d87))
* **webserver:** wait for the second frame the store test counts ([3dc8b0c](https://github.com/jacob-delgado/workflow/commit/3dc8b0c555f451141cd672e0a97b78b068dbe50c))
* **web:** settle the hermetic Settings scan on its Retry ([17976ec](https://github.com/jacob-delgado/workflow/commit/17976ec227927b8e6040b0134258a84db7ad3aef))
* **web:** type the e2e snapshot literals against the contract ([3b08f10](https://github.com/jacob-delgado/workflow/commit/3b08f103ce6ad6aec8c154e807dc2124de1e1ea7))
* **web:** use only documented placeholders in the mock config ([54fadca](https://github.com/jacob-delgado/workflow/commit/54fadca089bc373ae9b79740ef28d34203bf08ff))
* **web:** walk the push, announce and pull request steps at 640 px ([867346a](https://github.com/jacob-delgado/workflow/commit/867346a14c25f2fcdf5ab08cd926e32ec17c0a61))
* **wiring:** cover a failed opener, lefthook gone and a bare origin ([2b5668e](https://github.com/jacob-delgado/workflow/commit/2b5668ec6766c70151c3d50b94e102abcc6e0593))
* **wiring:** give each forge seam a subtest of its own ([0ecd7c7](https://github.com/jacob-delgado/workflow/commit/0ecd7c79e99885b8c676a3c9541abfb1cbe7aec8))
* **wiring:** reach the forge over HTTP, or refuse an unreadable body ([522330f](https://github.com/jacob-delgado/workflow/commit/522330f2b5633d8a722b93bb215cfc8c44e61f6b))
* **wiring:** refuse a commit whose message cannot be written whole ([bcfc445](https://github.com/jacob-delgado/workflow/commit/bcfc4454a49f431797e4c94be2ac4658ad0fac4a))
* **wiring:** report a fetch git cannot start ([825134c](https://github.com/jacob-delgado/workflow/commit/825134c31144f7d73b74ea9391ea15e87449bcf1))

## [0.3.1](https://github.com/jacob-delgado/workflow/compare/v0.3.0...v0.3.1) (2026-09-25)


### Features

* **config:** write the configuration only over the revision last read ([fc8dabf](https://github.com/jacob-delgado/workflow/commit/fc8dabf7bb74e71bdc1ac839096ccc67b7171614))
* **web:** offer Reload when Settings meets a change it has not seen ([076da0d](https://github.com/jacob-delgado/workflow/commit/076da0d944040585e857e33a3ee1328b701dcbea))


### Bug Fixes

* **scripts:** keep script tests out of the repository a hook names ([95964ea](https://github.com/jacob-delgado/workflow/commit/95964ea4aef374e58d31cc08a57c194fec4b98d6))
* **test:** keep git fixtures out of the repository a hook runs them in ([af55616](https://github.com/jacob-delgado/workflow/commit/af5561691c8c3738eaf32dae23083a03c0cdc1fe))
* **tui:** drop an issue read a later read superseded ([3bfe49f](https://github.com/jacob-delgado/workflow/commit/3bfe49fc44002045136d72aabeca121d9b5d51fe))
* **tui:** keep each pane's scroll when focus leaves and returns ([0b2971f](https://github.com/jacob-delgado/workflow/commit/0b2971f7294259014b353717d6f908920cea155c))
* **tui:** keep the CI polling generation on Model ([273aa15](https://github.com/jacob-delgado/workflow/commit/273aa1567e2bb5879884b256894aa3630b052b8d))
* **tui:** move a scroll stranded past the end on the first key ([9076d6d](https://github.com/jacob-delgado/workflow/commit/9076d6dace7b81ded7f9fb595f4df0d7bf0e1baa))
* **webserver:** refuse a Settings save over a change it has not seen ([38dbf38](https://github.com/jacob-delgado/workflow/commit/38dbf3877cc3c304e85d87cc880b63ae9355d9cc))


### Refactors

* **cli:** let a caller hand the root its interface and web server ([f55e833](https://github.com/jacob-delgado/workflow/commit/f55e8331728dda0582791f664fb7c04c5192ff38))
* **forge:** split GitHub's CI reads out of github.go ([8056091](https://github.com/jacob-delgado/workflow/commit/80560914dec184f8553e45a5c97570cbf9b94315))
* **gitrepo:** split changing branches out of branch.go ([7a7271e](https://github.com/jacob-delgado/workflow/commit/7a7271e4b7794dede87784c876ba759547a8e31a))
* **tui:** draw every picker's list through one pickList ([5884259](https://github.com/jacob-delgado/workflow/commit/5884259bfcb5bf5ecb52347d6fa70fa545221aca))
* **tui:** keep an overlay open with its failure through one helper ([afa58d4](https://github.com/jacob-delgado/workflow/commit/afa58d4b4ac42837ca453ae8d989e420d9703fb4))
* **tui:** split opening a pull request out of prcomposer.go ([e34cab6](https://github.com/jacob-delgado/workflow/commit/e34cab66e5f1af337b97f89c05de00db76662518))
* **tui:** split the announcement preview out of messaging.go ([b318832](https://github.com/jacob-delgado/workflow/commit/b3188322436f45488064b37c3f8204776bd7c76f))


### Documentation

* **backlog:** audit every surface, package and gate at f05ae9f ([df7e209](https://github.com/jacob-delgado/workflow/commit/df7e2097912051e2fbfdaf3a93a5c9fe08928d8c))
* **backlog:** re-pin the backlogs to the merged surface review ([89fef1b](https://github.com/jacob-delgado/workflow/commit/89fef1b7a18314cb622e967b29c64b861bbf51df))
* **debt:** refresh the trade-offs the gates changed ([a796ab8](https://github.com/jacob-delgado/workflow/commit/a796ab890ba4231f20bc31645a185748fc12e985))
* **features:** re-pin the ideas and correct what the audit disproved ([f989268](https://github.com/jacob-delgado/workflow/commit/f9892684da7d86548bde661c271491643f1093dc))
* **web:** write down why each dependency pin stands ([52feddf](https://github.com/jacob-delgado/workflow/commit/52feddf5818e27f867f59a0f270642c4733d2e2f))


### Build & Packaging

* Bump the web-minor-patch group across 1 directory with 11 updates ([996c3f3](https://github.com/jacob-delgado/workflow/commit/996c3f30b2c37459ac146233f0cd3b89b15596fb))
* run the container gate as the invoking user, under an init ([8737a8f](https://github.com/jacob-delgado/workflow/commit/8737a8fd27b18fa3075f5af92d2b5bc90e8bd6fc))
* **web:** drop @hey-api/client-fetch, which openapi-ts bundles ([c81e1b6](https://github.com/jacob-delgado/workflow/commit/c81e1b6957b4453c0cee0c1138f674555c28553a))


### CI

* Bump the actions-minor-patch group with 4 updates ([f05ae9f](https://github.com/jacob-delgado/workflow/commit/f05ae9f89e500c4f709ebd29e0ca9b5a33d28bd7))
* **dependabot:** schedule the web's npm updates ([6d2767a](https://github.com/jacob-delgado/workflow/commit/6d2767ad5ab1f2271614e212fa86e8a967e943d1))


### Tests

* **store:** report a directory, file or schema the store cannot use ([541ef9a](https://github.com/jacob-delgado/workflow/commit/541ef9a9b2c937e534e6dc33546a54296ce3410e))
* **tui:** hold every picker's selection in sight on a long list ([428ecd8](https://github.com/jacob-delgado/workflow/commit/428ecd8b9f97402f311bde5318f7770bbbe9beaf))
* **tui:** pin a failure in full in every overlay that pins one ([5212461](https://github.com/jacob-delgado/workflow/commit/5212461fcbe20276c25339c2352dd4abbdccc5ab))
* **tui:** pin where a pane starts and where a click lands ([8333a2e](https://github.com/jacob-delgado/workflow/commit/8333a2eefffe5412847b5f9910b15d69f918bd3a))
* **web:** hold the unit tests to floor(measured) − 2 on every metric ([9def03f](https://github.com/jacob-delgado/workflow/commit/9def03f8ef60c648eb0af21348c54983420758e5))
* **wiring:** drive the merge, merge-method and re-run seams both ways ([291dfe5](https://github.com/jacob-delgado/workflow/commit/291dfe57198d0cd023f05abc25acc7f9a7f020cf))

## [0.3.0](https://github.com/jacob-delgado/workflow/compare/v0.2.0...v0.3.0) (2026-09-24)


### ⚠ BREAKING CHANGES

* **cli:** status, reviews, standup, branch, pr and announce now fail with exit 3 when the .workflow.json in effect does not parse, where they ran on the default configuration. A file that cannot be opened is refused the same way, and exits 1.
* **cli:** `workflow status DIR...` exits 4 when any directory is not a git repository, and 1 when a repository could not be read, where it exited 0. Every directory still gets its line, on stdout, before the command fails.
* **cli:** `workflow config show` prints only the JSON on stdout; the `# <path>` line naming the file moved to stderr. Without a configuration file it now exits 3, where it exited 0, and its guidance is on stderr.
* **cli:** workflow exits 2 for a usage error, 3 for a configuration problem, 4 for a refused precondition, 5 for an unreachable service and 130 when interrupted, where every failure exited 1 before. `workflow config <unknown>` and `workflow completion <unknown>` now fail with 2 instead of printing help and exiting 0.
* **store:** workflow now keeps an on-disk store by default. Set store.disabled to true to keep nothing between sessions, as before.

### Features

* **api:** carry the issue's link and assignee in its detail ([bd1d5ce](https://github.com/jacob-delgado/workflow/commit/bd1d5ce08678ba82cadd38d48850d603e78a6dda))
* **api:** have health name the forge's noun and sigil ([4c25c50](https://github.com/jacob-delgado/workflow/commit/4c25c50c5763ae4cfded4f893eac9d7b8b58bf44))
* **api:** link the pull request and move the issue after opening ([313cf71](https://github.com/jacob-delgado/workflow/commit/313cf714a9d9f844010d6329ba7740810b4b8d7c))
* **api:** list the review queue at GET /api/reviews ([ba74b8b](https://github.com/jacob-delgado/workflow/commit/ba74b8b010a94d0341ec314403c2018c43b21be9))
* **api:** model web errors as RFC 9457 problem details ([8e501cb](https://github.com/jacob-delgado/workflow/commit/8e501cb7ade55ff958b7aea9eac55f5f7f1f2f00))
* **api:** stage and unstage from the web ([9db987c](https://github.com/jacob-delgado/workflow/commit/9db987c6c6b168de9aa86af6de718dd67f63a8ed))
* **api:** suggest the commit scope the interface would open on ([b8d3df0](https://github.com/jacob-delgado/workflow/commit/b8d3df0c72d6315f5f793b00dbeda271334a93a3))
* **cli:** accept --dry-run and --log before or after any command ([30e7dcd](https://github.com/jacob-delgado/workflow/commit/30e7dcdc67d556942038db9c86ea9c077e177819))
* **cli:** announce, not post ([44304a6](https://github.com/jacob-delgado/workflow/commit/44304a6d4fb7eb234ae8f0e862ea5956b4f9e700))
* **cli:** exit with a status that says what kind of failure it was ([f6dc585](https://github.com/jacob-delgado/workflow/commit/f6dc585efed9ec749c557cf3b3a053643a148c4d))
* **cli:** have pr's question name the push it makes first ([6e999d8](https://github.com/jacob-delgado/workflow/commit/6e999d8ffd036d33bc76e4524a5e1d6dce88c96a))
* **cli:** let standup run unattended with --dry-run and --yes ([281bcc8](https://github.com/jacob-delgado/workflow/commit/281bcc852485f73b9863f8b32eea95579cdefe3e))
* **cli:** offer to link the pull request on its issue after opening ([e4bf56f](https://github.com/jacob-delgado/workflow/commit/e4bf56fbf343039853104231f6fcd3e00083a599))
* **cli:** point a mistyped command at --help and its closest match ([9b6fc96](https://github.com/jacob-delgado/workflow/commit/9b6fc967dd4dbcb3a4cd5f15e2fc2cd202815487))
* **cli:** preview config init with --dry-run ([0c59c78](https://github.com/jacob-delgado/workflow/commit/0c59c78872667a65f1647715740cceb740e2a801))
* **cli:** remember what announce posted, and say when it already has ([db68c2c](https://github.com/jacob-delgado/workflow/commit/db68c2c30852af77b0ec85904d9b3c1e50560856))
* **convention:** team-configurable conventions (FEAT-14) ([6245499](https://github.com/jacob-delgado/workflow/commit/6245499303eec451dba0c6e16b8772c9907f9359))
* **hooks:** read ESLint stylish and Python traceback failures (FEAT-25) ([cb4d44d](https://github.com/jacob-delgado/workflow/commit/cb4d44d5b926a11a670bb42a37881e0fa6f6d53d))
* **jira:** mark a 404 answer as not found ([9b58f52](https://github.com/jacob-delgado/workflow/commit/9b58f527bfa58a39e34190bcd32ce41de5b5e9ce))
* **progress:** name the last stage for the messaging service ([1c3dbc1](https://github.com/jacob-delgado/workflow/commit/1c3dbc13c50afdeaa6e6fb9a832f179af76dd6a5))
* **store:** keep state on disk, learn the last scope (FEAT-67) ([662c610](https://github.com/jacob-delgado/workflow/commit/662c6100a6c5c059419b9bf809c19ae98f882715))
* **store:** remember what was announced across sessions (FEAT-66) ([f99e34a](https://github.com/jacob-delgado/workflow/commit/f99e34a5e552a40421585ff0ff715182906dd085))
* **store:** start instantly from a cached issue list (FEAT-68) ([7fbcf1f](https://github.com/jacob-delgado/workflow/commit/7fbcf1f9b72fb215895d086b874d50a60038f837))
* **tui:** announce, not post ([2fa20ce](https://github.com/jacob-delgado/workflow/commit/2fa20cef90de4bdb6ced6a045345859e5832c274))
* **tui:** draw a failure notice in the failure style ([59e41b0](https://github.com/jacob-delgado/workflow/commit/59e41b001663c64d44886902319307e0d618a7d1))
* **tui:** file ctrl+w and w under the overlays that answer them ([34c3eaf](https://github.com/jacob-delgado/workflow/commit/34c3eafa2d1d7ec9bf6c246f289849d4dc7f4579))
* **tui:** keep ? in every pane's footer however narrow ([45eeb6e](https://github.com/jacob-delgado/workflow/commit/45eeb6e4a41899d72d7cd4cdde64e62091c25536))
* **tui:** offer the review status once a PR is open (FEAT-05) ([2182cc2](https://github.com/jacob-delgado/workflow/commit/2182cc2affa0f873227ff601f5f283d844b742c7))
* **tui:** show every key the Issues pane answers in its footer ([f8bd772](https://github.com/jacob-delgado/workflow/commit/f8bd772bff6a35f771e7d6113263a4103528c683))
* **tui:** take a last look before re-running failed CI ([7ba0e4b](https://github.com/jacob-delgado/workflow/commit/7ba0e4b90152320d5c0547a11e4fb0b480fd941e))
* **tui:** take a last look before rebasing onto the base ([b69ad32](https://github.com/jacob-delgado/workflow/commit/b69ad3282c1db4b23f33974bc3bca05676309125))
* **tui:** take the interface's waits through a Deps.After timer ([b69db21](https://github.com/jacob-delgado/workflow/commit/b69db21dfc18542dd2a700478c29d72f45dc9428))
* **tui:** tell every one-row failure through failureLine ([f37a2e3](https://github.com/jacob-delgado/workflow/commit/f37a2e3ac3f2f97a4c7277fd4b6bfa1334bb66f8))
* **tui:** word every seam's failure briefly and in full ([3cbb095](https://github.com/jacob-delgado/workflow/commit/3cbb095e36aac1fa08ac15071dadef38421ff527))
* **web:** add the Reviews section ([03bb579](https://github.com/jacob-delgado/workflow/commit/03bb57978fc07c0055ad2414b2f33f1416b7aa48))
* **web:** announce, with the preview's confirm named Announce now ([948bc70](https://github.com/jacob-delgado/workflow/commit/948bc70c589b14bef2d6ad4bedfdeaeddccadb1d))
* **web:** call it a merge request, marked !N, on GitLab ([8ee17f5](https://github.com/jacob-delgado/workflow/commit/8ee17f5b1aeb5e798d10ce33f3b5045165f31f48))
* **web:** check out an in-flight branch from the Issues list (FEAT-77) ([44ed30a](https://github.com/jacob-delgado/workflow/commit/44ed30a8af3f7f7d4eb83efc2eb046fd904b53fd))
* **web:** draw every state by its shape with one StateMark ([594d47f](https://github.com/jacob-delgado/workflow/commit/594d47ff35bbdfe87aedb9cbad0f0b8bdcbfeb60))
* **web:** give each system its own hue ([1cdae96](https://github.com/jacob-delgado/workflow/commit/1cdae96373e8d5cd02ceff721c6985dfe66d3063))
* **web:** head every part in sentence case, not capitals ([4509ad3](https://github.com/jacob-delgado/workflow/commit/4509ad39494d4a96c057be56a8ad6c192120e3b8))
* **web:** hold the header still while the content scrolls ([4a3e009](https://github.com/jacob-delgado/workflow/commit/4a3e00904e72fac3144444143f8d522cff33184e))
* **web:** keep only the rail's icons below md ([279a2ff](https://github.com/jacob-delgado/workflow/commit/279a2ff30eff651363dc582e8a90e306d18cffae))
* **web:** let the content reflow to the width it has ([d0e975f](https://github.com/jacob-delgado/workflow/commit/d0e975f388d0c3e3ad760272c7cb3448a066e74b))
* **web:** load past the first page and filter the issue list ([76f2c55](https://github.com/jacob-delgado/workflow/commit/76f2c55020cb66dc9d2173a834e0a41ddebc9d40))
* **web:** mark the stream out of date when a frame cannot be read ([5e7724b](https://github.com/jacob-delgado/workflow/commit/5e7724b00c6cfd8c7dad9a94f60901c1b2b4083d))
* **web:** name the messaging section after its service ([e90b0cc](https://github.com/jacob-delgado/workflow/commit/e90b0cc757b5cf54b6e3b7fd739f643ce0715615))
* **web:** offer the link and the move after opening ([30bdf67](https://github.com/jacob-delgado/workflow/commit/30bdf6795a1d8f003ea0eca157c0b561c2a9ed9f))
* **web:** one line for connecting, and a way out of each dead end ([9d36d9f](https://github.com/jacob-delgado/workflow/commit/9d36d9f0f2773c05fcc49fcbcce3e812429ab574))
* **web:** open the commit form on the suggested scope ([390bf14](https://github.com/jacob-delgado/workflow/commit/390bf149779ef75813b8220dc33785491ad30633))
* **web:** read the selected issue in full ([c692a54](https://github.com/jacob-delgado/workflow/commit/c692a54758760fda1a7e6809bc5f7829eb853290))
* **web:** say "Start work" and nothing else for starting ([4b31c8e](https://github.com/jacob-delgado/workflow/commit/4b31c8e692d498564b8effd5469ce8b0ee786705))
* **web:** say what every write did, in a line the snapshot leaves ([6d15587](https://github.com/jacob-delgado/workflow/commit/6d155878ecd6b940eb1705c2d16dc302447b455a))
* **web:** set one scale each for type, space and corners ([1d16316](https://github.com/jacob-delgado/workflow/commit/1d16316c1751bd326daca662c3c7ca548aa214e3))
* **web:** show the version and hold every write under --dry-run ([f33515e](https://github.com/jacob-delgado/workflow/commit/f33515ee0feee547b318f8b7212f9851a1c98c3b))
* **web:** stack the issue list over its detail below lg ([37db99c](https://github.com/jacob-delgado/workflow/commit/37db99ce62c3925f113b164693cdf435937ec13b))
* **web:** stage from the working tree, and keep the commit form ([70f460d](https://github.com/jacob-delgado/workflow/commit/70f460dce60fe2b7925eeea6ff9b2a61e2439917))
* **web:** switch the issue list between the configured views ([8e78e3b](https://github.com/jacob-delgado/workflow/commit/8e78e3ba136965c63a1a5a6c2bf748bd67ff14a0))
* **web:** take focus through each step, and to the section chosen ([af183f6](https://github.com/jacob-delgado/workflow/commit/af183f6ee46f688e2174fd2cff73db922f8ddc4f))


### Bug Fixes

* **api:** tighten the RFC 9457 error classification ([5e69cb8](https://github.com/jacob-delgado/workflow/commit/5e69cb8c38f0adc50ce3bd155246a8b385536ca8))
* **cli:** fail status DIR... when a directory cannot be read ([3da27b7](https://github.com/jacob-delgado/workflow/commit/3da27b71aaf1293b7a3512092fd042ef93683de3))
* **cli:** have each refusal name its next step ([c2e7f56](https://github.com/jacob-delgado/workflow/commit/c2e7f56d08e06a8bc67d6526e4b1c3d5f541a225))
* **cli:** keep commentary on stderr and the artifact on stdout ([849cf2f](https://github.com/jacob-delgado/workflow/commit/849cf2f63ca685af830cc18172a14785d41412fd))
* **cli:** refuse a configuration file that cannot be read ([4b1c9eb](https://github.com/jacob-delgado/workflow/commit/4b1c9eb5cc0d9cd79b21c080744af4ffd7363ba3))
* **cli:** say that branch switches to the branch it creates ([7cf7d40](https://github.com/jacob-delgado/workflow/commit/7cf7d40f9213cb38f33cb5433d63c039c99590d2))
* **cli:** say to pass --yes when there is no terminal to confirm on ([15d9654](https://github.com/jacob-delgado/workflow/commit/15d96543886e045c1690fedffdbd93628a577a94))
* **cli:** write config show's JSON alone to stdout ([31d2391](https://github.com/jacob-delgado/workflow/commit/31d2391b014b2d092de3cd25063973243b9b075a))
* **gitrepo:** guard checkout against an option-like branch name ([8398cbc](https://github.com/jacob-delgado/workflow/commit/8398cbcab09b1d81c454a5ba843c0970a00242d0))
* **gitrepo:** read the status without taking the index lock ([542b8a4](https://github.com/jacob-delgado/workflow/commit/542b8a488b056e61be57bb6169d03df765890f50))
* **hooks:** stop the ESLint file leaking across tools (FEAT-25) ([a888063](https://github.com/jacob-delgado/workflow/commit/a888063c1450fa63e1274d6c91cf0242d0d70b8e))
* **messaging:** name the service a broken answer came from ([a1c898a](https://github.com/jacob-delgado/workflow/commit/a1c898aa3e8a978ee502360cae5825781021c48d))
* **tui:** end a cut footer on an ellipsis, never half a key ([d490b82](https://github.com/jacob-delgado/workflow/commit/d490b823e26b944aa8812d60d6c701861799b2c9))
* **tui:** fold the help into one column where two would be cut ([c1b7ad6](https://github.com/jacob-delgado/workflow/commit/c1b7ad6eff456f73d4a1d3f09af616b18976f193))
* **tui:** keep a failed finish in its preview ([e96d14b](https://github.com/jacob-delgado/workflow/commit/e96d14bcae6cc238cc9b20e5c2e516753558d0f5))
* **tui:** keep a refused merge in its preview ([e7e4f66](https://github.com/jacob-delgado/workflow/commit/e7e4f668308aece824ea62d765a6ddba91f97508))
* **tui:** name the failed step when git exits with a failure status ([c190f90](https://github.com/jacob-delgado/workflow/commit/c190f905b778fd874fd9b2154a452e7ced80ea77))
* **tui:** name the review and messaging panes, not Slack, in a key clash ([5d47028](https://github.com/jacob-delgado/workflow/commit/5d47028dcb9c8f4a5db86c6c4d469c2a84c3656e))
* **web:** describe the cockpit without naming Slack ([56a98c6](https://github.com/jacob-delgado/workflow/commit/56a98c68f15db7124366a83cbe0549aa6b78ea7b))
* **web:** hand focus to what a write said by the one rule ([dfb9550](https://github.com/jacob-delgado/workflow/commit/dfb955097afb87c97fc2b8257d9079b3fc0dd76f))
* **web:** have every fallback say what to do next ([b51e93f](https://github.com/jacob-delgado/workflow/commit/b51e93f1c3060f44070239ab71ead363ec8653c9))
* **web:** open each section at its top ([d0837fa](https://github.com/jacob-delgado/workflow/commit/d0837fa9c3e54ef83c0feaaeb2b928e50a144346))
* **webserver:** compose the pull request draft when the forge is unread ([00077fc](https://github.com/jacob-delgado/workflow/commit/00077fc2110b4e365ab03835dde02b7ca55c98c6))
* **webserver:** queue a commit and a new branch behind a stage ([b139c8a](https://github.com/jacob-delgado/workflow/commit/b139c8a286045a89e98779d7aa2eb4374ebf1cc3))
* **webserver:** refuse an unknown view instead of showing the default ([2aadb69](https://github.com/jacob-delgado/workflow/commit/2aadb69f491b86639e5c9468303a4d923fdc5628))
* **webserver:** say how to see git's reason for a refused switch ([6443786](https://github.com/jacob-delgado/workflow/commit/6443786da5d3ecca12592783a86ec441699f92ec))
* **webserver:** tell every messaging failure apart, never in its words ([898abfa](https://github.com/jacob-delgado/workflow/commit/898abfa8d542ea84382b550fc5680a975a3540d2))
* **webserver:** tell the forge's own failures apart ([a702b6f](https://github.com/jacob-delgado/workflow/commit/a702b6f13dcf859134b051396f6b0e56c5568d1a))
* **webserver:** word what it tells the page in the forge's noun ([b63eafd](https://github.com/jacob-delgado/workflow/commit/b63eafd2f2fb5ef4c98ceaa22890db13daf083af))
* **web:** show a control is off by color, and honor reduced motion ([87b30cc](https://github.com/jacob-delgado/workflow/commit/87b30cc561e0b7aed0b5f78fba8770ebf70f86a0))
* **web:** target the right branch when an issue has several (FEAT-77) ([bddd127](https://github.com/jacob-delgado/workflow/commit/bddd127b54d9b807fb482c1351a60d975ef05711))
* **wiring:** leave every webhook's path out of the request log ([1dbd16a](https://github.com/jacob-delgado/workflow/commit/1dbd16adfbbe9855b6d138579dbcf3fdaf6b57e3))


### Refactors

* **cli:** connect every command through one connect ([e3ecb36](https://github.com/jacob-delgado/workflow/commit/e3ecb36e748836738541930b1ad1fd60e589353c))
* **cli:** read status and standup through the git seams ([de70478](https://github.com/jacob-delgado/workflow/commit/de70478f168d643b886a5a42c9e98e948346898f))
* **cli:** write every command's JSON through one encoder ([aee76d4](https://github.com/jacob-delgado/workflow/commit/aee76d4e1b72dbb49fdcf6d0bffded1f7b8039c5))
* **config:** name branches through config.Branch.Naming ([3bc8857](https://github.com/jacob-delgado/workflow/commit/3bc8857339a8a617c95ab9212ec0eff23af90839))
* **forge:** name a change's noun and sigil on forge.Kind ([95cb466](https://github.com/jacob-delgado/workflow/commit/95cb466aa28e5ec809f44eb5dab8334ef218bf97))
* **forge:** order the review queue in one place ([e7495fc](https://github.com/jacob-delgado/workflow/commit/e7495fc36be854de82ee153267fb6b9caf06bcce))
* **gitrepo:** range strings.SplitSeq now that gobco reads iterators ([428fb1b](https://github.com/jacob-delgado/workflow/commit/428fb1b51db9d49e8bb9d2b69a21d910df0ee8d7))
* **loop:** compose the announcement once ([c7c4a92](https://github.com/jacob-delgado/workflow/commit/c7c4a92ecd22eec104356aa29423eff312872d4b))
* **loop:** compose the pull request and its push once ([26bc793](https://github.com/jacob-delgado/workflow/commit/26bc7931e7eaa2cb09f4d3d635433d698cd6a90d))
* **loop:** find the review-status move once ([a29e475](https://github.com/jacob-delgado/workflow/commit/a29e4754ea76963f41f6eff8ed48dbeaadd30696))
* **loop:** name what stage-all stages once, for every surface ([aaa6915](https://github.com/jacob-delgado/workflow/commit/aaa69155acf079d9bc3d62ef65700afa33d266e2))
* **loop:** say why no move to the review status is offered ([ba3a79c](https://github.com/jacob-delgado/workflow/commit/ba3a79c241d887728db5bad8c309cf56ae38d8c8))
* **loop:** share the dirty-tree and nothing-staged guards ([e76dbaa](https://github.com/jacob-delgado/workflow/commit/e76dbaa48281f88cbc35bd545a4b95092f9d369e))
* **messaging:** rename the internal/slack package (FEAT-58) ([eada135](https://github.com/jacob-delgado/workflow/commit/eada135189857a339e54414915b9db73700d8428))
* **store:** normalize the cache to 3NF and make tables STRICT ([23e39ed](https://github.com/jacob-delgado/workflow/commit/23e39ed3c860ab7909be0dcec19a34a0369d5aac))
* **tui:** carry the Messaging pane's post in a sendState ([e8ebc46](https://github.com/jacob-delgado/workflow/commit/e8ebc460c84ad9282b423c71449493cf55b0ddff))
* **tui:** carry the task switcher's switch in a sendState ([36d884e](https://github.com/jacob-delgado/workflow/commit/36d884ec424e202a2120a50e606a28280654e003))
* **tui:** gather the failure family in failure.go ([ec6d917](https://github.com/jacob-delgado/workflow/commit/ec6d91744f3a2f94188b02cf04c9f9ac180a7aac))
* **tui:** generalize the push preview into a last look ([32828c0](https://github.com/jacob-delgado/workflow/commit/32828c0fd5999ea9a3f100bf4927b3748e945e01))
* **tui:** name the review and messaging help group for neither ([d6bdac3](https://github.com/jacob-delgado/workflow/commit/d6bdac396ab87f0704ec2e44d741045f93401e86))
* **tui:** split the merge picker out of review.go ([fc88bf4](https://github.com/jacob-delgado/workflow/commit/fc88bf4aa6404415528b773f9068c206c5682952))
* **web:** give every write the one async state machine ([98059b8](https://github.com/jacob-delgado/workflow/commit/98059b8bbfa8537ef8e83f8ecf95c91924890676))
* **web:** move useAsyncAction to lib, where every write reaches it ([3d38168](https://github.com/jacob-delgado/workflow/commit/3d38168bdadab1d142e6a3515634e06601dcde0f))
* **web:** split the commit and pull request forms by their fields ([da277fc](https://github.com/jacob-delgado/workflow/commit/da277fcc51bea4209128ff146d4a63df408db553))
* **web:** split the settings form by fieldset ([47d2fd3](https://github.com/jacob-delgado/workflow/commit/47d2fd3a841a2a0b9d7df7011e9650876eb94ab7))


### Documentation

* add an architecture guide with mermaid diagrams ([addcb64](https://github.com/jacob-delgado/workflow/commit/addcb64a9f5843f675dedcd67d67659157e391f4))
* add the surface review handoff (REVIEW.md) ([cb325f9](https://github.com/jacob-delgado/workflow/commit/cb325f94d1869e01239705056761f2521fd6b5ec))
* **claude:** codify the database standards the store now follows ([f6aab44](https://github.com/jacob-delgado/workflow/commit/f6aab441ba31561f25a416e4918bf9913a45970e))
* **claude:** map every package and write down the web's exemption ([bb8fe70](https://github.com/jacob-delgado/workflow/commit/bb8fe70a5cf80b32497ebd9dfd1efa9b421f26d0))
* **cli:** document scripting, and generate --version and completion ([0d93078](https://github.com/jacob-delgado/workflow/commit/0d93078fa4331a0bd44d5c9312c900ed8e1ffc75))
* **cli:** name the messaging service, not Slack, where any service fits ([f1bf9dc](https://github.com/jacob-delgado/workflow/commit/f1bf9dc6b70410abe7b722d167b8682f063fb6f0))
* **debt:** record why task container:check is red (DEBT-72) ([555f3d8](https://github.com/jacob-delgado/workflow/commit/555f3d8fbbdff8119402b86e119c2fe116ce8150))
* **debt:** recount DEBT-57's overlay appliers ([37f7648](https://github.com/jacob-delgado/workflow/commit/37f7648dcb22ec4b3e9fcd33e7a355bc63db8d08))
* document the on-disk store and its store.disabled opt-out ([63a6ce3](https://github.com/jacob-delgado/workflow/commit/63a6ce38d01b5c96e96c38fcbad97723a5b4e442))
* document the RFC 9457 web API errors ([917ce84](https://github.com/jacob-delgado/workflow/commit/917ce84f33e0340839ae21b329dacb462382124f))
* **features:** record the web's two declared gaps before REVIEW.md goes ([75ae311](https://github.com/jacob-delgado/workflow/commit/75ae311223059ac907de9facca22053b0fd1ed1f))
* **features:** say six panes, and drop the notes of what shipped ([3784e94](https://github.com/jacob-delgado/workflow/commit/3784e94dfba8949060f6cc3401226ba93697a3ba))
* file the surface review's feature ideas (FEAT-78 to FEAT-83) ([c80e5c3](https://github.com/jacob-delgado/workflow/commit/c80e5c3d773a4236f4312147d2f2cabc4dbb0e96))
* pin the surface review to main's merged tip ([800a26d](https://github.com/jacob-delgado/workflow/commit/800a26d8d748540e244caffd75a1f34f2f5d8491))
* repopulate the UX backlog (UX-50 to UX-89) ([211238e](https://github.com/jacob-delgado/workflow/commit/211238e7266fb5e8a438611405aeceaec4aac44e))
* **review:** delete the surface review, every phase done ([3c93dbc](https://github.com/jacob-delgado/workflow/commit/3c93dbc8f2023c26d67e9d6b7606a8a5510b718e))
* **review:** mark phase 0 done ([f881d00](https://github.com/jacob-delgado/workflow/commit/f881d00629750149f8590a3451fc6a2f1b461714))
* **review:** mark phase 1 done ([90792e3](https://github.com/jacob-delgado/workflow/commit/90792e334ceb363293add6d1ffb9460bf7093c05))
* **review:** mark phase 10 done ([84288e6](https://github.com/jacob-delgado/workflow/commit/84288e6ecee738a73a0b16a515d3ccaa6a25bd67))
* **review:** mark phase 11 done ([f6f4ea2](https://github.com/jacob-delgado/workflow/commit/f6f4ea248fe019bf81d3ff22544aa527793f1ee1))
* **review:** mark phase 12 done ([c553061](https://github.com/jacob-delgado/workflow/commit/c553061667bf9be14143a49cbe932ca1657e0cbf))
* **review:** mark phase 13 done ([79306f1](https://github.com/jacob-delgado/workflow/commit/79306f163d59954c022614de5bd2ae5cb3231017))
* **review:** mark phase 14 done ([3ebf3e3](https://github.com/jacob-delgado/workflow/commit/3ebf3e30a7f4b97d3676e281bd13d988fea3a275))
* **review:** mark phase 15 done ([d29eed3](https://github.com/jacob-delgado/workflow/commit/d29eed3d0bc933d1990b2f0d4fe684e33e783365))
* **review:** mark phase 2 done ([3f42c3e](https://github.com/jacob-delgado/workflow/commit/3f42c3e1c0cbd350b251b565094e04b656d12350))
* **review:** mark phase 3 done ([5647e51](https://github.com/jacob-delgado/workflow/commit/5647e51acd8f65cc1a423bacdfc42bc7940d7d78))
* **review:** mark phase 4 done ([98f1f9c](https://github.com/jacob-delgado/workflow/commit/98f1f9c8cfe169078598afd422817eef8ecc8154))
* **review:** mark phase 5 done ([086573d](https://github.com/jacob-delgado/workflow/commit/086573dbbb56770ae09545e0baeb4469e4dc1a25))
* **review:** mark phase 6 done ([215aa77](https://github.com/jacob-delgado/workflow/commit/215aa77c1144714254da793fee2f25b47a68c644))
* **review:** mark phase 8 done ([40eb822](https://github.com/jacob-delgado/workflow/commit/40eb822deb227ab399a1586fbd25119a56cf6be0))
* **review:** mark phase 9 done ([e06ff57](https://github.com/jacob-delgado/workflow/commit/e06ff5702b0883a1182e864f0a3774bdb4c42c3e))
* **review:** record the maintainer's decisions and corrections ([f5d0994](https://github.com/jacob-delgado/workflow/commit/f5d099450b151906d8c04e51023c4780c1878b6c))
* **scripting:** say exit 3 is fixed by a credential or a login too ([e44c74e](https://github.com/jacob-delgado/workflow/commit/e44c74ef16412775f06e0e733e64f502eba139a0))
* start a fresh technical-debt backlog (DEBT-50 to DEBT-71) ([ffeac51](https://github.com/jacob-delgado/workflow/commit/ffeac51c4dfcb33af041cab6bcb99eb20ef10816))
* **usage:** describe the six panes and every key the interface has ([c15bf57](https://github.com/jacob-delgado/workflow/commit/c15bf5774b7bdb80cde74ac57442d2d078fcbcb9))
* **web:** give workflow --web a page of its own ([67c6919](https://github.com/jacob-delgado/workflow/commit/67c69190d361f2ea55e0b6a5087f1c2857d7e136))


### Build & Packaging

* Bump modernc.org/sqlite in the go-minor-patch group ([dff847b](https://github.com/jacob-delgado/workflow/commit/dff847bb29c87269e7de0f487c2f61f2c915e2c8))
* **lint:** measure TypeScript against the file-length gate ([24da323](https://github.com/jacob-delgado/workflow/commit/24da323ed3c0cb70f370d5343891e757630402c1))
* run the web lint, client check and unit tests in task check ([6ef2892](https://github.com/jacob-delgado/workflow/commit/6ef2892a12a23c7e37f30968f4b69c86e232ca56))
* **web:** cap a function's length in the web lint ([9582d90](https://github.com/jacob-delgado/workflow/commit/9582d90195598c759e848ba98d033a5ef630cd60))
* **web:** hold the generated SDK behind the api seam ([85b0fa2](https://github.com/jacob-delgado/workflow/commit/85b0fa2bbf8618a0a0a16bebc7ae6ee2ac098685))


### Tests

* **cli:** keep stdout and stderr apart in the command harness ([6231cba](https://github.com/jacob-delgado/workflow/commit/6231cba4a6a5a659a7d401eba63bef6a7a554361))
* **cli:** prove the web server gets every seam the interface wires ([4a56f82](https://github.com/jacob-delgado/workflow/commit/4a56f82019a69ad579a0b0d29a450b77c4835e5c))
* **cli:** run each command in a home the test can choose ([6c596bc](https://github.com/jacob-delgado/workflow/commit/6c596bc652698e6fbc6153a02e404a2fbcb04eff))
* **e2e:** scan and screenshot a populated mockup build ([32e0cf3](https://github.com/jacob-delgado/workflow/commit/32e0cf3252de47bd8a6af5f77e561262372a7d4e))
* **tui:** hold the help to a table of every placed binding ([93f9ca2](https://github.com/jacob-delgado/workflow/commit/93f9ca2b9326f374b28e5227674a15a06a8c3463))
* **tui:** settle the harness on a fake clock ([6f79df2](https://github.com/jacob-delgado/workflow/commit/6f79df2bae651a1beea2c62f2ab7f7dc09ea8081))
* **web:** finish transitions before a screen is saved ([9f7d0af](https://github.com/jacob-delgado/workflow/commit/9f7d0af7ce61419d25e122bf67ab1bee36325735))
* **web:** hold every section to 640, 1024 and 1440 px ([b6398ed](https://github.com/jacob-delgado/workflow/commit/b6398ed2c7f241d01375fa73d2650f43147fb9f7))
* **web:** read the server's own frames through the stream, losslessly ([0b72deb](https://github.com/jacob-delgado/workflow/commit/0b72deba78fc01fc48c91c91e176908259cb56b3))
* **webserver:** hold the stream's frames in a file the client reads ([4491af9](https://github.com/jacob-delgado/workflow/commit/4491af989ee887e73caa0fc0070dc3141ba6a838))
* **wiring:** prove every cached field is sanitized on read-back ([e87c967](https://github.com/jacob-delgado/workflow/commit/e87c967423490b893d3fe049dc13bd26496417b3))

## [0.2.0](https://github.com/jacob-delgado/workflow/compare/v0.1.0...v0.2.0) (2026-09-22)


### ⚠ BREAKING CHANGES

* the doctor --json keys slack_target and slack_mode are renamed messaging_target and messaging_mode, and the messaging credential's service value is the kind rather than "slack". The web REST route /api/slack becomes /api/messaging, its schema Slack becomes MessagingDestination and gains service and configured fields, and the snapshot's slack field becomes messaging. The Go and TypeScript clients are regenerated to match.

### Features

* name the messaging service in doctor, the TUI and web (FEAT-58) ([33a1c92](https://github.com/jacob-delgado/workflow/commit/33a1c928b07c1f78ecd3da7802fd31b3ab3c98bb))


### Build & Packaging

* take the container's tools from the mise.toml pins (FEAT-74) ([b4c4484](https://github.com/jacob-delgado/workflow/commit/b4c4484c4d22fde8caef61abd3a4f7026f8b1dad))


### CI

* gate the release tag on a green build (FEAT-75) ([cd7b75a](https://github.com/jacob-delgado/workflow/commit/cd7b75adf4f065a5e20586aeb8a6886481e81759))

## [0.1.0](https://github.com/jacob-delgado/workflow/compare/v0.0.7...v0.1.0) (2026-09-22)


### ⚠ BREAKING CHANGES

* the `slack` configuration block is renamed to `messaging` and takes a `kind`. An old `slack` block is refused with a message naming the rename. The web config editor and the api/openapi config schema move to `messaging` to match.

### Features

* announce to Teams, Discord or a webhook, not only Slack (FEAT-58) ([01dba38](https://github.com/jacob-delgado/workflow/commit/01dba387db7ca5c563834f1a2d0e78f7f3bd5ab6))
* **tui:** rebind the interface's keys from the configuration (FEAT-48) ([ff10a26](https://github.com/jacob-delgado/workflow/commit/ff10a268581bab6b2dbe436c3a8d590bb08010f2))


### Documentation

* **features:** drop the ideas whose work has shipped ([edc0022](https://github.com/jacob-delgado/workflow/commit/edc0022237c81d8d33d8f60ce0f216aa0a85560e))

## [0.0.7](https://github.com/jacob-delgado/workflow/compare/v0.0.6...v0.0.7) (2026-09-22)


### Features

* **cli:** scriptable branch, pr and announce commands (FEAT-41) ([d3659b3](https://github.com/jacob-delgado/workflow/commit/d3659b3985c8fde9554ff1587c952cae9373650b))
* **forge:** add reviewers, assignees and labels on open (FEAT-27) ([cd6fd9e](https://github.com/jacob-delgado/workflow/commit/cd6fd9e143b887cdf3bd247a1a77a5729b62cadd))
* **forge:** merge a pull request from the Review pane (FEAT-31) ([4c13f32](https://github.com/jacob-delgado/workflow/commit/4c13f32efec987968b283df5c2f8994156d5d3fd))
* **forge:** re-run failed checks from the Review pane (FEAT-32) ([b488b63](https://github.com/jacob-delgado/workflow/commit/b488b63c61fa4327d6396ae9e5bf5d82dc14a249))
* **hooks:** read the MSVC/TypeScript place format (FEAT-24) ([1b62591](https://github.com/jacob-delgado/workflow/commit/1b62591eec9d0c62933f8c16af9f4c06c2fb1817))
* **jira:** add assign and log-work client methods ([72cac41](https://github.com/jacob-delgado/workflow/commit/72cac416f3fc1eb6e454a0e8e94a7b2be379b64e))
* **jira:** fill user, date and multi-value transition fields (FEAT-07) ([a9326b7](https://github.com/jacob-delgado/workflow/commit/a9326b7606d6a6adfd1edb313554cda1ce47a980))
* **jira:** page in the comments past the first (FEAT-06) ([a2d1122](https://github.com/jacob-delgado/workflow/commit/a2d1122cbaca07addaae7672545b6d4efc7533e0))
* **jira:** post Markdown comments as wiki markup (FEAT-08) ([a8403d0](https://github.com/jacob-delgado/workflow/commit/a8403d0c02bd75345fac4b62fe132e32187d026e))
* read and show the whole issue (FEAT-06) ([12be73c](https://github.com/jacob-delgado/workflow/commit/12be73cbe8440a4e964722754b8261683efd2ac0))
* **slack:** announce the merged and CI-red moments (FEAT-38) ([de52d8c](https://github.com/jacob-delgado/workflow/commit/de52d8c87e55fad71fb0b2b73f7fb6c2e4f15d3f))
* **tui:** amend and fix up unpushed commits (FEAT-22) ([133c166](https://github.com/jacob-delgado/workflow/commit/133c166a24c486763eb074853bbcee74172c8849))
* **tui:** assign an issue and log work from the Issues pane ([00d807a](https://github.com/jacob-delgado/workflow/commit/00d807a44098dd4df87d6a10fdd120412dd6140c))
* **tui:** complete the pull request base from remote branches (FEAT-35) ([6f02122](https://github.com/jacob-delgado/workflow/commit/6f02122b2a7f97c1435df3154c9a150760e70a85))
* **tui:** edit an open pull request from the Review pane (FEAT-30) ([a25de0c](https://github.com/jacob-delgado/workflow/commit/a25de0c7310ef07bb176fc10cbf5824cf816c4c6))
* **tui:** finish a merged branch from the Review pane (FEAT-17) ([90c1ce7](https://github.com/jacob-delgado/workflow/commit/90c1ce78206f9b07d0c10bf992444ab18f6adefb))
* **tui:** offer the status change after branching (FEAT-05) ([9779a19](https://github.com/jacob-delgado/workflow/commit/9779a196765598aaddcc0af8c83fa8a33332d17e))
* **tui:** open and copy the issue or pull request link ([63ae680](https://github.com/jacob-delgado/workflow/commit/63ae680f246afceae7674cdcc9dd6e6a08b6b0a3))
* **tui:** show the diff before staging (FEAT-18) ([268afe2](https://github.com/jacob-delgado/workflow/commit/268afe2c2953b6e74e5d4e6ac6a45a78de887e55))
* **tui:** show the review queue in a sixth pane ([044a5b3](https://github.com/jacob-delgado/workflow/commit/044a5b36a64f845b57428d92b2d29a445c341d8c))
* **tui:** suggest a commit scope (FEAT-21) ([6013ac8](https://github.com/jacob-delgado/workflow/commit/6013ac8b723ddd43a47fcece38e47e745590847d))
* **web:** add a light/dark/system theme, verified for a11y ([bb3fc19](https://github.com/jacob-delgado/workflow/commit/bb3fc1951f47658e48b73615292efd367cc8dead))
* **web:** announce the pull request to Slack ([e522ea9](https://github.com/jacob-delgado/workflow/commit/e522ea966f6261c62007a574845dc33efea5472b))
* **web:** commit the staged changes with a message from the browser ([05f22eb](https://github.com/jacob-delgado/workflow/commit/05f22eba72ad9fe3bec442866bb3377082d645d3))
* **web:** open a pull request for the branch ([00f7563](https://github.com/jacob-delgado/workflow/commit/00f75632a7ed3d865a6506066a9ed7fb57467b0d))
* **web:** push the branch to its remote, read-only under dry-run ([12640d6](https://github.com/jacob-delgado/workflow/commit/12640d60edc6d23cf0b5da76604ae171defaf6ed))
* **web:** start work on an issue by creating its branch ([2c73133](https://github.com/jacob-delgado/workflow/commit/2c731331924f1a9edc53a6dcb51b4c27d74b7810))


### Bug Fixes

* **tui:** re-clamp the commits scroll after a shrinking reload ([ea1cc5e](https://github.com/jacob-delgado/workflow/commit/ea1cc5ed911892aa56f0358f58c0e98e16ed50da))
* **web:** announce a merge request on GitLab ([033abfb](https://github.com/jacob-delgado/workflow/commit/033abfb064708b47c7fcee282e1e0272286ad79a))


### Refactors

* **test:** make every test black-box and enforce it ([7344f70](https://github.com/jacob-delgado/workflow/commit/7344f70adf65c7cf10e43487461b1fa419dfca06))
* **tui:** build the help from the bindings, not a second list ([1aa21d5](https://github.com/jacob-delgado/workflow/commit/1aa21d58ed06f2b76a4aa6427fd5d710325b1aae))


### Build & Packaging

* **deps:** bring dependencies to their latest versions ([d7b8f05](https://github.com/jacob-delgado/workflow/commit/d7b8f050b62fa9a6f3d7840433a97307b230ab1b))
* **deps:** migrate the Charm stack to v2 ([7998ab2](https://github.com/jacob-delgado/workflow/commit/7998ab2376ddb9a1e72d6cfe202cf25dc77b5ac0))
* **deps:** update pinned tools to their latest safe versions ([815651d](https://github.com/jacob-delgado/workflow/commit/815651dbf25957f6190a445c2cbd343f4ad25001))
* **lint:** warn at 500, fail at 800 for file length ([f65e56b](https://github.com/jacob-delgado/workflow/commit/f65e56bba021e6488c7fa2d1765a8b37c34b3134))


### CI

* report test counts and coverage, and count the web in cloc ([3534de1](https://github.com/jacob-delgado/workflow/commit/3534de10c0da39f19397ae905a1538b8806a30b9))

## [0.0.6](https://github.com/jacob-delgado/workflow/compare/v0.0.5...v0.0.6) (2026-09-20)


### Features

* **api:** describe the web API read and config surface ([dbbb86f](https://github.com/jacob-delgado/workflow/commit/dbbb86f3bae536a84030f7d8dfa3aedf74e89731))
* **cli:** add a --web flag that serves the web interface ([1d924df](https://github.com/jacob-delgado/workflow/commit/1d924df00449c7e03521a867c437766d4565fcb8))
* **cli:** cancel the command context on SIGINT and SIGTERM ([10d3194](https://github.com/jacob-delgado/workflow/commit/10d3194d570812346e48c566fa34f46683452374))
* **clients:** tell a rate-limited caller how long to wait ([c65bf74](https://github.com/jacob-delgado/workflow/commit/c65bf7442580114b427511e960440f1e78f790f0))
* **cli:** tell doctor a signed-out gh from an absent one ([0acbf71](https://github.com/jacob-delgado/workflow/commit/0acbf710ff5947b847ea5a9193aef86c52fcd8f0))
* **config:** find the repository's config from a subdirectory ([bd64f09](https://github.com/jacob-delgado/workflow/commit/bd64f092b4e6e21339f9df4008a44e3072456dde))
* **config:** give the file format a version field ([45e8ee1](https://github.com/jacob-delgado/workflow/commit/45e8ee14fc7744a78a530fd1ac17839ed2cea7ca))
* **config:** make the number of comments shown a setting ([84ced5f](https://github.com/jacob-delgado/workflow/commit/84ced5faea75f4f9555bb7883b0de3e041db2a49))
* **convention:** read a branch as an issue only for its project ([a9a29bb](https://github.com/jacob-delgado/workflow/commit/a9a29bb5a84b48d87d74d1de2a11297a4a0a355e))
* **editor:** honor GIT_EDITOR and keep a spaced editor path whole ([cc468aa](https://github.com/jacob-delgado/workflow/commit/cc468aa7734d4170b9de143ef2d322f2d8cf8700))
* **gitrepo:** push to remote.pushDefault when the repository sets one ([49714a0](https://github.com/jacob-delgado/workflow/commit/49714a09aa977434a3a16e43dc4fad82e48b1813))
* **hooks:** read an extensionless Dockerfile or Makefile as a place ([b27fe03](https://github.com/jacob-delgado/workflow/commit/b27fe034ba10e9ca2cb30cba732d6627a49927e7))
* **proc:** bound a Run so a hung quick read recovers on its own ([1d4d49a](https://github.com/jacob-delgado/workflow/commit/1d4d49aaa130f721b3fe17797bc938e44dabb300))
* **proc:** kill a streamed run's whole process group on cancel ([0ceb405](https://github.com/jacob-delgado/workflow/commit/0ceb40504b96844761858281ba77656524eaa84a))
* **tui:** add a discoverable key to load the next page of issues ([5032707](https://github.com/jacob-delgado/workflow/commit/5032707918997ce9244b45fa4f0b8c5a3204a9b1))
* **tui:** let the commit composer mark a breaking change ([f32264e](https://github.com/jacob-delgado/workflow/commit/f32264e38f81fb7e6e3b278b051237f2419fad2b))
* **tui:** stop a streaming run with a key ([4cbacc4](https://github.com/jacob-delgado/workflow/commit/4cbacc40b43b4699f9eb591f83b8f611afd36342))
* **web:** add the dark cockpit theme ([554a6b1](https://github.com/jacob-delgado/workflow/commit/554a6b1f085bc43d5f04971799fe30b119053654))
* **web:** add the Issues master-detail panel ([2d73cd3](https://github.com/jacob-delgado/workflow/commit/2d73cd399260bd33ae833c125c7aec34d906c2b4))
* **web:** add the per-issue work-story pipeline ([22695de](https://github.com/jacob-delgado/workflow/commit/22695de0f55a2ad8cf9940616e2985e575eb01cd))
* **web:** add the Review and Slack panels ([8399b60](https://github.com/jacob-delgado/workflow/commit/8399b603dbf889ebdc2f49125e5fbecbf4d1d22a))
* **web:** add the Settings config editor ([a1314f6](https://github.com/jacob-delgado/workflow/commit/a1314f64db7353d2f195d73411b4aa6547e2ff17))
* **web:** build the cockpit shell with the section nav rail ([edc114a](https://github.com/jacob-delgado/workflow/commit/edc114a5b718bf28ec4e36eddc60563315fc30f4))
* **web:** check out a branch to switch issues from the web ([7bca9b6](https://github.com/jacob-delgado/workflow/commit/7bca9b673611477f8dd9e79e30c1b9e64cec921e))
* **web:** generate the typed API client and wire the query client ([a21a253](https://github.com/jacob-delgado/workflow/commit/a21a2536faf473244ad11ac3316d820c7a56fdf5))
* **web:** mark every in-flight issue from its local branch ([48c772b](https://github.com/jacob-delgado/workflow/commit/48c772bfe65d174d6c7cf27eb328ac04ab962606))
* **web:** one-shell live-reload dev and a mock-data review mode ([ed57856](https://github.com/jacob-delgado/workflow/commit/ed578566ff40c5300752c034d97c8cadde8785e1))
* **web:** scaffold the React and TypeScript frontend ([06a0785](https://github.com/jacob-delgado/workflow/commit/06a078506643209d28957d7aa53b2c652838ea9c))
* **web:** serve the embedded UI, guarded to the loopback interface ([af86c00](https://github.com/jacob-delgado/workflow/commit/af86c0042a5970a279add7651b2d44f8d635a2f8))
* **webserver:** push read state over a Server-Sent Events stream ([118f6eb](https://github.com/jacob-delgado/workflow/commit/118f6eb4556171d00033b44425fb1092268e0aae))
* **webserver:** serve the read and config API over the wiring seams ([7d4aa10](https://github.com/jacob-delgado/workflow/commit/7d4aa102a2ae088add6a50cb81b4c44933d48981))
* **webserver:** validate requests against the OpenAPI contract ([fba6a84](https://github.com/jacob-delgado/workflow/commit/fba6a849a5526fed31158027081ae5563bbdf1dd))
* **web:** show the current branch, its commits, and changes ([3b4013d](https://github.com/jacob-delgado/workflow/commit/3b4013d8d1fbdc01e28e93bb4cc124cd98de9b48))
* **web:** stream the read state over SSE into a shared store ([18f59b9](https://github.com/jacob-delgado/workflow/commit/18f59b98f1e339a40b1562a0af4a8be9dbfdf5cd))


### Bug Fixes

* **cli:** doctor reports configuration that is set but invalid ([a08f582](https://github.com/jacob-delgado/workflow/commit/a08f5826e260e9f1353a03b94a3c2c91445f8eb7))
* **clients:** stop a refused redirect reading as "could not reach" ([0f47507](https://github.com/jacob-delgado/workflow/commit/0f4750715afb768ae260b9f1d4f68c8c47470af9))
* **clients:** tell a 429 apart, assert every Stringer ([0df2ba8](https://github.com/jacob-delgado/workflow/commit/0df2ba8cc3a4c3af8f35a6364895bff189e69f95))
* **forge:** read a manual GitLab pipeline as no result, not running ([f220856](https://github.com/jacob-delgado/workflow/commit/f22085677f540d0a08486935aeb2c8f5ec26b6ea))
* **hooks:** convert only sh hooks that set errexit and read nothing ([adad525](https://github.com/jacob-delgado/workflow/commit/adad525170035e42373298423cd93d30cb79f0eb))
* **hooks:** read a Windows drive letter as part of a path ([47d3b8e](https://github.com/jacob-delgado/workflow/commit/47d3b8eaa3b89a822363a4f92b064d24a76c09e9))
* **proc:** let Output.Wait be called more than once ([5208999](https://github.com/jacob-delgado/workflow/commit/52089991ec2638401f2df67ac20f1ca60732c2bf))
* **tui:** clear a stale Slack error when the branch changes ([f03ea4d](https://github.com/jacob-delgado/workflow/commit/f03ea4dbe18abc940c0fcfc4a9476ba6155e9dc1))
* **tui:** hold every write back at the seam under dry run ([cd35a9f](https://github.com/jacob-delgado/workflow/commit/cd35a9f1ddd41401fd1dddd8d58a94ff33ffa78f))
* **tui:** keep CI polling to one chain per review ([2f46ccb](https://github.com/jacob-delgado/workflow/commit/2f46ccbc80887c77788ea2a1b7baf223226d5987))
* **tui:** map a click below wrapped jobs to the right failure ([a320fe4](https://github.com/jacob-delgado/workflow/commit/a320fe434c1a2d5719b8167890043479f1f017d3))
* **tui:** offer only failure places that resolve to a file ([d2abe36](https://github.com/jacob-delgado/workflow/commit/d2abe36d2c5522aa46078605cd1f627e57690c45))
* **web:** correct the work story, detached HEAD, and a bad SSE frame ([53ca870](https://github.com/jacob-delgado/workflow/commit/53ca87020378dda0e4f3e6101f179f0095cd7186))
* **web:** key the work story to the issue that owns the branch ([3d6c698](https://github.com/jacob-delgado/workflow/commit/3d6c698c122c82672f272106b09c42dd265be137))
* **web:** link config hints, fix a heading level, refresh config cache ([094131e](https://github.com/jacob-delgado/workflow/commit/094131e488fac3783f716d06c7559c122ed9f693))
* **webserver:** keep the stored jira.base_url password on a config save ([12dbcc2](https://github.com/jacob-delgado/workflow/commit/12dbcc26f56d1b5f4eb8040fd0e8788a55d27a58))
* **webserver:** write config only to the server's own file path ([976b653](https://github.com/jacob-delgado/workflow/commit/976b653d96cb7d4a4cad6fb3ffa23bd5f0714f6f))
* **windows:** find hooks and pick an editor on Windows ([6bd3a40](https://github.com/jacob-delgado/workflow/commit/6bd3a408bbc3ac4fef1f1715881e6296f7e89dbd))


### Performance

* **tui:** fold hook-run jobs in as they arrive, cap the output ([0656bac](https://github.com/jacob-delgado/workflow/commit/0656bac0543ac0a7dc69e33a5fe79b1db1360711))


### Refactors

* **cli:** inject doctor's online transport ([c11bc5e](https://github.com/jacob-delgado/workflow/commit/c11bc5eedab170ca69d0aa25f7d0713fd3649dbb))
* **config:** expose Parse for decoding a config from bytes ([0062264](https://github.com/jacob-delgado/workflow/commit/00622649b1c76f3081b107bf48f5ad1755993bb6))
* **config:** make the credentials a typed masking Secret ([1b29c46](https://github.com/jacob-delgado/workflow/commit/1b29c46c1da725a708750896019a78b3164249e1))
* **gitrepo:** carry run and dir on a Repository value ([9b55b9f](https://github.com/jacob-delgado/workflow/commit/9b55b9feab0a3010de909004c390fd189adc6d5f))
* **gitrepo:** name the remote once as DefaultRemote ([6a1deb7](https://github.com/jacob-delgado/workflow/commit/6a1deb7e8b09259a60bfd8a6267738a273cc4beb))
* **httpx:** share the redirect-refusing HTTP client ([702345e](https://github.com/jacob-delgado/workflow/commit/702345e35b2a947474536974d044b91c47131c97))
* **httpx:** strip the request URL from every transport error once ([703b3cd](https://github.com/jacob-delgado/workflow/commit/703b3cd4c3c7332a50a28c127344bf03cd570533))
* **jira:** drop the unused User.Active and correct stale comments ([b68d983](https://github.com/jacob-delgado/workflow/commit/b68d983b0396ea2bda5bb28483ed07fe8a617425))
* **jira:** give the issue key a type ([d226b7b](https://github.com/jacob-delgado/workflow/commit/d226b7b3fb787176fed7802c721c087f5ea18df7))
* **jira:** make the status category a type ([f22de81](https://github.com/jacob-delgado/workflow/commit/f22de815b79d20f423ff72c4f4c50b544be2f27f))
* **jira:** share the exchange-and-decode every read repeats ([3a62412](https://github.com/jacob-delgado/workflow/commit/3a62412ad575f570f49cabb93932222b100761c6))
* **tui:** derive the pane digit keys and the Review reference ([22a6bb2](https://github.com/jacob-delgado/workflow/commit/22a6bb2187cd92cbe10577e2c1fd16f8ca73c0d3))
* **tui:** give every overlay one sendState ([26fdd23](https://github.com/jacob-delgado/workflow/commit/26fdd23a7afcca85ed3d6b898e8366b6edefabc4))
* **tui:** make the key list an overlay, drop helpOpen ([c6d8ba0](https://github.com/jacob-delgado/workflow/commit/c6d8ba0b0765deee7bebfbb6ef74626e80b8ed45))
* **tui:** measure the click offset from the header the view drew ([ef9aa0f](https://github.com/jacob-delgado/workflow/commit/ef9aa0f1c7ad28b48e2d826b91e8a5cc7e2f453a))
* **tui:** read the branch's issue and base in one place ([f90c5e3](https://github.com/jacob-delgado/workflow/commit/f90c5e34841db213b2c53ea05ca77fdcb5f99359))
* **tui:** route the three body editors through one message ([7538ae1](https://github.com/jacob-delgado/workflow/commit/7538ae1b9154debd8e6648038bf54a617cb80e38))
* **wiring:** share the forge parse-then-kind preamble ([807deaf](https://github.com/jacob-delgado/workflow/commit/807deaf90100e67ba0f5d496d8196b456842676b))
* **wiring:** share the forge resolver, split forge wiring out ([bf299a5](https://github.com/jacob-delgado/workflow/commit/bf299a5a39ed662d6a657af3e7fb33160ed68ea7))


### Documentation

* correct drifted platform, scissors and gobco notes ([66c70cb](https://github.com/jacob-delgado/workflow/commit/66c70cb7d1a038e92cf6829dfae9a845c7439910))
* **debt:** hold DEBT-23 as a YAGNI conflict until a real rename ([31dcbf2](https://github.com/jacob-delgado/workflow/commit/31dcbf25fc503e2516146242132eb036d8069f64))
* **debt:** retire the entries already fixed on main ([3e330d7](https://github.com/jacob-delgado/workflow/commit/3e330d71d694f7b651e372b1727692a211efeea7))
* **debt:** trim DEBT-33 to what current code leaves open ([1fe5b92](https://github.com/jacob-delgado/workflow/commit/1fe5b92ec4d78fe287280c59eabd13369f6873b2))
* **features:** remove the shipped features, trim the partly done ([cf5ee91](https://github.com/jacob-delgado/workflow/commit/cf5ee91f6de21880acc6f7b6f81d5bb860fd0373))
* list the missing keys, stop enumerating tools ([2acb0fe](https://github.com/jacob-delgado/workflow/commit/2acb0fe17cd22b5e8b5afc563d249232e2e1585c))
* name an installable version, not v0.1.0 ([86a05fe](https://github.com/jacob-delgado/workflow/commit/86a05fe01850c2518275679b008f75f71a246242))
* note the new internal/httpx package in the layout ([76b2b94](https://github.com/jacob-delgado/workflow/commit/76b2b94ec092c7c767b9d8c78bd02ec964d25528))
* point the README's Jira setup at the SSO guidance ([aeaec0f](https://github.com/jacob-delgado/workflow/commit/aeaec0fca29c0c23fb39685988ddbfcd0778f064))
* record switching issues by branch in the web as FEAT-77 ([b34678f](https://github.com/jacob-delgado/workflow/commit/b34678fa04f876563caeb3215640dd71f69f1d39))
* relocate the debt file's future work and decisions ([388ee44](https://github.com/jacob-delgado/workflow/commit/388ee448c2a611ab71baa3b95c4ddaecb1fa2c5c))
* remove TECH_DEBT.md ([7afac6c](https://github.com/jacob-delgado/workflow/commit/7afac6ceb27e5b3843a05668c78f5b5f3927ece2))
* sanction an opt-in local web mode ([55986fb](https://github.com/jacob-delgado/workflow/commit/55986fb4d3db2fa004c85d0e235f1eb23544a25c))


### Build & Packaging

* add an advisory task fuzz that runs every target ([4433983](https://github.com/jacob-delgado/workflow/commit/4433983ad6e6b64031a999c2b42fe436305b32a1))
* Bump ghcr.io/devcontainers/features/docker-in-docker ([f06e1fb](https://github.com/jacob-delgado/workflow/commit/f06e1fbeed666cadae363857efc39c0ede64d489))
* Bump the go-minor-patch group with 2 updates ([0ad3f2c](https://github.com/jacob-delgado/workflow/commit/0ad3f2c3f2c9a10a5d9e7fd986b05edd5ef41181))
* check enum maps, guard fork PRs, widen dependabot ([5d38bc3](https://github.com/jacob-delgado/workflow/commit/5d38bc3fca85461e082ece845bce22c666ee6c83))
* **gen:** generate the strict server and models from the contract ([f5fd438](https://github.com/jacob-delgado/workflow/commit/f5fd4387437c2dc53ff7e44a940d5e60d702aa51))
* **lint:** exempt generated Go from the header and length gates ([b7c556d](https://github.com/jacob-delgado/workflow/commit/b7c556d50eb17103db6ecb4b8556371ba7a7c1ef))
* **lint:** guard os/exec to internal/proc with depguard ([df37f7c](https://github.com/jacob-delgado/workflow/commit/df37f7cad68a400ad9e9ee6503aa83f776c7cb7f))
* make gobco account for every package, tests or none ([c1f99c4](https://github.com/jacob-delgado/workflow/commit/c1f99c443a7f9655fdae25c01e9ca4114421e873))
* pin node and jq in mise.toml ([b3b4f92](https://github.com/jacob-delgado/workflow/commit/b3b4f92d70e21bed014d4ee8cb99b920c97ae758))
* **scripts:** gate goroutines to internal/proc ([dde05e6](https://github.com/jacob-delgado/workflow/commit/dde05e6bf892dcaac1289ab3767ebd23dac7c5bf))
* **scripts:** gate the Go version against drift across its sources ([5d81d52](https://github.com/jacob-delgado/workflow/commit/5d81d52352a549fc0cc94c4ad0d8dd5cede55c2e))
* stop a gate passing having measured nothing ([2b24da4](https://github.com/jacob-delgado/workflow/commit/2b24da401f07f8f2079ed713c06008352cae9f82))
* **web:** force js-yaml to 4.3.2 to clear high-severity advisories ([e6aa202](https://github.com/jacob-delgado/workflow/commit/e6aa202b108061ba25894e8977527441389043af))
* **web:** run the cockpit with two tasks ([69e32e1](https://github.com/jacob-delgado/workflow/commit/69e32e13d60860483c24779108a993abe186b768))


### CI

* build the container and run the gate inside it weekly ([21e01e3](https://github.com/jacob-delgado/workflow/commit/21e01e38a429d9d06a7566db8ff9f9918c9171e3))
* Bump the actions-minor-patch group across 1 directory with 4 updates ([e09a37c](https://github.com/jacob-delgado/workflow/commit/e09a37c415eea3715442c8c610949c7bcac6402d))
* gate the generated web client against the contract ([ddbf124](https://github.com/jacob-delgado/workflow/commit/ddbf124a7acdfcd13bb1e3cf979f931e1bb57ab8))
* gate the web frontend's lint, test, and build ([8663bb5](https://github.com/jacob-delgado/workflow/commit/8663bb555d2344e5a6f8402a269a0e5c493c0e1c))
* pin the mise binary version in every workflow ([18177b2](https://github.com/jacob-delgado/workflow/commit/18177b2684ff97f83c9b85808e58ea0d475039f3))
* scan commit history for secrets, not just the tree ([70a0b61](https://github.com/jacob-delgado/workflow/commit/70a0b61f324c1c0377897078dbca305add0ac15c))


### Tests

* **cli:** run the root command through an injected runner ([fb30718](https://github.com/jacob-delgado/workflow/commit/fb3071887addb35585c7183a9b3ffc79ba84ca9d))
* **scripts:** cover the gate scripts' own failure paths ([66aa146](https://github.com/jacob-delgado/workflow/commit/66aa146665440163177e2b66653d0268e61e08a2))
* **tui:** split the test files that neared the length gate ([a6602a3](https://github.com/jacob-delgado/workflow/commit/a6602a32ba99ec6499b0da55df4a9309b59003ff))
* **web:** add Playwright and axe end-to-end tests ([0f0ef17](https://github.com/jacob-delgado/workflow/commit/0f0ef1757a96890c693d127aebe51c387e53738d))
* **wiring:** drive the forge seams' success path ([fa6180e](https://github.com/jacob-delgado/workflow/commit/fa6180ecb95283a760424d4f22e1c6f06dbd89cb))

## [0.0.5](https://github.com/jacob-delgado/workflow/compare/v0.0.4...v0.0.5) (2026-09-18)


### Features

* **convention:** read a forge issue number from a branch ([ce40dbb](https://github.com/jacob-delgado/workflow/commit/ce40dbb73190713d7b1ca1cf45058cadd189f73a))
* draft a standup to share ([b9d05e1](https://github.com/jacob-delgado/workflow/commit/b9d05e1fb373863564d354150f847f72a72ef174))
* **forge:** read and close a repository's assigned issues ([9eab18e](https://github.com/jacob-delgado/workflow/commit/9eab18e385cd8e66c76e5ba5f1bad04b44b56ad9))
* **forge:** route forge calls through gh or glab when asked ([015a5f4](https://github.com/jacob-delgado/workflow/commit/015a5f4490821e653cb2924b78f3a5e55e4046ed))
* list the pull requests waiting on your review ([33c951a](https://github.com/jacob-delgado/workflow/commit/33c951a1312e355df9bb242d640382fede1e231b))
* print the work's status on one line ([c0574a8](https://github.com/jacob-delgado/workflow/commit/c0574a860e2839a44ab2424b08260e3306503e8f))
* report several repositories at once ([36f8257](https://github.com/jacob-delgado/workflow/commit/36f8257d62d0b3f0f982fd53c5fafb37f8c9a9d9))
* **tui:** back the Issues pane with forge issues when there is no Jira ([639af6f](https://github.com/jacob-delgado/workflow/commit/639af6f6dd5f553bf27d6fb495ea083d00167d5d))


### Documentation

* drop the retired Go Report Card badge ([a80971f](https://github.com/jacob-delgado/workflow/commit/a80971f01eea36e358fdea60ba19a55b8064e452))

## [0.0.4](https://github.com/jacob-delgado/workflow/compare/v0.0.3...v0.0.4) (2026-09-18)


### Features

* a default scope for commits ([e5b0d0f](https://github.com/jacob-delgado/workflow/commit/e5b0d0f9ec8162067054aebf6713487e7c076b8c))
* catch up with the base ([2ba3b18](https://github.com/jacob-delgado/workflow/commit/2ba3b18678366bf5d3bc05684f71db341958590a))
* keep the Jira token in the operating system's keychain ([332346c](https://github.com/jacob-delgado/workflow/commit/332346cbae5d1752a40320fd04ecd6e21c5a8704))
* see which check failed ([3c1cd87](https://github.com/jacob-delgado/workflow/commit/3c1cd878ba6244dbce2e6d03f1c48a7d3cfe7dd9))
* send extra headers to a Jira behind an SSO proxy ([a370543](https://github.com/jacob-delgado/workflow/commit/a370543d64e33f87c11c0f2d21a0605613e02325))
* set up the configuration by asking, and checking, each value ([f4c1d02](https://github.com/jacob-delgado/workflow/commit/f4c1d02e99a3a65c01d3dc28fce7cbf6395839c8))


### CI

* lead each release with install and verify steps ([4e7de0b](https://github.com/jacob-delgado/workflow/commit/4e7de0b016d347cb0223f111e47b12556bb2b5ee))

## [0.0.3](https://github.com/jacob-delgado/workflow/compare/v0.0.2...v0.0.3) (2026-09-18)


### Features

* move between named issue views ([8512394](https://github.com/jacob-delgado/workflow/commit/85123946609004cf0e5123248fdeb2250b049bf6))
* ring the terminal when CI finishes ([1ada424](https://github.com/jacob-delgado/workflow/commit/1ada424aed61cfaa3656101f33a60825b67dc9cb))
* show a pull request's review state ([1170355](https://github.com/jacob-delgado/workflow/commit/11703558e79d9ab5b2bc7118ca14ce5c7dc52d0c))
* start a task in a worktree ([caf7e57](https://github.com/jacob-delgado/workflow/commit/caf7e57d82fee12c0b8bc2cd7153f2a6ab389147))


### Bug Fixes

* build a binary per release platform, with an SBOM ([e0906ce](https://github.com/jacob-delgado/workflow/commit/e0906ce69d2389797c58e8b60591502ff24e90a1))


### CI

* stop typos flagging commit hashes as misspellings ([854730e](https://github.com/jacob-delgado/workflow/commit/854730e61b3edd1adc2daa5bf9c3e0ef7c421f06))

## [0.0.2](https://github.com/jacob-delgado/workflow/compare/v0.0.1...v0.0.2) (2026-09-17)


### Features

* bootstrap the workflow CLI, gates, and release automation ([a0050b9](https://github.com/jacob-delgado/workflow/commit/a0050b917c4fba0539899e8aca556e10141903a5))
* **cli:** add --dry-run to open the interface with writes held back ([2b65c3d](https://github.com/jacob-delgado/workflow/commit/2b65c3d28f456a580f647c6ae39cd4595b4643d4))
* **cli:** add a buildinfo package for the running version ([b93c6d9](https://github.com/jacob-delgado/workflow/commit/b93c6d995088db2590b314cada22d716a8b0e428))
* **cli:** report the version with --version and in doctor ([56b9327](https://github.com/jacob-delgado/workflow/commit/56b932718d9fa56f4469528456b84b6d2a9ff229))
* **config:** add ui.mouse and ui.ascii ([11b1e93](https://github.com/jacob-delgado/workflow/commit/11b1e9337a9f89f261285c949c66b333afd099e1))
* **config:** make the request timeout and CI interval settable ([d881845](https://github.com/jacob-delgado/workflow/commit/d88184520d807b53bb6bf0d5c234f9b7f2f3a39d))
* **config:** take a token from a command or environment variable ([23d306b](https://github.com/jacob-delgado/workflow/commit/23d306b7081a4d6dddeb2c2838edde9c047243a0))
* **convention:** name branches, find issue keys, assemble subjects ([1ae9c01](https://github.com/jacob-delgado/workflow/commit/1ae9c01812b9eb64e8e2174d27a0e5426e6a9568))
* **convention:** propose a pull request's title and description ([ab2a16b](https://github.com/jacob-delgado/workflow/commit/ab2a16b751c398b97b08917647c45f4b6c57da17))
* **convention:** shape branch names to a team's convention ([8c7820d](https://github.com/jacob-delgado/workflow/commit/8c7820dd28b1349f6cc7b9888d3bffc642c50391))
* **doctor:** check the Jira credential with --online ([f0784d9](https://github.com/jacob-delgado/workflow/commit/f0784d996fd192f7f449f33b6a8a66cd39450909))
* **doctor:** check the Slack credential with --online ([6c87d32](https://github.com/jacob-delgado/workflow/commit/6c87d32ff63e25443c4017d685be4897c9f97fba))
* **doctor:** report the repository and external tooling ([48e66a1](https://github.com/jacob-delgado/workflow/commit/48e66a1420cdeb7a4d0a68d0369e49684be4410e))
* **doctor:** report the same facts as JSON ([902adb5](https://github.com/jacob-delgado/workflow/commit/902adb5f411b13265380196779cf4315d2143804))
* **editor:** hand drafts and files to $EDITOR ([565b662](https://github.com/jacob-delgado/workflow/commit/565b66271d9caf090675ad15fa6d12ee09c332ce))
* fetch the base before branching ([2be0725](https://github.com/jacob-delgado/workflow/commit/2be0725fd8abf13763f6802da9c7a4932c21a82c))
* **forge:** check the forge credential against its API with --online ([924f538](https://github.com/jacob-delgado/workflow/commit/924f5383411abdf6f84df1956eea0286b0c54e98))
* **forge:** find, open and check pull requests; read templates ([9d34c3e](https://github.com/jacob-delgado/workflow/commit/9d34c3eb118bf787058a1375cf8696bf106c2205))
* **forge:** read the forge and its API base from the git remote ([d74c18a](https://github.com/jacob-delgado/workflow/commit/d74c18a61272ca3436adfdc9f9213a8296f968e4))
* **forge:** resolve the forge token from the environment, gh, or config ([b6613c3](https://github.com/jacob-delgado/workflow/commit/b6613c3ac556f3bc0677b4aa215fbf4814f75002))
* **gitrepo:** read changes and the branch; stage, unstage, branch ([3814bd8](https://github.com/jacob-delgado/workflow/commit/3814bd85da1db32e749ef8e6a3a57e7dd7374e12))
* **hooks:** read lefthook's output and config; generate lefthook.yml ([913e481](https://github.com/jacob-delgado/workflow/commit/913e481e9bf4806b86e25186ee1b2a7993f6f345))
* **jira:** read an issue in full, comment on it, fill transition fields ([e76ea6f](https://github.com/jacob-delgado/workflow/commit/e76ea6fc4e4c235df3c6ab858b1af97d65eb1ef0))
* **jira:** say why Jira rejected a request ([eba8832](https://github.com/jacob-delgado/workflow/commit/eba883246188988d9b1a7c925d6042c9e4c6dfb4))
* keep bold, faint and the cursor when color is off ([440553d](https://github.com/jacob-delgado/workflow/commit/440553de90cb99e1aee584c69fc57b771eee48cc))
* link the pull request on its issue ([41631aa](https://github.com/jacob-delgado/workflow/commit/41631aa50f6c901672d1381ea8bac43de5cc02f8))
* load past the first fifty issues ([f58dada](https://github.com/jacob-delgado/workflow/commit/f58dada97e097d8e348f4aaf8d05f5b5bb31dc22))
* log each request's outline for a bug report ([5147bf1](https://github.com/jacob-delgado/workflow/commit/5147bf1f70279e4eb1b2e39208227a9431dd5da2))
* name both setup steps wherever config is missing ([d06dae6](https://github.com/jacob-delgado/workflow/commit/d06dae656955520f3cb2a6d6194e1ae1cfa2a7ab))
* **proc:** stream a program's output, and build one for the terminal ([5329919](https://github.com/jacob-delgado/workflow/commit/532991958c195832ee556b930be4ebf8055a7094))
* show how old the base is when starting a branch ([f249d1b](https://github.com/jacob-delgado/workflow/commit/f249d1b52805815799d9116aa575281143f99a9a))
* **slack:** accept an incoming webhook as well as a bot token ([b1cdcc2](https://github.com/jacob-delgado/workflow/commit/b1cdcc219956d24c1f95d8af0ad56125794f6fe5))
* **slack:** choose the channel when posting ([f044693](https://github.com/jacob-delgado/workflow/commit/f0446934265ce7f6128fa6f47a1cd44a76cdc886))
* **slack:** post through a bot token or a webhook, and announce a PR ([16b2096](https://github.com/jacob-delgado/workflow/commit/16b20961af4a3bad8d08c702646366f04c40f46f))
* **slack:** shape the announcement to a team's own words ([efdeeb4](https://github.com/jacob-delgado/workflow/commit/efdeeb476b786f7038ebb414e1703a9efed90fad))
* switch to another task's branch ([c9eea2e](https://github.com/jacob-delgado/workflow/commit/c9eea2ef172875b5b16b290edf1d8340a807345b))
* **tui:** bold the focused pane's title ([d71de7a](https://github.com/jacob-delgado/workflow/commit/d71de7a1893eddbd7f0a75fdab00bef76a95e57c))
* **tui:** call it a merge request on GitLab ([b391df4](https://github.com/jacob-delgado/workflow/commit/b391df4e37dade9abaf0869ba993d28a0a487794))
* **tui:** draw the footer keys from the theme, with contrast ([70f02e2](https://github.com/jacob-delgado/workflow/commit/70f02e274e29ee0470f8845025abac6612c36647))
* **tui:** draw the rail as one box with shared rules ([c5bb439](https://github.com/jacob-delgado/workflow/commit/c5bb43961231b7ebeff4f9b796ca3f064ff3e65e))
* **tui:** filter the issue list as you type ([0ae742a](https://github.com/jacob-delgado/workflow/commit/0ae742aa0781a9e58abb7ebfd3bce8c8b94d74ab))
* **tui:** give a result its own row, kept until the next action ([31ea44b](https://github.com/jacob-delgado/workflow/commit/31ea44b38c02e4c2122a28bb3c2645325ffd2143))
* **tui:** give the focused pane the room, and draw narrow and plain ([a5ac789](https://github.com/jacob-delgado/workflow/commit/a5ac789098272e302da51d5e71f8cc3b333b5e94))
* **tui:** keep a dropped post visible, and guard quit ([cbda70b](https://github.com/jacob-delgado/workflow/commit/cbda70b3c093965240b13520f7103d0f68da100e))
* **tui:** keep the pull request draft when a push fails ([a793759](https://github.com/jacob-delgado/workflow/commit/a7937595c525f67511eb36e61d1fca48051da589))
* **tui:** keep the rail beside the detail down to 80 columns ([bcd614b](https://github.com/jacob-delgado/workflow/commit/bcd614b7f3322c731bb08e226fea21a560118873))
* **tui:** lead a failed run with the step, and let output be read ([908003d](https://github.com/jacob-delgado/workflow/commit/908003d3564f1b49ee53ab30cdfdf40c153e6a96))
* **tui:** list the Jira issues assigned to you in the Issues pane ([9f0e388](https://github.com/jacob-delgado/workflow/commit/9f0e3882cdbfc2cae015eccb55326c81ef5bbf18))
* **tui:** make the failure glyph red at every site ([b35bdc9](https://github.com/jacob-delgado/workflow/commit/b35bdc9f088310609932267eb72c0b2afbfcd361))
* **tui:** mark a pane's title in flight while it reloads ([eadcfc6](https://github.com/jacob-delgado/workflow/commit/eadcfc63472c0af02bf1ba7c0e4f13ae336dc597))
* **tui:** move the selected issue to a new status ([17fc2f0](https://github.com/jacob-delgado/workflow/commit/17fc2f03228466757ee0ac917ff11ef791368749))
* **tui:** name the stages on the compact spine and collapsed pane ([8c8cab0](https://github.com/jacob-delgado/workflow/commit/8c8cab0e5336ea247fa06fd911ebaddf47d79306))
* **tui:** offer lefthook from the Commits pane, not at start ([83927e5](https://github.com/jacob-delgado/workflow/commit/83927e5465255e46d64f7524c05fcade8366f847))
* **tui:** open the composer on the branch's type ([adc5bb1](https://github.com/jacob-delgado/workflow/commit/adc5bb1855a824798b4da38715dfd8ff5899da3a))
* **tui:** pin an overlay's outcome under its title ([7407f5b](https://github.com/jacob-delgado/workflow/commit/7407f5bcd71a8d845819937022671294a522ba96))
* **tui:** put the heavy focus border where the cursor is ([3c6bb28](https://github.com/jacob-delgado/workflow/commit/3c6bb28925041c5d39b34b2166d8ae136ec125e6))
* **tui:** reach the issue and setup steps on a narrow terminal ([bf57885](https://github.com/jacob-delgado/workflow/commit/bf5788580dce95e44107667c4b4238eb2a4782ab))
* **tui:** read a known error as a sentence, keep the rest raw ([3c2fca9](https://github.com/jacob-delgado/workflow/commit/3c2fca9fbc2c031cfe1d37be52ec96483b7a84a3))
* **tui:** replace the stub with the pane rail, spine, and keys ([32e1aca](https://github.com/jacob-delgado/workflow/commit/32e1aca0bb3c0193e1482dbd940d7d48522baa25))
* **tui:** run the loop from issue to Slack in the interface ([48d4056](https://github.com/jacob-delgado/workflow/commit/48d405667edd2f553fc5c2542d4c411b25d68a0c))
* **tui:** say it plainly when run outside a repository ([2c59e26](https://github.com/jacob-delgado/workflow/commit/2c59e268304e85342bdc5740f378738e4f43d66d))
* **tui:** show a push for a last look before it is sent ([75146c1](https://github.com/jacob-delgado/workflow/commit/75146c1185d4a2c12b540720daa4675dcc375095))
* **tui:** show every key in the help, grouped by place ([6f79499](https://github.com/jacob-delgado/workflow/commit/6f79499f22f7f006bf0093862084fe775875216b))
* **tui:** show the reload key in every pane that reloads ([0a4263b](https://github.com/jacob-delgado/workflow/commit/0a4263bf66fee908e6d7d648e662e1f78492b7fd))
* **tui:** stamp the CI line, and hide post-when-green with no CI ([6675235](https://github.com/jacob-delgado/workflow/commit/6675235e8af57d889e9f63ee74c2f6e376ef3375))
* **tui:** stop drawing empty-state sentences faint ([3ad621b](https://github.com/jacob-delgado/workflow/commit/3ad621bbd9417d1f620579e4bb2d48fee21f7416))
* **tui:** tell the forge's failures apart, and offer nothing on one ([debdca6](https://github.com/jacob-delgado/workflow/commit/debdca67903d0e7a4f002982906dcf936b4c7c88))


### Bug Fixes

* **cli:** name the command that creates a configuration ([a3273b3](https://github.com/jacob-delgado/workflow/commit/a3273b3f3d5cc7649bc4cbbb141bc8d6f3cfc528))
* **cli:** tell an unreachable service from a rejected credential ([a49415c](https://github.com/jacob-delgado/workflow/commit/a49415c5947a9d854b957765dbf5f9e5102aa8af))
* **config:** mask a password written into jira.base_url ([821309e](https://github.com/jacob-delgado/workflow/commit/821309e29036bae351301921a2bede079d336de0))
* **config:** mask the whole userinfo of a displayed URL ([dffe208](https://github.com/jacob-delgado/workflow/commit/dffe2088f7a615193b753b481b34e95bf9e942d8))
* **config:** set the file's mode when saving over an existing one ([86fab5a](https://github.com/jacob-delgado/workflow/commit/86fab5aa302dd4fe44ed638d8b8cac0c4d6f27d8))
* **convention:** do not read a standards token as an issue key ([1a91303](https://github.com/jacob-delgado/workflow/commit/1a913033cdb167f41d9bb0ef979d8af8982b9009))
* **convention:** match issue references by token and line, not substring ([03c3303](https://github.com/jacob-delgado/workflow/commit/03c33039942e80595c74c3f4cf07b42dce9d4c71))
* **convention:** report "@" for what it is, not as empty ([80cfb8a](https://github.com/jacob-delgado/workflow/commit/80cfb8a70ad51b1a3f329290582921dc241438d9))
* **editor:** name a file in full before opening it ([cf347c7](https://github.com/jacob-delgado/workflow/commit/cf347c708413495a50e3bbbeb87af623d60843e6))
* **forge:** drop an SSH remote's port from the API base ([7e04f32](https://github.com/jacob-delgado/workflow/commit/7e04f32e5fe74ef590285afd2da3ba1918731d2c))
* **forge:** fail a check run whose conclusion is not a pass ([1b3d80d](https://github.com/jacob-delgado/workflow/commit/1b3d80de608c0dfdab7539530a2e201e1765cd6e))
* **forge:** page GitHub's CI listings to the end ([6ab8234](https://github.com/jacob-delgado/workflow/commit/6ab82342b97aab54634ca50844b5a7605bfa2f49))
* **forge:** read each token source for the host it names ([74b203b](https://github.com/jacob-delgado/workflow/commit/74b203b177abaa29fdde5a1cb0c5e3e839e1a634))
* **gitrepo:** keep a branch's first commit past the commit cap ([6c817f5](https://github.com/jacob-delgado/workflow/commit/6c817f5d3683bbd7235fc468335d552f9dde2dd2))
* **gitrepo:** pass file names to git as literal pathspecs ([a062f0e](https://github.com/jacob-delgado/workflow/commit/a062f0ee16ea0263b00d68db4816cb5781f12183))
* **gitrepo:** read a branch's commits by a separator no subject holds ([a47d433](https://github.com/jacob-delgado/workflow/commit/a47d43399c68773f7d9fd0d3dbe09aca3e42ba01))
* **gobco:** measure every package, not just internal/config ([a87ce04](https://github.com/jacob-delgado/workflow/commit/a87ce04fe4167629e6fdaddc8715f2e9eba6205c))
* **gobco:** read the real gobco behind a mise shim ([cb2a40b](https://github.com/jacob-delgado/workflow/commit/cb2a40bc37c8a110862a61bd5f3e62a8ab022e1d))
* **hooks:** survive a bare env shebang and skip its options ([66df5ee](https://github.com/jacob-delgado/workflow/commit/66df5ee0a281aad6f79d012303976e9071308d43))
* **hooks:** undo a half-written lefthook configuration ([f8dee38](https://github.com/jacob-delgado/workflow/commit/f8dee38b9e323e30d5358ddb047bd4b47b67eabf))
* keep text from servers from driving the terminal ([e7b660b](https://github.com/jacob-delgado/workflow/commit/e7b660b7d9f55e920d954a12701dc11b484bc0e7))
* keep three reachable error strings in plain ASCII ([9e18c0b](https://github.com/jacob-delgado/workflow/commit/9e18c0b319be58bd0189c7b243e5a6525cbf08cb))
* **sanitize:** keep a name on one line and clean git's text on entry ([2b00163](https://github.com/jacob-delgado/workflow/commit/2b00163c2abb888b36f79f1bdccb5ad21bea7f55))
* **slack:** say what Slack refused, and how to fix it ([f0e1a7f](https://github.com/jacob-delgado/workflow/commit/f0e1a7f175430c74da7569edf47a5ffb98331c82))
* **tui:** clamp the detail scroll where the offset is written ([2e6e634](https://github.com/jacob-delgado/workflow/commit/2e6e634a4aacd8736e168ae7fcf4770ace03a83c))
* **tui:** count one staged file in the singular ([09e702e](https://github.com/jacob-delgado/workflow/commit/09e702e22bd1ad7733c48d1f867b6cdcfd195159))
* **tui:** draw a failed search like every other failure ([97437b4](https://github.com/jacob-delgado/workflow/commit/97437b4bced0ba7f41d0400b21dbf2ef26a82975))
* **tui:** flag an invalid commit scope as it is typed ([1fa30c5](https://github.com/jacob-delgado/workflow/commit/1fa30c50b0c9f0f47e2bbdaa38d8929afb30cd3b))
* **tui:** fold a multi-line notice onto its single row ([572188b](https://github.com/jacob-delgado/workflow/commit/572188bf983c6f5bfa22abcd1b5dd07ed8635409))
* **tui:** free the editing keys inside a composer's text fields ([361bac0](https://github.com/jacob-delgado/workflow/commit/361bac0b757bb9e8351ff04a7c5dabc8da9a0fc1))
* **tui:** have the Slack pane say what it needs when unset ([d0e5113](https://github.com/jacob-delgado/workflow/commit/d0e51139a204cb7f0c0c4bcba16386a9813888e5))
* **tui:** keep a Slack post with the pull request it was written for ([409a903](https://github.com/jacob-delgado/workflow/commit/409a9039214941d022555fc84899ab43324450d1))
* **tui:** keep the comment when a re-edit fails ([aa19707](https://github.com/jacob-delgado/workflow/commit/aa1970737bbf82d31048145490c007b21f55bc67))
* **tui:** keep the selected file on screen in the Commits pane ([03dff16](https://github.com/jacob-delgado/workflow/commit/03dff1680712f820295d0e4ff95809d72089ddab))
* **tui:** label esc and enter for what they do in the field form ([b2ce98f](https://github.com/jacob-delgado/workflow/commit/b2ce98fd179e183460a4c66bb1640a3da3f3b707))
* **tui:** let r retry a failed detail load ([9fab484](https://github.com/jacob-delgado/workflow/commit/9fab484f4e6b184ca959bd3d1ead3732465f92a0))
* **tui:** mask a password written into jira.base_url ([ff43daf](https://github.com/jacob-delgado/workflow/commit/ff43daf2614c8867daa93eeddb63968b93f27e47))
* **tui:** mention the push in the dry-run notice ([b850ecb](https://github.com/jacob-delgado/workflow/commit/b850ecb0955882f16ce7c4b29e5541841de97143))
* **tui:** name each kind of change in words ([5d088a3](https://github.com/jacob-delgado/workflow/commit/5d088a39553b6cefca47d894f485adf55ecd3cd8))
* **tui:** name the branch for the issue it is for ([cc9a872](https://github.com/jacob-delgado/workflow/commit/cc9a872b0f6803c0ce3e6e8483ee48aa4ebd8534))
* **tui:** say a status change in the words the person uses ([2f5e2f1](https://github.com/jacob-delgado/workflow/commit/2f5e2f1b2b3ecf6391f255b3076d09fd20d16741))
* **tui:** say when esc will discard what was typed ([da439f3](https://github.com/jacob-delgado/workflow/commit/da439f38a98af00ebe7b511ed12122132c5cb1ae))
* **tui:** say why the branch could not be read ([5499d7e](https://github.com/jacob-delgado/workflow/commit/5499d7e1ab75e680cb0836e5619d7d7bbc97455d))
* **tui:** style pane failures per row so no color leaks ([f809e22](https://github.com/jacob-delgado/workflow/commit/f809e22ba8c6f806c947a0f476d2d8fd32635f79))
* **wiring:** remember a forge connection only when it succeeds ([c91f4a4](https://github.com/jacob-delgado/workflow/commit/c91f4a46d7b7a573ca2923941d99eaa16a8d854c))


### Refactors

* **config:** show a URL one way, with its password masked ([bfb5e61](https://github.com/jacob-delgado/workflow/commit/bfb5e61cd0a7d46126aa133034726fe21a07dff9))
* **hooks:** delete the unused config reader and File.Executable ([9f3203b](https://github.com/jacob-delgado/workflow/commit/9f3203bdfa463a66e22af901ab267720f46c1634))


### Documentation

* add feature, UX and technical-debt backlogs ([e92149c](https://github.com/jacob-delgado/workflow/commit/e92149cfd8cce162acfb75692c96d1f606e88ffd))
* add status and supply-chain badges to the README ([51c4367](https://github.com/jacob-delgado/workflow/commit/51c4367d34e769fe32d854744a20e1b704ae22dc))
* correct guidance the enabled linters reject ([e267779](https://github.com/jacob-delgado/workflow/commit/e267779837399c0f1015b957313c3f9e992b82d3))
* correct the pre-1.0 version bump rules ([c4d88bf](https://github.com/jacob-delgado/workflow/commit/c4d88bf464ed8ae40d5b3ab99268abfd8da58221))
* document the panes, keys and the loop from issue to Slack ([e77e8ca](https://github.com/jacob-delgado/workflow/commit/e77e8ca22515cf261d54e77f307f98b565b204be))
* publish a documentation site and document installing ([57c8f6c](https://github.com/jacob-delgado/workflow/commit/57c8f6cc9f2cc29a262c0a9046d4d02d11f23338))
* retire the completed UX-49 backlog entry ([60583f7](https://github.com/jacob-delgado/workflow/commit/60583f7ea38794245114ed13d67d56eef2e7db45))


### Build & Packaging

* add a cloc task, and say that task run takes arguments ([cd78d05](https://github.com/jacob-delgado/workflow/commit/cd78d05a848c200b3f9afaf839376e0d7bade8e1))
* bump golang.org/x/text past CVE-2026-56852 ([0b25bc4](https://github.com/jacob-delgado/workflow/commit/0b25bc418cdb177ee8c6e90abe33f152e3ccee9e))
* **commit-msg:** enforce subject rules and ignore git boilerplate ([8dbd5ec](https://github.com/jacob-delgado/workflow/commit/8dbd5ec3d09f66c83e4f466d3bbfb0bf3ca7b7aa))
* **devcontainer:** install a pinned, checksum-verified mise ([03d97cb](https://github.com/jacob-delgado/workflow/commit/03d97cbc739fe6d607e2fddd399484dec2879c34))
* leave the generated changelog out of the Markdown lint ([edf2aca](https://github.com/jacob-delgado/workflow/commit/edf2aca32db6330e9b2f700fce0416621c9b5f80))
* lint Markdown, TOML, file length, and workflow security ([c075b30](https://github.com/jacob-delgado/workflow/commit/c075b303391e9c53ebd5bf015dfc9a6069b1a9e0))
* pin the build container's base image by digest ([7f7de96](https://github.com/jacob-delgado/workflow/commit/7f7de96a2ce2f0ce3053f602d3a987beb6f5359a))
* **typos:** enforce American English with an en-us locale ([5d8b470](https://github.com/jacob-delgado/workflow/commit/5d8b470640ebee16287d272429667ed14fb572be))


### CI

* add a ci-gate job as the single required check ([31f75c4](https://github.com/jacob-delgado/workflow/commit/31f75c49e39a80f42ec4cccdb8e0d33a40958256))
* **release:** check a release commit before tagging it ([3493454](https://github.com/jacob-delgado/workflow/commit/34934540384f9842b5c7b09966e9e14ffff70c49))


### Tests

* **config:** fuzz the redactor and the configuration loader ([4a428a6](https://github.com/jacob-delgado/workflow/commit/4a428a6d981cd9fa88e26a2301eeccc6b265cc83))
* **doctor:** cover the conditions the whole-module gate exposed ([0f5115a](https://github.com/jacob-delgado/workflow/commit/0f5115a45567f9a06fb02ea3ecf9b5f2b882b691))
* **gitrepo:** split the git-command tests off branch_test ([52cf893](https://github.com/jacob-delgado/workflow/commit/52cf893d4a17150244569b404b40c01b94cbb4c1))
* mark Arrange, Act and Assert in every test and check them ([817d323](https://github.com/jacob-delgado/workflow/commit/817d323fcd06597168d6a48476f3342dd5735909))
* **tui:** move the over-scroll test out to fit the length gate ([a13fc11](https://github.com/jacob-delgado/workflow/commit/a13fc1119392fd809d541dc82273f381a54a9f17))
