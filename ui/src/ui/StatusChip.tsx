import React from 'react';
import { Status } from '../api/v2/schema';

type Props = {
  status?: Status;
  children: React.ReactNode; // Allow ReactNode for flexibility, though we expect string
};

function StatusChip({ status, children }: Props) {
  // Determine the background color class for the circle based on status
  let circleColorClass = '';
  let textColorClass = '';
  switch (status) {
    case Status.Success: // done -> green
      circleColorClass = 'bg-[green] dark:bg-[darkgreen]';
      textColorClass = 'text-[green] dark:text-[lightgreen]';
      break;
    case Status.Failed: // error -> red
      circleColorClass = 'bg-[red] dark:bg-[darkred]';
      textColorClass = 'text-[red] dark:text-[lightcoral]';
      break;
    case Status.Running: // running -> lime
      circleColorClass = 'bg-[lime] dark:bg-[limegreen]';
      textColorClass = 'text-[limegreen] dark:text-[lime]';
      break;
    case Status.Cancelled: // cancel -> pink
      circleColorClass = 'bg-[pink] dark:bg-[deeppink]';
      textColorClass = 'text-[deeppink] dark:text-[pink]';
      break;
    // Note: Status enum might not have Skipped, handle NotStarted
    case Status.NotStarted: // none -> lightblue
      circleColorClass = 'bg-[lightblue] dark:bg-[steelblue]';
      textColorClass = 'text-[steelblue] dark:text-[lightblue]';
      break;
    default: // Fallback to gray for any other status (including undefined)
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

export default StatusChip;
