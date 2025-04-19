/**
 * DAGSpec component displays and allows editing of a DAG specification.
 *
 * @module features/dags/components/dag-editor
 */
import React, { useEffect } from 'react';
import { DAGContext } from '../../contexts/DAGContext';
import DAGEditor from './DAGEditor';
import DAGAttributes from './DAGAttributes';
import DAGDefinition from './DAGDefinition';
import { useCookies } from 'react-cookie';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { useQuery } from '../../../../hooks/api';
import LoadingIndicator from '../../../../ui/LoadingIndicator';
import { components } from '../../../../api/v2/schema';
import { useClient } from '../../../../hooks/api';
import { useParams } from 'react-router-dom';
import { Graph, FlowchartType, FlowchartSwitch } from '../visualization';
import { DAGStepTable } from '../dag-details';
import { Button } from '../../../../components/ui/button';
import { Save, X, Edit, AlertTriangle } from 'lucide-react';

/**
 * Props for the DAGSpec component
 */
type Props = {
  /** DAG file ID */
  fileId: string;
};

/**
 * DAGSpec displays and allows editing of a DAG specification
 * including visualization, attributes, steps, and YAML definition
 */
function DAGSpec({ fileId }: Props) {
  const appBarContext = React.useContext(AppBarContext);
  const params = useParams();
  const client = useClient();

  // State for editing mode and current YAML value
  const [editing, setEditing] = React.useState(false);
  const [currentValue, setCurrentValue] = React.useState<string | undefined>();

  // Flowchart direction preference stored in cookies
  const [cookie, setCookie] = useCookies(['flowchart']);
  const [flowchart, setFlowchart] = React.useState(cookie['flowchart']);

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
    '/dags/{fileId}/spec',
    {
      params: {
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
        },
        path: {
          fileId: fileId,
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
            <div className="flex justify-between items-center my-4">
              <FlowchartSwitch
                value={cookie['flowchart']}
                onChange={onChangeFlowchart}
              />
            </div>

            <div className="mb-6 overflow-x-auto border border-slate-200 dark:border-slate-700 rounded-md bg-white dark:bg-slate-900 p-4">
              <Graph
                steps={data.dag.steps}
                type="config"
                flowchart={flowchart}
                showIcons={false}
              />
            </div>

            <DAGAttributes dag={data.dag!} />

            {data.errors?.length ? (
              <div className="mb-6 border border-red-200 dark:border-red-800 bg-white dark:bg-slate-900 rounded-md p-6">
                <div className="flex items-center gap-2 mb-3 text-red-600 dark:text-red-400">
                  <AlertTriangle className="h-5 w-5" />
                  <h3 className="font-semibold">Configuration Errors</h3>
                </div>

                <div className="space-y-2">
                  {data.errors?.map((e, i) => (
                    <div
                      key={i}
                      className="p-2 border-l-2 border-red-300 dark:border-red-700 text-red-600 dark:text-red-400 font-mono text-sm"
                    >
                      {e}
                    </div>
                  ))}
                </div>
              </div>
            ) : null}

            {data.dag.steps ? <DAGStepTable steps={data.dag.steps} /> : null}

            {handlers?.length ? <DAGStepTable steps={handlers} /> : null}

            <div className="mb-6 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-md p-4">
              <div className="flex justify-end items-center mb-4">
                {editing ? (
                  <div className="flex gap-2">
                    <Button
                      id="save-config"
                      variant="default"
                      size="sm"
                      onClick={async () => {
                        if (!currentValue) {
                          alert('No changes to save');
                          return;
                        }
                        const { data, error } = await client.PUT(
                          '/dags/{fileId}/spec',
                          {
                            params: {
                              path: {
                                fileId: props.fileId,
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
                      onClick={() => setEditing(false)}
                    >
                      <X className="h-4 w-4 mr-1" />
                      Cancel
                    </Button>
                  </div>
                ) : (
                  <Button
                    id="edit-config"
                    variant="outline"
                    size="sm"
                    onClick={() => setEditing(true)}
                  >
                    <Edit className="h-4 w-4 mr-1" />
                    Edit
                  </Button>
                )}
              </div>

              <div>
                {editing ? (
                  <DAGEditor
                    value={data.spec}
                    onChange={(newValue) => {
                      setCurrentValue(newValue || '');
                    }}
                  />
                ) : (
                  <DAGDefinition value={data.spec} lineNumbers />
                )}
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
