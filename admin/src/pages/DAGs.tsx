import React from "react";
import DAGErrors from "../components/DAGErrors";
import Box from "@mui/material/Box";
import CreateWorkflowButton from "../components/CreateWorkflowButton";
import WithLoading from "../components/WithLoading";
import DAGTable from "../components/DAGTable";
import Title from "../components/Title";
import Paper from "@mui/material/Paper";
import { useGetApi } from "../hooks/useWorkflowsGetApi";
import { WorkflowData, WorkflowDataType } from "../models/DAG";
import { useLocation } from "react-router-dom";
import { GetDAGsResponse } from "../api/DAGs";

function DAGs() {
  const useQuery = () => new URLSearchParams(useLocation().search);
  let query = useQuery();
  const group = query.get("group") || "";

  const { data, doGet } = useGetApi<GetDAGsResponse>("/", {});

  React.useEffect(() => {
    doGet();
    const timer = setInterval(doGet, 10000);
    return () => clearInterval(timer);
  }, [group]);

  const merged = React.useMemo(() => {
    const ret: WorkflowData[] = [];
    if (data) {
      // TODO: need refactoring
      if (group != "") {
        ret.push({
          Type: WorkflowDataType.Group,
          Name: "../",
          Group: {
            Name: "",
            Dir: "",
          },
        });
      }
      for (const val of data.Groups) {
        ret.push({
          Type: WorkflowDataType.Group,
          Name: val.Name,
          Group: val,
        });
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
    }
    return ret;
  }, [data, query.get("group")]);

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
        <Title>DAGs</Title>
        <CreateWorkflowButton refresh={doGet}></CreateWorkflowButton>
      </Box>
      <Box>
        <WithLoading loaded={!!data && !!merged}>
          {data && (
            <React.Fragment>
              <DAGErrors
                DAGs={data.DAGs}
                errors={data.Errors}
                hasError={data.HasError}
              ></DAGErrors>
              <DAGTable
                workflows={merged}
                group={data.Group}
                refreshFn={doGet}
              ></DAGTable>
            </React.Fragment>
          )}
        </WithLoading>
      </Box>
    </Paper>
  );
}
export default DAGs;
