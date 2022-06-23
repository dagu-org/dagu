import React from "react";
import { useParams } from "react-router-dom";
import { GetDAGResponse } from "../api/DAG";
import ConfigErrors from "../components/ConfigErrors";
import DAGStatus from "../components/DAGStatus";
import { DAGContext } from "../contexts/DAGContext";
import { DetailTabId } from "../models/DAG";
import DAGConfig from "../components/DAGConfig";
import DAGHistory from "../components/DAGHistory";
import DAGLog from "../components/DAGLog";
import { Box, Paper, Stack, Tab, Tabs } from "@mui/material";
import Title from "../components/Title";
import DAGActions from "../components/DAGActions";
import ConfigEditButtons from "../components/ConfigEditButtons";
import Loading from "../components/Loading";

type Params = {
  name: string;
};

function DAGDetails() {
  const params = useParams<Params>();
  const [data, setData] = React.useState<GetDAGResponse | undefined>(
    undefined
  );
  const [tab, setTab] = React.useState(DetailTabId.Status);
  const [group, setGroup] = React.useState("");
  React.useEffect(() => {
    const urlParams = new URLSearchParams(window.location.search);
    let t = urlParams.get("t");
    if (t) {
      setTab(t as DetailTabId);
    }
    let group = urlParams.get("group");
    if (group) {
      setGroup(group);
    }
  }, []);
  async function getData() {
    let url = API_URL + `/dags/${params.name}?format=json`;
    const urlParams = new URLSearchParams(window.location.search);
    url += "&" + urlParams.toString();
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
    if (tab == DetailTabId.Status || tab == DetailTabId.Config) {
      const timer = setInterval(getData, 2000);
      return () => clearInterval(timer);
    }
  }, [tab]);

  if (!params.name || !data || !data.DAG) {
    return <Loading />;
  }

  const contents: Partial<{
    [key in DetailTabId]: React.ReactNode;
  }> = {
    [DetailTabId.Status]: (
      <DAGStatus
        DAG={data.DAG}
        group={group}
        name={params.name}
        refresh={getData}
      />
    ),
    [DetailTabId.Config]: <DAGConfig data={data} />,
    [DetailTabId.History]: <DAGHistory logData={data.LogData} />,
    [DetailTabId.StepLog]: <DAGLog log={data.StepLog} />,
    [DetailTabId.ScLog]: <DAGLog log={data.ScLog} />,
  };
  const ctx = {
    data: data,
    refresh: getData,
    tab,
    group,
    name: params.name,
  };

  const baseUrl = `/dags/${encodeURI(params.name)}?group=${encodeURI(group)}`;

  return (
    <DAGContext.Provider value={ctx}>
      <Stack
        sx={{
          width: "100%",
          direction: "column",
        }}
      >
        <Paper
          sx={{
            mx: 4,
            p: 2,
            borderBottomLeftRadius: 0,
            borderBottomRightRadius: 0,
          }}
        >
          <ConfigErrors errors={data.Errors} />

          <Box
            sx={{
              display: "flex",
              flexDirection: "row",
              alignItems: "center",
              justifyContent: "space-between",
            }}
          >
            <Title>{data.Title}</Title>
            <DAGActions
              status={data.DAG.Status}
              group={group}
              name={params.name!}
              refresh={getData}
            />
          </Box>

          <Box sx={{ borderBottom: 1, borderColor: "divider" }}>
            <Stack
              sx={{
                flexDirection: "row",
                justifyContent: "space-between",
                alignItems: "center",
              }}
            >
              <Tabs value={tab}>
                <LinkTab
                  label="Status"
                  value={DetailTabId.Status}
                  href={`${baseUrl}&t=${DetailTabId.Status}`}
                />
                <LinkTab
                  label="Config"
                  value={DetailTabId.Config}
                  href={`${baseUrl}&t=${DetailTabId.Config}`}
                />
                <LinkTab
                  label="History"
                  value={DetailTabId.History}
                  href={`${baseUrl}&t=${DetailTabId.History}`}
                />
              </Tabs>
              {tab == DetailTabId.Config ? (
                <ConfigEditButtons group={group} name={params.name} />
              ) : null}
            </Stack>
          </Box>
        </Paper>

        {contents[tab]}
      </Stack>
    </DAGContext.Provider>
  );
}
export default DAGDetails;

interface LinkTabProps {
  label?: string;
  href?: string;
  value: string;
}

function LinkTab({ href, ...props }: LinkTabProps) {
  return (
    <a href={href}>
      <Tab {...props} />
    </a>
  );
}
