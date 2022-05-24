import React from "react";
import { GetListResponse } from "../api/List";
import WorkflowErrors from "../components/WorkflowErrors";
import Header from "../components/Header";
import WithLoading from "../components/WithLoading";
import WorkflowTable from "../components/WorkflowTable";

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
    <WithLoading loaded={!!data}>
      <Header refresh={getData}></Header>
      <div className="mx-5 mt-5">
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
      </div>
    </WithLoading>
  );
}
export default WorkflowsPage;
