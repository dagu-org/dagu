import React from 'react';
import { DAGContext } from '../contexts/DAGContext';
import { getEventHandlers } from '../lib/getEventHandlers';
import { NodeStatusTable, DAGStatusOverview } from './dag-details';
import { StatusUpdateModal } from './dag-execution';
import SubTitle from '../../../ui/SubTitle';
import { components, NodeStatus, Status } from '../../../api/v2/schema';
import { DAGGraph } from './visualization';
import { useClient } from '../../../hooks/api';
import { AppBarContext } from '../../../contexts/AppBarContext';
import { cn } from '@/lib/utils';

type Props = {
  run: components['schemas']['RunDetails'];
  fileId: string;
};

function DAGStatus({ run, fileId }: Props) {
  const appBarContext = React.useContext(AppBarContext);
  const [modal, setModal] = React.useState(false);
  const [selectedStep, setSelectedStep] = React.useState<
    components['schemas']['Step'] | undefined
  >(undefined);
  const client = useClient();
  const dismissModal = () => setModal(false);
  const onUpdateStatus = async (
    step: components['schemas']['Step'],
    status: NodeStatus
  ) => {
    const { error } = await client.PATCH(
      '/runs/{dagName}/{requestId}/{stepName}/status',
      {
        params: {
          path: {
            dagName: run.name,
            requestId: run.requestId,
            stepName: step.name,
          },
          query: {
            remoteNode: appBarContext.selectedRemoteNode || 'local',
          },
        },
        body: {
          status,
        },
      }
    );
    if (error) {
      alert(error.message || 'An error occurred');
      return;
    }
    dismissModal();
  };
  const onSelectStepOnGraph = React.useCallback(
    async (id: string) => {
      const status = run.status;
      if (status == Status.Running || status == Status.NotStarted) {
        return;
      }
      // find the clicked step
      const n = run.nodes.find((n) => n.step.name.replace(/\s/g, '_') == id);
      if (n) {
        setSelectedStep(n.step);
        setModal(true);
      }
    },
    [run]
  );

  const handlers = getEventHandlers(run);

  return (
    <div className="space-y-4">
      <div className="bg-white dark:bg-slate-900 rounded-xl shadow-md p-6 overflow-hidden">
        <DAGGraph run={run} onSelectStep={onSelectStepOnGraph} />
      </div>

      <DAGContext.Consumer>
        {(props) => (
          <React.Fragment>
            <div className="bg-white dark:bg-slate-900 rounded-xl shadow-md p-6 overflow-hidden">
              <SubTitle className="mb-4">Status</SubTitle>
              <DAGStatusOverview status={run} fileId={fileId} />
            </div>

            <div className="bg-white dark:bg-slate-900 rounded-xl shadow-md p-6 overflow-hidden">
              <SubTitle className="mb-4">Steps</SubTitle>
              <NodeStatusTable nodes={run.nodes} status={run} {...props} />
            </div>

            {handlers?.length ? (
              <div className="bg-white dark:bg-slate-900 rounded-xl shadow-md p-6 overflow-hidden">
                <SubTitle className="mb-4">Lifecycle Hooks</SubTitle>
                <NodeStatusTable nodes={handlers} status={run} {...props} />
              </div>
            ) : null}
          </React.Fragment>
        )}
      </DAGContext.Consumer>

      <StatusUpdateModal
        visible={modal}
        step={selectedStep}
        dismissModal={dismissModal}
        onSubmit={onUpdateStatus}
      />
    </div>
  );
}

export default DAGStatus;
