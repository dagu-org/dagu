import React from "react";
import WorkflowErrors from "../components/WorkflowErrors";
import Box from "@mui/material/Box";
import WithLoading from "../components/WithLoading";
import WorkflowTable from "../components/WorkflowTable";
import Title from "../components/Title";
import Paper from "@mui/material/Paper";
import { useGetApi } from "../hooks/useWorkflowsGetApi";
import { WorkflowData, WorkflowDataType } from "../models/Workflow";
import { useParams } from "react-router-dom";
import { DAG } from "../models/Dag";

export type ApiResponse = {
  Title: string;
  Charset: string;
  DAGs: DAG[];
  Errors: string[];
  HasError: boolean;
};

type Params = {
  name: string;
};

function View() {
  const params = useParams<Params>();

  const { data, doGet } = useGetApi<ApiResponse>(
    `/views/${params.name}?format=json`,
    {}
  );

  React.useEffect(() => {
    doGet();
    const timer = setInterval(doGet, 10000);
    return () => clearInterval(timer);
  }, []);

  const workflows = React.useMemo(() => {
    const ret: WorkflowData[] = [];
    if (!data) {
      return ret;
    }
    for (const val of data.DAGs) {
      if (!val.Error) {
        ret.push({
          Type: WorkflowDataType.Workflow,
          Name: val.Config.Name,
          DAG: val,
        });
      }
    }
    return ret;
  }, [data]);

  return (
    <Paper
      sx={{
        p: 2,
        mx: 4,
        display: "flex",
        flexDirection: "column",
        width: "100%",
      }}
    >
      <Box
        sx={{
          display: "flex",
          flexDirection: "row",
          alignItems: "center",
          justifyContent: "space-between",
        }}
      >
        <Title>{`${decodeURI(params.name || "View")}`}</Title>
      </Box>
      <Box>
        <WithLoading loaded={!!data}>
          {data && (
            <React.Fragment>
              <WorkflowErrors
                workflows={data.DAGs}
                errors={data.Errors}
                hasError={data.HasError}
              ></WorkflowErrors>
              <WorkflowTable
                workflows={workflows}
                group={""}
                refreshFn={doGet}
              ></WorkflowTable>
            </React.Fragment>
          )}
        </WithLoading>
      </Box>
    </Paper>
  );
}
export default View;
