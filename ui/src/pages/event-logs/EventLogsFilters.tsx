import { Button } from '@/components/ui/button';
import { DateRangePicker } from '@/components/ui/date-range-picker';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { ToggleButton, ToggleGroup } from '@/components/ui/toggle-group';
import { FilterX, Search } from 'lucide-react';
import * as React from 'react';
import { EVENT_KIND_OPTIONS } from './options';
import type {
  DateRangeMode,
  EventLogFilters,
  SpecificPeriod,
} from './types';
import { getInputTypeForPeriod } from './utils';

type EventLogsFiltersProps = {
  draftFilters: EventLogFilters;
  eventTypeOptions: ReadonlyArray<{ value: string; label: string }>;
  tzLabel: string;
  onKindChange: (value: string) => void;
  onTypeChange: (value: string) => void;
  updateDraftFilters: (patch: Partial<EventLogFilters>) => void;
  onApply: () => void;
  onClear: () => void;
  onDatePresetChange: (preset: string) => void;
  onSpecificPeriodChange: (value: string, period?: SpecificPeriod) => void;
  onDateRangeModeChange: (nextMode: DateRangeMode) => void;
  onSpecificPeriodSelect: (value: string) => void;
  onKeyDown: (event: React.KeyboardEvent<HTMLInputElement>) => void;
};

export function EventLogsFilters({
  draftFilters,
  eventTypeOptions,
  tzLabel,
  onKindChange,
  onTypeChange,
  updateDraftFilters,
  onApply,
  onClear,
  onDatePresetChange,
  onSpecificPeriodChange,
  onDateRangeModeChange,
  onSpecificPeriodSelect,
  onKeyDown,
}: EventLogsFiltersProps) {
  return (
    <div className="card-obsidian p-4 flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-2">
        <Select value={draftFilters.kind} onValueChange={onKindChange}>
          <SelectTrigger className="w-[160px] h-8">
            <SelectValue placeholder="All kinds" />
          </SelectTrigger>
          <SelectContent>
            {EVENT_KIND_OPTIONS.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {option.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Select value={draftFilters.type} onValueChange={onTypeChange}>
          <SelectTrigger className="w-[180px] h-8">
            <SelectValue placeholder="All event types" />
          </SelectTrigger>
          <SelectContent>
            {eventTypeOptions.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {option.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <div className="relative">
          <Search className="absolute left-2 top-2 h-3.5 w-3.5 text-muted-foreground" />
          <Input
            value={draftFilters.dagName}
            onChange={(event) =>
              updateDraftFilters({ dagName: event.target.value })
            }
            onKeyDown={onKeyDown}
            placeholder="Filter by DAG name"
            className="h-8 w-[220px] pl-7"
          />
        </div>
        <Input
          value={draftFilters.automataName}
          onChange={(event) =>
            updateDraftFilters({ automataName: event.target.value })
          }
          onKeyDown={onKeyDown}
          placeholder="Automata name"
          className="h-8 w-[220px]"
        />
        <Input
          value={draftFilters.dagRunId}
          onChange={(event) =>
            updateDraftFilters({ dagRunId: event.target.value })
          }
          onKeyDown={onKeyDown}
          placeholder="DAG run ID"
          className="h-8 w-[220px]"
        />
        <Input
          value={draftFilters.attemptId}
          onChange={(event) =>
            updateDraftFilters({ attemptId: event.target.value })
          }
          onKeyDown={onKeyDown}
          placeholder="Attempt ID"
          className="h-8 w-[180px]"
        />
        <Button type="button" size="sm" onClick={onApply}>
          Apply Filters
        </Button>
        <Button type="button" size="sm" variant="ghost" onClick={onClear}>
          <FilterX className="h-4 w-4" />
          Clear
        </Button>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <ToggleGroup aria-label="Date range mode">
          <ToggleButton
            value="preset"
            groupValue={draftFilters.dateRangeMode}
            onClick={() => onDateRangeModeChange('preset')}
            aria-label="Quick select"
          >
            Quick
          </ToggleButton>
          <ToggleButton
            value="specific"
            groupValue={draftFilters.dateRangeMode}
            onClick={() => onDateRangeModeChange('specific')}
            aria-label="Specific date, month, or year"
          >
            Specific
          </ToggleButton>
          <ToggleButton
            value="custom"
            groupValue={draftFilters.dateRangeMode}
            onClick={() => onDateRangeModeChange('custom')}
            aria-label="Custom range"
          >
            Custom
          </ToggleButton>
        </ToggleGroup>

        {draftFilters.dateRangeMode === 'preset' ? (
          <Select
            value={draftFilters.datePreset}
            onValueChange={onDatePresetChange}
          >
            <SelectTrigger className="w-[180px] h-8">
              <SelectValue placeholder="Select period" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="today">Today</SelectItem>
              <SelectItem value="yesterday">Yesterday</SelectItem>
              <SelectItem value="last7days">Last 7 days</SelectItem>
              <SelectItem value="last30days">Last 30 days</SelectItem>
              <SelectItem value="thisWeek">This week</SelectItem>
              <SelectItem value="thisMonth">This month</SelectItem>
            </SelectContent>
          </Select>
        ) : draftFilters.dateRangeMode === 'specific' ? (
          <>
            <Select
              value={draftFilters.specificPeriod}
              onValueChange={onSpecificPeriodSelect}
            >
              <SelectTrigger className="w-[110px] h-8">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="date">Date</SelectItem>
                <SelectItem value="month">Month</SelectItem>
                <SelectItem value="year">Year</SelectItem>
              </SelectContent>
            </Select>
            <Input
              type={getInputTypeForPeriod(draftFilters.specificPeriod)}
              value={draftFilters.specificValue}
              onChange={(event) => onSpecificPeriodChange(event.target.value)}
              onKeyDown={onKeyDown}
              placeholder={
                draftFilters.specificPeriod === 'year' ? 'YYYY' : undefined
              }
              min={draftFilters.specificPeriod === 'year' ? '2000' : undefined}
              max={draftFilters.specificPeriod === 'year' ? '2100' : undefined}
              className="w-[140px] h-8"
            />
          </>
        ) : (
          <DateRangePicker
            fromDate={draftFilters.fromDate}
            toDate={draftFilters.toDate}
            onFromDateChange={(value) => updateDraftFilters({ fromDate: value })}
            onToDateChange={(value) => updateDraftFilters({ toDate: value })}
            onEnterPress={onApply}
            fromLabel={`From ${tzLabel}`}
            toLabel={`To ${tzLabel}`}
            className="w-full md:w-auto"
          />
        )}
      </div>
    </div>
  );
}
