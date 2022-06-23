import React from "react";
import DAGErrors from "../components/DAGErrors";
import Box from "@mui/material/Box";
import DAGCreationButton from "../components/DAGCreationButton";
import WithLoading from "../components/WithLoading";
import DAGTable from "../components/DAGTable";
import Title from "../components/Title";
import Paper from "@mui/material/Paper";
import { useDAGGetAPI } from "../hooks/useDAGGetAPI";
import { DAGItem, DAGDataType } from "../models/DAG";
import { useLocation } from "react-router-dom";
import { GetDAGsResponse } from "../api/DAGs";

function DAGs() {
  const useQuery = () => new URLSearchParams(useLocation().search);
  let query = useQuery();
  const group = query.get("group") || "";

  const { data, doGet } = useDAGGetAPI<GetDAGsResponse>("/", {});

  React.useEffect(() => {
    doGet();
    const timer = setInterval(doGet, 10000);
    return () => clearInterval(timer);
  }, [group]);

  const merged = React.useMemo(() => {
    const ret: DAGItem[] = [];
    if (data) {
      // TODO: need refactoring
      if (group != "") {
        ret.push({
          Type: DAGDataType.Group,
          Name: "../",
          Group: {
            Name: "",
            Dir: "",
          },
        });
      }
      for (const val of data.Groups) {
        ret.push({
          Type: DAGDataType.Group,
          Name: val.Name,
          Group: val,
        });
      }
      for (const val of data.DAGs) {
        if (!val.Error) {
          ret.push({
            Type: DAGDataType.DAG,
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
        <DAGCreationButton refresh={doGet}></DAGCreationButton>
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
                DAGs={merged}
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
