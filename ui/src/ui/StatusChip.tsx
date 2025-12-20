import { cn } from '@/lib/utils';
import React from 'react';
import { Status } from '../api/v2/schema';

type Props = {
  status?: Status;
  children: React.ReactNode; // Allow ReactNode for flexibility, though we expect string
  size?: 'xs' | 'sm' | 'md' | 'lg'; // Size variants
};

function StatusChip({ status, children, size = 'md' }: Props) {
  // Determine the colors and icon based on status
  let bgColorClass = '';
  let textColorClass = '';
  let borderColorClass = '';
  switch (status) {
    case Status.Success: // done -> muted green (sepia-compatible)
      bgColorClass = 'bg-[rgba(107,168,107,0.12)] dark:bg-[rgba(125,168,125,0.18)]';
      borderColorClass = 'border-[#6ba86b] dark:border-[#7da87d]';
      textColorClass = 'text-[#5a8a5a] dark:text-[#8ac48a]';
      break;
    case Status.Failed: // error -> warm coral red (sepia-compatible)
      bgColorClass = 'bg-[rgba(196,114,106,0.12)] dark:bg-[rgba(196,114,106,0.18)]';
      borderColorClass = 'border-[#c4726a] dark:border-[#d4847a]';
      textColorClass = 'text-[#b05a52] dark:text-[#e89a92]';
      break;
    case Status.Running: // running -> accent green (sepia-compatible)
      bgColorClass = 'bg-[rgba(125,168,125,0.15)] dark:bg-[rgba(138,196,138,0.2)]';
      borderColorClass = 'border-[#7da87d] dark:border-[#8ac48a]';
      textColorClass = 'text-[#6b9a6b] dark:text-[#9ad49a]';
      break;
    case Status.Aborted: // aborted -> muted coral/pink (sepia-compatible)
      bgColorClass = 'bg-[rgba(212,132,122,0.12)] dark:bg-[rgba(212,132,122,0.18)]';
      borderColorClass = 'border-[#d4847a] dark:border-[#e4948a]';
      textColorClass = 'text-[#c06a62] dark:text-[#f0a49a]';
      break;
    case Status.NotStarted: // none -> slate blue (sepia-compatible)
      bgColorClass = 'bg-[rgba(138,159,196,0.12)] dark:bg-[rgba(138,159,196,0.18)]';
      borderColorClass = 'border-[#8a9fc4] dark:border-[#9aafda]';
      textColorClass = 'text-[#6a7fa4] dark:text-[#aabfea]';
      break;
    case Status.Queued: // queued -> muted purple (sepia-compatible)
      bgColorClass = 'bg-[rgba(154,122,196,0.12)] dark:bg-[rgba(154,122,196,0.18)]';
      borderColorClass = 'border-[#9a7ac4] dark:border-[#aa8ad4]';
      textColorClass = 'text-[#7a5aa4] dark:text-[#ba9ae4]';
      break;
    case Status.PartialSuccess: // partial success -> warm amber (sepia-compatible)
      bgColorClass = 'bg-[rgba(212,148,106,0.12)] dark:bg-[rgba(212,148,106,0.18)]';
      borderColorClass = 'border-[#d4946a] dark:border-[#e4a47a]';
      textColorClass = 'text-[#c47a4a] dark:text-[#f4b48a]';
      break;
    default: // Fallback to warm gray (sepia-compatible)
      bgColorClass = 'bg-[rgba(168,160,152,0.12)] dark:bg-[rgba(168,160,152,0.18)]';
      borderColorClass = 'border-[#a8a098] dark:border-[#b8b0a8]';
      textColorClass = 'text-[#6b635a] dark:text-[#c8c0b8]';
  }

  // Size classes
  const sizeClasses = {
    xs: 'text-[10px] py-0 px-1.5',
    sm: 'text-xs py-0.5 px-2',
    md: 'text-sm py-1 px-3',
    lg: 'text-base py-1.5 px-4',
  };

  // Render a pill-shaped badge with icon and text
  return (
    <div
      className={cn(
        'inline-flex items-center rounded-full border',
        bgColorClass,
        borderColorClass,
        textColorClass,
        sizeClasses[size]
      )}
    >
      <span
        className={cn(
          'font-normal break-keep text-nowrap whitespace-nowrap',
          textColorClass
        )}
      >
        {children}
      </span>
    </div>
  );
}

export default StatusChip;
