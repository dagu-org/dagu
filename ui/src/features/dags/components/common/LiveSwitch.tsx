/**
 * LiveSwitch component provides a toggle switch for enabling/disabling DAG scheduling.
 *
 * @module features/dags/components/common
 */
import { Switch } from '@mui/material';
import React from 'react';
import { components } from '../../../../api/v2/schema';
import { useClient, useMutate } from '../../../../hooks/api';

/**
 * Props for the LiveSwitch component
 */
type Props = {
  /** Additional props to pass to the input element */
  inputProps?: React.HTMLProps<HTMLInputElement>;
  /** DAG file information */
  dag: components['schemas']['DAGFile'];
  /** Function to refresh data after toggling */
  refresh?: () => void;
};

/**
 * Switch component for toggling DAG suspension state
 * When enabled (checked), the DAG is active and can be scheduled
 * When disabled (unchecked), the DAG is suspended and won't be scheduled
 */
function LiveSwitch({ dag, refresh, inputProps }: Props) {
  const client = useClient();
  const mutate = useMutate();

  // Initialize state based on DAG suspension state
  const [checked, setChecked] = React.useState(!dag.suspended);

  /**
   * Submit the suspension state change to the API
   */
  const onSubmit = React.useCallback(
    async (suspend: boolean) => {
      const { error } = await client.POST('/dags/{fileId}/suspend', {
        params: {
          path: {
            fileId: dag.fileId,
          },
        },
        body: {
          suspend,
        },
      });
      if (error) {
        alert(error.message || 'Error occurred');
        return;
      }
      mutate(['/dags/{fileId}']);
      if (refresh) {
        refresh();
      }
    },
    [client, dag.fileId, mutate, refresh]
  );

  /**
   * Handle switch toggle
   */
  const onChange = React.useCallback(() => {
    const enabled = !checked;
    setChecked(enabled);
    onSubmit(!enabled);
  }, [checked, onSubmit]);

  return (
    <Switch checked={checked} onChange={onChange} inputProps={inputProps} />
  );
}

export default LiveSwitch;
