import { Box, Button, Stack } from '@mui/material';
import React, { useEffect } from 'react';
import { DAGContext } from '../../contexts/DAGContext';
import DAGEditor from '../atoms/DAGEditor';
import DAGAttributes from '../molecules/DAGAttributes';
import DAGDefinition from '../molecules/DAGDefinition';
import Graph, { FlowchartType } from '../molecules/Graph';
import DAGStepTable from '../molecules/DAGStepTable';
import BorderedBox from '../atoms/BorderedBox';
import SubTitle from '../atoms/SubTitle';
import FlowchartSwitch from '../molecules/FlowchartSwitch';
import { useCookies } from 'react-cookie';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import {
  faFloppyDisk,
  faXmark,
  faPenToSquare,
} from '@fortawesome/free-solid-svg-icons';
import { AppBarContext } from '../../contexts/AppBarContext';
import { useMutate, useQuery } from '../../hooks/api';
import LoadingIndicator from '../atoms/LoadingIndicator';
import { components } from '../../api/v2/schema';
import { useClient } from '../../hooks/api';

type Props = {
  location: string;
};

function DAGSpec({ location }: Props) {
  const appBarContext = React.useContext(AppBarContext);
  const client = useClient();
  const [editing, setEditing] = React.useState(false);
  const [currentValue, setCurrentValue] = React.useState<string | undefined>();
  const [cookie, setCookie] = useCookies(['flowchart']);
  const [flowchart, setFlowchart] = React.useState(cookie['flowchart']);
  const mutate = useMutate();
  const onChangeFlowchart = React.useCallback(
    (value: FlowchartType) => {
      setCookie('flowchart', value, { path: '/' });
      setFlowchart(value);
    },
    [setCookie, flowchart, setFlowchart]
  );
  const { data, isLoading } = useQuery(
    '/dags/{dagLocation}/spec',
    {
      params: {
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
        },
        path: {
          dagLocation: location,
        },
      },
    },
    { refreshInterval: 2000 }
  );
  useEffect(() => {
    if (data) {
      setCurrentValue(data.spec);
    }
  }, [data]);

  const handlers = getHandlers(data?.dag);
  if (isLoading) {
    return <LoadingIndicator />;
  }
  return (
    <DAGContext.Consumer>
      {(props) =>
        data?.dag && (
          <React.Fragment>
            <Box>
              <Stack direction="row" justifyContent="space-between">
                <SubTitle>Overview</SubTitle>
                <FlowchartSwitch
                  value={cookie['flowchart']}
                  onChange={onChangeFlowchart}
                />
              </Stack>
              <BorderedBox
                sx={{
                  mt: 2,
                  py: 2,
                  px: 2,
                  display: 'flex',
                  flexDirection: 'column',
                  overflowX: 'auto',
                }}
              >
                <Box
                  sx={{
                    overflowX: 'auto',
                  }}
                >
                  <Graph
                    steps={data.dag.steps}
                    type="config"
                    flowchart={flowchart}
                    showIcons={false}
                  />
                </Box>
              </BorderedBox>
            </Box>

            <Box sx={{ mt: 3 }}>
              <Box sx={{ mt: 2 }}>
                <DAGAttributes dag={data.dag!}></DAGAttributes>
              </Box>
            </Box>
            <Box sx={{ mt: 3 }}>
              <Box sx={{ mt: 2 }}>
                <SubTitle>Steps</SubTitle>
                {data.errors?.length ? (
                  <BorderedBox
                    sx={{
                      mt: 2,
                      px: 2,
                      py: 1,
                      display: 'flex',
                      flexDirection: 'column',
                      backgroundColor: 'error.light',
                      color: 'error.contrastText',
                    }}
                  >
                    {data.errors.map((e, i) => (
                      <Box key={i} sx={{ mb: 1 }}>
                        {e}
                      </Box>
                    ))}
                  </BorderedBox>
                ) : null}
                {data.dag.steps ? (
                  <DAGStepTable steps={data.dag.steps}></DAGStepTable>
                ) : null}
              </Box>
            </Box>
            {handlers?.length ? (
              <Box sx={{ mt: 3 }}>
                <SubTitle>Lifecycle Hooks</SubTitle>
                <Box sx={{ mt: 2 }}>
                  <DAGStepTable steps={handlers}></DAGStepTable>
                </Box>
              </Box>
            ) : null}

            <Box sx={{ mt: 3 }}>
              <SubTitle>Spec</SubTitle>
              <BorderedBox
                sx={{
                  mt: 2,
                  px: 2,
                  py: 1,
                  display: 'flex',
                  flexDirection: 'column',
                }}
              >
                <Stack
                  direction="row"
                  justifyContent="space-between"
                  alignItems="center"
                >
                  <Box
                    sx={{
                      color: 'grey.600',
                    }}
                  >
                    {data.dag.location}
                  </Box>
                  {editing ? (
                    <Stack direction="row">
                      <Button
                        id="save-config"
                        color="primary"
                        variant="outlined"
                        startIcon={
                          <span className="icon">
                            <FontAwesomeIcon icon={faFloppyDisk} />
                          </span>
                        }
                        onClick={async () => {
                          if (!currentValue) {
                            alert('No changes to save');
                            return;
                          }
                          const { error, response } = await client.PUT(
                            '/dags/{dagLocation}/spec',
                            {
                              params: {
                                path: {
                                  dagLocation: props.location,
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
                          if (response.status == 400) {
                            type ValidationError = { errors: string[] };
                            const validationError = error as ValidationError;
                            alert(
                              validationError.errors
                                .map((e) => e.replace('spec.', ''))
                                .join('\n')
                            );
                            return;
                          }
                          if (response.status != 200) {
                            alert(error || 'Failed to save spec');
                          }
                          setEditing(false);
                          mutate(['/dags/{dagLocation}/spec']);
                          mutate(['/dags']);
                          props.refresh();
                        }}
                      >
                        Save
                      </Button>
                      <Button
                        color="error"
                        variant="outlined"
                        onClick={() => setEditing(false)}
                        sx={{ ml: 2 }}
                        startIcon={
                          <span className="icon">
                            <FontAwesomeIcon icon={faXmark} />
                          </span>
                        }
                      >
                        Cancel
                      </Button>
                    </Stack>
                  ) : (
                    <Stack direction="row">
                      <Button
                        id="edit-config"
                        variant="outlined"
                        color="primary"
                        onClick={() => setEditing(true)}
                        startIcon={
                          <span className="icon">
                            <FontAwesomeIcon icon={faPenToSquare} />
                          </span>
                        }
                      >
                        Edit
                      </Button>
                    </Stack>
                  )}
                </Stack>
                {editing ? (
                  <Box sx={{ mt: 2 }}>
                    <DAGEditor
                      value={data.spec}
                      onChange={(newValue) => {
                        setCurrentValue(newValue || '');
                      }}
                    ></DAGEditor>
                  </Box>
                ) : (
                  <DAGDefinition value={data.spec} lineNumbers />
                )}
              </BorderedBox>
            </Box>
          </React.Fragment>
        )
      }
    </DAGContext.Consumer>
  );
}
export default DAGSpec;

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
