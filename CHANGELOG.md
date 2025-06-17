<a id="v1.17.0-beta.7"></a>
# [v1.17.0-beta.7](https://github.com/dagu-org/dagu/releases/tag/v1.17.0-beta.7) - 2025-06-03

## Changelog
* [`0dfafd647e`](https://github.com/dagu-org/dagu/commit/0dfafd647e226aa4bf281ae6cd72a70d2adbcf76) [[#965](https://github.com/dagu-org/dagu/issues/965)] Fix: docker executor does not print stderr log ([#981](https://github.com/dagu-org/dagu/issues/981))



[Changes][v1.17.0-beta.7]


<a id="v1.17.0-beta.6"></a>
# [v1.17.0-beta.6](https://github.com/dagu-org/dagu/releases/tag/v1.17.0-beta.6) - 2025-06-02

## Changelog
* [`a3d73a41fc`](https://github.com/dagu-org/dagu/commit/a3d73a41fcc23095af47587bf4600d283b18abf0) [[#965](https://github.com/dagu-org/dagu/issues/965)] fix: remove restrictive step name validation ([#980](https://github.com/dagu-org/dagu/issues/980))



[Changes][v1.17.0-beta.6]


<a id="v1.17.0-beta.5"></a>
# [v1.17.0-beta.5](https://github.com/dagu-org/dagu/releases/tag/v1.17.0-beta.5) - 2025-06-01

## Changelog
* [`85936abd63`](https://github.com/dagu-org/dagu/commit/85936abd63a34106c2f5b19fb018ed7225536ca7) [[#967](https://github.com/dagu-org/dagu/issues/967)] fix: shell options are not working ([#974](https://github.com/dagu-org/dagu/issues/974))



[Changes][v1.17.0-beta.5]


<a id="v1.17.0-beta.4"></a>
# [v1.17.0-beta.4](https://github.com/dagu-org/dagu/releases/tag/v1.17.0-beta.4) - 2025-06-01

## Changelog
* [`c87ad77d13`](https://github.com/dagu-org/dagu/commit/c87ad77d13a66eb9c989f8eb70dc75c7cc7e7854) Fix: version name issue ([#973](https://github.com/dagu-org/dagu/issues/973))



[Changes][v1.17.0-beta.4]


<a id="v1.17.0-beta.3"></a>
# [v1.17.0-beta.3](https://github.com/dagu-org/dagu/releases/tag/v1.17.0-beta.3) - 2025-05-31

## Changelog
* [`2967d22849`](https://github.com/dagu-org/dagu/commit/2967d22849e77b7013028b9388a7dc6195dba95d) [fb-next] Command for migrating history data from v1.16.x to v1.17.x ([#969](https://github.com/dagu-org/dagu/issues/969))



[Changes][v1.17.0-beta.3]


<a id="v1.17.0-beta.2"></a>
# [v1.17.0-beta.2](https://github.com/dagu-org/dagu/releases/tag/v1.17.0-beta.2) - 2025-05-31

## Changelog
* [`5cb885a718`](https://github.com/dagu-org/dagu/commit/5cb885a718edcf5aca55af0db387b2abd00a571e) [[#965](https://github.com/dagu-org/dagu/issues/965)] fix: 404 error for different host name and port ([#966](https://github.com/dagu-org/dagu/issues/966))



[Changes][v1.17.0-beta.2]


<a id="v1.17.0-beta.1"></a>
# [v1.17.0-beta.1](https://github.com/dagu-org/dagu/releases/tag/v1.17.0-beta.1) - 2025-05-30

**🚀 Version 1.17.0-beta.1 Available - Significant Improvements & New Features**

We're excited to announce the beta release of Dagu 1.17.0! This release brings many improvements and new features while maintaining the core stability you rely on.

**Key Features in 1.17.0:**
- 🎯 **Improved Performance**: Refactored execution history data for more performant history lookup
- 🔄 **Hierarchical Execution**: Added capability for nested DAG execution
- 🎨 **Enhanced Web UI**: Overall UI improvements with better user experience
- 📊 **Advanced History Search**: New execution history page with date-range and status filters ([#933](https://github.com/dagu-org/dagu/issues/933))
- 🐛 **Better Debugging**: 
  - Display actual results of precondition evaluations ([#918](https://github.com/dagu-org/dagu/issues/918))
  - Show output variable values in the UI ([#916](https://github.com/dagu-org/dagu/issues/916))
  - Separate logs for stdout and stderr by default ([#687](https://github.com/dagu-org/dagu/issues/687))
- 📋 **Queue Management**: Added enqueue functionality for API and UI ([#938](https://github.com/dagu-org/dagu/issues/938))
- 🏗️ **API v2**: New `/api/v2` endpoints with refactored schema and better abstractions ([OpenAPI spec](./api/v2/api.yaml))
- 🔧 **Various Enhancements**: Including [#925](https://github.com/dagu-org/dagu/issues/925), [#898](https://github.com/dagu-org/dagu/issues/898), [#895](https://github.com/dagu-org/dagu/issues/895), [#868](https://github.com/dagu-org/dagu/issues/868), [#903](https://github.com/dagu-org/dagu/issues/903), [#911](https://github.com/dagu-org/dagu/issues/911), [#913](https://github.com/dagu-org/dagu/issues/913), [#921](https://github.com/dagu-org/dagu/issues/921), [#923](https://github.com/dagu-org/dagu/issues/923), [#887](https://github.com/dagu-org/dagu/issues/887), [#922](https://github.com/dagu-org/dagu/issues/922), [#932](https://github.com/dagu-org/dagu/issues/932), [#962](https://github.com/dagu-org/dagu/issues/962)

**⚠️ Note on History Data**: Due to internal improvements, history data from 1.16.x requires migration to work with 1.17.0. Most of other functionality remains stable and compatible except for a few changes. We're committed to maintaining full backward compatibility as much as possible in future releases.

### ❤️ Huge Thanks to Our Contributors

This release wouldn’t exist without the community’s time, sweat, and ideas. In particular:

| Contribution | Author |
|--------------|--------|
| Implemented queue functionality | [@kriyanshii](https://github.com/kriyanshii) |
| Optimized Docker image size **and** split into three baseline images | [@jerry-yuan](https://github.com/jerry-yuan) |
| Allow specifying container name & image platform) | [@vnghia](https://github.com/vnghia) |
| Enhanced repeat-policy – conditions, expected output, and exit codes | [@thefishhat](https://github.com/thefishhat) |
| Countless insightful reviews & feedback | [@ghansham](https://github.com/ghansham) |

*Thank you all for pushing Dagu forward! 💙*

**Your feedback is valuable!** Please test the beta and share your experience:
- 💬 [Join our Discord](https://discord.gg/gpahPUjGRk) for discussions
- 🐛 [Report issues on GitHub](https://github.com/dagu-org/dagu/issues)

To try the beta: `docker run ghcr.io/dagu-org/dagu:1.17.0-beta.1 dagu start-all`

**Full Changelog**: https://github.com/dagu-org/dagu/compare/v1.16.12...v1.17.0-beta.1

## Contributors

<a href="https://github.com/ghansham"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fghansham.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@ghansham"></a>
<a href="https://github.com/jerry-yuan"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fjerry-yuan.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@jerry-yuan"></a>
<a href="https://github.com/kriyanshii"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fkriyanshii.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@kriyanshii"></a>
<a href="https://github.com/thefishhat"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fthefishhat.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@thefishhat"></a>
<a href="https://github.com/vnghia"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fvnghia.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@vnghia"></a>

[Changes][v1.17.0-beta.1]


<a id="v1.16.12"></a>
# [v1.16.12](https://github.com/dagu-org/dagu/releases/tag/v1.16.12) - 2025-05-20

This release incledes a small follow-up fix to v1.16.11.

## Changelog
* [`73386c947f`](https://github.com/dagu-org/dagu/commit/73386c947fb2664bfb4be522963638c99a62bc5a) [[#946](https://github.com/dagu-org/dagu/issues/946)] fix: ensure the same config file is passed down to retry and restart execution as wel ([#948](https://github.com/dagu-org/dagu/issues/948))



[Changes][v1.16.12]


<a id="v1.16.11"></a>
# [v1.16.11](https://github.com/dagu-org/dagu/releases/tag/v1.16.11) - 2025-05-20

This release includes a minor bug fix addressing [#946](https://github.com/dagu-org/dagu/issues/946), where the scheduler did not pass the custom config file path to DAG executions.

## Changelog
* [`7a7ddbfcbd`](https://github.com/dagu-org/dagu/commit/7a7ddbfcbd3e41ab4d44e466ae0c28061c55e130) [[#946](https://github.com/dagu-org/dagu/issues/946)] fix: scheduler does not pass config file to DAG executions ([#947](https://github.com/dagu-org/dagu/issues/947))

## What's Changed
* feat: add `--version` and `--install-dir` options to `installer.sh` by [@yottahmd](https://github.com/yottahmd) in [#944](https://github.com/dagu-org/dagu/pull/944)
* [[#946](https://github.com/dagu-org/dagu/issues/946)] fix: scheduler does not pass config file to DAG executions by [@yottahmd](https://github.com/yottahmd) in [#947](https://github.com/dagu-org/dagu/pull/947)


**Full Changelog**: https://github.com/dagu-org/dagu/compare/v1.16.10...v1.16.11

## Contributors

<a href="https://github.com/yottahmd"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fyottahmd.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@yottahmd"></a>

[Changes][v1.16.11]


<a id="v1.16.10"></a>
# [v1.16.10](https://github.com/dagu-org/dagu/releases/tag/v1.16.10) - 2025-05-16

This release includes a minor fix to the Docker image.

## Changelog
* [`c4bc4ee075`](https://github.com/dagu-org/dagu/commit/c4bc4ee075c4a0f197387cf6782f426a559f7c83) chore: Fix docker image ([#943](https://github.com/dagu-org/dagu/issues/943))

## What's Changed
* chore: Fix the `sudoers` issue in the docker image by [@yottahmd](https://github.com/yottahmd) in [#943](https://github.com/dagu-org/dagu/pull/943)

**Full Changelog**: https://github.com/dagu-org/dagu/compare/v1.16.9...v1.16.10

## Contributors

<a href="https://github.com/yottahmd"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fyottahmd.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@yottahmd"></a>

[Changes][v1.16.10]


<a id="v1.16.9"></a>
# [v1.16.9](https://github.com/dagu-org/dagu/releases/tag/v1.16.9) - 2025-05-11

This is a release to fix an issue in loading legacy basic authentication configurations.

## Changelog
* [`557874ae8f`](https://github.com/dagu-org/dagu/commit/557874ae8f969fe41369587b52ada8e95295f913) [[#926](https://github.com/dagu-org/dagu/issues/926)] fix: set basicauth flag correctly when legacy config key specified ([#929](https://github.com/dagu-org/dagu/issues/929))

## What's Changed
* ci: add docker-next workflow and docs for next preview image by [@yottahmd](https://github.com/yottahmd) in [#928](https://github.com/dagu-org/dagu/pull/928)
* [[#926](https://github.com/dagu-org/dagu/issues/926)] fix: set basicauth flag correctly when legacy config key specified by [@yottahmd](https://github.com/yottahmd) in [#929](https://github.com/dagu-org/dagu/pull/929)


**Full Changelog**: https://github.com/dagu-org/dagu/compare/v1.16.8...v1.16.9

## Contributors

<a href="https://github.com/yottahmd"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fyottahmd.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@yottahmd"></a>

[Changes][v1.16.9]


<a id="v1.16.8"></a>
# [v1.16.8](https://github.com/dagu-org/dagu/releases/tag/v1.16.8) - 2025-05-01

## Changelog
* [`ada8c48f4e`](https://github.com/dagu-org/dagu/commit/ada8c48f4e68818817b0419649b80bf93df2f464) Fix build error in frontend

## What's Changed
* added zoom in zoom out for DAGs. [#774](https://github.com/dagu-org/dagu/issues/774) by [@kriyanshii](https://github.com/kriyanshii) in [#899](https://github.com/dagu-org/dagu/pull/899)
* update Makefile build-ui [#901](https://github.com/dagu-org/dagu/issues/901) by [@kriyanshii](https://github.com/kriyanshii) in [#904](https://github.com/dagu-org/dagu/pull/904)
* [[#905](https://github.com/dagu-org/dagu/issues/905)] DAG visualization updates by [@yottahmd](https://github.com/yottahmd) in [#906](https://github.com/dagu-org/dagu/pull/906)
* added support for custom exit codes on retry [#792](https://github.com/dagu-org/dagu/issues/792) by [@kriyanshii](https://github.com/kriyanshii) in [#902](https://github.com/dagu-org/dagu/pull/902)

## Special thanks
Huge shout-out to [@kriyanshii](https://github.com/kriyanshii)  for multiple awesome contributions recently! 

✨ Retry on Custom Exit Codes: Finer control over task retries. You can specify which exit codes trigger a retry (Resolves [#792](https://github.com/dagu-org/dagu/issues/792)).
✨ Diagram Zoom: Zoom in/out now supported for navigating large DAGs easily (Resolves [#774](https://github.com/dagu-org/dagu/issues/774))
✨ Build Fix: Resolved a key issue preventing the frontend (make build-ui) from building (Fixes [#901](https://github.com/dagu-org/dagu/issues/901)).

**Full Changelog**: https://github.com/dagu-org/dagu/compare/v1.16.7...v1.16.8

## Contributors

<a href="https://github.com/kriyanshii"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fkriyanshii.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@kriyanshii"></a>
<a href="https://github.com/yottahmd"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fyottahmd.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@yottahmd"></a>

[Changes][v1.16.8]


<a id="v1.16.7"></a>
# [v1.16.7](https://github.com/dagu-org/dagu/releases/tag/v1.16.7) - 2025-03-28

## Changelog
* [`6c1a2d9b99`](https://github.com/dagu-org/dagu/commit/6c1a2d9b9985314d10f1567fa8119f2698994cc4) [[#893](https://github.com/dagu-org/dagu/issues/893)] fix: Multi-line Python script result in error ([#894](https://github.com/dagu-org/dagu/issues/894))

## What's Changed
* [[#893](https://github.com/dagu-org/dagu/issues/893)] fix: Multi-line Python script result in error by [@yottahmd](https://github.com/yottahmd) in [#894](https://github.com/dagu-org/dagu/pull/894)


**Full Changelog**: https://github.com/dagu-org/dagu/compare/v1.16.6...v1.16.7

## Contributors

<a href="https://github.com/yottahmd"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fyottahmd.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@yottahmd"></a>

[Changes][v1.16.7]


<a id="v1.16.6"></a>
# [v1.16.6](https://github.com/dagu-org/dagu/releases/tag/v1.16.6) - 2025-03-23

## Changelog
* [`5ab3a06ce4`](https://github.com/dagu-org/dagu/commit/5ab3a06ce4cb31764cee837056ae3c1be8aba8e2) [[#889](https://github.com/dagu-org/dagu/issues/889)] fix: config: Remote node with auth not working ([#890](https://github.com/dagu-org/dagu/issues/890))

## What's Changed
* [[#889](https://github.com/dagu-org/dagu/issues/889)] fix: config: Remote node with auth not working by [@yottahmd](https://github.com/yottahmd) in [#890](https://github.com/dagu-org/dagu/pull/890)


**Full Changelog**: https://github.com/dagu-org/dagu/compare/v1.16.5...v1.16.6

## Contributors

<a href="https://github.com/yottahmd"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fyottahmd.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@yottahmd"></a>

[Changes][v1.16.6]


<a id="v1.16.5"></a>
# [v1.16.5](https://github.com/dagu-org/dagu/releases/tag/v1.16.5) - 2025-03-22

## Changelog
* [`f057884935`](https://github.com/dagu-org/dagu/commit/f0578849351b808eb353d2d7c8f58e1eb8148366) executor/mail: fix: line breaks in email body are not properly converted to `<br />` ([#888](https://github.com/dagu-org/dagu/issues/888))

## What's Changed
* docs: bump python version for docs to 3.11.9 by [@Lewiscowles1986](https://github.com/Lewiscowles1986) in [#865](https://github.com/dagu-org/dagu/pull/865)
* fix: snapshot deprecated warning (Fixes [#869](https://github.com/dagu-org/dagu/issues/869)) by [@arky](https://github.com/arky) in [#870](https://github.com/dagu-org/dagu/pull/870)
* docs: fixed incorrect command syntax in slack notification example by [@david-waterworth](https://github.com/david-waterworth) in [#874](https://github.com/dagu-org/dagu/pull/874)
* [[#877](https://github.com/dagu-org/dagu/issues/877)] ui: fix: base path is not working on web by [@yottahmd](https://github.com/yottahmd) in [#878](https://github.com/dagu-org/dagu/pull/878)
* [[#882](https://github.com/dagu-org/dagu/issues/882)] Fix context handling issue by [@yottahmd](https://github.com/yottahmd) in [#883](https://github.com/dagu-org/dagu/pull/883)
* [[#880](https://github.com/dagu-org/dagu/issues/880)] config: fix: auth token issue by [@yottahmd](https://github.com/yottahmd) in [#884](https://github.com/dagu-org/dagu/pull/884)
* executor/mail: fix: line breaks in email body are not properly converted to `<br />` by [@yottahmd](https://github.com/yottahmd) in [#888](https://github.com/dagu-org/dagu/pull/888)

## New Contributors
* [@david-waterworth](https://github.com/david-waterworth) made their first contribution in [#874](https://github.com/dagu-org/dagu/pull/874)

**Full Changelog**: https://github.com/dagu-org/dagu/compare/v1.16.4...v1.16.5

## Contributors

<a href="https://github.com/Lewiscowles1986"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2FLewiscowles1986.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@Lewiscowles1986"></a>
<a href="https://github.com/arky"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Farky.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@arky"></a>
<a href="https://github.com/david-waterworth"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fdavid-waterworth.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@david-waterworth"></a>
<a href="https://github.com/yottahmd"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fyottahmd.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@yottahmd"></a>

[Changes][v1.16.5]


<a id="v1.16.4"></a>
# [v1.16.4](https://github.com/dagu-org/dagu/releases/tag/v1.16.4) - 2025-02-25

## Changelog
* [`e1b639a06e`](https://github.com/dagu-org/dagu/commit/e1b639a06e485903c4be6496aed639c5176ab992) ci: fix: goreleaser config

## What's Changed
* docs: update quickstart to include messy output example by [@Lewiscowles1986](https://github.com/Lewiscowles1986) in [#851](https://github.com/dagu-org/dagu/pull/851)
* [[#854](https://github.com/dagu-org/dagu/issues/854)] config: fix: `BasePath` config is not working by [@yottahmd](https://github.com/yottahmd) in [#860](https://github.com/dagu-org/dagu/pull/860)
* docs: note block in execute the DAG of QuickStart by [@Lewiscowles1986](https://github.com/Lewiscowles1986) in [#862](https://github.com/dagu-org/dagu/pull/862)
* [[#861](https://github.com/dagu-org/dagu/issues/861)] test: add: integration test for `logDir` configuration by [@yottahmd](https://github.com/yottahmd) in [#864](https://github.com/dagu-org/dagu/pull/864)

## New Contributors
* [@Lewiscowles1986](https://github.com/Lewiscowles1986) made their first contribution in [#851](https://github.com/dagu-org/dagu/pull/851)

**Full Changelog**: https://github.com/dagu-org/dagu/compare/v1.16.3...v1.16.4

## Contributors

<a href="https://github.com/Lewiscowles1986"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2FLewiscowles1986.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@Lewiscowles1986"></a>
<a href="https://github.com/yottahmd"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fyottahmd.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@yottahmd"></a>

[Changes][v1.16.4]


<a id="v1.16.3"></a>
# [v1.16.3](https://github.com/dagu-org/dagu/releases/tag/v1.16.3) - 2025-02-19

## Changelog
* [`ec38f7e5a3`](https://github.com/dagu-org/dagu/commit/ec38f7e5a3e51cc585e9419cf2cfa2f2c1476f09) [[#840](https://github.com/dagu-org/dagu/issues/840)] config: fix: Add `dagsDir` config key ([#850](https://github.com/dagu-org/dagu/issues/850))

## What's Changed
* doc(executor/docker): add doc about working with env by [@vnghia](https://github.com/vnghia) in [#836](https://github.com/dagu-org/dagu/pull/836)
* doc: Add Github sponser button by [@arky](https://github.com/arky) in [#839](https://github.com/dagu-org/dagu/pull/839)
* doc: Update code of conduct by [@arky](https://github.com/arky) in [#843](https://github.com/dagu-org/dagu/pull/843)
* doc: improve contribution guide, add commit standards by [@arky](https://github.com/arky) in [#844](https://github.com/dagu-org/dagu/pull/844)
* [[#846](https://github.com/dagu-org/dagu/issues/846)] Fix: issue in calling sub-DAG by [@yohamta](https://github.com/yohamta) in [#848](https://github.com/dagu-org/dagu/pull/848)
* [[#847](https://github.com/dagu-org/dagu/issues/847)] fix: digraph: overload multiple `dotenv` files by [@yohamta](https://github.com/yohamta) in [#849](https://github.com/dagu-org/dagu/pull/849)
* [[#840](https://github.com/dagu-org/dagu/issues/840)] config: fix: Add `dagsDir` config key by [@yohamta](https://github.com/yohamta) in [#850](https://github.com/dagu-org/dagu/pull/850)

## New Contributors
* [@vnghia](https://github.com/vnghia) made their first contribution in [#836](https://github.com/dagu-org/dagu/pull/836)

**Full Changelog**: https://github.com/dagu-org/dagu/compare/v1.16.2...v1.16.3

## Contributors

<a href="https://github.com/arky"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Farky.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@arky"></a>
<a href="https://github.com/vnghia"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fvnghia.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@vnghia"></a>

[Changes][v1.16.3]


<a id="v1.16.2"></a>
# [v1.16.2](https://github.com/dagu-org/dagu/releases/tag/v1.16.2) - 2025-02-13

## Changelog
* [`28105028cd`](https://github.com/dagu-org/dagu/commit/28105028cda8cca147a8e8b8c30d642bbc982787) [[#830](https://github.com/dagu-org/dagu/issues/830)] executor/docker: support network configuration ([#835](https://github.com/dagu-org/dagu/issues/835))

## What's Changed
* system version fix by [@dayne](https://github.com/dayne) in [#829](https://github.com/dagu-org/dagu/pull/829)
* [[#827](https://github.com/dagu-org/dagu/issues/827)] executor/docker: Fix: command arguments are not evaluated by [@yohamta](https://github.com/yohamta) in [#832](https://github.com/dagu-org/dagu/pull/832)
* [[#831](https://github.com/dagu-org/dagu/issues/831)] cmd: fix: `--config` parameter handling by [@yohamta](https://github.com/yohamta) in [#834](https://github.com/dagu-org/dagu/pull/834)
* [[#830](https://github.com/dagu-org/dagu/issues/830)] executor/docker: support network configuration by [@yohamta](https://github.com/yohamta) in [#835](https://github.com/dagu-org/dagu/pull/835)

## New Contributors
* [@dayne](https://github.com/dayne) made their first contribution in [#829](https://github.com/dagu-org/dagu/pull/829)
* [@dependabot](https://github.com/dependabot) made their first contribution in [#833](https://github.com/dagu-org/dagu/pull/833)

**Full Changelog**: https://github.com/dagu-org/dagu/compare/v1.16.1...v1.16.2

## Contributors

<a href="https://github.com/dayne"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fdayne.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@dayne"></a>
<a href="https://github.com/dependabot"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fdependabot.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@dependabot"></a>

[Changes][v1.16.2]


<a id="v1.16.1"></a>
# [v1.16.1](https://github.com/dagu-org/dagu/releases/tag/v1.16.1) - 2025-02-11

## Changelog
* [`6cd013e81c`](https://github.com/dagu-org/dagu/commit/6cd013e81cf814e0f4356dc85050fd40cef8b38a) [[#821](https://github.com/dagu-org/dagu/issues/821)] Support Snapcraft distribution ([#824](https://github.com/dagu-org/dagu/issues/824))

## Summary
* Bugfixes ([#796](https://github.com/dagu-org/dagu/issues/796) [#799](https://github.com/dagu-org/dagu/issues/799) [#810](https://github.com/dagu-org/dagu/issues/810))
* Support `/health` endpoint
* Support headless mode
* Enhance YAML Editor with field completion and schema validation

## What's Changed
* support for headless mode as mentioned in [#800](https://github.com/dagu-org/dagu/issues/800) by [@kriyanshii](https://github.com/kriyanshii) in [#805](https://github.com/dagu-org/dagu/pull/805)
* chore: Update docker examples (Fixes [#638](https://github.com/dagu-org/dagu/issues/638)) by [@arky](https://github.com/arky) in [#818](https://github.com/dagu-org/dagu/pull/818)
* chore: Email attachment testcase (Fixes [#812](https://github.com/dagu-org/dagu/issues/812)) by [@arky](https://github.com/arky) in [#814](https://github.com/dagu-org/dagu/pull/814)
* [[#820](https://github.com/dagu-org/dagu/issues/820)] Add `/health` endpoint for health check by [@yohamta](https://github.com/yohamta) in [#823](https://github.com/dagu-org/dagu/pull/823)
* digraph: update node.go by [@eltociear](https://github.com/eltociear) in [#788](https://github.com/dagu-org/dagu/pull/788)

## New Contributors
* [@eltociear](https://github.com/eltociear) made their first contribution in [#788](https://github.com/dagu-org/dagu/pull/788)
* [@kennethpjdyer](https://github.com/kennethpjdyer) made their first contribution in [#791](https://github.com/dagu-org/dagu/pull/791)
* [@arky](https://github.com/arky) made their first contribution in [#808](https://github.com/dagu-org/dagu/pull/808)
* [@vhespanha](https://github.com/vhespanha) made their first contribution in [#659](https://github.com/dagu-org/dagu/pull/659)

**Full Changelog**: https://github.com/dagu-org/dagu/compare/v1.16.0...v1.16.1

## Contributors

<a href="https://github.com/arky"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Farky.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@arky"></a>
<a href="https://github.com/kennethpjdyer"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fkennethpjdyer.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@kennethpjdyer"></a>
<a href="https://github.com/kriyanshii"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fkriyanshii.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@kriyanshii"></a>
<a href="https://github.com/vhespanha"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fvhespanha.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@vhespanha"></a>

[Changes][v1.16.1]


<a id="v1.16.0"></a>
# [v1.16.0](https://github.com/dagu-org/dagu/releases/tag/v1.16.0) - 2025-01-09

## Changelog
https://docs.dagu.cloud/reference/changelog

**Full Changelog**: https://github.com/dagu-org/dagu/compare/v1.15.1...v1.16.0

[Changes][v1.16.0]


<a id="v1.15.1"></a>
# [v1.15.1](https://github.com/dagu-org/dagu/releases/tag/v1.15.1) - 2024-12-10

## Changelog
* [`5af66f1a96`](https://github.com/dagu-org/dagu/commit/5af66f1a96ccd557619ec58d43bbb52289518784) [[#736](https://github.com/dagu-org/dagu/issues/736)] add TLS skip verification option for remote node ([#739](https://github.com/dagu-org/dagu/issues/739))

## What's Changed
* [[#736](https://github.com/dagu-org/dagu/issues/736)] add TLS skip verification option for remote node by [@yohamta](https://github.com/yohamta) in [#739](https://github.com/dagu-org/dagu/pull/739)


**Full Changelog**: https://github.com/dagu-org/dagu/compare/v1.15.0...v1.15.1

[Changes][v1.15.1]


<a id="v1.15.0"></a>
# [v1.15.0](https://github.com/dagu-org/dagu/releases/tag/v1.15.0) - 2024-12-06

## What's Changed
* [[#709](https://github.com/dagu-org/dagu/issues/709)] feat: Add `skipIfSuccessful` by [@yohamta](https://github.com/yohamta) in [#712](https://github.com/dagu-org/dagu/pull/712)
* fix: incorrect paths in config docs by [@jonnochoo](https://github.com/jonnochoo) in [#713](https://github.com/dagu-org/dagu/pull/713)
* feat: Support configurable base path for server by [@chrishoage](https://github.com/chrishoage) in [#714](https://github.com/dagu-org/dagu/pull/714)
* docs: added docs for CRON_TZ by [@jonnochoo](https://github.com/jonnochoo) in [#716](https://github.com/dagu-org/dagu/pull/716)
* Improve Dockerfile to reduce amount of config needed in docker-compose by [@chrishoage](https://github.com/chrishoage) in [#723](https://github.com/dagu-org/dagu/pull/723)
* chore: add support for devcontainers by [@jonnochoo](https://github.com/jonnochoo) in [#728](https://github.com/dagu-org/dagu/pull/728)
* add support for default page by [@jonnochoo](https://github.com/jonnochoo) in [#729](https://github.com/dagu-org/dagu/pull/729)
* [[#730](https://github.com/dagu-org/dagu/issues/730)] Add Remote-Node support by [@yohamta](https://github.com/yohamta) in [#731](https://github.com/dagu-org/dagu/pull/731)
* [[#732](https://github.com/dagu-org/dagu/issues/732)] Upgrade to Go 1.23 and Golanci-lint 1.62 by [@yohamta](https://github.com/yohamta) in [#733](https://github.com/dagu-org/dagu/pull/733)

## New Features
### Remote Node support
Dagu now supports managing multiple Dagu servers from a single UI through its remote node feature. This allows you to:

- Monitor and manage DAGs across different environments (dev, staging, prod)
- Access multiple Dagu instances from a centralized UI
- Switch between nodes easily through the UI dropdown
- See [Remote Node Configuration](https://docs.dagu.cloud/reference/changelog) for more details.

**Configuration:**
Remote nodes can be configured by creating `admin.yaml` in `$HOME/.config/dagu/`:

```yaml
# admin.yaml
remoteNodes:
    - name: "prod" # Name of the remote node
      apiBaseUrl: "https://prod.example.com/api/v1" # Base URL of the remote node API
    - name: "staging"
      apiBaseUrl: "https://staging.example.com/api/v1"
```

### Timezone config in schedule
You can specify a cron expression to run within a specific timezone.

```yaml
schedule: "CRON_TZ=Asia/Tokyo 5 9 * * *" # Run at 09:05 in Tokyo
steps:
  - name: scheduled job
    command: job.sh
```

### `skipIfSuccessful`
skipIfSuccessful. When set to true, Dagu will automatically check the last successful run time against the defined schedule. If the DAG has already run successfully since the last scheduled time, the current run will be skipped.

```yaml
schedule: "0 */4 * * *"   # Run every 4 hours
skipIfSuccessful: true    # Skip if already succeeded since last schedule (e.g., manually triggered)
steps:
  - name: resource-intensive-job
    command: process_data.sh
```

## New Contributors
* [@chrishoage](https://github.com/chrishoage) made their first contribution in [#714](https://github.com/dagu-org/dagu/pull/714)

**Full Changelog**: https://github.com/dagu-org/dagu/compare/v1.14.8...v1.15.0

## Contributors

<a href="https://github.com/chrishoage"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fchrishoage.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@chrishoage"></a>
<a href="https://github.com/jonnochoo"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fjonnochoo.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@jonnochoo"></a>

[Changes][v1.15.0]


<a id="v1.14.8"></a>
# [v1.14.8](https://github.com/dagu-org/dagu/releases/tag/v1.14.8) - 2024-11-12

## What's Changed
* fixed bug when using the CRON_TZ= cron expression by [@jonnochoo](https://github.com/jonnochoo) in [#707](https://github.com/dagu-org/dagu/pull/707)


**Full Changelog**: https://github.com/dagu-org/dagu/compare/v1.14.7...v1.14.8

## Contributors

<a href="https://github.com/jonnochoo"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fjonnochoo.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@jonnochoo"></a>

[Changes][v1.14.8]


<a id="v1.14.7"></a>
# [v1.14.7](https://github.com/dagu-org/dagu/releases/tag/v1.14.7) - 2024-11-09

## What's Changed
* chore: update the Dockerfile & docs by [@yohamta](https://github.com/yohamta) in [#699](https://github.com/dagu-org/dagu/pull/699)
* ui: Add Page Limit Input and Improve Case-Insensitive DAG Search by [@yohamta](https://github.com/yohamta) in [#702](https://github.com/dagu-org/dagu/pull/702)
* ui: Reimplement Timeline Chart and Adjust Server Timezone Handling by [@yohamta](https://github.com/yohamta) in [#704](https://github.com/dagu-org/dagu/pull/704)

**Full Changelog**: https://github.com/dagu-org/dagu/compare/v1.14.6...v1.14.7

[Changes][v1.14.7]


<a id="v1.14.6"></a>
# [v1.14.6](https://github.com/dagu-org/dagu/releases/tag/v1.14.6) - 2024-11-06

## What's Changed
* docs: Add docs for special envs by [@yohamta](https://github.com/yohamta) in [#689](https://github.com/dagu-org/dagu/pull/689)
* docs: Add command to run server in docker compose file by [@KMe72](https://github.com/KMe72) in [#693](https://github.com/dagu-org/dagu/pull/693)
* fix: use the server timezone to parse the cron expression by [@jonnochoo](https://github.com/jonnochoo) in [#696](https://github.com/dagu-org/dagu/pull/696)
* docs: add documentation for the time zone configurations by [@yohamta](https://github.com/yohamta) in [#698](https://github.com/dagu-org/dagu/pull/698)
* add: new environment config key `DAGU_TZ` for server & scheduler's time zone setting

## New Contributors
* [@KMe72](https://github.com/KMe72) made their first contribution in [#693](https://github.com/dagu-org/dagu/pull/693)
* [@jonnochoo](https://github.com/jonnochoo) made their first contribution in [#696](https://github.com/dagu-org/dagu/pull/696)

**Full Changelog**: https://github.com/dagu-org/dagu/compare/v1.14.5...v1.14.6

## Contributors

<a href="https://github.com/KMe72"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2FKMe72.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@KMe72"></a>
<a href="https://github.com/jonnochoo"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fjonnochoo.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@jonnochoo"></a>

[Changes][v1.14.6]


<a id="v1.14.5"></a>
# [v1.14.5](https://github.com/dagu-org/dagu/releases/tag/v1.14.5) - 2024-09-24

## Changelog
* [`f7cc980239`](https://github.com/dagu-org/dagu/commit/f7cc980239ffa08b53b9f7d9dea89e812a92817c) ui: fix: DAG groups are not visible on UI ([#686](https://github.com/dagu-org/dagu/issues/686))

## What's Changed
* ui: fix: DAG groups are not visible on UI by [@yohamta](https://github.com/yohamta) in [#686](https://github.com/dagu-org/dagu/pull/686)


**Full Changelog**: https://github.com/dagu-org/dagu/compare/v1.14.4...v1.14.5

[Changes][v1.14.5]


<a id="v1.14.4"></a>
# [v1.14.4](https://github.com/dagu-org/dagu/releases/tag/v1.14.4) - 2024-09-11

## What's Changed
* [ISSUE [#592](https://github.com/dagu-org/dagu/issues/592)] formatting of error text in web ui is not very noticeable by [@halalala222](https://github.com/halalala222) in [#670](https://github.com/dagu-org/dagu/pull/670)
* ISSUE[581] Add Built-in Execution Context Variables by [@halalala222](https://github.com/halalala222) in [#654](https://github.com/dagu-org/dagu/pull/654)
* Fixed build issue by [@yohamta](https://github.com/yohamta) in [#672](https://github.com/dagu-org/dagu/pull/672)
* [[#675](https://github.com/dagu-org/dagu/issues/675)] fix: dashboard page fetch http 422 with DAG list API by [@yohamta](https://github.com/yohamta) in [#680](https://github.com/dagu-org/dagu/pull/680)
* [[#674](https://github.com/dagu-org/dagu/issues/674)] Support env var in ssh config by [@yohamta](https://github.com/yohamta) in [#683](https://github.com/dagu-org/dagu/pull/683)


**Full Changelog**: https://github.com/dagu-org/dagu/compare/v1.14.3...v1.14.4

## Contributors

<a href="https://github.com/halalala222"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fhalalala222.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@halalala222"></a>

[Changes][v1.14.4]


<a id="v1.14.3"></a>
# [v1.14.3](https://github.com/dagu-org/dagu/releases/tag/v1.14.3) - 2024-08-14

## What's Changed
* docs: Update documents for executors by [@yohamta](https://github.com/yohamta) in [daguflow/dagu#642](https://github.com/daguflow/dagu/pull/642)
* doc: Remove duplicate header by [@yohamta](https://github.com/yohamta) in [daguflow/dagu#643](https://github.com/daguflow/dagu/pull/643)
* Update README.md by [@yohamta](https://github.com/yohamta) in [daguflow/dagu#644](https://github.com/daguflow/dagu/pull/644)
* Update documentation for schema definition by [@yohamta](https://github.com/yohamta) in [daguflow/dagu#645](https://github.com/daguflow/dagu/pull/645)
* Add license headers by [@yohamta](https://github.com/yohamta) in [daguflow/dagu#646](https://github.com/daguflow/dagu/pull/646)
* Organize config pkg by [@yohamta](https://github.com/yohamta) in [daguflow/dagu#647](https://github.com/daguflow/dagu/pull/647)
* Improve the test coverage by [@yohamta](https://github.com/yohamta) in [daguflow/dagu#649](https://github.com/daguflow/dagu/pull/649)
* Update codecov config by [@yohamta](https://github.com/yohamta) in [daguflow/dagu#650](https://github.com/daguflow/dagu/pull/650)
* Fix log not loading when '&' in name by [@Lucaslah](https://github.com/Lucaslah) in [daguflow/dagu#651](https://github.com/daguflow/dagu/pull/651)
* ISSUE[584] ssh executor does not support login in password by [@halalala222](https://github.com/halalala222) in [daguflow/dagu#655](https://github.com/daguflow/dagu/pull/655)
* ISSUE[578] Add json bool configuration option to HTTP executor by [@halalala222](https://github.com/halalala222) in [daguflow/dagu#656](https://github.com/daguflow/dagu/pull/656)
* [ISSUE 657] Fix HTTP_HandleCancel in TestAgent_HandleHTTP was a flakey test by [@Kiyo510](https://github.com/Kiyo510) in [daguflow/dagu#658](https://github.com/daguflow/dagu/pull/658)
* [ISSUE 565] Implement Timeout Configuration for DAG Tasks by [@Kiyo510](https://github.com/Kiyo510) in [daguflow/dagu#660](https://github.com/daguflow/dagu/pull/660)
* [ISSUE 661] Add documentation about timeouts for DAG tasks by [@Kiyo510](https://github.com/Kiyo510) in [daguflow/dagu#662](https://github.com/daguflow/dagu/pull/662)
* [ISSUE#653] Add Pagination Parameters to DAG List API to limit the response by [@halalala222](https://github.com/halalala222) in [daguflow/dagu#664](https://github.com/daguflow/dagu/pull/664)
* Additional tests by [@yohamta](https://github.com/yohamta) in [daguflow/dagu#663](https://github.com/daguflow/dagu/pull/663)

## New Contributors
* [@Lucaslah](https://github.com/Lucaslah) made their first contribution in [daguflow/dagu#651](https://github.com/daguflow/dagu/pull/651)
* [@halalala222](https://github.com/halalala222) made their first contribution in [daguflow/dagu#655](https://github.com/daguflow/dagu/pull/655)

**Full Changelog**: https://github.com/daguflow/dagu/compare/v1.14.2...v1.14.3

## Contributors

<a href="https://github.com/Kiyo510"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2FKiyo510.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@Kiyo510"></a>
<a href="https://github.com/Lucaslah"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2FLucaslah.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@Lucaslah"></a>
<a href="https://github.com/halalala222"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fhalalala222.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@halalala222"></a>

[Changes][v1.14.3]


<a id="v1.14.2"></a>
# [v1.14.2](https://github.com/dagu-org/dagu/releases/tag/v1.14.2) - 2024-08-02

## What's Changed
* Update actions/cache from v3 to v4 by [@Kiyo510](https://github.com/Kiyo510) in [daguflow/dagu#630](https://github.com/daguflow/dagu/pull/630)
* Remove container if workflow is cancelled by [@x4204](https://github.com/x4204) in [daguflow/dagu#634](https://github.com/daguflow/dagu/pull/634)
* Fix toggling DAG suspension for DAGs with custom names by [@rocwang](https://github.com/rocwang) in [daguflow/dagu#636](https://github.com/daguflow/dagu/pull/636)
* Improve the pattern of "schedule" in the JSON schema file by [@rocwang](https://github.com/rocwang) in [daguflow/dagu#637](https://github.com/daguflow/dagu/pull/637)
* [[#635](https://github.com/dagu-org/dagu/issues/635)] fix: Parameter does not work by [@yohamta](https://github.com/yohamta) in [daguflow/dagu#641](https://github.com/daguflow/dagu/pull/641)

## Special Thanks
[@zph](https://github.com/zph) [@bbqi](https://github.com/bbqi) for addressing issue [#635](https://github.com/dagu-org/dagu/issues/635) 

**Full Changelog**: https://github.com/daguflow/dagu/compare/v1.14.1...v1.14.2

## Contributors

<a href="https://github.com/Kiyo510"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2FKiyo510.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@Kiyo510"></a>
<a href="https://github.com/bbqi"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fbbqi.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@bbqi"></a>
<a href="https://github.com/rocwang"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Frocwang.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@rocwang"></a>
<a href="https://github.com/x4204"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fx4204.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@x4204"></a>
<a href="https://github.com/zph"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fzph.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@zph"></a>

[Changes][v1.14.2]


<a id="v1.14.1"></a>
# [v1.14.1](https://github.com/dagu-org/dagu/releases/tag/v1.14.1) - 2024-07-22

## Changelog
* [`b1c961bdec`](https://github.com/dagu-org/dagu/commit/b1c961bdecb32b3eb44044061d97ce034512e0ff) fix: action buttons on the DAG list page don't work for DAGs with custom names ([#625](https://github.com/dagu-org/dagu/issues/625))

## What's Changed
* fix: action buttons on the DAG list page don't work for DAGs with custom names by [@rocwang](https://github.com/rocwang) in [dagu-dev/dagu#625](https://github.com/dagu-dev/dagu/pull/625)
* Structured logging by [@yohamta](https://github.com/yohamta) in [dagu-dev/dagu#623](https://github.com/dagu-dev/dagu/pull/623)
* Fix installer script bug by [@yohamta](https://github.com/yohamta) in [dagu-dev/dagu#624](https://github.com/dagu-dev/dagu/pull/624)

## New Contributors
* [@rocwang](https://github.com/rocwang) made their first contribution in [dagu-dev/dagu#625](https://github.com/dagu-dev/dagu/pull/625)

**Full Changelog**: https://github.com/dagu-dev/dagu/compare/v1.14.0...v1.14.1

## Contributors

<a href="https://github.com/rocwang"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Frocwang.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@rocwang"></a>

[Changes][v1.14.1]


<a id="v1.14.0"></a>
# [v1.14.0](https://github.com/dagu-org/dagu/releases/tag/v1.14.0) - 2024-07-18

## Changelog
* [`38695a2f86`](https://github.com/dagu-org/dagu/commit/38695a2f8697bebcef6aac5de443c59f98010fc6) Compliance with XDG ([#619](https://github.com/dagu-org/dagu/issues/619))

## What's Changed
* Compliance with XDG by [@yohamta](https://github.com/yohamta) in [dagu-dev/dagu#619](https://github.com/dagu-dev/dagu/pull/619)
* Fix miscellaneous bugs


**Full Changelog**: https://github.com/dagu-dev/dagu/compare/v1.13.1...v1.14.0

[Changes][v1.14.0]


<a id="v1.13.1"></a>
# [v1.13.1](https://github.com/dagu-org/dagu/releases/tag/v1.13.1) - 2024-07-14

## Changelog
* [`cad2555dc0`](https://github.com/dagu-org/dagu/commit/cad2555dc02e3a444bf803abc7dfba9bba442db5) Capture stderr from Docker container ([#615](https://github.com/dagu-org/dagu/issues/615))

## What's Changed
* (doc) fix http timeout syntax by [@x2ocoder](https://github.com/x2ocoder) in [dagu-dev/dagu#586](https://github.com/dagu-dev/dagu/pull/586)
* (ui) Improve UI when using long values in params by [@zph](https://github.com/zph) in [dagu-dev/dagu#596](https://github.com/dagu-dev/dagu/pull/596)
* (feat) Add service block to goreleaser brew config by [@zph](https://github.com/zph) in [dagu-dev/dagu#597](https://github.com/dagu-dev/dagu/pull/597)
* (feat) Publish homebrew formula to organization by [@zph](https://github.com/zph) in [dagu-dev/dagu#599](https://github.com/dagu-dev/dagu/pull/599)
* (bug) Remove ansi escape codes from UI Log Display by [@zph](https://github.com/zph) in [dagu-dev/dagu#600](https://github.com/dagu-dev/dagu/pull/600)
* (bug) Fix params quoting when using quotes on value by [@zph](https://github.com/zph) in [dagu-dev/dagu#602](https://github.com/dagu-dev/dagu/pull/602)
* (docker executor) Add option to skip docker image pull by [@x4204](https://github.com/x4204) in [dagu-dev/dagu#609](https://github.com/dagu-dev/dagu/pull/609)
* (docker executor) Propagate Docker container exit code by [@x4204](https://github.com/x4204) in [dagu-dev/dagu#613](https://github.com/dagu-dev/dagu/pull/613)
* (docker executor) Capture stderr from Docker container by [@x4204](https://github.com/x4204) in [dagu-dev/dagu#615](https://github.com/dagu-dev/dagu/pull/615)
* (internal) Miscellaneous improvements by [@yohamta](https://github.com/yohamta) in [dagu-dev/dagu#614](https://github.com/dagu-dev/dagu/pull/614)

## New Contributors
* [@x2ocoder](https://github.com/x2ocoder) made their first contribution in [dagu-dev/dagu#586](https://github.com/dagu-dev/dagu/pull/586)
* [@zph](https://github.com/zph) made their first contribution in [dagu-dev/dagu#593](https://github.com/dagu-dev/dagu/pull/593)
* [@x4204](https://github.com/x4204) made their first contribution in [dagu-dev/dagu#609](https://github.com/dagu-dev/dagu/pull/609)

**Full Changelog**: https://github.com/dagu-dev/dagu/compare/v1.13.0...v1.13.1

## Contributors

<a href="https://github.com/x2ocoder"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fx2ocoder.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@x2ocoder"></a>
<a href="https://github.com/x4204"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fx4204.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@x4204"></a>
<a href="https://github.com/zph"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fzph.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@zph"></a>

[Changes][v1.13.1]


<a id="v1.13.0"></a>
# [v1.13.0](https://github.com/dagu-org/dagu/releases/tag/v1.13.0) - 2024-05-25

## Changelog
* be528cd Add `run` and `params` field ([#573](https://github.com/dagu-org/dagu/issues/573))

## New Features
*  Added `run` and `params` field
    You can run another DAG from a DAG by specifying the name:
    ```yaml
    steps:
      - name: running sub_dag
        run: sub_dag      # This can be a path to a file such as `sub_dag.yaml` or `path/to/sub_dag.yaml`
        params: "FOO=BAR" # Optional
    ```

* Accept JSON list to specify command and args
  You can make the DAG to be more readable by using list notation for specifying complex arguments to a command
  ```yaml
  steps:
    - name: step1
      description: print current time
      command: [python, "-c", "import sys; print(sys.argv)", "argument"]
  ```

## What's Changed

* Made DAG scheduler inherit system environment variables on executing steps by [@yohamta](https://github.com/yohamta) in [dagu-dev/dagu#572](https://github.com/dagu-dev/dagu/pull/572)
* Made status of dags configurable by [@kriyanshishah](https://github.com/kriyanshishah) in [dagu-dev/dagu#558](https://github.com/dagu-dev/dagu/pull/558)
* Fixed data race by [@yohamta](https://github.com/yohamta) in [dagu-dev/dagu#560](https://github.com/dagu-dev/dagu/pull/560)
* Reduced the server's load for reading DAGs and status by caching by [@yohamta](https://github.com/yohamta) in [dagu-dev/dagu#564](https://github.com/dagu-dev/dagu/pull/564), [dagu-dev/dagu#569](https://github.com/dagu-dev/dagu/pull/569)
* Added TTL to cache by [@yohamta](https://github.com/yohamta) in [dagu-dev/dagu#569](https://github.com/dagu-dev/dagu/pull/569)
* Fixed the issue of loading empty config file  by [@yohamta](https://github.com/yohamta) in [dagu-dev/dagu#570](https://github.com/dagu-dev/dagu/pull/570)
* Add `run` and `params` field by [@yohamta](https://github.com/yohamta) in [dagu-dev/dagu#573](https://github.com/dagu-dev/dagu/pull/573)
* [#543](https://github.com/dagu-org/dagu/issues/543) Duplicate description removedcate description removed. by [@Kiyo510](https://github.com/Kiyo510) in [dagu-dev/dagu#555](https://github.com/dagu-dev/dagu/pull/555)
* [#544](https://github.com/dagu-org/dagu/issues/544) Corrected documentation regarding Basic Authentication config. by [@Kiyo510](https://github.com/Kiyo510) in [dagu-dev/dagu#554](https://github.com/dagu-dev/dagu/pull/554)
* Fixed linter errors by [@Kiyo510](https://github.com/Kiyo510) in [dagu-dev/dagu#556](https://github.com/dagu-dev/dagu/pull/556)

## New Contributors
* [@Kiyo510](https://github.com/Kiyo510) made their first contribution in [dagu-dev/dagu#555](https://github.com/dagu-dev/dagu/pull/555)
* [@kriyanshishah](https://github.com/kriyanshishah) made their first contribution in [dagu-dev/dagu#558](https://github.com/dagu-dev/dagu/pull/558)

**Full Changelog**: https://github.com/dagu-dev/dagu/compare/v1.12.11...v1.13.0

## Contributors

<a href="https://github.com/Kiyo510"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2FKiyo510.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@Kiyo510"></a>

[Changes][v1.13.0]


<a id="v1.12.11"></a>
# [v1.12.11](https://github.com/dagu-org/dagu/releases/tag/v1.12.11) - 2024-03-19

## Changelog
* 5754d1b fix https configuration errors ([#541](https://github.com/dagu-org/dagu/issues/541))



[Changes][v1.12.11]


<a id="v1.12.10"></a>
# [v1.12.10](https://github.com/dagu-org/dagu/releases/tag/v1.12.10) - 2024-03-14

## Changelog
* a7e7c11 fix race problem

## What's Changed
* Update logo by [@yohamta](https://github.com/yohamta) in [dagu-dev/dagu#523](https://github.com/dagu-dev/dagu/pull/523)
* fix install script by [@fruworg](https://github.com/fruworg) in [dagu-dev/dagu#525](https://github.com/dagu-dev/dagu/pull/525)
* Update Dockerfile [#524](https://github.com/dagu-org/dagu/issues/524) by [@JadRho](https://github.com/JadRho) in [dagu-dev/dagu#528](https://github.com/dagu-dev/dagu/pull/528)
* Small fixes to README.md formatting by [@yarikoptic](https://github.com/yarikoptic) in [dagu-dev/dagu#532](https://github.com/dagu-dev/dagu/pull/532)
* Add codespell support (config and github action to detect new) + make it fix a few typos by [@yarikoptic](https://github.com/yarikoptic) in [dagu-dev/dagu#531](https://github.com/dagu-dev/dagu/pull/531)
* Init dag dir by [@HtcOrange](https://github.com/HtcOrange) in [dagu-dev/dagu#537](https://github.com/dagu-dev/dagu/pull/537)

## New Contributors
* [@fruworg](https://github.com/fruworg) made their first contribution in [dagu-dev/dagu#525](https://github.com/dagu-dev/dagu/pull/525)
* [@JadRho](https://github.com/JadRho) made their first contribution in [dagu-dev/dagu#528](https://github.com/dagu-dev/dagu/pull/528)
* [@yarikoptic](https://github.com/yarikoptic) made their first contribution in [dagu-dev/dagu#532](https://github.com/dagu-dev/dagu/pull/532)
* [@HtcOrange](https://github.com/HtcOrange) made their first contribution in [dagu-dev/dagu#537](https://github.com/dagu-dev/dagu/pull/537)

**Full Changelog**: https://github.com/dagu-dev/dagu/compare/v1.12.9...v1.12.10

## Contributors

<a href="https://github.com/HtcOrange"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2FHtcOrange.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@HtcOrange"></a>
<a href="https://github.com/JadRho"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2FJadRho.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@JadRho"></a>
<a href="https://github.com/fruworg"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Ffruworg.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@fruworg"></a>
<a href="https://github.com/yarikoptic"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fyarikoptic.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@yarikoptic"></a>

[Changes][v1.12.10]


<a id="v1.12.9"></a>
# [v1.12.9](https://github.com/dagu-org/dagu/releases/tag/v1.12.9) - 2024-01-07

## Changelog
* c9435a8 Fix command flag parsing issue ([#522](https://github.com/dagu-org/dagu/issues/522))

## What's Changed
* Fix linter error by [@rafiramadhana](https://github.com/rafiramadhana) in [dagu-dev/dagu#515](https://github.com/dagu-dev/dagu/pull/515)
* Force assets removal by [@rafiramadhana](https://github.com/rafiramadhana) in [dagu-dev/dagu#519](https://github.com/dagu-dev/dagu/pull/519)
* Fix command flag duplication issue by [@yohamta](https://github.com/yohamta) in [dagu-dev/dagu#522](https://github.com/dagu-dev/dagu/pull/522)


**Full Changelog**: https://github.com/dagu-dev/dagu/compare/v1.12.8...v1.12.9

## Contributors

<a href="https://github.com/rafiramadhana"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Frafiramadhana.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@rafiramadhana"></a>

[Changes][v1.12.9]


<a id="v1.12.8"></a>
# [v1.12.8](https://github.com/dagu-org/dagu/releases/tag/v1.12.8) - 2024-01-01

## Changelog
* cc0c36e Update docs ([#512](https://github.com/dagu-org/dagu/issues/512))

## What's Changed
* The field `depends` is added to docs `Available Fields for Steps` by [@ArseniySavin](https://github.com/ArseniySavin) in [dagu-dev/dagu#503](https://github.com/dagu-dev/dagu/pull/503)
* `Cron expression generator link` added into the scheduler for help. by [@ArseniySavin](https://github.com/ArseniySavin) in [dagu-dev/dagu#507](https://github.com/dagu-dev/dagu/pull/507)
* Token auth by [@smekuria1](https://github.com/smekuria1) in [dagu-dev/dagu#508](https://github.com/dagu-dev/dagu/pull/508)
* Skip basic auth when auth token is set by [@rafiramadhana](https://github.com/rafiramadhana) in [dagu-dev/dagu#511](https://github.com/dagu-dev/dagu/pull/511)
* Update docs by [@yohamta](https://github.com/yohamta) in [dagu-dev/dagu#512](https://github.com/dagu-dev/dagu/pull/512)

## New Contributors
* [@ArseniySavin](https://github.com/ArseniySavin) made their first contribution in [dagu-dev/dagu#503](https://github.com/dagu-dev/dagu/pull/503)
* [@smekuria1](https://github.com/smekuria1) made their first contribution in [dagu-dev/dagu#508](https://github.com/dagu-dev/dagu/pull/508)
* [@rafiramadhana](https://github.com/rafiramadhana) made their first contribution in [dagu-dev/dagu#511](https://github.com/dagu-dev/dagu/pull/511)

**Full Changelog**: https://github.com/dagu-dev/dagu/compare/v1.12.7...v1.12.8

## Contributors

<a href="https://github.com/ArseniySavin"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2FArseniySavin.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@ArseniySavin"></a>
<a href="https://github.com/rafiramadhana"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Frafiramadhana.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@rafiramadhana"></a>
<a href="https://github.com/smekuria1"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fsmekuria1.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@smekuria1"></a>

[Changes][v1.12.8]


<a id="v1.12.7"></a>
# [v1.12.7](https://github.com/dagu-org/dagu/releases/tag/v1.12.7) - 2023-11-03

## Changelog
* a9fd44a add start-all command ([#498](https://github.com/dagu-org/dagu/issues/498))

## What's Changed
* Add `start-all` command by [@yohamta](https://github.com/yohamta) in [dagu-dev/dagu#498](https://github.com/dagu-dev/dagu/pull/498)


**Full Changelog**: https://github.com/dagu-dev/dagu/compare/v1.12.6...v1.12.7

[Changes][v1.12.7]


<a id="v1.12.6"></a>
# [v1.12.6](https://github.com/dagu-org/dagu/releases/tag/v1.12.6) - 2023-10-27

## What's Changed
* Add attach logs to report mail by [@triole](https://github.com/triole)

## New Contributors
* [@triole](https://github.com/triole) made their first contribution in [dagu-dev/dagu#495](https://github.com/dagu-dev/dagu/pull/495)

**Full Changelog**: https://github.com/dagu-dev/dagu/compare/v1.12.5...v1.12.6

## Contributors

<a href="https://github.com/triole"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Ftriole.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@triole"></a>

[Changes][v1.12.6]


<a id="v1.12.5"></a>
# [v1.12.5](https://github.com/dagu-org/dagu/releases/tag/v1.12.5) - 2023-10-18

## Changelog
* cfbd772 remove protobuf gen part from the release workflow

## What's Changed
* Just added generated protobuf files to get `go install` work and nothing else.


**Full Changelog**: https://github.com/dagu-dev/dagu/compare/v1.12.4...v1.12.5

[Changes][v1.12.5]


<a id="v1.12.4"></a>
# [v1.12.4](https://github.com/dagu-org/dagu/releases/tag/v1.12.4) - 2023-09-21

## Changelog
* e99b7ae fix basic auth issue ([#483](https://github.com/dagu-org/dagu/issues/483))

## What's Changed
* fix basic auth issue by [@yohamta](https://github.com/yohamta) in [dagu-dev/dagu#483](https://github.com/dagu-dev/dagu/pull/483)


**Full Changelog**: https://github.com/dagu-dev/dagu/compare/v1.12.3...v1.12.4

[Changes][v1.12.4]


<a id="v1.12.3"></a>
# [v1.12.3](https://github.com/dagu-org/dagu/releases/tag/v1.12.3) - 2023-09-21

## Changelog
* 8fa6681 fix api base url to be the same host ([#482](https://github.com/dagu-org/dagu/issues/482))

## What's Changed
* fix api base url to be the same host by [@yohamta](https://github.com/yohamta) in [dagu-dev/dagu#482](https://github.com/dagu-dev/dagu/pull/482)


**Full Changelog**: https://github.com/dagu-dev/dagu/compare/v1.12.2...v1.12.3

[Changes][v1.12.3]


<a id="v1.12.2"></a>
# [v1.12.2](https://github.com/dagu-org/dagu/releases/tag/v1.12.2) - 2023-09-20

## Changelog
* 55521af fix bug ([#481](https://github.com/dagu-org/dagu/issues/481))

## What's Changed
* Bugfix by [@yohamta](https://github.com/yohamta) in [dagu-dev/dagu#481](https://github.com/dagu-dev/dagu/pull/481)


**Full Changelog**: https://github.com/dagu-dev/dagu/compare/v1.12.1...v1.12.2

[Changes][v1.12.2]


<a id="v1.12.1"></a>
# [v1.12.1](https://github.com/dagu-org/dagu/releases/tag/v1.12.1) - 2023-09-10

## Changelog
* 84bcb19 fix server bug

**Full Changelog**: https://github.com/dagu-dev/dagu/compare/v1.12.0...v1.12.1

[Changes][v1.12.1]


<a id="v1.12.0"></a>
# [v1.12.0](https://github.com/dagu-org/dagu/releases/tag/v1.12.0) - 2023-09-10

## What's Changed
* Fixed some bugs and improved Web UI performance a bit

## New Contributors
* [@dat-adi](https://github.com/dat-adi) made their first contribution in [dagu-dev/dagu#474](https://github.com/dagu-dev/dagu/pull/474)

**Full Changelog**: https://github.com/dagu-dev/dagu/compare/v1.11.0...v1.12.0

## Contributors

<a href="https://github.com/dat-adi"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fdat-adi.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@dat-adi"></a>

[Changes][v1.12.0]


<a id="v1.11.0"></a>
# [v1.11.0](https://github.com/dagu-org/dagu/releases/tag/v1.11.0) - 2023-08-11

## What's Changed
* Simplify UI by [@yohamta](https://github.com/yohamta) in [dagu-dev/dagu#460](https://github.com/dagu-dev/dagu/pull/460)


**Full Changelog**: https://github.com/dagu-dev/dagu/compare/v1.10.6...v1.11.0

[Changes][v1.11.0]


<a id="v1.10.6"></a>
# [v1.10.6](https://github.com/dagu-org/dagu/releases/tag/v1.10.6) - 2023-08-10

## Changelog
* 92f8f75 fix bundling issue

## What's Changed
* Minor documentation review by [@fljdin](https://github.com/fljdin) in [dagu-dev/dagu#456](https://github.com/dagu-dev/dagu/pull/456)
* convert dag.Step struct to protocol buffers by [@garunitule](https://github.com/garunitule) in [dagu-dev/dagu#449](https://github.com/dagu-dev/dagu/pull/449)
* Add YAML syntax highlighting support on DAG editor by [@yohamta](https://github.com/yohamta) in [dagu-dev/dagu#459](https://github.com/dagu-dev/dagu/pull/459)

## New Contributors
* [@fljdin](https://github.com/fljdin) made their first contribution in [dagu-dev/dagu#456](https://github.com/dagu-dev/dagu/pull/456)

**Full Changelog**: https://github.com/dagu-dev/dagu/compare/v1.10.5...v1.10.6

## Contributors

<a href="https://github.com/fljdin"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Ffljdin.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@fljdin"></a>
<a href="https://github.com/garunitule"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fgarunitule.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@garunitule"></a>

[Changes][v1.10.6]


<a id="v1.10.5"></a>
# [v1.10.5](https://github.com/dagu-org/dagu/releases/tag/v1.10.5) - 2023-05-21

## Changelog

## What's Changed
* Add Sphinx Documentation by [@yohamta](https://github.com/yohamta) in [dagu-dev/dagu#431](https://github.com/dagu-dev/dagu/pull/431)
* update go version from 1.18 to 1.19 by [@yohamta](https://github.com/yohamta) in [dagu-dev/dagu#433](https://github.com/dagu-dev/dagu/pull/433)
* Add support for user-defined functions and call field in YAML format by [@garunitule](https://github.com/garunitule) in [dagu-dev/dagu#444](https://github.com/dagu-dev/dagu/pull/444)
* Add support for TLS for admin server by [@yohamta](https://github.com/yohamta) in [dagu-dev/dagu#445](https://github.com/dagu-dev/dagu/pull/445)
* Explanation about HTTPS configuration by [@yohamta](https://github.com/yohamta) in [dagu-dev/dagu#446](https://github.com/dagu-dev/dagu/pull/446)

## New Contributors
* [@garunitule](https://github.com/garunitule) made their first contribution in [dagu-dev/dagu#444](https://github.com/dagu-dev/dagu/pull/444)

**Full Changelog**: https://github.com/dagu-dev/dagu/compare/v1.10.4...v1.10.5

## Contributors

<a href="https://github.com/garunitule"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fgarunitule.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@garunitule"></a>

[Changes][v1.10.5]


<a id="v1.10.4"></a>
# [v1.10.4](https://github.com/dagu-org/dagu/releases/tag/v1.10.4) - 2023-04-01

## Changelog
* f851bd0 fix: cycle detection bug ([#427](https://github.com/dagu-org/dagu/issues/427))

## What's Changed
* fix: server command host & port description swapped by [@stefaan1o](https://github.com/stefaan1o) in [yohamta/dagu#420](https://github.com/yohamta/dagu/pull/420)
* fix: cycle detection bug by [@1005281342](https://github.com/1005281342) in [yohamta/dagu#427](https://github.com/yohamta/dagu/pull/427)

## New Contributors
* [@1005281342](https://github.com/1005281342) made their first contribution in [yohamta/dagu#427](https://github.com/yohamta/dagu/pull/427)

**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.10.3...v1.10.4

## Contributors

<a href="https://github.com/1005281342"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2F1005281342.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@1005281342"></a>
<a href="https://github.com/stefaan1o"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fstefaan1o.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@stefaan1o"></a>

[Changes][v1.10.4]


<a id="v1.10.3"></a>
# [v1.10.3](https://github.com/dagu-org/dagu/releases/tag/v1.10.3) - 2023-03-28

## Changelog
* b0a4fd8 Fixed incorrect app home directory bug ([#424](https://github.com/dagu-org/dagu/issues/424))



[Changes][v1.10.3]


<a id="v1.10.2"></a>
# [v1.10.2](https://github.com/dagu-org/dagu/releases/tag/v1.10.2) - 2023-03-19

## Changelog
* d0b5465 v1.10.2 ([#413](https://github.com/dagu-org/dagu/issues/413))

## What's Changed
* fix link to example by [@stefaan1o](https://github.com/stefaan1o) in [yohamta/dagu#409](https://github.com/yohamta/dagu/pull/409)
* v1.10.2 by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#413](https://github.com/yohamta/dagu/pull/413)


**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.10.1...v1.10.2

## Contributors

<a href="https://github.com/stefaan1o"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fstefaan1o.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@stefaan1o"></a>

[Changes][v1.10.2]


<a id="v1.10.1"></a>
# [v1.10.1](https://github.com/dagu-org/dagu/releases/tag/v1.10.1) - 2023-03-15

## Changelog
- Added support for sending emails within workflows through the new `mail` executor.
- Enhanced the `http` executor to include a silent option, which allows users to suppress unnecessary logging when executing HTTP requests.
- Improved Web UI to display modal on DAG execution start/stop/cancel.
- Updated the Web UI to improve the time format for the `Next Run` field.
- Enhanced the functionality of the `jq` executor by introducing a new `raw` option.

[Changes][v1.10.1]


<a id="v1.9.4"></a>
# [v1.9.4](https://github.com/dagu-org/dagu/releases/tag/v1.9.4) - 2023-03-02

## Changelog
* 7eead4f Add jq executor ([#391](https://github.com/dagu-org/dagu/issues/391))

## What's Changed
* Add an example `docker-compose.yml` and documentation by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#387](https://github.com/yohamta/dagu/pull/387)
* Update REST API documentation by [@vkill](https://github.com/vkill) in [yohamta/dagu#388](https://github.com/yohamta/dagu/pull/388)
* Update HTTP Executor to expand environment variables by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#390](https://github.com/yohamta/dagu/pull/390)
* Add `jq` executor that can be used to transform and query JSON by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#391](https://github.com/yohamta/dagu/pull/391)

## New Contributors
* [@vkill](https://github.com/vkill) made their first contribution in [yohamta/dagu#388](https://github.com/yohamta/dagu/pull/388)

**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.9.3...v1.9.4

## Contributors

<a href="https://github.com/vkill"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fvkill.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@vkill"></a>

[Changes][v1.9.4]


<a id="v1.9.3"></a>
# [v1.9.3](https://github.com/dagu-org/dagu/releases/tag/v1.9.3) - 2023-01-19

## Changelog
* 4a8b2dc fix [#382](https://github.com/dagu-org/dagu/issues/382) ([#384](https://github.com/dagu-org/dagu/issues/384))

## What's Changed
* update examples for docker container by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#365](https://github.com/yohamta/dagu/pull/365)
* Fixed grammar in README.md by [@emcassi](https://github.com/emcassi) in [yohamta/dagu#366](https://github.com/yohamta/dagu/pull/366)
* Fix lint errors by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#370](https://github.com/yohamta/dagu/pull/370)
* Add env field to step in DAG spec by [@TahirAIi](https://github.com/TahirAIi) in [yohamta/dagu#378](https://github.com/yohamta/dagu/pull/378)
* fix [#382](https://github.com/dagu-org/dagu/issues/382) by [@zwZjut](https://github.com/zwZjut) in [yohamta/dagu#384](https://github.com/yohamta/dagu/pull/384)

## New Contributors
* [@emcassi](https://github.com/emcassi) made their first contribution in [yohamta/dagu#366](https://github.com/yohamta/dagu/pull/366)
* [@TahirAIi](https://github.com/TahirAIi) made their first contribution in [yohamta/dagu#378](https://github.com/yohamta/dagu/pull/378)
* [@zwZjut](https://github.com/zwZjut) made their first contribution in [yohamta/dagu#384](https://github.com/yohamta/dagu/pull/384)

**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.9.2...v1.9.3

## Contributors

<a href="https://github.com/TahirAIi"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2FTahirAIi.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@TahirAIi"></a>
<a href="https://github.com/emcassi"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Femcassi.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@emcassi"></a>
<a href="https://github.com/zwZjut"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2FzwZjut.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@zwZjut"></a>

[Changes][v1.9.3]


<a id="v1.9.2"></a>
# [v1.9.2](https://github.com/dagu-org/dagu/releases/tag/v1.9.2) - 2022-11-20

## Changelog
* a6998b5 Add docker executor ([#363](https://github.com/dagu-org/dagu/issues/363))

## What's Changed
* Add docker executor by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#363](https://github.com/yohamta/dagu/pull/363)


**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.9.1...v1.9.2

[Changes][v1.9.2]


<a id="v1.9.1"></a>
# [v1.9.1](https://github.com/dagu-org/dagu/releases/tag/v1.9.1) - 2022-10-20

## Changelog
* 591f279 Update Dockerfile ([#356](https://github.com/dagu-org/dagu/issues/356))

## What's Changed
* Allow STMP config to be loaded from environment variables  by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#355](https://github.com/yohamta/dagu/pull/355)
* Update Dockerfile to allow TZ env to set timezone by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#356](https://github.com/yohamta/dagu/pull/356)


**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.9.0...v1.9.1

[Changes][v1.9.1]


<a id="v1.9.0"></a>
# [v1.9.0](https://github.com/dagu-org/dagu/releases/tag/v1.9.0) - 2022-10-10

## Changelog
* c33702f Add SSH Executor ([#351](https://github.com/dagu-org/dagu/issues/351))

## What's Changed
* fix misspelled word by [@SimonWaldherr](https://github.com/SimonWaldherr) in [yohamta/dagu#344](https://github.com/yohamta/dagu/pull/344)
* Fix web UI issues by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#350](https://github.com/yohamta/dagu/pull/350)
* Add SSH Executor by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#351](https://github.com/yohamta/dagu/pull/351)

## New Contributors
* [@SimonWaldherr](https://github.com/SimonWaldherr) made their first contribution in [yohamta/dagu#344](https://github.com/yohamta/dagu/pull/344)

**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.8.8...v1.9.0

## Contributors

<a href="https://github.com/SimonWaldherr"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2FSimonWaldherr.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@SimonWaldherr"></a>

[Changes][v1.9.0]


<a id="v1.8.8"></a>
# [v1.8.8](https://github.com/dagu-org/dagu/releases/tag/v1.8.8) - 2022-09-20

## Changelog
* 9553660 Merge pull request [#339](https://github.com/dagu-org/dagu/issues/339) from yohamta/feat/add-auth-stmp-mailer

## What's Changed
* refactor: internal/controller by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#337](https://github.com/yohamta/dagu/pull/337)
* Add auth stmp mailer by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#339](https://github.com/yohamta/dagu/pull/339)


**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.8.7...v1.8.8

[Changes][v1.8.8]


<a id="v1.8.7"></a>
# [v1.8.7](https://github.com/dagu-org/dagu/releases/tag/v1.8.7) - 2022-09-18

## Changelog
* a7b9e6e Merge pull request [#336](https://github.com/dagu-org/dagu/issues/336) from RamonEspinosa/fix/dag-table-a11y-issues

## What's Changed
* Add docker executor boilerplate by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#330](https://github.com/yohamta/dagu/pull/330)
* Remove unused files by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#333](https://github.com/yohamta/dagu/pull/333)
* Fix/dag table a11y issues by [@RamonEspinosa](https://github.com/RamonEspinosa) in [yohamta/dagu#336](https://github.com/yohamta/dagu/pull/336)

## New Contributors
* [@RamonEspinosa](https://github.com/RamonEspinosa) made their first contribution in [yohamta/dagu#336](https://github.com/yohamta/dagu/pull/336)

**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.8.6...v1.8.7

## Contributors

<a href="https://github.com/RamonEspinosa"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2FRamonEspinosa.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@RamonEspinosa"></a>

[Changes][v1.8.7]


<a id="v1.8.6"></a>
# [v1.8.6](https://github.com/dagu-org/dagu/releases/tag/v1.8.6) - 2022-09-10

## Changelog
* 558a781 Merge pull request [#329](https://github.com/dagu-org/dagu/issues/329) from yohamta/fix/signal-on-stop

## What's Change
* Fix signalOnStop bug by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#329](https://github.com/yohamta/dagu/pull/329)


**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.8.5...v1.8.6

[Changes][v1.8.6]


<a id="v1.8.5"></a>
# [v1.8.5](https://github.com/dagu-org/dagu/releases/tag/v1.8.5) - 2022-09-09

## Changelog
* 51decc1 Merge pull request [#323](https://github.com/dagu-org/dagu/issues/323) from Arvintian/main



[Changes][v1.8.5]


<a id="v1.8.4"></a>
# [v1.8.4](https://github.com/dagu-org/dagu/releases/tag/v1.8.4) - 2022-09-06

## Changelog
* 12828cc Merge pull request [#319](https://github.com/dagu-org/dagu/issues/319) from yohamta/feat/update-dialog-message

## What's Changed
* allow parameters to be given on start button by [@Arvintian](https://github.com/Arvintian) in [yohamta/dagu#315](https://github.com/yohamta/dagu/pull/315)
* fix an issue with starting a DAG with params by [@Arvintian](https://github.com/Arvintian) in [yohamta/dagu#317](https://github.com/yohamta/dagu/pull/317)
* Add `restart` command by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#318](https://github.com/yohamta/dagu/pull/318)


**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.8.3...v1.8.4

## Contributors

<a href="https://github.com/Arvintian"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2FArvintian.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@Arvintian"></a>

[Changes][v1.8.4]


<a id="v1.8.3"></a>
# [v1.8.3](https://github.com/dagu-org/dagu/releases/tag/v1.8.3) - 2022-09-02

## Changelog
* 496e1a1 Merge pull request [#313](https://github.com/dagu-org/dagu/issues/313) from yohamta/feat/stderr

## What's Changed
* Add `stderr` field to separate stderr output from the normal log file. by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#313](https://github.com/yohamta/dagu/pull/313)


**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.8.2...v1.8.3

[Changes][v1.8.3]


<a id="v1.8.2"></a>
# [v1.8.2](https://github.com/dagu-org/dagu/releases/tag/v1.8.2) - 2022-09-02

## Changelog
* 7aea586 Merge pull request [#311](https://github.com/dagu-org/dagu/issues/311) from yohamta/feat/multiple-schedule

## What's Changed
* Allow multiple start/stop schedule by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#311](https://github.com/yohamta/dagu/pull/311)


**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.8.1...v1.8.2

[Changes][v1.8.2]


<a id="v1.8.1"></a>
# [v1.8.1](https://github.com/dagu-org/dagu/releases/tag/v1.8.1) - 2022-09-01

## Changelog
* bcbc5c3 Merge pull request [#297](https://github.com/dagu-org/dagu/issues/297) from yohamta/feat/delete

## What's Changed
* Add function to delete DAGs on Web UI by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#297](https://github.com/yohamta/dagu/pull/297)


**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.8.0...v1.8.1

[Changes][v1.8.1]


<a id="v1.8.0"></a>
# [v1.8.0](https://github.com/dagu-org/dagu/releases/tag/v1.8.0) - 2022-08-31

## Changelog
* dc1cf21 Merge pull request [#306](https://github.com/dagu-org/dagu/issues/306) from yohamta/feat/schedule-process

## What's Changed
* Allow defining `start` and `stop` schedule by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#306](https://github.com/yohamta/dagu/pull/306)


**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.7.11...v1.8.0

[Changes][v1.8.0]


<a id="v1.7.11"></a>
# [v1.7.11](https://github.com/dagu-org/dagu/releases/tag/v1.7.11) - 2022-08-30

## What's Changed
* admin-web: fix filtering issue by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#302](https://github.com/yohamta/dagu/pull/302)


**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.7.10...v1.7.11

[Changes][v1.7.11]


<a id="v1.7.10"></a>
# [v1.7.10](https://github.com/dagu-org/dagu/releases/tag/v1.7.10) - 2022-08-30

## What's Changed
* admin-web: fix modal issue by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#295](https://github.com/yohamta/dagu/pull/295)
* admin-web: fix react-table issue by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#301](https://github.com/yohamta/dagu/pull/301)


**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.7.9...v1.7.10

[Changes][v1.7.10]


<a id="v1.7.9"></a>
# [v1.7.9](https://github.com/dagu-org/dagu/releases/tag/v1.7.9) - 2022-08-26

## Changelog
* 2713b23 Merge pull request [#294](https://github.com/dagu-org/dagu/issues/294) from yohamta/feat/admin-web

## What's Changed
* admin-web: Improved the search function by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#293](https://github.com/yohamta/dagu/pull/293)

**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.7.8...v1.7.9

[Changes][v1.7.9]


<a id="v1.7.8"></a>
# [v1.7.8](https://github.com/dagu-org/dagu/releases/tag/v1.7.8) - 2022-08-25

## Changelog
* 0652945 Merge pull request [#292](https://github.com/dagu-org/dagu/issues/292) from yohamta/feat/grep

## What's Changed
* Added `Search` page on the Web UI by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#292](https://github.com/yohamta/dagu/pull/292)
![dagu Admin Web 2022-08-25 23-24-25](https://user-images.githubusercontent.com/1475839/186691609-3aa4aeba-b6c8-43f5-871e-731c679d6dc9.jpg)

**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.7.7...v1.7.8

[Changes][v1.7.8]


<a id="v1.7.7"></a>
# [v1.7.7](https://github.com/dagu-org/dagu/releases/tag/v1.7.7) - 2022-08-22

## Changelog
* 8b34bc0 Merge pull request [#289](https://github.com/dagu-org/dagu/issues/289) from yohamta/feat/signal-on-step

## What's Changed
* Enhanced CLI help for "dagu server" by [@fishnux](https://github.com/fishnux) in [yohamta/dagu#286](https://github.com/yohamta/dagu/pull/286)
* Fix log page issue by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#287](https://github.com/yohamta/dagu/pull/287)
* Add version text on Web UI by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#288](https://github.com/yohamta/dagu/pull/288)
* Add function to specify signal on stop by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#289](https://github.com/yohamta/dagu/pull/289)

## New Contributors
* [@fishnux](https://github.com/fishnux) made their first contribution in [yohamta/dagu#286](https://github.com/yohamta/dagu/pull/286)

**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.7.6...v1.7.7

## Contributors

<a href="https://github.com/fishnux"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Ffishnux.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@fishnux"></a>

[Changes][v1.7.7]


<a id="v1.7.6"></a>
# [v1.7.6](https://github.com/dagu-org/dagu/releases/tag/v1.7.6) - 2022-08-19

## Changelog
* 1ab95f8 Merge pull request [#284](https://github.com/dagu-org/dagu/issues/284) from yohamta/admin-port-option

## What's Changed
* Fix execution history sorting issue in the Web UI by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#271](https://github.com/yohamta/dagu/pull/271)
* Fix scheduler to watch dags efficiently by [@Arvintian](https://github.com/Arvintian) in [yohamta/dagu#268](https://github.com/yohamta/dagu/pull/268)
* Fix http executor set timeout by [@Arvintian](https://github.com/Arvintian) in [yohamta/dagu#278](https://github.com/yohamta/dagu/pull/278)
* Update default dag log directory by [@Arvintian](https://github.com/Arvintian) in [yohamta/dagu#282](https://github.com/yohamta/dagu/pull/282)
Current: `${DAG_HOME}/logs/${name}/${name}`
Update: `${DAG_HOME}/logs/dags/${name}`
* Reduce admin-web bundle.js size by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#273](https://github.com/yohamta/dagu/pull/273)
* Fix other small issues in the Web UI by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#283](https://github.com/yohamta/dagu/pull/283)
* Add `host`, `port`, `dags` option to the `server` command by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#284](https://github.com/yohamta/dagu/pull/284)


**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.7.5...v1.7.6

## Contributors

<a href="https://github.com/Arvintian"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2FArvintian.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@Arvintian"></a>

[Changes][v1.7.6]


<a id="v1.7.5"></a>
# [v1.7.5](https://github.com/dagu-org/dagu/releases/tag/v1.7.5) - 2022-08-15

## Changelog
* d7c6b76 Merge pull request [#266](https://github.com/dagu-org/dagu/issues/266) from yohamta/develop



[Changes][v1.7.5]


<a id="v1.7.4"></a>
# [v1.7.4](https://github.com/dagu-org/dagu/releases/tag/v1.7.4) - 2022-08-15

## Changelog
* f6e912c Merge pull request [#264](https://github.com/dagu-org/dagu/issues/264) from yohamta/develop

## What's Changed
* fix specialchar parsing issue by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#263](https://github.com/yohamta/dagu/pull/263)
* change group row color by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#264](https://github.com/yohamta/dagu/pull/264)


**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.7.3...v1.7.4

[Changes][v1.7.4]


<a id="v1.7.3"></a>
# [v1.7.3](https://github.com/dagu-org/dagu/releases/tag/v1.7.3) - 2022-08-14

## What's Changed
* Update Logo and default navbar color by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#242](https://github.com/yohamta/dagu/pull/242)
* Add `--config` option for all commands by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#244](https://github.com/yohamta/dagu/pull/244)
* Add `baseConfig` option to admin config to specify base DAG config by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#245](https://github.com/yohamta/dagu/pull/245)
* Add `DAGU_HOME` env variable to set the Dagu's internal use directory by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#246](https://github.com/yohamta/dagu/pull/246)
* Update DAG appearance in the Web UI by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#253](https://github.com/yohamta/dagu/pull/253)
* Add `navbarColor` and `navbarTitle` field to admin config by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#256](https://github.com/yohamta/dagu/pull/256)
* Fix Web UI to remember search text & tags when browser back by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#258](https://github.com/yohamta/dagu/pull/258)
* Added a function to switch to vertical graph by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#261](https://github.com/yohamta/dagu/pull/261)


**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.7.0...v1.7.1

## Breaking change
This change disables env config below:

- DAGU__ADMIN_NAVBAR_COLOR
- DAGU__ADMIN_NAVBAR_TITLE
- DAGU__ADMIN_CONFIG
- DAGU__ADMIN_PORT
- DAGU__ADMIN_LOGS_DIR
- DAGU__ADMIN_DAGS_DIR

The user can still set config values within the admin config file:

example:
```yaml
navbarColor: red
navbarTitle: production
```

For the internal-use directory path you can set the environment variable `DAGU_HOME` (default: `~/.dagu`).

**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.6.9...v1.7.0

[Changes][v1.7.3]


<a id="v1.6.9"></a>
# [v1.6.9](https://github.com/dagu-org/dagu/releases/tag/v1.6.9) - 2022-08-05

## Changelog
* 5f5ad54 Merge pull request [#240](https://github.com/dagu-org/dagu/issues/240) from yohamta/develop

## What's Changed
* Fix parameter parsing issue by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#240](https://github.com/yohamta/dagu/pull/240)


**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.6.8...v1.6.9

[Changes][v1.6.9]


<a id="v1.6.8"></a>
# [v1.6.8](https://github.com/dagu-org/dagu/releases/tag/v1.6.8) - 2022-08-04

## Changelog
* 073b1d3 Merge pull request [#238](https://github.com/dagu-org/dagu/issues/238) from yohamta/develop

## What's Changed
* Allow users to specify a DAG name without `.yaml` when starting a DAG by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#236](https://github.com/yohamta/dagu/pull/236)
* Simplify the time format of `NEXT RUN` in the Web UI by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#237](https://github.com/yohamta/dagu/pull/237)
* Make scheduler logs to be rotated automatically by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#238](https://github.com/yohamta/dagu/pull/238)


**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.6.7...v1.6.8

[Changes][v1.6.8]


<a id="v1.6.7"></a>
# [v1.6.7](https://github.com/dagu-org/dagu/releases/tag/v1.6.7) - 2022-08-03

## Changelog
* 3c9d661 Merge pull request [#232](https://github.com/dagu-org/dagu/issues/232) from yohamta/fix/json-issue

## What's Changed
* Fix command parsing issue by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#232](https://github.com/yohamta/dagu/pull/232)


**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.6.6...v1.6.7

[Changes][v1.6.7]


<a id="v1.6.6"></a>
# [v1.6.6](https://github.com/dagu-org/dagu/releases/tag/v1.6.6) - 2022-08-02

## Changelog
* 2033cb4 Merge pull request [#230](https://github.com/dagu-org/dagu/issues/230) from yohamta/feat/webui-schedule

## What's Changed
* Add executor interface by [@Arvintian](https://github.com/Arvintian) in [yohamta/dagu#227](https://github.com/yohamta/dagu/pull/227)
* Fix timeline chart appearance issue in the Web UI by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#229](https://github.com/yohamta/dagu/pull/229)
* Eliminate the need to type `.yaml` in new DAG name in the Web UI by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#229](https://github.com/yohamta/dagu/pull/229)
* Add `Next Run` column to the DAG table in the Web UI by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#230](https://github.com/yohamta/dagu/pull/230)


**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.6.5...v1.6.6

## Contributors

<a href="https://github.com/Arvintian"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2FArvintian.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@Arvintian"></a>

[Changes][v1.6.6]


<a id="v1.6.5"></a>
# [v1.6.5](https://github.com/dagu-org/dagu/releases/tag/v1.6.5) - 2022-08-01

## Changelog
* 1b606a5 Merge pull request [#226](https://github.com/dagu-org/dagu/issues/226) from yohamta/fix/config-editor-issue

## What's Changed
* Fix config editor issue by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#226](https://github.com/yohamta/dagu/pull/226)


**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.6.4...v1.6.5

[Changes][v1.6.5]


<a id="v1.6.4"></a>
# [v1.6.4](https://github.com/dagu-org/dagu/releases/tag/v1.6.4) - 2022-07-30

## Changelog
* 7a5eeca Merge pull request [#223](https://github.com/dagu-org/dagu/issues/223) from yohamta/feat/parsing-parameters

## What's Changed
* Fix command params issue by [@Arvintian](https://github.com/Arvintian) in [yohamta/dagu#218](https://github.com/yohamta/dagu/pull/218)
* Allow set web page title by [@Arvintian](https://github.com/Arvintian) in [yohamta/dagu#218](https://github.com/yohamta/dagu/pull/218)
* Add sorting icon to the DAG table in the Web UI by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#219](https://github.com/yohamta/dagu/pull/219)
* Allow parameters field contains spaces inside each parameter by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#223](https://github.com/yohamta/dagu/pull/223)


**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.6.3...v1.6.4

## Contributors

<a href="https://github.com/Arvintian"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2FArvintian.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@Arvintian"></a>

[Changes][v1.6.4]


<a id="v1.6.3"></a>
# [v1.6.3](https://github.com/dagu-org/dagu/releases/tag/v1.6.3) - 2022-07-26

## What's Changed
* feat: Sort time chart graph in `startedAt` order on Dashboard page by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#212](https://github.com/yohamta/dagu/pull/212)
* feat: Make time chart graph scrollable on Dashboard page by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#212](https://github.com/yohamta/dagu/pull/212)


**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.6.2...v1.6.3

[Changes][v1.6.3]


<a id="v1.6.2"></a>
# [v1.6.2](https://github.com/dagu-org/dagu/releases/tag/v1.6.2) - 2022-07-26

## What's Changed
* fix: Create DAGs directory if not exist by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#211](https://github.com/yohamta/dagu/pull/211)


**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.6.1...v1.6.2

[Changes][v1.6.2]


<a id="v1.6.1"></a>
# [v1.6.1](https://github.com/dagu-org/dagu/releases/tag/v1.6.1) - 2022-07-25

## What's Changed
* fix: Fixed an issue that an output variable from steps can not be used in a script in subsequent steps by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#210](https://github.com/yohamta/dagu/pull/210)


**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.6.0...v1.6.1

[Changes][v1.6.1]


<a id="v1.6.0"></a>
# [v1.6.0](https://github.com/dagu-org/dagu/releases/tag/v1.6.0) - 2022-07-25

## What's Changed
* bugfix: Fixed the deletion logic of expired log files by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#203](https://github.com/yohamta/dagu/pull/203)
* refactor: Updated serve static file and fix makefile by [@Arvintian](https://github.com/Arvintian) in [yohamta/dagu#204](https://github.com/yohamta/dagu/pull/204)
* feat: Changed default DAGs directory to `~/.dagu/dags` for `scheduler` and `server` command by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#207](https://github.com/yohamta/dagu/pull/207)
* feat: Updated log file names for agent execution to avoid conflicts with other log files of the same DAG by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#208](https://github.com/yohamta/dagu/pull/208)
* feat: Fixed web UI to sort the DAG table on the Web UI in case-insensitive `Name` order by default by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#209](https://github.com/yohamta/dagu/pull/209)

## New Contributors
* [@Arvintian](https://github.com/Arvintian) made their first contribution in [yohamta/dagu#204](https://github.com/yohamta/dagu/pull/204)

**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.5.7...v1.6.0

## Contributors

<a href="https://github.com/Arvintian"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2FArvintian.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@Arvintian"></a>

[Changes][v1.6.0]


<a id="v1.5.7"></a>
# [v1.5.7](https://github.com/dagu-org/dagu/releases/tag/v1.5.7) - 2022-07-22

## Changelog
* 9fb1d57 fix: file cleaning bug

**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.5.6...v1.5.7

[Changes][v1.5.7]


<a id="v1.5.6"></a>
# [v1.5.6](https://github.com/dagu-org/dagu/releases/tag/v1.5.6) - 2022-07-22

## Changelog
* 5123c38 Merge pull request [#202](https://github.com/dagu-org/dagu/issues/202) from yohamta/fix/remove-expired-history-data

## What's Changed
* Automatic deletion of history files after `histRetentionDays` period has expired by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#202](https://github.com/yohamta/dagu/pull/202)


**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.5.5...v1.5.6

[Changes][v1.5.6]


<a id="v1.5.5"></a>
# [v1.5.5](https://github.com/dagu-org/dagu/releases/tag/v1.5.5) - 2022-07-20

## Changelog
* ffd713c Merge pull request [#201](https://github.com/dagu-org/dagu/issues/201) from yohamta/feat/overwrite-mailon-config

## What's Changed
* feat: allow overwriting global `MailOn` setting with individual DAG by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#201](https://github.com/yohamta/dagu/pull/201)


**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.5.4...v1.5.5

[Changes][v1.5.5]


<a id="v1.5.4"></a>
# [v1.5.4](https://github.com/dagu-org/dagu/releases/tag/v1.5.4) - 2022-07-13

## Changelog
* 27d0b1a Merge pull request [#198](https://github.com/dagu-org/dagu/issues/198) from yohamta/feat/fix-filename-shorter

## What's Changed
* feat: update log file view by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#198](https://github.com/yohamta/dagu/pull/198)


**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.5.3...v1.5.4

[Changes][v1.5.4]


<a id="v1.5.3"></a>
# [v1.5.3](https://github.com/dagu-org/dagu/releases/tag/v1.5.3) - 2022-07-12

## Changelog
* 3f9a3ef Merge pull request [#197](https://github.com/dagu-org/dagu/issues/197) from yohamta/feat/suspend-dag

## What's Changed
* feat: suspend DAG schedule by switches on web UI by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#197](https://github.com/yohamta/dagu/pull/197)


**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.5.2...v1.5.3

[Changes][v1.5.3]


<a id="v1.5.2"></a>
# [v1.5.2](https://github.com/dagu-org/dagu/releases/tag/v1.5.2) - 2022-07-11

## Changelog
* 2e48c01 Merge pull request [#196](https://github.com/dagu-org/dagu/issues/196) from yohamta/feat/improve-web-ui

## What's Changed
* add golangci-lint by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#195](https://github.com/yohamta/dagu/pull/195)
* feat: update web UI by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#196](https://github.com/yohamta/dagu/pull/196)


**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.5.1...v1.5.2

[Changes][v1.5.2]


<a id="v1.5.1"></a>
# [v1.5.1](https://github.com/dagu-org/dagu/releases/tag/v1.5.1) - 2022-07-11

## Changelog
* c93d40a Merge pull request [#194](https://github.com/dagu-org/dagu/issues/194) from yohamta/feat/header

## What's Changed
* feat: improve navigation header by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#194](https://github.com/yohamta/dagu/pull/194)


**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.5.0...v1.5.1

[Changes][v1.5.1]


<a id="v1.5.0"></a>
# [v1.5.0](https://github.com/dagu-org/dagu/releases/tag/v1.5.0) - 2022-07-10

## Changelog
* 91fc2b7 Merge pull request [#193](https://github.com/dagu-org/dagu/issues/193) from yohamta/feat/webui-improvement

## What's Changed
* refactor: removed View feature by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#192](https://github.com/yohamta/dagu/pull/192)
* feat: simplify web UI by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#193](https://github.com/yohamta/dagu/pull/193)


**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.4.4...v1.5.0

[Changes][v1.5.0]


<a id="v1.4.4"></a>
# [v1.4.4](https://github.com/dagu-org/dagu/releases/tag/v1.4.4) - 2022-07-06

## Changelog
- Fixed scheduler process issue

## What's Changed
* Refactor scheduler by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#190](https://github.com/yohamta/dagu/pull/190)
* feat: sort by next time to run by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#191](https://github.com/yohamta/dagu/pull/191)


**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.4.3...v1.4.4

[Changes][v1.4.4]


<a id="v1.4.3"></a>
# [v1.4.3](https://github.com/dagu-org/dagu/releases/tag/v1.4.3) - 2022-07-05

## Changelog
* ec60ab2 Merge pull request [#189](https://github.com/dagu-org/dagu/issues/189) from yohamta/feat/multiple-schedules

## What's Changed
* feat: fix frontend for multiple schedule by [@yohamta](https://github.com/yohamta) in [yohamta/dagu#189](https://github.com/yohamta/dagu/pull/189)


**Full Changelog**: https://github.com/yohamta/dagu/compare/v1.4.2...v1.4.3

[Changes][v1.4.3]


<a id="v1.4.2"></a>
# [v1.4.2](https://github.com/dagu-org/dagu/releases/tag/v1.4.2) - 2022-07-05

## Changelog
* 5593bbd Merge pull request [#188](https://github.com/dagu-org/dagu/issues/188) from yohamta/feat/multiple-schedules



[Changes][v1.4.2]


<a id="v1.4.1"></a>
# [v1.4.1](https://github.com/dagu-org/dagu/releases/tag/v1.4.1) - 2022-07-04

## Changelog
* eaed682 Merge pull request [#187](https://github.com/dagu-org/dagu/issues/187) from yohamta/feat/scheduler-log-rotation



[Changes][v1.4.1]


<a id="v1.4.0"></a>
# [v1.4.0](https://github.com/dagu-org/dagu/releases/tag/v1.4.0) - 2022-07-01

## Changelog
* ff0a1a6 Merge pull request [#179](https://github.com/dagu-org/dagu/issues/179) from yohamta/feat/scheduler



[Changes][v1.4.0]


<a id="v1.3.21"></a>
# [v1.3.21](https://github.com/dagu-org/dagu/releases/tag/v1.3.21) - 2022-06-24

## Changelog
* 4723f1f Merge pull request [#184](https://github.com/dagu-org/dagu/issues/184) from yohamta/feat/web-ui-collapsible-table-row



[Changes][v1.3.21]


<a id="v1.3.20"></a>
# [v1.3.20](https://github.com/dagu-org/dagu/releases/tag/v1.3.20) - 2022-06-23

## Changelog
* 12c1474 Merge pull request [#183](https://github.com/dagu-org/dagu/issues/183) from yohamta/fix/web-ui-issue



[Changes][v1.3.20]


<a id="v1.3.19"></a>
# [v1.3.19](https://github.com/dagu-org/dagu/releases/tag/v1.3.19) - 2022-06-23

## Changelog
* d818829 Merge pull request [#182](https://github.com/dagu-org/dagu/issues/182) from yohamta/feat/dag-groups



[Changes][v1.3.19]


<a id="v1.3.18"></a>
# [v1.3.18](https://github.com/dagu-org/dagu/releases/tag/v1.3.18) - 2022-06-22

## Changelog
* 7dc9f49 Merge pull request [#181](https://github.com/dagu-org/dagu/issues/181) from yohamta/feat/retry-interval



[Changes][v1.3.18]


<a id="v1.3.17"></a>
# [v1.3.17](https://github.com/dagu-org/dagu/releases/tag/v1.3.17) - 2022-06-19

## Changelog
* d065233 Merge pull request [#178](https://github.com/dagu-org/dagu/issues/178) from yohamta/fix/log-page-scrollbar-issue



[Changes][v1.3.17]


<a id="v1.3.16"></a>
# [v1.3.16](https://github.com/dagu-org/dagu/releases/tag/v1.3.16) - 2022-06-17

## Changelog
* 96343cb Merge pull request [#173](https://github.com/dagu-org/dagu/issues/173) from yohamta/fix/timechart-bug



[Changes][v1.3.16]


<a id="v1.3.15"></a>
# [v1.3.15](https://github.com/dagu-org/dagu/releases/tag/v1.3.15) - 2022-06-13

## Changelog
* e884bdd Merge pull request [#169](https://github.com/dagu-org/dagu/issues/169) from yohamta/fix/admin-config-issue



[Changes][v1.3.15]


<a id="v1.3.14"></a>
# [v1.3.14](https://github.com/dagu-org/dagu/releases/tag/v1.3.14) - 2022-06-13

## Changelog
* ae273f8 Merge pull request [#167](https://github.com/dagu-org/dagu/issues/167) from yohamta/fix/pass-params-as-vars



[Changes][v1.3.14]


<a id="v1.3.13"></a>
# [v1.3.13](https://github.com/dagu-org/dagu/releases/tag/v1.3.13) - 2022-06-13

## Changelog
* 65d796e Merge pull request [#165](https://github.com/dagu-org/dagu/issues/165) from yohamta/fix/parameter-parsing-issue



[Changes][v1.3.13]


<a id="v1.3.12"></a>
# [v1.3.12](https://github.com/dagu-org/dagu/releases/tag/v1.3.12) - 2022-06-08

## Changelog
* 9b42d4e Merge pull request [#162](https://github.com/dagu-org/dagu/issues/162) from yohamta/feat/view



[Changes][v1.3.12]


<a id="v1.3.11"></a>
# [v1.3.11](https://github.com/dagu-org/dagu/releases/tag/v1.3.11) - 2022-06-06

## Changelog
* 66d61f2 Merge pull request [#158](https://github.com/dagu-org/dagu/issues/158) from yohamta/fix/web-ui-bug



[Changes][v1.3.11]


<a id="v1.3.10"></a>
# [v1.3.10](https://github.com/dagu-org/dagu/releases/tag/v1.3.10) - 2022-06-06

## Changelog
* 1e3dd46 Merge pull request [#156](https://github.com/dagu-org/dagu/issues/156) from yohamta/refactor/agent



[Changes][v1.3.10]


<a id="v1.3.9"></a>
# [v1.3.9](https://github.com/dagu-org/dagu/releases/tag/v1.3.9) - 2022-06-02

## Changelog
* 48b8da8 Merge pull request [#152](https://github.com/dagu-org/dagu/issues/152) from yohamta/feat/tag-filter



[Changes][v1.3.9]


<a id="v1.3.8"></a>
# [v1.3.8](https://github.com/dagu-org/dagu/releases/tag/v1.3.8) - 2022-06-01

## Changelog
* 38d2e6a Merge pull request [#150](https://github.com/dagu-org/dagu/issues/150) from yohamta/docs/update-screenshots



[Changes][v1.3.8]


<a id="v1.3.7"></a>
# [v1.3.7](https://github.com/dagu-org/dagu/releases/tag/v1.3.7) - 2022-06-01

## Changelog
* 3c1a2d9 Merge pull request [#147](https://github.com/dagu-org/dagu/issues/147) from yohamta/feat/sort-filter-workflows



[Changes][v1.3.7]


<a id="v1.3.6"></a>
# [v1.3.6](https://github.com/dagu-org/dagu/releases/tag/v1.3.6) - 2022-06-01

## Changelog
* fc59cab fix: release build workflow issue



[Changes][v1.3.6]


<a id="v1.3.5"></a>
# [v1.3.5](https://github.com/dagu-org/dagu/releases/tag/v1.3.5) - 2022-06-01

## Changelog
* ca73bbc Merge pull request [#144](https://github.com/dagu-org/dagu/issues/144) from yohamta/feat/named-parameters



[Changes][v1.3.5]


<a id="v1.3.4"></a>
# [v1.3.4](https://github.com/dagu-org/dagu/releases/tag/v1.3.4) - 2022-05-31

## Changelog
* e452f9e Merge pull request [#141](https://github.com/dagu-org/dagu/issues/141) from yohamta/fix/webui-workflow-log



[Changes][v1.3.4]


<a id="v1.3.3"></a>
# [v1.3.3](https://github.com/dagu-org/dagu/releases/tag/v1.3.3) - 2022-05-31

## Changelog
* 0b11457 Update .goreleaser.yaml



[Changes][v1.3.3]


<a id="v1.3.2"></a>
# [v1.3.2](https://github.com/dagu-org/dagu/releases/tag/v1.3.2) - 2022-05-31

## Changelog
* 7f10def Merge pull request [#137](https://github.com/dagu-org/dagu/issues/137) from yohamta/feat/timeline-chart-ordering



[Changes][v1.3.2]


<a id="v1.3.1"></a>
# [v1.3.1](https://github.com/dagu-org/dagu/releases/tag/v1.3.1) - 2022-05-31

## Changelog
* 28afd70 Merge pull request [#136](https://github.com/dagu-org/dagu/issues/136) from yohamta/fix/timeline-issue



[Changes][v1.3.1]


<a id="v1.3.0"></a>
# [v1.3.0](https://github.com/dagu-org/dagu/releases/tag/v1.3.0) - 2022-05-31

## Changelog
* 976c582 Merge pull request [#134](https://github.com/dagu-org/dagu/issues/134) from yohamta/fix/readme



[Changes][v1.3.0]


<a id="v1.2.16"></a>
# [v1.2.16](https://github.com/dagu-org/dagu/releases/tag/v1.2.16) - 2022-05-31

## Changelog
* 0d1e8f1 Merge pull request [#132](https://github.com/dagu-org/dagu/issues/132) from yohamta/fix/github-workflows



[Changes][v1.2.16]


<a id="v1.2.15"></a>
# [v1.2.15](https://github.com/dagu-org/dagu/releases/tag/v1.2.15) - 2022-05-27

## Changelog
* ec7d5c6 Merge pull request [#126](https://github.com/dagu-org/dagu/issues/126) from yohamta/fix/status-correction



[Changes][v1.2.15]


<a id="v1.2.14"></a>
# [v1.2.14](https://github.com/dagu-org/dagu/releases/tag/v1.2.14) - 2022-05-27

## Changelog
* fff64c3 Merge pull request [#125](https://github.com/dagu-org/dagu/issues/125) from yohamta/fix/admin-ui-issue



[Changes][v1.2.14]


<a id="v1.2.13"></a>
# [v1.2.13](https://github.com/dagu-org/dagu/releases/tag/v1.2.13) - 2022-05-27

## Changelog
* 7312dce feat: update status onclick node on graph



[Changes][v1.2.13]


<a id="v1.2.12"></a>
# [v1.2.12](https://github.com/dagu-org/dagu/releases/tag/v1.2.12) - 2022-05-20

## Changelog
* 444797c Merge pull request [#115](https://github.com/dagu-org/dagu/issues/115) from yohamta/fix/status



[Changes][v1.2.12]


<a id="v1.2.11"></a>
# [v1.2.11](https://github.com/dagu-org/dagu/releases/tag/v1.2.11) - 2022-05-20

## Changelog
* ca4eefd Merge pull request [#112](https://github.com/dagu-org/dagu/issues/112) from yohamta/fix/add-default-env



[Changes][v1.2.11]


<a id="v1.2.10"></a>
# [v1.2.10](https://github.com/dagu-org/dagu/releases/tag/v1.2.10) - 2022-05-19

## Changelog
* 24ae41b Merge pull request [#108](https://github.com/dagu-org/dagu/issues/108) from yohamta/feat/run-code-snippet



[Changes][v1.2.10]


<a id="v1.2.9"></a>
# [v1.2.9](https://github.com/dagu-org/dagu/releases/tag/v1.2.9) - 2022-05-18

## Changelog
* f75b29a Merge pull request [#107](https://github.com/dagu-org/dagu/issues/107) from dagu-go/feat/output



[Changes][v1.2.9]


<a id="v1.2.8"></a>
# [v1.2.8](https://github.com/dagu-org/dagu/releases/tag/v1.2.8) - 2022-05-16

## Changelog
* c66fd79 Fix [#90](https://github.com/dagu-org/dagu/issues/90)



[Changes][v1.2.8]


<a id="v1.2.7"></a>
# [v1.2.7](https://github.com/dagu-org/dagu/releases/tag/v1.2.7) - 2022-05-15

## Changelog
* 06bc8b7 Fixed test



[Changes][v1.2.7]


<a id="v1.2.6"></a>
# [v1.2.6](https://github.com/dagu-org/dagu/releases/tag/v1.2.6) - 2022-05-13

## Changelog
* 22c434a Merge pull request [#77](https://github.com/dagu-org/dagu/issues/77) from yohamta/feature/syntax-highlignt



[Changes][v1.2.6]


<a id="v1.2.5"></a>
# [v1.2.5](https://github.com/dagu-org/dagu/releases/tag/v1.2.5) - 2022-05-13

## Changelog
* fef71ce Fixed dag preview to be more visible



[Changes][v1.2.5]


<a id="v1.2.4"></a>
# [v1.2.4](https://github.com/dagu-org/dagu/releases/tag/v1.2.4) - 2022-05-12

## Changelog
* c03b2ce Added a function to rename existing DAGs on web UI



[Changes][v1.2.4]


<a id="v1.2.3"></a>
# [v1.2.3](https://github.com/dagu-org/dagu/releases/tag/v1.2.3) - 2022-05-11

## Changelog
* faf9f48 Update README

[Changes][v1.2.3]


<a id="v1.2.2"></a>
# [v1.2.2](https://github.com/dagu-org/dagu/releases/tag/v1.2.2) - 2022-05-10

## Changelog
* 5bde94e Fix codecov.yml



[Changes][v1.2.2]


<a id="v1.2.1"></a>
# [v1.2.1](https://github.com/dagu-org/dagu/releases/tag/v1.2.1) - 2022-05-09

## Changelog
* a103bc3 Merge pull request [#61](https://github.com/dagu-org/dagu/issues/61) from yohamta/develop



[Changes][v1.2.1]


<a id="v1.2.0"></a>
# [v1.2.0](https://github.com/dagu-org/dagu/releases/tag/v1.2.0) - 2022-05-08

## Changelog
* 2079274 Merge pull request [#59](https://github.com/dagu-org/dagu/issues/59) from lrampa/feature/unique_file_names



[Changes][v1.2.0]


<a id="v1.1.9"></a>
# [v1.1.9](https://github.com/dagu-org/dagu/releases/tag/v1.1.9) - 2022-05-06

## Changelog
* e24e9c7 Add tests



[Changes][v1.1.9]


<a id="v1.1.8"></a>
# [v1.1.8](https://github.com/dagu-org/dagu/releases/tag/v1.1.8) - 2022-05-06

## Changelog
* fde7438 Merge pull request [#56](https://github.com/dagu-org/dagu/issues/56) from yohamta/develop



[Changes][v1.1.8]


<a id="v1.1.7"></a>
# [v1.1.7](https://github.com/dagu-org/dagu/releases/tag/v1.1.7) - 2022-05-02

## Changelog
* ab85b5f Add tests



[Changes][v1.1.7]


<a id="v1.1.6"></a>
# [v1.1.6](https://github.com/dagu-org/dagu/releases/tag/v1.1.6) - 2022-04-29

## Changelog
* ad55275 Merge pull request [#40](https://github.com/dagu-org/dagu/issues/40) from yohamta/develop



[Changes][v1.1.6]


<a id="v1.1.5"></a>
# [v1.1.5](https://github.com/dagu-org/dagu/releases/tag/v1.1.5) - 2022-04-28

## Changelog
* c604df2 Merge pull request [#25](https://github.com/dagu-org/dagu/issues/25) from yohamta/feat/cleanup-time-signal



[Changes][v1.1.5]


<a id="v1.1.4"></a>
# [v1.1.4](https://github.com/dagu-org/dagu/releases/tag/v1.1.4) - 2022-04-28

## Changelog
* 6e19286 Merge pull request [#24](https://github.com/dagu-org/dagu/issues/24) from yohamta/feature/retry-policy



[Changes][v1.1.4]


<a id="v1.1.3"></a>
# [v1.1.3](https://github.com/dagu-org/dagu/releases/tag/v1.1.3) - 2022-04-28

## Changelog
* 5c8d546 Merge pull request [#23](https://github.com/dagu-org/dagu/issues/23) from yohamta/fix-graph



[Changes][v1.1.3]


<a id="v1.1.2"></a>
# [v1.1.2](https://github.com/dagu-org/dagu/releases/tag/v1.1.2) - 2022-04-27

## Changelog
* 44d7e86 Merge pull request [#21](https://github.com/dagu-org/dagu/issues/21) from yohamta/feature/update-status



[Changes][v1.1.2]


<a id="v1.1.1"></a>
# [v1.1.1](https://github.com/dagu-org/dagu/releases/tag/v1.1.1) - 2022-04-27

## Changelog
* 17a5a2b Fix to use word "DAG" instead of "job"



[Changes][v1.1.1]


<a id="v1.1.0"></a>
# [v1.1.0](https://github.com/dagu-org/dagu/releases/tag/v1.1.0) - 2022-04-27

## Changelog
* c5f01ee Merge pull request [#16](https://github.com/dagu-org/dagu/issues/16) from yohamta/bugfix



[Changes][v1.1.0]


<a id="v1.0.2"></a>
# [v1.0.2](https://github.com/dagu-org/dagu/releases/tag/v1.0.2) - 2022-04-26

## Changelog
* 6bdb5a9 Merge pull request [#13](https://github.com/dagu-org/dagu/issues/13) from yohamta/issue-12



[Changes][v1.0.2]


<a id="v1.0.1"></a>
# [v1.0.1](https://github.com/dagu-org/dagu/releases/tag/v1.0.1) - 2022-04-24

## Changelog
* 2e53729 Fix master branch to main branch



[Changes][v1.0.1]


<a id="v1.0.0"></a>
# [v1.0.0](https://github.com/dagu-org/dagu/releases/tag/v1.0.0) - 2022-04-23

## Changelog
* 189697f Setup goreleaser



[Changes][v1.0.0]


[v1.17.0-beta.7]: https://github.com/dagu-org/dagu/compare/v1.17.0-beta.6...v1.17.0-beta.7
[v1.17.0-beta.6]: https://github.com/dagu-org/dagu/compare/v1.17.0-beta.5...v1.17.0-beta.6
[v1.17.0-beta.5]: https://github.com/dagu-org/dagu/compare/v1.17.0-beta.4...v1.17.0-beta.5
[v1.17.0-beta.4]: https://github.com/dagu-org/dagu/compare/v1.17.0-beta.3...v1.17.0-beta.4
[v1.17.0-beta.3]: https://github.com/dagu-org/dagu/compare/v1.17.0-beta.2...v1.17.0-beta.3
[v1.17.0-beta.2]: https://github.com/dagu-org/dagu/compare/v1.17.0-beta.1...v1.17.0-beta.2
[v1.17.0-beta.1]: https://github.com/dagu-org/dagu/compare/v1.16.12...v1.17.0-beta.1
[v1.16.12]: https://github.com/dagu-org/dagu/compare/v1.16.11...v1.16.12
[v1.16.11]: https://github.com/dagu-org/dagu/compare/v1.16.10...v1.16.11
[v1.16.10]: https://github.com/dagu-org/dagu/compare/v1.16.9...v1.16.10
[v1.16.9]: https://github.com/dagu-org/dagu/compare/v1.16.8...v1.16.9
[v1.16.8]: https://github.com/dagu-org/dagu/compare/v1.16.7...v1.16.8
[v1.16.7]: https://github.com/dagu-org/dagu/compare/v1.16.6...v1.16.7
[v1.16.6]: https://github.com/dagu-org/dagu/compare/v1.16.5...v1.16.6
[v1.16.5]: https://github.com/dagu-org/dagu/compare/v1.16.4...v1.16.5
[v1.16.4]: https://github.com/dagu-org/dagu/compare/v1.16.3...v1.16.4
[v1.16.3]: https://github.com/dagu-org/dagu/compare/v1.16.2...v1.16.3
[v1.16.2]: https://github.com/dagu-org/dagu/compare/v1.16.1...v1.16.2
[v1.16.1]: https://github.com/dagu-org/dagu/compare/v1.16.0...v1.16.1
[v1.16.0]: https://github.com/dagu-org/dagu/compare/v1.15.1...v1.16.0
[v1.15.1]: https://github.com/dagu-org/dagu/compare/v1.15.0...v1.15.1
[v1.15.0]: https://github.com/dagu-org/dagu/compare/v1.14.8...v1.15.0
[v1.14.8]: https://github.com/dagu-org/dagu/compare/v1.14.7...v1.14.8
[v1.14.7]: https://github.com/dagu-org/dagu/compare/v1.14.6...v1.14.7
[v1.14.6]: https://github.com/dagu-org/dagu/compare/v1.14.5...v1.14.6
[v1.14.5]: https://github.com/dagu-org/dagu/compare/v1.14.4...v1.14.5
[v1.14.4]: https://github.com/dagu-org/dagu/compare/v1.14.3...v1.14.4
[v1.14.3]: https://github.com/dagu-org/dagu/compare/v1.14.2...v1.14.3
[v1.14.2]: https://github.com/dagu-org/dagu/compare/v1.14.1...v1.14.2
[v1.14.1]: https://github.com/dagu-org/dagu/compare/v1.14.0...v1.14.1
[v1.14.0]: https://github.com/dagu-org/dagu/compare/v1.13.1...v1.14.0
[v1.13.1]: https://github.com/dagu-org/dagu/compare/v1.13.0...v1.13.1
[v1.13.0]: https://github.com/dagu-org/dagu/compare/v1.12.11...v1.13.0
[v1.12.11]: https://github.com/dagu-org/dagu/compare/v1.12.10...v1.12.11
[v1.12.10]: https://github.com/dagu-org/dagu/compare/v1.12.9...v1.12.10
[v1.12.9]: https://github.com/dagu-org/dagu/compare/v1.12.8...v1.12.9
[v1.12.8]: https://github.com/dagu-org/dagu/compare/v1.12.7...v1.12.8
[v1.12.7]: https://github.com/dagu-org/dagu/compare/v1.12.6...v1.12.7
[v1.12.6]: https://github.com/dagu-org/dagu/compare/v1.12.5...v1.12.6
[v1.12.5]: https://github.com/dagu-org/dagu/compare/v1.12.4...v1.12.5
[v1.12.4]: https://github.com/dagu-org/dagu/compare/v1.12.3...v1.12.4
[v1.12.3]: https://github.com/dagu-org/dagu/compare/v1.12.2...v1.12.3
[v1.12.2]: https://github.com/dagu-org/dagu/compare/v1.12.1...v1.12.2
[v1.12.1]: https://github.com/dagu-org/dagu/compare/v1.12.0...v1.12.1
[v1.12.0]: https://github.com/dagu-org/dagu/compare/v1.11.0...v1.12.0
[v1.11.0]: https://github.com/dagu-org/dagu/compare/v1.10.6...v1.11.0
[v1.10.6]: https://github.com/dagu-org/dagu/compare/v1.10.5...v1.10.6
[v1.10.5]: https://github.com/dagu-org/dagu/compare/v1.10.4...v1.10.5
[v1.10.4]: https://github.com/dagu-org/dagu/compare/v1.10.3...v1.10.4
[v1.10.3]: https://github.com/dagu-org/dagu/compare/v1.10.2...v1.10.3
[v1.10.2]: https://github.com/dagu-org/dagu/compare/v1.10.1...v1.10.2
[v1.10.1]: https://github.com/dagu-org/dagu/compare/v1.9.4...v1.10.1
[v1.9.4]: https://github.com/dagu-org/dagu/compare/v1.9.3...v1.9.4
[v1.9.3]: https://github.com/dagu-org/dagu/compare/v1.9.2...v1.9.3
[v1.9.2]: https://github.com/dagu-org/dagu/compare/v1.9.1...v1.9.2
[v1.9.1]: https://github.com/dagu-org/dagu/compare/v1.9.0...v1.9.1
[v1.9.0]: https://github.com/dagu-org/dagu/compare/v1.8.8...v1.9.0
[v1.8.8]: https://github.com/dagu-org/dagu/compare/v1.8.7...v1.8.8
[v1.8.7]: https://github.com/dagu-org/dagu/compare/v1.8.6...v1.8.7
[v1.8.6]: https://github.com/dagu-org/dagu/compare/v1.8.5...v1.8.6
[v1.8.5]: https://github.com/dagu-org/dagu/compare/v1.8.4...v1.8.5
[v1.8.4]: https://github.com/dagu-org/dagu/compare/v1.8.3...v1.8.4
[v1.8.3]: https://github.com/dagu-org/dagu/compare/v1.8.2...v1.8.3
[v1.8.2]: https://github.com/dagu-org/dagu/compare/v1.8.1...v1.8.2
[v1.8.1]: https://github.com/dagu-org/dagu/compare/v1.8.0...v1.8.1
[v1.8.0]: https://github.com/dagu-org/dagu/compare/v1.7.11...v1.8.0
[v1.7.11]: https://github.com/dagu-org/dagu/compare/v1.7.10...v1.7.11
[v1.7.10]: https://github.com/dagu-org/dagu/compare/v1.7.9...v1.7.10
[v1.7.9]: https://github.com/dagu-org/dagu/compare/v1.7.8...v1.7.9
[v1.7.8]: https://github.com/dagu-org/dagu/compare/v1.7.7...v1.7.8
[v1.7.7]: https://github.com/dagu-org/dagu/compare/v1.7.6...v1.7.7
[v1.7.6]: https://github.com/dagu-org/dagu/compare/v1.7.5...v1.7.6
[v1.7.5]: https://github.com/dagu-org/dagu/compare/v1.7.4...v1.7.5
[v1.7.4]: https://github.com/dagu-org/dagu/compare/v1.7.3...v1.7.4
[v1.7.3]: https://github.com/dagu-org/dagu/compare/v1.6.9...v1.7.3
[v1.6.9]: https://github.com/dagu-org/dagu/compare/v1.6.8...v1.6.9
[v1.6.8]: https://github.com/dagu-org/dagu/compare/v1.6.7...v1.6.8
[v1.6.7]: https://github.com/dagu-org/dagu/compare/v1.6.6...v1.6.7
[v1.6.6]: https://github.com/dagu-org/dagu/compare/v1.6.5...v1.6.6
[v1.6.5]: https://github.com/dagu-org/dagu/compare/v1.6.4...v1.6.5
[v1.6.4]: https://github.com/dagu-org/dagu/compare/v1.6.3...v1.6.4
[v1.6.3]: https://github.com/dagu-org/dagu/compare/v1.6.2...v1.6.3
[v1.6.2]: https://github.com/dagu-org/dagu/compare/v1.6.1...v1.6.2
[v1.6.1]: https://github.com/dagu-org/dagu/compare/v1.6.0...v1.6.1
[v1.6.0]: https://github.com/dagu-org/dagu/compare/v1.5.7...v1.6.0
[v1.5.7]: https://github.com/dagu-org/dagu/compare/v1.5.6...v1.5.7
[v1.5.6]: https://github.com/dagu-org/dagu/compare/v1.5.5...v1.5.6
[v1.5.5]: https://github.com/dagu-org/dagu/compare/v1.5.4...v1.5.5
[v1.5.4]: https://github.com/dagu-org/dagu/compare/v1.5.3...v1.5.4
[v1.5.3]: https://github.com/dagu-org/dagu/compare/v1.5.2...v1.5.3
[v1.5.2]: https://github.com/dagu-org/dagu/compare/v1.5.1...v1.5.2
[v1.5.1]: https://github.com/dagu-org/dagu/compare/v1.5.0...v1.5.1
[v1.5.0]: https://github.com/dagu-org/dagu/compare/v1.4.4...v1.5.0
[v1.4.4]: https://github.com/dagu-org/dagu/compare/v1.4.3...v1.4.4
[v1.4.3]: https://github.com/dagu-org/dagu/compare/v1.4.2...v1.4.3
[v1.4.2]: https://github.com/dagu-org/dagu/compare/v1.4.1...v1.4.2
[v1.4.1]: https://github.com/dagu-org/dagu/compare/v1.4.0...v1.4.1
[v1.4.0]: https://github.com/dagu-org/dagu/compare/v1.3.21...v1.4.0
[v1.3.21]: https://github.com/dagu-org/dagu/compare/v1.3.20...v1.3.21
[v1.3.20]: https://github.com/dagu-org/dagu/compare/v1.3.19...v1.3.20
[v1.3.19]: https://github.com/dagu-org/dagu/compare/v1.3.18...v1.3.19
[v1.3.18]: https://github.com/dagu-org/dagu/compare/v1.3.17...v1.3.18
[v1.3.17]: https://github.com/dagu-org/dagu/compare/v1.3.16...v1.3.17
[v1.3.16]: https://github.com/dagu-org/dagu/compare/v1.3.15...v1.3.16
[v1.3.15]: https://github.com/dagu-org/dagu/compare/v1.3.14...v1.3.15
[v1.3.14]: https://github.com/dagu-org/dagu/compare/v1.3.13...v1.3.14
[v1.3.13]: https://github.com/dagu-org/dagu/compare/v1.3.12...v1.3.13
[v1.3.12]: https://github.com/dagu-org/dagu/compare/v1.3.11...v1.3.12
[v1.3.11]: https://github.com/dagu-org/dagu/compare/v1.3.10...v1.3.11
[v1.3.10]: https://github.com/dagu-org/dagu/compare/v1.3.9...v1.3.10
[v1.3.9]: https://github.com/dagu-org/dagu/compare/v1.3.8...v1.3.9
[v1.3.8]: https://github.com/dagu-org/dagu/compare/v1.3.7...v1.3.8
[v1.3.7]: https://github.com/dagu-org/dagu/compare/v1.3.6...v1.3.7
[v1.3.6]: https://github.com/dagu-org/dagu/compare/v1.3.5...v1.3.6
[v1.3.5]: https://github.com/dagu-org/dagu/compare/v1.3.4...v1.3.5
[v1.3.4]: https://github.com/dagu-org/dagu/compare/v1.3.3...v1.3.4
[v1.3.3]: https://github.com/dagu-org/dagu/compare/v1.3.2...v1.3.3
[v1.3.2]: https://github.com/dagu-org/dagu/compare/v1.3.1...v1.3.2
[v1.3.1]: https://github.com/dagu-org/dagu/compare/v1.3.0...v1.3.1
[v1.3.0]: https://github.com/dagu-org/dagu/compare/v1.2.16...v1.3.0
[v1.2.16]: https://github.com/dagu-org/dagu/compare/v1.2.15...v1.2.16
[v1.2.15]: https://github.com/dagu-org/dagu/compare/v1.2.14...v1.2.15
[v1.2.14]: https://github.com/dagu-org/dagu/compare/v1.2.13...v1.2.14
[v1.2.13]: https://github.com/dagu-org/dagu/compare/v1.2.12...v1.2.13
[v1.2.12]: https://github.com/dagu-org/dagu/compare/v1.2.11...v1.2.12
[v1.2.11]: https://github.com/dagu-org/dagu/compare/v1.2.10...v1.2.11
[v1.2.10]: https://github.com/dagu-org/dagu/compare/v1.2.9...v1.2.10
[v1.2.9]: https://github.com/dagu-org/dagu/compare/v1.2.8...v1.2.9
[v1.2.8]: https://github.com/dagu-org/dagu/compare/v1.2.7...v1.2.8
[v1.2.7]: https://github.com/dagu-org/dagu/compare/v1.2.6...v1.2.7
[v1.2.6]: https://github.com/dagu-org/dagu/compare/v1.2.5...v1.2.6
[v1.2.5]: https://github.com/dagu-org/dagu/compare/v1.2.4...v1.2.5
[v1.2.4]: https://github.com/dagu-org/dagu/compare/v1.2.3...v1.2.4
[v1.2.3]: https://github.com/dagu-org/dagu/compare/v1.2.2...v1.2.3
[v1.2.2]: https://github.com/dagu-org/dagu/compare/v1.2.1...v1.2.2
[v1.2.1]: https://github.com/dagu-org/dagu/compare/v1.2.0...v1.2.1
[v1.2.0]: https://github.com/dagu-org/dagu/compare/v1.1.9...v1.2.0
[v1.1.9]: https://github.com/dagu-org/dagu/compare/v1.1.8...v1.1.9
[v1.1.8]: https://github.com/dagu-org/dagu/compare/v1.1.7...v1.1.8
[v1.1.7]: https://github.com/dagu-org/dagu/compare/v1.1.6...v1.1.7
[v1.1.6]: https://github.com/dagu-org/dagu/compare/v1.1.5...v1.1.6
[v1.1.5]: https://github.com/dagu-org/dagu/compare/v1.1.4...v1.1.5
[v1.1.4]: https://github.com/dagu-org/dagu/compare/v1.1.3...v1.1.4
[v1.1.3]: https://github.com/dagu-org/dagu/compare/v1.1.2...v1.1.3
[v1.1.2]: https://github.com/dagu-org/dagu/compare/v1.1.1...v1.1.2
[v1.1.1]: https://github.com/dagu-org/dagu/compare/v1.1.0...v1.1.1
[v1.1.0]: https://github.com/dagu-org/dagu/compare/v1.0.2...v1.1.0
[v1.0.2]: https://github.com/dagu-org/dagu/compare/v1.0.1...v1.0.2
[v1.0.1]: https://github.com/dagu-org/dagu/compare/v1.0.0...v1.0.1
[v1.0.0]: https://github.com/dagu-org/dagu/tree/v1.0.0

<!-- Generated by https://github.com/rhysd/changelog-from-release v3.9.0 -->
