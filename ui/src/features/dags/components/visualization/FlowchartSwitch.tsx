/**
 * FlowchartSwitch component provides a toggle for switching between horizontal and vertical flowchart layouts.
 *
 * @module features/dags/components/visualization
 */
import { AccountTreeOutlined } from '@mui/icons-material';
import { ToggleButton, ToggleButtonGroup } from '@mui/material';
import React from 'react';
import { FlowchartType } from './';

/**
 * Props for the FlowchartSwitch component
 */
type Props = {
  /** Current flowchart direction value */
  value: FlowchartType | undefined;
  /** Callback function when direction changes */
  onChange: (value: FlowchartType) => void;
};

/**
 * FlowchartSwitch component provides toggle buttons to switch between
 * horizontal (LR) and vertical (TD) flowchart layouts
 */
function FlowchartSwitch({ value = 'TD', onChange }: Props) {
  return (
    <ToggleButtonGroup
      value={value}
      exclusive
      onChange={(_, value) => {
        onChange(value as FlowchartType);
      }}
      aria-label="flowchart direction"
    >
      <ToggleButton value="LR" aria-label="horizontal layout">
        <AccountTreeOutlined />
      </ToggleButton>
      <ToggleButton value="TD" aria-label="vertical layout">
        <AccountTreeOutlined
          style={{
            transform: 'rotate(90deg)',
          }}
        />
      </ToggleButton>
    </ToggleButtonGroup>
  );
}

export default FlowchartSwitch;
