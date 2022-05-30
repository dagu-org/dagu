import React from "react";
import { GetListResponse } from "../api/List";
import WorkflowErrors from "../components/WorkflowErrors";
import Box from "@mui/material/Box";
import CreateWorkflowButton from "../components/CreateWorkflowButton";
import WithLoading from "../components/WithLoading";
import WorkflowTable from "../components/WorkflowTable";
import Title from "../components/Title";
import Paper from "@mui/material/Paper";

function WorkflowsPage() {
  const [data, setData] = React.useState<GetListResponse | undefined>();

  async function getData() {
    const urlParams = new URLSearchParams(window.location.search);
    let url = API_URL + "?format=json";
    const group = urlParams.get("group");
    if (group) {
      url += `&group=${group}`;
    }
    const resp = await fetch(url, {
      method: "GET",
      cache: "no-store",
      mode: "cors",
      headers: {
        Accept: "application/json",
      },
    });
    if (!resp.ok) {
      return;
    }
    const body = await resp.json();
    setData(body);
  }

  React.useEffect(() => {
    getData();
    const timer = setInterval(getData, 10000);
    return () => clearInterval(timer);
  }, []);

  if (!data) {
    return <div>Loading...</div>;
  }

  return (
    <Paper sx={{ p: 2, display: "flex", flexDirection: "column" }}>
      <Box
        sx={{
          display: "flex",
          flexDirection: "row",
          alignItems: "center",
          justifyContent: "space-between",
        }}
      >
        <Title>Workflows</Title>
        <CreateWorkflowButton refresh={getData}></CreateWorkflowButton>
      </Box>
      <Box>
        <WithLoading loaded={!!data}>
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
        </WithLoading>
      </Box>
    </Paper>
  );
}
export default WorkflowsPage;
