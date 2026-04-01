import { useCallback, useContext, useEffect, useMemo, useState } from 'react';
import { components } from '@/api/v1/schema';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useClient } from '@/hooks/api';

type AgentAuthProviderStatus = components['schemas']['AgentAuthProviderStatus'];
type StartAgentAuthProviderLoginResponse = components['schemas']['StartAgentAuthProviderLoginResponse'];
type CompleteAgentAuthProviderLoginRequest = components['schemas']['CompleteAgentAuthProviderLoginRequest'];

function upsertProvider(
  providers: AgentAuthProviderStatus[],
  provider: AgentAuthProviderStatus
): AgentAuthProviderStatus[] {
  const next = providers.filter((item) => item.id !== provider.id);
  next.push(provider);
  next.sort((a, b) => a.name.localeCompare(b.name));
  return next;
}

export function useAgentAuthProviders() {
  const client = useClient();
  const appBarContext = useContext(AppBarContext);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';

  const [providers, setProviders] = useState<AgentAuthProviderStatus[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refreshProviders = useCallback(async (): Promise<AgentAuthProviderStatus[]> => {
    setIsLoading(true);
    try {
      const { data, error: apiError } = await client.GET('/settings/agent/auth/providers', {
        params: { query: { remoteNode } },
      });
      if (apiError) {
        throw new Error(apiError.message || 'Failed to load provider connections');
      }
      const nextProviders = data?.providers || [];
      setProviders(nextProviders);
      setError(null);
      return nextProviders;
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to load provider connections';
      setError(message);
      setProviders([]);
      throw err;
    } finally {
      setIsLoading(false);
    }
  }, [client, remoteNode]);

  useEffect(() => {
    void refreshProviders().catch(() => {
      // Error state is handled in the hook.
    });
  }, [refreshProviders]);

  const startLogin = useCallback(async (providerId: string): Promise<StartAgentAuthProviderLoginResponse> => {
    const { data, error: apiError } = await client.POST('/settings/agent/auth/providers/{providerId}/login', {
      params: { path: { providerId }, query: { remoteNode } },
    });
    if (apiError) {
      throw new Error(apiError.message || 'Failed to start provider login');
    }
    if (!data) {
      throw new Error('Failed to start provider login');
    }
    return data;
  }, [client, remoteNode]);

  const completeLogin = useCallback(async (
    providerId: string,
    body: CompleteAgentAuthProviderLoginRequest
  ): Promise<AgentAuthProviderStatus | null> => {
    const { data, error: apiError } = await client.POST('/settings/agent/auth/providers/{providerId}/login/complete', {
      params: { path: { providerId }, query: { remoteNode } },
      body,
    });
    if (apiError) {
      throw new Error(apiError.message || 'Failed to complete provider login');
    }
    const provider = data?.provider ?? null;
    if (provider) {
      setProviders((current) => upsertProvider(current, provider));
    }
    return provider;
  }, [client, remoteNode]);

  const disconnect = useCallback(async (providerId: string): Promise<void> => {
    const { error: apiError } = await client.DELETE('/settings/agent/auth/providers/{providerId}/login', {
      params: { path: { providerId }, query: { remoteNode } },
    });
    if (apiError) {
      throw new Error(apiError.message || 'Failed to disconnect provider');
    }
    await refreshProviders();
  }, [client, refreshProviders, remoteNode]);

  const providerMap = useMemo<Record<string, AgentAuthProviderStatus>>(
    () => Object.fromEntries(providers.map((provider) => [provider.id, provider])) as Record<string, AgentAuthProviderStatus>,
    [providers]
  );

  return {
    providers,
    providerMap,
    remoteNode,
    isLoading,
    error,
    refreshProviders,
    startLogin,
    completeLogin,
    disconnect,
  };
}
