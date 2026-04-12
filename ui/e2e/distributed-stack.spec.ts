import { expect, test, type APIRequestContext } from '@playwright/test';
import {
  enqueueRunFromUI,
  getQueue,
  getStepStdout,
  getWorkers,
  loadStack,
  loginViaAPI,
  loginViaUI,
  waitForQueueCounts,
  waitForWorkerSet,
} from './helpers/e2e';

const DAG_FILE = 'e2e-distributed-queue.yaml';
const DAG_NAME = 'e2e-distributed-queue';
const STEP_NAME = 'hold-and-report';
const EXPECTED_STEP_OUTPUT = 'e2e distributed worker ok';

test('exercises the web UI against the real distributed shared-nothing worker stack', async ({
  page,
  request,
}) => {
  test.slow();

  const stack = await loadStack();
  await loginViaUI(page, stack.auth.adminUsername, stack.auth.adminPassword);
  const token = await loginViaAPI(
    request,
    stack.auth.adminUsername,
    stack.auth.adminPassword
  );

  await waitForWorkerSet(request, token, stack.workers);
  await waitForQueueCounts(request, token, stack.queues.shared, {
    runningCount: 0,
    queuedCount: 0,
  });

  await page.goto('/queues');
  const sharedQueueCard = page.getByRole('link', {
    name: new RegExp(stack.queues.shared, 'i'),
  });
  await expect(sharedQueueCard).toBeVisible();
  await expect(sharedQueueCard).toContainText('No activity');

  await page.goto('/system-status');
  await expect(page.getByRole('heading', { name: 'System Status' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Workers' })).toBeVisible();
  for (const workerId of stack.workers) {
    await expect(page.getByText(workerId)).toBeVisible();
  }
  await expect(page.getByText('role=e2e')).toHaveCount(stack.workers.length);

  await page.goto(`/dags/${DAG_FILE}`);
  await expect(page.getByText(DAG_NAME)).toBeVisible();

  const firstRunId = await enqueueRunFromUI(page, DAG_FILE);
  const activeWorkerId = await waitForWorkerState(
    request,
    token,
    {
      busyPollers: 1,
      hasTask: true,
    },
    30_000
  );
  await waitForQueueCounts(
    request,
    token,
    stack.queues.shared,
    {
      runningCount: 1,
      queuedCount: 0,
    },
    30_000
  );

  const secondRunId = await enqueueRunFromUI(page, DAG_FILE);
  expect(secondRunId).not.toBe(firstRunId);
  await waitForQueueCounts(
    request,
    token,
    stack.queues.shared,
    {
      runningCount: 1,
      queuedCount: 1,
    },
    30_000
  );

  await page.goto(`/queues/${stack.queues.shared}`);
  await expect(
    page.getByText(stack.queues.shared, { exact: true }).first()
  ).toBeVisible();
  await expect(page.getByText('Running (1)')).toBeVisible();
  await expect(page.getByText('Queued (1)')).toBeVisible();
  await expect(page.getByText(DAG_NAME).first()).toBeVisible();

  await page.goto('/system-status');
  await page.getByText(activeWorkerId).click();
  await expect(page.getByText(DAG_NAME)).toBeVisible();

  await waitForWorkerState(
    request,
    token,
    {
      busyPollers: 0,
      hasTask: false,
    },
    120_000
  );
  await waitForQueueCounts(
    request,
    token,
    stack.queues.shared,
    {
      runningCount: 0,
      queuedCount: 0,
    },
    120_000
  );

  await page.goto(`/queues/${stack.queues.shared}`);
  await expect(
    page.getByText('No DAG runs are currently executing in this queue.')
  ).toBeVisible();
  await expect(page.getByText('No queued items in this queue.')).toBeVisible();

  await expect
    .poll(() => getStepStdout(request, token, DAG_NAME, firstRunId, STEP_NAME), {
      timeout: 30_000,
    })
    .toContain(EXPECTED_STEP_OUTPUT);
  await expect
    .poll(() => getStepStdout(request, token, DAG_NAME, secondRunId, STEP_NAME), {
      timeout: 30_000,
    })
    .toContain(EXPECTED_STEP_OUTPUT);
});

async function waitForWorkerState(
  request: APIRequestContext,
  token: string,
  expected: { busyPollers: number; hasTask: boolean },
  timeout: number
): Promise<string> {
  let activeWorkerId = '';

  await expect
    .poll(
      async () => {
        const workers = await getWorkers(request, token);
        const activeWorker = workers.workers.find((worker) =>
          worker.runningTasks.some((task) => task.dagName === DAG_NAME)
        );
        const busyWorker = activeWorker ?? workers.workers.find((worker) => worker.busyPollers > 0);
        activeWorkerId = activeWorker?.id ?? busyWorker?.id ?? '';

        return {
          busyPollers: busyWorker?.busyPollers ?? 0,
          hasTask: Boolean(activeWorker),
        };
      },
      {
        timeout,
      }
    )
    .toEqual(expected);

  return activeWorkerId;
}
