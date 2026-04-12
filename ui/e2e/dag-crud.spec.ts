// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { expect, test } from '@playwright/test';
import {
  loadStack,
  loginViaAPI,
  loginViaUI,
  uniqueName,
  waitForDAGAvailable,
  writeLocalDAG,
} from './helpers/e2e';

test.describe('DAG CRUD operations', () => {
  test.beforeEach(async ({ page }) => {
    const stack = await loadStack();
    await loginViaUI(page, stack.auth.adminUsername, stack.auth.adminPassword);
  });

  test('creates a new DAG from the UI', async ({ page }) => {
    const dagName = uniqueName('e2e-create');

    await page.goto('/dags/');
    await page.getByRole('button', { name: 'Create new DAG' }).click();

    const dialog = page.getByRole('dialog');
    await expect(dialog).toBeVisible();
    await dialog.getByLabel('DAG Name').fill(dagName);
    await dialog.getByRole('button', { name: 'Create' }).click();

    await expect(page).toHaveURL(new RegExp(`/dags/${dagName}\\.yaml/spec`));
  });

  test('renames a DAG from the UI', async ({ page, request }) => {
    const stack = await loadStack();
    const token = await loginViaAPI(
      request,
      stack.auth.adminUsername,
      stack.auth.adminPassword
    );

    const dagName = uniqueName('e2e-rename');
    const newName = uniqueName('e2e-renamed');
    const fileName = await writeLocalDAG(
      dagName,
      `
name: ${dagName}
steps:
  - name: echo
    command: echo "rename test"
`
    );
    await waitForDAGAvailable(request, token, fileName);

    await page.goto(`/dags/${encodeURIComponent(fileName)}/spec`);
    await expect(page.getByText(dagName)).toBeVisible();
    await page.getByRole('button', { name: 'Rename' }).click();

    const dialog = page.getByRole('dialog');
    await expect(dialog).toBeVisible();
    await dialog.getByLabel('DAG Name').clear();
    await dialog.getByLabel('DAG Name').fill(newName);
    await dialog.getByRole('button', { name: 'Rename' }).click();

    await expect(page).toHaveURL(new RegExp(`/dags/${newName}\\.yaml`));
  });

  test('deletes a DAG from the UI', async ({ page, request }) => {
    const stack = await loadStack();
    const token = await loginViaAPI(
      request,
      stack.auth.adminUsername,
      stack.auth.adminPassword
    );

    const dagName = uniqueName('e2e-delete');
    const fileName = await writeLocalDAG(
      dagName,
      `
name: ${dagName}
steps:
  - name: echo
    command: echo "delete test"
`
    );
    await waitForDAGAvailable(request, token, fileName);

    await page.goto(`/dags/${encodeURIComponent(fileName)}/spec`);
    await expect(page.getByText(dagName)).toBeVisible();

    page.once('dialog', (d) => d.accept());
    await page.getByRole('button', { name: 'Delete' }).click();

    await expect(page).not.toHaveURL(new RegExp(dagName));

    // Verify DAG is gone via API
    const response = await request.get(
      `/api/v1/dags/${encodeURIComponent(fileName)}?remoteNode=local`,
      {
        headers: {
          Authorization: `Bearer ${token}`,
          'Content-Type': 'application/json',
        },
      }
    );
    expect(response.ok()).toBeFalsy();
  });

  test('suspends and resumes a DAG schedule', async ({ page, request }) => {
    const stack = await loadStack();
    const token = await loginViaAPI(
      request,
      stack.auth.adminUsername,
      stack.auth.adminPassword
    );

    const dagName = uniqueName('e2e-suspend');
    const fileName = await writeLocalDAG(
      dagName,
      `
name: ${dagName}
schedule: "0 0 * * *"
steps:
  - name: echo
    command: echo "suspend test"
`
    );
    await waitForDAGAvailable(request, token, fileName);

    await page.goto(`/dags/${encodeURIComponent(fileName)}/`);
    await expect(page.getByText(dagName)).toBeVisible();

    // DAG starts active — click switch to disable schedule
    await page.getByRole('switch').click();
    const disableDialog = page.getByRole('dialog');
    await expect(disableDialog).toBeVisible();
    await expect(disableDialog).toContainText('Disable Schedule');
    await disableDialog.getByRole('button', { name: 'Disable' }).click();
    await expect(disableDialog).toBeHidden();

    // Verify suspended via API
    await expect
      .poll(
        async () => {
          const resp = await request.get(
            `/api/v1/dags/${encodeURIComponent(fileName)}?remoteNode=local`,
            {
              headers: {
                Authorization: `Bearer ${token}`,
                'Content-Type': 'application/json',
              },
            }
          );
          const body = (await resp.json()) as { suspended: boolean };
          return body.suspended;
        },
        { timeout: 5_000 }
      )
      .toBeTruthy();

    // Click switch again to re-enable schedule
    await page.getByRole('switch').click();
    const enableDialog = page.getByRole('dialog');
    await expect(enableDialog).toBeVisible();
    await expect(enableDialog).toContainText('Enable Schedule');
    await enableDialog.getByRole('button', { name: 'Enable' }).click();
    await expect(enableDialog).toBeHidden();

    // Verify resumed via API
    await expect
      .poll(
        async () => {
          const resp = await request.get(
            `/api/v1/dags/${encodeURIComponent(fileName)}?remoteNode=local`,
            {
              headers: {
                Authorization: `Bearer ${token}`,
                'Content-Type': 'application/json',
              },
            }
          );
          const body = (await resp.json()) as { suspended: boolean };
          return body.suspended;
        },
        { timeout: 5_000 }
      )
      .toBeFalsy();
  });

  test('rejects DAG creation with invalid name', async ({ page }) => {
    await page.goto('/dags/');
    await page.getByRole('button', { name: 'Create new DAG' }).click();

    const dialog = page.getByRole('dialog');
    await expect(dialog).toBeVisible();
    await dialog.getByLabel('DAG Name').fill('invalid name with spaces!');
    await dialog.getByRole('button', { name: 'Create' }).click();

    // Dialog stays open — validation prevents creation
    await expect(dialog).toBeVisible();
    await expect(page).not.toHaveURL(/\/spec$/);
  });
});
