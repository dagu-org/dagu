/**
 * FlowchartSwitch component provides a toggle for switching between horizontal and vertical flowchart layouts.
 *
 * @module features/dags/components/visualization
 */
import { ArrowRightLeft, ArrowDownUp } from 'lucide-react';
import React from 'react';
import { ToggleGroup, ToggleButton } from '@/components/ui/toggle-group';
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
  const handleChange = (newValue: string) => {
    onChange(newValue as FlowchartType);
  };

  return (
    <div className="flex flex-col items-start">
      <ToggleGroup
        value={value}
        onChange={handleChange}
        aria-label="flowchart direction"
      >
        <ToggleButton
          value="LR"
          groupValue={value}
          onClick={() => handleChange('LR')}
          aria-label="horizontal layout"
          className="px-2 py-1 w-18"
          position="first"
        >
          <div className="flex flex-col items-center">
            <ArrowRightLeft className="h-4 w-4" />
            <span className="text-xs mt-1">Horizontal</span>
          </div>
        </ToggleButton>

        <ToggleButton
          value="TD"
          groupValue={value}
          onClick={() => handleChange('TD')}
          aria-label="vertical layout"
          className="px-2 py-1 w-18"
          position="last"
        >
          <div className="flex flex-col items-center">
            <ArrowDownUp className="h-4 w-4" />
            <span className="text-xs mt-1">Vertical</span>
          </div>
        </ToggleButton>
      </ToggleGroup>
    </div>
  );
}

export default FlowchartSwitch;
