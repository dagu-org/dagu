import React, { useContext, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Loader2,
  MoreHorizontal,
  Pencil,
  Plus,
  Search,
  Sparkles,
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
import { Switch } from '@/components/ui/switch';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useIsAdmin } from '@/contexts/AuthContext';
import { useClient, useQuery } from '@/hooks/api';
import { useDebouncedValue } from '@/hooks/useDebouncedValue';
import { DAGPagination } from '@/features/dags/components/common';
import ConfirmModal from '@/ui/ConfirmModal';

type SkillResponse = components['schemas']['SkillResponse'];

const DEFAULT_PER_PAGE = 30;

export default function AgentSkillsPage(): React.ReactNode {
  const client = useClient();
  const isAdmin = useIsAdmin();
  const appBarContext = useContext(AppBarContext);
  const navigate = useNavigate();

  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const [searchQuery, setSearchQuery] = useState('');
  const [filterTab, setFilterTab] = useState<'all' | 'enabled' | 'disabled'>('all');
  const [page, setPage] = useState(1);
  const [perPage, setPerPage] = useState(DEFAULT_PER_PAGE);

  const [deletingSkill, setDeletingSkill] = useState<SkillResponse | null>(null);

  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const debouncedQuery = useDebouncedValue(searchQuery, 300);

  useEffect(() => {
    appBarContext.setTitle('Agent Skills');
  }, [appBarContext]);

  // Reset to page 1 when search query changes
  useEffect(() => {
    setPage(1);
  }, [debouncedQuery]);

  const { data, mutate, isLoading } = useQuery(
    '/settings/agent/skills',
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

  const skills = data?.skills ?? [];
  const pagination = data?.pagination;

  async function handleToggleEnabled(skill: SkillResponse): Promise<void> {
    setError(null);
    setSuccess(null);

    const willEnable = !skill.enabled;

    // Optimistically update the local data
    await mutate(
      (current) => {
        if (!current) return current;
        return {
          ...current,
          skills: current.skills.map((s) =>
            s.id === skill.id ? { ...s, enabled: willEnable } : s
          ),
        };
      },
      { revalidate: false }
    );

    // Compute new enabled list from the optimistic state
    const updatedSkills = skills.map((s) =>
      s.id === skill.id ? { ...s, enabled: willEnable } : s
    );
    const newEnabled = updatedSkills.filter((s) => s.enabled).map((s) => s.id);

    try {
      const { error: apiError } = await client.PUT('/settings/agent/enabled-skills', {
        params: { query: { remoteNode } },
        body: { skillIds: newEnabled },
      });
      if (apiError) {
        await mutate(); // Revert to server state
        throw new Error(apiError.message || 'Failed to update enabled skills');
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update skill');
    }
  }

  async function handleDeleteSkill(): Promise<void> {
    if (!deletingSkill) return;
    try {
      const { error: apiError } = await client.DELETE('/settings/agent/skills/{skillId}', {
        params: { path: { skillId: deletingSkill.id }, query: { remoteNode } },
      });
      if (apiError) throw new Error(apiError.message || 'Failed to delete skill');
      setDeletingSkill(null);
      setSuccess(`Skill "${deletingSkill.name}" deleted`);
      await mutate();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete skill');
    }
  }

  // Client-side post-filter for enabled/disabled tabs within the current page
  const filteredSkills = skills.filter((skill) => {
    if (filterTab === 'enabled' && !skill.enabled) return false;
    if (filterTab === 'disabled' && skill.enabled) return false;
    return true;
  });

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
          <h1 className="text-lg font-semibold">Agent Skills</h1>
          <p className="text-sm text-muted-foreground">
            Manage knowledge and instruction sets for the AI agent
          </p>
        </div>
        <Button onClick={() => navigate('/agent-skills/new')} size="sm" className="h-8">
          <Plus className="h-4 w-4 mr-1.5" />
          Create Skill
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

      {/* Search and Filter */}
      <div className="flex items-center gap-3">
        <div className="relative flex-1 max-w-sm">
          <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="Search skills..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="h-8 pl-8 text-sm"
          />
        </div>
        <div className="flex gap-1">
          {(['all', 'enabled', 'disabled'] as const).map((tab) => (
            <Button
              key={tab}
              variant={filterTab === tab ? 'default' : 'outline'}
              size="sm"
              className="h-8 text-xs capitalize"
              onClick={() => setFilterTab(tab)}
            >
              {tab}
              {tab === 'enabled' && (
                <span className="ml-1 text-xs">
                  ({skills.filter((s) => s.enabled).length})
                </span>
              )}
            </Button>
          ))}
        </div>
      </div>

      {/* Skills Grid */}
      {isLoading && !data ? (
        <div className="flex items-center justify-center h-64">
          <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
        </div>
      ) : filteredSkills.length === 0 ? (
        <div className="card-obsidian p-8 text-center">
          <Sparkles className="h-8 w-8 mx-auto text-muted-foreground mb-2" />
          <p className="text-sm text-muted-foreground">
            {skills.length === 0 && !debouncedQuery
              ? 'No skills configured. Create a skill to get started.'
              : 'No skills match your search criteria.'}
          </p>
        </div>
      ) : (
        <div className="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
          {filteredSkills.map((skill) => (
            <div
              key={skill.id}
              className="card-obsidian p-4 space-y-3"
            >
              <div className="flex items-start justify-between gap-2">
                <div className="min-w-0 flex-1">
                  <h3 className="text-sm font-medium truncate">{skill.name}</h3>
                  <code className="text-xs text-muted-foreground">{skill.id}</code>
                </div>
                <div className="flex items-center gap-1.5 shrink-0">
                  <Switch
                    checked={skill.enabled}
                    onCheckedChange={() => handleToggleEnabled(skill)}
                  />
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button variant="ghost" size="icon" className="h-7 w-7">
                        <MoreHorizontal className="h-4 w-4" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      <DropdownMenuItem onClick={() => navigate(`/agent-skills/${skill.id}`)}>
                        <Pencil className="h-4 w-4 mr-2" />
                        Edit
                      </DropdownMenuItem>
                      <DropdownMenuItem
                        onClick={() => setDeletingSkill(skill)}
                        className="text-destructive"
                      >
                        <Trash2 className="h-4 w-4 mr-2" />
                        Delete
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>
              </div>

              {skill.description && (
                <p className="text-xs text-muted-foreground line-clamp-2">
                  {skill.description}
                </p>
              )}

              <div className="flex items-center gap-2 flex-wrap">
                {skill.tags?.map((tag) => (
                  <span
                    key={tag}
                    className="text-xs px-1.5 py-0.5 rounded bg-muted text-muted-foreground"
                  >
                    {tag}
                  </span>
                ))}
              </div>

              <div className="flex items-center gap-3 text-xs text-muted-foreground">
                {skill.author && <span>by {skill.author}</span>}
                {skill.version && <span>v{skill.version}</span>}
              </div>
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
        title="Delete Skill"
        buttonText="Delete"
        visible={!!deletingSkill}
        dismissModal={() => setDeletingSkill(null)}
        onSubmit={handleDeleteSkill}
      >
        <p>
          Are you sure you want to delete the skill &quot;{deletingSkill?.name}
          &quot;? This action cannot be undone.
        </p>
      </ConfirmModal>
    </div>
  );
}
