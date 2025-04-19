import React, { useMemo } from 'react';
import { useParams, useLocation } from 'react-router-dom';
import { DAGStatus } from '../../../features/dags/components';
import { DAGContext } from '../../../features/dags/contexts/DAGContext';
import { DAGSpec } from '../../../features/dags/components/dag-editor';
import { LinkTab } from '../../../features/dags/components/common';
import { Tabs } from '@/components/ui/tabs';
import { DAGEditButtons } from '../../../features/dags/components/dag-editor';
import LoadingIndicator from '../../../ui/LoadingIndicator';
import { AppBarContext } from '../../../contexts/AppBarContext';
import moment from 'moment-timezone';
import { RunDetailsContext } from '../../../features/dags/contexts/DAGStatusContext';
import { useQuery } from '../../../hooks/api';
import { components, Status } from '../../../api/v2/schema';
import {
  DAGExecutionHistory,
  ExecutionLog,
  StepLog,
} from '../../../features/dags/components/dag-execution';
import { DAGHeader } from '../../../features/dags/components/dag-details';
import { ActivitySquare, FileCode, History, ScrollText } from 'lucide-react';

type Params = {
  fileId: string;
  name: string;
  tab?: string;
};

function DAGDetails() {
  const params = useParams<Params>();
  const appBarContext = React.useContext(AppBarContext);
  const { pathname } = useLocation();
  const { data, isLoading, mutate } = useQuery(
    '/dags/{fileId}',
    {
      params: {
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
        },
        path: {
          fileId: params.fileId || '',
        },
      },
    },
    { refreshInterval: 2000 }
  );
  const baseUrl = `/dags/${params.fileId}`;
  const [currentRun, setCurrentRun] = React.useState<
    components['schemas']['RunDetails'] | undefined
  >();
  const query = new URLSearchParams(window.location.search);
  const requestId =
    query.get('requestId') || data?.latestRun?.requestId || 'latest';
  const stepName = query.get('step');

  const refreshFn = React.useCallback(() => {
    setTimeout(() => mutate(), 500);
  }, [mutate, params.fileId]);

  React.useEffect(() => {
    if (data) {
      appBarContext.setTitle(data.dag?.name || '');
      setCurrentRun(data.latestRun);
    }
  }, [data, appBarContext]);

  const tab = useMemo(() => {
    return params.tab || 'status';
  }, [params]);

  const formatDuration = (startDate: string, endDate: string) => {
    if (!startDate || !endDate) return '--';
    const duration = moment.duration(moment(endDate).diff(moment(startDate)));
    const hours = Math.floor(duration.asHours());
    const minutes = duration.minutes();
    const seconds = duration.seconds();

    if (hours > 0) {
      return `${hours}h ${minutes}m ${seconds}s`;
    } else if (minutes > 0) {
      return `${minutes}m ${seconds}s`;
    }
    return `${seconds}s`;
  };

  if (!params.fileId || isLoading || !data || !data.latestRun) {
    return <LoadingIndicator />;
  }

  return (
    <DAGContext.Provider
      value={{
        refresh: refreshFn,
        fileId: params.fileId || '',
        name: data.dag?.name || '',
      }}
    >
      <RunDetailsContext.Provider
        value={{
          data: currentRun,
          setData: (status: components['schemas']['RunDetails']) => {
            setCurrentRun(status);
          },
        }}
      >
        <div className="w-full flex flex-col">
          <DAGHeader
            dag={data.dag}
            latestRun={data.latestRun}
            fileId={params.fileId || ''}
            refreshFn={refreshFn}
            formatDuration={formatDuration}
          />
          <div className="mx-4 my-4 flex flex-row justify-between items-center">
            <Tabs
              value={pathname}
              className="bg-white p-1.5 rounded-lg shadow-sm border border-gray-100/80"
            >
              <LinkTab
                label="Status"
                value={`${baseUrl}`}
                isActive={pathname === `${baseUrl}`}
                icon={ActivitySquare}
              />
              <LinkTab
                label="Spec"
                value={`${baseUrl}/spec`}
                isActive={pathname === `${baseUrl}/spec`}
                icon={FileCode}
              />
              <LinkTab
                label="History"
                value={`${baseUrl}/history`}
                isActive={pathname === `${baseUrl}/history`}
                icon={History}
              />
              {pathname === `${baseUrl}/log` ||
              pathname === `${baseUrl}/scheduler-log` ? (
                <LinkTab
                  label="Log"
                  value={pathname}
                  isActive={true}
                  icon={ScrollText}
                />
              ) : null}
            </Tabs>
            {pathname === `${baseUrl}/spec` ? (
              <DAGEditButtons fileId={params.fileId || ''} />
            ) : null}
          </div>
          <div className="mx-4 flex-1">
            {tab == 'status' ? (
              <DAGStatus run={data.latestRun} fileId={params.fileId || ''} />
            ) : null}
            {tab == 'spec' ? <DAGSpec fileId={params.fileId} /> : null}
            {tab == 'history' ? (
              <DAGExecutionHistory fileId={params.fileId || ''} />
            ) : null}
            {tab == 'scheduler-log' ? (
              <ExecutionLog name={data.dag?.name || ''} requestId={requestId} />
            ) : null}
            {tab == 'log' && stepName ? (
              <StepLog
                dagName={data.dag?.name || ''}
                requestId={requestId}
                stepName={stepName}
              />
            ) : null}
          </div>
        </div>
      </RunDetailsContext.Provider>
    </DAGContext.Provider>
  );
}
export default DAGDetails;
