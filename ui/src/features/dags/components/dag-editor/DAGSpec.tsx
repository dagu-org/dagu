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
import { useMutate, useQuery } from '../../../../hooks/api';
import LoadingIndicator from '../../../../ui/LoadingIndicator';
import { components } from '../../../../api/v2/schema';
import { useClient } from '../../../../hooks/api';
import { useParams } from 'react-router-dom';
import { Graph, FlowchartType, FlowchartSwitch } from '../visualization';
import { DAGStepTable } from '../dag-details';
import { Button } from '../../../../components/ui/button';
import { Save, X, Edit, AlertTriangle } from 'lucide-react';
import SubTitle from '@/ui/SubTitle';
import BorderedBox from '@/ui/BorderedBox';

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
  const client = useClient();
  const mutate = useMutate();

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
            <div className="space-y-4">
              <div className="overflow-x-auto rounded-xl shadow-md bg-white dark:bg-slate-900 p-6">
                <div className="flex justify-between items-center mb-4">
                  <SubTitle className="mb-0">Graph</SubTitle>
                  <FlowchartSwitch
                    value={cookie['flowchart']}
                    onChange={onChangeFlowchart}
                  />
                </div>
                <BorderedBox className="mt-4 py-4 px-4 flex flex-col overflow-x-auto">
                  <Graph
                    steps={data.dag.steps}
                    type="config"
                    flowchart={flowchart}
                    showIcons={false}
                  />
                </BorderedBox>
              </div>

              <div className="bg-white dark:bg-slate-900 rounded-xl shadow-md p-6 overflow-hidden">
                <SubTitle className="mb-4">Attributes</SubTitle>
                <DAGAttributes dag={data.dag!} />
              </div>

              {data.errors?.length ? (
                <div className="bg-white dark:bg-slate-900 rounded-xl shadow-md p-6 border-l-4 border-red-500 dark:border-red-700">
                  <div className="flex items-center gap-2 mb-4 text-red-600 dark:text-red-400">
                    <AlertTriangle className="h-5 w-5" />
                    <h3 className="font-semibold">Configuration Errors</h3>
                  </div>

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
              ) : null}

              {data.dag.steps ? (
                <div className="bg-white dark:bg-slate-900 rounded-xl shadow-md p-6 overflow-hidden">
                  <SubTitle className="mb-4">Steps</SubTitle>
                  <DAGStepTable steps={data.dag.steps} />
                </div>
              ) : null}

              {handlers?.length ? (
                <div className="bg-white dark:bg-slate-900 rounded-xl shadow-md p-6 overflow-hidden">
                  <SubTitle className="mb-4">Lifecycle Hooks</SubTitle>
                  <DAGStepTable steps={handlers} />
                </div>
              ) : null}

              <div className="bg-white dark:bg-slate-900 rounded-xl shadow-md p-6 overflow-hidden">
                <div className="flex justify-between items-center mb-4">
                  <SubTitle className="mb-0">Spec</SubTitle>
                  {editing ? (
                    <div className="flex gap-2">
                      <Button
                        id="save-config"
                        variant="default"
                        size="sm"
                        className="cursor-pointer"
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
                          mutate(['/dags/{fileId}/spec']);
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
                        className="cursor-pointer"
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
                      className="cursor-pointer"
                      onClick={() => setEditing(true)}
                    >
                      <Edit className="h-4 w-4 mr-1" />
                      Edit
                    </Button>
                  )}
                </div>

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
