import { expect, test, type APIRequestContext, type Page } from '@playwright/test';

const DAG_FILE = 'e2e-distributed-queue.yaml';
const DAG_NAME = 'e2e-distributed-queue';
const QUEUE_NAME = 'e2e-shared';
const WORKER_ID = 'worker-1';

type QueueDetails = {
  name: string;
  runningCount: number;
  queuedCount: number;
  running: Array<{ name: string; dagRunId: string }>;
};

type WorkersResponse = {
  workers: Array<{
    id: string;
    busyPollers: number;
    labels: Record<string, string>;
    runningTasks: Array<{ dagName: string; dagRunId: string }>;
  }>;
};

test('exercises the web UI against the real distributed queue stack', async ({
  page,
  request,
}) => {
  test.slow();

  await expect
    .poll(async () => {
      const workers = await getWorkers(request);
      return workers.workers.some((worker) => worker.id === WORKER_ID);
    })
    .toBeTruthy();

  await expect.poll(() => getQueueCounts(request)).toEqual({
    runningCount: 0,
    queuedCount: 0,
  });

  await page.goto('/queues');
  await expect(page.getByRole('link', { name: /e2e-shared/i })).toBeVisible();
  await expect(page.getByText('No activity')).toBeVisible();

  await page.goto('/system-status');
  await expect(page.getByRole('heading', { name: 'System Status' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Workers' })).toBeVisible();
  await expect(page.getByText(WORKER_ID)).toBeVisible();
  await expect(page.getByText('role=e2e')).toBeVisible();

  await page.goto(`/dags/${DAG_FILE}`);
  await expect(page.getByText(DAG_NAME)).toBeVisible();

  const firstRunId = await enqueueRunFromUI(page);
  await waitForWorkerState(
    request,
    {
      busyPollers: 1,
      hasTask: true,
    },
    30_000
  );
  await expect
    .poll(() => getQueueCounts(request), {
      timeout: 30_000,
    })
    .toEqual({
      runningCount: 1,
      queuedCount: 0,
    });

  const secondRunId = await enqueueRunFromUI(page);
  expect(secondRunId).not.toBe(firstRunId);
  await expect
    .poll(() => getQueueCounts(request), {
      timeout: 30_000,
    })
    .toEqual({
      runningCount: 1,
      queuedCount: 1,
    });

  await page.goto(`/queues/${QUEUE_NAME}`);
  await expect(page.getByText(QUEUE_NAME, { exact: true }).first()).toBeVisible();
  await expect(page.getByText('Running (1)')).toBeVisible();
  await expect(page.getByText('Queued (1)')).toBeVisible();
  await expect(page.getByText(DAG_NAME).first()).toBeVisible();

  await page.goto('/system-status');
  await page.getByText(WORKER_ID).click();
  await expect(page.getByText(DAG_NAME)).toBeVisible();
  await waitForWorkerState(
    request,
    {
      busyPollers: 1,
      hasTask: true,
    },
    30_000
  );

  await waitForWorkerState(
    request,
    {
      busyPollers: 0,
      hasTask: false,
    },
    120_000
  );
  await expect
    .poll(() => getQueueCounts(request), {
      timeout: 120_000,
    })
    .toEqual({
      runningCount: 0,
      queuedCount: 0,
    });

  await page.goto(`/queues/${QUEUE_NAME}`);
  await expect(
    page.getByText('No DAG runs are currently executing in this queue.')
  ).toBeVisible();
  await expect(page.getByText('No queued items in this queue.')).toBeVisible();
});

async function enqueueRunFromUI(page: Page): Promise<string> {
  const responsePromise = page.waitForResponse(
    (response) =>
      response.request().method() === 'POST' &&
      response.url().includes(`/api/v1/dags/${encodeURIComponent(DAG_FILE)}/enqueue`)
  );

  await page.getByRole('button', { name: 'Enqueue' }).first().click();

  const dialog = page.getByRole('dialog');
  await expect(dialog).toBeVisible();

  const enqueueToggle = dialog.getByRole('checkbox', { name: 'Enqueue' });
  if ((await enqueueToggle.getAttribute('data-state')) !== 'checked') {
    await enqueueToggle.click();
  }

  await dialog.getByRole('button', { name: /^Enqueue$/ }).click();

  const response = await responsePromise;
  expect(response.ok()).toBeTruthy();

  const body = (await response.json()) as { dagRunId?: string };
  expect(body.dagRunId).toBeTruthy();
  await expect(dialog).toBeHidden();

  return body.dagRunId as string;
}

async function getQueueCounts(
  request: APIRequestContext
): Promise<{ runningCount: number; queuedCount: number }> {
  const queue = await getQueue(request);
  return {
    runningCount: queue.runningCount,
    queuedCount: queue.queuedCount,
  };
}

async function getQueue(request: APIRequestContext): Promise<QueueDetails> {
  const response = await request.get(`/api/v1/queues/${encodeURIComponent(QUEUE_NAME)}`);
  expect(response.ok()).toBeTruthy();
  return (await response.json()) as QueueDetails;
}

async function getWorkers(
  request: APIRequestContext
): Promise<WorkersResponse> {
  const response = await request.get('/api/v1/workers');
  expect(response.ok()).toBeTruthy();
  return (await response.json()) as WorkersResponse;
}

async function waitForWorkerState(
  request: APIRequestContext,
  expected: { busyPollers: number; hasTask: boolean },
  timeout: number
): Promise<void> {
  await expect
    .poll(
      async () => {
        const worker = (await getWorkers(request)).workers.find(
          (candidate) => candidate.id === WORKER_ID
        );
        return {
          busyPollers: worker?.busyPollers ?? 0,
          hasTask:
            worker?.runningTasks.some((task) => task.dagName === DAG_NAME) ??
            false,
        };
      },
      {
        timeout,
      }
    )
    .toEqual(expected);
}
