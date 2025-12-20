import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useQuery } from '@/hooks/api';
import { ExternalLink, Layers } from 'lucide-react';
import React, { useContext, useMemo } from 'react';
import { components } from '../../../../api/v2/schema';
import { StatusDot } from '../common';

type SubDAGRun = components['schemas']['SubDAGRun'];
type SubDAGRunDetail = components['schemas']['SubDAGRunDetail'];

type Props = {
  isOpen: boolean;
  onClose: () => void;
  stepName: string;
  subDAGName: string;
  subRuns: SubDAGRun[];
  onSelectSubRun: (subRunIndex: number, openInNewTab?: boolean) => void;
  /** Root DAG name for API calls */
  rootDagName: string;
  /** Root DAG run ID for API calls */
  rootDagRunId: string;
  /** Current DAG run ID (parent of sub-runs) - for multi-level nested DAGs */
  parentDagRunId?: string;
};

export function ParallelExecutionModal({
  isOpen,
  onClose,
  subDAGName,
  subRuns,
  onSelectSubRun,
  rootDagName,
  rootDagRunId,
  parentDagRunId,
}: Props) {
  const isMac = navigator.platform.toUpperCase().indexOf('MAC') >= 0;
  const [selectedIndex, setSelectedIndex] = React.useState<number | null>(null);
  const scrollContainerRef = React.useRef<HTMLDivElement>(null);
  const appBarContext = useContext(AppBarContext);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';

  // Determine if this is a nested sub-DAG
  const isNestedSubDAG = parentDagRunId && parentDagRunId !== rootDagRunId;

  // Fetch sub DAG run details with status information
  const { data: subRunsData } = useQuery(
    '/dag-runs/{name}/{dagRunId}/sub-dag-runs',
    {
      params: {
        path: {
          name: rootDagName,
          dagRunId: rootDagRunId,
        },
        query: {
          remoteNode,
          parentSubDAGRunId: isNestedSubDAG ? parentDagRunId : undefined,
        },
      },
    },
    {
      isPaused: () => !isOpen,
      refreshInterval: isOpen ? 3000 : 0,
    }
  );

  // Create a map of dagRunId to status details
  const statusMap = useMemo(() => {
    const map = new Map<string, SubDAGRunDetail>();
    if (subRunsData?.subRuns) {
      for (const detail of subRunsData.subRuns) {
        map.set(detail.dagRunId, detail);
      }
    }
    return map;
  }, [subRunsData]);

  // Handle keyboard navigation
  React.useEffect(() => {
    if (!isOpen) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      switch (e.key) {
        case 'ArrowDown':
          e.preventDefault();
          setSelectedIndex((prev) => prev === null ? 0 : (prev + 1) % subRuns.length);
          break;
        case 'ArrowUp':
          e.preventDefault();
          setSelectedIndex((prev) => prev === null ? subRuns.length - 1 : (prev - 1 + subRuns.length) % subRuns.length);
          break;
        case 'Enter':
          e.preventDefault();
          if (selectedIndex !== null) {
            const openInNewTab = e.metaKey || e.ctrlKey;
            onSelectSubRun(selectedIndex, openInNewTab);
            if (!openInNewTab) {
              onClose();
            }
          }
          break;
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, selectedIndex, subRuns.length, onSelectSubRun, onClose]);

  // Auto-scroll to selected item
  React.useEffect(() => {
    if (selectedIndex !== null && scrollContainerRef.current) {
      const container = scrollContainerRef.current;
      const selectedElement = container.children[selectedIndex] as HTMLElement;
      
      if (selectedElement) {
        // Use scrollIntoView for more reliable scrolling
        selectedElement.scrollIntoView({
          block: 'nearest',
          behavior: 'smooth'
        });
      }
    }
  }, [selectedIndex]);

  return (
    <Dialog open={isOpen} onOpenChange={onClose}>
      <DialogContent className="sm:max-w-[500px] overflow-hidden p-0">
        <div className="p-4 border-b border-zinc-200 dark:border-zinc-800">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2 text-base font-mono">
              <Layers className="h-4 w-4 text-violet-600 dark:text-violet-500" />
              {subDAGName}
            </DialogTitle>
            <DialogDescription className="text-xs mt-1 font-mono text-muted-foreground">
              {subRuns.length} sub DAG-runs
            </DialogDescription>
          </DialogHeader>
        </div>
        
        <div className="p-3">
          <div 
            ref={scrollContainerRef}
            className="space-y-1 max-h-[400px] overflow-y-auto"
          >
            {subRuns.map((subRun, index) => {
              const detail = statusMap.get(subRun.dagRunId);
              return (
              <div
                key={subRun.dagRunId}
                className="group relative flex items-center gap-2"
                onMouseEnter={() => setSelectedIndex(index)}
              >
                <button
                  className={`
                    flex-1 text-left transition-all duration-150 border rounded px-3 py-2 flex items-center gap-3 focus:outline-none
                    ${selectedIndex === index
                      ? 'border-violet-500 dark:border-violet-500 bg-violet-50 dark:bg-violet-950/20'
                      : 'border-transparent hover:border-zinc-300 dark:hover:border-zinc-700 hover:bg-zinc-50 dark:hover:bg-zinc-900'
                    }
                  `}
                  onClick={(e) => {
                    const openInNewTab = e.metaKey || e.ctrlKey;
                    onSelectSubRun(index, openInNewTab);
                    if (!openInNewTab) {
                      onClose();
                    }
                  }}
                >
                  <span className="font-mono text-xs text-zinc-500 dark:text-zinc-600 min-w-[24px]">
                    {String(index + 1).padStart(2, '0')}
                  </span>
                  {detail && (
                    <StatusDot status={detail.status} statusLabel={detail.statusLabel} />
                  )}
                  <span className="flex-1 min-w-0">
                    {subRun.params ? (
                      <code className="text-sm font-mono text-zinc-700 dark:text-zinc-300 truncate block">
                        {subRun.params}
                      </code>
                    ) : (
                      <span className="text-sm text-zinc-400 dark:text-zinc-600 italic">
                        No parameters
                      </span>
                    )}
                  </span>
                </button>
                <button
                  className="opacity-0 group-hover:opacity-100 transition-opacity duration-150 p-1.5 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800 focus:outline-none"
                  onClick={() => {
                    onSelectSubRun(index, true);
                  }}
                  title="Open in new tab"
                >
                  <ExternalLink className="h-3 w-3 text-zinc-500 dark:text-zinc-500" />
                </button>
              </div>
            );
            })}
          </div>
        </div>
        
        <div className="px-4 py-2 bg-zinc-50 dark:bg-zinc-900 border-t border-zinc-200 dark:border-zinc-800">
          <div className="flex items-center gap-3 text-xs text-zinc-500 dark:text-zinc-500 font-mono">
            <span>{isMac ? '⌘' : 'Ctrl'}+Click: new tab</span>
            <span className="opacity-40">•</span>
            <span>↑↓ Navigate</span>
            <span className="opacity-40">•</span>
            <span>Enter: select</span>
            <span className="opacity-40">•</span>
            <span>ESC: close</span>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
