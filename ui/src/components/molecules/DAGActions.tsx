import { Box, Stack } from '@mui/material';
import React from 'react';
import ActionButton from '../atoms/ActionButton';
import { useNavigate } from 'react-router-dom';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faPlay, faStop, faReply } from '@fortawesome/free-solid-svg-icons';
import VisuallyHidden from '../atoms/VisuallyHidden';
import StartDAGModal from './StartDAGModal';
import ConfirmModal from './ConfirmModal';
import LabeledItem from '../atoms/LabeledItem';
import { AppBarContext } from '../../contexts/AppBarContext';
import { components } from '../../api/v2/schema';

type LabelProps = {
  show: boolean;
  children: React.ReactNode;
};

type Props = {
  status?:
    | components['schemas']['RunSummary']
    | components['schemas']['RunDetails'];
  location: string;
  dag:
    | components['schemas']['DAG']
    | components['schemas']['DAGDetails']
    | undefined;
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
  location,
  dag,
  refresh,
  redirectTo,
  label = true,
}: Props) {
  const nav = useNavigate();
  const appBarContext = React.useContext(AppBarContext);

  const [isStartModal, setIsStartModal] = React.useState(false);
  const [isStopModal, setIsStopModal] = React.useState(false);
  const [isRetryModal, setIsRetryModal] = React.useState(false);

  const onSubmit = React.useCallback(
    async (params: {
      name: string;
      action: string;
      requestId?: string;
      params?: string;
    }) => {
      const url = `${getConfig().apiURL}/dags/${params.name}?remoteNode=${
        appBarContext.selectedRemoteNode || 'local'
      }`;
      const ret = await fetch(url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        mode: 'cors',
        body: JSON.stringify(params),
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

  const buttonState = {
    start: status?.status != 1,
    stop: status?.status == 1,
    retry: status?.status != 1 && status?.requestId != '',
  };
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
        onClick={() => setIsStartModal(true)}
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
        onClick={() => setIsStopModal(true)}
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
        onClick={() => setIsRetryModal(true)}
      >
        {label && 'Retry'}
      </ActionButton>
      <ConfirmModal
        title="Confirmation"
        buttonText="Stop"
        visible={isStopModal}
        dismissModal={() => setIsStopModal(false)}
        onSubmit={() => {
          setIsStopModal(false);
          onSubmit({ name: location, action: 'stop' });
        }}
      >
        <Box>Do you really want to cancel the DAG?</Box>
      </ConfirmModal>
      <ConfirmModal
        title="Confirmation"
        buttonText="Rerun"
        visible={isRetryModal}
        dismissModal={() => setIsRetryModal(false)}
        onSubmit={() => {
          setIsRetryModal(false);
          onSubmit({
            name: location,
            action: 'retry',
            requestId: status?.requestId,
          });
        }}
      >
        <Stack direction="column">
          <Box>Do you really want to rerun the following execution?</Box>
          <LabeledItem label="Request-ID">{null}</LabeledItem>
          <Box>{status?.requestId}</Box>
        </Stack>
      </ConfirmModal>
      {dag && (
        <StartDAGModal
          dag={dag}
          visible={isStartModal}
          onSubmit={(params) => {
            setIsStartModal(false);
            onSubmit({ name: location, action: 'start', params: params });
          }}
          dismissModal={() => {
            setIsStartModal(false);
          }}
        />
      )}
    </Stack>
  );
}
export default DAGActions;
