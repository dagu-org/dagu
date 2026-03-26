  # MVP: Automata With Fixed State and User-Defined Stage

  ## Summary

  - Introduce Automata as instance-level, file-backed resources stored under dags/automata/*.yaml. Each Automata has two separate dimensions:
      - fixed lifecycle state: idle, running, waiting, finished
      - user-defined current stage: a label chosen from that Automata’s own declared stage list
  - Reuse the existing agent session/store stack for long-term context. Each Automata owns one durable background session under a synthetic system user;
    before each activation, the controller runs session compaction/handoff and updates the stored session ID if it rotates.
  - Keep the MVP intentionally constrained: one active child DAG at a time per Automata, allowlisted DAG scope only, queue-based DAG execution only, no file
    editing/self-modification, and no overlapping schedule backlog while an Automata is already running or waiting.

  ## Public Interfaces

  - Add an Automata YAML schema with root fields: description, purpose, goal, stages, schedule, allowedDAGs.names, allowedDAGs.tags, agent.model, agent.soul,
    agent.enabledSkills, agent.safeMode, and disabled.
  - Require purpose, goal, and at least one stage for every Automata. Stages are an ordered user-authored list, but transition enforcement is free-jump: the
    agent or user may move the current stage to any declared stage.
  - Persist AutomataState separately from the YAML definition. State stores state, currentStage, stageChangedAt, stageChangedBy, stageNote, sessionID,
    currentRunRef, waitingReason, pendingPromptID, lastSummary, lastError, finishedAt, backoffUntil, and consecutiveErrors.
  - Initialize currentStage to the first declared stage on create/reset. Keep the last assigned stage visible after finished.
  - Extend core.TriggerType and the API schema with automata. Every child DAG run launched by an Automata gets merged tags automata=<name> and
    automata_cycle=<uuid>. Execution history is derived from DAG runs filtered by those tags instead of a duplicate Automata run database.
  - Extend agent session/loop creation with per-session tool allowlists and extra system-prompt text. Automata sessions only get read, think, ask_user,
    search_skills, use_skill, plus new Automata-only tools: list_allowed_dags, run_allowed_dag, retry_automata_run, set_automata_stage, and finish_automata.
    bash, patch, delegate, navigate, and remote tools are not available in Automata mode.
  - Add REST/OpenAPI endpoints for list/detail/spec CRUD and control: list Automata, get detail, get/update raw YAML spec, create/delete, start/restart,
    respond to an Automata prompt, and manually override currentStage.

  ## Implementation Changes

  - Add a file-backed definition store and watcher for dags/automata, separate from DAG loading so Automata YAML never hits the DAG parser. Validation should
    enforce non-empty unique stage names, required purpose/goal, and a non-empty allowlisted DAG candidate set.
  - Add an Automata controller loop inside the scheduler leader process. On each scheduler tick it reconciles all loaded Automata: start a turn when idle and
    due, inspect the current child DAG run when running or waiting, enqueue a follow-up agent turn when a child run completes, skip schedule fires while non-
    idle, and apply exponential backoff only for controller/LLM/tool errors.
  - Derive fixed lifecycle state this way:
      - running: an agent turn is active or a tagged child DAG run is still active
      - waiting: either the agent has a pending ask_user prompt or the current child DAG run is in DAG waiting
      - finished: set only by finish_automata
      - idle: none of the above
  - Treat stage as separate mutable workflow context, not a lifecycle state. set_automata_stage and manual override both validate against the declared stage
    list, update persisted stage metadata, and append a controller note into the backing transcript so the next turn has explicit context.
  - Use the existing agent session store for transcript durability and prompt handling. Automata sessions run under a synthetic system user so they do not
    appear in the normal chat sidebar. The controller injects structured user messages for initial activation, child-run completion/failure summaries, restart
    recovery, and manual stage overrides.
  - Build the Automata prompt from purpose, goal, declared stages, current stage, allowlisted DAG catalog, current child-run status, and recent tagged run
    history. This is the per-Automata “mission brief” the user defines differently for each resource.
  - Launch child DAGs through the existing queue path, not local start, so distributed execution and current scheduler semantics keep working. run_allowed_dag
    must reject non-allowlisted targets and only allow one active child DAG per Automata at a time.
  - Add a minimal UI under /automata with a list page and a detail page.
      - List page columns should include at least name, state, stage, purpose, current child DAG ref, and last update time.
      - Detail page should show current state, current stage, raw spec, transcript, recent executed DAG runs, waiting actions, and a manual stage override
        control.
      - Agent-originated prompts can be answered inline. Child DAG approval waits should deep-link to the existing DAG run approval UI instead of duplicating
        that workflow.
      - Use REST polling, not a new SSE stream: 2s while running or waiting, 15s otherwise.
  - Use existing audit plumbing to record Automata create/update/delete/start/restart/respond/stage-override actions under a new automata audit category.
    Reuse current run/write permission checks: write permissions for create/update/delete, run permissions for start/restart/respond/stage override.

  ## Test Plan

  - Loader/store tests: valid and invalid Automata YAML, required purpose/goal, unique/non-empty stage validation, explicit DAG name validation, tag-based
    candidate resolution, disabled definitions, and file watcher add/update/delete behavior.
  - Controller tests: idle Automata starts when due, overlapping schedules are skipped, multiple Automata reconcile in parallel, finish_automata becomes
    terminal, controller backoff works, and scheduler restart restores running or waiting state from persisted session/run refs.
  - Stage tests: initial stage defaults to the first declared stage, agent stage changes can jump to any declared stage, invalid stage names are rejected,
    manual stage override persists immediately, and list/detail responses always return both state and stage.
  - Agent/tool tests: restricted tool set is enforced, run_allowed_dag injects triggerType=automata plus tags, retry_automata_run only targets runs owned by
    the same Automata, set_automata_stage updates transcript and state metadata, ask_user drives waiting, and finish_automata is rejected while a child DAG is
    still active.
  - Integration tests: one Automata advances through user-defined stages while launching allowlisted DAGs and finishes after the follow-up turn; one Automata
    handles a failed child DAG by retrying or switching DAGs on the next turn; one Automata enters waiting on an agent prompt and resumes after response; one
    Automata enters waiting because a child DAG approval step pauses, then resumes after the DAG approval is completed.
  - API/UI tests: list/detail/control endpoints, state-plus-stage rendering, inline prompt response flow, manual stage override flow, child DAG waiting link
    flow, and polling-based status refresh.

  ## Assumptions

  - MVP Automata are instance-level resources, not per-user-owned resources.
  - MVP supports one active child DAG per Automata at a time; parallel child DAG fan-out is out of scope.
  - MVP does not add a separate Automata memory file. Long-term context is the durable session transcript, session compaction handoff, and tagged DAG-run
    history.
  - MVP stages are user-defined labels, not a transition graph. Free-jump validation only checks membership in the declared stage list.
  - Manual stage override never kills an active child DAG; it updates persisted context immediately and is seen by the next agent turn.
  - Child DAGs may still run in existing local or distributed modes, but the Automata controller itself runs only on the scheduler leader because it depends
    on local session/state storage and human approval handling.



