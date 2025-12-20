/**
 * DAGSpec component displays and allows editing of a DAG specification.
 *
 * @module features/dags/components/dag-editor
 */
import BorderedBox from '@/ui/BorderedBox';
import { AlertTriangle, Save } from 'lucide-react';
import React, { useEffect } from 'react';
import { useCookies } from 'react-cookie';
import { components } from '../../../../api/v2/schema';
import { Button } from '../../../../components/ui/button';
import { useSimpleToast } from '../../../../components/ui/simple-toast';
import { Tab, Tabs } from '../../../../components/ui/tabs';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { useConfig } from '../../../../contexts/ConfigContext';
import { useClient, useQuery } from '../../../../hooks/api';
import LoadingIndicator from '../../../../ui/LoadingIndicator';
import { DAGContext } from '../../contexts/DAGContext';
import { DAGStepTable } from '../dag-details';
import { FlowchartSwitch, FlowchartType, Graph } from '../visualization';
import DAGAttributes from './DAGAttributes';
import DAGEditor from './DAGEditor';

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

  // Editability is derived from permissions; no explicit toggle
  const editable = !!config.permissions.writeDags;
  const [currentValue, setCurrentValue] = React.useState<string | undefined>();
  const [scrollPosition, setScrollPosition] = React.useState(0);
  const [activeTab, setActiveTab] = React.useState('parent');

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
      setCurrentValue((prev) => (typeof prev === 'undefined' ? fetchedSpec : prev));
      return;
    }

    lastFetchedSpecRef.current = fetchedSpec;
    setCurrentValue(fetchedSpec);
  }, [data?.spec]);

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
  }, [currentValue, fileName, appBarContext.selectedRemoteNode, client, saveScrollPosition, showToast]);

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
        <div className="bg-card rounded-2xl border border-border hover: overflow-hidden">
          <div className="border-b border-border bg-error-muted px-6 py-4">
            <h2 className="text-lg font-semibold text-error flex items-center gap-2">
              <AlertTriangle className="h-5 w-5" />
              Configuration Errors
            </h2>
          </div>
          <div className="p-6">
            <div className="space-y-3">
              {errors.map((e, i) => (
                <div
                  key={i}
                  className="p-3 bg-error-muted rounded-md text-error font-mono text-sm break-words"
                >
                  {e}
                </div>
              ))}
            </div>
          </div>
        </div>
      ) : null}

      <div className="bg-card rounded-2xl border border-border hover: overflow-hidden">
        <div className="border-b border-border bg-muted px-6 py-4 flex justify-between items-center">
          <h2 className="text-lg font-semibold text-foreground">
            Graph
          </h2>
          {!errors?.length && (
            <FlowchartSwitch
              value={cookie['flowchart']}
              onChange={onChangeFlowchart}
            />
          )}
        </div>
        <div className="p-6">
          {errors?.length || !dag.steps || dag.steps.length === 0 ? (
            <div className="py-8 px-4 text-center">
              <AlertTriangle className="h-12 w-12 text-warning mx-auto mb-4" />
              <p className="text-muted-foreground mb-2">
                Cannot render graph due to configuration errors
              </p>
              <p className="text-sm text-muted-foreground">
                Please fix the errors above and save the configuration to view the graph
              </p>
            </div>
          ) : (
            <BorderedBox className="py-4 px-4 flex flex-col overflow-x-auto">
              <Graph
                steps={dag.steps}
                type="config"
                flowchart={flowchart}
                showIcons={false}
              />
            </BorderedBox>
          )}
        </div>
      </div>

      <div className="bg-card rounded-2xl border border-border hover: overflow-hidden">
        <div className="border-b border-border bg-muted px-6 py-4">
          <h2 className="text-lg font-semibold text-foreground">
            Attributes
          </h2>
        </div>
        <div className="p-6">
          <DAGAttributes dag={dag} />
        </div>
      </div>

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

        return data?.dag && (
          <React.Fragment>
            <div className="space-y-6" ref={containerRef}>
              {hasLocalDags ? (
                <div className="space-y-6">
                  <div className="overflow-x-auto -mx-2 px-2 scrollbar-thin scrollbar-thumb-gray-300">
                    <Tabs className="mb-4 w-max min-w-full">
                      <Tab
                        isActive={activeTab === 'parent'}
                        onClick={() => setActiveTab('parent')}
                        className="cursor-pointer whitespace-nowrap"
                      >
                        {data?.dag?.name} (Parent)
                      </Tab>
                      {dagDetails?.localDags?.map((localDag: components['schemas']['LocalDag']) => (
                        <Tab
                          key={localDag.name}
                          isActive={activeTab === localDag.name}
                          onClick={() => setActiveTab(localDag.name)}
                          className="cursor-pointer whitespace-nowrap"
                        >
                          {localDag.name}
                        </Tab>
                      ))}
                    </Tabs>
                  </div>
                  
                  {activeTab === 'parent' && data?.dag && renderDAGContent(data.dag, data?.errors)}
                  
                  {dagDetails?.localDags?.map((localDag: components['schemas']['LocalDag']) => (
                    activeTab === localDag.name && (
                      <div key={localDag.name}>
                        {localDag.dag ? (
                          renderDAGContent(localDag.dag, localDag.errors)
                        ) : (
                          <div className="bg-card rounded-2xl border border-border p-6">
                            <div className="text-error">
                              <AlertTriangle className="h-5 w-5 inline mr-2" />
                              Failed to load local DAG: {localDag.name}
                            </div>
                            {localDag.errors?.length ? (
                              <div className="mt-4 space-y-2">
                                {localDag.errors.map((e: string, i: number) => (
                                  <div key={i} className="text-sm font-mono">{e}</div>
                                ))}
                              </div>
                            ) : null}
                          </div>
                        )}
                      </div>
                    )
                  ))}
                </div>
              ) : (
                data?.dag && renderDAGContent(data.dag, data?.errors)
              )}

              <div
                className={
                  'rounded-2xl border hover: overflow-hidden bg-card border-border'
                }
              >
                <div
                  className={
                    'border-b border-border px-6 py-4 flex justify-between items-center bg-muted'
                  }
                >
                  <div className="flex items-center">
                    <h2 className="text-lg font-semibold text-foreground mr-3">
                      Definition
                    </h2>
                  </div>

                  {editable ? (
                    <div className="flex gap-2">
                      <Button
                        id="save-config"
                        variant="default"
                        size="sm"
                        title="Save changes (Ctrl+S / Cmd+S)"
                        className="cursor-pointer hover: relative group"
                        onClick={async () => {
                          await handleSave();
                          props.refresh();
                        }}
                      >
                        <Save className="h-4 w-4 mr-1" />
                        Save Changes
                        <span className="absolute -bottom-1 -right-1 bg-primary-foreground text-primary text-[10px] font-medium px-1 rounded-sm opacity-0 group-hover:opacity-100 transition-opacity">
                          {navigator.platform.indexOf('Mac') > -1 ? '⌘S' : 'Ctrl+S'}
                        </span>
                      </Button>
                    </div>
                  ) : null}
                </div>

                <div className="p-6">
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
                  />
                </div>
              </div>
            </div>
          </React.Fragment>
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
