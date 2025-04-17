/**
 * DAGActions component provides action buttons for DAG operations (start, stop, retry).
 *
 * @module features/dags/components/common
 */
import { Box, Stack } from '@mui/material';
import React from 'react';
import ActionButton from '../../../../ui/ActionButton';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faPlay, faStop, faReply } from '@fortawesome/free-solid-svg-icons';
import VisuallyHidden from '../../../../ui/VisuallyHidden';
import { StartDAGModal } from '../dag-execution';
import ConfirmModal from '../../../../ui/ConfirmModal';
import LabeledItem from '../../../../ui/LabeledItem';
import { components } from '../../../../api/v2/schema';
import { useClient, useMutate } from '../../../../hooks/api';
import { AppBarContext } from '../../../../contexts/AppBarContext';

/**
 * Props for the Label component
 */
type LabelProps = {
  /** Whether to show the label text */
  show: boolean;
  /** Label content */
  children: React.ReactNode;
};

/**
 * Props for the DAGActions component
 */
type Props = {
  /** Current status of the DAG */
  status?:
    | components['schemas']['RunSummary']
    | components['schemas']['RunDetails'];
  /** File ID of the DAG */
  fileId: string;
  /** DAG definition */
  dag?: components['schemas']['DAG'] | components['schemas']['DAGDetails'];
  /** Whether to show text labels on buttons */
  label?: boolean;
  /** Function to refresh data after actions */
  refresh?: () => void;
};

/**
 * Helper component to handle accessibility for button labels
 */
function Label({ show, children }: LabelProps): React.JSX.Element {
  if (show) return <>{children}</>;
  return <VisuallyHidden>{children}</VisuallyHidden>;
}

/**
 * DAGActions component provides buttons to start, stop, and retry DAG executions
 */
function DAGActions({ status, fileId, dag, refresh, label = true }: Props) {
  const appBarContext = React.useContext(AppBarContext);
  const [isStartModal, setIsStartModal] = React.useState(false);
  const [isStopModal, setIsStopModal] = React.useState(false);
  const [isRetryModal, setIsRetryModal] = React.useState(false);

  const client = useClient();
  const mutate = useMutate();

  /**
   * Reload DAG data after an action is performed
   */
  const reloadData = () => {
    mutate(['/dags/{fileId}']);
    mutate(['/dags/{fileId}/runs']);
    refresh && refresh();
  };

  // Determine which buttons should be enabled based on current status
  const buttonState = {
    start: status?.status != 1,
    stop: status?.status == 1,
    retry: status?.status != 1 && status?.requestId != '',
  };

  if (!dag) {
    return <></>;
  }

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
        onSubmit={async () => {
          setIsStopModal(false);
          const { error } = await client.POST('/dags/{fileId}/stop', {
            params: {
              query: {
                remoteNode: appBarContext.selectedRemoteNode || 'local',
              },
              path: {
                fileId: fileId,
              },
            },
          });
          if (error) {
            alert(error.message || 'An error occurred');
            return;
          }
          reloadData();
        }}
      >
        <Box>Do you really want to cancel the DAG?</Box>
      </ConfirmModal>
      <ConfirmModal
        title="Confirmation"
        buttonText="Rerun"
        visible={isRetryModal}
        dismissModal={() => setIsRetryModal(false)}
        onSubmit={async () => {
          setIsRetryModal(false);
          const { error } = await client.POST('/dags/{fileId}/retry', {
            params: {
              path: {
                fileId: fileId,
              },
              query: {
                remoteNode: appBarContext.selectedRemoteNode || 'local',
              },
            },
            body: {
              requestId: status?.requestId || '',
            },
          });
          if (error) {
            alert(error.message || 'An error occurred');
            return;
          }
          reloadData();
        }}
      >
        <Stack direction="column">
          <Box>Do you really want to rerun the following execution?</Box>
          <LabeledItem label="Request-ID">{null}</LabeledItem>
          <Box>{status?.requestId}</Box>
        </Stack>
      </ConfirmModal>
      <StartDAGModal
        dag={dag}
        visible={isStartModal}
        onSubmit={async (params) => {
          setIsStartModal(false);
          const { error } = await client.POST('/dags/{fileId}/start', {
            params: {
              path: {
                fileId: fileId,
              },
              query: {
                remoteNode: appBarContext.selectedRemoteNode || 'local',
              },
            },
            body: {
              params: params,
            },
          });
          if (error) {
            alert(error.message || 'An error occurred');
            return;
          }
          reloadData();
        }}
        dismissModal={() => {
          setIsStartModal(false);
        }}
      />
    </Stack>
  );
}

export default DAGActions;
