import React from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import useSWR from 'swr';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { AppBarContext } from '@/contexts/AppBarContext';
import fetchJson from '@/lib/fetchJson';
import LoadingIndicator from '@/ui/LoadingIndicator';

declare const getConfig: () => { apiURL: string };

type AutomataSummary = {
  name: string;
  description?: string;
  purpose: string;
  goal: string;
  state: string;
  stage?: string;
  disabled?: boolean;
  lastUpdatedAt?: string;
  currentRun?: {
    name: string;
    dagRunId: string;
    status: string;
  };
};

type AutomataDetail = {
  definition: {
    name: string;
    description?: string;
    purpose: string;
    goal: string;
    stages: string[];
    disabled?: boolean;
  };
  state: {
    state: string;
    currentStage?: string;
    waitingReason?: string;
    pendingPrompt?: {
      id: string;
      question: string;
      options?: { id: string; label: string; description?: string }[];
      allowFreeText?: boolean;
      freeTextPlaceholder?: string;
    };
    sessionId?: string;
    currentRunRef?: { name: string; id: string };
    lastSummary?: string;
    lastError?: string;
  };
  allowedDags: {
    name: string;
    description?: string;
    tags?: string[];
  }[];
  currentRun?: {
    name: string;
    dagRunId: string;
    status: string;
  };
  recentRuns?: {
    name: string;
    dagRunId: string;
    status: string;
    startedAt?: string;
    finishedAt?: string;
    error?: string;
  }[];
  messages?: {
    id: string;
    type: string;
    content?: string;
    created_at?: string;
    user_prompt?: {
      question: string;
    };
    tool_results?: { content: string; is_error?: boolean }[];
  }[];
};

async function sendJSON(
  path: string,
  method: string,
  body?: unknown
): Promise<void> {
  const token = localStorage.getItem('dagu_auth_token');
  const response = await fetch(`${getConfig().apiURL}${path}`, {
    method,
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: body ? JSON.stringify(body) : undefined,
  });

  if (!response.ok) {
    let message = response.statusText;
    try {
      const data = await response.json();
      message = data?.message || message;
    } catch {
      // keep status text
    }
    throw new Error(message);
  }
}

function statusClass(state: string): string {
  switch (state) {
    case 'running':
      return 'bg-sky-100 text-sky-800 dark:bg-sky-900/40 dark:text-sky-200';
    case 'waiting':
      return 'bg-amber-100 text-amber-900 dark:bg-amber-900/40 dark:text-amber-200';
    case 'finished':
      return 'bg-emerald-100 text-emerald-900 dark:bg-emerald-900/40 dark:text-emerald-200';
    default:
      return 'bg-muted text-muted-foreground';
  }
}

function AutomataPage(): React.ReactElement {
  const appBar = React.useContext(AppBarContext);
  const navigate = useNavigate();
  const { name } = useParams();
  const [stageOverride, setStageOverride] = React.useState('');
  const [stageNote, setStageNote] = React.useState('');
  const [freeTextResponse, setFreeTextResponse] = React.useState('');
  const [selectedOptions, setSelectedOptions] = React.useState<string[]>([]);
  const [error, setError] = React.useState('');

  React.useEffect(() => {
    appBar.setTitle('Automata');
  }, [appBar]);

  const listQuery = useSWR<{ automata: AutomataSummary[] }>(
    '/automata',
    fetchJson,
    { refreshInterval: 15000 }
  );

  const detailQuery = useSWR<AutomataDetail>(
    name ? `/automata/${encodeURIComponent(name)}` : null,
    fetchJson,
    {
      refreshInterval: (data) =>
        data?.state?.state === 'running' || data?.state?.state === 'waiting'
          ? 2000
          : 15000,
    }
  );

  const specQuery = useSWR<{ spec: string }>(
    name ? `/automata/${encodeURIComponent(name)}/spec` : null,
    fetchJson,
    { refreshInterval: 15000 }
  );

  const detail = detailQuery.data;

  React.useEffect(() => {
    if (detail?.definition?.stages?.length && !stageOverride) {
      const initialStage =
        detail.state?.currentStage ?? detail.definition.stages[0] ?? '';
      if (initialStage) {
        setStageOverride(initialStage);
      }
    }
  }, [detail?.definition?.stages, detail?.state?.currentStage, stageOverride]);

  const onStart = async () => {
    if (!name) return;
    setError('');
    try {
      await sendJSON(`/automata/${encodeURIComponent(name)}/start`, 'POST');
      void detailQuery.mutate();
      void listQuery.mutate();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to start automata');
    }
  };

  const onOverrideStage = async () => {
    if (!name || !stageOverride) return;
    setError('');
    try {
      await sendJSON(`/automata/${encodeURIComponent(name)}/stage`, 'POST', {
        stage: stageOverride,
        note: stageNote,
      });
      setStageNote('');
      void detailQuery.mutate();
      void listQuery.mutate();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update stage');
    }
  };

  const onRespond = async () => {
    if (!name || !detail?.state?.pendingPrompt) return;
    setError('');
    try {
      await sendJSON(`/automata/${encodeURIComponent(name)}/response`, 'POST', {
        promptId: detail.state.pendingPrompt.id,
        selectedOptionIds: selectedOptions,
        freeTextResponse,
      });
      setSelectedOptions([]);
      setFreeTextResponse('');
      void detailQuery.mutate();
      void listQuery.mutate();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to respond');
    }
  };

  return (
    <div className="grid grid-cols-1 gap-6 p-4 lg:grid-cols-[360px_minmax(0,1fr)]">
      <section className="rounded-xl border bg-card">
        <div className="border-b px-4 py-3">
          <h2 className="text-sm font-semibold tracking-wide text-muted-foreground uppercase">
            Automata
          </h2>
        </div>
        {listQuery.isLoading ? (
          <LoadingIndicator />
        ) : (
          <div className="max-h-[calc(100vh-12rem)] overflow-y-auto p-2">
            {(listQuery.data?.automata || []).map((item) => (
              <button
                key={item.name}
                onClick={() =>
                  navigate(`/automata/${encodeURIComponent(item.name)}`)
                }
                className={`mb-2 w-full rounded-lg border p-3 text-left transition ${
                  name === item.name
                    ? 'border-primary bg-primary/5'
                    : 'border-border hover:bg-muted/50'
                }`}
              >
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <div className="font-medium">{item.name}</div>
                    <div className="mt-1 text-xs text-muted-foreground">
                      {item.purpose}
                    </div>
                  </div>
                  <span
                    className={`rounded-full px-2 py-1 text-[11px] font-medium ${statusClass(item.state)}`}
                  >
                    {item.state}
                  </span>
                </div>
                <div className="mt-2 flex items-center justify-between text-xs text-muted-foreground">
                  <span>Stage: {item.stage || 'n/a'}</span>
                  {item.currentRun ? (
                    <span>{item.currentRun.status}</span>
                  ) : null}
                </div>
              </button>
            ))}
          </div>
        )}
      </section>

      <section className="rounded-xl border bg-card">
        {!name ? (
          <div className="p-8 text-sm text-muted-foreground">
            Select an Automata to inspect its state, stage, transcript, and
            recent DAG runs.
          </div>
        ) : detailQuery.isLoading ? (
          <LoadingIndicator />
        ) : detail ? (
          <div className="space-y-6 p-4">
            <div className="flex flex-wrap items-start justify-between gap-4">
              <div>
                <h1 className="text-2xl font-semibold">
                  {detail.definition.name}
                </h1>
                {detail.definition.description ? (
                  <p className="mt-1 text-sm text-muted-foreground">
                    {detail.definition.description}
                  </p>
                ) : null}
              </div>
              <div className="flex items-center gap-2">
                <span
                  className={`rounded-full px-3 py-1 text-xs font-medium ${statusClass(detail.state.state)}`}
                >
                  {detail.state.state}
                </span>
                <span className="rounded-full bg-muted px-3 py-1 text-xs font-medium">
                  {detail.state.currentStage || 'no stage'}
                </span>
                <Button onClick={onStart}>Start</Button>
              </div>
            </div>

            {error ? (
              <div className="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
                {error}
              </div>
            ) : null}

            <div className="grid gap-4 lg:grid-cols-2">
              <div className="rounded-lg border p-4">
                <h2 className="mb-2 text-sm font-semibold">Mission</h2>
                <div className="space-y-2 text-sm">
                  <p>
                    <span className="font-medium">Purpose:</span>{' '}
                    {detail.definition.purpose}
                  </p>
                  <p>
                    <span className="font-medium">Goal:</span>{' '}
                    {detail.definition.goal}
                  </p>
                  {detail.state.lastSummary ? (
                    <p>
                      <span className="font-medium">Last Summary:</span>{' '}
                      {detail.state.lastSummary}
                    </p>
                  ) : null}
                  {detail.state.lastError ? (
                    <p className="text-destructive">
                      <span className="font-medium">Last Error:</span>{' '}
                      {detail.state.lastError}
                    </p>
                  ) : null}
                </div>
              </div>

              <div className="rounded-lg border p-4">
                <h2 className="mb-3 text-sm font-semibold">Stage Override</h2>
                <div className="space-y-3">
                  <select
                    className="w-full rounded-md border bg-background px-3 py-2 text-sm"
                    value={stageOverride}
                    onChange={(e) => setStageOverride(e.target.value)}
                  >
                    {detail.definition.stages.map((stage) => (
                      <option key={stage} value={stage}>
                        {stage}
                      </option>
                    ))}
                  </select>
                  <Input
                    value={stageNote}
                    onChange={(e) => setStageNote(e.target.value)}
                    placeholder="Optional note"
                  />
                  <Button variant="outline" onClick={onOverrideStage}>
                    Update Stage
                  </Button>
                </div>
              </div>
            </div>

            {detail.state.pendingPrompt ? (
              <div className="rounded-lg border border-amber-400/40 bg-amber-50/50 p-4 dark:bg-amber-950/20">
                <h2 className="mb-2 text-sm font-semibold">
                  Waiting For Human Input
                </h2>
                <p className="mb-3 text-sm">
                  {detail.state.pendingPrompt.question}
                </p>
                <div className="space-y-2">
                  {(detail.state.pendingPrompt.options || []).map((option) => {
                    const selected = selectedOptions.includes(option.id);
                    return (
                      <label
                        key={option.id}
                        className="flex cursor-pointer items-start gap-2 rounded-md border p-2 text-sm"
                      >
                        <input
                          type="checkbox"
                          checked={selected}
                          onChange={(e) => {
                            setSelectedOptions((prev) =>
                              e.target.checked
                                ? [...prev, option.id]
                                : prev.filter((id) => id !== option.id)
                            );
                          }}
                        />
                        <span>
                          <span className="font-medium">{option.label}</span>
                          {option.description ? (
                            <span className="block text-xs text-muted-foreground">
                              {option.description}
                            </span>
                          ) : null}
                        </span>
                      </label>
                    );
                  })}
                  {detail.state.pendingPrompt.allowFreeText ? (
                    <Textarea
                      value={freeTextResponse}
                      onChange={(e) => setFreeTextResponse(e.target.value)}
                      placeholder={
                        detail.state.pendingPrompt.freeTextPlaceholder ||
                        'Enter response'
                      }
                    />
                  ) : null}
                  <Button onClick={onRespond}>Submit Response</Button>
                </div>
              </div>
            ) : null}

            <div className="grid gap-4 lg:grid-cols-2">
              <div className="rounded-lg border p-4">
                <h2 className="mb-3 text-sm font-semibold">Allowed DAGs</h2>
                <div className="space-y-2 text-sm">
                  {detail.allowedDags.map((dag) => (
                    <div key={dag.name} className="rounded-md border p-2">
                      <div className="font-medium">{dag.name}</div>
                      {dag.description ? (
                        <div className="text-xs text-muted-foreground">
                          {dag.description}
                        </div>
                      ) : null}
                      {dag.tags?.length ? (
                        <div className="mt-1 text-[11px] text-muted-foreground">
                          {dag.tags.join(', ')}
                        </div>
                      ) : null}
                    </div>
                  ))}
                </div>
              </div>

              <div className="rounded-lg border p-4">
                <h2 className="mb-3 text-sm font-semibold">Current Run</h2>
                {detail.currentRun ? (
                  <div className="space-y-2 text-sm">
                    <div>
                      <span className="font-medium">DAG:</span>{' '}
                      {detail.currentRun.name}
                    </div>
                    <div>
                      <span className="font-medium">Run ID:</span>{' '}
                      {detail.currentRun.dagRunId}
                    </div>
                    <div>
                      <span className="font-medium">Status:</span>{' '}
                      {detail.currentRun.status}
                    </div>
                    <Link
                      className="text-primary underline underline-offset-2"
                      to={`/dag-runs/${encodeURIComponent(detail.currentRun.name)}/${encodeURIComponent(detail.currentRun.dagRunId)}`}
                    >
                      Open DAG run
                    </Link>
                  </div>
                ) : (
                  <div className="text-sm text-muted-foreground">
                    No active child DAG run.
                  </div>
                )}
              </div>
            </div>

            <div className="rounded-lg border p-4">
              <h2 className="mb-3 text-sm font-semibold">Recent Runs</h2>
              <div className="space-y-2">
                {(detail.recentRuns || []).map((run) => (
                  <div
                    key={`${run.name}:${run.dagRunId}`}
                    className="grid gap-1 rounded-md border p-2 text-sm lg:grid-cols-[1fr_160px_140px]"
                  >
                    <div>
                      <div className="font-medium">{run.name}</div>
                      <div className="text-xs text-muted-foreground">
                        {run.dagRunId}
                      </div>
                    </div>
                    <div>{run.status}</div>
                    <div className="text-xs text-muted-foreground">
                      {run.finishedAt || run.startedAt || ''}
                    </div>
                  </div>
                ))}
              </div>
            </div>

            <div className="grid gap-4 lg:grid-cols-2">
              <div className="rounded-lg border p-4">
                <h2 className="mb-3 text-sm font-semibold">Transcript</h2>
                <div className="max-h-[28rem] space-y-2 overflow-y-auto">
                  {(detail.messages || []).slice(-40).map((message) => (
                    <div
                      key={message.id}
                      className="rounded-md border p-2 text-sm"
                    >
                      <div className="mb-1 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
                        {message.type}
                      </div>
                      {message.content ? (
                        <div className="whitespace-pre-wrap">
                          {message.content}
                        </div>
                      ) : null}
                      {message.user_prompt?.question ? (
                        <div className="whitespace-pre-wrap">
                          {message.user_prompt.question}
                        </div>
                      ) : null}
                      {message.tool_results?.length ? (
                        <div className="mt-2 space-y-1">
                          {message.tool_results.map((result, index) => (
                            <div
                              key={index}
                              className="rounded bg-muted p-2 text-xs whitespace-pre-wrap"
                            >
                              {result.content}
                            </div>
                          ))}
                        </div>
                      ) : null}
                    </div>
                  ))}
                </div>
              </div>

              <div className="rounded-lg border p-4">
                <h2 className="mb-3 text-sm font-semibold">Raw Spec</h2>
                <pre className="max-h-[28rem] overflow-auto rounded-md bg-muted p-3 text-xs">
                  {specQuery.data?.spec || ''}
                </pre>
              </div>
            </div>
          </div>
        ) : (
          <div className="p-8 text-sm text-muted-foreground">
            Automata definition not found.
          </div>
        )}
      </section>
    </div>
  );
}

export default AutomataPage;
