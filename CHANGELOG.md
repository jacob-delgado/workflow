# Changelog

## [0.13.1](https://github.com/jacob-delgado/workflow/compare/v0.13.0...v0.13.1) (2026-10-24)


### Refactors

* **web:** build IssueLink's test branches in one helper ([48b2d33](https://github.com/jacob-delgado/workflow/commit/48b2d335a54337a0da46ba317fe3f93a676b7ef9))


### Build & Packaging

* Bump undici from 8.11.2 to 8.12.2 in /web ([9e48e38](https://github.com/jacob-delgado/workflow/commit/9e48e382cf849aab2b88e9a71230eb10e1bb0c41))
* **deps:** move kin-openapi to 0.151.0 ([1720061](https://github.com/jacob-delgado/workflow/commit/172006154aae057da327c6c74b17c4fe09b906a8))
* **deps:** move the web app's ten dependencies Dependabot grouped ([609b13f](https://github.com/jacob-delgado/workflow/commit/609b13fc01c16913050213d9e953d7d0690a2741))
* say --require-hashes on the image's pip install line ([6b1cdcb](https://github.com/jacob-delgado/workflow/commit/6b1cdcbc6ef53d1f8750d86134f9a141f4a1eb5c))
* take katex 0.18.11 over micromark-extension-math's 0.16 range ([b304e39](https://github.com/jacob-delgado/workflow/commit/b304e3991156218d4440eb9ddf5c3ff7ee7e5f03))


### CI

* Bump the actions group with 5 updates ([3624933](https://github.com/jacob-delgado/workflow/commit/3624933baac152b3a2c317cc3268841e09308822))
* **deps:** hold GitHub's own actions to Dependabot's week as well ([297b442](https://github.com/jacob-delgado/workflow/commit/297b442b473e9d5b9ee872d29a4502fb39d5eaa0))
* move mise-action to 5.2.0 and codeql-action to 4.39.1 ([035cfe8](https://github.com/jacob-delgado/workflow/commit/035cfe8da48ca8c71da7f06f032255976bf032f2))
* run the end-to-end jobs in Playwright's image ([d2f2cea](https://github.com/jacob-delgado/workflow/commit/d2f2ceaecaa1853d0a192ec38a8553e86d5d7400))


### Tests

* **web:** wait for focus to land in IssueLink's handback tests ([a26014e](https://github.com/jacob-delgado/workflow/commit/a26014e3f0447c515db0ce3c5e2d20bca4fc5620))

## [0.13.0](https://github.com/jacob-delgado/workflow/compare/v0.12.0...v0.13.0) (2026-10-23)


### ⚠ BREAKING CHANGES

* **webserver:** a --web API request whose JSON body names the same key twice in an object is refused with 400 bad_request, "the request did not match the API contract", and nothing it asks for is done; before, the last value counted. A client sends each key once.
* **config:** a .workflow.json whose forge.kind is not exactly "github" or "gitlab" (in capitals, or with spaces around it) no longer loads; workflow doctor names the setting. Write it in lowercase.

### Bug Fixes

* **config:** refuse a forge.kind not spelled in lowercase ([024fc07](https://github.com/jacob-delgado/workflow/commit/024fc0789c9a23e5a6719bd3804e23aec2f5f044))
* **places:** keep the status chips in workflow order ([9fef17b](https://github.com/jacob-delgado/workflow/commit/9fef17baf43114696f04b7886ca7adc611cbcd8f))
* **places:** rank a status named in two categories by the earlier one ([42c6775](https://github.com/jacob-delgado/workflow/commit/42c6775ce23da624631fe1385ba39ebac680d637))
* **progress:** count changes toward Commits only on a feature branch ([0ef8cd6](https://github.com/jacob-delgado/workflow/commit/0ef8cd601ecee383afd445417e195d6e25987a85))
* **tui:** keep the Slack stage in flight while an announcement sends ([d21e16c](https://github.com/jacob-delgado/workflow/commit/d21e16c1b7d7474b4b73393b4943485b171274e4))
* **tui:** label shift+tab on the pull request title "back" ([aa7dcbc](https://github.com/jacob-delgado/workflow/commit/aa7dcbc299d1b826461b4f10768308aa450b4d68))
* **tui:** name the announce action for what it does while one waits ([06da534](https://github.com/jacob-delgado/workflow/commit/06da534eead63b778788854faee44fef8f38c25d))
* **tui:** take a notice's row from the focused pane alone ([6edccf6](https://github.com/jacob-delgado/workflow/commit/6edccf665f50a1e616412063a88ea72a3ac8f668))
* **web:** bring a focused control wholly into view in Firefox ([601b4bd](https://github.com/jacob-delgado/workflow/commit/601b4bd225f3c0856a9268ad20b0d0226c1446fc))
* **web:** grow the pull request Description to its whole text ([2463ba1](https://github.com/jacob-delgado/workflow/commit/2463ba1d832699537257e86ccc239c8356484e59))
* **web:** hand a write on beside the first frame that counts it ([054c4bd](https://github.com/jacob-delgado/workflow/commit/054c4bd7ff891030544af9b7ae072b91f8891fc4))
* **web:** keep the commit button on one line beside its outcome ([b7cbac8](https://github.com/jacob-delgado/workflow/commit/b7cbac878ec88de0c94d0953cc278b944b3eefbc))
* **web:** keep the commit type used, and show the header it counts ([a981782](https://github.com/jacob-delgado/workflow/commit/a9817821b2fecdff6977f33c7b79001cc9de3b7b))
* **web:** keep Unstage all beside Stage all once it has spoken ([6dde624](https://github.com/jacob-delgado/workflow/commit/6dde62456125ee38fdb59ef01a6f32d61684b5d7))
* **web:** name the announce action for what it does while one waits ([46efc6c](https://github.com/jacob-delgado/workflow/commit/46efc6c43f82a7450cea0a3ea3620d57a47e4ad1))
* **web:** refuse a write the contract refused without a frame ([963a233](https://github.com/jacob-delgado/workflow/commit/963a233e7a0bc53baf0881f7cbf5cf8f1edcc8cc))
* **web:** say a finish finished above the review it restarts ([6a93af9](https://github.com/jacob-delgado/workflow/commit/6a93af9004ef1d2a317578d1fe68cf7d2075998b))
* **web:** say a merge announced though the ready one was already said ([7c05bb3](https://github.com/jacob-delgado/workflow/commit/7c05bb3f2f3115d57734a7a82f66aa8f0723b20d))
* **web:** say a write is done only once the page shows it ([f7db3d4](https://github.com/jacob-delgado/workflow/commit/f7db3d427c79dce5e2e8100de72d478708a5aace))
* **web:** say an announcement already made by its moment, once ([6f88b70](https://github.com/jacob-delgado/workflow/commit/6f88b70995207b6c19f68af11783bda65347d02b))
* **web:** say an announcement made here once while its answer lands ([6bf0633](https://github.com/jacob-delgado/workflow/commit/6bf0633ef1117c08f11fe039f1db917924e905e5))
* **web:** say plainly how to drop an armed announcement ([639a9c3](https://github.com/jacob-delgado/workflow/commit/639a9c36fe6d58f464a7189fbe97e74f0f9ac8f6))
* **webserver:** keep a seeded pull request from an older head's read ([e05a59e](https://github.com/jacob-delgado/workflow/commit/e05a59e0a9e961e8773e519bf97b46d7614fb6c8))
* **webserver:** let a frame take the forge read it waited on ([4290c2b](https://github.com/jacob-delgado/workflow/commit/4290c2b3ed56c430b1f09f28ca30f371ea86d56b))
* **webserver:** let waiters take a read a seed came after a drop during ([c1f58fb](https://github.com/jacob-delgado/workflow/commit/c1f58fbd407516eed09d30a29ba6eec193b4aaa3))
* **webserver:** number every write, and count them in each frame ([3b3451a](https://github.com/jacob-delgado/workflow/commit/3b3451ada93846304ccb2e35f2a0e0bf9a78c1be))
* **webserver:** push a frame as a write lands, never one read across it ([14feb0d](https://github.com/jacob-delgado/workflow/commit/14feb0da34e287f48b58d1a0deb801fada1474d3))
* **webserver:** refuse a request body that repeats a key ([9e78e96](https://github.com/jacob-delgado/workflow/commit/9e78e96db7bbf468a340720bed0889dde06c4c4e))
* **webserver:** send no frame from a server switched away from ([e4a3152](https://github.com/jacob-delgado/workflow/commit/e4a31527412758a6e0865a63baeb02b1c8c02472))
* **webserver:** serve no answer a drop came during the read of ([6d9d4fe](https://github.com/jacob-delgado/workflow/commit/6d9d4fe7665e6b12fdba89a9978b1ddd9532829f))


### Refactors

* **ci:** move ci-gate's verdict into scripts/ci-gate.sh ([e0d29e6](https://github.com/jacob-delgado/workflow/commit/e0d29e650a8a93b609d5495dadd78b609a0d85e4))
* **demo:** name the recording's steps the terminal tests repeat ([1993b1a](https://github.com/jacob-delgado/workflow/commit/1993b1a7093e42fe8078d9deab3b8a2d1a588a9d))
* **demo:** read the demo's command line in internal/demo ([d3d9132](https://github.com/jacob-delgado/workflow/commit/d3d9132f21f460b02028e4ecafe43858af3e897e))
* **demo:** read the demo's events, tasks and pulls without an error ([4359738](https://github.com/jacob-delgado/workflow/commit/43597387de0a764d33666dd02a905ae36d1667f8))
* **scripts:** share gate-as-dev's test fixtures from lib ([4456cd1](https://github.com/jacob-delgado/workflow/commit/4456cd12fa3b205fcbb71505c1a07f41d5fc64f7))
* **web:** drop the mock's released-version override ([e3aba5e](https://github.com/jacob-delgado/workflow/commit/e3aba5eca0e835d54a795e8e9df483d82fbbcd23))
* **web:** launch the server run's Chromium from the shared device ([87a2388](https://github.com/jacob-delgado/workflow/commit/87a238849ae4c5d5de0c1d80af37c1abb6e4a9fc))
* **web:** lift the film's glide and click out of filmedPace ([f0de118](https://github.com/jacob-delgado/workflow/commit/f0de118161864e6ed89ac83ba016bb16c71c2000))
* **web:** move the film's page scrolls into scroll.ts ([d833c16](https://github.com/jacob-delgado/workflow/commit/d833c1663b7fb4940aa9c4ac3ac47859335deba1))


### Documentation

* add a page of questions and troubleshooting ([5607650](https://github.com/jacob-delgado/workflow/commit/5607650b8375ba635025650b78f3f4637e48fefe))
* **api:** say a write's answer wakes the event stream ([f7a341a](https://github.com/jacob-delgado/workflow/commit/f7a341a65c1c8ae92931973cdd7a04acd249882e))
* **config:** say what a Slack app configuration token is for ([2a533db](https://github.com/jacob-delgado/workflow/commit/2a533db01a7fb3bd5ec47ea0314ded26f2f71f3a))
* **demo:** say where the demo's issues differ from the web's mock ([4c55223](https://github.com/jacob-delgado/workflow/commit/4c55223167614f1434b742cfecbeb05ab0857ba6))
* describe the gate for a docs-only change ([3c8b720](https://github.com/jacob-delgado/workflow/commit/3c8b72071b0699f955e38c40a49d39bd323f1ea5))
* give each pull request one review round ([02164b5](https://github.com/jacob-delgado/workflow/commit/02164b517295fb05c8bd5c6ab9966cefbd13d842))
* lead with the web walkthrough beside the terminal ([0b45d80](https://github.com/jacob-delgado/workflow/commit/0b45d808bf00950285f1637abe1aa6b6e206e55f))
* link Taskwarrior's site where each page first names it ([6fd9cd8](https://github.com/jacob-delgado/workflow/commit/6fd9cd81d5fdb5b3f791682c7f5f419863d6cde3))
* name Announce now instead where the armed state's controls are ([0f2b051](https://github.com/jacob-delgado/workflow/commit/0f2b0516ec4bcae76c5a1e69ff3771f44611cbe3))
* record the write gate's unbounded wait as kept in TRADE-4 ([017057b](https://github.com/jacob-delgado/workflow/commit/017057bef0b23534e96082adc809beb0d2151bd2))
* regenerate the images and tell one story under them ([e82b8c3](https://github.com/jacob-delgado/workflow/commit/e82b8c3adfc76d54574bbbd2d7b02f12d010bd85))
* show the terminal and web interfaces ([89a06d9](https://github.com/jacob-delgado/workflow/commit/89a06d9ff6d0b00ee40d59f0c7a97b0c94462191))
* **skill:** keep git config out of a linked worktree ([441eee8](https://github.com/jacob-delgado/workflow/commit/441eee88e615cea53bc30bfbad59c2677e6b598c))
* **skill:** say that stopping a run can break the shared tools ([9c4a678](https://github.com/jacob-delgado/workflow/commit/9c4a678c0712f152027c170efca822c90d014a19))
* **web:** regenerate the docs' media over the demo world ([2a1cd4d](https://github.com/jacob-delgado/workflow/commit/2a1cd4d71dc218c4778f22a6d307b844d408c9a8))


### Build & Packaging

* bump markdownlint-cli2 to 0.24.0 ([de06255](https://github.com/jacob-delgado/workflow/commit/de062557e5d5b61ed3e8c05cc1f24479f46e100e))
* move Go to 1.27.3 for crypto/x509's fix ([226f016](https://github.com/jacob-delgado/workflow/commit/226f016007cd83a4366a0cf7313309c6f9e04220))


### CI

* skip the code jobs for a docs-only pull request ([8dc3351](https://github.com/jacob-delgado/workflow/commit/8dc33517cd12807f2c8f4f683c7c7f80757eddeb))
* **web:** run every spec on Firefox and WebKit, sharded ([9e58c49](https://github.com/jacob-delgado/workflow/commit/9e58c49bd1e72a25235f616044408eeec16bd2a3))


### Tests

* **demo:** hold the month's moves off the worked issue ([734cd46](https://github.com/jacob-delgado/workflow/commit/734cd467beae749cb9f1651f2916ab26724b44c8))
* **demo:** read the opened pull's CI once the forge is asked again ([2df2181](https://github.com/jacob-delgado/workflow/commit/2df2181b0d3e636e77222ffb52297f0a5396c788))
* **e2e:** hold each write's notice to the page the binary serves ([61d0481](https://github.com/jacob-delgado/workflow/commit/61d048186eb8598c92637cdc4f5bf35dc87890f0))
* **e2e:** number the hermetic page's writes, and count them in frames ([1ebe060](https://github.com/jacob-delgado/workflow/commit/1ebe060d236a3ecadc136a250825ed6905b7d96a))
* start each docs contract test's name with TestDocs ([0f7029e](https://github.com/jacob-delgado/workflow/commit/0f7029efe66d35b36e6638c68696ba726439302c))
* **tui:** hold the rail contiguous while a notice shows ([701276b](https://github.com/jacob-delgado/workflow/commit/701276b418ada6d714ccc1a0af2089d094545b18))
* **webserver:** hold the forge cache to writes made during its read ([694aaae](https://github.com/jacob-delgado/workflow/commit/694aaae36d8e6241fef0c396fc4d18b655909371))

## [0.12.0](https://github.com/jacob-delgado/workflow/compare/v0.11.0...v0.12.0) (2026-10-18)


### ⚠ BREAKING CHANGES

* **cli:** in ASCII (ui.ascii, or status --plain) the live line's mark turns through |, /, - and \ instead of holding a still `*`; under ui.motion: reduce it still holds on `*`. A script reading a terminal's stderr for the mark matches one of those five instead.
* **api:** GET /api/directories no longer answers truncated. A listing cut short carries truncated_to, the number of directories it was cut to (1000 today), and a whole listing leaves the field out. An API client that read truncated reads whether truncated_to is present instead.
* **cli:** ui.ascii, and --plain on status, now draw the live line in ASCII (a * that does not turn, and a bar of = and -), and ui.motion: reduce holds its mark still; before, the line was Unicode with a turning mark whatever they said. For the Unicode line, leave ui.ascii and --plain off; for a turning mark, leave ui.motion empty. The line is kept only on a terminal, so stderr read from a pipe is unchanged. The refusal of status --plain --json still exits 2, but its message now reads "--plain draws the status line's marks and --json prints no status line: use one or the other; ui.ascii draws a JSON run's live line in ASCII"; a script that matched the old words tells the refusal by its exit status instead.

### Features

* **api:** report the limit a directory listing was cut short at ([5be0ba9](https://github.com/jacob-delgado/workflow/commit/5be0ba936b216013714cc100efe771121121ccb2))
* **tui:** mark each refused setting on its own Settings row ([25eb502](https://github.com/jacob-delgado/workflow/commit/25eb502d5f2934749d118174db63fa151acb1d63))


### Bug Fixes

* **cli:** draw the live line as ui.ascii, --plain and ui.motion ask ([679ee89](https://github.com/jacob-delgado/workflow/commit/679ee89ad3c5bf78c47dc0f56ddc94a064892785))
* **cli:** say status's --plain and --json refusal in one sentence ([ed42e51](https://github.com/jacob-delgado/workflow/commit/ed42e51bb7435c7cf6300cdf32b9a00e98ac35d9))
* **cli:** turn the in-flight mark in ASCII through | / - \ ([0006ad0](https://github.com/jacob-delgado/workflow/commit/0006ad08a8c45741103c9479f3e2cb0009ba469f))
* **convention:** offer no scope for a directory named by spaces ([3a3cf32](https://github.com/jacob-delgado/workflow/commit/3a3cf32b4f8c4e09d6ba639d23459eaa58ff65cc))
* **e2e:** bring the Tab walk back in when focus leaves the page unseen ([7d4085f](https://github.com/jacob-delgado/workflow/commit/7d4085ffdfd691b9b155d1907791753d151d2df1))
* **e2e:** read each Tab stop after the frame that reveals it ([9915cb3](https://github.com/jacob-delgado/workflow/commit/9915cb3560516f4a04b2a358ec174f7bcaf5ce55))
* **proc:** deliver nothing once a run has abandoned its output ([b30d47e](https://github.com/jacob-delgado/workflow/commit/b30d47e22ade0ca50c5acc0f901f6c3112202479))
* **proc:** end a failure with no standard error at its cause ([34e29be](https://github.com/jacob-delgado/workflow/commit/34e29be37963556588fc0fb183cce15c0a40d884))
* **proc:** end a run with a line too long once its program exits ([4ff8643](https://github.com/jacob-delgado/workflow/commit/4ff86436a61347709bab52ad0bf7dde944fcdf18))
* **quietinput:** complete only what is typed in the field ([30bd2f7](https://github.com/jacob-delgado/workflow/commit/30bd2f769c40f014aee41b3958c01c1674a13c12))
* **quietinput:** keep a suggestion typed whole in another case ([38477bf](https://github.com/jacob-delgado/workflow/commit/38477bf5106ef9049a22d434ad752433ba16188a))
* **quietinput:** keep a typed value that is itself a suggestion ([c8ef12d](https://github.com/jacob-delgado/workflow/commit/c8ef12d8445646808df471555d6a1b61c119b863))
* **quietinput:** match again once a completion taken was refused ([41fe482](https://github.com/jacob-delgado/workflow/commit/41fe482e1122244724e5f72e12c8bfd492a0ab62))
* **scripts:** refuse a ratcheted budget at or under the default ([2c2b89b](https://github.com/jacob-delgado/workflow/commit/2c2b89b0998b05641a82ca43561a72dd6ec21549))
* **tasks:** say a directory switch applies a Taskwarrior setting too ([25d7813](https://github.com/jacob-delgado/workflow/commit/25d7813c84aace813804f90f9f5ae6054ea81a54))
* **tui:** join a listing asked again by name with the whole one ([b267553](https://github.com/jacob-delgado/workflow/commit/b26755396f87030624e126a48cc9716d1371afe7))
* **tui:** keep Settings' refusals in sight while a form is open ([81544a0](https://github.com/jacob-delgado/workflow/commit/81544a008a93ef6d38ad800238b00236b1d4181d))
* **web:** bring a focused issue row wholly into the list for Firefox ([bd94d97](https://github.com/jacob-delgado/workflow/commit/bd94d97f25819d9eb42ef71a8bf9f47f406a0391))
* **web:** pad the Issues detail's scroll as the Tasks one's ([276cece](https://github.com/jacob-delgado/workflow/commit/276ceceaef30035956ce59a8126fad2b713bf163))
* **web:** pad the Tasks detail's scroll as the content's ([7bcf928](https://github.com/jacob-delgado/workflow/commit/7bcf92881bc62be03bd52a112aa4f0531c00edc7))
* **web:** say a Taskwarrior setting also applies on a directory switch ([e69e7a5](https://github.com/jacob-delgado/workflow/commit/e69e7a563a2c4e3483b87ad3903ec335c0f70b6c))
* **web:** say the dialog is reading until the whole listing answers ([7c487e6](https://github.com/jacob-delgado/workflow/commit/7c487e608e5dba97b36497a6ce533c4ea028b2bc))
* **web:** say when a Jira setting applies ([0e1a071](https://github.com/jacob-delgado/workflow/commit/0e1a071a1f584028f4245f6c8a42a1a938602028))
* **webserver:** allow the style WebKit gives a select's slot, by hash ([6877ad6](https://github.com/jacob-delgado/workflow/commit/6877ad6c4cafdfd9df37f8e5342ea08390d0602c))
* **web:** word an empty list field as the server refuses it ([d11a793](https://github.com/jacob-delgado/workflow/commit/d11a793f6b24cb48c644e22e25347b0a3cf803dc))
* **workdirs:** match directory names alike on both surfaces ([b1d532c](https://github.com/jacob-delgado/workflow/commit/b1d532c3fb185dd02563427fbab7a3ab0717d26e))


### Refactors

* **activity:** hold the calendar's weeks to the shared dates ([15e386a](https://github.com/jacob-delgado/workflow/commit/15e386ae9ac5b46bea545c7285746aaa9fca63c4))
* **cli:** declare --port beside the root's other flags ([0294ee8](https://github.com/jacob-delgado/workflow/commit/0294ee875dcbf8bfe7974d2becbe948f45c8aa55))
* **cli:** drop the errors no doctor check meets from its outcome ([014df3a](https://github.com/jacob-delgado/workflow/commit/014df3a27bb1484a06cfdf73acb58a7f35bd3201))
* **cli:** drop the reviews constant the output tests name once ([388e6fe](https://github.com/jacob-delgado/workflow/commit/388e6fe605d94b9430efd106996b65a22def6a2b))
* **cli:** give the exit statuses a file of their own ([26f9e18](https://github.com/jacob-delgado/workflow/commit/26f9e180d56ec7d6721aac2ae861b816f3d17dda))
* **cli:** keep the shared connection helpers in connect.go ([2b33745](https://github.com/jacob-delgado/workflow/commit/2b33745db6de06a2e764711b7351041d697c4253))
* **cli:** lift what every command shares into internal/cli/cmdkit ([76d7c77](https://github.com/jacob-delgado/workflow/commit/76d7c777895271276427af3d6df66f4f6b6f3d0f))
* **cli:** move doctor's checks to internal/cli/doctor ([79eaa75](https://github.com/jacob-delgado/workflow/commit/79eaa75280315756f3ae441c989ad7a7973af1b6))
* **cli:** move the issue commands to internal/cli/issues ([e48d772](https://github.com/jacob-delgado/workflow/commit/e48d772b764b1596be8253fc5707b7f9c9408623))
* **cli:** move the messaging refusal beside postingIn ([5584046](https://github.com/jacob-delgado/workflow/commit/558404681b6bdc0a7c69c275969c60f93ae6dfa3))
* **cli:** move the pr and announce flow to internal/cli/pr ([5dac888](https://github.com/jacob-delgado/workflow/commit/5dac888de3087b62c6975155595bcaf96b5db0ba))
* **cli:** move workflow status to internal/cli/status ([27de625](https://github.com/jacob-delgado/workflow/commit/27de62548b37831d22fc66c3a9d9270e1b9d2cd2))
* **cli:** name the clients' unreachable errors on their own ([db7906c](https://github.com/jacob-delgado/workflow/commit/db7906c984062501595f3660ed675312701088c0))
* **cli:** read the configuration's home in environment.go ([1c93637](https://github.com/jacob-delgado/workflow/commit/1c93637b560e52db78e6adb74266ef56c10ca7d7))
* **cli:** run summary's dated commit through proc ([a0f296a](https://github.com/jacob-delgado/workflow/commit/a0f296acea6b9dd0ff90ea971175c85adf738561))
* **convention:** drop the scope guards ValidateScope already keeps ([32bf3df](https://github.com/jacob-delgado/workflow/commit/32bf3dfd20abc2a8519e8539c0ec5f59fdc4107a))
* **convention:** move the scope helpers beside convention.Scopes ([dfa05a1](https://github.com/jacob-delgado/workflow/commit/dfa05a1ac4fd147600c90a3cdb7ecdf58e058214))
* **e2e:** build the Tab walk's report as its lap closes ([a705765](https://github.com/jacob-delgado/workflow/commit/a7057650d87f4d4d5131825c90f5cc443894f89a))
* **lifetime:** move the outlasting plumbing out of tui/deps.go ([02fc4d2](https://github.com/jacob-delgado/workflow/commit/02fc4d2801b7905115db7c95809637a5e9bd968c))
* **proc:** close a run's lines without checking they are open ([c5dae1c](https://github.com/jacob-delgado/workflow/commit/c5dae1cd35eb8e51dfebb808be2786867d2a6348))
* **proc:** drop the abandonment a waiting send can never meet ([86ee158](https://github.com/jacob-delgado/workflow/commit/86ee1589cc268ddcf7e09e5409c1e6b7e457882f))
* **proc:** give each left-behind helper mode a function of its own ([ef7a183](https://github.com/jacob-delgado/workflow/commit/ef7a183cb37c8c4e5d633f257ccb373a040dc90d))
* **proc:** leave closing the pipe past the grace to follow ([ffcf69a](https://github.com/jacob-delgado/workflow/commit/ffcf69ad23502b7bf36bef9444ce1d5bddefb67b))
* **proc:** move the left-behind helper modes to their own file ([d957ec8](https://github.com/jacob-delgado/workflow/commit/d957ec803c016abcb601e7241c60e54b15fe959d))
* **proc:** pause the grace by dropping its timer alone ([47c384d](https://github.com/jacob-delgado/workflow/commit/47c384d78838e699f10583dc781345837c56f1f9))
* **proc:** read a run's exit the same way whichever end comes first ([e2523c5](https://github.com/jacob-delgado/workflow/commit/e2523c5b5b5d747f78a070f41bf5dee9da86c109))
* **proc:** start the grace's count without asking if it runs ([ba63fe7](https://github.com/jacob-delgado/workflow/commit/ba63fe76a68057f73bed89cf76541ab493a76d71))
* **progress:** give the in-flight mark's turn one source ([d57f29f](https://github.com/jacob-delgado/workflow/commit/d57f29f5b022299b70d15f5a8c7d588ffbbab580))
* **quietinput:** read whether suggestions are on from the input ([fbeb111](https://github.com/jacob-delgado/workflow/commit/fbeb1115f5d6b44978b4ee8aaaccd9cb6bd22e92))
* **scripts:** read the layout block through scripts/lib/layout.sh ([ba62786](https://github.com/jacob-delgado/workflow/commit/ba6278629255ee4d899dc4e12ad4a216596689e0))
* **tui:** draw a refusal's failure lines in a helper of their own ([70aedb0](https://github.com/jacob-delgado/workflow/commit/70aedb0f7d46f8b92c72c252954334375ac353f6))
* **tui:** drop the picker's check for a listing not yet asked for ([4c22d3c](https://github.com/jacob-delgado/workflow/commit/4c22d3c0c20d25b318b6a9924ad7824a9edd67d7))
* **tui:** give the quiet input a package of its own ([fc90104](https://github.com/jacob-delgado/workflow/commit/fc901049bda0d3e92f1667c20fa150408d2fbd07))
* **tui:** keep the turned-to mark on the kit, not the glyphs ([2436acb](https://github.com/jacob-delgado/workflow/commit/2436acb514b83d8452780310092fd69be5e1e31a))
* **tui:** name the edited configuration's JSON for what it is ([ff6cb4d](https://github.com/jacob-delgado/workflow/commit/ff6cb4d2dd5fcfe73e1a0a35a6131637ebf86363))
* **web:** drop the Issues detail's inner key ([98c3736](https://github.com/jacob-delgado/workflow/commit/98c373674f3fd21869740dde420f6f9312d0166c))
* **web:** drop the Tasks summary's second check for a list ([371362f](https://github.com/jacob-delgado/workflow/commit/371362f3dd5fc9ace6681e4ba77a7594ab7f09df))
* **web:** hear the content policy's refusals in one helper ([9e28726](https://github.com/jacob-delgado/workflow/commit/9e28726224f91a76448d82771ff83fabc3705f9b))
* **web:** move the directory dialog's way up into its own file ([5b4d57d](https://github.com/jacob-delgado/workflow/commit/5b4d57d3fe2d13eecd0cf254aee85162945d087f))
* **web:** name the Issues pane's reveal so focus can use it too ([d9a51e7](https://github.com/jacob-delgado/workflow/commit/d9a51e7c3d6abfffc7223389b38a966d0f8b2a93))
* **webserver:** dereference an optional value one way ([0c5d238](https://github.com/jacob-delgado/workflow/commit/0c5d238cfb088aa1ed3d139790dd90e276190e3e))
* **webserver:** gather the shared test helpers ([6109319](https://github.com/jacob-delgado/workflow/commit/610931926f745db5ae740226e1f8e265e16019ed))
* **webserver:** give the shared test helpers a package of their own ([8d36e0d](https://github.com/jacob-delgado/workflow/commit/8d36e0d0b987729354189c4b8f9d8f8f90b740e1))
* **webserver:** give webtest's package comment its own file ([2c2b31e](https://github.com/jacob-delgado/workflow/commit/2c2b31ef91369e5b8dd0aa80046a6fd4058718a3))
* **webserver:** keep fault with the classes it tells apart ([9ea4690](https://github.com/jacob-delgado/workflow/commit/9ea469078333162c71d0a8445bb6a7ad822eeb0e))
* **webserver:** keep the server's accessors beside it ([4c6996f](https://github.com/jacob-delgado/workflow/commit/4c6996faf9a5c0190edb3ac3477eae6cc67044a3))
* **webserver:** keep whom an announcement tags in announcetags.go ([a28ee61](https://github.com/jacob-delgado/workflow/commit/a28ee61f9e179cd1cc1367f41891409db5cdf8b1))
* **webserver:** move the git runs into internal/webserver/runs ([34e240a](https://github.com/jacob-delgado/workflow/commit/34e240a0783e92afcb8e9ea2098bae734e7072f1))
* **webserver:** move the problem details into their own package ([d4de366](https://github.com/jacob-delgado/workflow/commit/d4de36694e0298a3d3461ff27cd277fb80b3a5e3))
* **webserver:** move the problem media type into webtest ([7d13724](https://github.com/jacob-delgado/workflow/commit/7d13724396fc3b6af5f6754ed14c1e01137ea2de))
* **webserver:** move the request boundary to webserver/edge ([688c962](https://github.com/jacob-delgado/workflow/commit/688c962d88b71ed6fc7579e1ae488ead8385a366))
* **webserver:** move the Taskwarrior handlers to webserver/tasks ([edc25ce](https://github.com/jacob-delgado/workflow/commit/edc25cec6beb1f9189678f137ec62a7841340da6))
* **webserver:** unexport webtest's canceling flusher ([f17aeaf](https://github.com/jacob-delgado/workflow/commit/f17aeafbce67706a5398f318e8c7b372bd396ba2))
* **wiring:** name isolateGit for the one thing it still does ([fbc9a18](https://github.com/jacob-delgado/workflow/commit/fbc9a185c33f9c9deff05ec3aaca101184b5df42))


### Documentation

* **arch:** give each store file its own reason for no migrations ([271ea43](https://github.com/jacob-delgado/workflow/commit/271ea43b8115899c1b368e7012465fcef97315f0))
* **claude:** correct fourteen stale facts ([a81150a](https://github.com/jacob-delgado/workflow/commit/a81150ad880ee9180fa8641e0dd53d632a74cc78))
* **claude:** cut the root to every-session rules and an area index ([4032b51](https://github.com/jacob-delgado/workflow/commit/4032b51ede2a3dc9c0f4ad66c00fee80cb6ac305))
* **claude:** let cohesive packages shed code with its own reason ([19a05cf](https://github.com/jacob-delgado/workflow/commit/19a05cf00ff1e9c644de1b67f81b9300caddc15b))
* **claude:** say when a coverage floor may drop ([61c306f](https://github.com/jacob-delgado/workflow/commit/61c306f3116f0b919b73d427bc3e9aa2ed11d916))
* **claude:** write down what cloud sessions and lanes must know ([a12740a](https://github.com/jacob-delgado/workflow/commit/a12740a6717d2e60ce44428545da7246120d4c93))
* **cli:** say how the gates treat clitest ([d5d1503](https://github.com/jacob-delgado/workflow/commit/d5d1503da3bc2ee25aea2561a285ce0e0ae80ccf))
* **cli:** say the live line follows ui.ascii, --plain and ui.motion ([383292e](https://github.com/jacob-delgado/workflow/commit/383292ed35600cd486f8c50ca5251fe20b684601))
* **cli:** say what doctor asks Slack in its test's file comment ([c41cd8e](https://github.com/jacob-delgado/workflow/commit/c41cd8e2894df26c06f4994c6cf044541588e9d8))
* **cli:** say why doctor words a forge check its time limit stopped ([929c97c](https://github.com/jacob-delgado/workflow/commit/929c97c7a94d176b6ffcef8c29570a58a635c2d8))
* **cli:** say why no forge reaches failedChecks' empty answer ([8c72df4](https://github.com/jacob-delgado/workflow/commit/8c72df400b2fc0fcce4bb36c48752233e016d7c8))
* **cli:** write down the command line's contract beside it ([dc37e53](https://github.com/jacob-delgado/workflow/commit/dc37e535bb644ad65e40eed6a35b9289c769e8dc))
* **config:** say ui.ascii and ui.motion draw a command's live line ([7978168](https://github.com/jacob-delgado/workflow/commit/79781681802ffff8f454a7c5fef38f16e6bc33fd))
* **contributing:** write the body and one-change rules down ([8644029](https://github.com/jacob-delgado/workflow/commit/86440290d48a0c0a835ecf23ffdc1b20795d3721))
* correct what TRADE-24 says is read once ([f1502bf](https://github.com/jacob-delgado/workflow/commit/f1502bfd23dfbc3c62e46af83f392163dd542a0c))
* **debt:** record cli.go and announce.go past the soft length target ([cdccc56](https://github.com/jacob-delgado/workflow/commit/cdccc566bea6e77d4e539519e1a1726964a8d097))
* **debt:** record the directory matcher twin as TRADE-52 ([da5ab44](https://github.com/jacob-delgado/workflow/commit/da5ab4405dd8eada80935c1956b7f2c37802c3ad))
* **debt:** reword the register entries the refresh outdated ([d260ddf](https://github.com/jacob-delgado/workflow/commit/d260ddff9c2d39a95f0fc2cdf9570b3efb35e05e))
* **docs:** move the site's rules beside the documentation ([17172ab](https://github.com/jacob-delgado/workflow/commit/17172ab3bc08b9bdad87879be16e693c4e98c8da))
* **features:** restamp FEATURES.md and correct FEAT-87's budgets ([c936829](https://github.com/jacob-delgado/workflow/commit/c93682993f7acdde8d7a9a32f9264dfe9105345c))
* give other coding agents an AGENTS.md that points to CLAUDE.md ([53e51cd](https://github.com/jacob-delgado/workflow/commit/53e51cd1ae7f936eac46ad4d10381161a8a3474d))
* **hugo:** say only the site's build is Node-free ([2272f1d](https://github.com/jacob-delgado/workflow/commit/2272f1daef10e9fc0861c2a74b03b40c46bd6a5f))
* name three gates where their rules are written ([70f8610](https://github.com/jacob-delgado/workflow/commit/70f8610e2322248dee09cf5e0133d7b4d9569603))
* point every surface's area guide to UX.md ([22c2fcb](https://github.com/jacob-delgado/workflow/commit/22c2fcb235968233ec776eb02e439bf513d19321))
* point UX.md's and FEATURES.md's citations at their symbols ([5ab8229](https://github.com/jacob-delgado/workflow/commit/5ab82297fb00218f1955291eefc863a31cf72628))
* **proc:** say what ends the output Wait waits for ([1159dcb](https://github.com/jacob-delgado/workflow/commit/1159dcbf7ae546d78d48f90564901fd2ea4adf6f))
* say when a web save of a Jira setting applies ([fc19de6](https://github.com/jacob-delgado/workflow/commit/fc19de621c8e7c0669e3278522d4add7f499dc6f))
* **scripts:** move the shell rules beside the gate scripts ([d6acb1f](https://github.com/jacob-delgado/workflow/commit/d6acb1fb7565ad7f934a71ccd2f6bf20c1e1d993))
* **skill:** write down what three pull requests taught the lanes ([dbde323](https://github.com/jacob-delgado/workflow/commit/dbde323343261dea6813f9ac16ba19f70e1e7d43))
* **store:** move the database rules beside the store ([a5267ce](https://github.com/jacob-delgado/workflow/commit/a5267cea557dd4fa8fad05494467a5c76d1232e0))
* **tech-debt:** point TRADE-19 at CONTRIBUTING.md's error check ([aeec475](https://github.com/jacob-delgado/workflow/commit/aeec47541723e2443a14839e27e84797010f8181))
* **tech-debt:** sharpen TRADE-24, TRADE-44 and TRADE-46 ([ea599b4](https://github.com/jacob-delgado/workflow/commit/ea599b4ef926b978ee6e85250ee09a10d04adbf5))
* **tui:** drop the ceiling sentence the budget's ratchet left behind ([fdba64e](https://github.com/jacob-delgado/workflow/commit/fdba64e59104ccaa8b7aba095fece6fea0b5c0ad))
* **tui:** move the terminal's rules beside the interface ([ba7bb4d](https://github.com/jacob-delgado/workflow/commit/ba7bb4d18233171e23ee0cf34fcd57f1ef104125))
* **ux:** list the differences the picker and Settings keep ([8fd0737](https://github.com/jacob-delgado/workflow/commit/8fd0737ea2dd6c21001fb25fa9a5e46657c46867))
* **ux:** restamp UX.md at 388e6fe6 and move its drifted pointers ([596db0f](https://github.com/jacob-delgado/workflow/commit/596db0fb0fcb99e9ae4bd21ab628bd917701eaf6))
* **ux:** restamp UX.md at main and say what stays open ([41bcb4b](https://github.com/jacob-delgado/workflow/commit/41bcb4bbe32ee5c30729084a4546ed672623f331))
* **web:** move the frontend's rules beside it ([1465c68](https://github.com/jacob-delgado/workflow/commit/1465c6849d4d74e2ad0e07a72127d09e52cdc75c))
* **web:** say the configuration file names the task program ([36a13ee](https://github.com/jacob-delgado/workflow/commit/36a13eea13f9b3fcbc75b020a6a7965a07f59b4c))
* **web:** say the Safari the page supports is desktop Safari ([401f85c](https://github.com/jacob-delgado/workflow/commit/401f85c62edbbcda927b5d8d465447fcfd7da6ce))
* **webserver:** move the web API's rules beside the server ([408482e](https://github.com/jacob-delgado/workflow/commit/408482e800ef91d3526bae2e40401eed64e394f2))
* **webserver:** point to the problem details in their own package ([8b83be4](https://github.com/jacob-delgado/workflow/commit/8b83be49b0d328b8af83b23d8038c45c4349894b))


### Build & Packaging

* install a cloud session's linters from their pin files ([2910265](https://github.com/jacob-delgado/workflow/commit/29102650a8d43878b89a0c7f62a10a414391d2ba))
* install yamllint from one hash-pinned requirements file ([e3b35df](https://github.com/jacob-delgado/workflow/commit/e3b35df13afc3c6201157a63223ff0eb414effc7))
* lock markdownlint-cli2's whole tree and install it with npm ci ([91bb741](https://github.com/jacob-delgado/workflow/commit/91bb74112a44c68d81d1ceafa20b2b29484a2aa6))
* take smol-toml 1.9.1 over markdownlint-cli2's exact 1.8.0 ([bab87f1](https://github.com/jacob-delgado/workflow/commit/bab87f1c056dadc902cced828f550db160261b9f))
* **web:** fail the build when a chunk passes its size limit ([1356398](https://github.com/jacob-delgado/workflow/commit/1356398f0c61ed01a855d4d823d51591870cb2e8))
* **web:** name the browsers the page supports ([1daf1e1](https://github.com/jacob-delgado/workflow/commit/1daf1e18fc0d256cc01132c5a06184d994e03a87))


### CI

* allow braces' advisory in dependency review, which no release fixes ([8b37803](https://github.com/jacob-delgado/workflow/commit/8b37803fb3dec2599cc0d9922a508804e37b869b))
* annotate a waiver past its week on the pull request ([205a09c](https://github.com/jacob-delgado/workflow/commit/205a09cb4881f497b44bf85e142e2eaee3e21c03))
* Bump the actions group with 4 updates ([ced4bf2](https://github.com/jacob-delgado/workflow/commit/ced4bf2f95a9b8c444dcf51333e4610d9e367fda))
* date each pin of a hashed requirements file and an npm lockfile ([2ed3076](https://github.com/jacob-delgado/workflow/commit/2ed307632d10a409d2bcfe96a23c0f6a12465ee9))
* fail a script under scripts/ that has no sibling test ([510da46](https://github.com/jacob-delgado/workflow/commit/510da4616dbf073d8fed4a25fefca2f24ccc511a))
* fail a TECH_DEBT.md path that no tracked file has ([a90cafd](https://github.com/jacob-delgado/workflow/commit/a90cafd6afc8c5bb67683c2ce765fd69b3b0a458))
* hold TypeScript 7 back from Dependabot until its migration ([54421e8](https://github.com/jacob-delgado/workflow/commit/54421e8045dba5b822ee6d87508b958f8b414c9c))
* leave test-only packages out of the statement total ([92b7cfb](https://github.com/jacob-delgado/workflow/commit/92b7cfb7b9558223a6e259cae28bcc873f07f5b7))
* let Dependabot move the markdownlint-cli2 lock ([f7d95c4](https://github.com/jacob-delgado/workflow/commit/f7d95c44f975774f3aa4645ca791fc1c70dc5684))
* vet every release platform's build, tests and all ([c898523](https://github.com/jacob-delgado/workflow/commit/c898523b2c27c7ebbf570f40a14cee21db879a43))
* warn of a dependency waiver a week past its day ([2f1d9bb](https://github.com/jacob-delgado/workflow/commit/2f1d9bb3e5f0052f70d81cece5088ab990fc45e9))
* **web:** run the layout, axe and page specs on Firefox and WebKit ([fb8cfa2](https://github.com/jacob-delgado/workflow/commit/fb8cfa2430e8194d9c99b5655b25ed14308b7d42))


### Tests

* **cli:** catch an empty wait line and a passing check named failed ([1d12730](https://github.com/jacob-delgado/workflow/commit/1d12730844d337d60e0e7eb8af24ebb7394403a0))
* **cli:** catch eight more pr and announce conditions ([e68c443](https://github.com/jacob-delgado/workflow/commit/e68c443d7d0d266816968a30c182925d56383299))
* **cli:** catch tags previewed for an announcement with no one to tag ([e9fd22e](https://github.com/jacob-delgado/workflow/commit/e9fd22e7f93f81453dde474a5d0258f22bf26bbb))
* **cli:** catch three announce conditions no test told apart ([ee45555](https://github.com/jacob-delgado/workflow/commit/ee45555f05908b6b6288cfb4bcd4ad0e49533826))
* **cli:** drop the root's copies of the command kit's own tests ([5aea82f](https://github.com/jacob-delgado/workflow/commit/5aea82fdc49b2346948896f61a35a29709d576c4))
* **cli:** gather the fixtures doctor's tests share into clitest ([245457c](https://github.com/jacob-delgado/workflow/commit/245457c7110df69fb1d5c83b48de5aa8ad0e9b6a))
* **cli:** gather the shared test fixtures into internal/cli/clitest ([73f26e3](https://github.com/jacob-delgado/workflow/commit/73f26e33a2e8816da90a92e27011bbddeb9afe0f))
* **cli:** hold a still live line to starting no timer for its mark ([8d8d2b1](https://github.com/jacob-delgado/workflow/commit/8d8d2b136ab09df152949ec1234488d6cba31ebd))
* **cli:** hold doctor's report and rows to their every word ([97c0873](https://github.com/jacob-delgado/workflow/commit/97c0873041d080ce8585a9d270bd9ebfb175b244))
* **cli:** keep the counting webhook with pr's tests, its only users ([04a121a](https://github.com/jacob-delgado/workflow/commit/04a121ae5fdaebec1e1401315331e6b2bdc8cd13))
* **cli:** lift the forge, Slack and prompt fakes into clitest ([0badac9](https://github.com/jacob-delgado/workflow/commit/0badac9da45443474925a39fec5608f231f3e21a))
* **cli:** make the tests' repositories in clitest, once ([72c2540](https://github.com/jacob-delgado/workflow/commit/72c25409964c708ed35d450e526daa999c98946f))
* **cli:** pin five facts doctor reports that no test held ([223729a](https://github.com/jacob-delgado/workflow/commit/223729ad97962a55c7c43939a51d4fa30974fa0a))
* **cli:** pin four branches of the issue writes ([9b22a9d](https://github.com/jacob-delgado/workflow/commit/9b22a9dcfa0ebf3a9bf8b2eb6848e48da269a066))
* **cli:** pin issues' silence over a list shown whole ([ba59501](https://github.com/jacob-delgado/workflow/commit/ba59501b352dffe65eb7b17032c6f2c8f4e24d4e))
* **cli:** pin six more facts doctor reports that no test held ([0a0244c](https://github.com/jacob-delgado/workflow/commit/0a0244cccb0388c21b3a6b55cbe5a5bafebda1d6))
* **cli:** pin the base remote's mask and the verdict's order ([4415285](https://github.com/jacob-delgado/workflow/commit/4415285f678cfab6fb857744d980bed62817ca48))
* **cli:** print a sparse issue in internal/cli/issues ([e37db93](https://github.com/jacob-delgado/workflow/commit/e37db9360b61f55a398bba987ff4634658b3feec))
* **cli:** reach doctor's last untested conditions both ways ([1e77862](https://github.com/jacob-delgado/workflow/commit/1e7786281c57b020545689e2b85f21f311b8266c))
* **cli:** reach status's last two conditions both ways ([88cd45a](https://github.com/jacob-delgado/workflow/commit/88cd45acfd085c7865c96e4a1ecd8cb9df73cc98))
* **cli:** split doctor's JSON tests by whether they go online ([d980675](https://github.com/jacob-delgado/workflow/commit/d98067558e325602263a93d5571774a01f9b40ec))
* **cli:** test the kit's Windows path clauses beside them ([d811de1](https://github.com/jacob-delgado/workflow/commit/d811de1524d482f35abfd00f31d96bf074a08ec1))
* **cli:** wait on the live line's frames, not on sleeps ([7eec72d](https://github.com/jacob-delgado/workflow/commit/7eec72d782a0cebfc8f987a3ace3225e4aafae2f))
* **convention:** move the commit tests beside the commit convention's ([5383909](https://github.com/jacob-delgado/workflow/commit/5383909c56bcb0542b5495476fcffc70ed097678))
* **convention:** offer no scope for files in separate trees ([8e8bdea](https://github.com/jacob-delgado/workflow/commit/8e8bdead787d29fab80960f5ede0f55166c49aa4))
* **convention:** see every condition both ways ([4a7520b](https://github.com/jacob-delgado/workflow/commit/4a7520b4692c5d8c8490042e6192949bd332f21c))
* **directory:** split the directory's tests along their behaviors ([71f4492](https://github.com/jacob-delgado/workflow/commit/71f44928fee8ab4efb893d11aa6ec52348479bd9))
* **e2e:** keep git's detached maintenance out of the server fixture ([e885306](https://github.com/jacob-delgado/workflow/commit/e885306ff8911f89eb8cfd7d163abce48d0e5782))
* **e2e:** pin both signs the Tab walk takes for focus leaving unseen ([4f546a6](https://github.com/jacob-delgado/workflow/commit/4f546a6ffdca0b725d2d0907750a9449e6e7b33e))
* **e2e:** run the Tab walk's own spec on Firefox and WebKit ([b71d2d2](https://github.com/jacob-delgado/workflow/commit/b71d2d212888f3a7068a431e313de97be3987ef5))
* **gittest:** keep git's detached maintenance out of test repositories ([c98dbe0](https://github.com/jacob-delgado/workflow/commit/c98dbe0274e534a4de43959a1eb787b93390f96a))
* hold both directory pickers to one shared row cap ([d3b556f](https://github.com/jacob-delgado/workflow/commit/d3b556f09b2bce12b1c533c189cc754b56aa627f))
* **lifetime:** bound the wait for a detached context's end ([3fffb79](https://github.com/jacob-delgado/workflow/commit/3fffb792ba7d1ff65232d2930ece953a04a51742))
* **proc:** count the grace from the exit and across every wait ([d420aa9](https://github.com/jacob-delgado/workflow/commit/d420aa9edad656e0b8e9b650843ec09b49374f01))
* **proc:** count the lines a grace in all lets through ([9691f72](https://github.com/jacob-delgado/workflow/commit/9691f726007826eef6c9698cac91ba2259cf5366))
* **proc:** end a run whose lines nobody reads once ctx is canceled ([4d72d2d](https://github.com/jacob-delgado/workflow/commit/4d72d2d695ed7a5e8979d3666b3c0b3fab9e42d9))
* **proc:** end a waiting helper once its test ends, however it ends ([213078e](https://github.com/jacob-delgado/workflow/commit/213078e9136aa1a351c0585356818c99ebacddbd))
* **proc:** give a reader slow past the grace every line ([05eff94](https://github.com/jacob-delgado/workflow/commit/05eff94554c08c1feac540dbf4ee8d2ad2e32749))
* **proc:** keep Lines open until a program that ended its output exits ([d994ffb](https://github.com/jacob-delgado/workflow/commit/d994ffb77b103b3a4b23f540392f347ba91aa4e1))
* **proc:** kill a left-behind helper however its test ends ([0eb566e](https://github.com/jacob-delgado/workflow/commit/0eb566eea69177b6e040b0906992f6d31247d7f7))
* **proc:** kill what the sh left-behind tests leave however they end ([1369306](https://github.com/jacob-delgado/workflow/commit/136930692104514a2305462a6ce27d4fc5e85ce2))
* **proc:** leave nothing running once a run has given up its output ([0e1bf78](https://github.com/jacob-delgado/workflow/commit/0e1bf7880defa33a90110619c80e79122c30c8d0))
* **proc:** make the run itself the Act of the goroutine test ([6e587c1](https://github.com/jacob-delgado/workflow/commit/6e587c1e519a9a5a75c906b3af13d4bcd7ddca59))
* **proc:** name the line buffer unreadLines has to stay above ([2ba8134](https://github.com/jacob-delgado/workflow/commit/2ba8134ed43e552ba66d9f2e5f5fab57832f4312))
* **proc:** read a left-behind line only once the run takes the exit ([cc77307](https://github.com/jacob-delgado/workflow/commit/cc7730794b88cb7703cfeaecd59378375670f4a3))
* **proc:** report an empty stderr and a program that could not run ([fb22664](https://github.com/jacob-delgado/workflow/commit/fb22664407cabb9735590a6ecd5923a72da0bfb4))
* **proc:** require the goroutine test's run to have given something up ([27ad8ba](https://github.com/jacob-delgado/workflow/commit/27ad8ba7b6c9cf4071b78a8bf70e7cdfa0ac051f))
* **quietinput:** ask a running form on a terminal through Run ([b29c5a8](https://github.com/jacob-delgado/workflow/commit/b29c5a81d97528e4f2fe9afe36e0f9e54c79291b))
* **quietinput:** ask for a secret only on a terminal ([910a981](https://github.com/jacob-delgado/workflow/commit/910a98150268b37c81e42871ce4cac59eff5ad2c))
* **quietinput:** cycle the suggestions from a value typed whole ([a8cf5f2](https://github.com/jacob-delgado/workflow/commit/a8cf5f2f0698ed56ac6f9abcb09e79e87d63cf3f))
* **quietinput:** hold an untouched value apart from option order ([2be44b7](https://github.com/jacob-delgado/workflow/commit/2be44b7a1c5a57ab8ced11526d58eb094b73b519))
* **quietinput:** hold two completion tests to what their Act does ([5055d99](https://github.com/jacob-delgado/workflow/commit/5055d99d5f2cea98495d6d19f2a05d23be46ce3d))
* **quietinput:** leave an untouched value through an empty paste ([b0a6ba1](https://github.com/jacob-delgado/workflow/commit/b0a6ba182dd923db7f24db1110f7f2b0d54b339c))
* **quietinput:** move the suggestion tests to a file of their own ([73e7152](https://github.com/jacob-delgado/workflow/commit/73e715279def8d34cc179a4492d7345a5c6d0868))
* **quietinput:** never type the key leaving the field into it ([6e66f53](https://github.com/jacob-delgado/workflow/commit/6e66f53e4d916d376b003fc1af3eae116a2f285b))
* **scripts:** count Chromium's end-to-end projects in the summary ([e31ec95](https://github.com/jacob-delgado/workflow/commit/e31ec95311c28be088c49e4d3851cf18cb7d10fa))
* **scripts:** keep git's detached maintenance out of the script tests ([91cbee6](https://github.com/jacob-delgado/workflow/commit/91cbee6ec0ce038a1ffaded388402259138ffcd2))
* **tui:** hold the base tests to leaving the base field ([7f1720c](https://github.com/jacob-delgado/workflow/commit/7f1720c3431780a9a12ffeecfeb7b8f74778b628))
* **tui:** keep a base typed whole in the pull request composer ([086a07f](https://github.com/jacob-delgado/workflow/commit/086a07f4d125af5ef3b6b4c328ee51c66b8e5f52))
* **tui:** keep the base the pull request composer opens at ([d0ecf00](https://github.com/jacob-delgado/workflow/commit/d0ecf000a7135401300122cb9c9f7d85cb39fbb4))
* **tui:** list nothing a failed directory read handed back ([888783e](https://github.com/jacob-delgado/workflow/commit/888783e4a87ddc4f81f98fd909521f2290464a4c))
* **web:** count what both listings hold once the whole one answers ([472ebcb](https://github.com/jacob-delgado/workflow/commit/472ebcb920fe9d7a87c7030c6da88de41d8e6a6f))
* **web:** hear a policy refusal the way every engine reports it ([7aa1d08](https://github.com/jacob-delgado/workflow/commit/7aa1d0850375969adacd4db086d177888541835a))
* **web:** launch the Chromium PLAYWRIGHT_CHROMIUM_EXECUTABLE names ([c18a9b4](https://github.com/jacob-delgado/workflow/commit/c18a9b4dc4e2b63d85ddc85bc2490812db4cdfc6))
* **web:** name what each refused style or script came from ([85fa9e4](https://github.com/jacob-delgado/workflow/commit/85fa9e495ee95db9c9657307abea0fcb622e1a37))
* **web:** open Settings' tabs and the tag picker under the policy ([cc5f28f](https://github.com/jacob-delgado/workflow/commit/cc5f28fda2dbd14731328864d32a8bc2a92a00fc))
* **web:** open the directory picker under the content policy ([ff6d96f](https://github.com/jacob-delgado/workflow/commit/ff6d96f304376617cc0ef409461a5b462379aed4))
* **webserver:** cover the run's index lock and its tail ([7ae940b](https://github.com/jacob-delgado/workflow/commit/7ae940bc918de733a2665ba1dccdad41e4eb3370))
* **webserver:** split the issue-write and repository tests ([80ee404](https://github.com/jacob-delgado/workflow/commit/80ee404c9b95cd9c3c89e1b114387bd496c71fdb))

## [0.11.0](https://github.com/jacob-delgado/workflow/compare/v0.10.0...v0.11.0) (2026-10-14)


### ⚠ BREAKING CHANGES

* **cli:** --log is now --request-log; --log FILE is refused with exit status 2.
* **cli:** an error reads "ERROR <message>" rather than

### Features

* **api:** name each refused setting in a save's problem ([ceb0521](https://github.com/jacob-delgado/workflow/commit/ceb0521aeb75fa79e0323f0c8bfdd31f50278c68))
* **api:** narrow GET /api/directories by a name prefix ([38e7752](https://github.com/jacob-delgado/workflow/commit/38e7752c57ab18b1bef0407ddd68519130928a74))
* **cli:** add --verbose, --quiet and --log-format ([ce41521](https://github.com/jacob-delgado/workflow/commit/ce4152137049caafcd4de482170ca007c529c725))
* **cli:** keep a live line while a command waits on a terminal ([24cba9a](https://github.com/jacob-delgado/workflow/commit/24cba9a0d70fd4399623f0237152592b32002330))
* **cli:** rename --log to --request-log ([b6504b8](https://github.com/jacob-delgado/workflow/commit/b6504b8055a64e20815206067d09c0fd4f048612))
* **cli:** say notes, warnings and errors through one log ([ea03b50](https://github.com/jacob-delgado/workflow/commit/ea03b50c076ffdfdd7740e12547503918467136f))
* **cli:** show CI's progress while announce waits for it ([ef3cc81](https://github.com/jacob-delgado/workflow/commit/ef3cc818a365a2c6083643414df8cdf321efd372))
* **cli:** style help, usage and errors with fang ([45af262](https://github.com/jacob-delgado/workflow/commit/45af2626fb035730db897c94f187b002436b3e52))
* **config:** add ui.motion, which reduce turns to stillness ([b327a21](https://github.com/jacob-delgado/workflow/commit/b327a21bff793735c5802d002b3ef51c2a9f953f))
* **config:** name the setting each validation refuses ([f2e160e](https://github.com/jacob-delgado/workflow/commit/f2e160e8c1c590022bdaeca7f5fed50c20c33beb))
* **tui:** ask the first run's questions in Huh forms ([b5cae7a](https://github.com/jacob-delgado/workflow/commit/b5cae7ab468e0a70ee4fb9d6cb2066c423100e04))
* **tui:** browse for a directory in a picker dialog ([c38c1f7](https://github.com/jacob-delgado/workflow/commit/c38c1f778b56bcec52b93e9af162bc9860c236cc))
* **tui:** compose a pull request in two Huh forms ([f38f4b9](https://github.com/jacob-delgado/workflow/commit/f38f4b9d4ded6d0cf33fe629ef59d807d44685ae))
* **tui:** draw the loop as a track of filled and open rules ([5ddaeaa](https://github.com/jacob-delgado/workflow/commit/5ddaeaa52d3bd13a39ff534b7b055648f7a562f8))
* **tui:** edit a Settings section in a Huh form ([134eab2](https://github.com/jacob-delgado/workflow/commit/134eab2bf01e95d5488196f7d12bc3748289add6))
* **tui:** fill a stage's rule as the stage completes ([0ac2146](https://github.com/jacob-delgado/workflow/commit/0ac214674bb70efa0669be33b50f9bb583599a34))
* **tui:** fill in a status change's fields in a Huh form ([151bafc](https://github.com/jacob-delgado/workflow/commit/151bafc6362c052f3e574ce2d67b2633edc257eb))
* **tui:** float the keys, last looks and checklists as dialogs ([8d95e3e](https://github.com/jacob-delgado/workflow/commit/8d95e3e66450654dd70d1fd6e00a17d64b47aacb))
* **tui:** hand a form to Huh's accessible prompts on request ([97db146](https://github.com/jacob-delgado/workflow/commit/97db146303da9941007b0634acafaf5c1f8e8ecb))
* **tui:** spring the Review pane's bar toward checks finished ([9dd097e](https://github.com/jacob-delgado/workflow/commit/9dd097e7207dd9d0f9cdbeaeb9874b50e06dd3ba))
* **tui:** turn the in-flight mark while a read is out ([77d112e](https://github.com/jacob-delgado/workflow/commit/77d112e9dcc4cf2c7e800826094746e7d153abbd))
* **web:** browse for a directory in a dialog with typeahead ([777dcaf](https://github.com/jacob-delgado/workflow/commit/777dcafb85ed1e7c80afabad3cf10ad2ad5eb2be))
* **web:** draw dialogs and the command palette with Base UI ([3208066](https://github.com/jacob-delgado/workflow/commit/3208066a80376f7bee64fa3f4e0da7104f7d956b))
* **web:** draw the loop as a track under the header ([518d3c9](https://github.com/jacob-delgado/workflow/commit/518d3c9927de9e1d8030aded7f880de990d025d3))
* **web:** keep a save bar in view with the unsaved count ([d78f113](https://github.com/jacob-delgado/workflow/commit/d78f113018cfea94510222933364ef4d689ee5c0))
* **web:** mark each section's changes and refusals on its tab ([4d7816e](https://github.com/jacob-delgado/workflow/commit/4d7816e82a696a3b5b3144f94891b623bfd04b76))
* **web:** mark refused fields inline across the forms ([abec28e](https://github.com/jacob-delgado/workflow/commit/abec28ec36c0ddcc0b3ed5277263af7d644be876))
* **web:** pick whom an owner is on Slack by typing their name ([dea5d63](https://github.com/jacob-delgado/workflow/commit/dea5d63ff578800cd67312e4839fe97268f2cfb0))
* **web:** say beside each setting why it was refused ([9b146bd](https://github.com/jacob-delgado/workflow/commit/9b146bdde0e7ae282fd4c9d454d6d26b2731b9d7))
* **web:** say where each stored credential is kept ([efe27f4](https://github.com/jacob-delgado/workflow/commit/efe27f42fccfed002a39bfd8f8701bf1d975d623))
* **web:** set the page in Recursive, its mono axis for code ([e72fd54](https://github.com/jacob-delgado/workflow/commit/e72fd54c8ab4e6b59621b19b2fa66ee19ddc6dd4))
* **web:** show Settings a section at a time beside its list ([13f05c7](https://github.com/jacob-delgado/workflow/commit/13f05c74bd524cf1d578d42027236a61e8674300))
* **workdirs:** rank names by prefix, then by letters in order ([8684aa8](https://github.com/jacob-delgado/workflow/commit/8684aa8c16d68692b71c23fe0100c47a36a11b95))


### Bug Fixes

* **release:** say brew tap-new failed, not that it made no tap ([bc7adaa](https://github.com/jacob-delgado/workflow/commit/bc7adaa73b163d35bfb1d86915ed0002f016bd64))
* **web:** pin the save bar to the bottom of the content ([5ac8fe8](https://github.com/jacob-delgado/workflow/commit/5ac8fe8c62c8e3a1c42cce10f03d763bbec64cd3))
* **workdirs:** narrow a listing by a prefix in any case ([c05ba96](https://github.com/jacob-delgado/workflow/commit/c05ba9644e233bdc219953ce7cdf6e1fdff1a67e))


### Performance

* **tui:** type a form's text in a field that never blinks ([39af1a6](https://github.com/jacob-delgado/workflow/commit/39af1a6e8a713b7c536b717ac26e7cd8e1df8b1b))


### Refactors

* **cli:** move the root's flags out of cli.go ([785776a](https://github.com/jacob-delgado/workflow/commit/785776af82f0fedfe0b4f2b2620ed0ee9adb287f))
* **tui:** assert at compile time that a motion tick applies ([c317303](https://github.com/jacob-delgado/workflow/commit/c31730305e6938162c732898918a204808f17dbd))
* **tui:** label the failed check's choices from a map ([9e2c767](https://github.com/jacob-delgado/workflow/commit/9e2c7671c8930f639431a1092f3c665a60341587))
* **tui:** move Model's options out of tui.go ([0d83652](https://github.com/jacob-delgado/workflow/commit/0d8365231150300b5136f491968c1592a380f9a3))
* **tui:** move the go-to prompt's footer beside its overlay ([5d085b1](https://github.com/jacob-delgado/workflow/commit/5d085b1293eca1d5d821192c68b373f52b9129e8))
* **tui:** name what draws an overlay in the detail pane ([7695aba](https://github.com/jacob-delgado/workflow/commit/7695abaf79075830c0351c695d845c315a95b3b2))
* **web:** keep each stage's section in sections.ts ([ba03c9b](https://github.com/jacob-delgado/workflow/commit/ba03c9b1f7e01d60788e08944f12466b22a35def))
* **web:** mark refused settings through the fields' own error ([a927159](https://github.com/jacob-delgado/workflow/commit/a92715975d0cad29cbd70f171ee9b28e0abd3a88))
* **web:** merge the small settings fieldsets by section ([9c9eac9](https://github.com/jacob-delgado/workflow/commit/9c9eac9a3e77f0edc7b26337e63885c812aa642b))
* **webserver:** read an optional query parameter in one place ([789022e](https://github.com/jacob-delgado/workflow/commit/789022e3f9290b25da13169b3fd8352935a5acd1))


### Documentation

* **contributing:** say CI audits a stand-in and publishes no formula ([f1818f4](https://github.com/jacob-delgado/workflow/commit/f1818f48e52d4f1728c947bf9a05b4f9f45dc444))
* describe dialogs and accessible forms ([c38bca9](https://github.com/jacob-delgado/workflow/commit/c38bca9ef0821dcbe00551c22d3d4643184a7f9d))
* describe the directory dialog on the web ([0651958](https://github.com/jacob-delgado/workflow/commit/0651958d59c8a4ab1f76688d1a0adc92978ed174))
* describe the loop track and ui.motion ([8b5f0f5](https://github.com/jacob-delgado/workflow/commit/8b5f0f59c7c82f52f2cd3d4ac973a197e21f5ae5))
* describe the terminal's directory picker ([60afcb9](https://github.com/jacob-delgado/workflow/commit/60afcb9baa1fbbc68cabc8b0764dda2afa4c2154))
* **scripting:** describe log levels, formats and the live line ([2369d1c](https://github.com/jacob-delgado/workflow/commit/2369d1c745b65b56b89b9925471e4264be6acb73))
* **usage:** describe the terminal's forms and their keys ([f872898](https://github.com/jacob-delgado/workflow/commit/f8728987e383b53a418d1337250207d716acf19d))
* **web:** describe Settings' sections and its save bar ([05aabd8](https://github.com/jacob-delgado/workflow/commit/05aabd8791b3fc79fff616bd9aaa6a553d49f9af))
* **web:** describe the typeface, the track and the dialogs ([5afbbc3](https://github.com/jacob-delgado/workflow/commit/5afbbc3d47f16db04c2613940eff718fa69841ed))


### Build & Packaging

* **release:** fail the formula audit on a Homebrew deprecation ([388d78c](https://github.com/jacob-delgado/workflow/commit/388d78c2eb2009ea853c7cdafebfac442c943d86))
* **release:** name the Homebrew that audits the formula ([d24e4b2](https://github.com/jacob-delgado/workflow/commit/d24e4b244dc53e15f8e881591141e6ae8bd83e9d))


### Tests

* **config:** read ui.motion's reduce both ways ([231635e](https://github.com/jacob-delgado/workflow/commit/231635e37b7a07d8ed2733efa1b23536f8bfad93))
* **e2e:** hold the directory dialog to every width and theme ([59a41cf](https://github.com/jacob-delgado/workflow/commit/59a41cf464b11668720b69dde3134693ffb64d1c))
* **e2e:** hold the Settings sections to every width and theme ([aaebb8c](https://github.com/jacob-delgado/workflow/commit/aaebb8c88aec923fe747758b8363e2fcf36f4a93))
* **e2e:** wait for focus to leave a dialog's focus guard ([8994472](https://github.com/jacob-delgado/workflow/commit/8994472ed1136a1075a7ce617b02d8100212bcb4))
* **tui:** hold the track's widths and ASCII in one table ([4df234f](https://github.com/jacob-delgado/workflow/commit/4df234f65ddb9de0577fc1d0c76e74cb6983b803))
* **tui:** keep the tests off the repository git's environment names ([b918109](https://github.com/jacob-delgado/workflow/commit/b91810977c882c77e1424af61c9a7e7d14b37fab))
* **tui:** run a sequence of commands in the test harness ([4bb226a](https://github.com/jacob-delgado/workflow/commit/4bb226a57c6656ab3d070edb1b99cff083df198b))

## [0.10.0](https://github.com/jacob-delgado/workflow/compare/v0.9.1...v0.10.0) (2026-10-14)


### ⚠ BREAKING CHANGES

* **loop:** `workflow announce --yes` exits 4 on a pull request a check failed on, where it used to post; POST /api/announce answers 409 with code ci_failed there; loop.Deliver takes DeliverSeams and a Delivery carrying the pull request in place of Made.
* **api:** POST /api/runs no longer takes kind pre_commit; send {"kind":"hook","hook":"pre-commit"}. GET /api/hooks lists the hooks it takes.
* **tui:** the ui.keys action run-pre-commit is renamed run-hook. A map that still names run-pre-commit is refused at start as an unknown action (exit status 3); rename it. h now opens a list of lefthook's hooks; enter runs pre-commit, as h alone did.

### Features

* **api:** run any hook lefthook runs by hand through /api/runs ([8c7adad](https://github.com/jacob-delgado/workflow/commit/8c7adad3cd4b7dbf512f5cec49a42487e97887a4))
* **api:** send a view's kept issues before the first snapshot ([9338fd3](https://github.com/jacob-delgado/workflow/commit/9338fd3497410997c75581006455e379566130de))
* **cli:** announce and post the summary in the channel asked ([87d5630](https://github.com/jacob-delgado/workflow/commit/87d563053b59b164936ccff1df23f7555f184dc8))
* **cli:** assign an issue with workflow assign ([99f594c](https://github.com/jacob-delgado/workflow/commit/99f594c9cfcc9a6d6fd1730ccecb43b2cf02b476))
* **cli:** complete the issue key of every issue command ([461f526](https://github.com/jacob-delgado/workflow/commit/461f526c307e5cc5a767382621f3ead8c882c78f))
* **cli:** draw status's marks in ASCII with --plain ([7b5796b](https://github.com/jacob-delgado/workflow/commit/7b5796b1d95b83a1192830f181a31e10ecefbaa3))
* **cli:** finish a merged branch with workflow finish ([91cfb1c](https://github.com/jacob-delgado/workflow/commit/91cfb1c38b86da8818e5436969ac3683022ce5de))
* **cli:** list a view's issues and read one with issues and issue ([7927a9c](https://github.com/jacob-delgado/workflow/commit/7927a9c58460bcdc15805c09e90b0c8865bbb9d9))
* **cli:** log work on a Jira issue with workflow worklog ([92aced6](https://github.com/jacob-delgado/workflow/commit/92aced64cd5e656ea681afaf5e24e30d787e227d))
* **cli:** move an issue to a status with workflow transition ([a9060fa](https://github.com/jacob-delgado/workflow/commit/a9060fa89e2bb51d24ca425d5e7d8affc80b80fb))
* **cli:** open a draft, into a chosen base, asking chosen reviewers ([3d119c4](https://github.com/jacob-delgado/workflow/commit/3d119c499c89c27ba8eca0ba49b52d868b3283e1))
* **cli:** open and then announce once CI passes with pr --announce ([3ed69e3](https://github.com/jacob-delgado/workflow/commit/3ed69e33f6cd0d826c53d2418f8d525e6519efb9))
* **cli:** spell --dry-run, --yes and --json as -n, -y and -j ([648390c](https://github.com/jacob-delgado/workflow/commit/648390c9e73db5384be60c4362d9d4b2ea516c82))
* **cli:** title, describe and template a pull request from flags ([9eeb89c](https://github.com/jacob-delgado/workflow/commit/9eeb89c237504e47740778ee40176b9c3d92114d))
* **cli:** wait for CI to pass with announce --when-green ([001fcce](https://github.com/jacob-delgado/workflow/commit/001fcce7638c42b3df35144f6894b6f03d3d34f8))
* **config:** turn automatic announcing off with messaging.auto_announce ([d0f412b](https://github.com/jacob-delgado/workflow/commit/d0f412bcdb21b724375fe11c8fc46f163a68c745))
* fetch the base remote before starting work ([78f7ba1](https://github.com/jacob-delgado/workflow/commit/78f7ba117e35a2164d7284ac26242159dff5a29a))
* **forge:** find and open a GitHub pull request from a fork ([ab30013](https://github.com/jacob-delgado/workflow/commit/ab3001399ac05f5c6039ab156cddf3b2c1e8a70b))
* **forge:** find and open a GitLab merge request from a fork ([cec7469](https://github.com/jacob-delgado/workflow/commit/cec7469cdfd6ea29696ea84f0379c601f4dddfaf))
* **gitrepo:** push a branch where its own pushRemote says ([2509ca0](https://github.com/jacob-delgado/workflow/commit/2509ca0c1c025e3cc2fd012f436048ce311443bb))
* **gitrepo:** read the base on the remote gh marked, else upstream ([64b8752](https://github.com/jacob-delgado/workflow/commit/64b875245f0c583a2b449a39974c376005ca206d))
* **hooks:** read which configured hooks a run by hand can run ([1b01e71](https://github.com/jacob-delgado/workflow/commit/1b01e713d099473bbdb59142c7acf0488f1d374e))
* **loop:** announce a pull request at two moments, ready and merged ([684e32a](https://github.com/jacob-delgado/workflow/commit/684e32ac4c983efcda916db1c151fe5c8b5e0d15))
* **loop:** compose a pull request into the base a surface chose ([b239d5b](https://github.com/jacob-delgado/workflow/commit/b239d5ba0370f16dd30db7cbb0e63fafccddbeb0))
* **loop:** draft a pull request from the commits past the base chosen ([d5ab1fa](https://github.com/jacob-delgado/workflow/commit/d5ab1facef1dbeddf050cc6a0f541aaa2d71e8d2))
* **loop:** refuse to announce a pull request a check failed on ([7f4d6fd](https://github.com/jacob-delgado/workflow/commit/7f4d6fdbb160dd4e2d0714917b650481312bc152))
* **loop:** watch an armed announcement's CI until it passes ([9d230ac](https://github.com/jacob-delgado/workflow/commit/9d230ac45678ba18505bfc49adac59bd23d5a76c))
* **release:** point a Homebrew tap at each release ([4d34cba](https://github.com/jacob-delgado/workflow/commit/4d34cba94e1cab30f9e36fc13f5204f628989a47))
* say where a push goes when it is not the base remote ([a6d8a6e](https://github.com/jacob-delgado/workflow/commit/a6d8a6e820d33adb50db0fdc9c774a1b5e850194))
* show the base repository beside origin where they differ ([6ac5103](https://github.com/jacob-delgado/workflow/commit/6ac5103fe45a267a7ae8628afb101a9b314c05e1))
* **tui:** arm a pull request's announcement as it opens ([ac185d3](https://github.com/jacob-delgado/workflow/commit/ac185d3004b7ff80fdf5b849dd9f1b9367887134))
* **tui:** choose which of lefthook's hooks h runs ([e72b5a4](https://github.com/jacob-delgado/workflow/commit/e72b5a497acaee190b483d804b19a42244445cc1))
* **tui:** draft the pull request again onto the base typed in ([3ecdd5c](https://github.com/jacob-delgado/workflow/commit/3ecdd5c2349d4af7399be749a14ddb5153ccefd6))
* **tui:** hold a waiting announcement through a failed run ([cd67dfd](https://github.com/jacob-delgado/workflow/commit/cd67dfd04a9a3c9c174de22b17ccb10a5ab298ca))
* **tui:** say why a merged branch cannot be finished ([2215e58](https://github.com/jacob-delgado/workflow/commit/2215e58155bcf1df3fc820c87ac9efd5b605f5b5))
* **web:** choose the hook Run a hook runs ([94f5f6d](https://github.com/jacob-delgado/workflow/commit/94f5f6d1f24a9cd536a02386b0e3c8fd42cff73d))
* **web:** draft the pull request again past the base chosen ([0568cce](https://github.com/jacob-delgado/workflow/commit/0568cce2e6c35631a7e9cb89e82ea8dbdff49968))
* **web:** list the view's kept issues before the first update ([d1e483f](https://github.com/jacob-delgado/workflow/commit/d1e483f41217738109933dee558a12de7c22fd3c))
* **web:** name the fields a move to the review status asks for ([468de0f](https://github.com/jacob-delgado/workflow/commit/468de0f9ddc039c0bbc6d48e0718d661d4d0f5f7))
* **web:** say the opened pull request's announcement is armed ([0dc53c6](https://github.com/jacob-delgado/workflow/commit/0dc53c67c5c4a4065790c27cacde36eeaa8efbc4))
* **web:** say what a held announcement waits on ([979a9bf](https://github.com/jacob-delgado/workflow/commit/979a9bf304a452ba43014608f3acb3faf9b5eeb4))
* **web:** say why a merged branch cannot be finished ([fca69ab](https://github.com/jacob-delgado/workflow/commit/fca69ab6ff92be2aee6e9b93a6052bb59ec5249d))
* **webserver:** arm the announcement of a pull request the page opens ([5927657](https://github.com/jacob-delgado/workflow/commit/5927657ecd01c8fe67d25a42ba49ad7581fba5fc))
* **webserver:** hold an announcement through a failed run ([9d63950](https://github.com/jacob-delgado/workflow/commit/9d6395015f46b22e79d74813e6a97d15d61ea142))
* **webserver:** keep each view's first page for the next session ([3292a8f](https://github.com/jacob-delgado/workflow/commit/3292a8f9565fb2cf34f277086cf0498cbfb51d68))
* **wiring:** list lefthook's configured hooks through the seams ([9de4ea7](https://github.com/jacob-delgado/workflow/commit/9de4ea7923cd788a448f2b46c9b317c513a0dd7a))
* **wiring:** reach the forge at the base and open from the push remote ([8d67a7c](https://github.com/jacob-delgado/workflow/commit/8d67a7c13ae56d949a550d1e3f2a223dfe14bc06))


### Bug Fixes

* catch a finished branch's base up from the base remote ([b623763](https://github.com/jacob-delgado/workflow/commit/b62376339bc9c5f6c0cc9d386163b1e9ab3c2dd8))
* **cli:** label status . by the directory it names ([1dd6f05](https://github.com/jacob-delgado/workflow/commit/1dd6f0581efe3cbb4d44e92d3f61d5a8776b79be))
* **cli:** link the key the tracker answered with in workflow issue ([128cac8](https://github.com/jacob-delgado/workflow/commit/128cac8df64988d99b0ffad7d43643f52150718a))
* **cli:** trim transition's status and refuse a blank one as usage ([fcc70b5](https://github.com/jacob-delgado/workflow/commit/fcc70b57f24d39714eb445e704dada422e674e9e))
* **forge:** pass over a fork's merge request from a same-named branch ([7383e0a](https://github.com/jacob-delgado/workflow/commit/7383e0a7bd27275262424cb41b96aa0789858c74))
* **forge:** read a GitLab pipeline's jobs, retry and log where it ran ([5cda270](https://github.com/jacob-delgado/workflow/commit/5cda270d40403d82e3ada250c16bdf07ae24d00f))
* **gitrepo:** switch to a branch two remotes hold by way of the base ([861b839](https://github.com/jacob-delgado/workflow/commit/861b8390d0356ce2699e8f43cf199acf20c40b5c))
* **hooks:** see a lefthook.jsonc as the configuration it is ([2d92cda](https://github.com/jacob-delgado/workflow/commit/2d92cdae135f94151acb8f2871059c74ffe48b90))
* **loop:** refuse a finish that would lose commits, and say why ([fba9e40](https://github.com/jacob-delgado/workflow/commit/fba9e40159b2ed32706c5a376acf4dd844e6c276))
* **release:** write a formula brew style and brew audit --strict pass ([58fc93f](https://github.com/jacob-delgado/workflow/commit/58fc93f7ffc0b13832ebde633f67eb1da2f755ec))
* **report:** carry a status under Jira's No Category as not yet started ([82d3f7f](https://github.com/jacob-delgado/workflow/commit/82d3f7f272c03915ef864b287e9c9027bf287280))
* say to re-run a merge's failed checks rather than wait for CI ([0a08891](https://github.com/jacob-delgado/workflow/commit/0a0889172db5a1d22f07402e2e4cd364e0741509))
* **tui:** drop a waiting post whose pull request merged first ([0a2a7c7](https://github.com/jacob-delgado/workflow/commit/0a2a7c7088f4d2e5ab317709c5be66b12000b137))
* **tui:** keep reading CI for a waiting post after a read fails ([db7adbf](https://github.com/jacob-delgado/workflow/commit/db7adbfbd1367dbb44a6a9317db6e9d84bd5967e))
* **tui:** re-read the store before a held post goes ([654f56b](https://github.com/jacob-delgado/workflow/commit/654f56b8a4b2de46fed096d2f8e29d3b201dff96))


### Refactors

* **cli:** build announce's seams once for announce and pr ([b60d86e](https://github.com/jacob-delgado/workflow/commit/b60d86ef7f40119c01a72c831278919d002c2d82))
* **cli:** declare every --json flag through one helper ([bbe4d7b](https://github.com/jacob-delgado/workflow/commit/bbe4d7b8d142e2ae2424f53ddd69754fb4108170))
* **cli:** keep pr's offers after opening in a file of their own ([0ab0f41](https://github.com/jacob-delgado/workflow/commit/0ab0f41ae91dcc558ad398e0f4ed858fc3cdd56a))
* **cli:** keep the issue-key completion beside the issue commands ([8231f43](https://github.com/jacob-delgado/workflow/commit/8231f43db1321e6de7963e38d3de52f3da9676d4))
* **cli:** name status's labels and JSON rows for directories ([8f0d0ee](https://github.com/jacob-delgado/workflow/commit/8f0d0ee3d625d16aea4e488c29898d20c303e157))
* **cli:** post a comment through the shared issue write ([20e1446](https://github.com/jacob-delgado/workflow/commit/20e1446ffdb62ade6258e23f6683907a19beecff))
* **cli:** wire announce's tagging seams apart from the command ([f7ff0c7](https://github.com/jacob-delgado/workflow/commit/f7ff0c796aadbea790f1eb4713a5c41a01faa3ae))
* derive the issue views once, in internal/jira ([6272350](https://github.com/jacob-delgado/workflow/commit/6272350a5dfd32e298d3a5226106340b8884d5b3))
* **forge:** keep GitLab's issue reads beside its issue writes ([15e1622](https://github.com/jacob-delgado/workflow/commit/15e1622b2c9e7c80f2117511e3c188a07b5ab378))
* **forge:** name a pull request's head by its branch and its fork ([deb4a7a](https://github.com/jacob-delgado/workflow/commit/deb4a7a9f55d83d8f25913326be490c29d51d317))
* **gitrepo:** call origin's address Origin ([a39e315](https://github.com/jacob-delgado/workflow/commit/a39e3152590a766572c5d47e3c2500cd00f3d823))
* **gitrepo:** read a branch's commits in a function of their own ([4e27429](https://github.com/jacob-delgado/workflow/commit/4e274290e5b7edfc377984fcbc5102a215b90c56))
* **hooks:** move the hooks git runs out of the generator ([2f0f52d](https://github.com/jacob-delgado/workflow/commit/2f0f52dabe8da150c7835206c3687bddf4f7a1cd))
* **hooks:** spell lefthook install beside lefthook run ([55b7bca](https://github.com/jacob-delgado/workflow/commit/55b7bcaacab2d37c73d3ac8e93c7c313d5b915d2))
* **hooks:** spell lefthook's file names as its loader does ([b317e09](https://github.com/jacob-delgado/workflow/commit/b317e09b334e9a71384eefbeff5c49ceb3f846a2))
* **loop:** answer the branch an announcement was composed on ([b0e0b99](https://github.com/jacob-delgado/workflow/commit/b0e0b99f682c77633bd10d5bcb78eedba0464e6b))
* **loop:** find the move to any status by name ([24e8da0](https://github.com/jacob-delgado/workflow/commit/24e8da0312af92a80232c30a5a25546666b257f8))
* map issues onto the API's shapes in internal/report ([ec3ec75](https://github.com/jacob-delgado/workflow/commit/ec3ec75da3020e49906f7d5b4e013dbe0ad912a1))
* **tui:** name the Commits pane's hook binding runHook ([63a8430](https://github.com/jacob-delgado/workflow/commit/63a8430dc73ed74691badfb2b4c4c65d0136a658))
* **web:** give each of the stream's listeners a function of its own ([99f0490](https://github.com/jacob-delgado/workflow/commit/99f0490103b8b3de820adb1259aa3f52b44be719))
* **web:** name the seven-character short hash once ([fa9332f](https://github.com/jacob-delgado/workflow/commit/fa9332f668dd32e219df11638a3a73b0b166b610))
* **webserver:** keep a view's issue cache in a file of its own ([8551fdd](https://github.com/jacob-delgado/workflow/commit/8551fdd7105866c5d9aa7cf1ab9f103bc94478de))
* **webserver:** name hooksetup.go for the hooks it serves ([8343f5a](https://github.com/jacob-delgado/workflow/commit/8343f5a37b1c01795fd81dedab9938cc2d9b6e6c))
* **webserver:** push the stream's snapshots from their own loop ([6f450c7](https://github.com/jacob-delgado/workflow/commit/6f450c76406a9dad68fbce572cd6cebf3f863f3a))


### Documentation

* **cli:** say pr --base asks owners an edited base would not ([2ccbe9b](https://github.com/jacob-delgado/workflow/commit/2ccbe9b6e2f0d07dae913513da9a7eb6f01b7cd8))
* **contributing:** say CI audits the formula each change would publish ([7984e38](https://github.com/jacob-delgado/workflow/commit/7984e38d5b17d911da636ea8bf2565fa5d008a62))
* describe a fork's pull requests; FEAT-15 is done ([2443c63](https://github.com/jacob-delgado/workflow/commit/2443c631c9ba390a2f5dd131b379a5a9e502f24a))
* describe working from a fork's clone, and narrow FEAT-15 ([1086ad3](https://github.com/jacob-delgado/workflow/commit/1086ad334252bb098a67b13beb9903c453032427))
* drop FEAT-78 from FEATURES.md now that its commands shipped ([c01c1ee](https://github.com/jacob-delgado/workflow/commit/c01c1ee657ae89c833d7ca98338cf074f8a4a166))
* drop FEAT-83 from FEATURES.md now that finish has shipped ([5e27562](https://github.com/jacob-delgado/workflow/commit/5e275628d774a4622ffcc659b88c02e301e27d63))
* drop FEAT-85 now that the web paints from the cache ([fed162b](https://github.com/jacob-delgado/workflow/commit/fed162be061c5c3729df4d6116c7c3d466874a27))
* **features:** retire FEAT-24 now that any hook can be run ([6a65940](https://github.com/jacob-delgado/workflow/commit/6a65940857cede317c0166a9c267c9b8f3568561))
* **install:** install with Homebrew ([6b2d87f](https://github.com/jacob-delgado/workflow/commit/6b2d87f09267e0674de3f9b4396eca5aea2788be))
* **install:** set off the tools doctor reports for either forge ([5e9c67e](https://github.com/jacob-delgado/workflow/commit/5e9c67eb8e504d127adc36b13734eb983ab9da2d))
* **loop:** say which template ComposePull starts from ([44fe66b](https://github.com/jacob-delgado/workflow/commit/44fe66bc250955b183d872ade70e74c51e323e36))
* say every surface gates and remembers its announcements ([49dde7f](https://github.com/jacob-delgado/workflow/commit/49dde7f62892ecfb001707c941f2a7cac39082d4))
* say the first page of issues holds the forge's issues too ([2f6b65d](https://github.com/jacob-delgado/workflow/commit/2f6b65db436bbb8b7794697497bd78feaff73cc2))
* **ux:** drop UX-62, now that the scriptable commands take its flags ([9ee3d7a](https://github.com/jacob-delgado/workflow/commit/9ee3d7abf25bf7d38cd00919d3596b0407bae281))
* **ux:** drop UX-92, now that a script can join and time the JSON ([ead8bc3](https://github.com/jacob-delgado/workflow/commit/ead8bc3d4441d81f586ef3312616bf61f7b7e77b))


### Build & Packaging

* **release:** audit the Homebrew formula in a throwaway tap ([7cd64bf](https://github.com/jacob-delgado/workflow/commit/7cd64bf7aac9f5a07fda7adca93a9028601b9449))
* **release:** commit the formula to the Homebrew tap ([3d4e964](https://github.com/jacob-delgado/workflow/commit/3d4e9645aa89547df558346b500263d61d510feb))
* **release:** write a Homebrew formula for each release ([6b935cf](https://github.com/jacob-delgado/workflow/commit/6b935cf293f50b1c4357d1a9cea833ca53dec5d8))


### CI

* audit the Homebrew formula from every change's binaries ([68e28d0](https://github.com/jacob-delgado/workflow/commit/68e28d02bb7edd4314cba3d6ebcf486203678451))
* install only the headless shell the end-to-end runs drive ([583ad7a](https://github.com/jacob-delgado/workflow/commit/583ad7a44d05d061d1fed6012cdb5deef2de787a))
* reuse main's Go build cache in the Test job ([3d48e66](https://github.com/jacob-delgado/workflow/commit/3d48e6658606bd4c4beed625db5568ef06f632e9))
* run the script tests in a job of their own, beside Test ([7118d6d](https://github.com/jacob-delgado/workflow/commit/7118d6d384dac3e00644c469426c6cd0076e69c6))
* write the Homebrew formula from every change's binaries ([075f40d](https://github.com/jacob-delgado/workflow/commit/075f40d642c8f74785a9b08909d9046190f54eab))


### Tests

* **cli:** keep the tests of status across directories apart ([6f4a41d](https://github.com/jacob-delgado/workflow/commit/6f4a41d4b6195c64e87fcc6eee72cc56333315ee))
* **e2e:** run the end-to-end specs on every core of a CI runner ([e830d1c](https://github.com/jacob-delgado/workflow/commit/e830d1c41934ae671291b4d6d4291fb9b4a606a5))
* **webserver:** hold that a seed not written stops the stream ([960911f](https://github.com/jacob-delgado/workflow/commit/960911f2489c2d102529fb78c44ce6c4b45884ab))

## [0.9.1](https://github.com/jacob-delgado/workflow/compare/v0.9.0...v0.9.1) (2026-10-13)


### Features

* **web:** draw secondary buttons in a tonal wash of the accent ([9593ca0](https://github.com/jacob-delgado/workflow/commit/9593ca0a406dc0b8a29e5b480abff89c3ef91677))
* **web:** give each filter chip the mark its rows draw ([6111719](https://github.com/jacob-delgado/workflow/commit/611171933e4f7c121245c2a7b5158aa276f9846a))
* **web:** mark the selected issue or task with an accent edge ([17706a7](https://github.com/jacob-delgado/workflow/commit/17706a7c789b4a3f5d97de6d5d7d08981d8e11a8))
* **web:** put notices on a ground of their state's color ([5865971](https://github.com/jacob-delgado/workflow/commit/58659718b223f3ff7b3ed92066acc00cdb2067d6))
* **web:** set the issue and task lists apart from their view ([ab5f6c0](https://github.com/jacob-delgado/workflow/commit/ab5f6c041217be3d2ad25b63f73945a8ae9b67b4))
* **web:** shade each Summary calendar day by how much was done ([94da4d2](https://github.com/jacob-delgado/workflow/commit/94da4d24448930df45eddc9a45b4aa1e92bf1990))
* **web:** soften both themes to a dimmed dark and a soft light ([02c0830](https://github.com/jacob-delgado/workflow/commit/02c083087251bfbd6daef5c08d856021924f393d))
* **web:** theme native checkboxes and the text selection ([14bb0c0](https://github.com/jacob-delgado/workflow/commit/14bb0c01f54065e52fd5b7ffb60154056b3ccd2f))


### Bug Fixes

* **web:** draw a chip's mark only where its rows all draw that mark ([def6cea](https://github.com/jacob-delgado/workflow/commit/def6cea1483ed2487b140bb215c50e3a89081849))
* **web:** draw every failure and result line on its notice ground ([e29854b](https://github.com/jacob-delgado/workflow/commit/e29854badb8ed165b83da442498eee7d708f88a0))
* **web:** hedge a partly read calendar month and dot each day's step ([5334a33](https://github.com/jacob-delgado/workflow/commit/5334a33a8b06aa5c2b4d4c4f948759d06ff659ed))
* **web:** keep selected text, pressed chips and off buttons legible ([d9193b0](https://github.com/jacob-delgado/workflow/commit/d9193b04b3be97c4effffe4b939860ab6445e831))
* **web:** read the calendar's month apart from the period's read ([eea488e](https://github.com/jacob-delgado/workflow/commit/eea488ecf654dcdce0d3815e08ee794f742eb38b))


### Refactors

* **web:** drop the color tokens nothing uses ([bbde5eb](https://github.com/jacob-delgado/workflow/commit/bbde5ebf97c4d4314e6666200f786f6d454d290c))


### Documentation

* **ux:** point at where index.css now declares the hues and scales ([562758b](https://github.com/jacob-delgado/workflow/commit/562758bb2685b2ae3d4441aaf1e29e2e65bd2f7d))

## [0.9.0](https://github.com/jacob-delgado/workflow/compare/v0.8.0...v0.9.0) (2026-10-13)


### ⚠ BREAKING CHANGES

* **tui:** on the Repositories pane, enter on a directory no longer switches to it; press s (the switch-directory action in ui.keys) and confirm the last look with enter.
* **setup:** on macOS, config init in a repository now asks whether to keep the Jira token in the keychain, as config init --global does; a script that answers its prompts answers that question too. The setup API no longer refuses the keychain for the repository's file, and its offer marks the keychain for every place where one is wired.
* **config:** config init --global and the first-run forms no longer write a jira.token_command reading the workflow-jira item; they keep the token in the item "workflow-jira <jira.base_url>" and write "keychain": true. A file an earlier config init wrote keeps reading the old item through its token_command; config init --global --force keeps the token for its address and writes the new setting. The settings API's JiraConfig gains keychain.

### Features

* **api:** describe each review request's facets on the server ([37fc99f](https://github.com/jacob-delgado/workflow/commit/37fc99f7d3d85a157d7b08811b202d9723d8d626))
* **api:** name the forge's kind in health, not only its words ([66409fa](https://github.com/jacob-delgado/workflow/commit/66409faeea5c898f81b7a315eb5277ddd2994b3d))
* **api:** rank and describe each task on the server ([f9af69f](https://github.com/jacob-delgado/workflow/commit/f9af69f4a570957dbe49a5bdacf5b7648133c447))
* **api:** ship the loop's stages in every frame of the stream ([91e142b](https://github.com/jacob-delgado/workflow/commit/91e142b2865d839d1e1cb57072f24869beb6a737))
* **api:** ship the stages of an issue whose branch is not checked out ([92d62d9](https://github.com/jacob-delgado/workflow/commit/92d62d9b8b44fc6685c58b755706ee37a85a98c4))
* **cli:** wait for the writes in flight when the last interface quits ([a548461](https://github.com/jacob-delgado/workflow/commit/a54846137cd35cf14a56f783be7c9ff7e0ae2bfa))
* **config:** keep each Jira address's token in its own keychain item ([352bb0c](https://github.com/jacob-delgado/workflow/commit/352bb0c0114fe4c6127fe80f67f6417e645763a2))
* **settings:** keep a Jira token typed in Settings in its keychain item ([47c5d9c](https://github.com/jacob-delgado/workflow/commit/47c5d9cd3f1cf934356ab1dce09df0880b857a2e))
* **setup:** offer the keychain for a repository's file in a first run ([82d997e](https://github.com/jacob-delgado/workflow/commit/82d997e246b945cf62def9ef14dd96baa383574d))
* **tui:** switch directories on s, no longer on enter ([b45b791](https://github.com/jacob-delgado/workflow/commit/b45b791b4709dbcdad1f355df454f80078438f75))


### Bug Fixes

* **api:** rank the active task among the list that holds it ([b87007d](https://github.com/jacob-delgado/workflow/commit/b87007d64aa6f084b29471156104dfc14a7c2c47))
* **api:** read the clock once for a task list and its filter ([5c8a772](https://github.com/jacob-delgado/workflow/commit/5c8a772b1f0ca17ad79770efd5f9bec7be6e2d12))
* **cli:** begin no wait for writes a signal ended with the interface ([bab6159](https://github.com/jacob-delgado/workflow/commit/bab6159e3d13efecc8bfa21918651192dcc527ac))
* **cli:** exit 3 for every refused ui.keys map ([187a8a9](https://github.com/jacob-delgado/workflow/commit/187a8a96a5a1bd0949a639785c6eeb55503c2264))
* **cli:** have doctor read its working directory once ([c5f1e62](https://github.com/jacob-delgado/workflow/commit/c5f1e62c7390cbe61d11f876862e1b2c6498511a))
* **cli:** leave a path Windows reads from elsewhere as it is given ([254aa78](https://github.com/jacob-delgado/workflow/commit/254aa7849301c6ddd1b310978fc238bf5a057301))
* **cli:** let a terminal write outlast the exit after a quit ([c2b79c4](https://github.com/jacob-delgado/workflow/commit/c2b79c406bc2f22c1100762ed8bfc115b7ca5f92))
* **cli:** name in announce's tags line only whom the post tags ([867bd76](https://github.com/jacob-delgado/workflow/commit/867bd76c04f174349de1037d8d58b6139bfceb6c))
* **cli:** read the directory a command was run from once a run ([19a643f](https://github.com/jacob-delgado/workflow/commit/19a643f89bedb9440866473ae94de5743720ab25))
* **cli:** read the tagging grant with the workspace, not drop its error ([28593ed](https://github.com/jacob-delgado/workflow/commit/28593ed6a1a8a3732bb036f5892d58c48755d987))
* **cli:** say a command has no environment rather than panic ([ddb8013](https://github.com/jacob-delgado/workflow/commit/ddb80134f4143ad8adb880e36bb64174b202f871))
* **cli:** settle the writes once on every quit, even with none counted ([6523c3a](https://github.com/jacob-delgado/workflow/commit/6523c3a988644b133ced61ae385c55c97afa1a65))
* **config:** refuse a gone home file's credential in the repo file ([515973b](https://github.com/jacob-delgado/workflow/commit/515973ba258b396f330d671d04fcac6629432af5))
* **config:** refuse a typed Slack secret before the keychain is touched ([1bda579](https://github.com/jacob-delgado/workflow/commit/1bda57974aa30216c012112f0eea229688c05ac0))
* **config:** touch no keychain or Slack for a save the file refuses ([f15f78a](https://github.com/jacob-delgado/workflow/commit/f15f78ad3889567a726ea6ae7e3fe4e66f8b91fc))
* **directory:** finish a shared Slack read for those still waiting ([02f5693](https://github.com/jacob-delgado/workflow/commit/02f5693683a127bb502abaad219d750d9037cf5f))
* end detached Slack reads with the server or the command ([a355b54](https://github.com/jacob-delgado/workflow/commit/a355b549b83f04c0b950d17998bdfab2102baddd))
* **frame:** draw the light border for a style outside the four ([b97cacd](https://github.com/jacob-delgado/workflow/commit/b97cacd827a0d4c1c15d50e2bd021194ed3958f6))
* **gitrepo:** say a read was stopped rather than outside a repository ([d0e4794](https://github.com/jacob-delgado/workflow/commit/d0e47941783eba0b958cf80bf45e207bc332c887))
* **keychain:** bound every security run the item makes ([e4bfeef](https://github.com/jacob-delgado/workflow/commit/e4bfeef2108b99ab4d04e470781f9a85ccc5b330))
* **keychain:** report a security run stopped at its bound as timed out ([98aa6ab](https://github.com/jacob-delgado/workflow/commit/98aa6abd09d2a53d0795ffe9f591f26bb9b8c4d5))
* **loop:** offer no repository group whose ID is not a group's ([3f8a629](https://github.com/jacob-delgado/workflow/commit/3f8a629b98ec41d7999182db65dfda96a1152c89))
* **loop:** read whether work was announced by one rule everywhere ([ae7a3ed](https://github.com/jacob-delgado/workflow/commit/ae7a3edb2786bfdcda9053d1ec4302320f3f5072))
* **loop:** tag the checked groups in the order the tags offer them ([b4414bf](https://github.com/jacob-delgado/workflow/commit/b4414bf0173e18cc84cf56084a17cad927eade06))
* **loop:** word the not-remembered error as the one sentence ([42aed01](https://github.com/jacob-delgado/workflow/commit/42aed01a0504b33f70a1a373ebd6c78f584c8ae1))
* **messaging:** mask a webhook that does not parse in a refusal ([bb1a5b4](https://github.com/jacob-delgado/workflow/commit/bb1a5b4be06371a8120b8d89ecd32f34f0996145))
* **progress:** count an announcement only for a pull request followed ([45f9e0e](https://github.com/jacob-delgado/workflow/commit/45f9e0ee58cb16f7560712402d38f58da99b868f))
* **report:** word a read outside a repository for every surface ([82fb141](https://github.com/jacob-delgado/workflow/commit/82fb14119dc2af18b596af3052dc55bcf9faee33))
* **scripts:** fail the condition gate when its floors file is not there ([ad03d34](https://github.com/jacob-delgado/workflow/commit/ad03d34e97caff44d702f45f3c26a2bdbf800010))
* **scripts:** hold no floor to a share of no conditions ([2b26ed3](https://github.com/jacob-delgado/workflow/commit/2b26ed3902b3de623df054cc931618bbccfc2e3a))
* **scripts:** name a method of a type with several type parameters ([b946976](https://github.com/jacob-delgado/workflow/commit/b9469769ec9328252dd6811f5bba9eae5f25105e))
* **seams:** ask Taskwarrior under the context of whatever asked ([7b28ab4](https://github.com/jacob-delgado/workflow/commit/7b28ab47175bff0fa6ce74b099df723fcce1f1c5))
* **seams:** ask the forge under the context of whatever asked ([1f06fe5](https://github.com/jacob-delgado/workflow/commit/1f06fe50987d15a604aabbddf82b1e33bedacb67))
* **seams:** ask the messaging service under the asker's context ([84250f6](https://github.com/jacob-delgado/workflow/commit/84250f6da9bac70916cf8cb18de668d806f3a23c))
* **seams:** ask the tracker under the context of whatever asked ([fa95fc6](https://github.com/jacob-delgado/workflow/commit/fa95fc6013963e8d68d4322488ceafc77b7ff964))
* **seams:** run git under the context of whatever asked ([29ac051](https://github.com/jacob-delgado/workflow/commit/29ac0513fdac907ebf43e70470311882b1965517))
* **seams:** run the remaining seams under the context that asked ([0aec673](https://github.com/jacob-delgado/workflow/commit/0aec673e469e2a4335394a127adf61e407c52319))
* **seams:** use the store under the asker's context, recording detached ([b7ee304](https://github.com/jacob-delgado/workflow/commit/b7ee304f56da24c19293574b0a50558443de29db))
* **setup:** keep a first run's token under its caller's context ([f196d0d](https://github.com/jacob-delgado/workflow/commit/f196d0d4f9bc13c6b9bb7f2fbf569eec907d89f9))
* **slackauth:** keep a rotated Slack token though its asker leaves ([2ce9acd](https://github.com/jacob-delgado/workflow/commit/2ce9acd35304fbbe2704e5dd63d4d0f24ced3aba))
* **store:** make and stamp the file whatever the caller's context ([0c37eda](https://github.com/jacob-delgado/workflow/commit/0c37eda88e9fa679836c6d2bad24c606763f9f45))
* **store:** mark again a favorite whose time does not read back ([c1eb283](https://github.com/jacob-delgado/workflow/commit/c1eb283a6e470b183f41fb51dedd5a8e1f29a955))
* **tui:** build each pane's key context from what its handler answers ([1ecca04](https://github.com/jacob-delgado/workflow/commit/1ecca0464a36e13ea973310094662d645badade4))
* **tui:** count only web-bound actions in a shared web section ([4342f79](https://github.com/jacob-delgado/workflow/commit/4342f7963fe164aaa3af3c7718c154434054323b))
* **tui:** end a terminal write when its program does ([6649c1b](https://github.com/jacob-delgado/workflow/commit/6649c1b8637a1fd6cb7b352d970e62a733ae2b92))
* **tui:** give a focused ASCII rail pane the corners of a focused box ([0d40c3e](https://github.com/jacob-delgado/workflow/commit/0d40c3e514172d1e9a67c8f536974f9befc05c55))
* **tui:** give each overlay a key context of its own ([a603e36](https://github.com/jacob-delgado/workflow/commit/a603e36b229e7ea955e4faa5ca0a5e278838e18b))
* **tui:** hand a pane every key, so a missed key context fails a test ([db12f25](https://github.com/jacob-delgado/workflow/commit/db12f251f2732d8a91ba3931c739bd7ecfdc176c))
* **tui:** handle no signal in the interface, so SIGTERM ends it ([9bbc154](https://github.com/jacob-delgado/workflow/commit/9bbc1544b645748af296ca24767fbd62a62b9dbb))
* **tui:** let a write outlast the session that sent it ([1af80b4](https://github.com/jacob-delgado/workflow/commit/1af80b4805ed3e8da9d59fb8865321a5fdbc38a7))
* **tui:** list each pane's keys under its own heading ([e9173eb](https://github.com/jacob-delgado/workflow/commit/e9173ebd7e9100cb9d60b19bd4722270d7d7cd7c))
* **tui:** name the keychain in the token's hint as it is turned on ([8a9420d](https://github.com/jacob-delgado/workflow/commit/8a9420dc202057644a7b99cfea95578944bdf11f))
* **tui:** offer no start or done on a task just marked done ([8860c98](https://github.com/jacob-delgado/workflow/commit/8860c9858ad3a0a0823a21aa7089cdbc615cb6ff))
* **tui:** offer nothing on a task just marked done until read again ([a36d53c](https://github.com/jacob-delgado/workflow/commit/a36d53c421bc8fd04c8971a062c4a79975ca6337))
* **tui:** offer the other task tracking an issue whose task is done ([1e8ffb7](https://github.com/jacob-delgado/workflow/commit/1e8ffb7125f10a867948104a25044c93aaf56040))
* **tui:** page a long job log with pgup and pgdn ([9330838](https://github.com/jacob-delgado/workflow/commit/9330838383aeeeadeefe447fd9640eae3361e60b))
* **tui:** post the tags the preview names, in the order it names them ([7676388](https://github.com/jacob-delgado/workflow/commit/7676388bf98805435f47dbc384b194796f2c7b1d))
* **tui:** read a run's output to its end once the interface leaves it ([4578410](https://github.com/jacob-delgado/workflow/commit/4578410294892aaac9a5255ad41ccd8afa38bd0c))
* **tui:** read keys from the terminal when stdin is not one ([bb75f35](https://github.com/jacob-delgado/workflow/commit/bb75f356d4beec117714b3eca180e8f32e39901d))
* **tui:** refuse a key two panes the web shows together share ([de343b8](https://github.com/jacob-delgado/workflow/commit/de343b8949db5e4d3183e4da2e3ab372f30b335d))
* **tui:** refuse an action that asks first on apply's key as such ([2044934](https://github.com/jacob-delgado/workflow/commit/20449343c868d31ca6aa1ae8018498b6eab1f3a6))
* **tui:** refuse an action that asks first on the key that confirms it ([d26ca6b](https://github.com/jacob-delgado/workflow/commit/d26ca6b576d1a08d8b84eded0a732b2c8ae9d27e))
* **tui:** refuse an overlay's action on a text field's cursor keys ([bffb90e](https://github.com/jacob-delgado/workflow/commit/bffb90e0c2b2b6bfbb0b3c929b00817d547d7ed0))
* **tui:** say where the keys the help listed under Everywhere work ([52e83b8](https://github.com/jacob-delgado/workflow/commit/52e83b8a2ecb7f653d26282a69f349409a9bc46e))
* **tui:** say why a removal falls short as a refused save says it ([64acd90](https://github.com/jacob-delgado/workflow/commit/64acd90294c33fa118eba6dbc9a6f0ba782c618e))
* **tui:** wait for no page opening in the browser on a quit ([74ec480](https://github.com/jacob-delgado/workflow/commit/74ec4800140c34bfd177d8aab26a80e9b1b55145))
* **web:** draw each stage in the state the server read it ([c2fbe5c](https://github.com/jacob-delgado/workflow/commit/c2fbe5c400d33d5ea9e0b54f79bc7fee8f9f4c79))
* **web:** have the mockup's done answer the task as it stands done ([347711b](https://github.com/jacob-delgado/workflow/commit/347711b8d63ed3e23505e6d30f2344a2e02ec7b9))
* **web:** hold a done task's writes until a list answers after it ([385592e](https://github.com/jacob-delgado/workflow/commit/385592e662495b941c22df74c7a27f7d94e63c03))
* **web:** offer nothing to do on a task just marked done ([e98f271](https://github.com/jacob-delgado/workflow/commit/e98f271751ece225cbb1a6b308d99c22bc4f6a8b))
* **web:** say a done task leaves the Tasks list once workflow reads it ([b617598](https://github.com/jacob-delgado/workflow/commit/b6175981c090ebc0fadede2406d01634c7e187ad))
* **web:** say a done the card could not describe shows done later ([73ac85e](https://github.com/jacob-delgado/workflow/commit/73ac85ebf9ef4948e03a046abadb57473aad7095))
* **web:** say a review request's facts in the words its filter uses ([781a547](https://github.com/jacob-delgado/workflow/commit/781a547eeed1f89e05794be8d02bb0e92eb812e1))
* **webserver:** answer a request whose context ended without alarm ([d8e99d5](https://github.com/jacob-delgado/workflow/commit/d8e99d540414d6e4b250327ddefab4c55e6d7557))
* **webserver:** answer a run's request that does not decode, not drop it ([ee4cdcb](https://github.com/jacob-delgado/workflow/commit/ee4cdcbfda7e56aaf94bcb7130c5950d149ef771))
* **webserver:** end a shared read when the server stops ([79f7a45](https://github.com/jacob-delgado/workflow/commit/79f7a45cc2d80dc8ffbec2bf9ca7623029139ebd))
* **webserver:** end each request in flight when the server stops ([6baeca6](https://github.com/jacob-delgado/workflow/commit/6baeca63d5c08db2c6c26978c557db3010ce7207))
* **webserver:** keep every canceled failure from Unexpected ([29e2fd2](https://github.com/jacob-delgado/workflow/commit/29e2fd24060dfe5ad78e2c300d21c9bfd2f61924))
* **webserver:** return from Serve only once the drain is done ([b398f75](https://github.com/jacob-delgado/workflow/commit/b398f75f08b0f96094712a6f9ebb4fb45460dcc6))
* **web:** show a task marked done as the server describes it ([1877c5a](https://github.com/jacob-delgado/workflow/commit/1877c5a027b9a1ff2c0d36dc50592ee2c5185b04))
* **web:** take a test's body from after its name in the marker rule ([b8e7b54](https://github.com/jacob-delgado/workflow/commit/b8e7b540bae6cecd7e56146ba9d1070c16c9e352))
* **web:** title the work story's stages as the terminal names them ([1913836](https://github.com/jacob-delgado/workflow/commit/19138361089fff3d617e7c7057c4ab296122635a))
* **web:** word a task's state as the server reads it ([e8f6191](https://github.com/jacob-delgado/workflow/commit/e8f6191b88610b845e557bc84ab191b22e51a2bf))
* **web:** word the Issue stage by the state the server sent ([502d9d9](https://github.com/jacob-delgado/workflow/commit/502d9d96978bff6d968641d1d2cb701f4a493b23))
* **wiring:** ask which forge it is at each group lookup ([9c46c67](https://github.com/jacob-delgado/workflow/commit/9c46c676c9019c2929b166c4371db9f8d5e44b85))
* **wiring:** bound a request gh or glab carries by the request timeout ([23d0cdd](https://github.com/jacob-delgado/workflow/commit/23d0cdd6f25837586f6f70b31a36618869dd1552))
* **wiring:** keep the forge row count when a first page's page left ([7c1dcc3](https://github.com/jacob-delgado/workflow/commit/7c1dcc3c6a1d5f9fc8ebd2bc675778cf60782e64))
* **wiring:** send a forge CLI request under its own context ([01e3ca2](https://github.com/jacob-delgado/workflow/commit/01e3ca245c6661ba3db397c536a74ddb9462acde))


### Performance

* **webserver:** parse the embedded API contract once a process ([d75ab13](https://github.com/jacob-delgado/workflow/commit/d75ab1373469733693cd0d4be97afcbb55c2fc30))


### Refactors

* **buildinfo:** compile the pseudo-version pattern once ([41c86d6](https://github.com/jacob-delgado/workflow/commit/41c86d6619c88127627406b853561495322c5418))
* carry the server's lifetime and the terminal's session ([294615e](https://github.com/jacob-delgado/workflow/commit/294615ec66f76d328dc8db5418e1a40eb1b652ec))
* **cli:** drop doctor's Jira address check loading already makes ([9874aa9](https://github.com/jacob-delgado/workflow/commit/9874aa9efb2ccc472163f2d685f9b74508d9d323))
* **cli:** name a never-run Taskwarrior without a dead check ([928a78d](https://github.com/jacob-delgado/workflow/commit/928a78d9a801d73d5e8f861b12e215ff5452c36e))
* **cli:** read the branch announce composes and tags from once ([d6fa13e](https://github.com/jacob-delgado/workflow/commit/d6fa13e65f5359dbd473c85a3dc3a1a326a83f45))
* **cli:** read the tagging grant and base as the proposal's reads ([f44d86d](https://github.com/jacob-delgado/workflow/commit/f44d86dff6ac086955462ccb5b7758974c3c174d))
* **cli:** run the unmovable switch through runRootUnmoving ([14b3f33](https://github.com/jacob-delgado/workflow/commit/14b3f331514ff8387731c27a3251e2119c8f5ae3))
* **cli:** tag an announcement's proposal through Tags.Proposed ([2d59b57](https://github.com/jacob-delgado/workflow/commit/2d59b57fba1e911ac974a3ded9d5166f2c348274))
* **codeowners:** compile GitLab's section header once ([316b15a](https://github.com/jacob-delgado/workflow/commit/316b15a32b91e78e9e3ba7745c92c08664a8e692))
* **config:** name the repository file's refusal for what it checks ([f0f658d](https://github.com/jacob-delgado/workflow/commit/f0f658d9fb96a2530482cd0d629f6f9c7cb0d3f3))
* **config:** pass on no encoding failure that cannot happen ([2ca2585](https://github.com/jacob-delgado/workflow/commit/2ca25852be44c2263f29b6f9a044dd2038075264))
* **convention:** compile fixed regular expressions once ([d13c414](https://github.com/jacob-delgado/workflow/commit/d13c4146697ef05699221d8296c6a642ad975dca))
* detach a context until an end through lifetime.Until ([3eda062](https://github.com/jacob-delgado/workflow/commit/3eda062861e05c8831c5315e8ae801b05b8aa858))
* drop the group members seam nothing calls ([cdbfe4c](https://github.com/jacob-delgado/workflow/commit/cdbfe4c1f71e7293e0883d5f5df7b67fd1bdd4ec))
* **filelock:** report a lock another holds as ErrHeld ([7ffca95](https://github.com/jacob-delgado/workflow/commit/7ffca953024434e6f490e6916db31af3a03ba018))
* **fileowner:** declare Owner beside each platform's Of ([0fcb663](https://github.com/jacob-delgado/workflow/commit/0fcb6630754a5f7db318aa0919afe097df133a9a))
* **forge:** move the review queue out of pulls.go ([ae54597](https://github.com/jacob-delgado/workflow/commit/ae54597a9ba2c36f914c3a67ac6ee536b58ba992))
* **forge:** parse a log's redirect once, in the request itself ([3a616f1](https://github.com/jacob-delgado/workflow/commit/3a616f1aa299905ab5e4ed2ff5983596f942d75a))
* **gittest:** take a testing.TB in Run and FilesUnder ([52101af](https://github.com/jacob-delgado/workflow/commit/52101afccd9e3da5691adb968305824605b99a43))
* **gittest:** unset every local variable, then report what failed ([1199071](https://github.com/jacob-delgado/workflow/commit/11990712892c92d4da3601bb1a2faecf7588fdb2))
* hand every command the environment it runs in ([84a92b7](https://github.com/jacob-delgado/workflow/commit/84a92b78ee551689178f4648869b1e755ce28c50))
* hand the web server the interface's seam groups whole ([9bf7ef2](https://github.com/jacob-delgado/workflow/commit/9bf7ef2d0697d4b0182635442b81adcc5feef91c))
* **hooks:** compile fixed regular expressions once ([c53a3b1](https://github.com/jacob-delgado/workflow/commit/c53a3b12aa4169761a8fe171c0396e6bb4a674bd))
* **hooks:** name the ESLint header's builder for its platform ([9166499](https://github.com/jacob-delgado/workflow/commit/9166499c07105c1cda73172d63bc5c0703b1144f))
* **httpx:** send a client through a transport its caller hands it ([d0b9612](https://github.com/jacob-delgado/workflow/commit/d0b9612b0b799daf98e57df9f69d0e1b43b43c73))
* **jira:** compile fixed regular expressions once ([250a06e](https://github.com/jacob-delgado/workflow/commit/250a06ef2aa8edd25fc35bafa697d29c35d1bcd6))
* **loop:** answer why a post was not remembered from the loop ([4a135c1](https://github.com/jacob-delgado/workflow/commit/4a135c1ed6ef89b393fe4d9b6985e9c088b1f3b8))
* name the web page's sections as seams.PageSection ([f666438](https://github.com/jacob-delgado/workflow/commit/f6664386f8716c4d349374991a1d51fc4e3382ff))
* **pgroup:** pass a failed group kill on as the system call's error ([4791db6](https://github.com/jacob-delgado/workflow/commit/4791db6b7c29744317c72d52fb81936122bfd507))
* **proc:** run a program a caller found on a PATH of its own ([dc29f4b](https://github.com/jacob-delgado/workflow/commit/dc29f4b077abfc006820452274ddb3b534c8a3ba))
* report what every surface shares from internal/report ([3ab1847](https://github.com/jacob-delgado/workflow/commit/3ab184784d0976b74e79d3a1259dde8f7fbf9ae9))
* **scripts:** move the condition gate's arm check to its own file ([89d1cf1](https://github.com/jacob-delgado/workflow/commit/89d1cf13a059a1aeeb00919333374e0e9bb60a6d))
* **store:** look at each of a database's files once when listing ([c9cb988](https://github.com/jacob-delgado/workflow/commit/c9cb988fd59b3e5f59e1780da972ad88692ea795))
* **store:** settle the kept version in one place ([df906e7](https://github.com/jacob-delgado/workflow/commit/df906e76df5bd7f370e2d9a48471ad0f10d2689f))
* **taskwarrior:** compile fixed regular expressions once ([e78d608](https://github.com/jacob-delgado/workflow/commit/e78d608ae89638a37808ca679aea4dde95a29aa1))
* **tui:** answer a list overlay's esc and cursor keys in one place ([93390b2](https://github.com/jacob-delgado/workflow/commit/93390b29815212081b87539c8c36e9aa042a9ded))
* **tui:** ask each pane's state what it can do and draw the spine ([8e9e7f0](https://github.com/jacob-delgado/workflow/commit/8e9e7f0fd1d661cc2442469c265082a4ea802ea9))
* **tui:** ask the loop once whether a post was remembered ([845e66f](https://github.com/jacob-delgado/workflow/commit/845e66f6756437c0bfd1fab8f52b4c25597b1d8b))
* **tui:** assert at compile time that Local data is an overlay ([2386dba](https://github.com/jacob-delgado/workflow/commit/2386dba183a5daa989a7de3d3071344926398930))
* **tui:** assert each applier beside its definition ([ac1f0e9](https://github.com/jacob-delgado/workflow/commit/ac1f0e91098b2497a9283edc59e06ca82d4fe66e))
* **tui:** build the web-bound actions once a shared section ([0c8b214](https://github.com/jacob-delgado/workflow/commit/0c8b214cd7e9f347d29660c85dfe8caf2ca2c3bd))
* **tui:** compare a setting's kind only to the kinds that reach ([eb97de7](https://github.com/jacob-delgado/workflow/commit/eb97de7f6069607bad221755fdf79287a022a05a))
* **tui:** compare the first run's step only to the steps that reach ([a92d135](https://github.com/jacob-delgado/workflow/commit/a92d135424fde442b59384a38068e51b4e20b124))
* **tui:** define each pane's behavior beside the pane ([89b8737](https://github.com/jacob-delgado/workflow/commit/89b8737d2809f86bb53d1bbab6773a7e8f84a9c8))
* **tui:** draw failures through the render kit ([0d9b5fd](https://github.com/jacob-delgado/workflow/commit/0d9b5fdff3c2168f933d2bf6766adc439a6694f2))
* **tui:** draw the Repositories pane from a file of its own ([71cc65e](https://github.com/jacob-delgado/workflow/commit/71cc65eb7ff994ac1a7f2ea9ab0eb3af31deea2a))
* **tui:** drop the status picker's paste check for a move out ([99710a3](https://github.com/jacob-delgado/workflow/commit/99710a3fe1750e7d88970b09d0a2ba06cdfd20b5))
* **tui:** hand onFieldNav the ring of fields a composer stands in ([340d0d9](https://github.com/jacob-delgado/workflow/commit/340d0d97f714eea24bd563a646e9e5cd5dbe9016))
* **tui:** hand overlays a render kit instead of copying it in ([2c95f34](https://github.com/jacob-delgado/workflow/commit/2c95f3464c35a800aeec9b9616c512b8b4fbbbc3))
* **tui:** hand pollInterval the UI settings, not a bare flag ([9e2026b](https://github.com/jacob-delgado/workflow/commit/9e2026b7645a60306b77609918d9565d3f8195a4))
* **tui:** hold the edited settings to a file's standard in edited ([991b113](https://github.com/jacob-delgado/workflow/commit/991b1132ae4f15a253f9bc631a0baa94cdf314b6))
* **tui:** keep the shared path helpers with the text helpers ([17a9769](https://github.com/jacob-delgado/workflow/commit/17a97692fa04382439f8df3112cc83f4716f105d))
* **tui:** leave Model to route and compose the panes ([565ce3d](https://github.com/jacob-delgado/workflow/commit/565ce3d911c12b4bc1d83239f7560714de8fd28d))
* **tui:** list the scroll keys the help and a job's log answer ([5ef78e6](https://github.com/jacob-delgado/workflow/commit/5ef78e6e7ee1adf96598b9e56913291c2125e22d))
* **tui:** move focus around one ring; navigate composers alike ([1751bb0](https://github.com/jacob-delgado/workflow/commit/1751bb053208532094c8324c394eb6a7d2bf2ae7))
* **tui:** move the panes' key bindings into a file of their own ([4f37e9f](https://github.com/jacob-delgado/workflow/commit/4f37e9fe8aeb9a0849c489328277ba75aa825c64))
* **tui:** name each pane's help group from the pane ([a58b1d5](https://github.com/jacob-delgado/workflow/commit/a58b1d5ce6791a5d0bd81c3629a65ced8866b35f))
* **tui:** name the key context each overlay is checked in ([5eeffac](https://github.com/jacob-delgado/workflow/commit/5eeffac8ad724f7b62e714b94c529795026acc5e))
* **tui:** name the selected issue's URL apart from browseURL ([17cadf6](https://github.com/jacob-delgado/workflow/commit/17cadf6b4e297c06b9e58fbd6af65644db3f177a))
* **tui:** pass layout a Terminal and a Rail, not four ints ([d702185](https://github.com/jacob-delgado/workflow/commit/d7021854492018b34c1a3087d96f186c7181fe43))
* **tui:** pick the wheel's direction with two tests, not a switch ([3f38037](https://github.com/jacob-delgado/workflow/commit/3f3803792dc354957aa0ad9e399d216059a42ddd))
* **tui:** rename the exported key context ListedKeyContext ([4be1da8](https://github.com/jacob-delgado/workflow/commit/4be1da8dc82c12fc14d27255fefb71daf5654535))
* **tui:** render Repositories, Branch and Commits from their state ([38c5353](https://github.com/jacob-delgado/workflow/commit/38c53535e2eb0b4a6c4c5541119667d47bfd4da4))
* **tui:** render Tasks, Reviews and Summary from their own state ([3e555c5](https://github.com/jacob-delgado/workflow/commit/3e555c56ec144892fe2bdbaaf6e469046e65b605))
* **tui:** split the long files by concern ([ecafc09](https://github.com/jacob-delgado/workflow/commit/ecafc09542bbec574ba1f36c045098b6747c6f8c))
* **tui:** step the channel and setting cycles round a ring ([e008535](https://github.com/jacob-delgado/workflow/commit/e00853580c2d6de2a1975db11e61b16afb3ccb6d))
* **tui:** take the interface's input from the caller ([b6d4ebf](https://github.com/jacob-delgado/workflow/commit/b6d4ebfaf2dbed703e575356dc0bf870461c74d8))
* **tui:** take work that needs no interface state off Model ([a08d0f0](https://github.com/jacob-delgado/workflow/commit/a08d0f0035e0844d96a55e07af8ae08ba7e6320d))
* **web:** keep the mockup's task routes with its task data ([585c7fc](https://github.com/jacob-delgado/workflow/commit/585c7fc0e71a9e24138e7bf450656f5b48d49676))
* **web:** keep the test setup's stand-ins apart from the builders ([2bdfc25](https://github.com/jacob-delgado/workflow/commit/2bdfc25d184c2ff345948528f62cfd0cc8df9f71))
* **web:** read which section shows each pane from the server ([6681320](https://github.com/jacob-delgado/workflow/commit/6681320f3020ff87f6141681486379d08df732ad))
* **webserver:** ask once whether a task's branch is checked out ([ad77c93](https://github.com/jacob-delgado/workflow/commit/ad77c93962cbde972ef62fcd671915b3672408b1))
* **webserver:** compile fixed regular expressions once ([62cacd4](https://github.com/jacob-delgado/workflow/commit/62cacd445c1bb2b3432889ba1e34d217aed182ba))
* **webserver:** give what was announced a file of its own ([6428b99](https://github.com/jacob-delgado/workflow/commit/6428b9975688a0784b8cffd0bcd459bf0ba79e54))
* **webserver:** name the held announcement's verdict apart ([65fb59b](https://github.com/jacob-delgado/workflow/commit/65fb59b7245f0d8744df2cc22774c87de6d8c786))
* **webserver:** read a branch's stages by field, not by a flag ([bbf2619](https://github.com/jacob-delgado/workflow/commit/bbf261991328af045e17a72de7da8a8719b66513))
* **webserver:** read a run's request as the contract checked it ([efd2a7c](https://github.com/jacob-delgado/workflow/commit/efd2a7cad3ad22a058c088b292c301da70f161d2))
* **webserver:** release every shared read's context by defer ([8641c1f](https://github.com/jacob-delgado/workflow/commit/8641c1f86f28b01bdcb8c2bd8c6b14494528ee69))
* **webserver:** route each API request once, in its own validator ([d565d7b](https://github.com/jacob-delgado/workflow/commit/d565d7b297f1752e92b0aa5d093562461d914f46))
* **web:** take no zero time as an unknown one in ago ([2eefdf3](https://github.com/jacob-delgado/workflow/commit/2eefdf3e97a570e41805e39626a74be80b55dd7b))
* **wiring:** keep GitLab's group answers by the name alone ([0883c4b](https://github.com/jacob-delgado/workflow/commit/0883c4bbf466caee898d3324cc9bf053001cf9d4))


### Documentation

* **api:** describe a stage by every list that carries it ([90933ae](https://github.com/jacob-delgado/workflow/commit/90933ae26c2700e3b825c609d58349a321c4fd4f))
* **api:** say which credential a repository's file refuses ([ceb2f74](https://github.com/jacob-delgado/workflow/commit/ceb2f74224d6c34be1b765bdb556551bb1c9524a))
* **architecture:** describe kept.db beside the cache ([12b53d9](https://github.com/jacob-delgado/workflow/commit/12b53d94c71a4be03161d2a6295f4884c192dc15))
* **architecture:** say --web takes the bundle's groups whole ([a7e440a](https://github.com/jacob-delgado/workflow/commit/a7e440ac3753e40f411a962e6f7c15fea09f52d6))
* **architecture:** say the web API asks for the run's session ([9754d66](https://github.com/jacob-delgado/workflow/commit/9754d663b1e5e5a04d562e87dc31a19d3c574c23))
* **architecture:** say what a dry run reads from each store file ([b959ed9](https://github.com/jacob-delgado/workflow/commit/b959ed9a8821761b9e5196d12eb70e6e029c3806))
* **architecture:** say what a favorite of the wrong shape costs ([5668c1a](https://github.com/jacob-delgado/workflow/commit/5668c1a20f9fa680b6713b5bb43013fea608356a))
* **ci:** say how gobco reads build-tagged twins in the PR comment ([fbd8af2](https://github.com/jacob-delgado/workflow/commit/fbd8af289251ae4830a7f3bcffb27b279b4e668e))
* **claude:** ask for the Boy Scout improvement on the branch ([ddf3093](https://github.com/jacob-delgado/workflow/commit/ddf3093be44261058225c57c25bfa6389b53fb88))
* **claude:** declare compiled regular expressions at package level ([02805de](https://github.com/jacob-delgado/workflow/commit/02805ded4d275dce927444cf686b3f2fa3cfcafb))
* **claude:** every I/O seam takes a context first ([45ecf80](https://github.com/jacob-delgado/workflow/commit/45ecf801e2aaa5874fb20a5a58139a65a4b55f15))
* **claude:** land a Boy Scout improvement as its own commit ([08ea4e2](https://github.com/jacob-delgado/workflow/commit/08ea4e2a85959bb210f4e858a2c9f69fc092a1f9))
* **claude:** mark the goroutine smell as gated ([2c60792](https://github.com/jacob-delgado/workflow/commit/2c60792a90d641e9342671708bb1bf652aa1d196))
* **claude:** name Taskwarrior, slackauth and kept favorites ([5f6ef9a](https://github.com/jacob-delgado/workflow/commit/5f6ef9ab27a098822f238071101a97cb2eff44f7))
* **claude:** say the layout block is checked ([20837b5](https://github.com/jacob-delgado/workflow/commit/20837b5bc449e6990c96d3bcbde82a3ea55ae18f))
* **claude:** say where a new class of web failure is written ([395f7a5](https://github.com/jacob-delgado/workflow/commit/395f7a5dc292132254c423a7e05bca4e5f74cfeb))
* **cli:** name in Execute's comment the detached work a quit waits for ([3c2897c](https://github.com/jacob-delgado/workflow/commit/3c2897c1a55c276226b43d8fee315fa90a7735e8))
* **cli:** name the work a signal leaves to finish in Execute's comment ([6565ee5](https://github.com/jacob-delgado/workflow/commit/6565ee549312affed6287fc0a69fcb5c57963bab))
* **config:** drop the earlier-build workspace promise ([c182934](https://github.com/jacob-delgado/workflow/commit/c1829343c25c7d566d822cfe12ae379b4607d2b5))
* **config:** say disabling the store drops what was decided too ([a7ee66b](https://github.com/jacob-delgado/workflow/commit/a7ee66b2427720e42be6b3f33083c528c56eb801))
* **config:** say only a non-empty NO_COLOR turns color off ([86d9c34](https://github.com/jacob-delgado/workflow/commit/86d9c34b6e92ce1b7c8110b05893c5438758c297))
* **config:** say the preview lists each code owner it proposes ([57daca4](https://github.com/jacob-delgado/workflow/commit/57daca47e252f9acfb021db04fbe726a6687554b))
* **config:** say what a disabled store leaves an announcement ([58df76a](https://github.com/jacob-delgado/workflow/commit/58df76ae81fcd984428ef8fda9c3b691dd20c944))
* **configuration:** leave apply out of the composer's action list ([0bdf55b](https://github.com/jacob-delgado/workflow/commit/0bdf55b538ad6217288b4270ca24df5d7472caea))
* **configuration:** say People and groups links an owner with apply ([adefb4c](https://github.com/jacob-delgado/workflow/commit/adefb4c1835965fcb4502cc157d654ac32be86d2))
* **configuration:** say the keychain gives security 30 seconds ([b045604](https://github.com/jacob-delgado/workflow/commit/b045604397418faf5bfc54a5333320fbd46c7cb2))
* **contributing:** name mise.toml among what lints the Markdown ([8c20bd6](https://github.com/jacob-delgado/workflow/commit/8c20bd61f1d8f97de573ad93c03075b99bb306f6))
* **contributing:** say a subject may be 72 characters ([89bbbdb](https://github.com/jacob-delgado/workflow/commit/89bbbdbbeb202dcf1338b98603c2be8066a47155))
* **debt:** bring TRADE-16 and TRADE-18 to what the lane's tests do ([09fc6fe](https://github.com/jacob-delgado/workflow/commit/09fc6fe5c193cd6003294898f7c6d94c103e463f))
* **debt:** bring TRADE-19's figures and site to the gate as it stands ([364176e](https://github.com/jacob-delgado/workflow/commit/364176eef6270620f8e8bf26dd7ff14b4d6c6cd1))
* **debt:** close DEBT-216, now health names the forge's kind ([5da8bd2](https://github.com/jacob-delgado/workflow/commit/5da8bd272aa3282da5eca0671ddbfb3dbacffe07))
* **debt:** close DEBT-239, now the gate fails an unseen arm ([b664191](https://github.com/jacob-delgado/workflow/commit/b664191064472797ab2c8410be62ac753dbbc720))
* **debt:** close DEBT-241 and TRADE-2, now no test file passes 700 ([42756ec](https://github.com/jacob-delgado/workflow/commit/42756ec9d5d018c8ef91dbdb68c6dfa21cebecdb))
* **debt:** close DEBT-243, now the timing tests wait on signals ([f96bdd6](https://github.com/jacob-delgado/workflow/commit/f96bdd609d6f4ab5ed171747e81b035c06d98c9e))
* **debt:** close DEBT-278, now the register cites what the code has ([9f20ded](https://github.com/jacob-delgado/workflow/commit/9f20ded303216593401277bd517fce09703eb156))
* **debt:** close the architecture seams' entries ([a36cda0](https://github.com/jacob-delgado/workflow/commit/a36cda000928618b8149a7392cce5a8183788361))
* **debt:** close the docs entries ([871bb06](https://github.com/jacob-delgado/workflow/commit/871bb066d7f07f9a3fe8c7d1ef1ea281d384d1b8))
* **debt:** close the regular-expression sweep's entry ([41cf41e](https://github.com/jacob-delgado/workflow/commit/41cf41efc8d8b1877c68129655b06dc379f7841b))
* **debt:** close the rules the web wrote a second time ([a6a8245](https://github.com/jacob-delgado/workflow/commit/a6a824597bc1ae1897ae74d4d7b9804cd5f20ccd))
* **debt:** close the terminal's structure entries ([7754d9f](https://github.com/jacob-delgado/workflow/commit/7754d9ff25d0376198a10755b02e05f347effbca))
* **debt:** close TRADE-7, now that gobco reads every package ([debf6a8](https://github.com/jacob-delgado/workflow/commit/debf6a8f60ec9817e7286fb78caac0412fd44a77))
* **debt:** count the editor's Windows default in TRADE-35 ([59b32a2](https://github.com/jacob-delgado/workflow/commit/59b32a21089990aba7a43efd81ef03ea3b2706c7))
* **debt:** count TRADE-13's comments and arms as they stand ([331e417](https://github.com/jacob-delgado/workflow/commit/331e417c37867fcf1c2704844948e5ebf1547f8c))
* **debt:** date each trade-off's scopes and name every detached read ([fc9fa70](https://github.com/jacob-delgado/workflow/commit/fc9fa7096cf62674662af69516c253db4063ea9a))
* **debt:** date TRADE-35's macOS scope by the change that added it ([5d0385b](https://github.com/jacob-delgado/workflow/commit/5d0385bdac1bbc06ab8ea537ac405143bdd01ced))
* **debt:** give TRADE-19 what is left of it, and its numbers ([11964d2](https://github.com/jacob-delgado/workflow/commit/11964d2e1bf6478090832600ab2fd8d3222380fa))
* **debt:** keep keysFrom's file arm under TRADE-18 with its site ([e548efa](https://github.com/jacob-delgado/workflow/commit/e548efa8ec7bc75f897ff46a1ea80028555ffef0))
* **debt:** keep only the open trade-offs ([c52bc48](https://github.com/jacob-delgado/workflow/commit/c52bc482506c5c130a260f5807c3e1e78e33fcf6))
* **debt:** keep TRADE-18's pull request off the start of a line ([7e91181](https://github.com/jacob-delgado/workflow/commit/7e91181a9dd3c8aedc1d5c47ac1c3c747a264ab3))
* **debt:** list the paydown's pull requests without each deleting one ([c854fbb](https://github.com/jacob-delgado/workflow/commit/c854fbb9a271e26635a87876f7877df4033bd539))
* **debt:** measure TRADE-19 again, naming no commit a merge drops ([df2d3ca](https://github.com/jacob-delgado/workflow/commit/df2d3ca3c224e1cd8aee6643e6fc41b9889f9897))
* **debt:** name in DEBT-278 only the register text still wrong ([d4663f0](https://github.com/jacob-delgado/workflow/commit/d4663f093252196ae3f520ed4c9f322a525c72e4))
* **debt:** name ptytest's fallback among the Windows-only files ([6daad1d](https://github.com/jacob-delgado/workflow/commit/6daad1de5bfe5a875186d0567aab2c7c978011e7))
* **debt:** name the pull request TRADE-35 landed in ([b519fd6](https://github.com/jacob-delgado/workflow/commit/b519fd6494df8c89086d1332a4895a585cb14bef))
* **debt:** name the Windows-only clauses TRADE-35 keeps ([a8972a6](https://github.com/jacob-delgado/workflow/commit/a8972a67d63cc2ab3a5cd63b45549c236ec6ac35))
* **debt:** narrow DEBT-286 to the gate texts still wrong ([009480a](https://github.com/jacob-delgado/workflow/commit/009480ab71c51c23e81d91da249ec7a44bede8fd))
* **debt:** point three register entries at what the code holds ([2305712](https://github.com/jacob-delgado/workflow/commit/230571244e4cdca9a61d1766855a8ce576b683c6))
* **debt:** record the stat of a file just opened as TRADE-36 ([cee3d6e](https://github.com/jacob-delgado/workflow/commit/cee3d6ef873c8201f0612e11b02e36d19be0caed))
* **debt:** record Windows-only code as built but never run ([388d1f3](https://github.com/jacob-delgado/workflow/commit/388d1f32f5fcc9c4df9378b7ce0e0d24266c3874))
* **debt:** reopen TRADE-38 on a write reported left half made ([411a173](https://github.com/jacob-delgado/workflow/commit/411a173c5308038a020c657213396bbb7d259851))
* **debt:** say a paid trade was deleted or rewritten to what is left ([6ebff17](https://github.com/jacob-delgado/workflow/commit/6ebff172744e21c8b604064e39c134fbbc1dde55))
* **debt:** say a shared Slack read ends with workflow's last quit ([ea72b5e](https://github.com/jacob-delgado/workflow/commit/ea72b5e737fcd27ab7dfb6f9602cfdd6f69661bc))
* **debt:** say in the register's opening how each trade-off went ([3062134](https://github.com/jacob-delgado/workflow/commit/3062134e27010526eb9f92dea925c20d50dca1cc))
* **debt:** say in TRADE-13 that the config encoders now panic ([aad804d](https://github.com/jacob-delgado/workflow/commit/aad804d6cd1ca5ca2a942022ad3c54ea2753ff44))
* **debt:** say the pre-1.0 edition is paid ([1e379e9](https://github.com/jacob-delgado/workflow/commit/1e379e9b449603d5f6240ae329e47693222900ee))
* **debt:** say what a broken pipe does on Windows in TRADE-38 ([45444a1](https://github.com/jacob-delgado/workflow/commit/45444a1319a0246e898ab10568c2b10378018c3f))
* **debt:** say what still ends a shared Slack read and a lost refresh ([6b4af7c](https://github.com/jacob-delgado/workflow/commit/6b4af7ccd6cb4fc45131869959bcbcd8d286f9ea))
* **debt:** say what TRADE-38's writes do once workflow exits ([a33f6da](https://github.com/jacob-delgado/workflow/commit/a33f6da4eb73eb4d2747dc4772ac3a4254e51cda))
* **debt:** spell canceling the American way in TRADE-38 ([8b5b7b2](https://github.com/jacob-delgado/workflow/commit/8b5b7b222063adb6013e5190de1a24ee7e95d673))
* **debt:** widen TRADE-14 to the web app the binary embeds ([97cc081](https://github.com/jacob-delgado/workflow/commit/97cc0819ac1f25976cba9a39c21df2824c31f076))
* **debt:** widen TRADE-35 to the code only macOS runs ([13a00d8](https://github.com/jacob-delgado/workflow/commit/13a00d8ebc18f708cdcf3f5d6c22d38cf84cb3cf))
* describe gobco's gate without the retired skip list ([86ca399](https://github.com/jacob-delgado/workflow/commit/86ca399041f4614319efec8e688d497d35e328e8))
* describe the dry run's store as the code reads it ([3cab444](https://github.com/jacob-delgado/workflow/commit/3cab444da82e61f8b2ea514fc39f66a623b2fcaf))
* **directory:** say the asker who starts a shared read waits it out ([18ac498](https://github.com/jacob-delgado/workflow/commit/18ac498657f4052d7d296d9705d65d57b7a07ee9))
* drop release history from the reference pages ([54b4b34](https://github.com/jacob-delgado/workflow/commit/54b4b343d04dc3d25e5e421e79df451bb780a9a1))
* keep one Status list, on the site's home page ([4e69284](https://github.com/jacob-delgado/workflow/commit/4e692845cc9c574d2a631839ef7f59da194b3b69))
* keep proc by its subpackage, and name a quit's wait in CLAUDE.md ([eca41c1](https://github.com/jacob-delgado/workflow/commit/eca41c179b194a8a5e7dc630f497752273e339b6))
* keep setup and the gate in CONTRIBUTING, linked from the site ([20e2634](https://github.com/jacob-delgado/workflow/commit/20e2634d14ac6252203c7afdc22c4737f895be0a))
* **lifetime:** say the package counts work in flight as well ([c4ee401](https://github.com/jacob-delgado/workflow/commit/c4ee401966a8ed89f3828650eb09f55747f6d066))
* **loop:** point at the depguard rule rather than list the imports ([aac4cbb](https://github.com/jacob-delgado/workflow/commit/aac4cbb33f9f73f26435804e3ce328393af0da0b))
* name the linters that are off, and why, rightly ([47a27d4](https://github.com/jacob-delgado/workflow/commit/47a27d4e62ad82cd3c3ba4d04e316cfb3b8bea13))
* name the plain webhook in the taglines ([3251cce](https://github.com/jacob-delgado/workflow/commit/3251cce31baad3617ac88c001f0a3c49e82bc5be))
* name the release platforms and contents in one place ([5d41c7c](https://github.com/jacob-delgado/workflow/commit/5d41c7c7793cba72126eafddd7f190d2054b1382))
* **proc:** say a streamed line waits for its reader while ctx runs ([97633bd](https://github.com/jacob-delgado/workflow/commit/97633bd00475891cd4c7f26fd13f95da4c8cf1d0))
* **readme:** link to the install page and setup, not copy them ([4386f36](https://github.com/jacob-delgado/workflow/commit/4386f36a0c2b42aaa8c9ba68e34305b0cbec4182))
* **readme:** link to what is kept rather than list it ([3bbe1e2](https://github.com/jacob-delgado/workflow/commit/3bbe1e2199d1171787fe2a19801684a9b6d4594b))
* **readme:** name nine panes' keys and the web's nine sections ([62c9850](https://github.com/jacob-delgado/workflow/commit/62c9850b03028c3f111282b2b72609b0fab4919f))
* **readme:** say once how the configuration file is kept safe ([7d2467b](https://github.com/jacob-delgado/workflow/commit/7d2467b3aaed1c618746403809cdc587c92446bd))
* **report:** say FaultDetail gives the shared classes' words ([d0b4d84](https://github.com/jacob-delgado/workflow/commit/d0b4d84a00388736167e1f7e501ff178d45bdcdb))
* say a shared web section refuses only keys the page binds ([0a37137](https://github.com/jacob-delgado/workflow/commit/0a371378cf62a7b642f26fbb9f472e31e79801fd))
* say detached work finishes only where something waits for it ([e894f3a](https://github.com/jacob-delgado/workflow/commit/e894f3a307d8f35ed1dcd63d9d852bc460815844))
* say exactly which picked filter value goes last at zero ([4abc798](https://github.com/jacob-delgado/workflow/commit/4abc798c758d1a90f9693b7b76c03f062dbbaded))
* say how the Tasks list and the filters read now ([10259de](https://github.com/jacob-delgado/workflow/commit/10259de8d86b13f4cd1e73742fc2bbf9673176cf))
* say the release is built on a runner, not in the container ([30c9aad](https://github.com/jacob-delgado/workflow/commit/30c9aadbb573a573cfd62fe2188cc05403f535da))
* say when a function compares a kind rather than switching on it ([7408526](https://github.com/jacob-delgado/workflow/commit/7408526aa5f243cec4f884ef277066e83a317068))
* say where a context is detached, and that none is kept ([f7bd09d](https://github.com/jacob-delgado/workflow/commit/f7bd09dc784682f718cfdef7e0c28602741e114d))
* say which of the every-pane keys the help answers, and how ([17d597d](https://github.com/jacob-delgado/workflow/commit/17d597d215fb6ef97fe0da0591aae89ec642d3d7))
* say which picked filter value goes last, as the code orders it ([4ec5cb8](https://github.com/jacob-delgado/workflow/commit/4ec5cb8ee5018a514deb4164306515601a5b8bf9))
* say which reads a quit or a directory switch stops ([31f87d6](https://github.com/jacob-delgado/workflow/commit/31f87d629f5272852fe993151b2bd258755c51f7))
* **scripts:** drop entry IDs the register no longer explains ([b0d3fda](https://github.com/jacob-delgado/workflow/commit/b0d3fda4462064f2e2d870fcdf484456ac9df706))
* **scripts:** say internal/seams declares data types as well ([53ad8aa](https://github.com/jacob-delgado/workflow/commit/53ad8aaaf06c78713354faa30ce4b3d2f19f91b6))
* **security:** describe both store files and the gitignore warning ([451487f](https://github.com/jacob-delgado/workflow/commit/451487f364b7f790373b04206254666ee4b444d6))
* **site:** point at the README rather than repeat its tagline ([b83e7eb](https://github.com/jacob-delgado/workflow/commit/b83e7eb39f2b7250e3e6701562ae4ebcbb412205))
* state the rule by which the gate tells an error check ([c8cbb32](https://github.com/jacob-delgado/workflow/commit/c8cbb327260bb1ffde4ff1ae8be78bd268cfda21))
* **store:** name both files in the package doc ([21d7043](https://github.com/jacob-delgado/workflow/commit/21d7043c2971adcebeba02d9ad39d08c733fcb65))
* **store:** say a dry run reads no cache, not no store ([39b86a0](https://github.com/jacob-delgado/workflow/commit/39b86a0c3d448d8c6bf8dbdab256f8edc1fa5ba4))
* **tui:** name every refusal in CheckKeys' doc comment ([bb8cac1](https://github.com/jacob-delgado/workflow/commit/bb8cac125764f902c469f4b24f937d8ac89d1107))
* **tui:** say every pane where two comments still said everywhere ([0e33e4e](https://github.com/jacob-delgado/workflow/commit/0e33e4e5dc1ff4cb259178e052b7b3bb8ba7b2a7))
* **tui:** say what one step is to each overlay that steps ([8cb0f4d](https://github.com/jacob-delgado/workflow/commit/8cb0f4d94d070a7b1f491c1abe82e9ba56a0fd5a))
* **tui:** say what ring is for without listing its users ([031b874](https://github.com/jacob-delgado/workflow/commit/031b8745b348f7fbf6b0a7d4169a09e72d373d5c))
* **usage:** say a signal's exit cuts work that would carry on ([fdcbada](https://github.com/jacob-delgado/workflow/commit/fdcbadaffbb2b66e42876e274eb1ad197b132d7b))
* **usage:** say the scroll keys page a failed check's log ([f1673f1](https://github.com/jacob-delgado/workflow/commit/f1673f1ae16415972a44033f2fdffd619d499fcd))
* **usage:** say what a quit, a switch and a signal do in flight ([46eee76](https://github.com/jacob-delgado/workflow/commit/46eee7619393e4ba048943a715abedb400321dd5))
* **ux:** point at the key-group symbols the terminal holds now ([67f2008](https://github.com/jacob-delgado/workflow/commit/67f2008f5f80c41424ce01728048f604246d32f0))
* **web:** count the Repositories section's writes as requests ([6a71b45](https://github.com/jacob-delgado/workflow/commit/6a71b4598dd663945cdb34345dba2970ae125197))
* **web:** name the terminal's keys and the API's requests rightly ([4ea74a5](https://github.com/jacob-delgado/workflow/commit/4ea74a5c1fe6faa854575ded076a198cea7a8b65))
* **web:** say a done answers the task done beside the list ([375344e](https://github.com/jacob-delgado/workflow/commit/375344e8087c22ae6e8729058c61c62ced400d5b))
* **web:** say a story stage's mark takes its section's hue ([a056e02](https://github.com/jacob-delgado/workflow/commit/a056e02dc7148aee2f846fdc551014b3c38328d1))
* **web:** say what a task just marked done offers ([19a777f](https://github.com/jacob-delgado/workflow/commit/19a777fbd12edb4aa1c7681b8669eab715aa52c5))
* **web:** say Write it anyway refuses http to another machine ([913b0f7](https://github.com/jacob-delgado/workflow/commit/913b0f776358d89d3ccd8f37e40c5f2bedb26d55))
* **webserver:** say plainly where an issue not branched for shows ([43a30d3](https://github.com/jacob-delgado/workflow/commit/43a30d3b55ee26e24414e3e39eb3d4a6182f11ca))
* **webserver:** say where an issue picked but not branched for shows ([b798507](https://github.com/jacob-delgado/workflow/commit/b798507e5cb6edc0435774e7ffb866f2efcbeedb))
* **wiring:** name Environment.Deps in the tests' comments ([c1d02b1](https://github.com/jacob-delgado/workflow/commit/c1d02b158fb9a7ad7cedade1a9fb114ff01c619d))


### Build & Packaging

* add task e2e and name it in the definition of done ([f362cb9](https://github.com/jacob-delgado/workflow/commit/f362cb92acec3c00efeb131c80268fe6c02aab20))
* Bump source-map-js from 1.2.1 to 1.2.2 in /web ([f4b7856](https://github.com/jacob-delgado/workflow/commit/f4b7856147956a6c8224e68b85421e3bc50fbbd7))
* Bump the web group across 1 directory with 6 updates ([57016dd](https://github.com/jacob-delgado/workflow/commit/57016dda743ee0539a8dfb2bd6f06530084253fb))
* check that CLAUDE.md's layout names every Go package ([d921ff8](https://github.com/jacob-delgado/workflow/commit/d921ff8dacd361383f6239d690b76a08b3b1fb74))
* cite CI's coverage reading beside the local one for the floor ([6715b17](https://github.com/jacob-delgado/workflow/commit/6715b17e1feb5b255eb31a8271e88e8fc9bd15a0))
* count an errors.Is after an error check as an error arm ([6900480](https://github.com/jacob-delgado/workflow/commit/690048074daff935c6bcf483d9dba511d805d0e2))
* count err2, errParse and ctx.Err() as error checks in the gate ([47ce721](https://github.com/jacob-delgado/workflow/commit/47ce721eed0eada4161ad66b1842c961f9834d89))
* **deps:** install mise 2026.10.8 everywhere ([7bc0ed1](https://github.com/jacob-delgado/workflow/commit/7bc0ed1ec1450b74f8ee00e6297b076f6f5c809a))
* **deps:** move golang.org/x to its newest releases ([04bc37f](https://github.com/jacob-delgado/workflow/commit/04bc37fad6638e5057e984de51959c4a73dd7b6e))
* **deps:** move lefthook, typos, govulncheck and deadcode to latest ([95bed0a](https://github.com/jacob-delgado/workflow/commit/95bed0a25d42f871e046d9fe88c45f0c166fb778))
* **deps:** move the API code generator's readers to their newest ([b71b2b6](https://github.com/jacob-delgado/workflow/commit/b71b2b676dbfef207693fddbef52f014da9ab402))
* **deps:** move the Charm stack to bubbletea v2.1.0 ([38b1d9f](https://github.com/jacob-delgado/workflow/commit/38b1d9ffaae9ed4c2c75f64881ac5525f02cab03))
* **deps:** move the devcontainer's image and feature to latest ([c07ae38](https://github.com/jacob-delgado/workflow/commit/c07ae38b3467a654569886f23a0281099accad83))
* **deps:** run the web app's package manager at yarn 4.18.1 ([b3287e8](https://github.com/jacob-delgado/workflow/commit/b3287e83a3263f8e8a4d652c6f6247a5f7bd80d7))
* **deps:** say why modernc.org/libc stays at what sqlite pins ([eec59e0](https://github.com/jacob-delgado/workflow/commit/eec59e06a9f2adfeb95b492ed5580916290d93f5))
* **deps:** update the web packages within their majors ([2e39fc8](https://github.com/jacob-delgado/workflow/commit/2e39fc8ea342f837e78e23828d62e46410613027))
* **deps:** waive the age gate for the tool versions under a week old ([d95b330](https://github.com/jacob-delgado/workflow/commit/d95b3305f2e1a882d67e980095999fb3f43b6493))
* **deps:** waive the age gate for the web versions under a week old ([b5bde05](https://github.com/jacob-delgado/workflow/commit/b5bde05fa52ee35a46247e1180876e5b8c9e526f))
* **deps:** waive the week-long wait for the young Go modules ([25d2829](https://github.com/jacob-delgado/workflow/commit/25d28290c81bf15fca1f5286319392f8fdf2f39b))
* drop solid-js 1.9.16 from the age gate's waivers ([c804c51](https://github.com/jacob-delgado/workflow/commit/c804c5161afeca7a3124e27093ca2ca35bbf7bcb))
* drop the age gate's exception for Go 1.27.2, a week old now ([4e95692](https://github.com/jacob-delgado/workflow/commit/4e95692fb5b26828aca7e1c3370b655d8c6c8b8f))
* fail condition coverage on an arm no test reaches ([5425a8a](https://github.com/jacob-delgado/workflow/commit/5425a8ac840969bb8a867d39f1edc3fdc01630ba))
* give the race tests an explicit twenty-minute timeout ([1babefa](https://github.com/jacob-delgado/workflow/commit/1babefa6faf3b1104a8904381786f9effb529523))
* hold the coverage allowlist's arms to the trade-off register ([5594325](https://github.com/jacob-delgado/workflow/commit/559432549129173081b309d0618917c720385afb))
* hold the credential and store packages to floors of their own ([69794c9](https://github.com/jacob-delgado/workflow/commit/69794c91e226c563563f3b9cf1406f05ef9587a0))
* keep an allowlisted arm by its number in its function ([7a6a130](https://github.com/jacob-delgado/workflow/commit/7a6a1300d5a87db5dd84e961a4d8c4aa63e93141))
* keep no arm in the allowlist that no trade-off entry names ([78b277b](https://github.com/jacob-delgado/workflow/commit/78b277b8e42cf141573b335ee60bf7785386c520))
* let the age gate take solid-js 1.9.16 for its advisory fix ([89aecf5](https://github.com/jacob-delgado/workflow/commit/89aecf5b47a78d4d80ebc650b64275a121557906))
* let the maintainer waive the dependency age gate by name ([cd6303d](https://github.com/jacob-delgado/workflow/commit/cd6303d4bf6ecefe2f8f4fe0d4fc8d43e5cb225b))
* lint with golangci-lint 2.14.0 ([9d84506](https://github.com/jacob-delgado/workflow/commit/9d845066b8b6fb767832f5ba06916b1129ecdc45))
* **lint:** refuse a test file past 700 lines ([834224f](https://github.com/jacob-delgado/workflow/commit/834224febab0a3db8ae92dc32a7f66bbead59a4e))
* measure a package of build-tagged twins a file at a time ([d12c906](https://github.com/jacob-delgado/workflow/commit/d12c906e6ad5d0d844b038fb405e58d48d1329d3))
* move cobra's and sqlite's own dependencies to their newest ([200ee10](https://github.com/jacob-delgado/workflow/commit/200ee10aeebf385e909ba738ccf7c91658d8a0d5))
* move Go to 1.27.2 for the standard library's fixes ([d263d4f](https://github.com/jacob-delgado/workflow/commit/d263d4f445750c257d1de91350c59db2e7c4afe9))
* move go-runewidth and xo/terminfo under the Charm stack ([9fa1eec](https://github.com/jacob-delgado/workflow/commit/9fa1eec7370e054fbc410cdc19545e51e406fe18))
* move lefthook to 2.1.16 and typos to 1.50.3 ([2a0f9b0](https://github.com/jacob-delgado/workflow/commit/2a0f9b07d240baa0f04e8d87f25012840a4e9421))
* move task, hugo and markdownlint-cli2 to their newest ([a117fff](https://github.com/jacob-delgado/workflow/commit/a117fffad61cd351e71a025c386bfc849f12ee55))
* move the OpenAPI stack's jsonpointer and speakeasy openapi ([0df7461](https://github.com/jacob-delgado/workflow/commit/0df74614ae3bf26ff9962ed221ab22bd3b4a6b6f))
* name the one trade an allowlist number cannot catch ([8364438](https://github.com/jacob-delgado/workflow/commit/8364438e7fc47b781b38dbb0c30f2a932f01eb85))
* raise internal/tui's ceiling to 96 once its shape was looked at ([2f9561d](https://github.com/jacob-delgado/workflow/commit/2f9561d606ced15c0c006375120a7636da3e7164))
* raise the condition coverage floor to 93 ([0a0f79f](https://github.com/jacob-delgado/workflow/commit/0a0f79f61ce62ad607d8c8ce7863172190ad68d9))
* ratchet the coverage floors to what the paydown's end measures ([6833780](https://github.com/jacob-delgado/workflow/commit/6833780adbbf35bf23d1eb6736e2c6edd5e2400c))
* say the docs name a platform only as an example ([8294673](https://github.com/jacob-delgado/workflow/commit/82946739b3931e32ec963dc4186ed1976d357245))
* say when the condition gate runs as root what that leaves unseen ([0c2f20e](https://github.com/jacob-delgado/workflow/commit/0c2f20e8674f06555397c9734150d72401626085))
* say why internal/tui stays one package as it stands now ([2681f06](https://github.com/jacob-delgado/workflow/commit/2681f06249ac4259b82e73ed6443c9a5d6e06a8d))
* **testshape:** read a synctest bubble's body for its markers ([38ce834](https://github.com/jacob-delgado/workflow/commit/38ce83469d0247639b44809d51e44ec56468ddcd))
* **web:** take solid-js 1.9.16 and seroval 1.6.8 ([b97fe03](https://github.com/jacob-delgado/workflow/commit/b97fe03a60a522ca0179333ed668f1799a9273d9))


### CI

* Bump the actions group across 1 directory with 6 updates ([46be594](https://github.com/jacob-delgado/workflow/commit/46be5940cf6454449db04f888e054ca3ce256367))
* keep the statement figure when condition coverage is missing ([172c41b](https://github.com/jacob-delgado/workflow/commit/172c41b3436861bc5c930b85b2cc5565238eee91))
* move the pinned actions to their latest releases ([cadc6a5](https://github.com/jacob-delgado/workflow/commit/cadc6a56dceb9957f587297eb4c426729ec9848e))
* name the exact release beside every pinned action ([1a64ed1](https://github.com/jacob-delgado/workflow/commit/1a64ed10dd26595dddc87b31de31efd064f8e5f2))
* run condition coverage in its own job beside Test ([29fdfe9](https://github.com/jacob-delgado/workflow/commit/29fdfe9837cc6ddfebe3e10a13002a906cd19ac8))


### Tests

* **cli:** find the server's seams with a tagless switch ([e042002](https://github.com/jacob-delgado/workflow/commit/e042002d145481eda57df0d6e92e99fb32ff84bc))
* **cli:** hold a signal to ending the wait under a write that goes on ([2848ca6](https://github.com/jacob-delgado/workflow/commit/2848ca6eeb00fcd550f1a205ae7f9adad445d19c))
* **cli:** hold scripting.md's command lists to the command tree ([6d620a7](https://github.com/jacob-delgado/workflow/commit/6d620a720662fa7f41635d2f8fd26b995af46e02))
* **cli:** hold the fork lock while writing a stand-in program ([9ed3161](https://github.com/jacob-delgado/workflow/commit/9ed3161f6a5849a66640d27f47d813eb639db8b3))
* **cli:** let a signal test's fake interface end with the test ([cc3f065](https://github.com/jacob-delgado/workflow/commit/cc3f0658a0ee5d5673e66941792d9c43da571ec8))
* **cli:** pin the reason that follows the not-remembered sentence ([c058f87](https://github.com/jacob-delgado/workflow/commit/c058f87710f4e08054d8f13c42e651fcc3d62d71))
* **cli:** reach config init's and slack login's failures ([d55d543](https://github.com/jacob-delgado/workflow/commit/d55d54325bb6b1cded01fbe6823671fe9807a6c7))
* **cli:** reach db-clean's, repositories' and a switch's failures ([e9401d2](https://github.com/jacob-delgado/workflow/commit/e9401d21f20412f2b6bf47fa03715f41baf9ee8f))
* **cli:** reach doctor's token arms and a terminal's secret read ([ccf8829](https://github.com/jacob-delgado/workflow/commit/ccf8829614e5186a7aa0aa8b9aa85e4cff51a56d))
* **cli:** reach the request log's own reading of a removed directory ([647ca53](https://github.com/jacob-delgado/workflow/commit/647ca537f890ce86bef37a8e36a5cbdaed4a63cc))
* **cli:** release a held write before waiting for workflow to return ([4ee945e](https://github.com/jacob-delgado/workflow/commit/4ee945ef21fd7544350a4514f0a9bfc6bc14e8cb))
* **cli:** reopen where it was when a switch cannot move the process ([2e75ce0](https://github.com/jacob-delgado/workflow/commit/2e75ce0398b837e824a75b41373cedc3735c51ee))
* **cli:** split four command test files that passed 500 lines ([c69d9ad](https://github.com/jacob-delgado/workflow/commit/c69d9adf0a3d16cee15f78732962b4abbfe80fe8))
* **cli:** watch the interface's context for an end after a quit ([47500ad](https://github.com/jacob-delgado/workflow/commit/47500adb8eab52a85b32ebad92a8e1e515e6139e))
* **codeowners:** give GitLab's reading of the file a test file ([e8c05f3](https://github.com/jacob-delgado/workflow/commit/e8c05f309794133102b32c2361777eda7e8a7244))
* **config:** leave an inherited home-only key out of a repository save ([60db239](https://github.com/jacob-delgado/workflow/commit/60db2395b39cc6bd4f15c6ebbe3a624511b3f7c3))
* **config:** report a file that cannot be opened, read or created ([75704d4](https://github.com/jacob-delgado/workflow/commit/75704d43b7c16912dc1610216412143018158ade))
* **directory:** wait for the second reader to join, not 50 ms ([48b6c32](https://github.com/jacob-delgado/workflow/commit/48b6c326c1f30f3dd5c0b4aa0bf1eb4b2943eb5d))
* **docsgen:** name a directory it cannot list ([9edb151](https://github.com/jacob-delgado/workflow/commit/9edb151fe6efdd363eea4797cc4ea075166e71fd))
* **filelock:** report a file that cannot be locked ([c8a4861](https://github.com/jacob-delgado/workflow/commit/c8a4861ead44ae51820962fdab10e5b2e8abeeae))
* **forge:** fail the storage's transport as http.Client.Do fails ([d66f46f](https://github.com/jacob-delgado/workflow/commit/d66f46f62ad4f630a04991979c756ef05da834e6))
* **forge:** reach a job log's redirect, storage and read failures ([7deaa62](https://github.com/jacob-delgado/workflow/commit/7deaa62922cba2c85de3a90a29595aa975dc6cf6))
* **forge:** reach the arms a pull request's people and reads left ([87267b7](https://github.com/jacob-delgado/workflow/commit/87267b7c15befe94ac5e8ae95e8f222e3d311d52))
* **forge:** reach the failures of assigning, reading and commenting ([de7c96d](https://github.com/jacob-delgado/workflow/commit/de7c96d8fb08e51738656c93de263a5800ae2217))
* **gittest:** fail the test with git's reason and an unread tree ([b7c2736](https://github.com/jacob-delgado/workflow/commit/b7c273631f87efbe4f76a958eaf25f9364414a17))
* **httpx:** say why an answer could not be closed ([4a9b07a](https://github.com/jacob-delgado/workflow/commit/4a9b07a7387d5391b9286fe739d87b0ff558db6e))
* **jira:** reach Activity's failures, its paging and a type unread ([3c474b3](https://github.com/jacob-delgado/workflow/commit/3c474b35c778454019e313c368cc0173b531d2a3))
* **jira:** read an unreadable date over a period from the start ([530df84](https://github.com/jacob-delgado/workflow/commit/530df84891e0fd6295c4f2478fbf68cf758ac650))
* **keychain:** read nothing without an account name ([7d94d74](https://github.com/jacob-delgado/workflow/commit/7d94d742af83ed33564730822ca336e14f0d6033))
* **lifetime:** begin the following work while the wait settles ([6e078f3](https://github.com/jacob-delgado/workflow/commit/6e078f312114ceec2a6e06b33718736bb9feb460))
* **lifetime:** watch a detached context for an end after its asker ([3c54ae2](https://github.com/jacob-delgado/workflow/commit/3c54ae226b98ee842095b446a8ca033c5050110c))
* **loop:** pin that the not-remembered error says the warning's words ([1eea2de](https://github.com/jacob-delgado/workflow/commit/1eea2dee5d2b9374950f792c7dc4d26b5d3141ee))
* **messaging:** reach the reads, tokens and links no test failed ([9d61fd2](https://github.com/jacob-delgado/workflow/commit/9d61fd22ad66badd065574e6043bc2d2ba799ee8))
* **proc:** deliver a line written after the program exits ([f85bcd5](https://github.com/jacob-delgado/workflow/commit/f85bcd5966e5d13eb17179fb596105dd3c1a7367))
* **proc:** exit the helper only once the late writer it leaves starts ([291e88b](https://github.com/jacob-delgado/workflow/commit/291e88b82d44465c543026a68f37288c4ef5f3dc))
* **proc:** give pgroup's cancel tests of its own ([5cd3f06](https://github.com/jacob-delgado/workflow/commit/5cd3f0630cbc01b0ff0864cc2022497f2bf481e8))
* **proc:** say on standard error which step of a helper failed ([3eacb34](https://github.com/jacob-delgado/workflow/commit/3eacb3492f77645e9878a5c6464c79a7f8852de0))
* **ptytest:** open a pseudo-terminal for a test on Linux and macOS ([a77bc86](https://github.com/jacob-delgado/workflow/commit/a77bc86fa39b22654c8514b74dbc5fc5a96b2265))
* say commit and stage-all share the Commits pane's context ([2927b8a](https://github.com/jacob-delgado/workflow/commit/2927b8a613e7befc70d41cb169629f01a234fbd1))
* **scripts:** make the gate's errorHandler case fail a loose pattern ([c60ca08](https://github.com/jacob-delgado/workflow/commit/c60ca0852526b6b266b3793fa19a25a25c7e0b90))
* **scripts:** pin a prefixed and a numbered name as error checks ([6ffc8ab](https://github.com/jacob-delgado/workflow/commit/6ffc8ab1c21efc3e4f09f4fb7486c67c1136b22d))
* **seams:** hold every I/O seam to a context first ([a854251](https://github.com/jacob-delgado/workflow/commit/a854251c71a39aa16189a151d58b44218438eac2))
* **setup:** check Keepable and a write over an unreadable home file ([22ce63e](https://github.com/jacob-delgado/workflow/commit/22ce63e554faf036a98cffa0f0c3e2d2193f1550))
* **setup:** gather what a new file lies over in one test file ([8c9f147](https://github.com/jacob-delgado/workflow/commit/8c9f1474e00138d3151572a83c547c6b94ce2604))
* **setup:** give the keychain offer's tests a file of their own ([dec9fb6](https://github.com/jacob-delgado/workflow/commit/dec9fb6501d445d8a2fe4a1cf8aa50a351cc6dbb))
* **setup:** refuse in Beneath a home file that is no configuration ([2e36a68](https://github.com/jacob-delgado/workflow/commit/2e36a68ff51a25901b033ab9d7b40eae4d211b70))
* skip under root the three tests a sealed file cannot fail there ([f2ed3a8](https://github.com/jacob-delgado/workflow/commit/f2ed3a85d3e460fb49e1c3c91f1a4d0904c08e02))
* **slackauth:** report a lock or a file store that cannot be read ([5925d67](https://github.com/jacob-delgado/workflow/commit/5925d676cac1325b1ea530e4ec594dbd32854332))
* **sqlitefile:** report a database file that cannot be opened ([084c755](https://github.com/jacob-delgado/workflow/commit/084c75567da0031342522d16802e586e93c1895d))
* **store:** report kept data and a clean that cannot be read or made ([4e5a651](https://github.com/jacob-delgado/workflow/commit/4e5a6514498788d909c3cea243ca918813dd2c13))
* **taskwarrior:** reach Touched's context failure and Narrows ([ffdb8ae](https://github.com/jacob-delgado/workflow/commit/ffdb8ae5036cf1fc780219cacaf45533695a5dbf))
* **tui:** add nothing to a task's row when sorted by issue ([9ae7f7c](https://github.com/jacob-delgado/workflow/commit/9ae7f7c4f1d7a815ab2d06d67cd26099e042fabe))
* **tui:** check apart that the help puts jump-to-pane on every pane ([fa1841d](https://github.com/jacob-delgado/workflow/commit/fa1841d55cec298124299eff5014dac48a444675))
* **tui:** gather the task list's click tests in a file of their own ([cc709dd](https://github.com/jacob-delgado/workflow/commit/cc709dd162445db05fa911f2aa295e3254319455))
* **tui:** hold each key to its own action's row in usage.md ([790a078](https://github.com/jacob-delgado/workflow/commit/790a07866fb590c9c9a100eee174aff639b7af8a))
* **tui:** hold the mockup's pane sections to the server's ([8672189](https://github.com/jacob-delgado/workflow/commit/8672189a791a9ce9fb5ce0b6b815cfb137b8a856))
* **tui:** hold the overlay probe's table to the contexts both ways ([66e4295](https://github.com/jacob-delgado/workflow/commit/66e42955bb11ccb90c1d65179b094eae1c0d19f7))
* **tui:** hold usage.md's key table to every key the help lists ([a425d68](https://github.com/jacob-delgado/workflow/commit/a425d68082965e60904c1a25c63886d0c29243b6))
* **tui:** keep a comment draft per issue, replaced when rewritten ([4695168](https://github.com/jacob-delgado/workflow/commit/46951682c4229e7e4bf7fb02880f087d204b01ec))
* **tui:** keep the issues listed when a further page fails ([97caed1](https://github.com/jacob-delgado/workflow/commit/97caed1789eeb442d1485642756c9373fd8cd5c4))
* **tui:** leave tagging out when the workspace has no credential ([a08af1f](https://github.com/jacob-delgado/workflow/commit/a08af1f27bda3b6653f2e56277714471c62a0dba))
* **tui:** move CheckKeys' refusals but conflicts to their own file ([530733c](https://github.com/jacob-delgado/workflow/commit/530733ceed258e5c0e14cc1e80a1d64c3e42a302))
* **tui:** name the no-store preview test for the seams it unsets ([4feb673](https://github.com/jacob-delgado/workflow/commit/4feb673ed02279d613f4d8c840ca26e9ea87a974))
* **tui:** name the strings the new tests repeat ([2fd2911](https://github.com/jacob-delgado/workflow/commit/2fd2911373408d8647327aeb85b8a1002a6140e5))
* **tui:** offer the done again on a task whose done was refused ([b701f66](https://github.com/jacob-delgado/workflow/commit/b701f66b5245b33b2037a5bb7c0ef014f068b511))
* **tui:** offer the first run over issues an earlier session cached ([2a9a6fe](https://github.com/jacob-delgado/workflow/commit/2a9a6fedbfe7654d02d30b8b0c825896ef461a14))
* **tui:** pin that a preview with no store links and tags no one ([2ce973b](https://github.com/jacob-delgado/workflow/commit/2ce973ba3037fea3298133a31dc761665f4b452b))
* **tui:** press the pane numbers in every overlay that types nothing ([c7777ab](https://github.com/jacob-delgado/workflow/commit/c7777abaa5eb04b121fbc4dc46e0f949e85d38b1))
* **tui:** probe each pane in every state that answers its keys ([85a3b03](https://github.com/jacob-delgado/workflow/commit/85a3b039c6b1955258d320ffb0ecf7d45cc510f1))
* **tui:** read a Summary source left out as not set up again ([7e8216c](https://github.com/jacob-delgado/workflow/commit/7e8216ce39429938c05e225a43407661a8a11a2e))
* **tui:** read keys from a terminal Run is handed ([ce4425d](https://github.com/jacob-delgado/workflow/commit/ce4425d7d5bce4bd6fd5c33d4da21601a60a4f2a))
* **tui:** refuse a calendar range longer than a year ([59627eb](https://github.com/jacob-delgado/workflow/commit/59627ebc08e50769ee6b3fe7e411d9a376a508ae))
* **tui:** refuse to queue a post announced while its preview was open ([f9b4405](https://github.com/jacob-delgado/workflow/commit/f9b4405b03bfa59a9d3c9abc1633869571ac5a30))
* **tui:** see Run wait for its input before q ends it ([144aaec](https://github.com/jacob-delgado/workflow/commit/144aaecee5907a555f0477c987e74ead3dbd932d))
* **tui:** select a task by a click only where one is drawn ([d07fb7e](https://github.com/jacob-delgado/workflow/commit/d07fb7e90e15298394906754d6054cf136b1db2e))
* **tui:** show a directory with no path from its root as the root ([c372dd3](https://github.com/jacob-delgado/workflow/commit/c372dd3856ededac2377ca4f876bafcce9ec70d6))
* **tui:** spell the apply key's name in a test name as typos reads it ([45f1ac8](https://github.com/jacob-delgado/workflow/commit/45f1ac8009260a186e5d2e4e671bf8eb28cb29e8))
* **tui:** stop polling CI after a failed check a post waits on ([0fe9ab5](https://github.com/jacob-delgado/workflow/commit/0fe9ab54b8fd209ab9b7e4e852797876697e47d5))
* **tui:** tag the rest where a group Slack cannot tag is not offered ([a7072b2](https://github.com/jacob-delgado/workflow/commit/a7072b21503596604f5a422bccea8740209a5c80))
* **tui:** take a view's name only when one is named the same ([11798d2](https://github.com/jacob-delgado/workflow/commit/11798d2db3b68c118cda63b9b55575b6e76de146))
* **tui:** track nothing when work starts on a key that is not one word ([176d34b](https://github.com/jacob-delgado/workflow/commit/176d34bc58fae99434d3d614a5cc46e214c92e39))
* **tui:** try Local data again from its failure, not from a removal ([d0f3493](https://github.com/jacob-delgado/workflow/commit/d0f349352cb05fc63693fb23058d696ce9947493))
* **tui:** type a paste into each form's field, and only there ([2855be8](https://github.com/jacob-delgado/workflow/commit/2855be873fd3af627d409191a60ccb62ad1b858b))
* **tui:** type a paste into the commit's scope or subject ([319d2bd](https://github.com/jacob-delgado/workflow/commit/319d2bd7afbabdf8eb44027eb1116243b642b36f))
* **tui:** write nothing to remove from a configuration that is invalid ([2acc4f7](https://github.com/jacob-delgado/workflow/commit/2acc4f7fba49544870102ffe4f77eb72c2e28414))
* **web:** check the line that names GitLab's merge request ([b9dc001](https://github.com/jacob-delgado/workflow/commit/b9dc001c55de2b5258a4cbf4d68479c3efe4dcd7))
* **web:** drop a stage the old story always drew ([52d5700](https://github.com/jacob-delgado/workflow/commit/52d5700f08a2a93ba8001f8695f6e4834f3e306b))
* **web:** find the marker rule's test function among its arguments ([7e719e6](https://github.com/jacob-delgado/workflow/commit/7e719e6fce7bcd1b2e1a4168f451924b2da5590f))
* **web:** give a failed check's reason and log a test file ([3dd28d4](https://github.com/jacob-delgado/workflow/commit/3dd28d4690a19eca55172d062c44489a164c1a69))
* **web:** give each task and review request data agreeing with itself ([39f9227](https://github.com/jacob-delgado/workflow/commit/39f922785e07e78665c207e0eedd77d199059936))
* **web:** give rotate the lower id so only tag order puts renew first ([2c932ef](https://github.com/jacob-delgado/workflow/commit/2c932ef6e4e4443d27ea7f92a220b6ba82ad2ddc))
* **web:** give the story's worktree flows a test file ([4aca9bb](https://github.com/jacob-delgado/workflow/commit/4aca9bba38d2bb11a029f8261718b9068d34c1fb))
* **web:** name the query client's test for its defaults ([544af7b](https://github.com/jacob-delgado/workflow/commit/544af7b1eba5ea42d049cf35373576048ea2b657))
* **web:** pin that a closed pull request is never said announced ([fd7235d](https://github.com/jacob-delgado/workflow/commit/fd7235d2dce77daa8912543909ece7c269bf2f12))
* **web:** prove a review request's CI words are the server's label ([8182795](https://github.com/jacob-delgado/workflow/commit/8182795ce453ce7d76283310d549ebde7b255860))
* **web:** quote the server's refusal of a Jira address as it says it ([faeee6b](https://github.com/jacob-delgado/workflow/commit/faeee6b3f8a211437323c5138ce7b4ca934deb6f))
* **web:** rank each list of several tasks as the server ranks it ([d637d02](https://github.com/jacob-delgado/workflow/commit/d637d02d48f4e891133175a5532379abe8d1060c))
* **web:** rank the Mark done case's two tasks as the server does ([d079c53](https://github.com/jacob-delgado/workflow/commit/d079c538552ff9bbd777843a8ad7e8f712bd1beb))
* **web:** rank the sorted e2e list's tasks as the server does ([ca27ed8](https://github.com/jacob-delgado/workflow/commit/ca27ed8ee1e25277e56cf25ab5b6387130dec483))
* **web:** read the golden frames inside the frame-count test ([bb0db97](https://github.com/jacob-delgado/workflow/commit/bb0db97f27fc17b68ed7e2e70a727bd878bc57b8))
* **web:** read the keyboard tests' pane sections from the mockup ([48ea1f0](https://github.com/jacob-delgado/workflow/commit/48ea1f0e593ce820cb9b0c2e0a8ee28522ce61d2))
* **web:** replace a task's state facet by its kind, not its place ([6dc9682](https://github.com/jacob-delgado/workflow/commit/6dc9682b91e6222394b2ea7d63b92357783bd22c))
* **web:** see a done's missing answer give way to the next frame ([4873da2](https://github.com/jacob-delgado/workflow/commit/4873da2f41d265fc50d736bcfa3b6d1fa4be941b))
* **webserver:** fail a facet test whose answer lists nothing ([d3c734e](https://github.com/jacob-delgado/workflow/commit/d3c734e75fc7be59e518943a416cc9503568f951))
* **webserver:** find the done before asking what was read after it ([cfbeb23](https://github.com/jacob-delgado/workflow/commit/cfbeb23ba96b92ba88c0be319b1817a75b88337b))
* **webserver:** hold scripting.md's families table to every code ([841f3f7](https://github.com/jacob-delgado/workflow/commit/841f3f743dfba90431ea48aac5fcc2a1259e9d93))
* **webserver:** hold the overlap tests' windows inside their seams ([125cebf](https://github.com/jacob-delgado/workflow/commit/125cebfc98f06f7a9925e2dbec56d784c40e3406))
* **webserver:** move the refusal wording tests beside taskrefusal.go ([bd3540d](https://github.com/jacob-delgado/workflow/commit/bd3540d812e446097231d093c0e6870634a54913))
* **webserver:** pin that a new hold drops the last one's warning ([9932215](https://github.com/jacob-delgado/workflow/commit/993221575e5bd113baec4595bebb7003c2064fa3))
* **webserver:** reach a run request that does not decode ([0bbe328](https://github.com/jacob-delgado/workflow/commit/0bbe3288389a5e40d027b402b084ef2b083c91a1))
* **webserver:** reach announcing's arms that wait, tag or race ([cf67187](https://github.com/jacob-delgado/workflow/commit/cf6718797f90be6bc96c2113d3dbe4d92d5f2103))
* **webserver:** reach the branch, check, issue and task error arms ([fb5be70](https://github.com/jacob-delgado/workflow/commit/fb5be70d2e76f9edadd1a539937780efff0f910d))
* **webserver:** reach the faults of people, groups, setup, local data ([43bd536](https://github.com/jacob-delgado/workflow/commit/43bd536805e3d64c10a520e82993a40e14c926f2))
* **webserver:** read a held announcement on a synctest clock ([46d906c](https://github.com/jacob-delgado/workflow/commit/46d906c8d383032814eaad82da5814d2562bfd26))
* **webserver:** run the stream's timing on a synctest clock ([9b7f1de](https://github.com/jacob-delgado/workflow/commit/9b7f1de0489c5b9ad19ea4da1dedb0a371870381))
* **webserver:** split the run tests by what they hold a run to ([0051fa3](https://github.com/jacob-delgado/workflow/commit/0051fa32855dea55351d87ff41d7fea4e1f3289a))
* **webserver:** wait on a run's own signals rather than the clock ([27f46e1](https://github.com/jacob-delgado/workflow/commit/27f46e10e87a792e8678ca02c071137b46de57e7))
* **web:** show no time on a comment the tracker gave no date for ([91e1f6f](https://github.com/jacob-delgado/workflow/commit/91e1f6fa753e52062863e89be149664bc873e88c))
* **web:** show the state before the frame in the undescribed done test ([43c8607](https://github.com/jacob-delgado/workflow/commit/43c8607af3272eb7dd735fdf08eb2634f4a4d5e0))
* **web:** split how the Reviews section reads into a file of its own ([529f5e3](https://github.com/jacob-delgado/workflow/commit/529f5e39f4b29e2339d817cf1b4c042e33f2b980))
* **web:** split how the Tasks section reads into a file of its own ([4a0c86a](https://github.com/jacob-delgado/workflow/commit/4a0c86a4a0c21f35f1cd9259fba6ca18e0a5af04))
* **web:** split the issue card's done cases into a file of their own ([5c405c1](https://github.com/jacob-delgado/workflow/commit/5c405c1f0234ea6a8029d3137220ed8407fe9275))
* **web:** split the issue detail's Try again cases into a file ([7dcb1ec](https://github.com/jacob-delgado/workflow/commit/7dcb1ec65cdc50a65c6ce15a0ad7d566123d849d))
* **web:** split the Issues rows' task marks into a file of their own ([ff77d9f](https://github.com/jacob-delgado/workflow/commit/ff77d9f80c345cd57f72588bbbccd081b4f55992))
* **web:** split the keyboard cases by what they meet ([d78c581](https://github.com/jacob-delgado/workflow/commit/d78c58172ad38b997db254d4829361813c47f019))
* **web:** split the work story's switch and start cases into a file ([a91bef4](https://github.com/jacob-delgado/workflow/commit/a91bef4ca7d342d7de9fbfa71112ba8fbcce599f))
* **web:** wait for the page before the skip link's first Tab ([c1f216e](https://github.com/jacob-delgado/workflow/commit/c1f216eb241d1381cbc78957dd0c16eaf1ef9f87))
* **web:** write what the server describes into each case's data ([ddf36ab](https://github.com/jacob-delgado/workflow/commit/ddf36abc9f8184aff36ad95b50ddbe4f762c5dca))
* **wiring:** give the keychain tests an account of their own ([cf0dbfa](https://github.com/jacob-delgado/workflow/commit/cf0dbfa4b642edaea0ef5548ed58da317cb9c27d))
* **wiring:** hold the fork lock while writing a stand-in script ([f5b3d1b](https://github.com/jacob-delgado/workflow/commit/f5b3d1b4f8762107634f5664ccc2de9657f6e6bd))
* **wiring:** move the template tests to a file of their own ([c35509b](https://github.com/jacob-delgado/workflow/commit/c35509ba1b9b64652591e854e6e6cc3896fe2a28))
* **wiring:** reach the seams' failures and the arms no test ran ([20107ff](https://github.com/jacob-delgado/workflow/commit/20107ff672636f178b107d4bd6d26accfb2da4b9))
* **workdirs:** reach the error arms of listing and checking ([381b6dd](https://github.com/jacob-delgado/workflow/commit/381b6ddb329294bb26a4aa169f11e88676422580))

## [0.8.0](https://github.com/jacob-delgado/workflow/compare/v0.7.0...v0.8.0) (2026-10-07)


### ⚠ BREAKING CHANGES

* **api:** PUT /api/config requires jira.token, forge.token, messaging.client_secret, messaging.refresh_token and messaging.webhook_url in its body; send the masked value or "" to keep a credential, and null to remove it.
* the refusal announce and summary --post print on stderr when Slack will not deliver to the channel no longer ends "then press enter to try again", and its sentences are joined by a semicolon ("you are not in #dev; join it"). A script matching the old text must match the new.
* **cli:** on a GitLab remote, reviews writes each number with "!" rather than "#" on stdout and says "No merge requests are waiting on your review." on stderr, and pr's and announce's refusals say "merge request". A script parsing "#N" from reviews on GitLab must accept "!N".
* announce's "to" line on stdout and its dry-run and "Announced to" lines on stderr name the destination as Target() does: the channel, "(no channel set)", or "the channel its webhook is bound to" in place of "the configured SERVICE channel". A script matching the old phrase must match the new one.
* `workflow config init` with nothing to answer its prompts now exits 2 (usage) instead of 1. A script that ran the guided init unattended should pass --template, or check for status 2.

### Features

* **api:** name the key each action has with no ui.keys entry ([0df3d6b](https://github.com/jacob-delgado/workflow/commit/0df3d6ba85f32b73ea873262bd3bbd14a33fafac))
* **api:** remove a stored credential when a save sends null ([962fe97](https://github.com/jacob-delgado/workflow/commit/962fe97d9fb86d0b9471345e515c78c6eec79ec2))
* name the store's directory in doctor, or why there is none ([f16adef](https://github.com/jacob-delgado/workflow/commit/f16adeff534bfb07e64ca806435e2476f89e9360))
* say what a slow command is reading while it waits ([c580b7e](https://github.com/jacob-delgado/workflow/commit/c580b7e3bd8b31115d2f3e426e39d24ebfdbcbba))
* **tui:** edit every section of the configuration in Settings ([79f23ce](https://github.com/jacob-delgado/workflow/commit/79f23ce0864a61ac6ee0ea4b417d86ce14714c6c))
* **tui:** jump to a list's ends with home, end and G ([46e61c6](https://github.com/jacob-delgado/workflow/commit/46e61c63e61278bfeaa59610ae488d477b7eeca8))
* **tui:** remove a stored credential from Settings ([fff426b](https://github.com/jacob-delgado/workflow/commit/fff426b5c90a3a82ceea0fcf320994d10875f4aa))
* **web:** borrow the terminal's rename, header count and Copy URL ([023a046](https://github.com/jacob-delgado/workflow/commit/023a046820c9660a7f2110126ec7902b56e499ac))
* **web:** edit every section of the configuration in Settings ([7be1b7c](https://github.com/jacob-delgado/workflow/commit/7be1b7c917f8d0882e24cf11a847d46b0a3df0a9))
* **web:** mark what the stream just changed, and when CI settles ([8ed5acc](https://github.com/jacob-delgado/workflow/commit/8ed5accba7c514f3d945f5f3b59bd08d4b209407))
* **web:** remove a stored credential from Settings ([4aadf8c](https://github.com/jacob-delgado/workflow/commit/4aadf8c5543585127bfde850d4049da1364e2aea))


### Bug Fixes

* **cli:** word pr, announce and reviews in the forge's own vocabulary ([e9f6c39](https://github.com/jacob-delgado/workflow/commit/e9f6c39e4bd2507b79863b3af191304e7676da78))
* **config:** refuse jira.headers and prefixes that name one thing twice ([aeb6b78](https://github.com/jacob-delgado/workflow/commit/aeb6b782b27598c2c69edcec36ff4070160d4368))
* end pr by naming announce when messaging is set up ([d7b4d97](https://github.com/jacob-delgado/workflow/commit/d7b4d976785031d11c166121d86b902934b246cb))
* keep the progress note off a terminal that cannot erase ([10e707f](https://github.com/jacob-delgado/workflow/commit/10e707fa07c613b78885d905e08a3c9f9e89016a))
* name announce's destination in the words the interface uses ([3efa6f0](https://github.com/jacob-delgado/workflow/commit/3efa6f00def1638f3284729ae6fb7d74f4864cb9))
* point config init with no terminal at --template ([a4f8d86](https://github.com/jacob-delgado/workflow/commit/a4f8d86801dedc34a1518d6667d6a1aa15f1af50))
* **settings:** refuse a header or prefix the terminal already lists ([b7f804e](https://github.com/jacob-delgado/workflow/commit/b7f804edaec05b79cdb9a492f7a52cb582448e40))
* **settings:** refuse a web header or prefix named as an earlier row ([d96ecf5](https://github.com/jacob-delgado/workflow/commit/d96ecf5996cf742170f1061fe2ff90f037880e74))
* **settings:** say where the token, channel and announcement apply ([de3b929](https://github.com/jacob-delgado/workflow/commit/de3b92977e49a197f249304c540ff229d5595ede))
* **settings:** show the default a title source or service leaves empty ([bcef5cf](https://github.com/jacob-delgado/workflow/commit/bcef5cf3aead9cf07a824cfbb71b1b93edfbb915))
* show a date field's shape as YYYY-MM-DD in hint and refusal ([0083174](https://github.com/jacob-delgado/workflow/commit/0083174663c62931d12c966a068c0c79e3702326))
* tell a channel's refusal as its fix, on every surface ([3481985](https://github.com/jacob-delgado/workflow/commit/3481985a16f9b7a03291f76a97951a6d94660284))
* tell the dirty-tree refusal in one sentence on every surface ([79e221b](https://github.com/jacob-delgado/workflow/commit/79e221bcc0c64494af4e2d0f018bc859609ae49c))
* **tui:** draw a chosen row as a checkbox, not a status ([31e89c5](https://github.com/jacob-delgado/workflow/commit/31e89c5aaba4d52b13a980c4785995f78067ab66))
* **tui:** mark a draft review request on its row in the queue ([286fb93](https://github.com/jacob-delgado/workflow/commit/286fb9340647d01fb7959232b7d077345776769a))
* **tui:** mark every pane's title in flight while its read is out ([c1bcd29](https://github.com/jacob-delgado/workflow/commit/c1bcd298e73a769c55cf77d14be3ee8ad7035238))
* **tui:** name the bound key in every sentence that offers one ([0ebb06c](https://github.com/jacob-delgado/workflow/commit/0ebb06c974b4010287f149d3ab51d693b90e7921))
* **tui:** offer Start after a worktree, and name the issue on review ([6ea6771](https://github.com/jacob-delgado/workflow/commit/6ea6771b6a0298e0d650bd16286d639ce26ff5e1))
* **tui:** say a refused keymap whole, and mark only moved keys edited ([1034458](https://github.com/jacob-delgado/workflow/commit/103445883c959149d29d5643a211831a9d93b2e4))
* **tui:** tell unset messaging in the shared set-up advice ([482e0fd](https://github.com/jacob-delgado/workflow/commit/482e0fd0a37ee6b932ed8c8013d459135c6df530))
* **tui:** word the review queue and editor help in the forge's nouns ([d5aeb3e](https://github.com/jacob-delgado/workflow/commit/d5aeb3e046d24c0010cb10f19a1e3e5a7905b13f))
* **web:** announce the server's address only once it holds the port ([068712b](https://github.com/jacob-delgado/workflow/commit/068712b049dfa37871357c337b7dc5b88f709be9))
* **web:** confirm Copy URL inside the queue row it copied from ([81ec360](https://github.com/jacob-delgado/workflow/commit/81ec360b8bde39be4e06f0773989801fc01e3bab))
* **web:** count in words, and say each count in one form ([814a5ef](https://github.com/jacob-delgado/workflow/commit/814a5ef05607b351bba353a01f5f8d22fc2bbf51))
* **web:** draw the key table only while Rebind keys is open ([41bd8a7](https://github.com/jacob-delgado/workflow/commit/41bd8a7e3513f7b31a863ca2d9b1fea067d2c623))
* **web:** keep every issue row at one right edge ([1c928da](https://github.com/jacob-delgado/workflow/commit/1c928da158dfcc4de84ff374ba4901fe387f4283))
* **web:** keep focus on a control held while its request runs ([7a902f1](https://github.com/jacob-delgado/workflow/commit/7a902f1cc5b126ac4f51a7529715a38d1dc2750f))
* **web:** mark how much of each file is staged with a StateMark ([42911ab](https://github.com/jacob-delgado/workflow/commit/42911aba05eb2fb00d66db29b6d6c92152c61545))
* **web:** mark the Review rows' states and a queued draft ([58a9a7e](https://github.com/jacob-delgado/workflow/commit/58a9a7e65e619b9a8063053e60092532e12ab3cf))
* **web:** one case for placeholders, and words that match the state ([8ca3f85](https://github.com/jacob-delgado/workflow/commit/8ca3f85165d6869cadf6cfa5bd37aa2b82df7787))
* **web:** point a detached HEAD at Issues for a way out ([1f8f284](https://github.com/jacob-delgado/workflow/commit/1f8f28422145368685a64dee311f79bccd162309))
* **web:** say a clean working tree under its heading ([5fba456](https://github.com/jacob-delgado/workflow/commit/5fba4569f4c390aea1c45fcb0bd1262ccfff3328))
* **web:** say a failed configuration read as a failure ([b006894](https://github.com/jacob-delgado/workflow/commit/b006894792142911ade813b30fd0203dbfe43b95))
* **web:** say a stage not done, and a value missing, one way each ([6041a8e](https://github.com/jacob-delgado/workflow/commit/6041a8e0fcf5b37266a53bd70aa75ffbfeeb5502))
* **web:** say the empty review queue as the terminal and reviews do ([67c0d8f](https://github.com/jacob-delgado/workflow/commit/67c0d8f087977507e4c99eb7485e28f147514b7f))
* **web:** say what forge links, story stages and two controls do ([14d6639](https://github.com/jacob-delgado/workflow/commit/14d663972eabee866892b3047485d32fe8d4a25a))
* **web:** set every Review value at one left edge ([c4081e6](https://github.com/jacob-delgado/workflow/commit/c4081e693254d0ae0591d4b8374990509d730e83))
* **web:** set every section's content at one named measure ([86b5615](https://github.com/jacob-delgado/workflow/commit/86b561516043fe8d93fd3c0b3de758d050307379))
* **web:** set story branch names in the code face, rows at text-sm ([4a7dd03](https://github.com/jacob-delgado/workflow/commit/4a7dd033ed4c20f7491e49ec2ac97fcb1bcac58a))
* **web:** word a section's wait by the stream's state, as the header ([c32c52c](https://github.com/jacob-delgado/workflow/commit/c32c52cefe80931620d0b48f8514edd4d7938572))
* **web:** write a date as the terminal does, YYYY-MM-DD ([9288db0](https://github.com/jacob-delgado/workflow/commit/9288db012e3fdf54d0142b49979c19b62de46d4d))


### Documentation

* lead scriptable commands' help with examples and exit statuses ([2ac289e](https://github.com/jacob-delgado/workflow/commit/2ac289eaab4babac8649fa04af0771aa5ccd6fba))
* list pr's pointer to announce among its stderr lines ([5980d2f](https://github.com/jacob-delgado/workflow/commit/5980d2fc32924b53970d733a35211bec2e5ef168))
* say what config init does, and who ignores the file ([6c77fac](https://github.com/jacob-delgado/workflow/commit/6c77fac632c95f19ac5e6eed7857f958e297d7cc))
* **ux:** close the command line entries the branch fixed ([70f2646](https://github.com/jacob-delgado/workflow/commit/70f2646e07bf923a9e43601a79f08afcfca66db0))
* **ux:** close the Settings entries the branch fixed ([7690447](https://github.com/jacob-delgado/workflow/commit/769044747aa1016a5394b3d065ec9f06a91efa77))
* **ux:** close the terminal entries the branch fixed ([8f91cba](https://github.com/jacob-delgado/workflow/commit/8f91cbaadd6f94d64ed8d0446585f8e086b8c34c))
* **ux:** close the web focus and link entries the branch fixed ([4110611](https://github.com/jacob-delgado/workflow/commit/4110611168c4c47ceb526b9fa59695d7f8564f1d))
* **ux:** close the web layout entries the branch fixed ([b82e88e](https://github.com/jacob-delgado/workflow/commit/b82e88e3554e27d7693d59a2538760ca6c4b2a72))
* **ux:** close the web liveness entries the branch fixed ([1446c35](https://github.com/jacob-delgado/workflow/commit/1446c3536e0d8776dca491c50a54ec4e2c175d94))
* **ux:** close the wording entries the branch fixed ([481d15a](https://github.com/jacob-delgado/workflow/commit/481d15afbe6b9d4e6f0df2565711ab5135c1a5e4))


### Build & Packaging

* Bump smol-toml from 1.8.0 to 1.9.0 in /web ([b0e6d6d](https://github.com/jacob-delgado/workflow/commit/b0e6d6d99bc5e88a2ab827524cf657d54c5a5349))
* Bump the go group with 2 updates ([d63fe8d](https://github.com/jacob-delgado/workflow/commit/d63fe8d873b426047a6e62e51670a9b84130db97))
* Bump the web group across 1 directory with 11 updates ([b9ab000](https://github.com/jacob-delgado/workflow/commit/b9ab000294003896932469bbe352748a1aa4b6cf))


### CI

* Bump jdx/mise-action from 4.3.0 to 5.0.1 ([0b949b8](https://github.com/jacob-delgado/workflow/commit/0b949b8d7c32e60faea8784c69b7b9e5ac81b703))


### Tests

* **cli:** wait for the web server to start, not for five seconds ([cf1d17a](https://github.com/jacob-delgado/workflow/commit/cf1d17a84ecdc4978d5fb481eb833d115dcd0cfa))
* **settings:** cover editing and removing entries in the terminal ([7d9053a](https://github.com/jacob-delgado/workflow/commit/7d9053a8149eae38302509fb28071cbb746e3103))
* **settings:** cover the refresh token's removal and default keys ([48c12a8](https://github.com/jacob-delgado/workflow/commit/48c12a80525f0d6936c13363faff3c0afdae0b13))
* **settings:** hold Jira headers and the client secret to KeepStored ([34519f3](https://github.com/jacob-delgado/workflow/commit/34519f3f6b21cae1490d10b9f2d6dddc7de73f93))
* **web:** count a disclosure's summary as a Tab stop in the walk ([0542f9e](https://github.com/jacob-delgado/workflow/commit/0542f9e647a75b50f9fc6fd1a257c5818cb4f1b2))
* **web:** say when Tab leaves a focus-ring check off its control ([9894e7d](https://github.com/jacob-delgado/workflow/commit/9894e7defc859847a2b4bfaf1e91aef63773e323))
* **web:** walk the Repositories section once its picker has drawn ([b24dc2e](https://github.com/jacob-delgado/workflow/commit/b24dc2e24e1d1afe55c223b75fe482d915bfed3e))

## [0.7.0](https://github.com/jacob-delgado/workflow/compare/v0.6.0...v0.7.0) (2026-10-06)


### ⚠ BREAKING CHANGES

* GET /api/activity and summary --json replace each source's boolean failed with state (read, not_set_up or failed); read state == "failed" where failed was read. summary exits 0 when the only sources not read are not set up. An error answer for a missing Jira or forge token, a remote naming no forge, no Taskwarrior or no git user.email has code not_set_up instead of unprocessable.
* **cli:** workflow standup is removed, and its --days and --no-edit with it. Run `workflow summary --post` instead (add --yes to post unattended, and --from YYYY-MM-DD for a period other than the previous working day); `workflow summary --json` gives a script the same summary as data.
* **cli:** `workflow pr` prints a blank line and the pull request's body after `BRANCH → BASE` on stdout; a script that read stdout line by line should read `pr --json` instead.
* **tui:** worktree defaults to ctrl+g and toggle-breaking to ctrl+x. A ui.keys map that binds one of those seven actions to a letter, digit, symbol, space, or to ctrl+a, b, d, e, f, h, k, u, v, w, n or p is now refused; move it to another control key.
* **tui:** ui.keys action names follow the shown verbs. Rename filter to search-issues, filter-place to filter-issues, filter-tasks (the / search) to search-tasks, narrow-tasks to filter-tasks, switch-task to switch-branch, complete-task to mark-done and branch-for-issue to start-work. A map that still names filter-tasks now moves the Tasks checklist rather than the search. The default for filter-issues moves from p to f and for sort-reviews from s to O.
* **cli:** `workflow branch` prints its preview on stdout as "Start work on KEY: create NAME from BASE and switch to it", not "Branch NAME from BASE and switch to it". A script matching the old first line must match the new one; the "Created NAME" line is unchanged.
* **cli:** `workflow slack login` prints nothing on stdout; its dry-run line and "Logged in to Slack as ..." are on stderr. `workflow db-clean` prints "Nothing to remove." and "Removed ..." on stderr; only the listing stays on stdout. A script reading those lines from stdout must read stderr.
* **cli:** `workflow reviews --sort` with an unknown order now exits 2, not 1; `workflow slack login` exits 3, not 1, with no .workflow.json, over a Slack webhook or with another messaging.kind, and 2, not 1, when an answer is left blank. A script that tested for 1 must test for these statuses.

### Features

* **cli:** comment on an issue with the text on stdin ([8b29095](https://github.com/jacob-delgado/workflow/commit/8b2909569d7599a179920f87e84c152c9ccc9e7f))
* **cli:** list where workflow works with repositories --json ([d9ca584](https://github.com/jacob-delgado/workflow/commit/d9ca584c63c75992323bca5fed577ce19df26da3))
* **cli:** print what pr opened as JSON ([841fc4d](https://github.com/jacob-delgado/workflow/commit/841fc4d68b384de705826208cd4c7a61b53d5eef))
* **cli:** replace standup with summary, read as the Summary pane reads ([fd4ea6c](https://github.com/jacob-delgado/workflow/commit/fd4ea6c8744a7e4aba880d0ad47f94234d49987a))
* **cli:** say "start work" in branch's preview and help ([b15bbff](https://github.com/jacob-delgado/workflow/commit/b15bbff334149fb99e343e92f37499b31855a673))
* **cli:** start work from a fresh base or in a worktree ([e2e4788](https://github.com/jacob-delgado/workflow/commit/e2e4788857f75c992eeeaee49437ebdd9a66756d))
* **gitrepo:** discard one file's changes ([a40890a](https://github.com/jacob-delgado/workflow/commit/a40890aa1bae05444b22af9793bef2d68d5c16a5))
* **loop:** post the Summary rendered for its service ([823dc4d](https://github.com/jacob-delgado/workflow/commit/823dc4da6c1c0309e79d90cb4d8e7444b8f53618))
* **messaging:** render Markdown for the service it is posted to ([b485827](https://github.com/jacob-delgado/workflow/commit/b48582792134cffa0de57ac16be8ad61db7c38e4))
* tell a service not set up apart from one that refused ([7a28280](https://github.com/jacob-delgado/workflow/commit/7a282802c4d10731b31db4ce957596d3b447070c))
* **tui:** give each shared verb one key and one name ([db7efaa](https://github.com/jacob-delgado/workflow/commit/db7efaa920ff83f298b3f29fda577729b464f819))
* **tui:** keep one verb from an act's key to its notice ([6e13c57](https://github.com/jacob-delgado/workflow/commit/6e13c57be02deb16bb8f0886b1ebf831e3692801))
* **tui:** list and remove the local data from the Repositories pane ([3c13906](https://github.com/jacob-delgado/workflow/commit/3c13906325dcfd4a99a15e11dc20d3ae07b11e59))
* **tui:** post the Summary from its pane, after a preview ([89a6a70](https://github.com/jacob-delgado/workflow/commit/89a6a70e1ed8fbaa5dbfb4c0a696b96a707ca1d9))
* **tui:** read and change the configuration in Settings ([1094a13](https://github.com/jacob-delgado/workflow/commit/1094a133e40d02e32866277ada4abf60ca591cc9))
* **tui:** set up a first configuration file from inside the interface ([60b2e3e](https://github.com/jacob-delgado/workflow/commit/60b2e3ec8f4bd1a921d02b6b79c32ed337f7a2ce))
* **tui:** unstage everything, and discard a file's changes ([a3e3e75](https://github.com/jacob-delgado/workflow/commit/a3e3e7510bfc28545a80987345ae9df86e61be9e))
* **web:** answer the terminal's keys, with a ? sheet and a palette ([301f51d](https://github.com/jacob-delgado/workflow/commit/301f51d6a4f57130fd4d7ad006d7641d2a32b996))
* **web:** change an issue's status, assign it and log work ([8c40335](https://github.com/jacob-delgado/workflow/commit/8c4033590420ae840296cd1a8b9bfdead3ab9d4b))
* **web:** edit the announcement, and announce when CI passes ([150f069](https://github.com/jacob-delgado/workflow/commit/150f0690637bc36995e1527a03fef5f4ae351d97))
* **web:** edit, merge, finish and re-run the pull request ([79d1f3a](https://github.com/jacob-delgado/workflow/commit/79d1f3a367312c95888607c6bbf17ae88d5fb4c1))
* **web:** fetch before starting work, with a way out when it fails ([e59a950](https://github.com/jacob-delgado/workflow/commit/e59a950d4f8c0f2a80320a91034b74240c8d50f3))
* **web:** know an announcement was already made ([9e4f74f](https://github.com/jacob-delgado/workflow/commit/9e4f74fcff684f8bd1553c3bc216a37d7669d5bc))
* **web:** post the Summary from its section, after a preview ([fc038a1](https://github.com/jacob-delgado/workflow/commit/fc038a1c4dbe03e1d6fb9ea10ce192b55b8113da))
* **web:** read a changed file's diff ([3966162](https://github.com/jacob-delgado/workflow/commit/39661622448ebd9486d1a05d902db6fb4e5e6427))
* **web:** run pre-commit, rebase, amend and fix up, streamed ([f046cf3](https://github.com/jacob-delgado/workflow/commit/f046cf35f65bd7e2a9db319a58f1748882f06750))
* **web:** serve the terminal's key actions to the web ([2292a85](https://github.com/jacob-delgado/workflow/commit/2292a859b90ceb31f13e1027ceb2614c4c04c276))
* **webserver:** carry each panel's failed read beside it on the stream ([06660a6](https://github.com/jacob-delgado/workflow/commit/06660a6bb88a8e95c5e8a5bf2869118ca0540c0d))
* **webserver:** discard a changed file's changes ([651cd34](https://github.com/jacob-delgado/workflow/commit/651cd347f163eb8e0c74760afb699723037af456))
* **webserver:** post the Summary through POST /api/activity/post ([3542aaf](https://github.com/jacob-delgado/workflow/commit/3542aafca30a6742c4b167f91e040446d3381df8))
* **web:** set up a first configuration file from Settings ([fbeb627](https://github.com/jacob-delgado/workflow/commit/fbeb62780f280704d15b115a7115487eecb29383))
* **web:** set up lefthook for the hooks it does not manage ([8a7e5af](https://github.com/jacob-delgado/workflow/commit/8a7e5af9919536232deba2b4a6515d65a7e82222))
* **web:** unstage all, and discard a file after a last look ([164d394](https://github.com/jacob-delgado/workflow/commit/164d394163ee16bd0f230dfd30c3104dde750601))


### Bug Fixes

* **activity:** escape each title's Markdown in a summary's text ([c30b846](https://github.com/jacob-delgado/workflow/commit/c30b846c437166027d03c85b3fd393e85976e184))
* **cli:** end notices with a period and hints after a semicolon ([25e16b6](https://github.com/jacob-delgado/workflow/commit/25e16b62b469d3aa26c4c0ff50123c6a34baf52c))
* **cli:** exit 2 and 3 for refusals that exited 1 ([96e95c0](https://github.com/jacob-delgado/workflow/commit/96e95c04163b2a05b7df7eb90473e5eb78618721))
* **cli:** give branch's existing-branch hint after a semicolon ([9eab119](https://github.com/jacob-delgado/workflow/commit/9eab119dad71ebc366ad685f6a75e7c9a4bb40fc))
* **cli:** give every command's help one voice ([17229c2](https://github.com/jacob-delgado/workflow/commit/17229c21c91579a34059c0058ad29543e3d2c137))
* **cli:** name each service status could not read on stderr ([e64aa58](https://github.com/jacob-delgado/workflow/commit/e64aa58bcbd89b0700d9914edfd0f18b1643f0d3))
* **cli:** name what to do when slack login has no terminal ([01abdb9](https://github.com/jacob-delgado/workflow/commit/01abdb96eda4df61a43dfbf655c39de571d3d321))
* **cli:** print where summary --post goes on stderr, not stdout ([2ba322d](https://github.com/jacob-delgado/workflow/commit/2ba322d7e93e4cb9159300ec65e7745f58b97e8f))
* **cli:** say on stderr which service status found not set up ([e954568](https://github.com/jacob-delgado/workflow/commit/e95456809634ec548144caa622864ceb8a4959fe))
* **cli:** show the whole pull request before asking to open it ([e6a0861](https://github.com/jacob-delgado/workflow/commit/e6a08612539a2b108e13dad5893bb6bdcb3fae4a))
* **cli:** summarize db-clean as removing workflow's local data ([2108e10](https://github.com/jacob-delgado/workflow/commit/2108e10771ee1ee84f3a562f5b8a868b9b3e1239))
* **cli:** write slack login's and db-clean's notices to stderr ([8bd967a](https://github.com/jacob-delgado/workflow/commit/8bd967a27375e42ddb7e61559ff365971deafc40))
* **messaging:** refuse a Summary too long for its service before it goes ([af1462e](https://github.com/jacob-delgado/workflow/commit/af1462edef10f59dc1a485c79ebf9d1f69a9b18a))
* **setup:** never keep an address that is no Jira address ([3b485be](https://github.com/jacob-delgado/workflow/commit/3b485be4e433d49b5b46f319e009b4eca4bedbb7))
* **setup:** offer no home place where no home directory is known ([ad8c508](https://github.com/jacob-delgado/workflow/commit/ad8c508f74151f1a2c94301f6a0a42a9baea7321))
* **setup:** refuse a link where the new configuration file goes ([1e19a65](https://github.com/jacob-delgado/workflow/commit/1e19a658a68d1b9f6346d65fc37def32e1aaec5e))
* tell messaging with no credential as not set up, like the rest ([7092186](https://github.com/jacob-delgado/workflow/commit/7092186975a04f97387c24914c7c914e49643d64))
* **tui:** ask before a write that leaves or cannot be taken back ([918264c](https://github.com/jacob-delgado/workflow/commit/918264c3944b7af22226563bed4ff10a48f098ba))
* **tui:** cut a long Repositories path in its middle ([2c946f2](https://github.com/jacob-delgado/workflow/commit/2c946f227b22eb42d60410986443b7e288039c25))
* **tui:** keep focus, room and the way out at 80 by 24 ([c2d2ae6](https://github.com/jacob-delgado/workflow/commit/c2d2ae6b7a5c2722589f0d3a840df57c0aab1788))
* **tui:** keep Taskwarrior's and the service's own case in the help ([deb6fe6](https://github.com/jacob-delgado/workflow/commit/deb6fe6a1c68b95d2fcdaac10109cc2fdb6bc937))
* **tui:** label esc by what it does to the work ([f61e572](https://github.com/jacob-delgado/workflow/commit/f61e572da19e4da54b256b93aab38226d5c06885))
* **tui:** leave a text field's editing keys to the field ([0a1c6f4](https://github.com/jacob-delgado/workflow/commit/0a1c6f4db97222aecee04371e20564761db2321f))
* **tui:** list only the places setup offers in the first run's form ([e4a6068](https://github.com/jacob-delgado/workflow/commit/e4a60684244dd721b59bce5e484ef3fccf94f11d))
* **tui:** move the calendar's day grid by day and by week ([0f61d59](https://github.com/jacob-delgado/workflow/commit/0f61d59478a4c48ee4fe3b9cf6ab237e1cb0cac8))
* **tui:** reopen workflow with what Settings saved ([4e1ae82](https://github.com/jacob-delgado/workflow/commit/4e1ae822bc83002615eab759c685ce4cff2863cd))
* **tui:** tell what is not set up as guidance, not as a failure ([cf3e83b](https://github.com/jacob-delgado/workflow/commit/cf3e83bbb1d2b1aca609baa83ae276a57f32d0d0))
* **tui:** unlink an issue from a branch in the link form ([4038f1f](https://github.com/jacob-delgado/workflow/commit/4038f1fbc79169c2613f3a1aa9f7f33bb7a1e96b))
* **tui:** word the empty states by the brief-and-full rule ([4c2926f](https://github.com/jacob-delgado/workflow/commit/4c2926fe9022ea82d3fc99a3f7accd8f88ad3454))
* **web:** announce every failed read and write as an alert ([16d3f1d](https://github.com/jacob-delgado/workflow/commit/16d3f1d76a6ab57efdca934bc24e1a7d72bbcb1b))
* **web:** ask before marking done, undoing, syncing, or switching ([02e54c9](https://github.com/jacob-delgado/workflow/commit/02e54c9792d8013f64ccd4854cee0f7d8655053e))
* **web:** bind post-summary to the Summary section's Post… ([6def0fd](https://github.com/jacob-delgado/workflow/commit/6def0fd05801ea38b94f7e8082b02038a5fd5c9f))
* **web:** draw a row's facts as elements apart, not a dotted string ([0c07b97](https://github.com/jacob-delgado/workflow/commit/0c07b976da4cdfbeed7fd6e6800625665a7326f3))
* **web:** draw every field and button through web/src/lib ([eec3091](https://github.com/jacob-delgado/workflow/commit/eec309196a4371366e9c20c87bf96b42294d942e))
* **web:** draw the comment box once, in its settled shape ([61a2afb](https://github.com/jacob-delgado/workflow/commit/61a2afb8d4934c01332cbe666854a52146433da4))
* **web:** give a text field a boundary that holds 3:1 ([564ce52](https://github.com/jacob-delgado/workflow/commit/564ce528df740f69c498db76fdcffe1b1185e85a))
* **web:** head CI by its state when the forge counts no checks ([427a4ae](https://github.com/jacob-delgado/workflow/commit/427a4ae4d5e3a7437c78b8ba0e05623a4d090518))
* **web:** hold single-key shortcuts while a confirm is open ([b0e8e79](https://github.com/jacob-delgado/workflow/commit/b0e8e79141dc09b69b030a708fdfee2b8bb873e8))
* **web:** keep one name per action and per concept, as UX.md names them ([69ef3ed](https://github.com/jacob-delgado/workflow/commit/69ef3ed42947e3827c7a99f295e0334e37d0841d))
* **web:** keep the push confirm's buttons at their own heights ([c612b7c](https://github.com/jacob-delgado/workflow/commit/c612b7c3126e41488d32a7ab0cb72d833321b32b))
* **web:** leave Control+K to a Mac's text fields ([fb85cc3](https://github.com/jacob-delgado/workflow/commit/fb85cc316d4a0a2c572b9fbf5c7ab5480d65b251))
* **web:** name the push's branch and remote, and keep it off the base ([a783de8](https://github.com/jacob-delgado/workflow/commit/a783de8fe532c1132ca2d5fc90ebc6c6c65b9138))
* **web:** say every read in flight as one Reading status line ([9c1a95c](https://github.com/jacob-delgado/workflow/commit/9c1a95c3703791adc262af9678c28bb5981086ea))
* **web:** say no checks reported in the work story, not CI none ([12672e5](https://github.com/jacob-delgado/workflow/commit/12672e541543444447ec14780cd2b9f5e256b30b))
* **webserver:** run one first-run setup at a time, taking up a late file ([31699d1](https://github.com/jacob-delgado/workflow/commit/31699d12a9ea21a0e859e0b4427bc3c906c67f29))
* **web:** set a primary button's focus ring apart from its fill ([19d0adf](https://github.com/jacob-delgado/workflow/commit/19d0adf22181738f8bdca517e9376566bac16fb2))
* **web:** show a section's failed read as a failure, not its empty state ([316ccd3](https://github.com/jacob-delgado/workflow/commit/316ccd324194f91e12779ce5506018b1741b2ca9))
* **web:** wrap the push confirm's buttons under a long branch name ([672a1ed](https://github.com/jacob-delgado/workflow/commit/672a1ed9133338846ebf96524a12c5337b00468b))
* **web:** write every date through one formatter, in one locale ([15bd35b](https://github.com/jacob-delgado/workflow/commit/15bd35ba5e2f49cc783cd2488e84484b3794d209))
* word a refused post without calling it an announcement ([db90d04](https://github.com/jacob-delgado/workflow/commit/db90d046b763c781636470f716f7e31378500d9b))


### Refactors

* **api:** name the local data's removal remove, as the screen does ([d0583dd](https://github.com/jacob-delgado/workflow/commit/d0583dd6816a15c96d487023f144907a1562c226))
* **cli:** move the guided init's composition into internal/setup ([7f09637](https://github.com/jacob-delgado/workflow/commit/7f0963734b1027b4e33094addcb78376f5e95f81))
* **config:** share the save the web's Settings makes ([9a16534](https://github.com/jacob-delgado/workflow/commit/9a1653474f9d17986c5c7b5f19ec82954389240f))
* **jira:** check a transition's field values in one place ([9b02d57](https://github.com/jacob-delgado/workflow/commit/9b02d57d7e89b81bb209409ad7c3696e78d00961))
* **loop:** ask the Summary's sources through one SummaryReads ([992a26d](https://github.com/jacob-delgado/workflow/commit/992a26d607f07b12723f812d698c4540c64b0395))
* **loop:** decide when a merge, finish, re-run or rebase can go ([07223a1](https://github.com/jacob-delgado/workflow/commit/07223a138a70b26d77bcde50792d731facc97a4a))
* **loop:** word how to set up what is missing in one place ([0d05813](https://github.com/jacob-delgado/workflow/commit/0d058136de9c0b2afad375c95cf18a2d5fcd84cf))
* **tui:** assert discarded satisfies applier at compile time ([427ed42](https://github.com/jacob-delgado/workflow/commit/427ed4249b16f613e46b218938cd3f78411d0b2c))
* **webserver:** share the period asked and the Activity shape ([1d46ad2](https://github.com/jacob-delgado/workflow/commit/1d46ad2e9ae8bf1d174608c31488746137d15e6e))


### Documentation

* **budgets:** name the pull request the new budget rows came in ([bfe56e9](https://github.com/jacob-delgado/workflow/commit/bfe56e937026066e0c0c8b34e0d082b93c3e213e))
* call typed narrowing a search where the docs still said filter ([1879176](https://github.com/jacob-delgado/workflow/commit/18791769d83219904f059c858d9a83bd084798e4))
* **features:** add one summary for every surface, and undo ([d46af64](https://github.com/jacob-delgado/workflow/commit/d46af64618d3984deba1d33d7e9a2dd16f233846))
* **usage:** draw the screen with all nine panes, and number them ([170a76d](https://github.com/jacob-delgado/workflow/commit/170a76d590513485b3b27b0fb98d83621fd50fd4))
* **ux:** close the parity and docs entries the branch fixed ([5fdd7d1](https://github.com/jacob-delgado/workflow/commit/5fdd7d1b03bf47831e0ae59ea4856494ffeb5382))
* **ux:** drop standup from UX.md and FEATURES.md now summary ships ([b278895](https://github.com/jacob-delgado/workflow/commit/b278895d2ecda1a24f558d3922bae8b42dc39469))
* **ux:** drop the command line and terminal entries now fixed ([0e464d2](https://github.com/jacob-delgado/workflow/commit/0e464d2137788057445b0771b7498fd19b9dba8e))
* **ux:** drop the entries the small fixes closed ([47340d1](https://github.com/jacob-delgado/workflow/commit/47340d1fd12357dc6168ab9fcb4e196a07994262))
* **ux:** drop UX-128 now failed reads and missing setup each say so ([150f2df](https://github.com/jacob-delgado/workflow/commit/150f2df40290707c95cf059db85e886020799a3d))
* **ux:** drop UX-145 now every write keeps the confirmation rule ([ba3bdde](https://github.com/jacob-delgado/workflow/commit/ba3bddee33f7683e1bbad321cda912989b3661e7))
* **ux:** drop UX-150 now the terminal edits settings and local data ([008a27f](https://github.com/jacob-delgado/workflow/commit/008a27fa1c001b4e7c77405f19abbbb79d5d86a9))
* **ux:** drop UX-152 now the web answers the terminal's keys ([be05c71](https://github.com/jacob-delgado/workflow/commit/be05c71b8ddd2a521c1a469f3d5c67d477fd2aca))
* **ux:** drop UX-153 now both interfaces set up a first file ([a5c92ea](https://github.com/jacob-delgado/workflow/commit/a5c92ea25c00fce4cca15bf972bc29c76f4478f3))
* **ux:** re-check every open entry after the web and parity fixes ([bb6c05d](https://github.com/jacob-delgado/workflow/commit/bb6c05d204c3ec91bd22ac2a3cd1552a2bd0a201))
* **ux:** re-check UX.md after the Summary, Repositories and worktrees ([215ed88](https://github.com/jacob-delgado/workflow/commit/215ed884cd13fd5664397e04a38a9eb8579414e9))
* **ux:** re-check UX.md and FEATURES.md at this branch's head ([d0c06e1](https://github.com/jacob-delgado/workflow/commit/d0c06e10046ae347cc1144e3585d319521d0bec0))
* **ux:** strike the web's issue and messaging gaps from UX-149 ([971ce41](https://github.com/jacob-delgado/workflow/commit/971ce4121cd441ff16ab04209b5363369884c018))
* **web:** what the web now does that only the terminal did ([c89278e](https://github.com/jacob-delgado/workflow/commit/c89278efa00bec877dd6a159da18c79b53df074b))


### Tests

* **cli:** cover comment, repositories and pr --json edges ([39fa03d](https://github.com/jacob-delgado/workflow/commit/39fa03d4ebabe66cb45778281021f3ff8c2fbbd0))
* **cli:** pin summary --post naming its service when the post fails ([d1af7f9](https://github.com/jacob-delgado/workflow/commit/d1af7f928c16bcf67cb85eff9410bbb117d0616b))
* **config:** cover keeping and placing secrets on a settings save ([6e272de](https://github.com/jacob-delgado/workflow/commit/6e272def1f3e8bec8a4d8e503d617ad5b59a28c3))
* cover the first run's refusals, steps and places ([9b8cac9](https://github.com/jacob-delgado/workflow/commit/9b8cac935077f623b09b49e0c065eb57f0cf20ff))
* see every new condition of the Summary's reads and post both ways ([444fbcc](https://github.com/jacob-delgado/workflow/commit/444fbccc6f8348715b195961bd303c6a396e3669))
* **tui:** cover Settings and Local data while they read or save ([83a7833](https://github.com/jacob-delgado/workflow/commit/83a7833a04356236829cec3d73a2b6c22d4caa57))
* **tui:** see every way Settings, Local data and unlink can go ([1fc1ab4](https://github.com/jacob-delgado/workflow/commit/1fc1ab4a29db6b5704bb699dfe12d7b852a01a89))
* **web:** cover the Branch and Review sections' new reads and writes ([db61147](https://github.com/jacob-delgado/workflow/commit/db611470cb4259e25fbad641b2ac5cb0f8a573ab))
* **web:** cover the issue forms' and held announcement's edges ([a39b79e](https://github.com/jacob-delgado/workflow/commit/a39b79e1a20889bcf4e10c39605b157b8af9203c))
* **web:** give the nine-section layout walk a slow test's budget ([ce4ba7f](https://github.com/jacob-delgado/workflow/commit/ce4ba7f2c84a07ab57f5f1b087c524121bf3ed24))
* **webserver:** cover the held announcement's and issue forms' edges ([344ced9](https://github.com/jacob-delgado/workflow/commit/344ced9ef3cab2974a552cde6af41f84343040eb))
* **webserver:** hold an announcement through failed timer reads ([c34d2b3](https://github.com/jacob-delgado/workflow/commit/c34d2b3c3ef2ffe04ea86998131a8f78a47cb6ef))
* **webserver:** see each of the new handlers' conditions both ways ([28df0e8](https://github.com/jacob-delgado/workflow/commit/28df0e8ef88826dd433e6826acd46de0353fc783))
* **web:** settle a section only once no read of its own is in flight ([93bb8d1](https://github.com/jacob-delgado/workflow/commit/93bb8d13ed7c96e026c9095a5ec6461491409f7a))
* **web:** wait on the first Try again in the axe scan of the sections ([431cdfa](https://github.com/jacob-delgado/workflow/commit/431cdfabcb47197beadd6a64d25520a370619f8c))
* **web:** walk and scan the Branch and Review sections' new forms ([17edbc6](https://github.com/jacob-delgado/workflow/commit/17edbc61d406fafaa0c6b84b9fa4fee48e2de1c2))
* **wiring:** cover the local data a store directory cannot hold ([e4be804](https://github.com/jacob-delgado/workflow/commit/e4be804382c515c58bd00e66efc3ebc5b509e839))

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
