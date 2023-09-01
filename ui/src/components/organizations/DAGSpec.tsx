import { Box, Button, Stack } from '@mui/material';
import React from 'react';
import { GetDAGResponse } from '../../models/api';
import { DAGContext } from '../../contexts/DAGContext';
import { DAG, Step } from '../../models';
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

type Props = {
  data: GetDAGResponse;
};

function DAGSpec({ data }: Props) {
  const [editing, setEditing] = React.useState(false);
  const [currentValue, setCurrentValue] = React.useState(data.Definition);
  const handlers = getHandlers(data.DAG?.DAG);
  const [cookie, setCookie] = useCookies(['flowchart']);
  const [flowchart, setFlowchart] = React.useState(cookie['flowchart']);
  const onChangeFlowchart = React.useCallback(
    (value: FlowchartType) => {
      setCookie('flowchart', value, { path: '/' });
      setFlowchart(value);
    },
    [setCookie, flowchart, setFlowchart]
  );
  if (data.DAG?.DAG == null) {
    return null;
  }
  return (
    <DAGContext.Consumer>
      {(props) =>
        data?.DAG?.DAG && (
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
                    steps={data.DAG.DAG.Steps}
                    type="config"
                    flowchart={flowchart}
                  />
                </Box>
              </BorderedBox>
            </Box>

            <Box sx={{ mt: 3 }}>
              <Box sx={{ mt: 2 }}>
                <DAGAttributes dag={data.DAG.DAG!}></DAGAttributes>
              </Box>
            </Box>
            <Box sx={{ mt: 3 }}>
              <Box sx={{ mt: 2 }}>
                <SubTitle>Steps</SubTitle>
                <DAGStepTable steps={data.DAG.DAG.Steps}></DAGStepTable>
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
                    {data.DAG.DAG.Location}
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
                          const url = `${getConfig().apiURL}/dags/${
                            props.name
                          }`;
                          const resp = await fetch(url, {
                            method: 'POST',
                            headers: {
                              'Content-Type': 'application/json',
                            },
                            body: JSON.stringify({
                              action: 'save',
                              value: currentValue,
                            }),
                          });
                          if (resp.ok) {
                            setEditing(false);
                            props.refresh();
                          } else {
                            const e = await resp.text();
                            alert(e);
                          }
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
                      value={data.Definition}
                      onChange={(newValue) => {
                        setCurrentValue(newValue);
                      }}
                    ></DAGEditor>
                  </Box>
                ) : (
                  <DAGDefinition value={data.Definition} lineNumbers />
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

function getHandlers(dag?: DAG) {
  const r: Step[] = [];
  if (!dag) {
    return r;
  }
  const h = dag.HandlerOn;
  if (h.Success) {
    r.push(h.Success);
  }
  if (h.Failure) {
    r.push(h.Failure);
  }
  if (h.Cancel) {
    r.push(h.Cancel);
  }
  if (h.Exit) {
    r.push(h.Exit);
  }
  return r;
}
