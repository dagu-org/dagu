/**
 * DAGSpec component displays and allows editing of a DAG specification.
 *
 * @module features/dags/components/dag-editor
 */
import BorderedBox from '@/ui/BorderedBox';
import { AlertTriangle, Save, BookOpen } from 'lucide-react';
import React, { useEffect, useCallback } from 'react';
import { useCookies } from 'react-cookie';
import { components } from '../../../../api/v2/schema';
import { Button } from '../../../../components/ui/button';
import { useSimpleToast } from '../../../../components/ui/simple-toast';
import { Tab, Tabs } from '../../../../components/ui/tabs';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { useConfig } from '../../../../contexts/ConfigContext';
import { useUnsavedChanges } from '../../../../contexts/UnsavedChangesContext';
import { useClient, useQuery } from '../../../../hooks/api';
import LoadingIndicator from '../../../../ui/LoadingIndicator';
import { DAGContext } from '../../contexts/DAGContext';
import { DAGStepTable } from '../dag-details';
import { FlowchartType, Graph } from '../visualization';
import DAGAttributes from './DAGAttributes';
import DAGEditor, { type CursorPosition } from './DAGEditor';
import { SchemaDocSidebar } from './SchemaDocSidebar';
import { useDebouncedValue } from '../../../../hooks/useDebouncedValue';
import { useYamlCursorPath, type YamlPathSegment } from '../../../../hooks/useYamlCursorPath';

/**
 * Props for the DAGSpec component
 */
type Props = {
  /** DAG file name */
  fileName: string;
};

/**
 * DAGSpec displays and allows editing of a DAG specification
 * including visualization, attributes, steps, and YAML definition
 */
function DAGSpec({ fileName }: Props) {
  const appBarContext = React.useContext(AppBarContext);
  const client = useClient();
  const config = useConfig();
  const { showToast } = useSimpleToast();
  const { hasUnsavedChanges, setHasUnsavedChanges } = useUnsavedChanges();

  // Editability is derived from permissions; no explicit toggle
  const editable = !!config.permissions.writeDags;
  const [currentValue, setCurrentValue] = React.useState<string | undefined>();
  const [scrollPosition, setScrollPosition] = React.useState(0);
  const [activeTab, setActiveTab] = React.useState('parent');

  // Schema documentation sidebar state
  const [sidebarOpen, setSidebarOpen] = React.useState(() => {
    try {
      return localStorage.getItem('schema-sidebar-open') === 'true';
    } catch {
      return false;
    }
  });
  const [cursorPosition, setCursorPosition] = React.useState<CursorPosition>({
    lineNumber: 1,
    column: 1,
  });

  // Debounce cursor position to avoid too many re-renders
  const debouncedCursorPosition = useDebouncedValue(cursorPosition, 150);

  // Get YAML path from cursor position - uses currentValue which is initialized from data.spec
  const yamlPathInfo = useYamlCursorPath(
    currentValue ?? '',
    debouncedCursorPosition.lineNumber,
    debouncedCursorPosition.column
  );

  // Flowchart direction preference stored in cookies
  const [cookie, setCookie] = useCookies(['flowchart']);
  const [flowchart, setFlowchart] = React.useState(cookie['flowchart']);

  // Reference to the main container div
  const containerRef = React.useRef<HTMLDivElement>(null);
  const lastFetchedSpecRef = React.useRef<string | undefined>(undefined);

  // Reference to save function and refresh callback for keyboard shortcut
  const saveHandlerRef = React.useRef<(() => Promise<void>) | null>(null);
  const refreshCallbackRef = React.useRef<(() => void) | null>(null);

  /**
   * Handle flowchart direction change and save preference to cookie
   */
  const onChangeFlowchart = React.useCallback(
    (value: FlowchartType) => {
      if (!value) {
        return;
      }
      setCookie('flowchart', value, { path: '/' });
      setFlowchart(value);
    },
    [setCookie, flowchart, setFlowchart]
  );

  // Fetch DAG specification data
  const { data, isLoading } = useQuery(
    '/dags/{fileName}/spec',
    {
      params: {
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
        },
        path: {
          fileName: fileName,
        },
      },
    },
    { refreshInterval: 2000 } // Refresh every 2 seconds
  );

  // Fetch DAG details to get localDags
  const { data: dagDetails } = useQuery(
    '/dags/{fileName}',
    {
      params: {
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
        },
        path: {
          fileName: fileName,
        },
      },
    },
    { refreshInterval: 2000 } // Refresh every 2 seconds
  );

  // Update current value when data changes
  useEffect(() => {
    if (typeof data?.spec === 'undefined') {
      return;
    }

    const fetchedSpec = data.spec;

    if (lastFetchedSpecRef.current === fetchedSpec) {
      // Ensure the editor initializes with the fetched value on first load.
      setCurrentValue((prev) =>
        typeof prev === 'undefined' ? fetchedSpec : prev
      );
      return;
    }

    lastFetchedSpecRef.current = fetchedSpec;
    setCurrentValue(fetchedSpec);
  }, [data?.spec]);

  // Track unsaved changes
  useEffect(() => {
    if (
      typeof currentValue === 'undefined' ||
      typeof data?.spec === 'undefined'
    ) {
      setHasUnsavedChanges(false);
      return;
    }
    const hasChanges = currentValue !== data.spec;
    setHasUnsavedChanges(hasChanges);
  }, [currentValue, data?.spec, setHasUnsavedChanges]);

  // Clean up unsaved changes state on unmount
  useEffect(() => {
    return () => {
      setHasUnsavedChanges(false);
    };
  }, [setHasUnsavedChanges]);

  // Save scroll position before saving
  const saveScrollPosition = React.useCallback(() => {
    if (containerRef.current) {
      setScrollPosition(window.scrollY);
    }
  }, []);

  // Save handler function
  const handleSave = React.useCallback(async () => {
    if (!currentValue) {
      alert('No changes to save');
      return;
    }

    // Save current scroll position before any operations that might cause re-render
    saveScrollPosition();

    const { data: responseData, error } = await client.PUT(
      '/dags/{fileName}/spec',
      {
        params: {
          path: {
            fileName: fileName,
          },
          query: {
            remoteNode: appBarContext.selectedRemoteNode || 'local',
          },
        },
        body: {
          spec: currentValue,
        },
      }
    );

    if (error) {
      alert(error.message || 'Failed to save spec');
      return;
    }

    if (responseData?.errors) {
      alert(responseData.errors.join('\n'));
      return;
    }

    // Show success toast notification
    showToast('Changes saved successfully');
  }, [
    currentValue,
    fileName,
    appBarContext.selectedRemoteNode,
    client,
    saveScrollPosition,
    showToast,
  ]);

  // Restore scroll position after render
  useEffect(() => {
    if (scrollPosition > 0) {
      // Use a small timeout to ensure the DOM has updated before scrolling
      const timer = setTimeout(() => {
        window.scrollTo({
          top: scrollPosition,
          behavior: 'auto', // Use 'auto' instead of 'smooth' to avoid animation
        });
      }, 100);

      return () => clearTimeout(timer);
    }
  }, [scrollPosition]);

  // Update save handler ref when handleSave changes
  useEffect(() => {
    saveHandlerRef.current = handleSave;
  }, [handleSave]);

  // Toggle sidebar and persist state
  const toggleSidebar = useCallback(() => {
    setSidebarOpen((prev) => {
      const newValue = !prev;
      try {
        localStorage.setItem('schema-sidebar-open', String(newValue));
      } catch {
        // Ignore localStorage errors
      }
      return newValue;
    });
  }, []);

  // Handle cursor position change from editor
  const handleCursorPositionChange = useCallback((position: CursorPosition) => {
    setCursorPosition(position);
  }, []);

  // Add keyboard shortcut for sidebar toggle (Ctrl+Shift+D)
  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if ((event.ctrlKey || event.metaKey) && event.shiftKey && event.key === 'd') {
        event.preventDefault();
        toggleSidebar();
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [toggleSidebar]);

  // Add keyboard shortcut for saving (Ctrl+S / Cmd+S)
  useEffect(() => {
    if (!editable) {
      return;
    }

    const handleKeyDown = async (event: KeyboardEvent) => {
      // Check for Ctrl+S (Windows/Linux) or Cmd+S (macOS)
      if ((event.ctrlKey || event.metaKey) && event.key === 's') {
        event.preventDefault(); // Prevent browser's default save dialog

        // Call the save handler if available
        if (saveHandlerRef.current) {
          await saveHandlerRef.current();

          // Refresh after saving
          if (refreshCallbackRef.current) {
            refreshCallbackRef.current();
          }
        }
      }
    };

    // Add event listener to document
    document.addEventListener('keydown', handleKeyDown);

    // Cleanup on unmount
    return () => {
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [editable]);

  // Show loading indicator while fetching data
  if (isLoading) {
    return <LoadingIndicator />;
  }

  // Check if we have local DAGs
  const hasLocalDags = dagDetails?.localDags && dagDetails.localDags.length > 0;

  // Helper function to render DAG content (Graph, Attributes, Steps, Errors)
  const renderDAGContent = (
    dag: components['schemas']['DAGDetails'],
    errors?: string[]
  ) => (
    <div className="space-y-6">
      {errors?.length ? (
        <div className="space-y-3">
          {errors.map((e, i) => (
            <div
              key={i}
              className="p-3 bg-danger-highlight rounded-md text-danger font-mono text-sm break-words flex items-start gap-2"
            >
              <AlertTriangle className="h-4 w-4 mt-0.5 flex-shrink-0" />
              {e}
            </div>
          ))}
        </div>
      ) : null}

      {errors?.length || !dag.steps || dag.steps.length === 0 ? (
        <div className="py-8 px-4 text-center">
          <AlertTriangle className="h-12 w-12 text-warning mx-auto mb-4" />
          <p className="text-muted-foreground mb-2">
            Cannot render graph due to configuration errors
          </p>
          <p className="text-sm text-muted-foreground">
            Please fix the errors above and save the configuration to view the
            graph
          </p>
        </div>
      ) : (
        <div>
          <BorderedBox className="py-4 px-4 flex flex-col overflow-x-auto">
            <Graph
              steps={dag.steps}
              type="config"
              flowchart={flowchart}
              onChangeFlowchart={onChangeFlowchart}
              showIcons={false}
            />
          </BorderedBox>
        </div>
      )}

      <DAGAttributes dag={dag} />

      {dag.steps ? (
        <div className="overflow-hidden">
          <DAGStepTable steps={dag.steps} />
        </div>
      ) : null}

      {getHandlers(dag)?.length ? (
        <div className="overflow-hidden">
          <DAGStepTable steps={getHandlers(dag)} />
        </div>
      ) : null}
    </div>
  );

  return (
    <DAGContext.Consumer>
      {(props) => {
        // Update refresh callback ref directly (safe in render)
        refreshCallbackRef.current = props.refresh;

        return (
          data?.dag && (
            <React.Fragment>
              <div
                className="flex flex-col flex-1 h-full min-h-[500px] space-y-6 mb-6"
                ref={containerRef}
              >
                {hasLocalDags && (
                  <div className="flex-shrink-0">
                    <div className="overflow-x-auto -mx-2 px-2 scrollbar-thin scrollbar-thumb-gray-300">
                      <Tabs className="w-max min-w-full">
                        <Tab
                          isActive={activeTab === 'parent'}
                          onClick={() => setActiveTab('parent')}
                          className="cursor-pointer whitespace-nowrap"
                        >
                          {data?.dag?.name} (Parent)
                        </Tab>
                        {dagDetails?.localDags?.map(
                          (localDag: components['schemas']['LocalDag']) => (
                            <Tab
                              key={localDag.name}
                              isActive={activeTab === localDag.name}
                              onClick={() => setActiveTab(localDag.name)}
                              className="cursor-pointer whitespace-nowrap"
                            >
                              {localDag.name}
                            </Tab>
                          )
                        )}
                      </Tabs>
                    </div>
                  </div>
                )}

                {(() => {
                  if (activeTab === 'parent') {
                    return (
                      data?.dag && (
                        <div className="flex-shrink-0">
                          {renderDAGContent(data.dag, data?.errors)}
                        </div>
                      )
                    );
                  }
                  const selectedLocalDag = dagDetails?.localDags?.find(
                    (ld: components['schemas']['LocalDag']) =>
                      ld.name === activeTab
                  );
                  return (
                    selectedLocalDag?.dag && (
                      <div className="flex-shrink-0">
                        {renderDAGContent(
                          selectedLocalDag.dag,
                          selectedLocalDag.errors
                        )}
                      </div>
                    )
                  );
                })()}

                <div className="flex-1 flex flex-col bg-surface border border-border rounded-lg overflow-hidden min-h-[400px]">
                  <div className="flex-shrink-0 flex justify-between items-center p-2 border-b border-border">
                    <Button
                      variant="secondary"
                      size="xs"
                      onClick={toggleSidebar}
                      title="Toggle Schema Documentation (Ctrl+Shift+D)"
                    >
                      <BookOpen className="h-3.5 w-3.5" />
                      Docs
                    </Button>
                    {editable && (
                      <Button
                        id="save-config"
                        title="Save changes (Ctrl+S / Cmd+S)"
                        disabled={!hasUnsavedChanges}
                        onClick={async () => {
                          await handleSave();
                          props.refresh();
                        }}
                      >
                        <Save className="h-4 w-4" />
                        Save
                      </Button>
                    )}
                  </div>
                  <div className="flex-1 flex min-h-0">
                    <div className="flex-1 min-w-0">
                      <DAGEditor
                        value={editable ? (currentValue ?? data.spec) : data.spec}
                        readOnly={!editable}
                        lineNumbers={true}
                        onChange={
                          editable
                            ? (newValue) => {
                                setCurrentValue(newValue || '');
                              }
                            : undefined
                        }
                        onCursorPositionChange={handleCursorPositionChange}
                      />
                    </div>
                    <SchemaDocSidebar
                      isOpen={sidebarOpen}
                      onClose={toggleSidebar}
                      path={yamlPathInfo.path}
                      segments={yamlPathInfo.segments}
                    />
                  </div>
                </div>
              </div>
            </React.Fragment>
          )
        );
      }}
    </DAGContext.Consumer>
  );
}

/**
 * Extract lifecycle handlers from DAG definition
 */
function getHandlers(
  dag?: components['schemas']['DAGDetails']
): components['schemas']['Step'][] {
  const steps: components['schemas']['Step'][] = [];
  if (!dag) {
    return steps;
  }
  const h = dag.handlerOn;
  if (h?.success) {
    steps.push(h.success);
  }
  if (h?.failure) {
    steps.push(h?.failure);
  }
  if (h?.cancel) {
    steps.push(h?.cancel);
  }
  if (h?.exit) {
    steps.push(h?.exit);
  }
  return steps;
}

export default DAGSpec;
