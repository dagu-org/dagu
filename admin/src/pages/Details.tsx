import React from "react";
import { useParams } from "react-router-dom";
import { GetWorkflowResponse } from "../api/Workflow";
import ConfigErrors from "../components/ConfigErrors";
import WorkflowTabStatus from "../components/WorkflowTabStatus";
import WorkflowSubTabs from "../components/WorkflowSubTabs";
import WorkflowTabs from "../components/WorkflowTabs";
import WorkflowTitle from "../components/WorkflowTitle";
import { WorkflowContext } from "../contexts/WorkflowContext";
import { WorkflowTabType } from "../models/WorkflowTab";
import WorkflowTabConfig from "../components/WorkflowTabConfig";
import WorkflowTabHist from "../components/WorkflowTabHist";
import WorkflowTabLog from "../components/WorkflowTabLog";

type Params = {
  name: string;
};

function DetailsPage() {
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
  const [sub, setSub] = React.useState(0);
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
  if (!data || !data.DAG) {
    return <div>Loading...</div>;
  }
  const contents: Partial<{
    [key in WorkflowTabType]: React.ReactNode;
  }> = {
    [WorkflowTabType.Status]: (
      <WorkflowTabStatus workflow={data.DAG} subtab={sub} />
    ),
    [WorkflowTabType.Config]: <WorkflowTabConfig data={data} />,
    [WorkflowTabType.History]: <WorkflowTabHist logData={data.LogData} />,
    [WorkflowTabType.StepLog]: <WorkflowTabLog log={data.StepLog} />,
    [WorkflowTabType.ScLog]: <WorkflowTabLog log={data.ScLog} />,
  };
  const ctx = {
    data: data,
    refresh: getData,
    tab,
    group,
    name: params.name!,
  };
  return (
    <WorkflowContext.Provider value={ctx}>
      <ConfigErrors errors={data.Errors}></ConfigErrors>
      <WorkflowTitle title={data.Title}></WorkflowTitle>
      <WorkflowTabs tab={tab} group={group}></WorkflowTabs>
      <WorkflowSubTabs
        tab={tab}
        active={sub}
        setActive={setSub}
      ></WorkflowSubTabs>
      <div className="mx-5 mt-5">{contents[tab]}</div>
    </WorkflowContext.Provider>
  );
}
export default DetailsPage;
