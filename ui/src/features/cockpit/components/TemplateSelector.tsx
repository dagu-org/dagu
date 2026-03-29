import React, { useState, useEffect, useRef, useCallback, useContext, useMemo } from 'react';
import { useQuery } from '@/hooks/api';
import { AppBarContext } from '@/contexts/AppBarContext';
import { Badge } from '@/components/ui/badge';
import { whenEnabled } from '@/hooks/queryUtils';
import { Search, ChevronDown, X, AlertTriangle, Tags } from 'lucide-react';
import { cn } from '@/lib/utils';
import {
  appendWorkspaceTag,
  buildWorkspaceTag,
  filterWorkspaceTags,
  isWorkspaceTag,
} from '@/lib/workspaceTags';
import type { components } from '@/api/v1/schema';

type DAGFile = components['schemas']['DAGFile'];

interface Props {
  selectedTemplate: string;
  selectedWorkspace: string;
  workspaceReady: boolean;
  onSelect: (fileName: string) => void;
  onOpenChange?: (isOpen: boolean) => void;
}

export function TemplateSelector({
  selectedTemplate,
  selectedWorkspace,
  workspaceReady,
  onSelect,
  onOpenChange,
}: Props): React.ReactElement {
  const appBarContext = useContext(AppBarContext);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';

  const [isOpen, setIsOpen] = useState(false);
  const [searchTerm, setSearchTerm] = useState('');
  const [debouncedTerm, setDebouncedTerm] = useState('');
  const [selectedTags, setSelectedTags] = useState<string[]>([]);
  const [isTagFilterOpen, setIsTagFilterOpen] = useState(false);
  const [highlightedIndex, setHighlightedIndex] = useState(-1);
  const [selectedDag, setSelectedDag] = useState<DAGFile | null>(null);

  const containerRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  // Debounce search term
  useEffect(() => {
    const timer = setTimeout(() => setDebouncedTerm(searchTerm), 300);
    return () => clearTimeout(timer);
  }, [searchTerm]);

  useEffect(() => {
    onOpenChange?.(isOpen);
  }, [isOpen, onOpenChange]);

  const queryTags = useMemo(
    () => appendWorkspaceTag(selectedTags, selectedWorkspace),
    [selectedTags, selectedWorkspace]
  );
  const workspaceHint = buildWorkspaceTag(selectedWorkspace);

  // Keep tags fully lazy. We only request them when the user explicitly opens
  // the tag filter UI inside the selector.
  const { data: tagsData } = useQuery(
    '/dags/tags',
    whenEnabled(isOpen && isTagFilterOpen && workspaceReady, {
      params: { query: { remoteNode } },
    })
  );
  const availableTags = tagsData?.tags ?? [];

  // The DAG list only stays live while the selector dropdown is open. The
  // closed trigger uses locally cached selection metadata instead.
  const { data, isLoading } = useQuery(
    '/dags',
    whenEnabled(isOpen && workspaceReady, {
      params: {
        query: {
          remoteNode,
          perPage: 50,
          ...(debouncedTerm ? { name: debouncedTerm } : {}),
          ...(queryTags.length > 0 ? { tags: queryTags.join(',') } : {}),
        },
      },
    })
  );
  const dags = data?.dags ?? [];

  // Filter workspace= tags from the tag filter row
  const displayTags = useMemo(
    () => filterWorkspaceTags(availableTags),
    [availableTags]
  );

  const resetFilters = useCallback(() => {
    setSearchTerm('');
    setDebouncedTerm('');
    setSelectedTags([]);
    setHighlightedIndex(-1);
    setIsTagFilterOpen(false);
  }, []);

  // Cache selected DAG for trigger display
  useEffect(() => {
    if (!selectedTemplate) {
      setSelectedDag(null);
      return;
    }
    const found = dags.find((d) => d.fileName === selectedTemplate);
    if (found) {
      setSelectedDag(found);
    }
  }, [dags, selectedTemplate]);

  // Group DAGs by group field
  const groupedDags = useMemo(() => {
    const groups = new Map<string, DAGFile[]>();
    for (const dag of dags) {
      const group = dag.dag.group || '';
      const list = groups.get(group) || [];
      list.push(dag);
      groups.set(group, list);
    }
    // Sort groups alphabetically, ungrouped last
    const sorted = [...groups.entries()].sort((a, b) => {
      if (a[0] === '' && b[0] === '') return 0;
      if (a[0] === '') return 1;
      if (b[0] === '') return -1;
      return a[0].localeCompare(b[0]);
    });
    return sorted;
  }, [dags]);

  // Flattened list for keyboard navigation
  const flatList = useMemo(() => {
    const items: DAGFile[] = [];
    for (const [, dagList] of groupedDags) {
      items.push(...dagList);
    }
    return items;
  }, [groupedDags]);

  // Reset highlight on filter change
  useEffect(() => {
    setHighlightedIndex(-1);
  }, [debouncedTerm, selectedTags]);

  // Click outside to close
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(event.target as Node)) {
        setIsOpen(false);
        resetFilters();
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, [resetFilters]);

  useEffect(() => {
    setIsOpen(false);
    setIsTagFilterOpen(false);
    setSearchTerm('');
    setDebouncedTerm('');
    setSelectedTags([]);
    setHighlightedIndex(-1);
    setSelectedDag(null);
  }, [remoteNode]);

  // Scroll highlighted item into view
  useEffect(() => {
    if (highlightedIndex < 0 || !listRef.current) return;
    const items = listRef.current.querySelectorAll('[data-dag-item]');
    items[highlightedIndex]?.scrollIntoView({ block: 'nearest' });
  }, [highlightedIndex]);

  const handleSelect = useCallback(
    (fileName: string) => {
      const dag = dags.find((d) => d.fileName === fileName);
      if (dag) setSelectedDag(dag);
      setIsOpen(false);
      resetFilters();
      onSelect(fileName);
    },
    [dags, onSelect, resetFilters]
  );

  const toggleTag = useCallback((tag: string) => {
    setSelectedTags((prev) =>
      prev.includes(tag) ? prev.filter((t) => t !== tag) : [...prev, tag]
    );
  }, []);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault();
        setHighlightedIndex((prev) => (prev < flatList.length - 1 ? prev + 1 : 0));
        break;
      case 'ArrowUp':
        e.preventDefault();
        setHighlightedIndex((prev) => (prev > 0 ? prev - 1 : flatList.length - 1));
        break;
      case 'Enter':
        e.preventDefault();
        if (highlightedIndex >= 0 && highlightedIndex < flatList.length) {
          const dag = flatList[highlightedIndex];
          if (dag) handleSelect(dag.fileName);
        }
        break;
      case 'Escape':
        setIsOpen(false);
        resetFilters();
        break;
    }
  };

  const handleOpen = () => {
    resetFilters();
    setIsOpen(true);
    setTimeout(() => inputRef.current?.focus(), 0);
  };

  const selectedDagName = selectedDag?.dag.name;
  const hasGroups = groupedDags.some(([group]) => group !== '');

  return (
    <div ref={containerRef} className="relative">
      {/* Trigger button */}
      <button
        type="button"
        onClick={handleOpen}
        className={cn(
          'flex items-center justify-between gap-2 h-7 px-3 text-xs rounded-md border border-border bg-background whitespace-nowrap transition-colors outline-none',
          'hover:border-border-strong cursor-pointer w-48',
          isOpen && 'border-ring'
        )}
      >
        {selectedTemplate && selectedDagName ? (
          <>
            <span className="truncate flex-1 text-left">{selectedDagName}</span>
            <span
              role="button"
              tabIndex={0}
              onClick={(e) => {
                e.stopPropagation();
                onSelect('');
                setSelectedDag(null);
              }}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.stopPropagation();
                  e.preventDefault();
                  onSelect('');
                  setSelectedDag(null);
                }
              }}
              aria-label="Clear selection"
              className="p-0.5 rounded hover:bg-muted-foreground/20 hover:text-destructive cursor-pointer"
            >
              <X className="h-3 w-3" />
            </span>
          </>
        ) : (
          <>
            <Search className="h-3 w-3 text-muted-foreground shrink-0" />
            <span className="text-muted-foreground truncate flex-1 text-left">Select template...</span>
          </>
        )}
        <ChevronDown
          className={cn(
            'h-3 w-3 text-muted-foreground shrink-0 transition-transform',
            isOpen && 'rotate-180'
          )}
        />
      </button>

      {/* Dropdown */}
      {isOpen && (
        <div className="absolute z-50 mt-1 w-80 max-h-[min(70vh,600px)] rounded-md border border-border bg-popover shadow-md flex flex-col">
          {/* Search input */}
          <div className="flex items-center gap-2 px-3 py-1.5 border-b border-border">
            <Search className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
            <input
              ref={inputRef}
              type="text"
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="Search DAGs..."
              className="flex-1 bg-transparent border-none outline-none text-xs placeholder:text-muted-foreground"
            />
            {isLoading && debouncedTerm && (
              <span className="text-[10px] text-muted-foreground">Searching...</span>
            )}
            <button
              type="button"
              onClick={() => setIsTagFilterOpen((prev) => !prev)}
              className={cn(
                'inline-flex items-center gap-1 rounded border border-border px-2 py-1 text-[10px] text-muted-foreground hover:text-foreground',
                isTagFilterOpen && 'border-ring text-foreground'
              )}
            >
              <Tags className="h-3 w-3" />
              Tags
            </button>
          </div>

          {/* Tag filter row */}
          {isTagFilterOpen && (
            <div className="flex flex-wrap gap-1 px-3 pt-2 pb-2.5 border-b border-border max-h-[200px] overflow-y-auto shrink-0">
              {workspaceHint && (
                <span className="w-full text-[10px] text-muted-foreground">
                  Workspace filter: {workspaceHint}
                </span>
              )}
              {displayTags.length === 0 ? (
                <span className="text-[10px] text-muted-foreground">
                  {tagsData ? 'No tags found' : 'Loading tags...'}
                </span>
              ) : displayTags.map((tag) => {
                const isActive = selectedTags.includes(tag);
                return (
                  <button
                    key={tag}
                    type="button"
                    onClick={() => toggleTag(tag)}
                    className="cursor-pointer"
                  >
                    <Badge
                      variant={isActive ? 'primary' : 'default'}
                      className="text-[10px] px-1.5 cursor-pointer"
                    >
                      {isActive && <X className="h-2 w-2 mr-0.5" />}
                      {tag}
                    </Badge>
                  </button>
                );
              })}
            </div>
          )}

          {/* DAG list */}
          <div ref={listRef} className="overflow-y-auto flex-1 min-h-0" onKeyDown={handleKeyDown}>
            {flatList.length === 0 ? (
              <div className="px-3 py-4 text-xs text-muted-foreground text-center">
                No DAGs found
              </div>
            ) : (
              groupedDags.map(([group, dagList]) => (
                <div key={group}>
                  {/* Group header */}
                  {(hasGroups) && (
                    <div className="px-3 py-1 text-[10px] uppercase tracking-wider text-muted-foreground font-medium bg-popover border-b border-border sticky top-0 z-10">
                      {group || '(ungrouped)'}
                    </div>
                  )}
                  {/* DAG items */}
                  {dagList.map((dag) => {
                    const idx = flatList.indexOf(dag);
                    const params = dag.dag.params;
                    const tags = dag.dag.tags;
                    const description = dag.dag.description;
                    return (
                      <div
                        key={dag.fileName}
                        data-dag-item
                        className={cn(
                          'px-3 py-1.5 cursor-pointer transition-colors',
                          idx === highlightedIndex
                            ? 'bg-muted'
                            : 'hover:bg-muted'
                        )}
                        onClick={() => handleSelect(dag.fileName)}
                        onMouseEnter={() => setHighlightedIndex(idx)}
                      >
                        {/* Name + error/param indicators */}
                        <div className="flex items-center justify-between gap-2">
                          <span className={cn(
                            "font-medium text-xs truncate",
                            dag.errors?.length > 0 && "text-destructive"
                          )}>
                            {dag.dag.name}
                          </span>
                          <div className="flex items-center gap-1 shrink-0">
                            {dag.errors?.length > 0 && (
                              <AlertTriangle className="h-3 w-3 text-destructive" />
                            )}
                            {params && params.length > 0 && (
                              <span className="text-[10px] text-muted-foreground">
                                {params.length}p
                              </span>
                            )}
                          </div>
                        </div>
                        {/* Description */}
                        {description && (
                          <div className="text-[11px] text-muted-foreground truncate">
                            {description}
                          </div>
                        )}
                        {/* Tags */}
                        {tags && tags.length > 0 && (
                          <div className="flex items-center gap-1 mt-0.5">
                            {tags.slice(0, 3).map((tag) => (
                              <button
                                key={tag}
                                type="button"
                                onClick={(e) => {
                                  e.stopPropagation();
                                  if (isWorkspaceTag(tag)) {
                                    return;
                                  }
                                  toggleTag(tag);
                                }}
                              >
                                <Badge
                                  variant={selectedTags.includes(tag) ? 'primary' : 'default'}
                                  className="text-[10px] px-1 py-0 h-3 cursor-pointer"
                                >
                                  {tag}
                                </Badge>
                              </button>
                            ))}
                            {tags.length > 3 && (
                              <span className="text-[10px] text-muted-foreground">
                                +{tags.length - 3}
                              </span>
                            )}
                          </div>
                        )}
                      </div>
                    );
                  })}
                </div>
              ))
            )}
          </div>
        </div>
      )}
    </div>
  );
}
