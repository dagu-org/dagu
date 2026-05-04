// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import Title from '@/components/ui/title';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useUpdateConfig } from '@/contexts/ConfigContext';
import { useClient } from '@/hooks/api';
import { Loader2 } from 'lucide-react';
import React from 'react';
import { Link } from 'react-router-dom';

type AgentLink = {
  to: string;
  label: string;
  description: string;
};

type AgentSection = {
  title: string;
  links: AgentLink[];
};

const agentSections: AgentSection[] = [
  {
    title: 'Configuration',
    links: [
      {
        to: '/agent-settings',
        label: 'Models',
        description: 'Configure model access.',
      },
      {
        to: '/agent-tools',
        label: 'Tools',
        description: 'Configure web search and tool policy.',
      },
    ],
  },
  {
    title: 'Context',
    links: [
      {
        to: '/agent-memory',
        label: 'Memory',
        description: 'Manage persistent context.',
      },
      {
        to: '/agent-souls',
        label: 'Souls',
        description: 'Manage reusable personas.',
      },
    ],
  },
];

function SectionLinks({
  section,
}: {
  section: AgentSection;
}): React.ReactElement {
  return (
    <section className="space-y-2">
      <h3 className="text-xs font-semibold uppercase text-muted-foreground">
        {section.title}
      </h3>
      <div className="grid gap-2 md:grid-cols-2 xl:grid-cols-3">
        {section.links.map((link) => (
          <Link
            key={link.to}
            to={link.to}
            className="rounded-md border border-border bg-card px-4 py-3 transition-colors hover:border-border-strong hover:bg-muted focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
          >
            <span className="block text-sm font-medium text-foreground">
              {link.label}
            </span>
            <span className="mt-1 block text-xs text-muted-foreground">
              {link.description}
            </span>
          </Link>
        ))}
      </div>
    </section>
  );
}

export default function AgentPage(): React.ReactElement {
  const client = useClient();
  const updateConfig = useUpdateConfig();
  const { selectedRemoteNode, setTitle } = React.useContext(AppBarContext);
  const remoteNode = selectedRemoteNode || 'local';
  const [enabled, setEnabled] = React.useState(false);
  const [isLoading, setIsLoading] = React.useState(true);
  const [isSaving, setIsSaving] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [success, setSuccess] = React.useState<string | null>(null);

  React.useEffect(() => {
    setTitle('Agent');
  }, [setTitle]);

  React.useEffect(() => {
    let active = true;

    async function loadAgentStatus(): Promise<void> {
      setIsLoading(true);
      setError(null);
      try {
        const { data, error: apiError } = await client.GET('/settings/agent', {
          params: { query: { remoteNode } },
        });
        if (apiError) {
          throw new Error(apiError.message || 'Failed to load agent status');
        }
        if (active) {
          setEnabled(data.enabled ?? false);
        }
      } catch (err) {
        if (active) {
          setError(
            err instanceof Error ? err.message : 'Failed to load agent status'
          );
        }
      } finally {
        if (active) {
          setIsLoading(false);
        }
      }
    }

    void loadAgentStatus();

    return () => {
      active = false;
    };
  }, [client, remoteNode]);

  async function handleEnabledChange(nextEnabled: boolean): Promise<void> {
    const previousEnabled = enabled;
    setEnabled(nextEnabled);
    setIsSaving(true);
    setError(null);
    setSuccess(null);

    try {
      const { data, error: apiError } = await client.PATCH('/settings/agent', {
        params: { query: { remoteNode } },
        body: { enabled: nextEnabled },
      });

      if (apiError) {
        throw new Error(apiError.message || 'Failed to save agent setting');
      }

      const savedEnabled = data.enabled ?? nextEnabled;
      setEnabled(savedEnabled);
      updateConfig({ agentEnabled: savedEnabled });
      setSuccess('Agent setting saved successfully');
    } catch (err) {
      setEnabled(previousEnabled);
      setError(
        err instanceof Error ? err.message : 'Failed to save agent setting'
      );
    } finally {
      setIsSaving(false);
    }
  }

  return (
    <div className="flex h-full min-h-0 flex-col gap-5 overflow-auto">
      <Title>Agent</Title>

      <section className="space-y-2">
        <h3 className="text-xs font-semibold uppercase text-muted-foreground">
          Status
        </h3>
        <div className="grid gap-2 md:grid-cols-2 xl:grid-cols-3">
          <div className="rounded-md border border-border bg-card px-4 py-3">
            <div className="flex items-center justify-between gap-4">
              <div className="space-y-0.5">
                <Label htmlFor="agent-enabled" className="text-sm font-medium">
                  Enable Agent
                </Label>
                <p className="text-xs text-muted-foreground">
                  Turn on the AI assistant feature.
                </p>
              </div>
              {isLoading ? (
                <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
              ) : (
                <Switch
                  id="agent-enabled"
                  checked={enabled}
                  disabled={isSaving}
                  onCheckedChange={handleEnabledChange}
                />
              )}
            </div>
            {error && <p className="mt-2 text-xs text-destructive">{error}</p>}
            {success && (
              <p className="mt-2 text-xs text-green-600">{success}</p>
            )}
          </div>
        </div>
      </section>

      {agentSections.map((section) => (
        <SectionLinks key={section.title} section={section} />
      ))}
    </div>
  );
}
