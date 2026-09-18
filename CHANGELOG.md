# Changelog

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
