import React, { useCallback, useContext, useMemo, useState } from 'react';
import { Plus } from 'lucide-react';
import { useQuery, useClient } from '@/hooks/api';
import { AppBarContext } from '@/contexts/AppBarContext';
import { parseParams, stringifyParams, type Parameter } from '@/lib/parseParams';
import { Button } from '@/components/ui/button';

interface Props {
  selectedTemplate: string;
  selectedWorkspace: string;
}

function injectTagIntoSpec(yamlSpec: string, tag: string): string {
  // Match top-level "tags:" in various YAML forms:
  //   tags: "a,b"        → string form
  //   tags: a,b          → unquoted string form
  //   tags:\n  - a       → list form
  //   tags:              → empty
  const tagsRegex = /^tags:\s*(.*)$/m;
  const match = yamlSpec.match(tagsRegex);

  if (match) {
    const existingValue = (match[1] ?? '').trim();
    if (existingValue === '' || existingValue.startsWith('-')) {
      // Empty or block list form — append list item after the "tags:" line
      const idx = yamlSpec.indexOf(match[0]) + match[0].length;
      return yamlSpec.slice(0, idx) + `\n  - ${tag}` + yamlSpec.slice(idx);
    }
    // Inline string form (e.g. "a,b" or a,b) — append with comma
    const stripped = existingValue.replace(/^["']|["']$/g, '');
    const newValue = stripped ? `${stripped},${tag}` : tag;
    return yamlSpec.replace(tagsRegex, `tags: "${newValue}"`);
  }

  // No tags field — append one
  return yamlSpec.trimEnd() + `\ntags:\n  - ${tag}\n`;
}

export function QuickAddBar({ selectedTemplate, selectedWorkspace }: Props): React.ReactElement | null {
  const appBarContext = useContext(AppBarContext);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const client = useClient();
  const [isSubmitting, setIsSubmitting] = useState(false);

  const { data: specData } = useQuery(
    '/dags/{fileName}/spec',
    {
      params: {
        path: { fileName: selectedTemplate },
        query: { remoteNode },
      },
    },
    { isPaused: () => !selectedTemplate }
  );

  const defaultParams = useMemo(() => {
    if (!specData?.dag?.defaultParams) return [];
    return parseParams(specData.dag.defaultParams);
  }, [specData?.dag?.defaultParams]);

  const [paramValues, setParamValues] = useState<Record<string, string>>({});

  const handleParamChange = useCallback((name: string, value: string) => {
    setParamValues((prev) => ({ ...prev, [name]: value }));
  }, []);

  const handleAdd = useCallback(async () => {
    if (!specData?.spec || !selectedWorkspace) return;

    setIsSubmitting(true);
    try {
      const spec = injectTagIntoSpec(specData.spec, `workspace=${selectedWorkspace}`);
      const params: Parameter[] = defaultParams.map((p) => ({
        Name: p.Name,
        Value: paramValues[p.Name || ''] ?? p.Value,
      }));

      const paramsStr = params.length > 0 ? stringifyParams(params) : undefined;

      const { error } = await client.POST('/dag-runs/enqueue', {
        params: { query: { remoteNode } },
        body: {
          spec,
          params: paramsStr,
          name: specData.dag?.name,
        },
      });

      if (error) {
        console.error('Failed to enqueue:', error);
        return;
      }

      // Reset param values after successful enqueue
      setParamValues({});
    } finally {
      setIsSubmitting(false);
    }
  }, [specData, selectedWorkspace, defaultParams, paramValues, client, remoteNode]);

  if (!selectedTemplate) return null;

  return (
    <div className="flex items-center gap-2 flex-wrap">
      {defaultParams.map((param) => {
        const key = param.Name || '';
        return (
          <div key={key} className="flex items-center gap-1">
            {param.Name && (
              <label className="text-[11px] text-muted-foreground font-mono">{param.Name}:</label>
            )}
            <input
              className="h-7 px-2 text-xs rounded-md border border-border bg-background w-48"
              placeholder={param.Value || param.Name || 'value'}
              value={paramValues[key] ?? ''}
              onChange={(e) => handleParamChange(key, e.target.value)}
            />
          </div>
        );
      })}
      <Button
        size="sm"
        className="h-7 text-xs gap-1"
        onClick={handleAdd}
        disabled={isSubmitting || !selectedWorkspace || !specData?.spec}
      >
        <Plus size={14} />
        Add
      </Button>
    </div>
  );
}
