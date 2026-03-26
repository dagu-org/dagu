## Added

- Coordinator and Worker Health Endpoints: New `/healthz` endpoints for the coordinator and worker services, enabling native health checks in Kubernetes and other orchestrators. (#1802)

## Changed

- Vault Environment Variable Renamed: The HashiCorp Vault environment variable has been renamed for consistency with the `DAGU_` prefix convention. (#1801)
- Centralized Vault Config Defaults: Vault configuration defaults are now centralized, reducing duplication and improving maintainability. (#1804)

## Fixed

- Retry Endpoint Blocking: The retry API endpoint now returns immediately without blocking until the DAG run completes. (#1786)
- Scheduler Health Server Startup: The health server now starts before the scheduler acquires its lock, ensuring health checks pass during lock contention. (#1789)
- Duplicate Workers Across Coordinators: Workers are now deduplicated across multiple coordinator instances in distributed mode, preventing ghost entries in the System Status page. (#1791)
- Bot Session Continuity: Hardened Slack and Telegram bot session management for improved continuity and responsiveness. (#1793)
- Retry Scanner Scope: Narrowed the retry scanner's DAG-run scan scope to reduce unnecessary I/O. (#1794)
- Cancel Failed Auto-Retry DAG Runs: Failed DAG runs with auto-retry enabled can now be properly canceled. (#1795)
- Parallel Scheduling During Sub-DAG Retries: Fixed a deadlock where sub-DAG retries blocked parallel scheduling of other DAGs. (#1796)
- Parallel Sub-DAG Item Targets: Resolved variable expansion for `parallel` item targets in sub-DAG paths and hardened cancellation handling. (#1797)
- Bot Notifications Consolidation: Consolidated and hardened bot notification delivery to prevent duplicate or dropped messages. (#1798)
- SSE Topics and Dev Asset Versioning: Hardened SSE topic routing and fixed dev asset cache-busting. (#1799)
- Agent Approval Prompts: Fixed agent approval prompts being prematurely dismissed during long-running approval waits. (#1800)
- DAG File Traversal via Encoded Slashes: Rejected encoded slashes in DAG file paths to prevent path traversal attacks. (#1803)

## Contributors

Thanks to our contributors for this release:

| Contribution | Contributor |
| --- | --- |
| Retry endpoint non-blocking fix (#1786) | @mvanhorn |
| Vault environment variable rename (#1801) | @dohq |
| Retry endpoint blocking bug report (#608) | @kamandir (report) |
| Scheduler health check misbehavior in multi-instance deployments (#1156) | @jonasban (report) |
| Incorrect System Status in distributed mode (#1787), coordinator/worker health endpoint request (#1788) | @jonasban (report) |
| Task with retry stays in running state, blocking scheduling (#1792) | @mtaohuang (report) |
| Variables not resolved in sub-DAG paths with `parallel` (#1790) | @VKdennis (report) |

**Full Changelog**: https://github.com/dagu-org/dagu/compare/v2.3.0...v2.3.1
