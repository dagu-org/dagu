import { Save } from 'lucide-react';
import React, { useCallback, useEffect, useRef } from 'react';
import { Button } from '../../components/ui/button';
import { useErrorModal } from '../../components/ui/error-modal';
import { useSimpleToast } from '../../components/ui/simple-toast';
import { AppBarContext } from '../../contexts/AppBarContext';
import { useConfig } from '../../contexts/ConfigContext';
import DAGEditorWithDocs from '../../features/dags/components/dag-editor/DAGEditorWithDocs';
import { useClient, useQuery } from '../../hooks/api';

function BaseConfigPage(): React.ReactNode {
  const appBarContext = React.useContext(AppBarContext);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const client = useClient();
  const config = useConfig();
  const { showError } = useErrorModal();
  const { showToast } = useSimpleToast();

  const editable = !!config.permissions.writeDags;
  const [currentValue, setCurrentValue] = React.useState<string | null>(null);
  const [hasUnsavedChanges, setHasUnsavedChanges] = React.useState(false);
  const saveHandlerRef = useRef<(() => Promise<void>) | null>(null);

  useEffect(() => {
    appBarContext.setTitle('Base Config');
  }, [appBarContext]);

  const { data, mutate } = useQuery(
    '/settings/base-config',
    {
      params: {
        query: { remoteNode },
      },
    },
    {
      revalidateOnFocus: true,
      revalidateOnMount: true,
    }
  );

  // Initialize editor value from fetched data
  useEffect(() => {
    if (data?.spec !== undefined && currentValue === null) {
      setCurrentValue(data.spec);
    }
  }, [data, currentValue]);

  const handleChange = useCallback((newValue?: string) => {
    setCurrentValue(newValue || '');
    setHasUnsavedChanges(true);
  }, []);

  const handleSave = useCallback(async () => {
    if (currentValue === null) {
      return;
    }

    const { data: responseData, error } = await client.PUT(
      '/settings/base-config',
      {
        params: {
          query: { remoteNode },
        },
        body: {
          spec: currentValue,
        },
      }
    );

    if (error) {
      showError(
        error.message || 'Failed to save base configuration',
        'Please check the YAML syntax and try again.'
      );
      return;
    }

    if (responseData?.errors && responseData.errors.length > 0) {
      showError('Validation errors', responseData.errors.join('\n'));
      return;
    }

    setHasUnsavedChanges(false);
    mutate();
    showToast('Base configuration saved successfully');
  }, [currentValue, remoteNode, client, showError, showToast, mutate]);

  useEffect(() => {
    saveHandlerRef.current = handleSave;
  }, [handleSave]);

  // Ctrl+S / Cmd+S keyboard shortcut
  useEffect(() => {
    if (!editable) {
      return;
    }

    const handleKeyDown = async (event: KeyboardEvent) => {
      if ((event.ctrlKey || event.metaKey) && event.key === 's') {
        event.preventDefault();
        if (saveHandlerRef.current) {
          await saveHandlerRef.current();
        }
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [editable]);

  return (
    <div className="flex flex-col flex-1 min-h-0 space-y-4 p-4">
      <div>
        <h1 className="text-lg font-semibold">Base Configuration</h1>
        <p className="text-sm text-muted-foreground">
          Global defaults inherited by all DAG definitions
        </p>
      </div>

      <DAGEditorWithDocs
        value={
          editable ? currentValue ?? data?.spec ?? '' : data?.spec ?? ''
        }
        readOnly={!editable}
        onChange={editable ? handleChange : undefined}
        className="min-h-[400px]"
        headerActions={
          editable ? (
            <Button
              title="Save changes (Ctrl+S / Cmd+S)"
              disabled={!hasUnsavedChanges}
              onClick={handleSave}
            >
              <Save className="h-4 w-4" />
              Save
            </Button>
          ) : undefined
        }
      />
    </div>
  );
}

export default BaseConfigPage;
