# Changelog

## v1.24.0 (UNRELEASED)

### Added
- No changes yet.

## v1.23.3 (2025-10-26)

### Added
- CLI: Added `--name` flag to `start`, `retry`, and `enqueue` commands to override the DAG name specified in the YAML file (#1363)
- DAG: Support shell-like parameter expansion for `env` and `parameters` fields, including `${VAR:offset:length}` slices and shell-style defaults (`${VAR:-fallback}`) (#1354)
- Distributed: Added comprehensive Kubernetes deployment manifests with ConfigMaps, PVCs, and separate server/worker deployments (#1360)
- Distributed: Added automatic cleanup of stale coordinator service registrations (#1360)
- Distributed: Added retry mechanism with exponential backoff for coordinator client connections (#1360)

### Improved
- Distributed: Enhanced distributed execution with better worker coordination and status propagation (#1360)
- Distributed: Improved child DAG status propagation in distributed mode to correctly report final status to parent workflows (#1358)
- OIDC: Improved OIDC configuration validation with better error messages and logging (#1361)
- OIDC: Enhanced OIDC authentication flow with proper state management and error handling (#1361)
- API: Better error handling in API endpoints with more descriptive error messages (#1361)

### Fixed
- Distributed: Fixed child DAG status not being properly propagated to parent in distributed execution (#1358)
- Distributed: Fixed parallel execution status tracking for sub-DAGs (#1358)
- Distributed: Fixed service registry cleanup and stale entry detection (#1360)

### Contributors

Thanks to our contributors for this release:

| Contribution                           | Author                                   |
| -------------------------------------- | ---------------------------------------- |
| `--name` flag feature clarification and feedback (#1349) | [@ghansham](https://github.com/ghansham) |
| OIDC authentication broken interface issue report (#1359) | [@mitchplze](https://github.com/mitchplze) |
| Distributed execution lock issue on Kubernetes (#1353) | [@yangkghjh](https://github.com/yangkghjh) |
| Shell-like parameter expansion feature request (#1352) | [@ghansham](https://github.com/ghansham) |

**Full Changelog**: [v1.23.2...v1.23.3](https://github.com/dagu-org/dagu/compare/v1.23.2...v1.23.3)

## v1.23.2 (2025-10-22)

### Fixed
- Server: Fixed subprocess environment variable propagation - server now correctly passes environment variables to subprocesses (#1351)

### Contributors

Thanks to our contributors for this release:

| Contribution                           | Author                                   |
| -------------------------------------- | ---------------------------------------- |
| DAG manual launch bug report and detailed reproduction (#1345) | [@jeremydelattre59](https://github.com/jeremydelattre59) |
| WebUI blank page MIME type error report (#1350) | [@overflowy](https://github.com/overflowy) |

**Full Changelog**: [v1.23.1...v1.23.2](https://github.com/dagu-org/dagu/compare/v1.23.1...v1.23.2)

## v1.23.1 (2025-10-21)

### Documentation
- Updated GitHub Actions executor documentation

### Contributors

No external contributors for this release - documentation update only.

**Full Changelog**: [v1.23.0...v1.23.1](https://github.com/dagu-org/dagu/compare/v1.23.0...v1.23.1)

## v1.23.0 (2025-10-21)

### Changed
- Status: Adopted canonical lowercase tokens for DAG and node lifecycle states (`not_started`, `queued`, `running`, `succeeded`, `partially_succeeded`, `failed`, `canceled`), and updated API examples, docs, and telemetry labels to match.
- Security: Implemented security filtering for system environment variables passed to step processes and sub DAGs. System variables remain available for expansion (`${VAR}`) during DAG configuration parsing, but only whitelisted variables (`PATH`, `HOME`, `LANG`, `TZ`, `SHELL`) and variables with allowed prefixes (`DAGU_*`, `LC_*`, `DAG_*`) are passed to the step execution environment. This prevents accidental exposure of sensitive credentials to subprocess environments. Other variables must be explicitly defined in the workflow's `env` section to be available in step processes.
- Scheduler: Queue handler now processes items asynchronously, acknowledging work before heartbeat checks so long-running startups no longer starve the queue.
- Runtime: Subcommand execution inherits the filtered base environment and uses the caller's working directory.

### Added
- CLI: Added `--dagu-home` global flag to override the application home directory on a per-command basis. Useful for testing, running multiple instances with isolated data, and CI/CD scenarios.
- CLI: Added `dagu validate` command to validate DAG specifications without executing them. Prints human‑readable errors and exits with code 1 on failure.
- API: Added `POST /api/v2/dags/validate` to validate DAG YAML. Returns `{ valid: boolean, errors: string[], dag?: DAGDetails }`.
- API: `POST /api/v2/dags` now accepts optional `spec` to initialize a DAG. The spec is validated before creation and returns 400 on invalid input.
- API: Added `POST /api/v2/dag-runs` to create and start a DAG-run directly from an inline YAML `spec` without persisting a DAG file. Supports optional `name`, `params`, `dagRunId`, and `singleton`.
- API: Added `nextRun` sort option to `GET /api/v2/dags` to sort DAGs by their next scheduled run time. DAGs with earlier next runs appear first in ascending order, and DAGs without schedules appear last.
- Steps: Add support for shebang detection in `script`.
- Steps: Multi-line `command` strings now execute as inline scripts, including support for shebang.
- DAG: Introduced a `secrets` block that references external providers (built-in `env` and `file`) and resolves values at runtime with automatic log/output masking.
- Parameters: Added JSON Schema validation mode with `schema`.
- Runtime: Injects `DAG_RUN_STATUS` into handler environments so exit/success/failure/cancel scripts can branch on the final canonical status.
- Executors: Added an experimental GitHub Actions executor (`type: gha`) powered by nektos/act; action inputs come from the new step-level `params` map.
- UI: Added accordion-style expandable node rows to display step logs inline, similar to GitHub Actions, reducing the need to open popup windows (#1313).

### Fixed
- DAG name validation is centralized and enforced consistently: names must be `<= 40` chars and match `[A-Za-z0-9_.-]+`. Endpoints that accept `name` now return `400 bad_request` for invalid names.
- Docker: Fixed container initialization bug with `registryAuths` field (#1330)
- Windows: Fixed process cancellation not terminating subprocesses by recursively killing all child processes (#1342)
- UI: Fixed duration display update bug in DAG run details
- Other small issues and improvements

### Contributors

Thanks to our contributors for this release:

| Contribution                           | Contributor                              |
| -------------------------------------- | ---------------------------------------- |
| Docker-in-Docker container execution issues (#1228, #1231, #1235) and registryAuths bug report (#1327) | [@bellackn](https://github.com/bellackn) |
| Container name support (#1237), bash requirement (#1239), command field (#1261), log buttons (#1301), and scroll issues (#1324) | [@Pangolin2097](https://github.com/Pangolin2097) |
| Accordion-style log expansion feature request (#1313) | [@borestad](https://github.com/borestad) |
| SSH environment variables feature request (#1238) | [@n3storm](https://github.com/n3storm) |
| SSH config override issue report (#1249) | [@TrezOne](https://github.com/TrezOne) |
| DAG dependency resolution error report (#1262) | [@JuchangGit](https://github.com/JuchangGit) |
| Quickstart guide issue report (#1263) | [@Vad1mo](https://github.com/Vad1mo) |
| Parallel JSON execution issues (#1274) | [@tetedange13](https://github.com/tetedange13) |
| Grouped DAGs mobile UI bug report (#1294) | [@jarnik](https://github.com/jarnik) |
| Cleanup and status propagation feature request (#1305) | [@vnghia](https://github.com/vnghia) |
| Environment variables behavior bug report (#1320) | [@thibmart1](https://github.com/thibmart1) |
| System status UI issue report (#1224) | [@jeremydelattre59](https://github.com/jeremydelattre59) |
| Script execution error (#1203) and stop-all API feature request (#1211) | [@Kaiden0001](https://github.com/Kaiden0001) |
| Dotenv loading bug report (#1210) | [@don-philipe](https://github.com/don-philipe) |
| Script field issue report (#1334) | [@xinxinxinye](https://github.com/xinxinxinye) |
| Queue override implementation (#1240) and clear queue feature (#1298, #1299) | [@kriyanshii](https://github.com/kriyanshii) |
| JSON Schema validation for params implementation (#1273) | [@thefishhat](https://github.com/thefishhat) |
| SSH script validation implementation (#1308) | [@AdityaTel89](https://github.com/AdityaTel89) |
| README updates (#1326), unit tests (#1329), and legacy directory warning (#858, #1336) | [@arky](https://github.com/arky) |
| Windows process cancellation fix with recursive subprocess termination (#1207, #1342) | [@lvoeg](https://github.com/lvoeg) |
| Extensive troubleshooting and community support: container name (#1237), SSH environment variables (#1238), DAG dependency resolution (#1262), cleanup and status propagation (#1305), environment variables behavior (#1320), clear queue feature (#1298), Docker-in-Docker (#1235), and CLI/masking discussions (#1314, #1317, #1273) | [@ghansham](https://github.com/ghansham) |

## v1.22.0 (2025-08-24)

### New Features
- **Shorthand Step Syntax**: Added simplified step definition without requiring explicit name and command fields (#1206)
  ```yaml
  steps:
    - echo "hello"
  ```
  Equivalent to:
  ```yaml
  steps:
    - name: step 1
      command: echo "hello"
  ```
- **Working Directory Support**: Added DAG-level and step-level working directory configuration with inheritance for better file path management
- **Load Environment Support**: Enhanced environment variable loading capabilities with improved dotenv support
- **Queue Dashboard UI**: Complete queue management interface with visual feedback and improved user experience (#1217)
- **DAG Name Input Modal**: Improved UI for DAG name input and management with better validation (#1218)
- **Max Active Runs Enforcement**: DAG-level `maxActiveRuns` configuration enforcement in API and CLI when starting DAG-runs (#1214) - Thanks to [@ghansham](https://github.com/ghansham) for feedback
- **Queue Configuration Rename**: Renamed `queue.maxActiveRuns` to `queue.maxConcurrency` for clarity and consistency (#1215)

### Improvements
- **Directory Lock Management**: Improved directory lock and active process management for better reliability and reduced race conditions (#1202)
- **Empty Directory Cleanup**: Automatic removal of empty directories for proc and queue management to keep storage clean (#1208)
- **Default Start Command**: Made 'start' the default command, removing single flag requirement for better CLI usability
- **Max Active Runs Logic**: Enhanced check logic for `maxActiveRuns` configuration with improved validation (#1216)
- **Queue UI Enhancements**: Applied user feedback to improve queue UI usability and functionality (#1221, #1222)

### Bug Fixes
- **Script Execution**: Fixed script block execution that was failing with script parsing errors (#1204) - Thanks to [@Kaiden0001](https://github.com/Kaiden0001) for reporting the issue
- **Parallel Execution**: Fixed parallel execution parameter handling for JSON references and complex data structures (#1219) - Thanks to [@tetedange13](https://github.com/tetedange13) for reporting the issue

### Contributors

Thanks to our contributors for this release:

| Contribution                           | Contributor                              |
| -------------------------------------- | ---------------------------------------- |
| Script execution bug report (command not found error) | [@Kaiden0001](https://github.com/Kaiden0001) |
| Parallel execution with JSON references issue report | [@tetedange13](https://github.com/tetedange13) |
| Queue feature implementation feedback | [@ghansham](https://github.com/ghansham) |

**Full Changelog**: [v1.21.0...v1.22.0](https://github.com/dagu-org/dagu/compare/v1.21.0...v1.22.0)

## v1.21.0 (2025-08-17)

### New Features
- **Optional Step Names**: Made step names optional to remove the 40-character limit restriction, allowing more flexibility in workflow definitions (#1193) - Thanks to [@jonathonc](https://github.com/jonathonc) for raising the issue
- **Singleton DAG Execution**: Added `--singleton` flag to ensure only one instance of a DAG runs at a time, preventing duplicate executions (#1195) - Thanks to [@Kaiden0001](https://github.com/Kaiden0001) for the feature request
- **DAG-level SSH Configuration**: Implemented DAG-level SSH config for better control over remote executions across all steps (#1184)
- **Example DAGs for New Users**: Auto-create example DAGs when starting Dagu for the first time, helping new users get started quickly (#1190)
- **DAG-run Details Refresh**: Added refresh button to the DAG-run details page for immediate status updates (#1192)
- **Invalid DAG Handling**: Improved UI handling of invalid DAG configurations with better error messages and graceful degradation (#1186)

### Improvements
- **Queue Directory Management**: Ensure queue directory is created before starting file watch to prevent startup errors (#1191)
- **Coordinator Hostname Resolution**: Register configured hostname instead of resolved IP in coordinator.json for better network configuration (#1188) - Thanks to [@peterbuga](https://github.com/peterbuga) for reporting and proposing the solution

### Documentation
- **Examples Update**: Updated and cleaned up example DAGs to reflect current best practices (#1185)
- **Architecture Documentation**: Updated architecture diagrams and documentation

### Contributors

Thanks to our contributors for this release:

| Contribution                           | Contributor                              |
| -------------------------------------- | ---------------------------------------- |
| Optional step names feature request (40 char limit issue) | [@jonathonc](https://github.com/jonathonc) |
| Singleton flag feature request (split start API) | [@Kaiden0001](https://github.com/Kaiden0001) |
| Coordinator hostname resolution issue and solution | [@peterbuga](https://github.com/peterbuga) |

**Full Changelog**: [v1.20.0...v1.21.0](https://github.com/dagu-org/dagu/compare/v1.20.0...v1.21.0)

## v1.20.0 (2025-08-10)

### New Features
- **DAG Run Configuration**: Added ability to lock parameters and run ID in DAG configuration for controlled execution (#1176) - Thanks to [@kriyanshii](https://github.com/kriyanshii)
- **System Status UI**: Added comprehensive system health monitoring for scheduler and coordinator services (#1177)
- **Manual Refresh Controls**: Added refresh buttons to Dashboard, DAG-runs, and DAG-definitions pages for immediate data updates (#1178)
- **Immediate Execution Option**: Added checkbox to start DAGs immediately, bypassing the queue for urgent workflows (#1171)
- **Feedback Integration**: Added feedback button to sidebar with Web3Forms integration for better user communication (#1173)
- **Community Link**: Added Discord community link to navigation sidebar for easier access to support (#1175)
- **Auto-Navigation**: Automatically navigate to DAG-run detail page after starting or enqueuing a DAG (#1172)

### Improvements
- **Queue Management**: Refactored process store to use hierarchical directory structure, separating group names (queues) from DAG names for better organization (#1174)
- **Scheduler Timeout Handling**: Fixed scheduler queue reader to discard items when DAG runs don't become active within 10 seconds, preventing queue stacking (#1169) - Thanks to [@jrisch](https://github.com/jrisch) for reporting

### Bug Fixes
- **Status Display**: Fixed DAG header showing "0" instead of "not started" for DAGs that haven't been executed (#1168)

### Documentation
- **Roadmap**: Updated project roadmap with latest development priorities

### Contributors

Thanks to our contributors for this release:

| Contribution                           | Contributor                              |
| -------------------------------------- | ---------------------------------------- |
| DAG run configuration with parameter locking | [@kriyanshii](https://github.com/kriyanshii) |
| Timeout issues report and feedback | [@jrisch](https://github.com/jrisch) |

**Full Changelog**: [v1.19.0...v1.20.0](https://github.com/dagu-org/dagu/compare/v1.19.0...v1.20.0)

## v1.19.0 (2025-08-06)

### New Features
- **DAG-level Container Field**: Added support for running all steps in a single container with DAG-level container configuration (#1154)
- **Zombie DAG Detection**: Added automatic detection and cleanup of zombie DAG runs with configurable detection interval (#1163) - Thanks to [@jonasban](https://github.com/jonasban) for feedback
- **Container Registry Authentication**: Added support for pulling images from private registries with username/password and token-based authentication (#1165) - Thanks to [@vnghia](https://github.com/vnghia) for the feature request

### Improvements
- **Scheduler Health Check**: Fixed health check server startup to work correctly with multiple scheduler instances (#1157) - Thanks to [@jonasban](https://github.com/jonasban) for reporting
- **Stop Operation**: Fixed stop operation to properly handle multiple running instances of the same DAG (#1167) - Thanks to [@jeremydelattre59](https://github.com/jeremydelattre59) for reporting
- **Scheduler Queue Processing**: Fixed scheduler to use heartbeat monitoring instead of status files for more reliable process detection (#1166) - Thanks to [@jrisch](https://github.com/jrisch) for feedback
- **Environment Variables**: Corrected environment variable mapping for coordinator host and port configuration (#1162)
- **Docker Permissions**: Ensured DAGU_HOME directory has proper permissions in Docker containers (#1161)

### Bug Fixes
- **Continue On Skipped**: Fixed exit code 0 incorrectly triggering continuation for skipped steps with repeat policies (#1158) - Thanks to [@jeremydelattre59](https://github.com/jeremydelattre59) for reporting and [@thefishhat](https://github.com/thefishhat) for the fix
- **Queue Processing**: Fixed process store to correctly use queue name when specified (#1155) - Thanks to [@jonasban](https://github.com/jonasban) and [@ghansham](https://github.com/ghansham) for reporting

### Documentation
- **Docker Compose Example**: Added example configuration for Docker Compose setup
- **Roadmap**: Updated project roadmap based on general features

### Contributors

Thanks to our contributors for this release:

| Contribution                           | Contributor                              |
| -------------------------------------- | ---------------------------------------- |
| Zombie DAG detection feedback, health check and queue processing bug reports | [@jonasban](https://github.com/jonasban) |
| Continue on skipped bug report, stop operation issue report | [@jeremydelattre59](https://github.com/jeremydelattre59) |
| Exit code continuation fix | [@thefishhat](https://github.com/thefishhat) |
| Queue processing bug report | [@ghansham](https://github.com/ghansham) |
| Container registry authentication feature request | [@vnghia](https://github.com/vnghia) |
| Scheduler heartbeat monitoring feedback | [@jrisch](https://github.com/jrisch) |

**Full Changelog**: [v1.18.6...v1.19.0](https://github.com/dagu-org/dagu/compare/v1.18.6...v1.19.0)

## v1.18.6 (2025-08-03)

### Bug Fixes
- **Scheduler**: Fixed health check server startup race condition when multiple scheduler instances are deployed (#1157) - Thanks to [@jonasban](https://github.com/jonasban) for reporting

### Improvements
- **Node.js**: Upgraded to Node.js 22 (#1150) - Thanks to [@reneleonhardt](https://github.com/reneleonhardt)

### New Features
- **npm Package Verification**: Added automatic verification after npm package publishing (#1149)
- **npm Installation**: Added support for installing Dagu via npm

### Contributors

Thanks to our contributors for this release:

| Contribution                           | Contributor                              |
| -------------------------------------- | ---------------------------------------- |
| Health check race condition (report) | [@jonasban](https://github.com/jonasban) |
| Upgraded Node.js to 22 | [@reneleonhardt](https://github.com/reneleonhardt) |

**Full Changelog**: [v1.18.0...v1.18.6](https://github.com/dagu-org/dagu/compare/v1.18.5...v1.18.6)

## v1.18.0 (2025-07-29)

### New Features
- **Step-level Environment Variables**: Added support for environment variables at the step level (#1148) - Thanks to [@admerzeau](https://github.com/admerzeau) for reporting
- **Enhanced Repeat Policy**: Added explicit `until` and `while` modes for clearer repeat logic (#1050) - Thanks to [@thefishhat](https://github.com/thefishhat)
- **Live Log Loading**: Added real-time log streaming in the Web UI with reload button (#1085) - Thanks to [@tapir](https://github.com/tapir) for reporting
- **Exponential Backoff for RetryPolicy**: Added support for exponential backoff in retry policies (#1096, #1093) - Thanks to [@Sarvesh-11](https://github.com/Sarvesh-11)
- **OpenID Connect (OIDC) Authentication**: Added OIDC support for Web UI authentication (#1107) - Thanks to [@Arvintian](https://github.com/Arvintian)
- **Partial Success Status**: Added step-level partial success status for sub-DAG executions (#1115) - Thanks to [@ghansham](https://github.com/ghansham) for the feature request
- **Distributed Workflow Execution**: Implemented distributed task execution via worker processes (#1116, #1145)
- **Multiple Email Recipients**: Added support for multiple recipients in email notifications (#1125)
- **High Availability Support**: Added redundant scheduler support for high availability (#1147)
- **TLS/mTLS Support**: Added TLS/mTLS support for coordinator service
- **Scheduler Health Check**: Added health check endpoint for scheduler monitoring (#1129) - Thanks to [@jonasban](https://github.com/jonasban) for the feature request
- **Default DAG Sorting Configuration**: Added configuration for default DAG list sorting (#1135)
- **GitHub Repository Link**: Added GitHub repository link to sidebar
- **npm Installation Support**: Added global npm package for easy cross-platform installation via `npm install -g @dagu-org/dagu`

### Improvements
- **Output Capture**: Fixed maximum size setting for output capture
- **Web UI Sidebar**: Replaced automatic hover with manual toggle control, added persistence (#1121) - Thanks to [@ghansham](https://github.com/ghansham) for feedback
- **DAG Sorting**: Moved sorting logic from frontend to backend for proper pagination (#1123, #1134) - Thanks to [@ghansham](https://github.com/ghansham) for reporting
- **Dependency Upgrades**: Updated multiple dependencies (#1127) - Thanks to [@reneleonhardt](https://github.com/reneleonhardt)
- **Duration Display**: Fixed invalid date display in duration fields (#1137)
- **Orphaned DAG Handling**: Fixed handling of orphaned running DAGs after unexpected restarts (#1122)
- **Log File Migration**: Fixed log file path migration from legacy format (#1124)
- **Pagination**: Fixed hardcoded pagination limit (#1126)
- **DAG State Preservation**: Preserve previous DAG state when dequeuing (#1118)

### Bug Fixes
- **Installation Script**: Fixed installer script issues (#1091) - Thanks to [@Sarvesh-11](https://github.com/Sarvesh-11)
- **DAG List Sorting**: Fixed sort key issue in DAG list (#1134) - Thanks to [@ghansham](https://github.com/ghansham) for reporting
- **Next Run Display**: Fixed next run display for timezones (#1138)

### Documentation
- **OIDC Documentation**: Added comprehensive OIDC authentication documentation
- **Heartbeat Interval**: Documented heartbeat interval behavior

### Contributors

Thanks to our contributors for this release:

| Contribution | Contributor |
|--------------|--------|
| Installation script fixes, exponential backoff for retry policies | [@Sarvesh-11](https://github.com/Sarvesh-11) |
| Dependency upgrades | [@reneleonhardt](https://github.com/reneleonhardt) |
| Enhanced repeat policy with while/until modes | [@thefishhat](https://github.com/thefishhat) |
| OIDC authentication implementation | [@Arvintian](https://github.com/Arvintian) |
| Step-level environment variables feature request | [@admerzeau](https://github.com/admerzeau) |
| Live log loading feature request | [@tapir](https://github.com/tapir) |
| Partial success status request, DAG sorting issue, sidebar improvements | [@ghansham](https://github.com/ghansham) |
| Scheduler health check feature request | [@jonasban](https://github.com/jonasban) |

### New Contributors
- [@Sarvesh-11](https://github.com/Sarvesh-11) made their first contribution in [PR 1091](https://github.com/dagu-org/dagu/pull/1091)
- [@reneleonhardt](https://github.com/reneleonhardt) made their first contribution in [PR 1127](https://github.com/dagu-org/dagu/pull/1127)

**Full Changelog**: [v1.17.4...v1.18.0](https://github.com/dagu-org/dagu/compare/v1.17.4...v1.18.0)

## v1.17.4 (2025-06-30)

### New Features
- **Interactive DAG Selection**: Run `dagu start` without arguments to select DAGs interactively (#1074)
- **Bubble Tea Progress Display**: Replaced ANSI progress display with Bubble Tea TUI framework
- **OpenTelemetry Support**: Added distributed tracing with W3C trace context propagation (#1068)
- **Windows Support**: Initial Windows compatibility with PowerShell and cmd.exe (#1066)

### Improvements
- **Scheduler Refactoring**: Cleaned up scheduler code for better maintainability (#1062)
- **Error Handling**: Handle corrupted status files in scheduler queue processing

### Bug Fixes
- **UI**: Fixed 'f' key triggering fullscreen mode while editing DAGs (#1075)
- **SSH Executor**: Fixed handling of `||` and `&&` operators in command parsing (#1067)
- **JSON Schema**: Corrected DAG JSON schema for schedule field (#1071)
- **Scheduler**: Fixed scheduler discarding queued items when scheduled by `enqueue` (#1070)
- **Base DAG**: Fixed parameter parsing issue in base DAG loading

### Documentation
- Updated CLI documentation for interactive DAG selection
- Added OpenTelemetry configuration examples
- Fixed configuration documentation to match implementation
- Added missing feature pages to sidebar

### Contributors

Thanks to our contributors for this release:

| Contribution | Contributor |
|--------------|--------|
| Initial Windows support - PowerShell/cmd.exe compatibility | [@pdoronila](https://github.com/pdoronila) |
| Scheduler refactoring for improved maintainability | [@thefishhat](https://github.com/thefishhat) |
| Interactive DAG selection feature request | [@yottahmd](https://github.com/yottahmd) |
| OpenTelemetry distributed tracing feature request | [@jeremydelattre59](https://github.com/jeremydelattre59) |
| SSH executor double pipe operator (report) | [@NebulaCoding1029](https://github.com/NebulaCoding1029) |
| 'f' key interference in DAG editor (report) | [@NebulaCoding1029](https://github.com/NebulaCoding1029) |
| Log cleanup feature request | [@NebulaCoding1029](https://github.com/NebulaCoding1029) |
| Scheduler queue bug (report) | Jochen |

## v1.17.3 (2025-06-25)

### New Features
- **HTTP Executor**: Added `skipTLSVerify` option to support self-signed certificates (#1046)

### Bug Fixes
- **Configuration**: Fixed DAGU_DAGS_DIR environment variable not being recognized (#1060)
- **SSH Executor**: Fixed stdout and stderr streams being incorrectly merged (#1057)
- **Repeat Policy**: Fixed nodes being marked as failed when using repeat policy with non-zero exit codes (#1052)
- **UI**: Fixed retry individual step functionality for remote nodes (#1049)
- **Environment Variables**: Fixed environment variable evaluation and working directory handling (#1045)
- **Dashboard**: Prevented full page reload on date change and fixed invalid date handling (commit 58ad8e44)

### Documentation
- **Repeat Policy**: Corrected documentation and examples to accurately describe behavior (#1056)

### Contributors

Thanks to our contributors for this release:

| Contribution | Contributor |
|--------------|--------|
| HTTP executor skipTLSVerify feature | [@mnmercer](https://github.com/mnmercer) (report), [@nightly-brew](https://github.com/nightly-brew) (feedback) |
| DAGU_DAGS_DIR environment variable fix | [@Daffdi](https://github.com/Daffdi) (report) |
| SSH executor stdout/stderr separation | [@NebulaCoding1029](https://github.com/NebulaCoding1029) (report) |
| Repeat policy bug fixes and documentation | [@jeremydelattre59](https://github.com/jeremydelattre59) (reports) |
| Retry individual step UI fix | [@jeremydelattre59](https://github.com/jeremydelattre59) (report), [@thefishhat](https://github.com/thefishhat) (implementation) |
| Environment variable evaluation fixes | [@jhuang732](https://github.com/jhuang732) (report) |

## v1.17.2 (2025-06-20)

### Bug Fixes
- **HTTP Executor**: Fixed output not being written to stdout (#1042) - Thanks to [@nightly-brew](https://github.com/nightly-brew) for reporting

### Contributors

Thanks to our contributors for this release:

| Contribution | Contributor |
|--------------|--------|
| HTTP executor output capture fix | [@nightly-brew](https://github.com/nightly-brew) (report) |

## v1.17.1 (2025-06-20)

### New Features
- **One-click Step Re-run**: Retry an individual step without touching the rest of the DAG (#1030)
- **Nested-DAG Log Viewer**: See logs for every repeated sub run instead of only the last execution (#1029)

### Bug Fixes
- **Docker**: Fixed asset serving with base path and corrected storage volume locations (#1037)
- **Docker**: Updated Docker storage paths from `/dagu` to `/var/lib/dagu`
- **Steps**: Support camel case for step exit code field (#1031)

### Contributors

Thanks to our contributors for this release:

| Contribution | Contributor |
|--------------|--------|
| One-click Step Re-run – retry an individual step without touching the rest of the DAG | 🛠️ [@thefishhat](https://github.com/thefishhat) |
| Nested-DAG Log Viewer – see logs for every repeated sub run | 💡 [@jeremydelattre59](https://github.com/jeremydelattre59) |
| Docker image polish – fixes for asset paths & storage volumes | 🐳 [@jhuang732](https://github.com/jhuang732) (report) |

## v1.17.0 (2025-06-18)

### Major Features

####  Improved Performance
- Refactored execution history data for more performant history lookup
- Optimized internal data structures for better scalability

####  Hierarchical DAG Execution
Execute nested DAGs with full parameter passing and output bubbling:
```yaml
steps:
  - name: run_sub-dag
    run: sub-dag
    output: OUT
    params: "INPUT=${DATA_PATH}"

  - name: use output
    command: echo ${OUT.outputs.RESULT}
```

####  Multiple DAGs in Single File
Define multiple DAGs in one YAML file using `---` separator:
```yaml
name: main-workflow
steps:
  - name: process
    run: sub-workflow  # Defined below

---

name: sub-workflow
steps:
  - name: task
    command: echo "Hello from sub-workflow"
```

####  Parallel Execution with Parameters
Execute commands or sub-DAGs in parallel with different parameters for batch processing:
```yaml
steps:
  - name: get files
    command: find /data -name "*.csv"
    output: FILES
  
  - name: process files
    run: process-file
    parallel: ${FILES}
    params:
      - FILE_NAME: ${ITEM}
```

####  Enhanced Web UI
- Overall UI improvements with better user experience
- Cleaner design and more intuitive navigation
- Better performance for large DAG visualizations

####  Advanced History Search
New execution history page with:
- Date-range filtering
- Status filtering (success, failure, running, etc.)
- Improved search performance
- Better timeline visualization

####  Better Debugging
- **Precondition Results**: Display actual results of precondition evaluations in the UI
- **Output Variables**: Show output variable values in the UI for easier debugging
- **Separate Logs**: stdout and stderr are now separated by default for clearer log analysis

####  Queue Management
Added enqueue functionality for both API and UI:
```bash
# Queue a DAG for later execution
dagu enqueue --run-id=custom-id my-dag.yaml

# Dequeue
dagu dequeue my-dag.yaml
```

####  Partial Success Status
New "partial success" status for workflows where some steps succeed and others fail, providing better visibility into complex workflow states.

####  API v2
- New `/api/v2` endpoints with refactored schema
- Better abstractions and cleaner interfaces
- Improved error handling and response formats
- See [OpenAPI spec](https://github.com/dagu-org/dagu/blob/main/api/v2/api.yaml) for details

### Docker Improvements

####  Optimized Images
Thanks to @jerry-yuan:
- Significantly reduced Docker image size
- Split into three baseline images for different use cases
- Better layer caching for faster builds

####  Container Enhancements
Thanks to @vnghia:
- Allow specifying container name
- Support for image platform selection
- Better container management options

### Enhanced Features

####  Advanced Repeat Policy
Thanks to @thefishhat:
- **Enhanced in v1.17.5**: Explicit 'while' and 'until' modes for clear repeat logic
- Conditions for repeat execution
- Expected output matching  
- Exit code-based repeats

```yaml
steps:
  - name: wait for service
    command: check_service.sh
    repeatPolicy:
      repeat: until        # NEW: Explicit mode (while/until)
      condition: "${STATUS}"
      expected: "ready"    # Repeat UNTIL status is ready
      intervalSec: 30
      limit: 60           # Maximum attempts
      
  - name: monitor process
    command: pgrep myapp
    repeatPolicy:
      repeat: while       # Repeat WHILE process exists
      exitCode: [0]       # Exit code 0 means found
      intervalSec: 10
```

### Bug Fixes & Improvements

- Fixed history data migration issues
- Improved error messages and logging
- Better handling of edge cases in DAG execution
- Performance improvements for large workflows
- Various UI/UX enhancements: #925, #898, #895, #868, #903, #911, #913, #921, #923, #887, #922, #932, #962

### Breaking Changes

####  DAG Type Field (v1.17.0-beta.13+)

Starting from v1.17.0-beta.13, DAGs now have a `type` field that controls step execution behavior:

- **`type: chain`** (new default): Steps are automatically connected in sequence, even if no dependencies are specified
- **`type: graph`** (previous behavior): Steps only depend on explicitly defined dependencies

To maintain the previous behavior, add `type: graph` to your DAG configuration:

```yaml
type: graph
steps:
  - name: task1
    command: echo "runs in parallel"
  - name: task2
    command: echo "runs in parallel"
```

Alternatively, you can explicitly set empty dependencies for parallel steps:

```yaml
steps:
  - name: task1
    command: echo "runs in parallel"
    depends: []
  - name: task2
    command: echo "runs in parallel"
    depends: []
```

### Migration Required

 **History Data Migration**: Due to internal improvements, history data from 1.16.x requires migration:

```bash
# Migrate history data
dagu migrate history
```

After successful migration, legacy history directories are moved to `<DAGU_DATA_DIR>/history_migrated_<timestamp>` for safekeeping.

### Contributors

Huge thanks to our contributors for this release:

| Contribution | Contributor |
|--------------|--------|
| Optimized Docker image size and split into baseline images | [@jerry-yuan](https://github.com/jerry-yuan) |
| Container name & image platform support | [@vnghia](https://github.com/vnghia) |
| Enhanced repeat-policy conditions | [@thefishhat](https://github.com/thefishhat) |
| Queue functionality implementation | [@kriyanshii](https://github.com/kriyanshii) |
| Partial success status | [@thefishhat](https://github.com/thefishhat) |
| Countless reviews & feedback | [@ghansham](https://github.com/ghansham) |

### Installation

Try the beta version:

```bash
# Docker
docker run --rm -p 8080:8080 ghcr.io/dagu-org/dagu:latest dagu start-all

# Or download specific version
curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | bash -s -- --version v1.17.0-beta
```

---

## v1.16.0 (2025-01-09)

### New Features

####  Enhanced Docker Image
- Base image updated to `ubuntu:24.04`
- Pre-installed common tools: `sudo`, `git`, `curl`, `jq`, `python3`, and more
- Ready for production use with essential utilities

####  Dotenv File Support
Load environment variables from `.env` files:

```yaml
dotenv: /path/to/.env
# or multiple files
dotenv:
  - .env
  - .env.production
```

#### 🔗 JSON Reference Expansion
Access nested JSON values with path syntax:

```yaml
steps:
  - name: sub workflow
    run: sub_workflow
    output: SUB_RESULT
  - name: use output
    command: echo "The result is ${SUB_RESULT.outputs.finalValue}"
```

If `SUB_RESULT` contains:
```json
{
  "outputs": {
    "finalValue": "success"
  }
}
```
Then `${SUB_RESULT.outputs.finalValue}` expands to `success`.

####  Advanced Preconditions

**Regex Support**: Use `re:` prefix for pattern matching:
```yaml
steps:
  - name: some_step
    command: some_command
    preconditions:
      - condition: "`date '+%d'`"
        expected: "re:0[1-9]"  # Run only on days 01-09
```

**Command Preconditions**: Test conditions with commands:
```yaml
steps:
  - name: some_step
    command: some_command
    preconditions:
      - command: "test -f /tmp/some_file"
```

####  Enhanced Parameter Support

**List Format**: Define parameters as key-value pairs:
```yaml
params:
  - PARAM1: value1
  - PARAM2: value2
```

**CLI Flexibility**: Support both named and positional parameters:
```bash
# Positional
dagu start my_dag -- param1 param2

# Named
dagu start my_dag -- PARAM1=value1 PARAM2=value2

# Mixed
dagu start my_dag -- param1 param2 --param3 value3
```

####  Enhanced Continue On Conditions

**Exit Code Matching**:
```yaml
steps:
  - name: some_step
    command: some_command
    continueOn:
      exitCode: [1, 2]  # Continue if exit code is 1 or 2
```

**Mark as Success**:
```yaml
steps:
  - name: some_step
    command: some_command
    continueOn:
      exitCode: 1
      markSuccess: true  # Mark successful even if failed
```

**Output Matching**:
```yaml
steps:
  - name: some_step
    command: some_command
    continueOn:
      output: "WARNING"  # Continue if output contains "WARNING"
      
  # With regex
  - name: another_step
    command: another_command
    continueOn:
      output: "re:^ERROR: [0-9]+"  # Regex pattern matching
```

#### 🐚 Shell Features

**Piping Support**:
```yaml
steps:
  - name: pipe_example
    command: "cat file.txt | grep pattern | wc -l"
```

**Custom Shell Selection**:
```yaml
steps:
  - name: bash_specific
    command: "echo ${BASH_VERSION}"
    shell: bash
    
  - name: python_shell
    command: "print('Hello from Python')"
    shell: python3
```

####  Sub-workflow Output
Parent workflows now receive structured output from sub-workflows:

```json
{
  "name": "some_subworkflow",
  "params": "PARAM1=param1 PARAM2=param2",
  "outputs": {
    "RESULT1": "Some output",
    "RESULT2": "Another output"
  }
}
```

#### 🔗 Simplified Dependencies
String format now supported:
```yaml
steps:
  - name: first
    command: echo "First"
  - name: second
    command: echo "Second"
    depends: first  # Simple string instead of array
```

### Improvements

- **Environment Variable Expansion**: Now supported in most DAG fields
- **UI Enhancements**: Improved DAG visualization for better readability
- **Storage Optimization**: Reduced state file sizes by removing redundant data

### Bug Fixes

- Fixed: DAGs with dots (`.`) in names can now be edited in the Web UI

### Contributors

Thanks to our contributor for this release:

| Contribution | Contributor |
|--------------|--------|
| Improved parameter handling for CLI - support for both named and positional parameters | [@kriyanshii](https://github.com/kriyanshii) |

---

## Previous Versions

For older versions, please refer to the [GitHub releases page](https://github.com/dagu-org/dagu/releases).

## Version Support

- **Current**: v1.16.x (latest features and bug fixes)
- **Previous**: v1.15.x (bug fixes only)
- **Older**: Best effort support

## Migration Guides

### Upgrading to v1.16.0

Most changes are backward compatible. Key considerations:

1. **Docker Users**: The new Ubuntu base image includes more tools but is slightly larger
2. **Parameter Format**: Both old and new formats are supported
3. **State Files**: Old state files are automatically compatible

### Breaking Changes

None in v1.16.0

## See Also

- [Installation Guide](/getting-started/installation) - Upgrade instructions
- [Configuration Reference](/reference/config) - New configuration options
- [Examples](/writing-workflows/examples) - New feature examples
