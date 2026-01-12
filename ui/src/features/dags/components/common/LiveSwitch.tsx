/**
 * LiveSwitch component provides a toggle switch for enabling/disabling DAG scheduling.
 *
 * @module features/dags/components/common
 */
import { useErrorModal } from '@/components/ui/error-modal';
import { Switch } from '@/components/ui/switch';
import { useCallback, useContext, useState } from 'react';
import { components } from '../../../../api/v2/schema';
import { useConfig } from '../../../../contexts/ConfigContext';
import { useClient } from '../../../../hooks/api';
import { AppBarContext } from '@/contexts/AppBarContext';

/**
 * Props for the LiveSwitch component
 */
type Props = {
  /** DAG file information */
  dag: components['schemas']['DAGFile'];
  /** Function to refresh data after toggling */
  refresh?: () => void;
  /** Aria label for accessibility */
  'aria-label'?: string;
};

/**
 * Switch component for toggling DAG suspension state
 * When enabled (checked), the DAG is active and can be scheduled
 * When disabled (unchecked), the DAG is suspended and won't be scheduled
 */
function LiveSwitch({ dag, refresh, 'aria-label': ariaLabel }: Props) {
  const client = useClient();
  const config = useConfig();
  const { showError } = useErrorModal();
  const [checked, setChecked] = useState(!dag.suspended);
  const appBarContext = useContext(AppBarContext);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';

  const onSubmit = useCallback(
    async (suspend: boolean) => {
      const { error } = await client.POST('/dags/{fileName}/suspend', {
        params: {
          path: {
            fileName: dag.fileName,
          },
          query: {
            remoteNode,
          },
        },
        body: {
          suspend,
        },
      });
      if (error) {
        showError(
          error.message || 'Failed to update DAG status',
          'Please try again or check the server connection.'
        );
        return;
      }
      if (refresh) {
        refresh();
      }
    },
    [client, dag.fileName, refresh, remoteNode, showError]
  );

  const handleCheckedChange = useCallback(
    (newCheckedState: boolean) => {
      setChecked(newCheckedState);
      onSubmit(!newCheckedState);
    },
    [onSubmit]
  );

  return (
    <Switch
      checked={checked}
      onCheckedChange={
        config.permissions.runDags ? handleCheckedChange : undefined
      }
      disabled={!config.permissions.runDags}
      aria-label={ariaLabel}
    />
  );
}

export default LiveSwitch;
