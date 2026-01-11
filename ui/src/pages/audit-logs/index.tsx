import { useState, useEffect, useCallback, useContext } from 'react';
import { useConfig } from '@/contexts/ConfigContext';
import { useIsAdmin, TOKEN_KEY } from '@/contexts/AuthContext';
import { AppBarContext } from '@/contexts/AppBarContext';
import { components } from '@/api/v2/schema';
import { Button } from '@/components/ui/button';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { ScrollText, ChevronLeft, ChevronRight, RefreshCw } from 'lucide-react';
import dayjs from '@/lib/dayjs';

type AuditEntry = components['schemas']['AuditEntry'];

const CATEGORIES = [
  { value: 'all', label: 'All Categories' },
  { value: 'terminal', label: 'Terminal' },
  { value: 'user', label: 'User' },
  { value: 'dag', label: 'DAG' },
  { value: 'api_key', label: 'API Key' },
  { value: 'webhook', label: 'Webhook' },
];

const PAGE_SIZE = 50;

export default function AuditLogsPage() {
  const config = useConfig();
  const isAdmin = useIsAdmin();
  const appBarContext = useContext(AppBarContext);
  const [entries, setEntries] = useState<AuditEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Filter states
  const [category, setCategory] = useState('all');
  const [offset, setOffset] = useState(0);

  // Get selected remote node
  const remoteNode = appBarContext.selectedRemoteNode || 'local';

  // Set page title
  useEffect(() => {
    appBarContext.setTitle('Audit Logs');
  }, [appBarContext]);

  const fetchAuditLogs = useCallback(async () => {
    try {
      setIsLoading(true);
      const token = localStorage.getItem(TOKEN_KEY);

      const params = new URLSearchParams();
      params.set('remoteNode', remoteNode);
      if (category && category !== 'all') params.set('category', category);
      params.set('limit', String(PAGE_SIZE));
      params.set('offset', String(offset));

      const response = await fetch(`${config.apiURL}/audit?${params.toString()}`, {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });

      if (!response.ok) {
        if (response.status === 403) {
          throw new Error('You do not have permission to view audit logs');
        }
        throw new Error('Failed to fetch audit logs');
      }

      const data = await response.json();
      setEntries(data.entries || []);
      setTotal(data.total || 0);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load audit logs');
    } finally {
      setIsLoading(false);
    }
  }, [config.apiURL, category, offset, remoteNode]);

  useEffect(() => {
    fetchAuditLogs();
  }, [fetchAuditLogs]);

  // Reset offset when category or remote node changes
  useEffect(() => {
    setOffset(0);
  }, [category, remoteNode]);

  const handlePreviousPage = () => {
    setOffset(Math.max(0, offset - PAGE_SIZE));
  };

  const handleNextPage = () => {
    if (offset + PAGE_SIZE < total) {
      setOffset(offset + PAGE_SIZE);
    }
  };

  const currentPage = Math.floor(offset / PAGE_SIZE) + 1;
  const totalPages = Math.ceil(total / PAGE_SIZE);

  if (!isAdmin) {
    return (
      <div className="flex items-center justify-center h-64">
        <p className="text-muted-foreground">You do not have permission to access this page.</p>
      </div>
    );
  }

  const parseDetails = (details: string | undefined): Record<string, unknown> => {
    if (!details) return {};
    try {
      return JSON.parse(details);
    } catch {
      return { raw: details };
    }
  };

  const formatDetails = (entry: AuditEntry): string => {
    const details = parseDetails(entry.details);
    if (entry.category === 'terminal') {
      if (entry.action === 'session_start' || entry.action === 'session_end') {
        return `Session: ${details.session_id || 'N/A'}${details.reason ? ` (${details.reason})` : ''}`;
      }
      if (entry.action === 'command') {
        return `Command: ${details.command || 'N/A'}`;
      }
    }
    return entry.details || '-';
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold">Audit Logs</h1>
          <p className="text-sm text-muted-foreground">
            View system activity and security events
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Select value={category} onValueChange={setCategory}>
            <SelectTrigger className="w-[160px] h-8">
              <SelectValue placeholder="All Categories" />
            </SelectTrigger>
            <SelectContent>
              {CATEGORIES.map((cat) => (
                <SelectItem key={cat.value} value={cat.value}>
                  {cat.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button onClick={() => fetchAuditLogs()} size="sm" variant="outline" className="h-8">
            <RefreshCw className="h-4 w-4" />
          </Button>
        </div>
      </div>

      {error && (
        <div className="p-3 text-sm text-destructive bg-destructive/10 rounded-md">
          {error}
        </div>
      )}

      <div className="border rounded-lg">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-[180px]">Timestamp</TableHead>
              <TableHead className="w-[100px]">Category</TableHead>
              <TableHead className="w-[120px]">Action</TableHead>
              <TableHead className="w-[120px]">User</TableHead>
              <TableHead>Details</TableHead>
              <TableHead className="w-[120px]">IP Address</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={6} className="text-center text-muted-foreground py-8">
                  Loading audit logs...
                </TableCell>
              </TableRow>
            ) : entries.length === 0 ? (
              <TableRow>
                <TableCell colSpan={6} className="text-center text-muted-foreground py-8">
                  <ScrollText className="h-8 w-8 mx-auto mb-2 opacity-50" />
                  No audit log entries found
                </TableCell>
              </TableRow>
            ) : (
              entries.map((entry) => (
                <TableRow key={entry.id}>
                  <TableCell className="text-sm text-muted-foreground whitespace-nowrap">
                    {dayjs.utc(entry.timestamp).format('MMM D, YYYY HH:mm:ss')} UTC
                  </TableCell>
                  <TableCell>
                    <span className="text-xs px-1.5 py-0.5 rounded bg-muted text-muted-foreground capitalize">
                      {entry.category}
                    </span>
                  </TableCell>
                  <TableCell>
                    <span className="text-xs font-mono">
                      {entry.action}
                    </span>
                  </TableCell>
                  <TableCell className="text-sm">
                    {entry.username}
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground max-w-[300px] truncate" title={entry.details}>
                    {formatDetails(entry)}
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground font-mono">
                    {entry.ipAddress || '-'}
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      {/* Pagination */}
      {total > PAGE_SIZE && (
        <div className="flex items-center justify-between">
          <p className="text-sm text-muted-foreground">
            Showing {offset + 1} - {Math.min(offset + PAGE_SIZE, total)} of {total} entries
          </p>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={handlePreviousPage}
              disabled={offset === 0}
              className="h-8"
            >
              <ChevronLeft className="h-4 w-4 mr-1" />
              Previous
            </Button>
            <span className="text-sm text-muted-foreground">
              Page {currentPage} of {totalPages}
            </span>
            <Button
              variant="outline"
              size="sm"
              onClick={handleNextPage}
              disabled={offset + PAGE_SIZE >= total}
              className="h-8"
            >
              Next
              <ChevronRight className="h-4 w-4 ml-1" />
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
