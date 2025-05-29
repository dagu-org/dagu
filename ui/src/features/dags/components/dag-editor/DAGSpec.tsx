/**
 * DAGSpec component displays and allows editing of a DAG specification.
 *
 * @module features/dags/components/dag-editor
 */
import BorderedBox from '@/ui/BorderedBox';
import { AlertTriangle, Code, Edit, Eye, Save, X } from 'lucide-react';
import React, { useEffect } from 'react';
import { useCookies } from 'react-cookie';
import { components } from '../../../../api/v2/schema';
import { Button } from '../../../../components/ui/button';
import { useSimpleToast } from '../../../../components/ui/simple-toast';
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
  /** DAG file ID */
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

  // State for editing mode and current YAML value
  const [editing, setEditing] = React.useState(false);
  const [currentValue, setCurrentValue] = React.useState<string | undefined>();
  const [scrollPosition, setScrollPosition] = React.useState(0);

  // Flowchart direction preference stored in cookies
  const [cookie, setCookie] = useCookies(['flowchart']);
  const [flowchart, setFlowchart] = React.useState(cookie['flowchart']);

  // Reference to the main container div
  const containerRef = React.useRef<HTMLDivElement>(null);

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

  // Update current value when data changes
  useEffect(() => {
    if (data) {
      setCurrentValue(data.spec);
    }
  }, [data]);

  // Save scroll position before saving
  const saveScrollPosition = React.useCallback(() => {
    if (containerRef.current) {
      setScrollPosition(window.scrollY);
    }
  }, []);

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
  }, [scrollPosition, editing]);

  // Get lifecycle handlers from DAG definition
  const handlers = getHandlers(data?.dag);

  // Show loading indicator while fetching data
  if (isLoading) {
    return <LoadingIndicator />;
  }

  return (
    <DAGContext.Consumer>
      {(props) =>
        data?.dag && (
          <React.Fragment>
            <div className="space-y-6" ref={containerRef}>
              <div className="bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm hover:shadow-md transition-shadow duration-200 overflow-hidden">
                <div className="border-b border-slate-100 dark:border-slate-800 bg-slate-50 dark:bg-slate-800/50 px-6 py-4 flex justify-between items-center">
                  <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">
                    Graph
                  </h2>
                  <FlowchartSwitch
                    value={cookie['flowchart']}
                    onChange={onChangeFlowchart}
                  />
                </div>
                <div className="p-6">
                  <BorderedBox className="py-4 px-4 flex flex-col overflow-x-auto">
                    <Graph
                      steps={data.dag.steps}
                      type="config"
                      flowchart={flowchart}
                      showIcons={false}
                    />
                  </BorderedBox>
                </div>
              </div>

              <div className="bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm hover:shadow-md transition-shadow duration-200 overflow-hidden">
                <div className="border-b border-slate-100 dark:border-slate-800 bg-slate-50 dark:bg-slate-800/50 px-6 py-4">
                  <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">
                    Attributes
                  </h2>
                </div>
                <div className="p-6">
                  <DAGAttributes dag={data.dag!} />
                </div>
              </div>

              {data.errors?.length ? (
                <div className="bg-white dark:bg-slate-900 rounded-2xl border border-l-4 border-red-500 dark:border-red-700 shadow-sm hover:shadow-md transition-shadow duration-200 overflow-hidden">
                  <div className="border-b border-slate-100 dark:border-slate-800 bg-red-50 dark:bg-red-900/10 px-6 py-4">
                    <h2 className="text-lg font-semibold text-red-600 dark:text-red-400 flex items-center gap-2">
                      <AlertTriangle className="h-5 w-5" />
                      Configuration Errors
                    </h2>
                  </div>
                  <div className="p-6">
                    <div className="space-y-3">
                      {data.errors?.map((e, i) => (
                        <div
                          key={i}
                          className="p-3 bg-red-50 dark:bg-red-900/20 rounded-md text-red-600 dark:text-red-400 font-mono text-sm"
                        >
                          {e}
                        </div>
                      ))}
                    </div>
                  </div>
                </div>
              ) : null}

              {data.dag.steps ? (
                <div className="bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm hover:shadow-md transition-shadow duration-200 overflow-hidden">
                  <div className="border-b border-slate-100 dark:border-slate-800 bg-slate-50 dark:bg-slate-800/50 px-6 py-4">
                    <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100 flex items-center justify-between">
                      <span>Steps</span>
                      <span className="text-sm font-normal text-slate-500 dark:text-slate-400">
                        {data.dag.steps.length} step
                        {data.dag.steps.length !== 1 ? 's' : ''}
                      </span>
                    </h2>
                  </div>
                  <div className="overflow-x-auto">
                    <DAGStepTable steps={data.dag.steps} />
                  </div>
                </div>
              ) : null}

              {handlers?.length ? (
                <div className="bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm hover:shadow-md transition-shadow duration-200 overflow-hidden">
                  <div className="border-b border-slate-100 dark:border-slate-800 bg-slate-50 dark:bg-slate-800/50 px-6 py-4">
                    <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100 flex items-center justify-between">
                      <span>Lifecycle Hooks</span>
                      <span className="text-sm font-normal text-slate-500 dark:text-slate-400">
                        {handlers.length} hook{handlers.length !== 1 ? 's' : ''}
                      </span>
                    </h2>
                  </div>
                  <div className="overflow-x-auto">
                    <DAGStepTable steps={handlers} />
                  </div>
                </div>
              ) : null}

              <div
                className={`rounded-2xl border shadow-sm hover:shadow-md transition-shadow duration-200 overflow-hidden ${
                  editing
                    ? 'bg-white dark:bg-slate-900 border-2 border-blue-400 dark:border-blue-600'
                    : 'bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-700'
                }`}
              >
                <div
                  className={`border-b border-slate-100 dark:border-slate-800 px-6 py-4 flex justify-between items-center ${
                    editing
                      ? 'bg-blue-50 dark:bg-blue-900/10'
                      : 'bg-slate-50 dark:bg-slate-800/50'
                  }`}
                >
                  <div className="flex items-center">
                    <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100 mr-3">
                      Definition
                    </h2>
                    <div
                      className={`text-xs font-medium px-2 py-1 rounded-full transition-all duration-300 ${
                        editing
                          ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/50 dark:text-blue-300'
                          : 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300'
                      }`}
                    >
                      {editing ? (
                        <div className="flex items-center">
                          <Code className="h-3 w-3 mr-1" />
                          <span>Editing</span>
                        </div>
                      ) : (
                        <div className="flex items-center">
                          <Eye className="h-3 w-3 mr-1" />
                          <span>Viewing</span>
                        </div>
                      )}
                    </div>
                  </div>

                  {editing && config.permissions.writeDags ? (
                    <div className="flex gap-2">
                      <Button
                        id="save-config"
                        variant="default"
                        size="sm"
                        className="cursor-pointer shadow-sm hover:shadow-md transition-shadow duration-200"
                        onClick={async () => {
                          if (!currentValue) {
                            alert('No changes to save');
                            return;
                          }
                          // Save current scroll position before any operations that might cause re-render
                          saveScrollPosition();
                          const { data, error } = await client.PUT(
                            '/dags/{fileName}/spec',
                            {
                              params: {
                                path: {
                                  fileName: props.fileName,
                                },
                                query: {
                                  remoteNode:
                                    appBarContext.selectedRemoteNode || 'local',
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
                          if (data?.errors) {
                            alert(data.errors.join('\n'));
                            return;
                          }
                          // Show success toast notification
                          showToast('Changes saved successfully');

                          setEditing(false);
                          props.refresh();
                        }}
                      >
                        <Save className="h-4 w-4 mr-1" />
                        Save Changes
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        className="cursor-pointer hover:bg-red-50 hover:text-red-600 hover:border-red-200 dark:hover:bg-red-900/20 dark:hover:text-red-400 dark:hover:border-red-800 transition-colors duration-200"
                        onClick={() => {
                          saveScrollPosition();
                          setEditing(false);
                        }}
                      >
                        <X className="h-4 w-4 mr-1" />
                        Cancel
                      </Button>
                    </div>
                  ) : config.permissions.writeDags ? (
                    <Button
                      id="edit-config"
                      variant="outline"
                      size="sm"
                      className="cursor-pointer hover:bg-blue-50 hover:text-blue-600 hover:border-blue-200 dark:hover:bg-blue-900/20 dark:hover:text-blue-400 dark:hover:border-blue-800 transition-colors duration-200"
                      onClick={() => {
                        saveScrollPosition();
                        setEditing(true);
                      }}
                    >
                      <Edit className="h-4 w-4 mr-1" />
                      Edit
                    </Button>
                  ) : null}
                </div>

                <div className="p-6">
                  <DAGEditor
                    value={data.spec}
                    readOnly={!editing || !config.permissions.writeDags}
                    lineNumbers={true}
                    onChange={
                      editing && config.permissions.writeDags
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
        )
      }
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
