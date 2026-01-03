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
      bgColorClass = 'bg-[rgba(107,168,107,0.12)]';
      borderColorClass = 'border-[#6ba86b]';
      textColorClass = 'text-[#5a8a5a]';
      break;
    case Status.Failed: // error -> warm coral red (sepia-compatible)
      bgColorClass = 'bg-[rgba(196,114,106,0.12)]';
      borderColorClass = 'border-[#c4726a]';
      textColorClass = 'text-[#b05a52]';
      break;
    case Status.Running: // running -> accent green (sepia-compatible)
      bgColorClass = 'bg-[rgba(125,168,125,0.15)]';
      borderColorClass = 'border-[#7da87d]';
      textColorClass = 'text-[#6b9a6b]';
      break;
    case Status.Aborted: // aborted -> muted coral/pink (sepia-compatible)
      bgColorClass = 'bg-[rgba(212,132,122,0.12)]';
      borderColorClass = 'border-[#d4847a]';
      textColorClass = 'text-[#c06a62]';
      break;
    case Status.NotStarted: // none -> slate blue (sepia-compatible)
      bgColorClass = 'bg-[rgba(138,159,196,0.12)]';
      borderColorClass = 'border-[#8a9fc4]';
      textColorClass = 'text-[#6a7fa4]';
      break;
    case Status.Queued: // queued -> muted purple (sepia-compatible)
      bgColorClass = 'bg-[rgba(154,122,196,0.12)]';
      borderColorClass = 'border-[#9a7ac4]';
      textColorClass = 'text-[#7a5aa4]';
      break;
    case Status.PartialSuccess: // partial success -> warm amber (sepia-compatible)
      bgColorClass = 'bg-[rgba(212,148,106,0.12)]';
      borderColorClass = 'border-[#d4946a]';
      textColorClass = 'text-[#c47a4a]';
      break;
    case Status.Waiting: // waiting for approval -> amber/yellow (attention-grabbing)
      bgColorClass = 'bg-[rgba(245,158,11,0.15)]';
      borderColorClass = 'border-[#f59e0b]';
      textColorClass = 'text-[#d97706]';
      break;
    default: // Fallback to warm gray (sepia-compatible)
      bgColorClass = 'bg-[rgba(168,160,152,0.12)]';
      borderColorClass = 'border-[#a8a098]';
      textColorClass = 'text-[#6b635a]';
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
