import * as React from 'react';
import { useState, useContext, useMemo, useRef, useEffect } from 'react';
import { Paperclip, X, Search, Check } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { useQuery } from '@/hooks/api';
import { AppBarContext } from '@/contexts/AppBarContext';
import { DAGContext } from '../types';

interface DAGPickerProps {
  /** Additional DAGs selected by the user (excludes current page DAG) */
  selectedDags: DAGContext[];
  /** Callback when user adds/removes additional DAGs */
  onChange: (dags: DAGContext[]) => void;
  /** Current page DAG - always included automatically, shown as non-removable */
  currentPageDag?: DAGContext | null;
  disabled?: boolean;
}

export function DAGPicker({
  selectedDags,
  onChange,
  currentPageDag,
  disabled,
}: DAGPickerProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const dropdownRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const appBarContext = useContext(AppBarContext);
  const remoteNode = appBarContext?.selectedRemoteNode || 'local';

  const { data } = useQuery('/dags', {
    params: {
      query: {
        remoteNode,
        perPage: 100,
      },
    },
  });

  const dagFiles = useMemo(() => {
    if (!data?.dags) return [];
    return data.dags.map((d) => ({
      fileName: d.fileName,
      name: d.dag.name,
    }));
  }, [data]);

  // Filter out current page DAG from the picker list (it's always included)
  const filteredDags = useMemo(() => {
    let dags = dagFiles;

    // Exclude current page DAG since it's auto-included
    if (currentPageDag) {
      dags = dags.filter((d) => d.fileName !== currentPageDag.dag_file);
    }

    if (!searchQuery.trim()) return dags;
    const query = searchQuery.toLowerCase();
    return dags.filter(
      (d) =>
        d.fileName.toLowerCase().includes(query) ||
        d.name.toLowerCase().includes(query)
    );
  }, [dagFiles, searchQuery, currentPageDag]);

  // Check if a DAG is selected (in additional selections)
  const isSelected = (fileName: string) =>
    selectedDags.some((d) => d.dag_file === fileName);

  // Toggle DAG selection
  const toggleDag = (fileName: string) => {
    if (isSelected(fileName)) {
      onChange(selectedDags.filter((d) => d.dag_file !== fileName));
    } else {
      onChange([...selectedDags, { dag_file: fileName }]);
    }
  };

  // Remove a DAG from selection
  const removeDag = (fileName: string) => {
    onChange(selectedDags.filter((d) => d.dag_file !== fileName));
  };

  // Close dropdown when clicking outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (
        dropdownRef.current &&
        !dropdownRef.current.contains(event.target as Node)
      ) {
        setIsOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  // Focus search input when dropdown opens
  useEffect(() => {
    if (isOpen && inputRef.current) {
      inputRef.current.focus();
    }
  }, [isOpen]);

  const totalCount = (currentPageDag ? 1 : 0) + selectedDags.length;

  return (
    <div className="relative" ref={dropdownRef}>
      {/* Context chips: current page (non-removable) + additional selections (removable) */}
      {(currentPageDag || selectedDags.length > 0) && (
        <div className="flex flex-wrap gap-1 mb-1">
          {/* Current page DAG - always shown, not removable */}
          {currentPageDag && (
            <span
              className={cn(
                'inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs',
                'bg-blue-100 dark:bg-blue-900/40 text-blue-700 dark:text-blue-300'
              )}
            >
              <Paperclip className="h-3 w-3" />
              <span className="max-w-[120px] truncate">{currentPageDag.dag_file}</span>
              {currentPageDag.dag_run_id && (
                <span className="opacity-70">
                  #{currentPageDag.dag_run_id.slice(0, 8)}
                </span>
              )}
              <span className="opacity-60 text-[10px]">(current)</span>
            </span>
          )}

          {/* Additional selected DAGs - removable */}
          {selectedDags.map((dag) => (
            <span
              key={dag.dag_file}
              className={cn(
                'inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs',
                'bg-slate-200 dark:bg-slate-700 text-slate-700 dark:text-slate-300'
              )}
            >
              <Paperclip className="h-3 w-3" />
              <span className="max-w-[120px] truncate">{dag.dag_file}</span>
              <button
                type="button"
                onClick={() => removeDag(dag.dag_file)}
                className="hover:text-destructive"
                disabled={disabled}
              >
                <X className="h-3 w-3" />
              </button>
            </span>
          ))}
        </div>
      )}

      {/* Picker button and dropdown */}
      <div className="relative inline-block">
        <Button
          type="button"
          variant="ghost"
          size="sm"
          onClick={() => setIsOpen(!isOpen)}
          disabled={disabled}
          className="h-7 px-2 text-muted-foreground hover:text-foreground"
          title="Attach additional DAG context"
        >
          <Paperclip className="h-4 w-4" />
          {totalCount > 0 && (
            <span className="ml-1 text-xs">{totalCount}</span>
          )}
        </Button>

        {isOpen && (
          <div
            className={cn(
              'absolute bottom-full left-0 mb-1 z-50',
              'w-64 max-h-64 overflow-hidden',
              'bg-popover border rounded-md shadow-lg',
              'flex flex-col'
            )}
          >
            {/* Search input */}
            <div className="p-2 border-b">
              <div className="relative">
                <Search className="absolute left-2 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
                <input
                  ref={inputRef}
                  type="text"
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  placeholder="Add more DAGs..."
                  className={cn(
                    'w-full pl-7 pr-2 py-1.5 text-sm rounded',
                    'bg-background border border-input',
                    'focus:outline-none focus:ring-1 focus:ring-ring'
                  )}
                />
              </div>
            </div>

            {/* DAG list (excludes current page DAG) */}
            <div className="overflow-y-auto flex-1 max-h-48">
              {filteredDags.length === 0 ? (
                <div className="px-3 py-2 text-sm text-muted-foreground">
                  {searchQuery ? 'No DAGs found' : 'No additional DAGs available'}
                </div>
              ) : (
                filteredDags.map((dag) => (
                  <button
                    key={dag.fileName}
                    type="button"
                    onClick={() => toggleDag(dag.fileName)}
                    className={cn(
                      'w-full px-3 py-1.5 text-left text-sm',
                      'hover:bg-accent flex items-center justify-between',
                      isSelected(dag.fileName) && 'bg-accent'
                    )}
                  >
                    <span className="truncate">{dag.fileName}</span>
                    {isSelected(dag.fileName) && (
                      <Check className="h-4 w-4 text-primary shrink-0" />
                    )}
                  </button>
                ))
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
