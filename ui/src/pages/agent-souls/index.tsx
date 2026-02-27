import React, { useContext, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Ghost,
  Loader2,
  MoreHorizontal,
  Pencil,
  Plus,
  Search,
  Star,
  Trash2,
} from 'lucide-react';
import { components } from '@/api/v1/schema';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Input } from '@/components/ui/input';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useIsAdmin } from '@/contexts/AuthContext';
import { useClient, useQuery } from '@/hooks/api';
import { useDebouncedValue } from '@/hooks/useDebouncedValue';
import { DAGPagination } from '@/features/dags/components/common';
import ConfirmModal from '@/ui/ConfirmModal';

type SoulResponse = components['schemas']['SoulResponse'];

const DEFAULT_PER_PAGE = 30;

export default function AgentSoulsPage(): React.ReactNode {
  const client = useClient();
  const isAdmin = useIsAdmin();
  const appBarContext = useContext(AppBarContext);
  const navigate = useNavigate();

  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const [searchQuery, setSearchQuery] = useState('');
  const [page, setPage] = useState(1);
  const [perPage, setPerPage] = useState(DEFAULT_PER_PAGE);

  const [deletingSoul, setDeletingSoul] = useState<SoulResponse | null>(null);
  const [defaultSoulId, setDefaultSoulId] = useState<string | undefined>();

  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const debouncedQuery = useDebouncedValue(searchQuery, 300);

  useEffect(() => {
    appBarContext.setTitle('Agent Souls');
  }, [appBarContext]);

  useEffect(() => {
    setPage(1);
  }, [debouncedQuery]);

  useEffect(() => {
    (async () => {
      try {
        const { data } = await client.GET('/settings/agent', { params: { query: { remoteNode } } });
        if (data) setDefaultSoulId(data.selectedSoulId ?? undefined);
      } catch {
        // Best-effort fetch
      }
    })();
  }, [client, remoteNode]);

  const { data, mutate, isLoading } = useQuery(
    '/settings/agent/souls',
    {
      params: {
        query: {
          remoteNode,
          page,
          perPage,
          q: debouncedQuery || undefined,
        },
      },
    },
    { keepPreviousData: true }
  );

  const souls = data?.souls ?? [];
  const pagination = data?.pagination;

  async function handleSetDefault(soul: SoulResponse): Promise<void> {
    try {
      const { data, error: apiError } = await client.PATCH('/settings/agent', {
        params: { query: { remoteNode } },
        body: { selectedSoulId: soul.id },
      });
      if (apiError) throw new Error(apiError.message || 'Failed to set default soul');
      setDefaultSoulId(data.selectedSoulId ?? undefined);
      setSuccess(`"${soul.name}" is now the default soul`);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to set default soul');
    }
  }

  async function handleDeleteSoul(): Promise<void> {
    if (!deletingSoul) return;
    try {
      const { error: apiError } = await client.DELETE('/settings/agent/souls/{soulId}', {
        params: { path: { soulId: deletingSoul.id }, query: { remoteNode } },
      });
      if (apiError) throw new Error(apiError.message || 'Failed to delete soul');
      setDeletingSoul(null);
      setSuccess(`Soul "${deletingSoul.name}" deleted`);
      await mutate();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete soul');
    }
  }

  if (!isAdmin) {
    return (
      <div className="flex items-center justify-center h-64">
        <p className="text-muted-foreground">
          You do not have permission to access this page.
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-4 max-w-7xl pb-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold">Agent Souls</h1>
          <p className="text-sm text-muted-foreground">
            Manage personality and identity definitions for the AI agent
          </p>
        </div>
        <Button onClick={() => navigate('/agent-souls/new')} size="sm" className="h-8">
          <Plus className="h-4 w-4 mr-1.5" />
          Create Soul
        </Button>
      </div>

      {error && (
        <div className="p-3 text-sm text-destructive bg-destructive/10 rounded-md">
          {error}
        </div>
      )}

      {success && (
        <div className="p-3 text-sm text-green-600 dark:text-green-400 bg-green-500/10 rounded-md">
          {success}
        </div>
      )}

      {/* Search */}
      <div className="flex items-center gap-3">
        <div className="relative flex-1 max-w-sm">
          <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="Search souls..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="h-8 pl-8 text-sm"
          />
        </div>
      </div>

      {/* Souls Grid */}
      {isLoading && !data ? (
        <div className="flex items-center justify-center h-64">
          <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
        </div>
      ) : souls.length === 0 ? (
        <div className="card-obsidian p-8 text-center">
          <Ghost className="h-8 w-8 mx-auto text-muted-foreground mb-2" />
          <p className="text-sm text-muted-foreground">
            {!debouncedQuery
              ? 'No souls configured. Create a soul to get started.'
              : 'No souls match your search criteria.'}
          </p>
        </div>
      ) : (
        <div className="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
          {souls.map((soul) => (
            <div
              key={soul.id}
              className="card-obsidian p-4 space-y-3"
            >
              <div className="flex items-start justify-between gap-2">
                <div className="min-w-0 flex-1">
                  <h3 className="text-sm font-medium truncate">{soul.name}</h3>
                  <div className="flex items-center gap-1.5">
                    <code className="text-xs text-muted-foreground">{soul.id}</code>
                    {soul.id === defaultSoulId && (
                      <span className="text-xs px-1.5 py-0.5 rounded bg-primary/10 text-primary font-medium">
                        Default
                      </span>
                    )}
                  </div>
                </div>
                <div className="flex items-center gap-1.5 shrink-0">
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button variant="ghost" size="icon" className="h-7 w-7">
                        <MoreHorizontal className="h-4 w-4" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      <DropdownMenuItem
                        onClick={() => handleSetDefault(soul)}
                        disabled={soul.id === defaultSoulId}
                      >
                        <Star className="h-4 w-4 mr-2" />
                        Set as Default
                      </DropdownMenuItem>
                      <DropdownMenuItem onClick={() => navigate(`/agent-souls/${soul.id}`)}>
                        <Pencil className="h-4 w-4 mr-2" />
                        Edit
                      </DropdownMenuItem>
                      <DropdownMenuItem
                        onClick={() => setDeletingSoul(soul)}
                        className="text-destructive"
                      >
                        <Trash2 className="h-4 w-4 mr-2" />
                        Delete
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>
              </div>

              {soul.description && (
                <p className="text-xs text-muted-foreground line-clamp-2">
                  {soul.description}
                </p>
              )}

            </div>
          ))}
        </div>
      )}

      {/* Pagination */}
      {pagination && pagination.totalPages > 1 && (
        <div className="flex justify-end">
          <DAGPagination
            totalPages={pagination.totalPages}
            page={page}
            pageLimit={perPage}
            pageChange={setPage}
            onPageLimitChange={(limit) => {
              setPerPage(limit);
              setPage(1);
            }}
          />
        </div>
      )}

      {/* Delete Confirmation */}
      <ConfirmModal
        title="Delete Soul"
        buttonText="Delete"
        visible={!!deletingSoul}
        dismissModal={() => setDeletingSoul(null)}
        onSubmit={handleDeleteSoul}
      >
        <p>
          Are you sure you want to delete the soul &quot;{deletingSoul?.name}
          &quot;? This action cannot be undone.
        </p>
      </ConfirmModal>
    </div>
  );
}
