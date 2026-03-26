  # MVP: File-Backed Automata Controller

  ## Summary

  - Introduce Automata as instance-level, file-backed resources stored under dags/automata/*.yaml. Each Automata has durable controller state in data/
    automata/<name>/state.json, a backing agent session, and a public status of idle, running, waiting, or finished.
  - Reuse the existing agent session/store stack for long-term context. Each Automata owns one durable background session under a synthetic system user;
    before each activation, the controller runs session compaction/handoff and updates the stored session ID if it rotates.
  - Keep the MVP intentionally constrained: one active child DAG at a time per Automata, allowlisted DAG scope only, queue-based DAG execution only, no file
    editing/self-modification, and no overlapping schedule backlog while an Automata is already running or waiting.

  ## Public Interfaces

  - Add an Automata YAML schema with root fields: description, instruction, schedule, allowedDAGs.names, allowedDAGs.tags, agent.model, agent.soul,
    agent.enabledSkills, agent.safeMode, and disabled. schedule reuses the existing DAG cron shape; if omitted, the Automata is manual-start only.
  - Define persisted types: AutomataStatus, AutomataState, WaitingReason, and ExecutionRef. State stores sessionID, currentRunRef, waitingReason,
    pendingPromptID, lastSummary, lastError, finishedAt, backoffUntil, and consecutiveErrors.
  - Extend core.TriggerType and the API schema with automata. Every child DAG run launched by an Automata gets merged tags automata=<name> and
    automata_cycle=<uuid>. Execution history is derived from DAG runs filtered by those tags instead of a duplicate Automata run database.
  - Extend agent session/loop creation with per-session tool allowlists and extra system-prompt text. Automata sessions only get read, think, ask_user,
    search_skills, use_skill, plus new Automata-only tools: list_allowed_dags, run_allowed_dag, retry_automata_run, and finish_automata. bash, patch,
    delegate, navigate, and remote tools are not available in Automata mode.
  - Add REST/OpenAPI endpoints for list/detail/spec CRUD and control: list Automata, get detail, get/update raw YAML spec, create/delete, start/restart, and
    respond to an Automata prompt.

  ## Implementation Changes

  - Add a file-backed definition store and watcher for dags/automata, separate from DAG loading so Automata YAML never hits the DAG parser. Load-time
    validation should resolve explicit DAG names against the DAG store and cache the current candidate catalog used in prompts.
  - Add an Automata controller loop inside the scheduler leader process. On each scheduler tick it reconciles all loaded Automata: start a turn when idle and
    due, inspect the current child DAG run when running or waiting, enqueue a follow-up agent turn when a child run completes, skip schedule fires while non-
    idle, and apply exponential backoff only for controller/LLM/tool errors.
  - Derive status this way: running means an agent turn is active or a tagged child DAG run is still active; waiting means either the agent has a pending
    ask_user prompt or the current child DAG run is in DAG waiting; finished is set only by finish_automata; otherwise the Automata is idle.
  - Use the existing agent session store for transcript durability and prompt handling. Automata sessions run under a synthetic system user so they do not
    appear in the normal chat sidebar. The controller injects structured user messages for initial activation, child-run completion/failure summaries, and
    restart recovery. A failed child DAG does not create a separate Automata status; it becomes input to the next agent turn so the agent can retry, switch
    DAGs, ask for approval, or finish.
  - Launch child DAGs through the existing queue path, not local start, so distributed execution and current scheduler semantics keep working. run_allowed_dag
    must reject non-allowlisted targets and only allow one active child DAG per Automata at a time.
  - Add a minimal UI under /automata with a list page and a detail page. The detail page shows current status, raw spec, transcript, recent executed DAG runs,
    and waiting actions. Agent-originated prompts can be answered inline. Child DAG approval waits should deep-link to the existing DAG run approval UI
    instead of duplicating that workflow. Use REST polling, not a new SSE stream: 2s while running or waiting, 15s otherwise.
  - Use existing audit plumbing to record Automata create/update/delete/start/restart/respond actions under a new automata audit category. Reuse current run/
    write permission checks: write permissions for create/update/delete, run permissions for start/restart/respond.

  ## Test Plan

  - Loader/store tests: valid and invalid Automata YAML, explicit DAG name validation, tag-based candidate resolution, disabled definitions, and file watcher
    add/update/delete behavior.
  - Controller tests: idle Automata starts when due, overlapping schedules are skipped, multiple Automata reconcile in parallel, finish_automata becomes
    terminal, controller backoff works, and scheduler restart restores running or waiting state from persisted session/run refs.
  - Agent/tool tests: restricted tool set is enforced, run_allowed_dag injects triggerType=automata plus tags, retry_automata_run only targets runs owned by
    the same Automata, ask_user drives waiting, and finish_automata is rejected while a child DAG is still active.
  - Integration tests: one Automata launches an allowlisted DAG and finishes after the follow-up turn; one Automata handles a failed child DAG by retrying or
    switching DAGs on the next turn; one Automata enters waiting on an agent prompt and resumes after response; one Automata enters waiting because a child
    DAG approval step pauses, then resumes after the DAG approval is completed.
  - API/UI tests: list/detail/control endpoints, transcript rendering, inline prompt response flow, child DAG waiting link flow, and polling-based status
    refresh.

  ## Assumptions

  - MVP Automata are instance-level resources, not per-user-owned resources.
  - MVP supports one active child DAG per Automata at a time; parallel child DAG fan-out is out of scope.
  - MVP does not add a separate Automata memory file. Long-term context is the durable session transcript, session compaction handoff, and tagged DAG-run
    history.
  - MVP does not let Automata edit repository files or mutate DAG definitions automatically.
  - Child DAGs may still run in existing local or distributed modes, but the Automata controller itself runs only on the scheduler leader because it depends
    on local session/state storage and human approval handling.

