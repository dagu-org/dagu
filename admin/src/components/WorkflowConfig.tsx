import { Box, Button, Paper, Stack, Typography } from "@mui/material";
import React from "react";
import { GetWorkflowResponse } from "../api/Workflow";
import { WorkflowContext } from "../contexts/WorkflowContext";
import { Config } from "../models/Config";
import { Step } from "../models/Step";
import ConfigEditor from "./ConfigEditor";
import ConfigInfoTable from "./ConfigInfoTable";
import ConfigPreview from "./ConfigPreview";
import GraphDag from "./GraphDag";
import ConfigStepTable from "./ConfigStepTable";

type Props = {
  data: GetWorkflowResponse;
  width: number;
};

function WorkflowConfig({ data, width }: Props) {
  const [editing, setEditing] = React.useState(false);
  const [currentValue, setCurrentValue] = React.useState(data.Definition);
  const handlers = getHandlersFromConfig(data.DAG?.Config);
  if (data.DAG?.Config == null) {
    return null;
  }
  return (
    <WorkflowContext.Consumer>
      {(props) =>
        data.DAG &&
        data.DAG.Config && (
          <React.Fragment>
            <Paper
              sx={{
                pb: 4,
                px: 2,
                display: "flex",
                flexDirection: "column",
                overflowX: "auto",
                borderTopLeftRadius: 0,
                borderTopRightRadius: 0,
              }}
            >
              <Box
                maxWidth={width ? `${width - 100}px` : "100%"}
                sx={{
                  overflowX: "auto",
                }}
              >
                <GraphDag
                  steps={data.DAG.Config.Steps}
                  type="config"
                ></GraphDag>
              </Box>
            </Paper>

            <Box sx={{ mt: 2 }}>
              <ConfigInfoTable config={data.DAG.Config!}></ConfigInfoTable>
            </Box>
            <Box sx={{ mt: 2 }}>
              <ConfigStepTable steps={data.DAG.Config.Steps}></ConfigStepTable>
            </Box>
            <Box sx={{ mt: 2 }}>
              <ConfigStepTable steps={handlers}></ConfigStepTable>
            </Box>

            <Paper
              sx={{
                mt: 2,
                p: 2,
                display: "flex",
                flexDirection: "column",
              }}
            >
              <Stack
                direction="row"
                justifyContent="space-between"
                alignItems="center"
              >
                <Typography variant="body1">
                  {data.DAG.Config.ConfigPath}
                </Typography>
                {editing ? (
                  <Stack direction="row">
                    <Button
                      color="primary"
                      variant="contained"
                      onClick={async () => {
                        const formData = new FormData();
                        formData.append("action", "save");
                        formData.append("value", currentValue);
                        const url = `${API_URL}/dags/${props.name}`;
                        const resp = await fetch(url, {
                          method: "POST",
                          headers: {
                            Accept: "application/json",
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
                    >
                      Cancel
                    </Button>
                  </Stack>
                ) : (
                  <Stack direction="row">
                    <Button
                      variant="contained"
                      color="info"
                      onClick={() => setEditing(true)}
                    >
                      Edit
                    </Button>
                  </Stack>
                )}
              </Stack>
              {editing ? (
                <Box sx={{ mt: 2 }}>
                  <ConfigEditor
                    value={data.Definition}
                    onChange={(newValue) => {
                      setCurrentValue(newValue);
                    }}
                  ></ConfigEditor>
                </Box>
              ) : (
                <ConfigPreview value={data.Definition} />
              )}
            </Paper>
          </React.Fragment>
        )
      }
    </WorkflowContext.Consumer>
  );
}
export default WorkflowConfig;

function getHandlersFromConfig(cfg?: Config) {
  const r: Step[] = [];
  if (!cfg) {
    return r;
  }
  const h = cfg.HandlerOn;
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
