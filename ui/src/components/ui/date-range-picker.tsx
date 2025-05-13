import { CalendarRange } from 'lucide-react';
import React from 'react';
import { cn } from '../../lib/utils';
import { Input } from './input';

interface DateRangePickerProps extends React.HTMLAttributes<HTMLDivElement> {
  fromDate: string | undefined;
  toDate: string | undefined;
  onFromDateChange: (date: string) => void;
  onToDateChange: (date: string) => void;
  fromLabel?: string;
  toLabel?: string;
}

export function DateRangePicker({
  fromDate,
  toDate,
  onFromDateChange,
  onToDateChange,
  fromLabel = 'From',
  toLabel = 'To',
  className,
  ...props
}: DateRangePickerProps) {
  return (
    <div
      className={cn(
        'relative flex items-center rounded-md border px-1 bg-slate-50 dark:bg-slate-800 shadow-xs',
        className
      )}
      {...props}
    >
      <CalendarRange className="h-4 w-4 text-muted-foreground mr-2" />

      <div className="flex flex-1 flex-col sm:flex-row">
        <div className="relative flex-1">
          <label
            htmlFor="fromDate"
            className="absolute -top-2 left-2 px-1 text-xs text-muted-foreground bg-slate-50 dark:bg-slate-800"
          >
            {fromLabel}
          </label>
          <Input
            id="fromDate"
            type="datetime-local"
            value={fromDate || ''}
            onChange={(e) => onFromDateChange(e.target.value)}
            className="border-0 shadow-none focus-visible:ring-0 text-sm py-0.5 h-9 bg-transparent"
          />
        </div>

        <div className="flex items-center px-1 text-muted-foreground bg-slate-50 dark:bg-slate-800">
          →
        </div>

        <div className="relative flex-1">
          <label
            htmlFor="toDate"
            className="absolute -top-2 left-2 px-1 text-xs text-muted-foreground bg-slate-50 dark:bg-slate-800"
          >
            {toLabel}
          </label>
          <Input
            id="toDate"
            type="datetime-local"
            value={toDate || ''}
            onChange={(e) => onToDateChange(e.target.value)}
            className="border-0 shadow-none focus-visible:ring-0 text-sm py-0.5 h-9 bg-transparent"
          />
        </div>
      </div>
    </div>
  );
}
