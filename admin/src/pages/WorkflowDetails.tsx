import React from "react";
import { useParams } from "react-router-dom";
import { GetWorkflowResponse } from "../api/Workflow";
import ConfigErrors from "../components/ConfigErrors";
import WorkflowStatus from "../components/WorkflowStatus";
import { WorkflowContext } from "../contexts/WorkflowContext";
import { WorkflowTabType } from "../models/Workflow";
import WorkflowConfig from "../components/WorkflowConfig";
import WorkflowHistory from "../components/WorkflowHistory";
import WorkflowLog from "../components/WorkflowLog";
import { Box, Paper, Stack, Tab, Tabs } from "@mui/material";
import Title from "../components/Title";
import WorkflowActions from "../components/WorkflowActions";
import ConfigEditButtons from "../components/ConfigEditButtons";
import Loading from "../components/Loading";

type Params = {
  name: string;
};

function WorkflowDetail() {
  const params = useParams<Params>();
  const [data, setData] = React.useState<GetWorkflowResponse | undefined>(
    undefined
  );
  const [tab, setTab] = React.useState(WorkflowTabType.Status);
  const [group, setGroup] = React.useState("");
  React.useEffect(() => {
    const urlParams = new URLSearchParams(window.location.search);
    let t = urlParams.get("t");
    if (t) {
      setTab(t as WorkflowTabType);
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
    if (tab == WorkflowTabType.Status || tab == WorkflowTabType.Config) {
      const timer = setInterval(getData, 2000);
      return () => clearInterval(timer);
    }
  }, [tab]);

  if (!params.name || !data || !data.DAG) {
    return <Loading />;
  }

  const contents: Partial<{
    [key in WorkflowTabType]: React.ReactNode;
  }> = {
    [WorkflowTabType.Status]: (
      <WorkflowStatus
        workflow={data.DAG}
        group={group}
        name={params.name}
        refresh={getData}
      />
    ),
    [WorkflowTabType.Config]: <WorkflowConfig data={data} />,
    [WorkflowTabType.History]: <WorkflowHistory logData={data.LogData} />,
    [WorkflowTabType.StepLog]: <WorkflowLog log={data.StepLog} />,
    [WorkflowTabType.ScLog]: <WorkflowLog log={data.ScLog} />,
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
    <WorkflowContext.Provider value={ctx}>
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
            <WorkflowActions
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
                  value={WorkflowTabType.Status}
                  href={`${baseUrl}&t=${WorkflowTabType.Status}`}
                />
                <LinkTab
                  label="Config"
                  value={WorkflowTabType.Config}
                  href={`${baseUrl}&t=${WorkflowTabType.Config}`}
                />
                <LinkTab
                  label="History"
                  value={WorkflowTabType.History}
                  href={`${baseUrl}&t=${WorkflowTabType.History}`}
                />
              </Tabs>
              {tab == WorkflowTabType.Config ? (
                <ConfigEditButtons group={group} name={params.name} />
              ) : null}
            </Stack>
          </Box>
        </Paper>

        {contents[tab]}
      </Stack>
    </WorkflowContext.Provider>
  );
}
export default WorkflowDetail;

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
