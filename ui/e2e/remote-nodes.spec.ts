import { expect, test } from '@playwright/test';
import {
  loadStack,
  loginViaAPI,
  loginViaUI,
  selectRemoteNode,
  uniqueName,
  waitForDAGAvailable,
  writeLocalDAG,
  writeRemoteDAG,
} from './helpers/e2e';

test('adds a remote node and browses remote DAGs through the UI', async ({
  page,
  request,
}) => {
  const stack = await loadStack();
  await loginViaUI(page, stack.auth.adminUsername, stack.auth.adminPassword);
  const token = await loginViaAPI(
    request,
    stack.auth.adminUsername,
    stack.auth.adminPassword
  );

  const localDagName = uniqueName('e2e-local-only');
  const remoteDagName = uniqueName('e2e-remote-only');
  await writeLocalDAG(
    localDagName,
    `
name: ${localDagName}
steps:
  - name: local-step
    command: echo "local only"
`
  );
  const remoteFileName = await writeRemoteDAG(
    remoteDagName,
    `
name: ${remoteDagName}
steps:
  - name: remote-step
    command: echo "remote only"
`
  );

  await waitForDAGAvailable(request, token, `${localDagName}.yaml`);
  await expect
    .poll(async () => {
      const response = await request.get(
        `${stack.remote.apiBaseURL}/dags/${encodeURIComponent(remoteFileName)}`
      );
      return response.ok();
    })
    .toBeTruthy();

  const remoteNodeName = uniqueName('remote-node');
  await page.goto('/remote-nodes');
  await page.getByRole('button', { name: 'Add Node' }).click();

  const dialog = page.getByRole('dialog');
  await expect(dialog).toBeVisible();
  await dialog.getByLabel('Name').fill(remoteNodeName);
  await dialog.getByLabel('Description').fill('E2E remote node');
  await dialog.getByLabel('API Base URL').fill(stack.remote.apiBaseURL);
  await dialog.getByRole('button', { name: 'Create' }).click();

  await expect(dialog).toBeHidden();

  const row = page.getByRole('row').filter({ hasText: remoteNodeName });
  await expect(row).toBeVisible();
  await row.getByRole('button', { name: 'Actions' }).click();
  await page.getByRole('menuitem', { name: 'Test Connection' }).click();
  await expect(row).toContainText('OK');

  await selectRemoteNode(page, remoteNodeName);
  await page.getByRole('link', { name: 'Definitions' }).click();
  await expect(page).toHaveURL(/\/dags$/);
  await expect(page.getByRole('main')).toContainText(remoteDagName);
  await expect(page.getByRole('main')).not.toContainText(localDagName);
});
