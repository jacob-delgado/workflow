# Changelog

## [0.9.0](https://github.com/jacob-delgado/workflow/compare/v0.8.0...v0.9.0) (2026-10-09)


### ⚠ BREAKING CHANGES

* **setup:** on macOS, config init in a repository now asks whether to keep the Jira token in the keychain, as config init --global does; a script that answers its prompts answers that question too. The setup API no longer refuses the keychain for the repository's file, and its offer marks the keychain for every place where one is wired.
* **config:** config init --global and the first-run forms no longer write a jira.token_command reading the workflow-jira item; they keep the token in the item "workflow-jira <jira.base_url>" and write "keychain": true. A file an earlier config init wrote keeps reading the old item through its token_command; config init --global --force keeps the token for its address and writes the new setting. The settings API's JiraConfig gains keychain.
* when the store could not remember an announcement, workflow announce prints "Posted, but not remembered: it may be offered again." and why on stderr after "Announced to ...", in place of "the announcement was posted, but could not be remembered; a later session may offer to post it again: ...". A script matching the old text must match the new.
* **cli:** `workflow reviews --json` no longer prints `age`; each entry carries `opened_at`, an RFC 3339 time, instead. A script reading `.age` computes the wait from `.opened_at`.
* **config:** a jira.base_url of http:// to any host but localhost or a loopback address is refused. Change it to the instance's https:// address, or reach Jira through a tunnel or proxy on this machine.
* **setup:** GET /api/config/setup no longer answers a keychain field beside places; each place carries its own keychain instead, so a script reading the offer reads places[].keychain.
* **config:** a .workflow.json in a repository, or in the current directory outside one, that is a symbolic link no longer loads. Replace the link with the file itself, or keep the settings in ~/.workflow.json.
* **config:** a .workflow.json another user owns, or that others, or a group other than your own, may write, no longer loads. Make it yours with chown, and run chmod go-w on it.
* **taskwarrior:** a taskwarrior.program that is a relative path, such as bin/task or ./task, no longer loads. Name the program by its absolute path, or by a bare name looked up on PATH.
* **config:** a .workflow.json found in or above the current directory that sets jira.token_command, jira.token_env or taskwarrior.program no longer loads. Move those settings into ~/.workflow.json, which the repository's file layers over. config init offers the keychain only with --global, and a first run asked to keep the token in the keychain must write the home file.
* **config:** outside a git repository, a .workflow.json in a directory above the current one is no longer read. Move its settings into the current directory's file or into ~/.workflow.json.
* **config:** a .workflow.json whose jira.base_url is not an absolute http or https URL, or carries a username and password, or whose forge.kind is neither github nor gitlab, no longer loads. doctor reports it as a file that does not parse instead of listing a problem; correct the value in the file.
* **api:** an empty branch to POST /api/checkout, or issue_key to POST /api/branches or /api/worktrees, is answered 400 bad_request, not 422. A from or to that is not a YYYY-MM-DD date, on GET /api/activity or POST /api/activity/post, is a 400 too. Comment.created and ReviewRequest.opened_at are left out when the tracker or forge gave no time, rather than sent as 0001-01-01T00:00:00Z. MessagingDestination has a new required field, kind: slack, teams, discord or webhook.
* **webserver:** PUT /api/config refuses with 422 a body whose jira.token_command, jira.token_env or taskwarrior.program differs from the file's, and keeps them when the body leaves them out; edit .workflow.json to change them. Settings no longer edits the task program.
* **webserver:** every /api request must present the session the running workflow --web printed, as Authorization: Bearer <session>, or as ?session=<session> on GET /api/events; one without it is answered 401 with code unauthorized. The address workflow --web prints now ends in /#session=<session>, and a new one is made on each start.
* **webserver:** a rate limit from Jira, the forge or the messaging service is answered with code rate_limited and status 503, not unreachable and 502. A client that read unreachable to mean "wait" must read rate_limited, and may read Retry-After or retry_after for how long.

### Features

* **api:** describe each review request's facets on the server ([37fc99f](https://github.com/jacob-delgado/workflow/commit/37fc99f7d3d85a157d7b08811b202d9723d8d626))
* **api:** name the forge's kind in health, not only its words ([66409fa](https://github.com/jacob-delgado/workflow/commit/66409faeea5c898f81b7a315eb5277ddd2994b3d))
* **api:** rank and describe each task on the server ([f9af69f](https://github.com/jacob-delgado/workflow/commit/f9af69f4a570957dbe49a5bdacf5b7648133c447))
* **api:** ship the loop's stages in every frame of the stream ([91e142b](https://github.com/jacob-delgado/workflow/commit/91e142b2865d839d1e1cb57072f24869beb6a737))
* **api:** ship the stages of an issue whose branch is not checked out ([92d62d9](https://github.com/jacob-delgado/workflow/commit/92d62d9b8b44fc6685c58b755706ee37a85a98c4))
* **api:** state in the contract what the server enforces ([1fc230f](https://github.com/jacob-delgado/workflow/commit/1fc230f70985b4e1696d97c9c2c867d5655991cc))
* **cli:** print reviews --json as the web API's review requests ([9205a36](https://github.com/jacob-delgado/workflow/commit/9205a36aac087fa6fb53d02f3d42c4ab91404548))
* **config:** keep each Jira address's token in its own keychain item ([352bb0c](https://github.com/jacob-delgado/workflow/commit/352bb0c0114fe4c6127fe80f67f6417e645763a2))
* **httpx:** bound a streamed answer part by part ([d6de020](https://github.com/jacob-delgado/workflow/commit/d6de020f51ef8e47bbf15a862ac07d12347d7093))
* **httpx:** read an answer whole or say it was too large ([f6d12f5](https://github.com/jacob-delgado/workflow/commit/f6d12f5d1729cd6df6fcde489385c930ad5b3c37))
* **settings:** keep a Jira token typed in Settings in its keychain item ([47c5d9c](https://github.com/jacob-delgado/workflow/commit/47c5d9cd3f1cf934356ab1dce09df0880b857a2e))
* **setup:** offer the keychain for a repository's file in a first run ([82d997e](https://github.com/jacob-delgado/workflow/commit/82d997e246b945cf62def9ef14dd96baa383574d))
* show the finish commands gitrepo runs, in the terminal and the web ([58e8c3d](https://github.com/jacob-delgado/workflow/commit/58e8c3d0d444aa091e1c4de009f3bf4f0c95189c))
* **web:** name each issue's tracker from the server, not its key ([23ee1cd](https://github.com/jacob-delgado/workflow/commit/23ee1cdbb5c65f91ec3e3bcc100d533cd573168f))
* **webserver:** answer a rate limit as 503 rate_limited with its wait ([84e1583](https://github.com/jacob-delgado/workflow/commit/84e1583962b6515e1fda4598ff6a82f0b716d1f4))
* **webserver:** answer only a request presenting the run's session ([9201653](https://github.com/jacob-delgado/workflow/commit/92016535d67d25aa748782624dc559c6b159cae3))
* **webserver:** keep what workflow runs as you out of a settings save ([c9c0273](https://github.com/jacob-delgado/workflow/commit/c9c02737e1824b7a039693fc4293b36a4c52a925))
* **web:** warn when an announcement posts but is not remembered ([1c3d10b](https://github.com/jacob-delgado/workflow/commit/1c3d10b0c68d6abfbb2ba0a519a8d93926b44691))
* word store sizes and removal warnings once, in the store ([19f2b64](https://github.com/jacob-delgado/workflow/commit/19f2b6486cdb9a4bc09c496df6957ddb2ff9adff))


### Bug Fixes

* add the issue line to the description the forge holds ([da19282](https://github.com/jacob-delgado/workflow/commit/da19282c8fa07f1a5f5ded91bb55707cb91f9b0d))
* **api:** keep the issue key descriptions whole, and lint YAML strictly ([84b6b17](https://github.com/jacob-delgado/workflow/commit/84b6b175f026b64e7fc6d757be7c16c85b542373))
* **api:** rank the active task among the list that holds it ([b87007d](https://github.com/jacob-delgado/workflow/commit/b87007d64aa6f084b29471156104dfc14a7c2c47))
* **api:** read the clock once for a task list and its filter ([5c8a772](https://github.com/jacob-delgado/workflow/commit/5c8a772b1f0ca17ad79770efd5f9bec7be6e2d12))
* bracket a code fence the same in Preview as on Jira ([1787092](https://github.com/jacob-delgado/workflow/commit/1787092172764f855dc9d310597128b8fb61611d))
* **cli:** answer a secret asked of a pipe as the end of the input ([e254606](https://github.com/jacob-delgado/workflow/commit/e25460617bd91a2cdaad81d2d1b5949293cb6435))
* **cli:** exit 3 for every refused ui.keys map ([187a8a9](https://github.com/jacob-delgado/workflow/commit/187a8a96a5a1bd0949a639785c6eeb55503c2264))
* **cli:** fail a command whose prose output could not be written ([d0dd54a](https://github.com/jacob-delgado/workflow/commit/d0dd54a867d58f8c4d592d8da7f267f83c49a67a))
* **cli:** have doctor read its working directory once ([c5f1e62](https://github.com/jacob-delgado/workflow/commit/c5f1e62c7390cbe61d11f876862e1b2c6498511a))
* **cli:** name the Slack app only after its refresh is kept ([c2d664d](https://github.com/jacob-delgado/workflow/commit/c2d664d01e006c2220b42f34fb8b00eb0145eb4f))
* **cli:** outline slack login's requests in the --log file ([18394e8](https://github.com/jacob-delgado/workflow/commit/18394e8a604a2c5c6445454619c485fe90fa495f))
* **cli:** say in doctor --json why there is no repository ([af8b8cd](https://github.com/jacob-delgado/workflow/commit/af8b8cdcf4ab67c9f0d75d8d7c81f0d751302a87))
* **cli:** tell apart directories of one name in status DIR arguments ([c801e37](https://github.com/jacob-delgado/workflow/commit/c801e37764be8c0d99895a45b377f48200cec3c1))
* **cli:** warn of an unignored file from every config init write ([c13b620](https://github.com/jacob-delgado/workflow/commit/c13b620633774f902872ee3f22c6e01f1c96ea24))
* **cli:** wrap the root help's layering sentence within 80 columns ([f5b213f](https://github.com/jacob-delgado/workflow/commit/f5b213f6c741287f52868119ab9e81a4e1485f2a))
* **config:** let only the home file run a program or read a variable ([923d575](https://github.com/jacob-delgado/workflow/commit/923d5753f601b93150d70345c2f8e39fd52c9938))
* **config:** mask a short secret whole ([190d015](https://github.com/jacob-delgado/workflow/commit/190d015882c9cef739a0c5a81aabc93b14c27dc3))
* **config:** read and write no repository config file that is a link ([3493cd4](https://github.com/jacob-delgado/workflow/commit/3493cd4576b0f67d70fae52ee3f636066800e023))
* **config:** read no configuration file someone else could write ([92498f2](https://github.com/jacob-delgado/workflow/commit/92498f2e1525db09a7ab98602b4e2e3a42edbe8d))
* **config:** read no parent directory's file outside a repository ([c6045f9](https://github.com/jacob-delgado/workflow/commit/c6045f9868b3f2543e3c4755a45645f525b759e6))
* **config:** refuse a gone home file's credential in the repo file ([515973b](https://github.com/jacob-delgado/workflow/commit/515973ba258b396f330d671d04fcac6629432af5))
* **config:** refuse a typed Slack secret before the keychain is touched ([1bda579](https://github.com/jacob-delgado/workflow/commit/1bda57974aa30216c012112f0eea229688c05ac0))
* **config:** refuse an http jira.base_url off this machine ([4c292ca](https://github.com/jacob-delgado/workflow/commit/4c292ca3cd2fbae1ceadb5b8baa0ea884960efee))
* **config:** refuse an unusable Jira address or forge kind at load ([dd80b21](https://github.com/jacob-delgado/workflow/commit/dd80b21ca5ee35b9451522d7e3dfade92e93b4ec))
* **config:** touch no keychain or Slack for a save the file refuses ([f15f78a](https://github.com/jacob-delgado/workflow/commit/f15f78ad3889567a726ea6ae7e3fe4e66f8b91fc))
* **config:** write no credential typed in Settings into a repo's file ([09cfa49](https://github.com/jacob-delgado/workflow/commit/09cfa49b86cd61414bbcb3727aaa046b65a937f8))
* **convention:** write each issue key shape once; link only real keys ([ab471be](https://github.com/jacob-delgado/workflow/commit/ab471be43c46d80f4f05cca77ce40d01be6e6e18))
* **e2e:** build the server fixture in a directory of its own ([1f6e54b](https://github.com/jacob-delgado/workflow/commit/1f6e54b30fca8e65b47528bb497f8b335fc225cb))
* **editor:** honor the scissors only as a whole line ([07fc8f4](https://github.com/jacob-delgado/workflow/commit/07fc8f4580243dacf4039d5a9e3c989d92b21241))
* **forge:** count a review past the first page in GitHub activity ([aaa64e4](https://github.com/jacob-delgado/workflow/commit/aaa64e49606af13dc8e075b027e003c2f1854eeb))
* **forge:** keep why a group named as a user was not read ([85c00aa](https://github.com/jacob-delgado/workflow/commit/85c00aa2c24738970f300f6370ecdfef89bdef77))
* **forge:** mask the token in a forge's stated reason ([8322ef5](https://github.com/jacob-delgado/workflow/commit/8322ef54ac45f893db3ede7ae6de46a7040a191e))
* **forge:** read a long job log past the request timeout ([b7b6ab7](https://github.com/jacob-delgado/workflow/commit/b7b6ab72c6e94e20fd58be4226e89f251e5e4c0d))
* **forge:** read no template longer than a description can be ([907a413](https://github.com/jacob-delgado/workflow/commit/907a413a2d49385d368ede2ad235a3db45b4a815))
* **forge:** say when a listing stopped before its end ([3cd8342](https://github.com/jacob-delgado/workflow/commit/3cd8342680025f8d3d7f557263b77cf63116cd3b))
* **gitrepo:** keep branch and base names from reading as git options ([fa98683](https://github.com/jacob-delgado/workflow/commit/fa98683826a3823980e64a87545018e6c971fd0d))
* **gitrepo:** remove a file on Discard only when HEAD is truly unborn ([3e3b287](https://github.com/jacob-delgado/workflow/commit/3e3b287b3bbcb1e99521a6a3970fdcc12933d69d))
* **hooks:** create generated hook files only inside the repository ([b287c99](https://github.com/jacob-delgado/workflow/commit/b287c9919efbb6a76c397791b504fb6d3686d001))
* **hooks:** keep a hook whole when it sets an option jobs cannot keep ([7dd903b](https://github.com/jacob-delgado/workflow/commit/7dd903b677d93dbcb36aefc0f3857d9d58fea792))
* **hooks:** remove the directories a cut-short write created ([04dbe07](https://github.com/jacob-delgado/workflow/commit/04dbe07c5c1f19835170053cec1646b727ee5648))
* **hooks:** tell lefthook's shim apart from a hook that mentions it ([9a8e817](https://github.com/jacob-delgado/workflow/commit/9a8e8170d5d5b2d2847ac90771edf6c700b24d7c))
* **jira:** say an answer that is not JSON is not Jira ([b228285](https://github.com/jacob-delgado/workflow/commit/b228285b5dfea91fdd6ce3fceb9294e61c75760d))
* **loop:** neutralize a failed push's output where Push collects it ([ca76475](https://github.com/jacob-delgado/workflow/commit/ca7647524f494cec4aa5cde442493bdf38a0d1dd))
* **loop:** read whether work was announced by one rule everywhere ([ae7a3ed](https://github.com/jacob-delgado/workflow/commit/ae7a3edb2786bfdcda9053d1ec4302320f3f5072))
* **loop:** refuse a Summary post with no way to post it ([7694311](https://github.com/jacob-delgado/workflow/commit/769431153ad655eacfdf600c25c244efe82129a6))
* **loop:** word the not-remembered error as the one sentence ([42aed01](https://github.com/jacob-delgado/workflow/commit/42aed01a0504b33f70a1a373ebd6c78f584c8ae1))
* **messaging:** blame a webhook only for a refusal of the webhook ([64fc372](https://github.com/jacob-delgado/workflow/commit/64fc372867df536af8b648effe658c28c2ace78a))
* **messaging:** leave a directory read's 4xx a rejected read ([bd1cc64](https://github.com/jacob-delgado/workflow/commit/bd1cc645f2725ac02127eae1958a6d2b50dac128))
* **messaging:** mask a webhook quote that a refusal's cut falls inside ([c554358](https://github.com/jacob-delgado/workflow/commit/c55435897ea2c8193ad6900227b1b2b32ebea0d2))
* **proc:** end a streamed run whose leftover process holds its output ([4093b94](https://github.com/jacob-delgado/workflow/commit/4093b94323cb2dfa4e9560263f78c8c07dd0ff75))
* **proc:** report a caller's cancel from Run, and exits as ExitError ([a9326b0](https://github.com/jacob-delgado/workflow/commit/a9326b041222d9c7271b1e1a3157755cffdbb204))
* **progress:** count an announcement only for a pull request followed ([45f9e0e](https://github.com/jacob-delgado/workflow/commit/45f9e0ee58cb16f7560712402d38f58da99b868f))
* say an answer past its limit was too large, not broken JSON ([953e739](https://github.com/jacob-delgado/workflow/commit/953e7390afac774888c75d25ab321442e9423d37))
* say one not-remembered sentence on every surface ([c18468b](https://github.com/jacob-delgado/workflow/commit/c18468b74d91a1d805b15936c90319cb427993e1))
* **setup:** offer the keychain in a first-run form for the home file ([a73f76c](https://github.com/jacob-delgado/workflow/commit/a73f76c318a09b13d5a14e0d77cc9b8edb395fb2))
* **setup:** refuse a linked repository file that --force would replace ([d347288](https://github.com/jacob-delgado/workflow/commit/d34728833123b93c0864d196c969283de6cb835a))
* **setup:** touch the keychain only once the file is in place ([4e8eee5](https://github.com/jacob-delgado/workflow/commit/4e8eee5e1256c226a27c8e7d8b5cf1eb5669e849))
* **slackauth:** hold the refresh lock as a lock the system keeps ([54208a8](https://github.com/jacob-delgado/workflow/commit/54208a8a347043acdbe26ef507fdeed8697e1b4a))
* **slackauth:** read a refresh's status before its answer ([07fa3fa](https://github.com/jacob-delgado/workflow/commit/07fa3fa7e32771f509a92b1ee391883236008237))
* **store:** count the repository group choices in db-clean's summary ([4b9ff95](https://github.com/jacob-delgado/workflow/commit/4b9ff95895ce4c3125edae68934ffbbe2ca20ac1))
* **store:** ignore a relative XDG_STATE_HOME or AppData ([ded1e15](https://github.com/jacob-delgado/workflow/commit/ded1e1526d92a6f657ad2d1c16a6daadf4c96a02))
* **store:** open a database under a Windows share's path ([054a391](https://github.com/jacob-delgado/workflow/commit/054a391827e5910650b4d93854f139600d78df7d))
* **store:** open every database file by an escaped file URI ([77bd03f](https://github.com/jacob-delgado/workflow/commit/77bd03fbcb7c977398ac9722d1081b4f9ef6a0e6))
* **taskwarrior:** label no empty value of an unknown facet kind ([a47b86a](https://github.com/jacob-delgado/workflow/commit/a47b86ae3bea0a66fd4466a99526adfd03d9dfaa))
* **taskwarrior:** refuse a track line whose issue key is not one word ([8b22d2b](https://github.com/jacob-delgado/workflow/commit/8b22d2beb4a1dba28bf48e0dcebd3131aeb44f87))
* **taskwarrior:** run no task program named relative to the directory ([f38c859](https://github.com/jacob-delgado/workflow/commit/f38c85982ed719cfa38fd81fc76ddb94d0b17be3))
* tell an answer too large or too slow on every surface ([aacea30](https://github.com/jacob-delgado/workflow/commit/aacea30ddb26b687a9195b0338fd965b219e8228))
* tell Jira's answer that is not JSON on the web and the terminal ([80caa75](https://github.com/jacob-delgado/workflow/commit/80caa75fa50fde01b83fbf1dc8e9b8f459d97604))
* **testshape:** name a versioned import by its package, not its version ([854e7c0](https://github.com/jacob-delgado/workflow/commit/854e7c005803b492438c9fd93482f80792ead3a0))
* **tui:** build each pane's key context from what its handler answers ([1ecca04](https://github.com/jacob-delgado/workflow/commit/1ecca0464a36e13ea973310094662d645badade4))
* **tui:** complete a directory to its name as it is on disk ([c193c83](https://github.com/jacob-delgado/workflow/commit/c193c8359c1f302e875604198c80b8298567020e))
* **tui:** draw a repository's configured names as plain text ([c7fd588](https://github.com/jacob-delgado/workflow/commit/c7fd588f0a38c67b6a45c6f06347230240ab5e25))
* **tui:** drop a go-to look that answers after its prompt closed ([9dfc8af](https://github.com/jacob-delgado/workflow/commit/9dfc8af0d31d255e37333233b53cf4d145bbdaaa))
* **tui:** give a focused ASCII rail pane the corners of a focused box ([0d40c3e](https://github.com/jacob-delgado/workflow/commit/0d40c3e514172d1e9a67c8f536974f9befc05c55))
* **tui:** hand a pane every key, so a missed key context fails a test ([db12f25](https://github.com/jacob-delgado/workflow/commit/db12f251f2732d8a91ba3931c739bd7ecfdc176c))
* **tui:** keep a dry-run commit's message for the next composer ([12a080d](https://github.com/jacob-delgado/workflow/commit/12a080d455e1f5572305cf020d5f9dc3838cd337))
* **tui:** let each pane's footer and keys read one list of offers ([d0c1a2b](https://github.com/jacob-delgado/workflow/commit/d0c1a2bd9c7a000c647417f5e576d6d3dd240a04))
* **tui:** name the channel an announcement went to in its notice ([7fc3c4f](https://github.com/jacob-delgado/workflow/commit/7fc3c4f5ce0eadcab1b24aebeee23d367598dee3))
* **tui:** name the keychain in the token's hint as it is turned on ([8a9420d](https://github.com/jacob-delgado/workflow/commit/8a9420dc202057644a7b99cfea95578944bdf11f))
* **tui:** never post twice from a preview opened over a queued post ([b29f23e](https://github.com/jacob-delgado/workflow/commit/b29f23e45cbf88f5f74768fbf33318d3c940d2ae))
* **tui:** page a long job log with pgup and pgdn ([9330838](https://github.com/jacob-delgado/workflow/commit/9330838383aeeeadeefe447fd9640eae3361e60b))
* **tui:** read git, the store and templates off the update loop ([9f4d467](https://github.com/jacob-delgado/workflow/commit/9f4d46721f987644f596c65e00add9624446e799))
* **tui:** read keys from the terminal when stdin is not one ([bb75f35](https://github.com/jacob-delgado/workflow/commit/bb75f356d4beec117714b3eca180e8f32e39901d))
* **tui:** show a forge issue's number with its # wherever it is named ([3b28e49](https://github.com/jacob-delgado/workflow/commit/3b28e49945fbf2c7671fba0123ad02261d1aedfc))
* **tui:** size the help's key column from the widest key shown ([a57d7bd](https://github.com/jacob-delgado/workflow/commit/a57d7bdba685872f3c630971f8a69a1af7fb420e))
* **tui:** wrap the assign, log-work and link forms' refusals in full ([73eb4a9](https://github.com/jacob-delgado/workflow/commit/73eb4a9d383ea00cd612006bac0230d6b3be8ae7))
* **web:** draw each stage in the state the server read it ([c2fbe5c](https://github.com/jacob-delgado/workflow/commit/c2fbe5c400d33d5ea9e0b54f79bc7fee8f9f4c79))
* **web:** drop a run in flight on reset, and hold Cancel while it goes ([7da7c0d](https://github.com/jacob-delgado/workflow/commit/7da7c0dbf53333115f2c21fb530f9897040b74ec))
* **web:** link only to http and https addresses ([7ca141e](https://github.com/jacob-delgado/workflow/commit/7ca141e93f2529e65a1fdf57a59734ce96a4c710))
* **web:** say a review request's facts in the words its filter uses ([781a547](https://github.com/jacob-delgado/workflow/commit/781a547eeed1f89e05794be8d02bb0e92eb812e1))
* **web:** say a stream the browser gave up on is disconnected ([2368ae3](https://github.com/jacob-delgado/workflow/commit/2368ae3c708dd2084af4c93f1f9a0e804b1bbc39))
* **webserver:** answer a branch git could not read in the link preview ([e0b223b](https://github.com/jacob-delgado/workflow/commit/e0b223bd2df3a89698fcb7bd408c28ac5cc0a0e8))
* **webserver:** answer a task write that landed when its list is unread ([cfd0f99](https://github.com/jacob-delgado/workflow/commit/cfd0f99f9dce71359706448ccb92f9578b304922))
* **webserver:** ask who the author is once an interval, off the lock ([8d37362](https://github.com/jacob-delgado/workflow/commit/8d37362fc48807c17669515d9985dda140ac45e7))
* **webserver:** mask a failure's note with the configuration in effect ([9ee9db3](https://github.com/jacob-delgado/workflow/commit/9ee9db3618cb915fd216b788d6e69161e149e759))
* **webserver:** name the 422s from config and setup by the prefixed code ([4bc5047](https://github.com/jacob-delgado/workflow/commit/4bc5047243da93676f7b87dd4b138f6bd327d223))
* **webserver:** post an announcement once when two ask at once ([98a3c9e](https://github.com/jacob-delgado/workflow/commit/98a3c9ea61e5eddfba3ffe3d34698b250ef1b0a5))
* **webserver:** serve every answer under a same-origin content policy ([9923ffe](https://github.com/jacob-delgado/workflow/commit/9923ffe32530b869c96bed631b4968fe60477c71))
* **webserver:** serve which issues are yours while the tracker is asked ([3090684](https://github.com/jacob-delgado/workflow/commit/3090684dbf846ef2ee5e681dc34ec347038da1ad))
* **web:** take a test's body from after its name in the marker rule ([b8e7b54](https://github.com/jacob-delgado/workflow/commit/b8e7b540bae6cecd7e56146ba9d1070c16c9e352))
* **web:** title the work story's stages as the terminal names them ([1913836](https://github.com/jacob-delgado/workflow/commit/19138361089fff3d617e7c7057c4ab296122635a))
* **web:** word a task's state as the server reads it ([e8f6191](https://github.com/jacob-delgado/workflow/commit/e8f6191b88610b845e557bc84ab191b22e51a2bf))
* **web:** write every date and count through dates.ts ([27be292](https://github.com/jacob-delgado/workflow/commit/27be2928e395dbc5f6d4cda1d353cc5647bdab8f))
* **wiring:** ask which forge it is at each group lookup ([9c46c67](https://github.com/jacob-delgado/workflow/commit/9c46c676c9019c2929b166c4371db9f8d5e44b85))
* **wiring:** find the home file's Slack token when posting from a repo ([dcb2e9c](https://github.com/jacob-delgado/workflow/commit/dcb2e9ced298d8e8fdeef9cd68b86bffc7915c73))
* **wiring:** keep a failed token command's output out of its error ([0a9b1f3](https://github.com/jacob-delgado/workflow/commit/0a9b1f3e008008ea25598c15d34f85bd4a3c9d97))
* **wiring:** read pull request templates only from the repository ([4bb1926](https://github.com/jacob-delgado/workflow/commit/4bb1926143ba64dfe90b55df17b29a48bb52a7f9))
* **wiring:** refuse a bare forge number at every Jira issue seam ([56962fc](https://github.com/jacob-delgado/workflow/commit/56962fc616b76f751c3908cd5c70f1ad61c824a9))
* **wiring:** say when the store cannot keep a write, and trust no moment ([45f4046](https://github.com/jacob-delgado/workflow/commit/45f4046b37b4b5e55bc45bb1f1ff1d4a0932d45a))
* **wiring:** skip the forge token lookup when the forge's CLI logs in ([de53472](https://github.com/jacob-delgado/workflow/commit/de53472ef60abb4247e072c4cf9ade596e9e8c7b))
* **wiring:** tell a refresh Slack did not judge from a refused token ([a4a1ed8](https://github.com/jacob-delgado/workflow/commit/a4a1ed86748529c1e53550947cbc0c21dd35fc24))


### Performance

* **webserver:** share each frame's network reads among open streams ([3b15e22](https://github.com/jacob-delgado/workflow/commit/3b15e2236e8cbc34ac839194f6352afce734ec44))
* **workdirs:** check only the listed directories for a repository ([c870474](https://github.com/jacob-delgado/workflow/commit/c870474a8e079aeae2696dcfd5686b6bae198f4c))


### Refactors

* **activity:** let Merge order items without deduplicating them ([8c4985a](https://github.com/jacob-delgado/workflow/commit/8c4985ad2911cc5e84449f64219d63e5848d5383))
* **api:** exclude streaming routes by operation and mark switches ([35c4231](https://github.com/jacob-delgado/workflow/commit/35c4231ba705bccc7f6735e3bfe74b0f58b2be66))
* **api:** prefix every generated enum constant with its type ([ce63c43](https://github.com/jacob-delgado/workflow/commit/ce63c437616e90a205138b3da0b112cc45751c8d))
* build Settings hints from the defaults that own them ([ef3ec90](https://github.com/jacob-delgado/workflow/commit/ef3ec9094b36dcb47f018ff7b0f87e5659e5698f))
* **cli:** check doctor --json credentials only once the file loads ([50fbf69](https://github.com/jacob-delgado/workflow/commit/50fbf69793c01380fa332db54d2e20bd6213a04e))
* **cli:** extract the steps of announce, guided init and the root ([2dc69f5](https://github.com/jacob-delgado/workflow/commit/2dc69f54bcd4511ef8853b27c72827146a86964f))
* **cli:** give doctor's credential checks one line model ([25d3794](https://github.com/jacob-delgado/workflow/commit/25d3794586201dfd3e51d965e00a14f0bd821282))
* **cli:** match doctor's credential outcomes with isAny ([564fe1b](https://github.com/jacob-delgado/workflow/commit/564fe1bc2a4113a2777d99dc34071d547679c334))
* **cli:** split status by concern into gather and render files ([92bc8eb](https://github.com/jacob-delgado/workflow/commit/92bc8eb81e88a7d24b9274a8f494da8866c3c18a))
* **codeowners:** match each dialect with its own pattern ([f8f3d84](https://github.com/jacob-delgado/workflow/commit/f8f3d84f51e9838d91ec984fcbfdb2277ab9587f))
* **config:** delete the single-file load and save no caller uses ([f0f1e68](https://github.com/jacob-delgado/workflow/commit/f0f1e685a6915631bd98e41b1d4f8b35d1ae2788))
* **config:** move what a repository's file may inherit to trust.go ([783a1dc](https://github.com/jacob-delgado/workflow/commit/783a1dca7b81de4f3cb509eb45fe477731334a66))
* **config:** name the repository file's refusal for what it checks ([f0f658d](https://github.com/jacob-delgado/workflow/commit/f0f658d9fb96a2530482cd0d629f6f9c7cb0d3f3))
* **config:** read a link once to follow it, not stat it first ([0d670b8](https://github.com/jacob-delgado/workflow/commit/0d670b8383b492a40e2f6efe1306ca0e1fcea8a1))
* **config:** stop keeping a masked base URL's login ([9f16271](https://github.com/jacob-delgado/workflow/commit/9f16271eca9810a51479440db42f168e92a734cc))
* drop the group members seam nothing calls ([cdbfe4c](https://github.com/jacob-delgado/workflow/commit/cdbfe4c1f71e7293e0883d5f5df7b67fd1bdd4ec))
* **e2e:** build every stream frame from one empty snapshot ([540b9a0](https://github.com/jacob-delgado/workflow/commit/540b9a05737f7011c4c56aa5bf4d1c57c2526a25))
* **e2e:** move single-surface specs and the helpers into folders ([370a0f2](https://github.com/jacob-delgado/workflow/commit/370a0f2606efde59ce5364c13c819728905c49f0))
* **e2e:** move surface tests out of the layout and a11y sweeps ([d20f8c8](https://github.com/jacob-delgado/workflow/commit/d20f8c8d5a277d76f660b02234d2e32f84e2320d))
* **fileowner:** declare Owner beside each platform's Of ([0fcb663](https://github.com/jacob-delgado/workflow/commit/0fcb6630754a5f7db318aa0919afe097df133a9a))
* **forge:** move GitHub's people and issues out of github.go ([51ef2e3](https://github.com/jacob-delgado/workflow/commit/51ef2e36a1d09598873bd4d2805f74fa5ed76979))
* **forge:** move the review queue out of pulls.go ([ae54597](https://github.com/jacob-delgado/workflow/commit/ae54597a9ba2c36f914c3a67ac6ee536b58ba992))
* **forge:** name the conclusions that pass once ([2a539f2](https://github.com/jacob-delgado/workflow/commit/2a539f2504107486a2d2af8d1868d3ba0c17924c))
* **forge:** read a search item's repository in one place ([6664ede](https://github.com/jacob-delgado/workflow/commit/6664edec5fb5a07feac4446bb75755e7dd4ed0a8))
* **forge:** resolve a GitLab user's id in one place ([7605df1](https://github.com/jacob-delgado/workflow/commit/7605df1bf8059b7e282264977a50ec11870c230e))
* **gitrepo:** run every git command through gitProgram ([11af7f1](https://github.com/jacob-delgado/workflow/commit/11af7f1d902043099c63dbb1715c3fe5b63f85ac))
* give CI states one word and stage states one mark ([4e2dccb](https://github.com/jacob-delgado/workflow/commit/4e2dccb979547f6afe9c37daf1d5b5eb6f7bec6c))
* hand every command the environment it runs in ([84a92b7](https://github.com/jacob-delgado/workflow/commit/84a92b78ee551689178f4648869b1e755ce28c50))
* hand the web server the interface's seam groups whole ([9bf7ef2](https://github.com/jacob-delgado/workflow/commit/9bf7ef2d0697d4b0182635442b81adcc5feef91c))
* hold the place rules' two copies to one case file ([5836233](https://github.com/jacob-delgado/workflow/commit/58362331c7dd0aff6f91ac17baece74939ea41e3))
* **loop:** answer why a post was not remembered from the loop ([4a135c1](https://github.com/jacob-delgado/workflow/commit/4a135c1ed6ef89b393fe4d9b6985e9c088b1f3b8))
* **loop:** keep the setup advice in setup.go ([165d6e3](https://github.com/jacob-delgado/workflow/commit/165d6e37e909530254c28c8fd114620fe4a81323))
* **loop:** make SlackTarget messaging's own type ([df54d2e](https://github.com/jacob-delgado/workflow/commit/df54d2e06007102cb4b58eed97687de0cfacf4e4))
* mark the place and wiki rule twins as TRADE-21 and TRADE-28 ([8f6282e](https://github.com/jacob-delgado/workflow/commit/8f6282ed19ae6fae82708a12b5da41447ed1a800))
* **messaging:** delete Client.Workspace, which no caller uses ([c8c9c79](https://github.com/jacob-delgado/workflow/commit/c8c9c79136300aaf535fb9eb9f4f022b61225234))
* **messaging:** move the announcement out of post.go ([d6a9359](https://github.com/jacob-delgado/workflow/commit/d6a935931ef7e709d596f62b1bcd2c40c20e5fc4))
* move the Slack directory into internal/messaging/directory ([dc1664f](https://github.com/jacob-delgado/workflow/commit/dc1664f731680723583998ee8f8df20896615d62))
* **proc:** drain a streamed program's output in one place ([5c2ccc1](https://github.com/jacob-delgado/workflow/commit/5c2ccc182c3abe5b22eb34d156561c2fdfc55337))
* **proc:** run a program a caller found on a PATH of its own ([dc29f4b](https://github.com/jacob-delgado/workflow/commit/dc29f4b077abfc006820452274ddb3b534c8a3ba))
* report what every surface shares from internal/report ([3ab1847](https://github.com/jacob-delgado/workflow/commit/3ab184784d0976b74e79d3a1259dde8f7fbf9ae9))
* **sanitize:** escape Markdown from one list of its characters ([455c481](https://github.com/jacob-delgado/workflow/commit/455c48170145b5440b77d2ade32639d715a7c8d1))
* **store:** open the database files through internal/sqlitefile ([181f782](https://github.com/jacob-delgado/workflow/commit/181f7827c21d14d1f1441be05e37125aac6011f2))
* **store:** read a file's state at once and share one transaction ([de21955](https://github.com/jacob-delgado/workflow/commit/de21955b130dcf0261184d379a5cb10d5dbcac18))
* **taskwarrior:** rank priorities from one ordered list ([cabee2b](https://github.com/jacob-delgado/workflow/commit/cabee2b820178b16327f05d1770dc057968cde6d))
* **taskwarrior:** reach every case of a task's state ([36b38d4](https://github.com/jacob-delgado/workflow/commit/36b38d4425cf23581818606fabd70b7f78037d29))
* **tui:** answer a list overlay's esc and cursor keys in one place ([93390b2](https://github.com/jacob-delgado/workflow/commit/93390b29815212081b87539c8c36e9aa042a9ded))
* **tui:** ask each pane's state what it can do and draw the spine ([8e9e7f0](https://github.com/jacob-delgado/workflow/commit/8e9e7f0fd1d661cc2442469c265082a4ea802ea9))
* **tui:** ask the loop once whether a post was remembered ([845e66f](https://github.com/jacob-delgado/workflow/commit/845e66f6756437c0bfd1fab8f52b4c25597b1d8b))
* **tui:** assert each applier beside its definition ([ac1f0e9](https://github.com/jacob-delgado/workflow/commit/ac1f0e91098b2497a9283edc59e06ca82d4fe66e))
* **tui:** define each pane's behavior beside the pane ([89b8737](https://github.com/jacob-delgado/workflow/commit/89b8737d2809f86bb53d1bbab6773a7e8f84a9c8))
* **tui:** draw failures through the render kit ([0d9b5fd](https://github.com/jacob-delgado/workflow/commit/0d9b5fdff3c2168f933d2bf6766adc439a6694f2))
* **tui:** hand onFieldNav the ring of fields a composer stands in ([340d0d9](https://github.com/jacob-delgado/workflow/commit/340d0d97f714eea24bd563a646e9e5cd5dbe9016))
* **tui:** hand overlays a render kit instead of copying it in ([2c95f34](https://github.com/jacob-delgado/workflow/commit/2c95f3464c35a800aeec9b9616c512b8b4fbbbc3))
* **tui:** keep the shared path helpers with the text helpers ([17a9769](https://github.com/jacob-delgado/workflow/commit/17a97692fa04382439f8df3112cc83f4716f105d))
* **tui:** leave Model to route and compose the panes ([565ce3d](https://github.com/jacob-delgado/workflow/commit/565ce3d911c12b4bc1d83239f7560714de8fd28d))
* **tui:** move focus around one ring; navigate composers alike ([1751bb0](https://github.com/jacob-delgado/workflow/commit/1751bb053208532094c8324c394eb6a7d2bf2ae7))
* **tui:** name the selected issue's URL apart from browseURL ([17cadf6](https://github.com/jacob-delgado/workflow/commit/17cadf6b4e297c06b9e58fbd6af65644db3f177a))
* **tui:** pass layout a Terminal and a Rail, not four ints ([d702185](https://github.com/jacob-delgado/workflow/commit/d7021854492018b34c1a3087d96f186c7181fe43))
* **tui:** pick the wheel's direction with two tests, not a switch ([3f38037](https://github.com/jacob-delgado/workflow/commit/3f3803792dc354957aa0ad9e399d216059a42ddd))
* **tui:** render Repositories, Branch and Commits from their state ([38c5353](https://github.com/jacob-delgado/workflow/commit/38c53535e2eb0b4a6c4c5541119667d47bfd4da4))
* **tui:** render Tasks, Reviews and Summary from their own state ([3e555c5](https://github.com/jacob-delgado/workflow/commit/3e555c56ec144892fe2bdbaaf6e469046e65b605))
* **tui:** split the long files by concern ([ecafc09](https://github.com/jacob-delgado/workflow/commit/ecafc09542bbec574ba1f36c045098b6747c6f8c))
* **tui:** take the interface's input from the caller ([b6d4ebf](https://github.com/jacob-delgado/workflow/commit/b6d4ebfaf2dbed703e575356dc0bf870461c74d8))
* **tui:** take work that needs no interface state off Model ([a08d0f0](https://github.com/jacob-delgado/workflow/commit/a08d0f0035e0844d96a55e07af8ae08ba7e6320d))
* **web:** answer the mockup from one mock server ([854879f](https://github.com/jacob-delgado/workflow/commit/854879ff1e3780035140034ca5643cfb1b976492))
* **web:** count the task clock in dates.ts's minute ([4adacb8](https://github.com/jacob-delgado/workflow/commit/4adacb8d913675e2d2279fea6070760afc5340f8))
* **web:** draw every last look before a write through LastLook ([794b3c5](https://github.com/jacob-delgado/workflow/commit/794b3c5a9f355e61809cb291a59025d068825831))
* **web:** hold every running Button with held ([075d688](https://github.com/jacob-delgado/workflow/commit/075d688a71c30d55b95ab9800bb11b892ecf8982))
* **web:** move the state mark and empty state into lib, a leaf ([a1a649a](https://github.com/jacob-delgado/workflow/commit/a1a649ace45d860c9174ea03ea0525bf8b2b4a27))
* **web:** open the embedded app without an error arm ([26361e3](https://github.com/jacob-delgado/workflow/commit/26361e34bd0e39447a7f927c3bbdacdc1b6e1836))
* **web:** say every time ago through one formatter in dates.ts ([cce0b96](https://github.com/jacob-delgado/workflow/commit/cce0b96b2aa8a27fefeb7ed4010596ca2b230525))
* **webserver:** keep the shared reads and the delivery together ([7be3172](https://github.com/jacob-delgado/workflow/commit/7be317253f620c8ecc90b827e5d489b3d301922f))
* **webserver:** return a fault's problem alone and class sentinels ([9842a64](https://github.com/jacob-delgado/workflow/commit/9842a64b6f8fc0ede397e6a811c12ea3dfa02b65))
* **webserver:** set a held announcement anew in one place ([834a821](https://github.com/jacob-delgado/workflow/commit/834a821b50538f294dceb9e695de0099ba1f9749))
* **webserver:** settle the held announcement on the forge's read ([be0b039](https://github.com/jacob-delgado/workflow/commit/be0b0391b5c1013147447e1df1f8a76362e07276))
* **webserver:** split errors.go and people.go by concern ([d17c9e9](https://github.com/jacob-delgado/workflow/commit/d17c9e910612149b756626965ed4bb76ca0d9a1f))
* **webserver:** take every shared read's turns through one loop ([c3db7f4](https://github.com/jacob-delgado/workflow/commit/c3db7f4cf4de4b150f5a28c0c2b59f6728bd14eb))
* **web:** split the issue list's parts out of IssuesPanel ([1a087a6](https://github.com/jacob-delgado/workflow/commit/1a087a650b7ca758fcd935e0273658ff7399e29b))
* **web:** take no zero time as an unknown one in ago ([2eefdf3](https://github.com/jacob-delgado/workflow/commit/2eefdf3e97a570e41805e39626a74be80b55dd7b))
* **wiring:** ask httpx which hosts are this machine's ([c3d2edf](https://github.com/jacob-delgado/workflow/commit/c3d2edf9bc891b200189c4bcb157eb6952ed60ae))
* **wiring:** reach Jira, Taskwarrior and the forge through one ask ([63d1332](https://github.com/jacob-delgado/workflow/commit/63d13329765b796b99ba9794b60f87e176178cae))
* **wiring:** take the keychain's platform and runner as arguments ([469eafd](https://github.com/jacob-delgado/workflow/commit/469eafdc60ef9a19d56e844860e99a491604ea8f))


### Documentation

* **api:** describe a stage by every list that carries it ([90933ae](https://github.com/jacob-delgado/workflow/commit/90933ae26c2700e3b825c609d58349a321c4fd4f))
* **api:** name the new reasons a save and a setup answer 422 ([2e0a6d8](https://github.com/jacob-delgado/workflow/commit/2e0a6d8e074b1e4b65be7f4bfa0eb1001690a625))
* **api:** say a rate limit is a 503 where three writes describe it ([ab49298](https://github.com/jacob-delgado/workflow/commit/ab49298653c2d1b0e712c6430f1640bbcd59b3f3))
* **api:** say which credential a repository's file refuses ([ceb2f74](https://github.com/jacob-delgado/workflow/commit/ceb2f74224d6c34be1b765bdb556551bb1c9524a))
* **architecture:** describe kept.db beside the cache ([12b53d9](https://github.com/jacob-delgado/workflow/commit/12b53d94c71a4be03161d2a6295f4884c192dc15))
* **architecture:** say what a dry run reads from each store file ([b959ed9](https://github.com/jacob-delgado/workflow/commit/b959ed9a8821761b9e5196d12eb70e6e029c3806))
* **ci:** say how gobco reads build-tagged twins in the PR comment ([fbd8af2](https://github.com/jacob-delgado/workflow/commit/fbd8af289251ae4830a7f3bcffb27b279b4e668e))
* **claude:** ask for the Boy Scout improvement on the branch ([ddf3093](https://github.com/jacob-delgado/workflow/commit/ddf3093be44261058225c57c25bfa6389b53fb88))
* **claude:** land a Boy Scout improvement as its own commit ([08ea4e2](https://github.com/jacob-delgado/workflow/commit/08ea4e2a85959bb210f4e858a2c9f69fc092a1f9))
* **claude:** mark the goroutine smell as gated ([2c60792](https://github.com/jacob-delgado/workflow/commit/2c60792a90d641e9342671708bb1bf652aa1d196))
* **claude:** name Taskwarrior, slackauth and kept favorites ([5f6ef9a](https://github.com/jacob-delgado/workflow/commit/5f6ef9ab27a098822f238071101a97cb2eff44f7))
* **claude:** say the layout block is checked ([20837b5](https://github.com/jacob-delgado/workflow/commit/20837b5bc449e6990c96d3bcbde82a3ea55ae18f))
* **cli:** say reviews --json leaves out an opened_at with no time ([e28b05f](https://github.com/jacob-delgado/workflow/commit/e28b05f4a566743c87c0e4d588b7172f2d14bbd6))
* **cli:** show a worked invocation in every command's help ([8d86bd3](https://github.com/jacob-delgado/workflow/commit/8d86bd305af508ba52988cd32335ac40cbdf3037))
* **config:** drop the earlier-build workspace promise ([c182934](https://github.com/jacob-delgado/workflow/commit/c1829343c25c7d566d822cfe12ae379b4607d2b5))
* **config:** say disabling the store drops what was decided too ([a7ee66b](https://github.com/jacob-delgado/workflow/commit/a7ee66b2427720e42be6b3f33083c528c56eb801))
* **config:** say only a non-empty NO_COLOR turns color off ([86d9c34](https://github.com/jacob-delgado/workflow/commit/86d9c34b6e92ce1b7c8110b05893c5438758c297))
* **config:** say what a disabled store leaves an announcement ([58df76a](https://github.com/jacob-delgado/workflow/commit/58df76ae81fcd984428ef8fda9c3b691dd20c944))
* **contributing:** name mise.toml among what lints the Markdown ([8c20bd6](https://github.com/jacob-delgado/workflow/commit/8c20bd61f1d8f97de573ad93c03075b99bb306f6))
* **contributing:** say a subject may be 72 characters ([89bbbdb](https://github.com/jacob-delgado/workflow/commit/89bbbdbbeb202dcf1338b98603c2be8066a47155))
* **debt:** catalog the pre-1.0 audit ([53d3a0d](https://github.com/jacob-delgado/workflow/commit/53d3a0d98b9743b00de4283fefe7caa7b8c6e411))
* **debt:** close DEBT-216, now health names the forge's kind ([5da8bd2](https://github.com/jacob-delgado/workflow/commit/5da8bd272aa3282da5eca0671ddbfb3dbacffe07))
* **debt:** close DEBT-241 and TRADE-2, now no test file passes 700 ([42756ec](https://github.com/jacob-delgado/workflow/commit/42756ec9d5d018c8ef91dbdb68c6dfa21cebecdb))
* **debt:** close DEBT-243, now the timing tests wait on signals ([f96bdd6](https://github.com/jacob-delgado/workflow/commit/f96bdd609d6f4ab5ed171747e81b035c06d98c9e))
* **debt:** close DEBT-278, now the register cites what the code has ([9f20ded](https://github.com/jacob-delgado/workflow/commit/9f20ded303216593401277bd517fce09703eb156))
* **debt:** close the architecture seams' entries ([a36cda0](https://github.com/jacob-delgado/workflow/commit/a36cda000928618b8149a7392cce5a8183788361))
* **debt:** close the cli entries ([252db66](https://github.com/jacob-delgado/workflow/commit/252db6658dd7688ee8c904b98aa710c42d5fc054))
* **debt:** close the clients entries ([7b4d341](https://github.com/jacob-delgado/workflow/commit/7b4d3413961118b5416dd8ea25a584ccad9e57b8))
* **debt:** close the core entries ([355b613](https://github.com/jacob-delgado/workflow/commit/355b61370a4159b9f2361f6074d1fa2ec5ca8115))
* **debt:** close the docs entries ([871bb06](https://github.com/jacob-delgado/workflow/commit/871bb066d7f07f9a3fe8c7d1ef1ea281d384d1b8))
* **debt:** close the e2e entries ([7e85e99](https://github.com/jacob-delgado/workflow/commit/7e85e99300e0ae9b0a5f3d8306370117cffad082))
* **debt:** close the gates entries ([4b59389](https://github.com/jacob-delgado/workflow/commit/4b593896831f9ac562a06d039a34d340674008a6))
* **debt:** close the rules the web wrote a second time ([a6a8245](https://github.com/jacob-delgado/workflow/commit/a6a824597bc1ae1897ae74d4d7b9804cd5f20ccd))
* **debt:** close the server entries ([7262afe](https://github.com/jacob-delgado/workflow/commit/7262afeaaff7bda42cbd50438bb57d1be9beeb83))
* **debt:** close the state entries ([1218e94](https://github.com/jacob-delgado/workflow/commit/1218e9424771f9c688d2767353645b173c692ef5))
* **debt:** close the terminal's structure entries ([7754d9f](https://github.com/jacob-delgado/workflow/commit/7754d9ff25d0376198a10755b02e05f347effbca))
* **debt:** close the tui entries ([a4d7f1c](https://github.com/jacob-delgado/workflow/commit/a4d7f1c322b7c501a9b73802dc99d7845c04c57a))
* **debt:** close the web entries ([e654611](https://github.com/jacob-delgado/workflow/commit/e654611e34fecbf66ee997335582d1a8e38b7834))
* **debt:** close TRADE-7, now that gobco reads every package ([debf6a8](https://github.com/jacob-delgado/workflow/commit/debf6a8f60ec9817e7286fb78caac0412fd44a77))
* **debt:** name in DEBT-278 only the register text still wrong ([d4663f0](https://github.com/jacob-delgado/workflow/commit/d4663f093252196ae3f520ed4c9f322a525c72e4))
* **debt:** narrow DEBT-286 to the gate texts still wrong ([009480a](https://github.com/jacob-delgado/workflow/commit/009480ab71c51c23e81d91da249ec7a44bede8fd))
* describe gobco's gate without the retired skip list ([86ca399](https://github.com/jacob-delgado/workflow/commit/86ca399041f4614319efec8e688d497d35e328e8))
* describe ratcheted budgets and cohesive ceilings in CLAUDE.md ([de9dd49](https://github.com/jacob-delgado/workflow/commit/de9dd496089741e28e2ce2afa5e8a0f14818e747))
* describe the dry run's store as the code reads it ([3cab444](https://github.com/jacob-delgado/workflow/commit/3cab444da82e61f8b2ea514fc39f66a623b2fcaf))
* drop release history from the reference pages ([54b4b34](https://github.com/jacob-delgado/workflow/commit/54b4b343d04dc3d25e5e421e79df451bb780a9a1))
* keep one Status list, on the site's home page ([4e69284](https://github.com/jacob-delgado/workflow/commit/4e692845cc9c574d2a631839ef7f59da194b3b69))
* keep setup and the gate in CONTRIBUTING, linked from the site ([20e2634](https://github.com/jacob-delgado/workflow/commit/20e2634d14ac6252203c7afdc22c4737f895be0a))
* list internal/places and internal/messaging/directory in CLAUDE.md ([fbd5a41](https://github.com/jacob-delgado/workflow/commit/fbd5a41c1a24f20af298714323ab352c01b5b93b))
* **loop:** point at the depguard rule rather than list the imports ([aac4cbb](https://github.com/jacob-delgado/workflow/commit/aac4cbb33f9f73f26435804e3ce328393af0da0b))
* name the linters that are off, and why, rightly ([47a27d4](https://github.com/jacob-delgado/workflow/commit/47a27d4e62ad82cd3c3ba4d04e316cfb3b8bea13))
* name the plain webhook in the taglines ([3251cce](https://github.com/jacob-delgado/workflow/commit/3251cce31baad3617ac88c001f0a3c49e82bc5be))
* name the release platforms and contents in one place ([5d41c7c](https://github.com/jacob-delgado/workflow/commit/5d41c7c7793cba72126eafddd7f190d2054b1382))
* **readme:** link to the install page and setup, not copy them ([4386f36](https://github.com/jacob-delgado/workflow/commit/4386f36a0c2b42aaa8c9ba68e34305b0cbec4182))
* **readme:** link to what is kept rather than list it ([3bbe1e2](https://github.com/jacob-delgado/workflow/commit/3bbe1e2199d1171787fe2a19801684a9b6d4594b))
* **readme:** name nine panes' keys and the web's nine sections ([62c9850](https://github.com/jacob-delgado/workflow/commit/62c9850b03028c3f111282b2b72609b0fab4919f))
* **readme:** say once how the configuration file is kept safe ([7d2467b](https://github.com/jacob-delgado/workflow/commit/7d2467b3aaed1c618746403809cdc587c92446bd))
* say how the Tasks list and the filters read now ([10259de](https://github.com/jacob-delgado/workflow/commit/10259de8d86b13f4cd1e73742fc2bbf9673176cf))
* say the release is built on a runner, not in the container ([30c9aad](https://github.com/jacob-delgado/workflow/commit/30c9aadbb573a573cfd62fe2188cc05403f535da))
* **scripts:** cite a real layout in the package-size split advice ([e7812b4](https://github.com/jacob-delgado/workflow/commit/e7812b4db359b4f9c5ccaa4e70f72a3ec486d655))
* **security:** describe both store files and the gitignore warning ([451487f](https://github.com/jacob-delgado/workflow/commit/451487f364b7f790373b04206254666ee4b444d6))
* **site:** point at the README rather than repeat its tagline ([b83e7eb](https://github.com/jacob-delgado/workflow/commit/b83e7eb39f2b7250e3e6701562ae4ebcbb412205))
* **store:** name both files in the package doc ([21d7043](https://github.com/jacob-delgado/workflow/commit/21d7043c2971adcebeba02d9ad39d08c733fcb65))
* **store:** say a dry run reads no cache, not no store ([39b86a0](https://github.com/jacob-delgado/workflow/commit/39b86a0c3d448d8c6bf8dbdab256f8edc1fa5ba4))
* **usage:** say the scroll keys page a failed check's log ([f1673f1](https://github.com/jacob-delgado/workflow/commit/f1673f1ae16415972a44033f2fdffd619d499fcd))
* **web:** count the Repositories section's writes as requests ([6a71b45](https://github.com/jacob-delgado/workflow/commit/6a71b4598dd663945cdb34345dba2970ae125197))
* **web:** name the terminal's keys and the API's requests rightly ([4ea74a5](https://github.com/jacob-delgado/workflow/commit/4ea74a5c1fe6faa854575ded076a198cea7a8b65))
* **web:** say Write it anyway refuses http to another machine ([913b0f7](https://github.com/jacob-delgado/workflow/commit/913b0f776358d89d3ccd8f37e40c5f2bedb26d55))
* **webserver:** say the not-remembered warning is the web's own ([f59d505](https://github.com/jacob-delgado/workflow/commit/f59d505f81cb09dc306d0873526bcbbb6142586d))


### Build & Packaging

* add task e2e and name it in the definition of done ([f362cb9](https://github.com/jacob-delgado/workflow/commit/f362cb92acec3c00efeb131c80268fe6c02aab20))
* Bump source-map-js from 1.2.1 to 1.2.2 in /web ([f4b7856](https://github.com/jacob-delgado/workflow/commit/f4b7856147956a6c8224e68b85421e3bc50fbbd7))
* Bump the web group across 1 directory with 6 updates ([57016dd](https://github.com/jacob-delgado/workflow/commit/57016dda743ee0539a8dfb2bd6f06530084253fb))
* check that CLAUDE.md's layout names every Go package ([d921ff8](https://github.com/jacob-delgado/workflow/commit/d921ff8dacd361383f6239d690b76a08b3b1fb74))
* fail the generated-code checks on an uncommitted new file ([0b05ad6](https://github.com/jacob-delgado/workflow/commit/0b05ad69b28b7c1640ef3877d80fac68afc260e5))
* format the web frontend in task fmt ([867cac6](https://github.com/jacob-delgado/workflow/commit/867cac60f482c4da077f7beca5b8abdb3fb011fc))
* give file-per-concern packages a ceiling, not zero headroom ([8d6296f](https://github.com/jacob-delgado/workflow/commit/8d6296f98aac529b0620462472464013013ef11a))
* hash a download with shasum where sha256sum is missing ([97e19a0](https://github.com/jacob-delgado/workflow/commit/97e19a057aa861c316f24c44a2aa1f63d7c04089))
* install the hooks in a linked worktree's devcontainer too ([dcbb720](https://github.com/jacob-delgado/workflow/commit/dcbb72001ae7e878e813311d7bade2b87fa5d43f))
* lint only this commit's change in the pre-commit hook ([d3b895b](https://github.com/jacob-delgado/workflow/commit/d3b895b974dc180b1464123546584c0af1800a8e))
* lint with golangci-lint 2.14.0 ([9d84506](https://github.com/jacob-delgado/workflow/commit/9d845066b8b6fb767832f5ba06916b1129ecdc45))
* **lint:** refuse a test file past 700 lines ([834224f](https://github.com/jacob-delgado/workflow/commit/834224febab0a3db8ae92dc32a7f66bbead59a4e))
* measure a package of build-tagged twins a file at a time ([d12c906](https://github.com/jacob-delgado/workflow/commit/d12c906e6ad5d0d844b038fb405e58d48d1329d3))
* move cobra's and sqlite's own dependencies to their newest ([200ee10](https://github.com/jacob-delgado/workflow/commit/200ee10aeebf385e909ba738ccf7c91658d8a0d5))
* move Go to 1.27.2 for the standard library's fixes ([d263d4f](https://github.com/jacob-delgado/workflow/commit/d263d4f445750c257d1de91350c59db2e7c4afe9))
* move go-runewidth and xo/terminfo under the Charm stack ([9fa1eec](https://github.com/jacob-delgado/workflow/commit/9fa1eec7370e054fbc410cdc19545e51e406fe18))
* move lefthook to 2.1.16 and typos to 1.50.3 ([2a0f9b0](https://github.com/jacob-delgado/workflow/commit/2a0f9b07d240baa0f04e8d87f25012840a4e9421))
* move task, hugo and markdownlint-cli2 to their newest ([a117fff](https://github.com/jacob-delgado/workflow/commit/a117fffad61cd351e71a025c386bfc849f12ee55))
* move the OpenAPI stack's jsonpointer and speakeasy openapi ([0df7461](https://github.com/jacob-delgado/workflow/commit/0df74614ae3bf26ff9962ed221ab22bd3b4a6b6f))
* pin the devcontainer's image and feature by digest ([87070db](https://github.com/jacob-delgado/workflow/commit/87070db6bfdb0960b48663737641ac99dbb063f8))
* say the docs name a platform only as an example ([8294673](https://github.com/jacob-delgado/workflow/commit/82946739b3931e32ec963dc4186ed1976d357245))
* stop claiming Dependabot moves the build image's digest ([1d18214](https://github.com/jacob-delgado/workflow/commit/1d18214da96aeeabca113edc2fdeafb4a63af185))
* **testshape:** read a synctest bubble's body for its markers ([38ce834](https://github.com/jacob-delgado/workflow/commit/38ce83469d0247639b44809d51e44ec56468ddcd))
* verify every build-container download against a pinned SHA-256 ([9bc5670](https://github.com/jacob-delgado/workflow/commit/9bc56701b28f8825b9e56b40162009b201a78635))
* **web:** drop the unused jsx-a11y types package ([71156ef](https://github.com/jacob-delgado/workflow/commit/71156ef9cfc5a376db6ffe494986614dce095e2c))
* **web:** hold ESLint on 9, which every lint plugin declares ([7978b09](https://github.com/jacob-delgado/workflow/commit/7978b095ec8469e481f4037c859095f86e962370))
* **web:** lint switches for every case and list keys for identity ([b9421bc](https://github.com/jacob-delgado/workflow/commit/b9421bcad33cee03216ce292fc54f9f439897326))
* **web:** lint the e2e specs with Playwright's rules and the markers ([1b978e2](https://github.com/jacob-delgado/workflow/commit/1b978e2eb4802ac5f88b2102387c9eb5620d89f8))
* **web:** mark goober's csstype peer optional ([5d6ad3e](https://github.com/jacob-delgado/workflow/commit/5d6ad3eee810aaf8b72b3f0240aff273d8b9dac4))
* **web:** require a reason on every lint disable ([e97270e](https://github.com/jacob-delgado/workflow/commit/e97270e1d8ed8505cdaf6a90483346301b0c38f3))
* **web:** say why the page is one chunk, and warn above it ([3502c2c](https://github.com/jacob-delgado/workflow/commit/3502c2cc5d2a1b097f7755c1cf825008dd228d08))
* **web:** tighten the test project, its paths and its store resets ([b610abc](https://github.com/jacob-delgado/workflow/commit/b610abcd2e314cc993393f12a0bcf8e7beaa8365))
* write the Go package roots once, in Taskfile.yml ([cdb57bd](https://github.com/jacob-delgado/workflow/commit/cdb57bdeff07feafc6e04246192de33df8cdfbce))


### CI

* analyze the web app with CodeQL as well ([9d29aa8](https://github.com/jacob-delgado/workflow/commit/9d29aa8b53b8a1b3018240f2c2092f0a6b629060))
* bound every job's run time and group each workflow's runs ([c3a954c](https://github.com/jacob-delgado/workflow/commit/c3a954c7fb2d981a72f12b84aa8fc319e56fdf21))
* Bump the actions group across 1 directory with 6 updates ([46be594](https://github.com/jacob-delgado/workflow/commit/46be5940cf6454449db04f888e054ca3ce256367))
* catch goroutines started through WaitGroup.Go ([fa39961](https://github.com/jacob-delgado/workflow/commit/fa399611cac7889fc45487b736deeeacad297407))
* gate the Web job with the web tasks rather than a copy of them ([1577835](https://github.com/jacob-delgado/workflow/commit/1577835e2c76b1e6b4c56e20d05ba9a40fe4aa6c))
* hold each new dependency version to the week-long age gate ([9baab72](https://github.com/jacob-delgado/workflow/commit/9baab7216c95ce5a43788f7c3caf5b8f1341dc8a))
* hold the Pages rights in the deploy job alone ([62811b4](https://github.com/jacob-delgado/workflow/commit/62811b4228dd9e738ed06e9ddb51fc70529f9b1e))
* leave a body Dependabot signed out of the wrap rule ([e718b91](https://github.com/jacob-delgado/workflow/commit/e718b913b5e59f12e2956d90e7660475c2b696d0))
* let ci-gate pass a skip from commit-lint alone ([d3bf242](https://github.com/jacob-delgado/workflow/commit/d3bf242535548d7d3d0aeb4b10adc6dbb0a4fb06))
* lint the Markdown when mise.toml changes ([253433a](https://github.com/jacob-delgado/workflow/commit/253433acf334942f5d26a211c293cd77ef25315d))
* name each CodeQL job by its language alone ([0ebba3d](https://github.com/jacob-delgado/workflow/commit/0ebba3d941ccfd04212b9ee1dc0e5caa4cef1c30))
* refuse commit bodies not wrapped at 72 characters ([ca75cb2](https://github.com/jacob-delgado/workflow/commit/ca75cb2b30e594fa60391ba0eaa5ab74bf5d80dd))
* summarize uploaded results and install only what each job runs ([1cc5e4c](https://github.com/jacob-delgado/workflow/commit/1cc5e4c21c6b617487b185b1dde163f668dc5240))


### Tests

* **cli:** expect the help's words for where the file is looked for ([25d18e6](https://github.com/jacob-delgado/workflow/commit/25d18e689f9eb23a165d1fa40097f02f48fba597))
* **cli:** find the server's seams with a tagless switch ([e042002](https://github.com/jacob-delgado/workflow/commit/e042002d145481eda57df0d6e92e99fb32ff84bc))
* **cli:** hold scripting.md's command lists to the command tree ([6d620a7](https://github.com/jacob-delgado/workflow/commit/6d620a720662fa7f41635d2f8fd26b995af46e02))
* **cli:** name CI none by its prefixed constant in the reviews test ([e2752b7](https://github.com/jacob-delgado/workflow/commit/e2752b7da63716ef7c2d2d509ecf5e0a122b2c92))
* **cli:** pin the reason that follows the not-remembered sentence ([c058f87](https://github.com/jacob-delgado/workflow/commit/c058f87710f4e08054d8f13c42e651fcc3d62d71))
* **cli:** present the session in the web server's masking tests ([51ee423](https://github.com/jacob-delgado/workflow/commit/51ee4231aa20957d9393b1fb99132a1b0533336b))
* **cli:** reach the request log's own reading of a removed directory ([647ca53](https://github.com/jacob-delgado/workflow/commit/647ca537f890ce86bef37a8e36a5cbdaed4a63cc))
* **cli:** save the masked token over the home file in Settings ([5f3dfa5](https://github.com/jacob-delgado/workflow/commit/5f3dfa53c4772209c820f912f3d2893a846bfc63))
* **codeowners:** give GitLab's reading of the file a test file ([e8c05f3](https://github.com/jacob-delgado/workflow/commit/e8c05f309794133102b32c2361777eda7e8a7244))
* **config:** refuse an http base URL off this machine where it loads ([ff78aad](https://github.com/jacob-delgado/workflow/commit/ff78aadedb8860b857ad2502e65eef3d9edc8345))
* **config:** see every credential condition decide its case ([37e0ede](https://github.com/jacob-delgado/workflow/commit/37e0ede966d4ae5cf4cc125c923fe528757d1675))
* **directory:** wait for the second reader to join, not 50 ms ([48b6c32](https://github.com/jacob-delgado/workflow/commit/48b6c326c1f30f3dd5c0b4aa0bf1eb4b2943eb5d))
* **docsgen:** move the reference generator into a tested package ([f349b05](https://github.com/jacob-delgado/workflow/commit/f349b05b8e6f9a9cacfb923a01ed4e7efb2b7b4a))
* **e2e:** answer a repository switch with where it went ([2b9cfff](https://github.com/jacob-delgado/workflow/commit/2b9cfffbdde21b0bf969dcdcd28f7ecbdffde010))
* **e2e:** answer every refusal as the server builds its problem ([1c0176d](https://github.com/jacob-delgado/workflow/commit/1c0176df45b9d5497994341f5e04dc41370f7f20))
* **e2e:** fail the mark-hue test when no mark is drawn ([587e08a](https://github.com/jacob-delgado/workflow/commit/587e08a8641b3f6e43e0e337cb880f98e8820794))
* **e2e:** fit the served page spec to the moved helpers and the lint ([5099c2f](https://github.com/jacob-delgado/workflow/commit/5099c2f567732c2ca6b988c01745f3e461ae3765))
* **e2e:** give each section's layout, axe and screen a test of its own ([4e6fb6e](https://github.com/jacob-delgado/workflow/commit/4e6fb6e7ce5e7c88dce2d4e20dbd2ecb1d6e11c4))
* **e2e:** give the unauthorized and rate-limited codes their meanings ([b56e15f](https://github.com/jacob-delgado/workflow/commit/b56e15f8da1ca5e4c519f16c6838842eb252b528))
* **e2e:** grow each screen's window until its section stops growing ([447ba51](https://github.com/jacob-delgado/workflow/commit/447ba51e4b990a3458c342596b6acf4d5903fbbc))
* **e2e:** hold every surface to the layout floor through one helper ([5deaf31](https://github.com/jacob-delgado/workflow/commit/5deaf31a8383c63f2cf30620f492adcebc486d23))
* **e2e:** locate by role, assert by retrying, and label both flows ([39ecbbd](https://github.com/jacob-delgado/workflow/commit/39ecbbdfd7dc4e151241f7d8ec0e2710ccdcd1a9))
* **e2e:** make three tests fail when their Act does nothing ([57ca846](https://github.com/jacob-delgado/workflow/commit/57ca846497e2036587d18ef732856df35a9e54fd))
* **e2e:** open every section through openSection ([c1fa627](https://github.com/jacob-delgado/workflow/commit/c1fa627a5dff1ba753456acb3d55906d387d44d3))
* **e2e:** refuse the API a hermetic spec leaves unanswered ([5e55f0e](https://github.com/jacob-delgado/workflow/commit/5e55f0ed8c8757ea7547ac839058f4eb814b8b1a))
* **e2e:** report a stop the Tab walk reaches but nothing draws ([3753cc2](https://github.com/jacob-delgado/workflow/commit/3753cc2eb640f616db07e38f272c203dfe744169))
* **e2e:** scan each hermetic section once its own reads have settled ([e673ceb](https://github.com/jacob-delgado/workflow/commit/e673ceb0a277eade466cf0a286696505511996ee))
* **e2e:** share the confirm steps and section list the sweeps scan ([15db909](https://github.com/jacob-delgado/workflow/commit/15db909031885cda541cae8bc205f00986d55a71))
* **e2e:** walk the keyboard trap with the shared Tab walk ([8999a2a](https://github.com/jacob-delgado/workflow/commit/8999a2a96d1fbef618fa701dc0276a8f191a06e0))
* **fileowner:** read the owner of a file the test makes ([8c11f06](https://github.com/jacob-delgado/workflow/commit/8c11f069c216d06423175f1275645fffafdaa1cc))
* **forge:** cover a description rewrite that cannot read ([374b4e5](https://github.com/jacob-delgado/workflow/commit/374b4e543db121b9f10b2013893201fb81f58244))
* **forge:** exercise the failure paths of activity, logs and writes ([00e96f8](https://github.com/jacob-delgado/workflow/commit/00e96f8a7b19bae5c0aed357f4299720d3ac88c6))
* **forge:** serve every fake forge through one recording helper ([55165bb](https://github.com/jacob-delgado/workflow/commit/55165bbf88a4dd1620399098ab451e30bdc44d88))
* **gitrepo:** cover dropped issue links and failed dated-log reads ([3135c1d](https://github.com/jacob-delgado/workflow/commit/3135c1db19e6ec2afd1ee6bb035aee7531fadecf))
* **gitrepo:** make the with helper copy instead of writing in place ([db4a6d3](https://github.com/jacob-delgado/workflow/commit/db4a6d32bb83b37914d87193867d9cde98731904))
* hold the calendar's two copies to one case file ([d2d6e13](https://github.com/jacob-delgado/workflow/commit/d2d6e134f6d625f11f24e6e82a2e65db10d1a179))
* keep git's repository variables in one list both TestMains clear ([afa0ec7](https://github.com/jacob-delgado/workflow/commit/afa0ec74c172c7a9855ae759e7f1aa67966e9034))
* **loop:** pin that the not-remembered error says the warning's words ([1eea2de](https://github.com/jacob-delgado/workflow/commit/1eea2dee5d2b9374950f792c7dc4d26b5d3141ee))
* make six tests fail when the call they test does nothing ([8ca3b96](https://github.com/jacob-delgado/workflow/commit/8ca3b9692b8b3d94bb6f2bc41c251f08743bb88a))
* **messaging:** cover users.info and the scopes a token lacks ([cb99e59](https://github.com/jacob-delgado/workflow/commit/cb99e59f21a372647b1cf1557617d4be5cc77739))
* **proc:** give pgroup's cancel tests of its own ([5cd3f06](https://github.com/jacob-delgado/workflow/commit/5cd3f0630cbc01b0ff0864cc2022497f2bf481e8))
* **scripts:** cover gobco-report's floor, skip list and Go check ([f79f6c3](https://github.com/jacob-delgado/workflow/commit/f79f6c3a762259344c53ed3d7441c5e5b01a263f))
* **scripts:** match expect_output's text as written ([955efd6](https://github.com/jacob-delgado/workflow/commit/955efd678bfee3ce2e4c8dc86c2c530857f46b1c))
* **scripts:** share one harness across the script tests ([35e288d](https://github.com/jacob-delgado/workflow/commit/35e288df83e8f6dd6d3d51224348349604b813ef))
* **setup:** give the keychain offer's tests a file of their own ([dec9fb6](https://github.com/jacob-delgado/workflow/commit/dec9fb6501d445d8a2fe4a1cf8aa50a351cc6dbb))
* **slackauth:** see each check on Slack's answer and each failing step ([2118b28](https://github.com/jacob-delgado/workflow/commit/2118b283daa77567c0f6d7a3dc99440c55b12665))
* **store:** open a database as another program would without a lookup ([7814514](https://github.com/jacob-delgado/workflow/commit/781451489da8aef52475c66b285ff8f446cf8026))
* **tui:** drop only commands a hold keeps from answering ([383a670](https://github.com/jacob-delgado/workflow/commit/383a670a3e72280755c9f882ed3ab3fd1cac51fa))
* **tui:** exercise the stale-answer and data-loss guards ([b7b6a76](https://github.com/jacob-delgado/workflow/commit/b7b6a76748dad2611996598684c22b8384711487))
* **tui:** give the Settings paths test secrets long enough for a tail ([b6af07c](https://github.com/jacob-delgado/workflow/commit/b6af07c66b712a3c54beda811c5e4d8fa947d15b))
* **tui:** hold each key to its own action's row in usage.md ([790a078](https://github.com/jacob-delgado/workflow/commit/790a07866fb590c9c9a100eee174aff639b7af8a))
* **tui:** hold each write so a second send can be tried while it is out ([9ed200b](https://github.com/jacob-delgado/workflow/commit/9ed200bad154242f3526b053fa0d39b7ae51f613))
* **tui:** hold every Settings path to the setting it names ([176b96f](https://github.com/jacob-delgado/workflow/commit/176b96f1804a1b73ef64216259f10479698f933b))
* **tui:** hold usage.md's key table to every key the help lists ([a425d68](https://github.com/jacob-delgado/workflow/commit/a425d68082965e60904c1a25c63886d0c29243b6))
* **tui:** name the key the tests bind refresh to once ([8f5b6ba](https://github.com/jacob-delgado/workflow/commit/8f5b6ba9ed96f7c80ce159e24f8e2d0b967acef3))
* **tui:** pin that a preview with no store links and tags no one ([2ce973b](https://github.com/jacob-delgado/workflow/commit/2ce973ba3037fea3298133a31dc761665f4b452b))
* **tui:** probe each pane in every state that answers its keys ([85a3b03](https://github.com/jacob-delgado/workflow/commit/85a3b039c6b1955258d320ffb0ecf7d45cc510f1))
* **tui:** prove each held seam is reached before reading the screen ([ae2a2cc](https://github.com/jacob-delgado/workflow/commit/ae2a2ccdc24b62c309370c3345cbc37ba9127c9e))
* **tui:** return the store's error from the held write seams ([92981d7](https://github.com/jacob-delgado/workflow/commit/92981d79eb326238879576d17ad550d6868036c8))
* **tui:** see the link form's footer before a description is shown ([d076f17](https://github.com/jacob-delgado/workflow/commit/d076f17376349ab0e0ab0757405d081f03122b99))
* **web:** answer Settings' local data in the shape the store words ([97e44d4](https://github.com/jacob-delgado/workflow/commit/97e44d472a475ddf809a383c30bc191ef725cc51))
* **web:** drop a stage the old story always drew ([52d5700](https://github.com/jacob-delgado/workflow/commit/52d5700f08a2a93ba8001f8695f6e4834f3e306b))
* **web:** find the marker rule's test function among its arguments ([7e719e6](https://github.com/jacob-delgado/workflow/commit/7e719e6fce7bcd1b2e1a4168f451924b2da5590f))
* **web:** give a failed check's reason and log a test file ([3dd28d4](https://github.com/jacob-delgado/workflow/commit/3dd28d4690a19eca55172d062c44489a164c1a69))
* **web:** give each apiError case its own row, and cover problemCode ([2a1c1a6](https://github.com/jacob-delgado/workflow/commit/2a1c1a66dbadeac06c21047058e9ceb8cfd1e11e))
* **web:** give the story's worktree flows a test file ([4aca9bb](https://github.com/jacob-delgado/workflow/commit/4aca9bba38d2bb11a029f8261718b9068d34c1fb))
* **web:** hold every web test to the Go tests' marker rule ([84f71b8](https://github.com/jacob-delgado/workflow/commit/84f71b87cdb5d1425332ab87d2bfa0404f5b64c2))
* **web:** hold the branch and function floors two points under ([dbd99c7](https://github.com/jacob-delgado/workflow/commit/dbd99c7f42ee42f27cb37cec971b84cf11b176a0))
* **web:** hold the pre-paint theme script to the store's rules ([c9d48a4](https://github.com/jacob-delgado/workflow/commit/c9d48a48d0fbd7a24be25c60a366c6016b06e97a))
* **web:** make jsdom once per worker with the vmThreads pool ([fdde382](https://github.com/jacob-delgado/workflow/commit/fdde382a29779599c2d894ebc25d73c31b5bbee9))
* **web:** mark each unit test's earlier check as a step of its own ([b4747ff](https://github.com/jacob-delgado/workflow/commit/b4747ff5fa9c899211b563de2cb10ecc87da6e2b))
* **web:** quote the server's refusal of a Jira address as it says it ([faeee6b](https://github.com/jacob-delgado/workflow/commit/faeee6b3f8a211437323c5138ce7b4ca934deb6f))
* **web:** raise the branch floor to two points under measured ([42421ce](https://github.com/jacob-delgado/workflow/commit/42421cee865effcd885289aa8d97af8912d27e0f))
* **web:** read the golden frames inside the frame-count test ([bb0db97](https://github.com/jacob-delgado/workflow/commit/bb0db97f27fc17b68ed7e2e70a727bd878bc57b8))
* **web:** render feature tests under the app's own query policy ([03c784e](https://github.com/jacob-delgado/workflow/commit/03c784e5c54a66ab19e8b34c1e11e23d361e9a66))
* **webserver:** expect a layered save's program refused as held ([82492c6](https://github.com/jacob-delgado/workflow/commit/82492c6015c988f37c1e41eb73facd5823430e44))
* **webserver:** fail a facet test whose answer lists nothing ([d3c734e](https://github.com/jacob-delgado/workflow/commit/d3c734e75fc7be59e518943a416cc9503568f951))
* **webserver:** hear a store's note with the configuration in effect ([4297410](https://github.com/jacob-delgado/workflow/commit/42974104b2abcf0674a7558e9b3ced5cab63494e))
* **webserver:** hold scripting.md's families table to every code ([841f3f7](https://github.com/jacob-delgado/workflow/commit/841f3f743dfba90431ea48aac5fcc2a1259e9d93))
* **webserver:** hold the overlap tests' windows inside their seams ([125cebf](https://github.com/jacob-delgado/workflow/commit/125cebfc98f06f7a9925e2dbec56d784c40e3406))
* **webserver:** pin that a new hold drops the last one's warning ([9932215](https://github.com/jacob-delgado/workflow/commit/993221575e5bd113baec4595bebb7003c2064fa3))
* **webserver:** read a held announcement on a synctest clock ([46d906c](https://github.com/jacob-delgado/workflow/commit/46d906c8d383032814eaad82da5814d2562bfd26))
* **webserver:** read the held-settings fixture through LoadLayersAt ([2a0b166](https://github.com/jacob-delgado/workflow/commit/2a0b16699f52273cef1364e0d51cc01f00485e6c))
* **webserver:** run the stream's timing on a synctest clock ([9b7f1de](https://github.com/jacob-delgado/workflow/commit/9b7f1de0489c5b9ad19ea4da1dedb0a371870381))
* **webserver:** spell a read the test slows down once ([e0765eb](https://github.com/jacob-delgado/workflow/commit/e0765eb9fbb715285bf464713ba4e34d85ea2f98))
* **webserver:** split the run tests by what they hold a run to ([0051fa3](https://github.com/jacob-delgado/workflow/commit/0051fa32855dea55351d87ff41d7fea4e1f3289a))
* **webserver:** wait on a run's own signals rather than the clock ([27f46e1](https://github.com/jacob-delgado/workflow/commit/27f46e10e87a792e8678ca02c071137b46de57e7))
* **web:** show no time on a comment the tracker gave no date for ([91e1f6f](https://github.com/jacob-delgado/workflow/commit/91e1f6fa753e52062863e89be149664bc873e88c))
* **web:** show the strict a11y rules fire, and settle two peer warnings ([2ac6d96](https://github.com/jacob-delgado/workflow/commit/2ac6d9658b0ef3862dcb9c60bf545d45f856cb1c))
* **web:** split the issue detail's Try again cases into a file ([7dcb1ec](https://github.com/jacob-delgado/workflow/commit/7dcb1ec65cdc50a65c6ce15a0ad7d566123d849d))
* **web:** wait for the outcome line to take focus after a move ([ca14fa7](https://github.com/jacob-delgado/workflow/commit/ca14fa77df44f98f999abca5e6b7158b205b2e40))
* **web:** write what the server describes into each case's data ([ddf36ab](https://github.com/jacob-delgado/workflow/commit/ddf36abc9f8184aff36ad95b50ddbe4f762c5dca))
* **wiring:** give the keychain tests an account of their own ([cf0dbfa](https://github.com/jacob-delgado/workflow/commit/cf0dbfa4b642edaea0ef5548ed58da317cb9c27d))
* **wiring:** read the refresh tests' file through LoadLayersAt ([425d9d6](https://github.com/jacob-delgado/workflow/commit/425d9d6e972c4a4e7a806427689c25ff950cd4cd))
* **wiring:** show templates are read beside the link left out ([b2e7f5e](https://github.com/jacob-delgado/workflow/commit/b2e7f5e497aa493cadb754f7e3ac0e9f95f4b6bc))

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
