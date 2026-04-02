import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { FileJson, ExternalLink } from 'lucide-react';
import { Link } from 'react-router-dom';
import type { EventLogEntry } from './types';
import {
  buildRunPath,
  getContextLabel,
  getEventTypeLabel,
  getKindLabel,
  getSubjectName,
} from './utils';

type EventLogsTableProps = {
  entries: EventLogEntry[];
  isLoading: boolean;
  hasHeadResponse: boolean;
  hasMoreEntries: boolean;
  isLoadingMore: boolean;
  loadMoreError: string | null;
  onLoadMore: () => void;
  onSelectEvent: (entry: EventLogEntry) => void;
  formatTimestamp: (timestamp: string) => string;
};

function getEventVariant(type: string) {
  switch (type) {
    case 'dag.run.succeeded':
      return 'success';
    case 'dag.run.failed':
      return 'error';
    case 'dag.run.aborted':
      return 'cancelled';
    case 'dag.run.waiting':
      return 'warning';
    case 'dag.run.rejected':
      return 'warning';
    case 'automata.finished':
      return 'success';
    case 'automata.error':
      return 'error';
    case 'automata.needs_input':
      return 'warning';
    default:
      return 'default';
  }
}

export function EventLogsTable({
  entries,
  isLoading,
  hasHeadResponse,
  hasMoreEntries,
  isLoadingMore,
  loadMoreError,
  onLoadMore,
  onSelectEvent,
  formatTimestamp,
}: EventLogsTableProps) {
  return (
    <div className="card-obsidian flex flex-col flex-1 min-h-0 overflow-hidden">
      <div className="flex items-center justify-between px-4 py-3 border-b flex-shrink-0">
        <div>
          <h2 className="text-sm font-semibold">Event Feed</h2>
          <p className="text-xs text-muted-foreground">
            Loaded {entries.length} event{entries.length === 1 ? '' : 's'}
          </p>
        </div>
        <div className="text-xs text-muted-foreground">
          {hasMoreEntries ? 'More events available' : 'End of feed'}
        </div>
      </div>

      <div className="flex-1 min-h-0 overflow-auto">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Occurred</TableHead>
              <TableHead>Kind</TableHead>
              <TableHead>Event</TableHead>
              <TableHead>Subject</TableHead>
              <TableHead>Context</TableHead>
              <TableHead>Source</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading && !hasHeadResponse ? (
              <TableRow>
                <TableCell
                  colSpan={7}
                  className="py-8 text-center text-muted-foreground"
                >
                  Loading event feed...
                </TableCell>
              </TableRow>
            ) : entries.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={7}
                  className="py-8 text-center text-muted-foreground"
                >
                  No events matched the current filters.
                </TableCell>
              </TableRow>
            ) : (
              entries.map((entry) => {
                const runPath = buildRunPath(entry);
                return (
                  <TableRow key={entry.id}>
                    <TableCell className="whitespace-nowrap text-muted-foreground">
                      {formatTimestamp(entry.occurredAt)}
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline">{getKindLabel(entry.kind)}</Badge>
                    </TableCell>
                    <TableCell>
                      <Badge variant={getEventVariant(entry.type) as never}>
                        {getEventTypeLabel(entry.type, entry.status)}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      {runPath ? (
                        <Link
                          to={runPath}
                          className="font-medium text-primary hover:underline underline-offset-2"
                        >
                          {getSubjectName(entry)}
                        </Link>
                      ) : getSubjectName(entry) !== '-' ? (
                        <span className="font-medium">{getSubjectName(entry)}</span>
                      ) : (
                        <span className="text-muted-foreground">-</span>
                      )}
                    </TableCell>
                    <TableCell className="font-mono break-all">
                      {getContextLabel(entry)}
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-col">
                        <span>{entry.sourceService}</span>
                        {entry.sourceInstance ? (
                          <span className="text-muted-foreground break-all">
                            {entry.sourceInstance}
                          </span>
                        ) : null}
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center justify-end gap-2">
                        {runPath ? (
                          <Button variant="ghost" size="sm" asChild>
                            <Link to={runPath}>
                              <ExternalLink className="h-4 w-4" />
                              Open Run
                            </Link>
                          </Button>
                        ) : null}
                        <Button
                          type="button"
                          variant="ghost"
                          size="sm"
                          onClick={() => onSelectEvent(entry)}
                        >
                          <FileJson className="h-4 w-4" />
                          View Raw
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                );
              })
            )}
          </TableBody>
        </Table>
      </div>

      <div className="flex items-center justify-between px-4 py-3 border-t flex-shrink-0">
        <p className="text-xs text-muted-foreground">
          Results follow committed log order, newest committed entries first.
        </p>
        <div className="flex items-center gap-2">
          {loadMoreError ? (
            <p className="text-xs text-destructive">{loadMoreError}</p>
          ) : null}
          {hasMoreEntries ? (
            <Button
              type="button"
              size="sm"
              variant="ghost"
              onClick={onLoadMore}
              disabled={isLoadingMore}
            >
              {isLoadingMore ? 'Loading...' : 'Load More'}
            </Button>
          ) : null}
        </div>
      </div>
    </div>
  );
}
