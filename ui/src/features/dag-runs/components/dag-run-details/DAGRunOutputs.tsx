import React, { useState, useMemo } from 'react';
import { ArrowUpDown, Check, Copy, Package, Search } from 'lucide-react';
import { Status, StatusLabel } from '../../../../api/v2/schema';
import { useQuery } from '../../../../hooks/api';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Input } from '@/components/ui/input';
import StatusChip from '../../../../ui/StatusChip';
import dayjs from '@/lib/dayjs';

// Convert StatusLabel string to Status enum
function statusLabelToStatus(label: StatusLabel): Status {
  switch (label) {
    case StatusLabel.not_started:
      return Status.NotStarted;
    case StatusLabel.running:
      return Status.Running;
    case StatusLabel.failed:
      return Status.Failed;
    case StatusLabel.aborted:
      return Status.Aborted;
    case StatusLabel.succeeded:
      return Status.Success;
    case StatusLabel.queued:
      return Status.Queued;
    case StatusLabel.partially_succeeded:
      return Status.PartialSuccess;
    default:
      return Status.NotStarted;
  }
}

type Props = {
  dagName: string;
  dagRunId: string;
};

type SortConfig = {
  key: 'name' | 'value';
  direction: 'asc' | 'desc';
};

function DAGRunOutputs({ dagName, dagRunId }: Props) {
  const appBarContext = React.useContext(AppBarContext);
  const [filter, setFilter] = useState('');
  const [sortConfig, setSortConfig] = useState<SortConfig>({
    key: 'name',
    direction: 'asc',
  });
  const [copiedKey, setCopiedKey] = useState<string | null>(null);

  const { data, isLoading, error } = useQuery(
    '/dag-runs/{name}/{dagRunId}/outputs',
    {
      params: {
        query: { remoteNode: appBarContext.selectedRemoteNode || 'local' },
        path: { name: dagName, dagRunId },
      },
    },
    { revalidateOnFocus: false }
  );

  // Filter and sort outputs
  const filteredOutputs = useMemo(() => {
    if (!data?.outputs) return [];

    let entries = Object.entries(data.outputs);

    // Filter
    if (filter) {
      const lowerFilter = filter.toLowerCase();
      entries = entries.filter(
        ([key, value]) =>
          key.toLowerCase().includes(lowerFilter) ||
          value.toLowerCase().includes(lowerFilter)
      );
    }

    // Sort
    entries.sort((a, b) => {
      const aVal = sortConfig.key === 'name' ? a[0] : a[1];
      const bVal = sortConfig.key === 'name' ? b[0] : b[1];
      const cmp = aVal.localeCompare(bVal);
      return sortConfig.direction === 'asc' ? cmp : -cmp;
    });

    return entries;
  }, [data?.outputs, filter, sortConfig]);

  const handleSort = (key: 'name' | 'value') => {
    setSortConfig((prev) => ({
      key,
      direction: prev.key === key && prev.direction === 'asc' ? 'desc' : 'asc',
    }));
  };

  const handleCopy = async (key: string, value: string) => {
    await navigator.clipboard.writeText(value);
    setCopiedKey(key);
    setTimeout(() => setCopiedKey(null), 2000);
  };

  if (isLoading) {
    return (
      <div className="text-sm text-muted-foreground p-4">
        Loading outputs...
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="text-sm text-muted-foreground p-4">
        No outputs available
      </div>
    );
  }

  const { metadata, outputs } = data;
  const outputCount = Object.keys(outputs).length;

  return (
    <div className="space-y-4">
      {/* Metadata Header */}
      <div className="bg-surface border border-border rounded-lg p-4">
        <div className="flex items-center gap-2 mb-3">
          <Package className="h-4 w-4 text-muted-foreground" />
          <span className="text-sm font-semibold">Outputs</span>
          <span className="text-xs text-muted-foreground">
            ({outputCount} items)
          </span>
        </div>

        <div className="flex flex-wrap gap-4 text-xs">
          <div className="flex items-center gap-1">
            <span className="text-muted-foreground">Status:</span>
            <StatusChip status={statusLabelToStatus(metadata.status)} size="xs">
              {metadata.status}
            </StatusChip>
          </div>
          <div>
            <span className="text-muted-foreground">Completed: </span>
            <span className="font-mono">
              {dayjs(metadata.completedAt).format('YYYY-MM-DD HH:mm:ss')}
            </span>
          </div>
          <div>
            <span className="text-muted-foreground">Attempt: </span>
            <span className="font-mono">{metadata.attemptId}</span>
          </div>
          {metadata.params && (
            <div>
              <span className="text-muted-foreground">Params: </span>
              <span className="font-mono">{metadata.params}</span>
            </div>
          )}
        </div>
      </div>

      {/* Filter Input */}
      <div className="relative">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
        <Input
          placeholder="Filter outputs by key or value..."
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          className="pl-9 h-8 text-sm"
        />
      </div>

      {/* Outputs Table */}
      <div className="border border-border rounded-lg overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow className="hover:bg-transparent">
              <TableHead
                className="w-[200px] cursor-pointer select-none"
                onClick={() => handleSort('name')}
              >
                <div className="flex items-center gap-1">
                  Key
                  <ArrowUpDown className="h-3 w-3 text-muted-foreground" />
                </div>
              </TableHead>
              <TableHead
                className="cursor-pointer select-none"
                onClick={() => handleSort('value')}
              >
                <div className="flex items-center gap-1">
                  Value
                  <ArrowUpDown className="h-3 w-3 text-muted-foreground" />
                </div>
              </TableHead>
              <TableHead className="w-[50px]"></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {filteredOutputs.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={3}
                  className="text-center text-muted-foreground py-8"
                >
                  {filter
                    ? 'No outputs match your filter'
                    : 'No outputs collected'}
                </TableCell>
              </TableRow>
            ) : (
              filteredOutputs.map(([key, value]) => (
                <TableRow key={key}>
                  <TableCell className="font-mono text-sm font-medium">
                    {key}
                  </TableCell>
                  <TableCell className="font-mono text-sm text-muted-foreground break-all">
                    {value}
                  </TableCell>
                  <TableCell>
                    <button
                      onClick={() => handleCopy(key, value)}
                      className="p-1 hover:bg-accent rounded"
                      title="Copy value"
                    >
                      {copiedKey === key ? (
                        <Check className="h-3.5 w-3.5 text-success" />
                      ) : (
                        <Copy className="h-3.5 w-3.5 text-muted-foreground" />
                      )}
                    </button>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>
    </div>
  );
}

export default DAGRunOutputs;
