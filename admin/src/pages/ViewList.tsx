import React from "react";
import Box from "@mui/material/Box";
import WithLoading from "../components/WithLoading";
import Title from "../components/Title";
import Paper from "@mui/material/Paper";
import { useGetApi } from "../hooks/useWorkflowsGetApi";
import CreateViewButton from "../components/CreateViewButton";
import ViewTable from "../components/ViewTable";
import { View } from "../models/View";

function ViewList() {
  const { data, doGet } = useGetApi<{ Views: View[] }>("/views", {});

  React.useEffect(() => {
    doGet();
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
        <Title>Views</Title>
        <CreateViewButton refresh={doGet}></CreateViewButton>
      </Box>
      <Box>
        <WithLoading loaded={!!data}>
          {data && <ViewTable views={data.Views} refreshFn={doGet}></ViewTable>}
        </WithLoading>
      </Box>
    </Paper>
  );
}
export default ViewList;
