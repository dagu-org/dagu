import { useClient } from '@/hooks/api';
import { buildWorkspaceTag } from '@/lib/workspaceTags';

type APIClient = ReturnType<typeof useClient>;

type SubmitDAGExecutionArgs = {
  client: APIClient;
  fileName: string;
  remoteNode: string;
  selectedWorkspace: string;
  params?: string;
  dagRunId?: string;
  immediate?: boolean;
};

export async function submitDAGExecution({
  client,
  fileName,
  remoteNode,
  selectedWorkspace,
  params,
  dagRunId,
  immediate = false,
}: SubmitDAGExecutionArgs): Promise<string | void> {
  const workspaceTag = buildWorkspaceTag(selectedWorkspace);
  const body = {
    params: params || undefined,
    dagRunId: dagRunId || undefined,
    tags: workspaceTag ? [workspaceTag] : undefined,
  };

  const { data, error } = await (immediate
    ? client.POST('/dags/{fileName}/start', {
        params: {
          path: { fileName },
          query: { remoteNode },
        },
        body,
      })
    : client.POST('/dags/{fileName}/enqueue', {
        params: {
          path: { fileName },
          query: { remoteNode },
        },
        body,
      }));

  if (error) {
    throw new Error(error.message || 'Failed to start DAG execution.');
  }

  return data?.dagRunId;
}
