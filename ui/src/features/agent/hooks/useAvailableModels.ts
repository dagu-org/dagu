import { useState, useEffect, useContext } from 'react';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useClient } from '@/hooks/api';

interface ModelOption {
  id: string;
  name: string;
}

export function useAvailableModels() {
  const client = useClient();
  const appBarContext = useContext(AppBarContext);
  const [models, setModels] = useState<ModelOption[]>([]);
  const [selectedModel, setSelectedModel] = useState<string>('');

  useEffect(() => {
    const controller = new AbortController();
    const remoteNode = appBarContext.selectedRemoteNode || 'local';

    async function fetchModels() {
      try {
        const { data } = await client.GET('/settings/agent/models', {
          params: { query: { remoteNode } },
          signal: controller.signal,
        });
        if (!data) return;
        const modelList: ModelOption[] = (data.models || []).map((m) => ({
          id: m.id,
          name: m.name,
        }));
        setModels(modelList);
        if (data.defaultModelId) {
          setSelectedModel(data.defaultModelId);
        } else if (modelList.length > 0) {
          setSelectedModel(modelList[0]!.id);
        }
      } catch {
        // Models fetch is best-effort
      }
    }
    fetchModels();

    return () => controller.abort();
  }, [client, appBarContext.selectedRemoteNode]);

  return { models, selectedModel, setSelectedModel };
}
