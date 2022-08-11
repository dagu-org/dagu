import React from 'react';
import { useParams } from 'react-router-dom';
import { GetDAGResponse } from '../../../api/DAG';
import DAGSpecErrors from '../../../components/molecules/DAGSpecErrors';
import DAGStatus from '../../../components/organizations/DAGStatus';
import { DAGContext } from '../../../contexts/DAGContext';
import { DetailTabId } from '../../../models';
import DAGSpec from '../../../components/organizations/DAGSpec';
import DAGHistory from '../../../components/organizations/ExecutionHistory';
import ExecutionLog from '../../../components/organizations/ExecutionLog';
import { Box, Stack, Tab, Tabs } from '@mui/material';
import Title from '../../../components/atoms/Title';
import DAGActions from '../../../components/molecules/DAGActions';
import DAGEditButtons from '../../../components/molecules/DAGEditButtons';
import LoadingIndicator from '../../../components/atoms/LoadingIndicator';
import { AppBarContext } from '../../../contexts/AppBarContext';

type Params = {
  name: string;
};

function DAGDetails() {
  const params = useParams<Params>();
  const [data, setData] = React.useState<GetDAGResponse | undefined>(undefined);
  const [tab, setTab] = React.useState(DetailTabId.Status);
  const appBarContext = React.useContext(AppBarContext);
  React.useEffect(() => {
    const urlParams = new URLSearchParams(window.location.search);
    const t = urlParams.get('t');
    if (t) {
      setTab(t as DetailTabId);
    }
  }, []);
  async function getData() {
    let url = API_URL + `/dags/${params.name}?format=json`;
    const urlParams = new URLSearchParams(window.location.search);
    url += '&' + urlParams.toString();
    const resp = await fetch(url, {
      method: 'GET',
      cache: 'no-store',
      mode: 'cors',
      headers: {
        Accept: 'application/json',
      },
    });
    if (!resp.ok) {
      return;
    }
    const body = await resp.json();
    setData(body);
  }
  React.useEffect(() => {
    if (data) {
      appBarContext.setTitle(data.Title);
    }
  }, [data, appBarContext]);
  React.useEffect(() => {
    getData();
    if (tab == DetailTabId.Status || tab == DetailTabId.Spec) {
      const timer = setInterval(getData, 2000);
      return () => clearInterval(timer);
    }
  }, [tab]);

  if (!params.name || !data || !data.DAG) {
    return <LoadingIndicator />;
  }

  const contents: Partial<{
    [key in DetailTabId]: React.ReactNode;
  }> = {
    [DetailTabId.Status]: (
      <DAGStatus DAG={data.DAG} name={params.name} refresh={getData} />
    ),
    [DetailTabId.Spec]: <DAGSpec data={data} />,
    [DetailTabId.History]: <DAGHistory logData={data.LogData} />,
    [DetailTabId.StepLog]: <ExecutionLog log={data.StepLog} />,
    [DetailTabId.ScLog]: <ExecutionLog log={data.ScLog} />,
  };
  const ctx = {
    data: data,
    refresh: getData,
    tab,
    name: params.name,
  };

  const baseUrl = `/dags/${encodeURI(params.name)}`;

  return (
    <DAGContext.Provider value={ctx}>
      <Stack
        sx={{
          width: '100%',
          direction: 'column',
        }}
      >
        <Box
          sx={{
            mx: 4,
            display: 'flex',
            flexDirection: 'row',
            alignItems: 'center',
            justifyContent: 'space-between',
          }}
        >
          <Title>{data.Title}</Title>
          {tab == DetailTabId.Status || tab == DetailTabId.Spec ? (
            <DAGActions
              status={data.DAG.Status}
              name={params.name!}
              refresh={getData}
              redirectTo={
                tab == DetailTabId.Spec
                  ? `${baseUrl}?t=${DetailTabId.Status}`
                  : undefined
              }
            />
          ) : null}
        </Box>

        <Stack
          sx={{
            mx: 4,
            flexDirection: 'row',
            justifyContent: 'space-between',
            alignItems: 'center',
          }}
        >
          <Tabs value={tab}>
            <LinkTab
              label="Status"
              value={DetailTabId.Status}
              href={`${baseUrl}?t=${DetailTabId.Status}`}
            />
            <LinkTab
              label="Spec"
              value={DetailTabId.Spec}
              href={`${baseUrl}?t=${DetailTabId.Spec}`}
            />
            <LinkTab
              label="History"
              value={DetailTabId.History}
              href={`${baseUrl}?t=${DetailTabId.History}`}
            />
            {tab >= DetailTabId.StepLog && tab <= DetailTabId.ScLog ? (
              <LinkTab label="Log" value={tab} />
            ) : null}
          </Tabs>
          {tab == DetailTabId.Spec ? (
            <DAGEditButtons name={params.name} />
          ) : null}
        </Stack>

        <Box sx={{ mt: 2, mx: 4 }}>
          <DAGSpecErrors errors={data.Errors} />
        </Box>

        <Box sx={{ mx: 4, flex: 1 }}>{contents[tab]}</Box>
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
