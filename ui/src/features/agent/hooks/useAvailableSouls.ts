import { useState, useEffect, useContext } from 'react';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useClient } from '@/hooks/api';

interface SoulOption {
  id: string;
  name: string;
}

export function useAvailableSouls() {
  const client = useClient();
  const appBarContext = useContext(AppBarContext);
  const [souls, setSouls] = useState<SoulOption[]>([]);
  const [selectedSoul, setSelectedSoul] = useState<string>('__default__');

  useEffect(() => {
    const controller = new AbortController();
    const remoteNode = appBarContext.selectedRemoteNode || 'local';

    async function fetchSouls() {
      try {
        const { data } = await client.GET('/settings/agent/souls', {
          params: { query: { remoteNode } },
          signal: controller.signal,
        });
        if (!data) return;
        const soulList: SoulOption[] = (data.souls || []).map((s) => ({
          id: s.id,
          name: s.name,
        }));
        setSouls(soulList);
      } catch {
        // Souls fetch is best-effort
      }
    }
    fetchSouls();

    return () => controller.abort();
  }, [client, appBarContext.selectedRemoteNode]);

  return { souls, selectedSoul, setSelectedSoul };
}
