import React from "react";
import DAGErrors from "../components/DAGErrors";
import Box from "@mui/material/Box";
import WithLoading from "../components/WithLoading";
import DAGTable from "../components/DAGTable";
import Title from "../components/Title";
import Paper from "@mui/material/Paper";
import { useDAGGetAPI } from "../hooks/useDAGGetAPI";
import { DAGItem, DAGDataType } from "../models/DAGData";
import { useParams } from "react-router-dom";
import { DAG } from "../models/DAGData";

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

  const { data, doGet } = useDAGGetAPI<ApiResponse>(
    `/views/${params.name}?format=json`,
    {}
  );

  React.useEffect(() => {
    doGet();
    const timer = setInterval(doGet, 10000);
    return () => clearInterval(timer);
  }, []);

  const DAGs = React.useMemo(() => {
    const ret: DAGItem[] = [];
    if (!data) {
      return ret;
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
              <DAGErrors
                DAGs={data.DAGs}
                errors={data.Errors}
                hasError={data.HasError}
              ></DAGErrors>
              <DAGTable
                DAGs={DAGs}
                group={""}
                refreshFn={doGet}
              ></DAGTable>
            </React.Fragment>
          )}
        </WithLoading>
      </Box>
    </Paper>
  );
}
export default View;
