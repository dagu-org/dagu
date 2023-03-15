import { Stack } from '@mui/material';
import React from 'react';
import { DAG, Parameters, SchedulerStatus, Status } from '../../models';
import ActionButton from '../atoms/ActionButton';
import { useNavigate } from 'react-router-dom';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faPlay, faStop, faReply } from '@fortawesome/free-solid-svg-icons';
import VisuallyHidden from '../atoms/VisuallyHidden';
import StartDAGModal from './StartDAGModal';

type LabelProps = {
  show: boolean;
  children: React.ReactNode;
};

type Props = {
  status?: Status;
  name: string;
  dag: DAG;
  label?: boolean;
  redirectTo?: string;
  refresh?: () => void;
};

function Label({ show, children }: LabelProps): JSX.Element {
  if (show) return <>{children}</>;
  return <VisuallyHidden>{children}</VisuallyHidden>;
}

function DAGActions({
  status,
  name,
  dag,
  refresh,
  redirectTo,
  label = true,
}: Props) {
  const nav = useNavigate();

  const [isRunModalVisible, setIsRunModalVisible] = React.useState(false);

  const onSubmit = React.useCallback(
    async (
      warn: string,
      params: {
        name: string;
        action: string;
        requestId?: string;
        params?: Parameters;
      }
    ) => {
      const form = new FormData();
      if (params.action == 'start') {
        form.set('params', params.params!.Parameters);
      }
      if (warn != '' && !confirm(warn)) {
        return;
      }
      form.set('action', params.action);
      if (params.requestId) {
        form.set('request-id', params.requestId);
      }
      const url = `${API_URL}/dags/${params.name}`;
      const ret = await fetch(url, {
        method: 'POST',
        mode: 'cors',
        body: form,
      });
      if (redirectTo) {
        nav(redirectTo);
        refresh && refresh();
        return;
      }
      if (!ret.ok) {
        const e = await ret.text();
        alert(e || 'Failed to submit');
      }
      refresh && refresh();
    },
    [refresh]
  );

  const onRunTheDAG = React.useCallback(
    async (params: Parameters) => {
      onSubmit('', { name: name, action: 'start', params: params });
    },
    [onSubmit]
  );

  const buttonState = React.useMemo(
    () => ({
      start: status?.Status != SchedulerStatus.Running,
      stop: status?.Status == SchedulerStatus.Running,
      retry:
        status?.Status != SchedulerStatus.Running && status?.RequestId != '',
    }),
    [status]
  );
  return (
    <Stack direction="row" spacing={2}>
      <ActionButton
        label={label}
        icon={
          <>
            <Label show={false}>Start</Label>
            <span className="icon">
              <FontAwesomeIcon icon={faPlay} />
            </span>
          </>
        }
        disabled={!buttonState['start']}
        onClick={() => setIsRunModalVisible(true)}
      >
        {label && 'Start'}
      </ActionButton>
      <ActionButton
        label={label}
        icon={
          <>
            <Label show={false}>Stop</Label>
            <span className="icon">
              <FontAwesomeIcon icon={faStop} />
            </span>
          </>
        }
        disabled={!buttonState['stop']}
        onClick={() =>
          onSubmit('Do you really want to cancel the DAG?', {
            name: name,
            action: 'stop',
          })
        }
      >
        {label && 'Stop'}
      </ActionButton>
      <ActionButton
        label={label}
        icon={
          <>
            <Label show={false}>Retry</Label>
            <span className="icon">
              <FontAwesomeIcon icon={faReply} />
            </span>
          </>
        }
        disabled={!buttonState['retry']}
        onClick={() =>
          onSubmit(
            `Do you really want to rerun the last execution (${status?.RequestId}) ?`,
            {
              name: name,
              requestId: status?.RequestId,
              action: 'retry',
            }
          )
        }
      >
        {label && 'Retry'}
      </ActionButton>
      <StartDAGModal
        dag={dag}
        visible={isRunModalVisible}
        onSubmit={(params) => {
          setIsRunModalVisible(false);
          onRunTheDAG(params);
        }}
        dismissModal={() => {
          setIsRunModalVisible(false);
        }}
      />
    </Stack>
  );
}
export default DAGActions;
