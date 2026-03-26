# Current SSE Connection Management Research

This document describes the current SSE connection design in this repository as it exists on disk today.

Scope:
- Frontend SSE hook layer
- Frontend shared connection manager
- Backend multiplexed SSE stream and topic mutation handlers
- Backend topic polling model
- Cockpit-specific use of the shared SSE system

Method:
- Every statement below was derived from the current code under `ui/src/hooks`, `ui/src/features/cockpit`, `ui/src/features/dags`, `internal/service/frontend/server.go`, `internal/service/frontend/sse`, and the API fetchers in `internal/service/frontend/api/v1`.
- This document intentionally separates verified facts from implications. Where something is an implication of the code rather than a runtime measurement, it is labeled as such.

## 1. Executive Summary

Verified facts:

1. The browser does not open SSE connections to `/events/dags`, `/events/dag-runs`, `/events/queues`, or `/events/docs*`. Those are frontend logical endpoint strings only. The actual wire endpoints are:
   - `GET /api/v1/events/stream`
   - `POST /api/v1/events/stream/topics`
   Source: `ui/src/hooks/SSEManager.ts:91-188`, `internal/service/frontend/server.go:1073-1100`

2. Inside one browser tab, all non-agent SSE consumers share one `EventSource` per `(apiURL, remoteNode)`.
   Source: `ui/src/hooks/SSEManager.ts:59-60`, `ui/src/hooks/SSEManager.ts:232-304`

3. The shared connection carries one or more canonical topic strings such as `dagruns:fromDate=...&toDate=...` or `dag:example.yaml`. The browser adds or removes topics by `POST /events/stream/topics`.
   Source: `ui/src/hooks/SSEManager.ts:91-188`, `ui/src/hooks/SSEManager.ts:306-324`, `ui/src/hooks/SSEManager.ts:549-661`

4. The backend does not push events from an internal event bus. Each topic owns a poller that repeatedly re-fetches the corresponding REST-shaped payload, hashes it, and emits an SSE message only when the hash changes.
   Source: `internal/service/frontend/sse/multiplex.go:922-1114`

5. The polling interval of a topic is adaptive and grows with fetch duration.
   Source: `internal/service/frontend/sse/multiplex.go:1037-1062`

6. Cockpit currently subscribes to one `dagruns:` topic that spans the full date range of all currently loaded kanban dates, not just today.
   Source: `ui/src/features/cockpit/hooks/useCockpitDagRuns.ts:46-111`

7. Cockpit template search is not on the SSE path. It uses ordinary `GET /dags/tags` and `GET /dags` queries only.
   Source: `ui/src/features/cockpit/components/TemplateSelector.tsx:39-58`

8. Cockpit preview currently reuses the full DAG side panel and opens it on the `spec` tab. That panel subscribes to the DAG SSE topic and also fetches `/dags/{fileName}`. The `spec` tab then fetches `/dags/{fileName}/spec`.
   Source: `ui/src/features/cockpit/components/DAGPreviewModal.tsx:54-64`, `ui/src/features/dags/components/dag-details/DAGDetailsSidePanel.tsx:166-180`, `ui/src/features/dags/components/dag-details/DAGDetailsContent.tsx:157-172`, `ui/src/features/dags/components/dag-editor/DAGSpec.tsx:89-128`

9. Agent chat SSE is currently disabled on purpose. Agent session updates use polling instead.
   Source: `ui/src/features/agent/hooks/useSSEConnection.ts:12-32`

## 2. Terminology

The code uses three different names for one live subscription concept:

- Logical SSE endpoint:
  - Example: `/events/dag-runs?fromDate=...&toDate=...`
  - Used only inside frontend hook wrappers and `endpointToTopic`
  - Not a real backend route

- Canonical topic:
  - Example: `dagruns:fromDate=...&toDate=...`
  - This is the stable identity of a live subscription inside the browser and server multiplexer

- Wire endpoint:
  - `GET /api/v1/events/stream?topic=...`
  - `POST /api/v1/events/stream/topics`
  - These are the only backend multiplex SSE endpoints registered in production

## 3. High-Level Architecture

```mermaid
flowchart LR
  A[React page/component] --> B[useDAG* / useQueue* / useDoc* wrapper]
  B --> C[useSSE]
  C --> D[SSEManager singleton in tab]
  D -->|GET /api/v1/events/stream?topic=...| E[frontend SSE MultiplexHandler]
  D -->|POST /api/v1/events/stream/topics| E
  E --> F[Multiplexer session]
  F --> G[multiplexTopic per canonical topic]
  G --> H[registered fetcher in API v1]
  H --> I[file-backed stores / managers]
  H --> G
  G -->|payload changed| F
  F -->|event: message {topic,payload}| D
  D --> C
  C --> J[component state]
  C --> K[useSSECacheSync]
  K --> L[SWR cache]
```

## 4. Frontend Design

### 4.1 Hook stack

All normal UI SSE hooks are thin wrappers around `useSSE`.

Examples:
- `useDAGRunsListSSE` -> `/events/dag-runs?...` in `ui/src/hooks/useDAGRunsListSSE.ts`
- `useDAGsListSSE` -> `/events/dags?...` in `ui/src/hooks/useDAGsListSSE.ts`
- `useDAGSSE` -> `/events/dags/{fileName}` in `ui/src/hooks/useDAGSSE.ts`
- `useDAGHistorySSE` -> `/events/dags/{fileName}/dag-runs`
- `useDAGRunSSE` -> `/events/dag-runs/{name}/{dagRunId}`
- `useDAGRunLogsSSE` -> `/events/dag-runs/{name}/{dagRunId}/logs`
- `useStepLogSSE` -> `/events/dag-runs/{name}/{dagRunId}/logs/steps/{stepName}`
- `useQueuesListSSE`, `useQueueItemsSSE`, `useDocTreeSSE`, `useDocSSE`

These wrappers do not open `EventSource` directly. They pass a logical endpoint string into `useSSE`.

Source:
- `ui/src/hooks/useSSE.ts:42-103`
- `ui/src/hooks/useDAGRunsListSSE.ts:19-24`
- `ui/src/hooks/useDAGsListSSE.ts:22-27`
- `ui/src/hooks/useDAGSSE.ts:17-22`
- `ui/src/hooks/useDAGHistorySSE.ts:12-17`
- `ui/src/hooks/useDAGRunSSE.ts:10-16`
- `ui/src/hooks/useDAGRunLogsSSE.ts:28-36`
- `ui/src/hooks/useStepLogSSE.ts:11-18`
- `ui/src/hooks/useQueueItemsSSE.ts:11-16`
- `ui/src/hooks/useDocTreeSSE.ts:10-18`
- `ui/src/hooks/useDocSSE.ts:6-15`

### 4.2 `useSSE` behavior

`useSSE` does four things:

1. Reads `remoteNode` from `AppBarContext`
2. Reads `apiURL` from config
3. Resets local SSE state when `(endpoint, remoteNode, apiURL, enabled)` changes
4. Subscribes through the shared `sseManager`

Important detail:
- The reset happens synchronously during render by calling `setState` when the computed connection key changes.
- That is an intentional pattern in the current code.

Source: `ui/src/hooks/useSSE.ts:42-101`

### 4.3 Frontend topic canonicalization

`SSEManager.endpointToTopic()` maps logical endpoints to canonical topic keys.

Current mapping:

| Logical endpoint shape | Canonical topic |
| --- | --- |
| `/events/dags?...` | `dagslist:<query>` |
| `/events/dags/{file}` | `dag:{file}` |
| `/events/dags/{file}/dag-runs` | `daghistory:{file}` |
| `/events/dag-runs?...` | `dagruns:<query>` |
| `/events/dag-runs/{name}/{runId}` | `dagrun:{name}/{runId}` |
| `/events/dag-runs/{name}/{runId}/logs?...` | `dagrunlogs:{name}/{runId}?<query>` |
| `/events/dag-runs/{name}/{runId}/logs/steps/{step}` | `steplog:{name}/{runId}/{step}` |
| `/events/queues?...` | `queues:<query>` |
| `/events/queues/{queue}/items` | `queueitems:{queue}` |
| `/events/docs-tree?...` | `doctree:<query>` |
| `/events/docs/{path...}` | `doc:{path...}` |

Important detail:
- `token` and `remoteNode` are removed from the canonical topic query string.
- `remoteNode` is part of the shared connection key instead.

Source: `ui/src/hooks/SSEManager.ts:67-158`

### 4.4 Shared connection model in the browser

`SSEManager` stores a `ManagedConnection` keyed by `apiURL|remoteNode`.

That object contains:
- one `EventSource`
- current backend `sessionId`
- last received `lastEventId`
- retry timers and connect timeout
- topic registry for this tab/runtime
- `serverTopics`, `pendingAdd`, `pendingRemove`

This means:
- two components in the same tab and same `remoteNode` do not open two separate `EventSource`s if they can share one stream
- opening a new live screen mutates the shared topic set instead of opening a second stream

Source: `ui/src/hooks/SSEManager.ts:26-44`, `ui/src/hooks/SSEManager.ts:209-304`

### 4.5 Per-topic state semantics

The frontend exposes `SSEState<T>` with:
- `data`
- `error`
- `isConnected`
- `isConnecting`
- `shouldUseFallback`

These are not purely stream-level values.

For one topic:
- `isConnected` is true only when the shared connection is connected and the topic is present in `serverTopics`
- `isConnecting` is true when the shared connection is connecting, or when the topic is still in `pendingAdd`
- `shouldUseFallback` is copied from the shared connection state

Source: `ui/src/hooks/SSEManager.ts:190-203`

This is important because a topic added to an already healthy stream can still report:
- `isConnected = false`
- `isConnecting = true`

until the topic mutation response updates `serverTopics`.

### 4.6 Browser connect sequence

When the first topic for a given `(apiURL, remoteNode)` is added:

1. `handleTopicAdded()` calls `ensureConnected()`
2. `connect()` builds `GET {apiURL}/events/stream?topic=...&remoteNode=...`
3. The auth token from local storage is sent as a `token` query parameter
4. The browser opens `new EventSource(url)`
5. A 15s connect timeout is armed
6. The browser waits for a `control` event

Source: `ui/src/hooks/SSEManager.ts:306-324`, `ui/src/hooks/SSEManager.ts:375-469`

On `control`:
- `sessionId` is stored
- `serverTopics` is replaced with the server's subscribed list
- `retryCount` resets to 0
- `pendingAdd` entries already covered by the initial URL are cleared
- connection state flips to connected
- any remaining pending add/remove work is flushed immediately

Source: `ui/src/hooks/SSEManager.ts:426-463`

### 4.7 Browser add/remove topic mutation sequence

If the stream is already connected and a new topic is added:

1. The topic is placed in `pendingAdd`
2. Topic state is reported as connecting
3. `scheduleMutation()` debounces a flush by 200ms
4. `flushMutation()` sends:

```json
{
  "sessionID": "<current session id>",
  "add": ["topic-a"],
  "remove": ["topic-b"]
}
```

to `POST {apiURL}/events/stream/topics?remoteNode=<remoteNode>`

5. Auth is sent using standard `Authorization` headers from `fetch`, not the `token` query parameter
6. On success or partial success, `serverTopics` is updated from the response

Source: `ui/src/hooks/SSEManager.ts:306-313`, `ui/src/hooks/SSEManager.ts:549-661`

Important details:
- A `404` mutation response is treated specially as "session expired". The current `EventSource` is closed and reconnect flow begins.
- A `403` mutation response is not treated as a fatal transport error. The manager expects partial success semantics and still consumes `subscribed` and `errors`.
- Stale mutation responses are ignored if the connection has already reconnected with a different `sessionId` or different `EventSource`.

Source: `ui/src/hooks/SSEManager.ts:596-645`

### 4.8 Browser message handling

The stream uses:
- `event: control` for the initial control frame
- `event: message` for actual topic payloads
- `: heartbeat` comment frames for keepalive

For each `message` frame:
- `lastEventId` is recorded if present
- JSON is parsed as `{ topic, payload }`
- the payload is cached as `lastPayload` for that topic
- all subscribers for that topic receive `onData(payload)`

Source:
- frontend consumer: `ui/src/hooks/SSEManager.ts:471-505`
- backend frame writer: `internal/service/frontend/sse/multiplex.go:821-920`, `internal/service/frontend/sse/multiplex.go:1116-1120`

### 4.9 Disconnect and retry behavior

On transport error:
- current session ID is cleared
- `serverTopics`, `pendingAdd`, and `pendingRemove` are cleared
- state becomes disconnected
- reconnect is scheduled with exponential backoff

Current retry delay:
- 1s, 2s, 4s, 8s, 16s cap

`shouldUseFallback` is driven by `retryCount >= 5`.

Source: `ui/src/hooks/SSEManager.ts:205-207`, `ui/src/hooks/SSEManager.ts:518-547`

### 4.10 SWR fallback and cache sync

Pages usually combine SSE with SWR like this:

1. `useQuery(...)`
2. `sseFallbackOptions(sseResult)`
3. `useSSECacheSync(sseResult, mutate)`

Current behavior of `sseFallbackOptions()`:
- Polling is disabled while `isConnected`
- Polling is also disabled while `isConnecting` and `shouldUseFallback` is false
- The intent is to avoid polling during handshake/settling

Source: `ui/src/hooks/useSSECacheSync.ts:10-25`

Current behavior of `useSSECacheSync()`:
- it only writes into SWR when `sseResult.isConnected` is true
- it mutates SWR cache with `revalidate: false`
- an optional transform can map SSE payload shape to the SWR endpoint shape

Source: `ui/src/hooks/useSSECacheSync.ts:45-68`

Implication:
- A topic that is still pending add reports `isConnecting = true`, and polling remains suppressed during that window.
- Whether that is harmless or problematic depends on how long the topic stays in that state at runtime.

## 5. Backend Design

### 5.1 Registered routes

Production SSE routes are registered only here:

- `GET /api/v1/events/stream`
- `POST /api/v1/events/stream/topics`

Source: `internal/service/frontend/server.go:1073-1100`

There are no production routes for:
- `/api/v1/events/dags`
- `/api/v1/events/dag-runs`
- `/api/v1/events/queues`
- `/api/v1/events/docs*`

Those strings exist only on the frontend as logical endpoint names that are converted into multiplex topics.

### 5.2 Authentication model

The stream route uses `QueryTokenMiddleware()` because `EventSource` cannot add arbitrary headers.

Behavior:
- if the request already has `Authorization`, it is kept
- else if the query string contains `token`, it is converted to `Authorization: Bearer <token>`

Source: `internal/service/frontend/auth/middleware.go:38-60`

The SSE routes also use the normal auth middleware and client IP middleware.

Source: `internal/service/frontend/server.go:1092-1096`

If auth mode is `none`, a default admin user is injected into stream requests.

Source: `internal/service/frontend/server.go:1257-1279`

### 5.3 Remote node proxy behavior

If `remoteNode` is present and not `local`:
- `GET /events/stream` is proxied to the remote node
- `POST /events/stream/topics` is proxied to the remote node

While proxying:
- `remoteNode` is removed from the forwarded query
- `token` is also removed from the forwarded query
- remote node auth is applied through `node.ApplyAuth(req)`

Source: `internal/service/frontend/sse/multiplex_handler.go:47-50`, `internal/service/frontend/sse/multiplex_handler.go:75-78`, `internal/service/frontend/sse/multiplex_handler.go:133-199`, `internal/service/frontend/sse/multiplex_handler.go:214-236`

### 5.4 Session creation on the backend

`HandleStream()` does:

1. set SSE headers
2. parse `lastEventId` from query or `Last-Event-ID` header
3. create a multiplex session with the requested initial topics
4. write a `control` event first
5. bootstrap snapshot messages for the initial topics
6. enter the streaming loop

Source: `internal/service/frontend/sse/multiplex_handler.go:41-70`

### 5.5 Topic mutation handling on the backend

`HandleTopicMutation()`:

1. parses JSON request body
2. requires `sessionID`
3. mutates the existing backend session
4. if new topics were added, immediately bootstraps snapshot messages for them
5. writes JSON response with subscribed topics and optional errors

Error mapping:
- unknown session -> `404` with `{"error":"unknown_session","message":"unknown_session"}`
- too many topics -> `400`
- conflicting add/remove -> `400`
- invalid topic -> `400`

Source: `internal/service/frontend/sse/multiplex_handler.go:73-115`

Important fact:
- A `404` on `POST /events/stream/topics` means the backend handler ran and `getSession()` returned `ErrUnknownSession`. It is not "no route registered".

### 5.6 Multiplexer data model

The backend `Multiplexer` owns:
- active sessions
- shared topic registry
- fetcher registry
- optional authorizer registry
- config like max clients, max topics, heartbeat interval, write buffer, slow client timeout

Source: `internal/service/frontend/sse/multiplex.go:90-147`

Important fact:
- Production code registers fetchers but does not register any topic authorizers.
- `RegisterAuthorizer()` exists, but `rg -n "RegisterAuthorizer\\(" internal` finds only the function definition and tests.

Current production fetcher registration:
- DAG details
- DAG history
- DAG list
- DAG run details
- DAG run list
- DAG run logs
- step log
- queue list
- queue items
- doc
- doc tree

Source: `internal/service/frontend/server.go:1105-1115`

### 5.7 Topic parsing and canonicalization on the backend

`ParseTopic()` validates and canonicalizes topic strings.

Behavior:
- empty topic rejected
- malformed `type:identifier` rejected
- query-bearing topic types have `token` and `remoteNode` stripped
- DAG and DAG history validate DAG file names via `core.ValidateDAGName`
- unknown topic types remain parseable for forward compatibility, but still require a non-empty identifier

Source: `internal/service/frontend/sse/topic_parse.go:23-95`, `internal/service/frontend/sse/types.go:109-129`

### 5.8 Session mutation semantics

`applyMutation()` performs:

1. parse add/remove topics
2. reject same topic in both add and remove
3. apply per-topic authorizers if any exist
4. reject unsupported topic types as partial errors, not hard failure
5. resolve or create topic objects
6. enforce max topics per connection
7. remove old topics
8. attach new topics

Partial error behavior:
- unsupported and unauthorized add requests are accumulated in `Errors`
- the request still succeeds for valid topics
- HTTP status becomes `403` if there were partial errors

Source: `internal/service/frontend/sse/multiplex.go:256-396`

### 5.9 Shared topic registry

The backend shares one `multiplexTopic` object per canonical topic key across all sessions.

If 20 sessions all subscribe to `dag:test.yaml`:
- there is one poller for that topic
- all sessions attach to it

Source: `internal/service/frontend/sse/multiplex.go:483-519`, `internal/service/frontend/sse/multiplex_test.go:298-313`

### 5.10 Session queue and backpressure

Each session has:
- per-topic coalescing queue
- max write buffer size
- oldest-message dropping
- forced disconnect if one message exceeds the write buffer or the queue cannot be kept within bounds

Source: `internal/service/frontend/sse/multiplex.go:703-819`

Important detail:
- Each topic can have only one queued message at a time. Newer messages replace older queued messages for the same topic.

### 5.11 Topic poller model

Each `multiplexTopic`:
- starts polling when the first session attaches
- stops when retired
- fetches its current payload by calling the registered fetcher
- JSON-marshals that payload
- hashes the bytes
- emits only if the hash changed

Source: `internal/service/frontend/sse/multiplex.go:922-1114`

Adaptive interval:
- base interval starts at 1s
- max interval cap is 10s
- new interval is an EMA of the old interval and `max(base, min(3 * fetchDuration, max))`

Source: `internal/service/frontend/sse/types.go:31-39`, `internal/service/frontend/sse/multiplex.go:1042-1062`

Bootstrap behavior:
- initial stream open bootstraps snapshots for the initially subscribed topics
- topic mutation bootstraps snapshots for newly added topics
- if reconnecting with `lastEventId`, bootstrap skips topics whose `lastChangeEventID` is not newer than that ID

Source: `internal/service/frontend/sse/multiplex_handler.go:59-70`, `internal/service/frontend/sse/multiplex_handler.go:108-114`, `internal/service/frontend/sse/multiplex.go:692-700`, `internal/service/frontend/sse/multiplex.go:1002-1005`

## 6. Topic Catalog and Payload Sources

| Topic type | Identifier shape | Backend fetcher | Payload source |
| --- | --- | --- | --- |
| `dag` | `fileName` | `GetDAGDetailsData` | `getDAGDetailsData()` |
| `daghistory` | `fileName` | `GetDAGHistoryData` | recent run history |
| `dagslist` | query string | `GetDAGsListData` | DAG store list |
| `dagrun` | `dagName/dagRunId` | `GetDAGRunDetailsData` | single DAG run |
| `dagruns` | query string | `GetDAGRunsListData` | DAG run store list |
| `dagrunlogs` | `dagName/dagRunId[?tail=...]` | `GetDAGRunLogsData` | scheduler log + step summary |
| `steplog` | `dagName/dagRunId/stepName` | `GetStepLogData` | stdout/stderr tails |
| `queues` | query string | `GetQueuesListData` | queue summary |
| `queueitems` | `queueName` | `GetQueueItemsData` | running + queued items |
| `doc` | doc path | `GetDocContentData` | doc store content |
| `doctree` | query string | `GetDocTreeData` | doc tree listing |

Source:
- `internal/service/frontend/server.go:1105-1115`
- `internal/service/frontend/api/v1/dags.go:366-405`, `internal/service/frontend/api/v1/dags.go:1322-1388`
- `internal/service/frontend/api/v1/dagruns.go:2406-2595`
- `internal/service/frontend/api/v1/queues.go:281-293`
- `internal/service/frontend/api/v1/docs.go:368-418`

Important detail for DAG topics:
- The DAG details SSE payload already includes `spec`.
- That comes from `getDAGDetailsData()` returning `Spec: &yamlSpec`.

Source: `internal/service/frontend/api/v1/dags.go:379-405`

## 7. Cockpit-Specific Current Design

### 7.1 Cockpit page structure

`pages/cockpit/index.tsx` renders:
- `CockpitToolbar`
- `DateKanbanList`

Source: `ui/src/pages/cockpit/index.tsx:23-35`

### 7.2 Cockpit live board data

`DateKanbanList` uses `useCockpitDagRuns(loadedDates, selectedWorkspace)`.

Source: `ui/src/features/cockpit/components/DateKanbanList.tsx:14-47`

`useCockpitDagRuns()` currently:
- computes one `fromDate..toDate` range covering every loaded date section
- subscribes to one `dagruns:` topic for that full range
- issues one `/dag-runs` query for that full range
- writes SSE payloads into the SWR cache
- partitions the returned list into per-date kanban columns client-side

Source: `ui/src/features/cockpit/hooks/useCockpitDagRuns.ts:46-165`

Verified implication:
- As the user scrolls backward and `loadedDates` grows, the live `dagruns:` topic range also grows.

### 7.3 Cockpit template search

`TemplateSelector` is not using SSE.

It performs only regular SWR queries:
- `/dags/tags`
- `/dags`

It pauses those queries until the dropdown has been opened once.

Source: `ui/src/features/cockpit/components/TemplateSelector.tsx:39-58`

### 7.4 Cockpit template preview

`CockpitToolbar` renders `DAGPreviewModal` whenever `selectedTemplate` is non-empty.

Source: `ui/src/features/cockpit/components/CockpitToolbar.tsx:40-50`

Current preview path:

1. `DAGPreviewModal` mounts `DAGDetailsSidePanel`
2. It forces `initialTab="spec"`
3. `DAGDetailsSidePanel` opens `useDAGSSE(fileName, isOpen && !!fileName)` and fetches `/dags/{fileName}`
4. `DAGDetailsContent` renders the tab UI
5. Because the active tab is `spec`, `DAGSpec` mounts
6. `DAGSpec` reuses the parent DAG SSE result when provided, but still fetches `/dags/{fileName}/spec`

Source:
- `ui/src/features/cockpit/components/DAGPreviewModal.tsx:54-64`
- `ui/src/features/dags/components/dag-details/DAGDetailsSidePanel.tsx:166-180`
- `ui/src/features/dags/components/dag-details/DAGDetailsContent.tsx:157-172`, `ui/src/features/dags/components/dag-details/DAGDetailsContent.tsx:339-340`
- `ui/src/features/dags/components/dag-editor/DAGSpec.tsx:89-128`

Important detail:
- `DAGSpec` does not open a second DAG SSE topic if the parent already passed `sseResult`.
- It reuses the parent's DAG SSE subscription.

Source: `ui/src/features/dags/components/dag-editor/DAGSpec.tsx:89-92`

### 7.5 Post-enqueue status tracking in the preview

When enqueue returns a `dagRunId`, `DAGDetailsSidePanel` stores it in `trackedDagRunId`.

It then polls `/dag-runs/{name}/{dagRunId}` every 2s to keep `currentDAGRun` fresh.

That tracked-run path is polling, not SSE.

Source: `ui/src/features/dags/components/dag-details/DAGDetailsSidePanel.tsx:182-203`, `ui/src/features/dags/components/dag-details/DAGDetailsSidePanel.tsx:209-220`

## 8. Verified Observations Relevant to Current Debugging

These are code-backed observations. They are not claims about the exact live runtime failure unless stated as such.

### 8.1 The wire path is multiplexed

If a browser devtools trace shows:
- `GET /api/v1/events/stream?...`
- `POST /api/v1/events/stream/topics?...`

that is the expected live SSE transport in the current architecture.

It does not mean the app is using "some other SSE implementation". This is the intended implementation.

### 8.2 `POST /events/stream/topics` 404 has a very specific backend meaning

In the current code, the backend returns 404 for topic mutation only when the session ID is unknown or expired.

Source: `internal/service/frontend/sse/multiplex_handler.go:93-104`

So a live 404 on that POST means one of these happened:
- the browser sent a stale session ID
- the backend session was already removed
- the request was proxied to a remote node that does not know that session

What it does not mean:
- it does not mean there is no production POST route

### 8.3 A topic can be in a "no polling, not subscribed yet" state

This is a direct result of combining:
- `buildTopicState()` marking `pendingAdd` topics as `isConnecting`
- `sseFallbackOptions()` disabling polling while `isConnecting` and fallback is not active

Source:
- `ui/src/hooks/SSEManager.ts:190-203`, `ui/src/hooks/SSEManager.ts:306-313`
- `ui/src/hooks/useSSECacheSync.ts:14-25`

This state exists by design in the current code.

### 8.4 Cockpit and preview are not isolated live systems

Because the connection is shared per `(apiURL, remoteNode)`:
- the cockpit board topic
- any DAG preview topic
- any other DAG page topic in the same tab and same remote node

all share one browser-side `ManagedConnection` and one backend session.

That means opening the preview mutates the topic set of the same live session already carrying cockpit board updates.

Source: `ui/src/hooks/SSEManager.ts:232-304`, `ui/src/hooks/SSEManager.ts:306-324`

### 8.5 Cockpit live topic size grows with scrollback

`useCockpitDagRuns()` deliberately uses one topic covering the full range of loaded dates.

The backend poller for that topic re-fetches the full payload for that range on every poll, and the adaptive interval depends on fetch duration.

Source:
- `ui/src/features/cockpit/hooks/useCockpitDagRuns.ts:46-111`
- `internal/service/frontend/sse/multiplex.go:1037-1062`
- `internal/service/frontend/api/v1/dagruns.go:2549-2595`

Verified implication:
- if the full-range payload becomes expensive, the live topic will also become slower to refresh

### 8.6 Template search slowness is not directly explained by SSE alone

The template dropdown itself uses ordinary `/dags/tags` and `/dags` GETs, not SSE.

Source: `ui/src/features/cockpit/components/TemplateSelector.tsx:39-58`

If template search is slow, the explanation must involve:
- backend latency for those REST calls
- browser connection starvation / queuing
- or another shared resource issue

The search dropdown does not directly subscribe to SSE topics.

### 8.7 Preview currently combines shared SSE with extra REST work

Opening a cockpit preview currently does all of the following:
- add or reuse DAG SSE topic `dag:{fileName}`
- fetch `/dags/{fileName}`
- because the initial tab is `spec`, fetch `/dags/{fileName}/spec`

Source:
- `ui/src/features/cockpit/components/DAGPreviewModal.tsx:54-64`
- `ui/src/features/dags/components/dag-details/DAGDetailsSidePanel.tsx:166-180`
- `ui/src/features/dags/components/dag-editor/DAGSpec.tsx:94-128`

So preview latency cannot be explained by SSE transport alone. The current preview path always includes non-SSE REST work.

## 9. Files Inspected

Frontend:
- `ui/src/hooks/useSSE.ts`
- `ui/src/hooks/SSEManager.ts`
- `ui/src/hooks/useSSECacheSync.ts`
- `ui/src/hooks/useDAGRunsListSSE.ts`
- `ui/src/hooks/useDAGsListSSE.ts`
- `ui/src/hooks/useDAGSSE.ts`
- `ui/src/hooks/useDAGHistorySSE.ts`
- `ui/src/hooks/useDAGRunSSE.ts`
- `ui/src/hooks/useDAGRunLogsSSE.ts`
- `ui/src/hooks/useStepLogSSE.ts`
- `ui/src/hooks/useQueueItemsSSE.ts`
- `ui/src/hooks/useDocTreeSSE.ts`
- `ui/src/hooks/useDocSSE.ts`
- `ui/src/features/cockpit/hooks/useCockpitDagRuns.ts`
- `ui/src/features/cockpit/components/TemplateSelector.tsx`
- `ui/src/features/cockpit/components/DAGPreviewModal.tsx`
- `ui/src/features/dags/components/dag-details/DAGDetailsSidePanel.tsx`
- `ui/src/features/dags/components/dag-details/DAGDetailsContent.tsx`
- `ui/src/features/dags/components/dag-editor/DAGSpec.tsx`
- `ui/src/features/agent/hooks/useSSEConnection.ts`

Backend:
- `internal/service/frontend/server.go`
- `internal/service/frontend/auth/middleware.go`
- `internal/service/frontend/sse/types.go`
- `internal/service/frontend/sse/topic_parse.go`
- `internal/service/frontend/sse/multiplex.go`
- `internal/service/frontend/sse/multiplex_handler.go`
- `internal/service/frontend/api/v1/dags.go`
- `internal/service/frontend/api/v1/dagruns.go`
- `internal/service/frontend/api/v1/queues.go`
- `internal/service/frontend/api/v1/docs.go`

## 10. Next Use of This Document

This document is only the design baseline.

The next debugging step should use this baseline to answer, with runtime evidence:
- why cockpit's `dagruns:` topic is not producing visible board updates
- why preview requests are delayed or left pending in the user's environment
- whether the failure is session churn, topic mutation churn, connection starvation, backend poll latency, or stale frontend assets

That next step should be done with live traces against the running app, not more theory.
