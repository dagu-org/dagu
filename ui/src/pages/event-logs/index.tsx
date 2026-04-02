// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { RefreshButton } from '@/components/ui/refresh-button';
import { Button } from '@/components/ui/button';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useCanViewEventLogs } from '@/contexts/AuthContext';
import { useConfig } from '@/contexts/ConfigContext';
import { useSearchState } from '@/contexts/SearchStateContext';
import { useClient } from '@/hooks/api';
import { FetchError } from '@/lib/fetchJson';
import { cn } from '@/lib/utils';
import { Activity } from 'lucide-react';
import * as React from 'react';
import { useSearchParams } from 'react-router-dom';
import { EventLogsFilters } from './EventLogsFilters';
import { EventLogsTable } from './EventLogsTable';
import type { EventLogEntry } from './types';
import { safeStringify } from './utils';
import { useEventLogFeed } from './useEventLogFeed';
import { useEventLogFilters } from './useEventLogFilters';

export default function EventLogsPage() {
  const client = useClient();
  const config = useConfig();
  const canViewEventLogs = useCanViewEventLogs();
  const appBarContext = React.useContext(AppBarContext);
  const searchState = useSearchState();
  const remoteKey = appBarContext.selectedRemoteNode || 'local';
  const [searchParams, setSearchParams] = useSearchParams();
  const searchKey = searchParams.toString();
  const locationSearchParams = React.useMemo(
    () => new URLSearchParams(searchKey),
    [searchKey]
  );
  const [selectedEvent, setSelectedEvent] =
    React.useState<EventLogEntry | null>(null);

  React.useEffect(() => {
    appBarContext.setTitle('Events');
  }, [appBarContext]);

  const filters = useEventLogFilters({
    tzOffsetInSec: config.tzOffsetInSec,
    remoteKey,
    searchKey,
    locationSearchParams,
    searchState,
    setSearchParams,
  });

  const {
    data,
    error,
    isLoading,
    autoRefresh,
    setAutoRefresh,
    lastUpdatedAt,
    entries,
    hasMoreEntries,
    isAutoRefreshAvailable,
    isLoadingMore,
    loadMoreError,
    handleRefresh,
    handleLoadMore,
  } = useEventLogFeed(client, filters.query, filters.isReady);

  const rawEventJson = React.useMemo(
    () => (selectedEvent ? safeStringify(selectedEvent) : ''),
    [selectedEvent]
  );
  const tzLabel = filters.formatTimezoneOffset();

  const handleKeyDown = React.useCallback(
    (event: React.KeyboardEvent<HTMLInputElement>) => {
      if (event.key === 'Enter') {
        event.preventDefault();
        filters.handleApplyFilters();
      }
    },
    [filters]
  );

  let errorMessage: string | null = null;
  if (error instanceof FetchError) {
    errorMessage = error.data?.message || error.message;
  } else if (error instanceof Error) {
    errorMessage = error.message;
  } else if (error) {
    errorMessage = 'Failed to load event logs';
  }

  if (!canViewEventLogs) {
    return (
      <div className="flex items-center justify-center h-64">
        <p className="text-muted-foreground">
          You do not have permission to access this page.
        </p>
      </div>
    );
  }

  return (
    <>
      <div className="flex flex-col gap-4 max-w-7xl h-full">
        <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
          <div>
            <h1 className="text-lg font-semibold">Events</h1>
            <p className="text-sm text-muted-foreground">
              Recent operational events for the selected remote node
            </p>
            <p className="text-xs text-muted-foreground mt-1">
              {lastUpdatedAt
                ? `Last updated ${filters.formatTimestamp(lastUpdatedAt.toISOString())}`
                : 'Waiting for the first response'}
              {autoRefresh && isAutoRefreshAvailable
                ? ' • Refreshing every 5 seconds'
                : ''}
              {!isAutoRefreshAvailable
                ? ' • Auto-refresh is disabled after loading older events'
                : ''}
            </p>
          </div>
          <div className="flex items-center gap-2">
            <Button
              type="button"
              onClick={() => {
                if (isAutoRefreshAvailable) {
                  setAutoRefresh((prev) => !prev);
                }
              }}
              disabled={!isAutoRefreshAvailable}
              aria-label={`Auto-refresh ${autoRefresh ? 'enabled' : 'disabled'}`}
              title={
                isAutoRefreshAvailable
                  ? `Toggle auto-refresh (currently ${autoRefresh ? 'ON' : 'OFF'})`
                  : 'Auto-refresh is disabled after loading older events'
              }
            >
              <Activity
                className={cn(
                  'h-4 w-4',
                  autoRefresh && isAutoRefreshAvailable && 'text-success'
                )}
              />
              Auto: {autoRefresh && isAutoRefreshAvailable ? 'ON' : 'OFF'}
            </Button>
            <RefreshButton onRefresh={handleRefresh} />
          </div>
        </div>

        <EventLogsFilters
          draftFilters={filters.draftFilters}
          eventTypeOptions={filters.eventTypeOptions}
          tzLabel={tzLabel}
          onKindChange={filters.handleKindChange}
          onTypeChange={filters.handleTypeChange}
          updateDraftFilters={filters.updateDraftFilters}
          onApply={filters.handleApplyFilters}
          onClear={filters.handleClearFilters}
          onDatePresetChange={filters.handleDatePresetChange}
          onSpecificPeriodChange={filters.handleSpecificPeriodChange}
          onDateRangeModeChange={filters.handleDateRangeModeChange}
          onSpecificPeriodSelect={filters.handleSpecificPeriodSelect}
          onKeyDown={handleKeyDown}
        />

        {errorMessage ? (
          <div className="p-3 text-sm text-destructive bg-destructive/10 rounded-md">
            {errorMessage}
          </div>
        ) : null}

        <EventLogsTable
          entries={entries}
          isLoading={isLoading}
          hasHeadResponse={data !== undefined}
          hasMoreEntries={hasMoreEntries}
          isLoadingMore={isLoadingMore}
          loadMoreError={loadMoreError}
          onLoadMore={() => {
            void handleLoadMore();
          }}
          onSelectEvent={setSelectedEvent}
          formatTimestamp={filters.formatTimestamp}
        />
      </div>

      <Dialog
        open={selectedEvent !== null}
        onOpenChange={(open) => {
          if (!open) {
            setSelectedEvent(null);
          }
        }}
      >
        <DialogContent className="sm:max-w-3xl max-h-[80vh] flex flex-col">
          <DialogHeader>
            <DialogTitle>Raw Event</DialogTitle>
            <DialogDescription className="sr-only">
              Full JSON payload for the selected event log entry.
            </DialogDescription>
          </DialogHeader>
          <div className="min-h-0 overflow-auto rounded-md bg-muted p-3">
            <pre className="text-xs leading-5 whitespace-pre-wrap break-all font-mono">
              {rawEventJson}
            </pre>
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}
