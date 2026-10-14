// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { expect, test } from '@playwright/test';
import {
  loadStack,
  loginViaAPI,
  loginViaUI,
  startDAG,
  uniqueName,
  waitForDAGAvailable,
  waitForRunStatus,
  writeLocalDAG,
} from './helpers/e2e';

// The run page is the one place a run is selected, and its URL keeps the
// open tab and step so a link or reload lands on the same view.
test('opens runs, steps, and tabs from their links', async ({
  page,
  request,
}) => {
  const stack = await loadStack();
  const token = await loginViaAPI(
    request,
    stack.auth.adminUsername,
    stack.auth.adminPassword
  );
  const dagName = uniqueName('run-links');
  const fileName = await writeLocalDAG(
    dagName,
    `
name: ${dagName}
steps:
  - name: prepare
    run: echo prepared
  - name: publish
    run: echo published
    depends: prepare
`
  );
  await waitForDAGAvailable(request, token, fileName);
  const runId = await startDAG(request, token, fileName);
  await waitForRunStatus(request, token, dagName, runId, ['succeeded']);
  await loginViaUI(page, stack.auth.adminUsername, stack.auth.adminPassword);

  await page.goto(
    `/dags/${encodeURIComponent(fileName)}?dagRunId=${runId}&dagRunName=${dagName}`
  );
  await expect
    .poll(() => new URL(page.url()).pathname)
    .toBe(`/dag-runs/${dagName}/${runId}`);

  await page.goto(`/dag-runs/${dagName}/${runId}?step=publish`);
  await expect(page.getByRole('dialog', { name: 'publish' })).toBeVisible();
  await page.keyboard.press('Escape');
  await expect
    .poll(() => new URL(page.url()).searchParams.get('step'))
    .toBeNull();

  await page.getByRole('button', { name: 'Timeline', exact: true }).click();
  await expect
    .poll(() => new URL(page.url()).searchParams.get('tab'))
    .toBe('timeline');
  await page.reload();
  // The timeline replaces the step table that the Status tab shows.
  await expect(
    page.getByText('prepare', { exact: true }).first()
  ).toBeVisible();
  await expect(
    page.getByRole('columnheader', { name: 'Step Name' })
  ).toHaveCount(0);
  expect(new URL(page.url()).searchParams.get('tab')).toBe('timeline');
});
