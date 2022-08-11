import { Box, Button, Stack } from '@mui/material';
import React from 'react';
import { GetDAGResponse } from '../../api/DAG';
import { DAGContext } from '../../contexts/DAGContext';
import { DAG, Step } from '../../models';
import DAGEditor from '../atoms/DAGEditor';
import DAGAttributes from '../molecules/DAGAttributes';
import DAGDefinition from '../molecules/DAGDefinition';
import Graph from '../molecules/Graph';
import DAGStepTable from '../molecules/DAGStepTable';
import BorderedBox from '../atoms/BorderedBox';
import SubTitle from '../atoms/SubTitle';

type Props = {
  data: GetDAGResponse;
};

function DAGSpec({ data }: Props) {
  const [editing, setEditing] = React.useState(false);
  const [currentValue, setCurrentValue] = React.useState(data.Definition);
  const handlers = getHandlers(data.DAG?.DAG);
  if (data.DAG?.DAG == null) {
    return null;
  }
  return (
    <DAGContext.Consumer>
      {(props) =>
        data.DAG &&
        data.DAG.DAG && (
          <React.Fragment>
            <Box>
              <SubTitle>Overview</SubTitle>
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
                  <Graph steps={data.DAG.DAG.Steps} type="config"></Graph>
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
                <DAGStepTable
                  steps={data.DAG.DAG.Steps}
                ></DAGStepTable>
              </Box>
            </Box>
            {handlers && handlers.length ? (
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
                    {data.DAG.DAG.Path}
                  </Box>
                  {editing ? (
                    <Stack direction="row">
                      <Button
                        id="save-config"
                        color="primary"
                        variant="contained"
                        startIcon={
                          <span className="icon">
                            <i className="fa-solid fa-floppy-disk"></i>
                          </span>
                        }
                        onClick={async () => {
                          const formData = new FormData();
                          formData.append('action', 'save');
                          formData.append('value', currentValue);
                          const url = `${API_URL}/dags/${props.name}`;
                          const resp = await fetch(url, {
                            method: 'POST',
                            headers: {
                              Accept: 'application/json',
                            },
                            body: formData,
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
                        variant="contained"
                        onClick={() => setEditing(false)}
                        sx={{ ml: 2 }}
                        startIcon={
                          <span className="icon">
                            <i className="fa-solid fa-xmark"></i>
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
                        variant="contained"
                        color="info"
                        onClick={() => setEditing(true)}
                        startIcon={
                          <span className="icon">
                            <i className="fa-solid fa-pen-to-square"></i>
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
                  <DAGDefinition value={data.Definition} />
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
