import {
  forwardRef,
  useContext,
  useEffect,
  useImperativeHandle,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
} from 'react';

import { Search, Sparkles, X } from 'lucide-react';

import { AppBarContext } from '@/contexts/AppBarContext';
import { useClient } from '@/hooks/api';
import { cn } from '@/lib/utils';

export interface SkillRef {
  id: string;
  name: string;
}

interface SkillEntry {
  id: string;
  name: string;
  description?: string;
  tags?: string[];
}

export interface SkillPickerHandle {
  /** Returns true if the key was consumed by the picker. */
  handleKeyDown: (e: KeyboardEvent<HTMLTextAreaElement>) => boolean;
}

interface SkillPickerProps {
  selectedSkills: SkillRef[];
  onSelect: (skill: SkillRef) => void;
  onRemove: (id: string) => void;
  isOpen: boolean;
  onClose: () => void;
  filterQuery: string;
  disabled?: boolean;
}

export const SkillPicker = forwardRef<SkillPickerHandle, SkillPickerProps>(
  function SkillPicker(
    { selectedSkills, onSelect, onRemove, isOpen, onClose, filterQuery, disabled },
    ref
  ) {
    const client = useClient();
    const appBarContext = useContext(AppBarContext);
    const dropdownRef = useRef<HTMLDivElement>(null);
    const [skills, setSkills] = useState<SkillEntry[]>([]);
    const [highlightIndex, setHighlightIndex] = useState(0);

    // Fetch enabled skills once on mount.
    useEffect(() => {
      const controller = new AbortController();
      const remoteNode = appBarContext?.selectedRemoteNode || 'local';

      async function fetchSkills() {
        try {
          const { data } = await client.GET('/settings/agent/skills', {
            params: { query: { remoteNode } },
            signal: controller.signal,
          });
          if (!data?.skills) return;
          setSkills(
            data.skills
              .filter((s) => s.enabled)
              .map((s) => ({
                id: s.id,
                name: s.name,
                description: s.description,
                tags: s.tags,
              }))
          );
        } catch {
          // Best-effort
        }
      }
      fetchSkills();

      return () => controller.abort();
    }, [client, appBarContext?.selectedRemoteNode]);

    // Click-outside handler.
    useEffect(() => {
      if (!isOpen) return;
      function handleClickOutside(event: MouseEvent) {
        if (
          dropdownRef.current &&
          !dropdownRef.current.contains(event.target as Node)
        ) {
          onClose();
        }
      }
      document.addEventListener('mousedown', handleClickOutside);
      return () => document.removeEventListener('mousedown', handleClickOutside);
    }, [isOpen, onClose]);

    const selectedIds = useMemo(
      () => new Set(selectedSkills.map((s) => s.id)),
      [selectedSkills]
    );

    const filtered = useMemo(() => {
      const available = skills.filter((s) => !selectedIds.has(s.id));
      const q = filterQuery.trim().toLowerCase();
      if (!q) return available;
      return available.filter(
        (s) =>
          s.name.toLowerCase().includes(q) ||
          s.id.toLowerCase().includes(q) ||
          (s.description && s.description.toLowerCase().includes(q)) ||
          (s.tags && s.tags.some((t) => t.toLowerCase().includes(q)))
      );
    }, [skills, filterQuery, selectedIds]);

    // Reset highlight when filter changes.
    useEffect(() => {
      setHighlightIndex(0);
    }, [filterQuery]);

    // Expose keyboard handler to parent via ref.
    useImperativeHandle(ref, () => ({
      handleKeyDown(e: KeyboardEvent<HTMLTextAreaElement>): boolean {
        if (!isOpen) return false;
        if (e.key === 'ArrowDown') {
          e.preventDefault();
          setHighlightIndex((prev) => Math.min(prev + 1, filtered.length - 1));
          return true;
        }
        if (e.key === 'ArrowUp') {
          e.preventDefault();
          setHighlightIndex((prev) => Math.max(prev - 1, 0));
          return true;
        }
        if (e.key === 'Enter') {
          e.preventDefault();
          const s = filtered[highlightIndex];
          if (s) {
            onSelect({ id: s.id, name: s.name });
          }
          return true;
        }
        if (e.key === 'Escape') {
          e.preventDefault();
          onClose();
          return true;
        }
        return false;
      },
    }));

    return (
      <>
        {/* Skill chips */}
        {selectedSkills.length > 0 && (
          <div className="flex flex-wrap gap-1 mb-1">
            {selectedSkills.map((skill) => (
              <span
                key={skill.id}
                className={cn(
                  'inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs',
                  'bg-primary/15 text-primary'
                )}
              >
                <Sparkles className="h-3 w-3" />
                <span className="max-w-[120px] truncate">{skill.name}</span>
                <button
                  type="button"
                  onClick={() => onRemove(skill.id)}
                  className="hover:text-destructive"
                  disabled={disabled}
                >
                  <X className="h-3 w-3" />
                </button>
              </span>
            ))}
          </div>
        )}

        {/* Dropdown */}
        {isOpen && (
          <div
            ref={dropdownRef}
            className={cn(
              'absolute bottom-full left-0 mb-1 z-50',
              'w-72 max-h-64 overflow-hidden',
              'bg-popover border border-border rounded-md shadow-lg',
              'flex flex-col'
            )}
          >
            <div className="p-2 border-b">
              <div className="relative">
                <Search className="absolute left-2 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
                <div
                  className={cn(
                    'w-full pl-7 pr-2 py-1.5 text-sm',
                    'text-muted-foreground'
                  )}
                >
                  {filterQuery ? `/${filterQuery}` : 'Type to filter skills...'}
                </div>
              </div>
            </div>

            <div className="overflow-y-auto flex-1 max-h-48">
              {filtered.length === 0 && (
                <div className="px-3 py-2 text-sm text-muted-foreground">
                  {filterQuery ? 'No skills found' : 'No skills available'}
                </div>
              )}
              {filtered.map((skill, idx) => (
                <button
                  key={skill.id}
                  type="button"
                  onClick={() => onSelect({ id: skill.id, name: skill.name })}
                  className={cn(
                    'w-full px-3 py-1.5 text-left text-sm',
                    'hover:bg-accent',
                    idx === highlightIndex && 'bg-accent'
                  )}
                >
                  <div className="flex items-center gap-2">
                    <Sparkles className="h-3.5 w-3.5 text-primary shrink-0" />
                    <div className="min-w-0">
                      <div className="font-medium truncate">{skill.name}</div>
                      {skill.description && (
                        <div className="text-xs text-muted-foreground truncate">
                          {skill.description}
                        </div>
                      )}
                    </div>
                  </div>
                </button>
              ))}
            </div>
          </div>
        )}
      </>
    );
  }
);
