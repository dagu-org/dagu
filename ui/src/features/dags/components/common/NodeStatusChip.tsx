/**
 * NodeStatusChip component displays a chip with appropriate styling based on node status.
 *
 * @module features/dags/components/common
 */
import React from 'react';
import { NodeStatus } from '../../../../api/v2/schema';

/**
 * Props for the NodeStatusChip component
 */
type Props = {
  /** Status code of the node */
  status: NodeStatus;
  /** Text to display in the chip */
  children: React.ReactNode; // Allow ReactNode for flexibility
};

/**
 * NodeStatusChip displays a styled badge based on the node status
 */
function NodeStatusChip({ status, children }: Props) {
  // Determine the background color class for the circle based on status
  let circleColorClass = '';
  let textColorClass = '';
  switch (status) {
    case NodeStatus.Success: // done -> green
      circleColorClass = 'bg-[green] dark:bg-[darkgreen]';
      textColorClass = 'text-[green] dark:text-[lightgreen]';
      break;
    case NodeStatus.Failed: // error -> red
      circleColorClass = 'bg-[red] dark:bg-[darkred]';
      textColorClass = 'text-[red] dark:text-[lightcoral]'; // Use lightcoral for dark mode text
      break;
    case NodeStatus.Running: // running -> lime
      circleColorClass = 'bg-[lime] dark:bg-[limegreen]';
      textColorClass = 'text-[limegreen] dark:text-[lime]'; // Use limegreen/lime for text
      break;
    case NodeStatus.Cancelled: // cancel -> pink
      circleColorClass = 'bg-[pink] dark:bg-[deeppink]';
      textColorClass = 'text-[deeppink] dark:text-[pink]'; // Use deeppink/pink for text
      break;
    case NodeStatus.Skipped: // skipped -> gray
      circleColorClass = 'bg-[gray] dark:bg-[darkgray]';
      textColorClass = 'text-[gray] dark:text-[lightgray]';
      break;
    case NodeStatus.NotStarted: // none -> lightblue
      circleColorClass = 'bg-[lightblue] dark:bg-[steelblue]';
      textColorClass = 'text-[steelblue] dark:text-[lightblue]'; // Use steelblue/lightblue for text
      break;
    default: // Fallback to gray
      circleColorClass = 'bg-[gray] dark:bg-[darkgray]';
      textColorClass = 'text-[gray] dark:text-[lightgray]';
  }

  // Capitalize first letter if children is a string
  const displayChildren =
    typeof children === 'string'
      ? children.charAt(0).toUpperCase() + children.slice(1)
      : children;

  // Render a div with a colored circle and the text
  return (
    <div className="inline-flex items-center">
      <span
        className={`inline-block h-2.5 w-2.5 rounded-full mr-2 ${circleColorClass}`} // Increased size and margin
        aria-hidden="true"
      ></span>
      <span className={`text-xs font-medium ${textColorClass}`}>
        {displayChildren}
      </span>{' '}
      {/* Added text color */}
    </div>
  );
}

export default NodeStatusChip;
