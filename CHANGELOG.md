# Changelog

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
