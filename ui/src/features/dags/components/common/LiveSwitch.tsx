/**
 * LiveSwitch component provides a toggle switch for enabling/disabling DAG scheduling.
 *
 * @module features/dags/components/common
 */
import { Switch } from '@/components/ui/switch'; // Import Shadcn Switch
import React from 'react';
import { components } from '../../../../api/v2/schema';
import { useConfig } from '../../../../contexts/ConfigContext';
import { useClient } from '../../../../hooks/api';

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

  // Initialize state based on DAG suspension state
  const [checked, setChecked] = React.useState(!dag.suspended);

  /**
   * Submit the suspension state change to the API
   */
  const onSubmit = React.useCallback(
    async (suspend: boolean) => {
      const { error } = await client.POST('/dags/{fileName}/suspend', {
        params: {
          path: {
            fileName: dag.fileName,
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
      if (refresh) {
        refresh();
      }
    },
    [client, dag.fileName, refresh]
  );

  /**
   * Handle switch toggle
   */
  /**
   * Handle switch toggle using onCheckedChange
   */
  const handleCheckedChange = React.useCallback(
    (newCheckedState: boolean) => {
      setChecked(newCheckedState);
      onSubmit(!newCheckedState); // onSubmit expects the 'suspend' value
    },
    [onSubmit] // checked is implicitly handled by newCheckedState
  );

  return (
    <Switch
      checked={checked}
      onCheckedChange={config.permissions.runDags ? handleCheckedChange : undefined}
      disabled={!config.permissions.runDags}
      aria-label={ariaLabel} // Pass aria-label directly
      // Add custom styling for better visibility in both states
      className="data-[state=checked]:bg-emerald-700 dark:data-[state=checked]:bg-emerald-800 data-[state=unchecked]:bg-gray-300 dark:data-[state=unchecked]:bg-gray-600 cursor-pointer"
    />
  );
}

export default LiveSwitch;
