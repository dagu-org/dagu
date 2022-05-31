import React from "react";
import { GetListResponse } from "../api/List";
import WorkflowErrors from "../components/WorkflowErrors";
import Box from "@mui/material/Box";
import CreateWorkflowButton from "../components/CreateWorkflowButton";
import WithLoading from "../components/WithLoading";
import WorkflowTable from "../components/WorkflowTable";
import Title from "../components/Title";
import Paper from "@mui/material/Paper";
import { useGetApi } from "../hooks/useWorkflowsGetApi";
import Loading from "../components/Loading";

function Workflows() {
  const [group] = React.useState<string>(
    new URLSearchParams(window.location.search).get("group") || ""
  );
  const { data, doGet } = useGetApi<GetListResponse>("/", {
    group: group,
  });
  React.useEffect(() => {
    const timer = setInterval(doGet, 10000);
    return () => clearInterval(timer);
  }, []);

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
        <Title>Workflows</Title>
        <CreateWorkflowButton refresh={doGet}></CreateWorkflowButton>
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
                workflows={data.DAGs}
                groups={data.Groups}
                group={data.Group}
              ></WorkflowTable>
            </React.Fragment>
          )}
        </WithLoading>
      </Box>
    </Paper>
  );
}
export default Workflows;
