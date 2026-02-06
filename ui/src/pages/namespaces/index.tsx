import { components } from '@/api/v1/schema';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useClient } from '@/hooks/api';
import dayjs from '@/lib/dayjs';
import { Button } from '@/components/ui/button';
import { Layers, Pencil, Plus, Trash2 } from 'lucide-react';
import { useCallback, useContext, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';

type Namespace = components['schemas']['Namespace'];

type NamespaceWithDagCount = Namespace & {
  dagCount?: number;
};

export default function NamespacesPage() {
  const client = useClient();
  const navigate = useNavigate();
  const appBarContext = useContext(AppBarContext);
  const [namespaces, setNamespaces] = useState<NamespaceWithDagCount[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<NamespaceWithDagCount | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);

  useEffect(() => {
    appBarContext.setTitle('Namespaces');
  }, [appBarContext]);

  const fetchNamespaces = useCallback(async () => {
    try {
      const { data, error: apiError } = await client.GET('/namespaces');
      if (apiError) {
        throw new Error('Failed to fetch namespaces');
      }
      const nsList = data?.namespaces || [];
      setNamespaces(nsList);
      setError(null);

      // Fetch DAG counts for each namespace in parallel
      const countsPromises = nsList.map(async (ns) => {
        try {
          const { data: dagData } = await client.GET(
            '/namespaces/{namespaceName}/dags',
            {
              params: {
                path: { namespaceName: ns.name },
                query: { perPage: 1 },
              },
            }
          );
          return { name: ns.name, count: dagData?.pagination?.totalRecords ?? 0 };
        } catch {
          return { name: ns.name, count: 0 };
        }
      });
      const counts = await Promise.all(countsPromises);
      const countMap = new Map(counts.map((c) => [c.name, c.count]));
      setNamespaces((prev) =>
        prev.map((ns) => ({ ...ns, dagCount: countMap.get(ns.name) ?? 0 }))
      );
    } catch (err) {
      setError(
        err instanceof Error ? err.message : 'Failed to load namespaces'
      );
    } finally {
      setIsLoading(false);
    }
  }, [client]);

  useEffect(() => {
    fetchNamespaces();
  }, [fetchNamespaces]);

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setIsDeleting(true);
    setError(null);

    try {
      const { error: apiError } = await client.DELETE(
        '/namespaces/{namespaceName}',
        {
          params: { path: { namespaceName: deleteTarget.name } },
        }
      );

      if (apiError) {
        const msg =
          (apiError as { message?: string }).message ||
          'Failed to delete namespace';
        throw new Error(msg);
      }

      setDeleteTarget(null);
      fetchNamespaces();
    } catch (err) {
      setError(
        err instanceof Error ? err.message : 'Failed to delete namespace'
      );
      setDeleteTarget(null);
    } finally {
      setIsDeleting(false);
    }
  };

  return (
    <div className="flex flex-col gap-4 max-w-7xl">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold">Namespaces</h1>
          <p className="text-sm text-muted-foreground">
            Manage namespace isolation boundaries
          </p>
        </div>
        <Button size="sm" onClick={() => navigate('/namespaces/create')}>
          <Plus className="h-4 w-4" />
          Create Namespace
        </Button>
      </div>

      {error && (
        <div className="p-3 text-sm text-destructive bg-destructive/10 rounded-md">
          {error}
        </div>
      )}

      <div className="card-obsidian overflow-auto min-h-0">
        <Table className="text-xs">
          <TableHeader>
            <TableRow>
              <TableHead className="w-[200px]">Name</TableHead>
              <TableHead className="w-[300px]">Description</TableHead>
              <TableHead className="w-[100px]">DAGs</TableHead>
              <TableHead className="w-[180px]">Created</TableHead>
              <TableHead className="w-[80px]"></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell
                  colSpan={5}
                  className="text-center text-muted-foreground py-8"
                >
                  Loading namespaces...
                </TableCell>
              </TableRow>
            ) : namespaces.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={5}
                  className="text-center text-muted-foreground py-8"
                >
                  No namespaces found
                </TableCell>
              </TableRow>
            ) : (
              namespaces.map((ns) => (
                <TableRow key={ns.name}>
                  <TableCell className="font-medium">
                    <div className="flex items-center gap-2">
                      <Layers className="h-3.5 w-3.5 text-muted-foreground" />
                      {ns.name}
                      {ns.name === 'default' && (
                        <span className="text-xs px-1.5 py-0.5 rounded bg-muted text-muted-foreground">
                          default
                        </span>
                      )}
                    </div>
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {ns.description || '—'}
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {ns.dagCount !== undefined ? ns.dagCount : '...'}
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {dayjs(ns.createdAt).format('MMM D, YYYY HH:mm')}
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-1">
                      <Button
                        variant="ghost"
                        size="sm"
                        className="h-7 w-7 p-0"
                        onClick={() => navigate(`/namespaces/${ns.name}/edit`)}
                      >
                        <Pencil className="h-3.5 w-3.5" />
                      </Button>
                      {ns.name !== 'default' && (
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-7 w-7 p-0 text-destructive hover:text-destructive"
                          disabled={ns.dagCount !== undefined && ns.dagCount > 0}
                          title={
                            ns.dagCount !== undefined && ns.dagCount > 0
                              ? 'Cannot delete namespace with DAGs'
                              : 'Delete namespace'
                          }
                          onClick={() => setDeleteTarget(ns)}
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </Button>
                      )}
                    </div>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      <Dialog open={!!deleteTarget} onOpenChange={(open) => !open && setDeleteTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete Namespace</DialogTitle>
            <DialogDescription>
              Are you sure you want to delete the namespace{' '}
              <strong>{deleteTarget?.name}</strong>? This action cannot be
              undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="ghost"
              onClick={() => setDeleteTarget(null)}
              disabled={isDeleting}
            >
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={handleDelete}
              disabled={isDeleting}
            >
              {isDeleting ? 'Deleting...' : 'Delete'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
