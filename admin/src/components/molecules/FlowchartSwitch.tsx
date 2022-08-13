import { AccountTreeOutlined } from '@mui/icons-material';
import { ToggleButton, ToggleButtonGroup } from '@mui/material';
import React from 'react';
import { FlowchartType } from './Graph';

type Props = {
  value: FlowchartType | undefined;
  onChange: (value: FlowchartType) => void;
};

function FlowchartSwitch({ value = 'TD', onChange }: Props) {
  return (
    <ToggleButtonGroup
      value={value}
      exclusive
      onChange={(_, value) => {
        onChange(value as FlowchartType);
      }}
    >
      <ToggleButton value="LR" aria-label="horizontal">
        <AccountTreeOutlined />
      </ToggleButton>
      <ToggleButton value="TD" aria-label="vertical">
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
