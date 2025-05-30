/**
 * FlowchartSwitch component provides a toggle for switching between horizontal and vertical flowchart layouts.
 *
 * @module features/dags/components/visualization
 */
import { ToggleButton, ToggleGroup } from '@/components/ui/toggle-group';
import { ArrowDownUp, ArrowRightLeft } from 'lucide-react';
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
      <ToggleGroup aria-label="flowchart direction">
        <ToggleButton
          value="LR"
          groupValue={value}
          onClick={() => handleChange('LR')}
          aria-label="horizontal layout"
          className="px-2 py-1 sm:w-18 w-12 cursor-pointer"
          position="first"
        >
          <div className="flex flex-col items-center">
            <ArrowRightLeft className="h-4 w-4" />
            <span className="text-xs mt-1 hidden sm:inline">Horizontal</span>
          </div>
        </ToggleButton>

        <ToggleButton
          value="TD"
          groupValue={value}
          onClick={() => handleChange('TD')}
          aria-label="vertical layout"
          className="px-2 py-1 sm:w-18 w-12 cursor-pointer"
          position="last"
        >
          <div className="flex flex-col items-center">
            <ArrowDownUp className="h-4 w-4" />
            <span className="text-xs mt-1 hidden sm:inline">Vertical</span>
          </div>
        </ToggleButton>
      </ToggleGroup>
    </div>
  );
}

export default FlowchartSwitch;
