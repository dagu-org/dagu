import { Check, Copy, FolderOpen, X } from 'lucide-react';
import React from 'react';
import { Button } from '../../../components/ui/button';
import { useConfig, type PathsConfig } from '../../../contexts/ConfigContext';
import { cn } from '../../../lib/utils';

interface PathItemProps {
  label: string;
  path: string;
}

function PathItem({ label, path }: PathItemProps) {
  const [copied, setCopied] = React.useState(false);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(path);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      // Clipboard API might not be available
    }
  };

  return (
    <div className="flex items-center gap-2 py-0.5 group text-[11px]">
      <span className="text-muted-foreground shrink-0 w-20">{label}</span>
      <code
        className={cn(
          'font-mono px-1 py-0.5 rounded truncate flex-1',
          'bg-muted text-muted-foreground'
        )}
        title={path}
      >
        {path || '-'}
      </code>
      <button
        onClick={handleCopy}
        className={cn(
          'p-0.5 rounded transition-all shrink-0',
          'opacity-0 group-hover:opacity-100',
          'hover:bg-accent',
          copied && 'opacity-100 text-success'
        )}
        title="Copy path"
      >
        {copied ? <Check className="h-3 w-3" /> : <Copy className="h-3 w-3" />}
      </button>
    </div>
  );
}

function PathsDialog({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}) {
  const config = useConfig();
  const paths: PathsConfig = config.paths;

  const pathItems = React.useMemo(() => {
    if (!paths) return [];
    return [
      { label: 'Config File', path: paths.configFileUsed },
      { label: 'Base Config', path: paths.baseConfig },
      { label: 'DAGs', path: paths.dagsDir },
      { label: 'DAG Runs', path: paths.dagRunsDir },
      { label: 'Logs', path: paths.logDir },
      { label: 'Admin Logs', path: paths.adminLogsDir },
      { label: 'Queue', path: paths.queueDir },
      { label: 'Process', path: paths.procDir },
      { label: 'Services', path: paths.serviceRegistryDir },
      { label: 'Suspend', path: paths.suspendFlagsDir },
    ];
  }, [paths]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="fixed inset-0 bg-black/50" onClick={onClose} />
      <div className="relative bg-gradient-to-br from-slate-900 via-slate-900 to-slate-950 border border-indigo-500/30 rounded-xl w-full max-w-lg mx-4 shadow-2xl">
        <div className="flex items-center justify-between p-3 border-b border-indigo-500/20">
          <div className="flex items-center gap-1.5">
            <FolderOpen className="h-3.5 w-3.5 text-muted-foreground" />
            <span className="text-xs font-medium">System Paths</span>
          </div>
          <button
            onClick={onClose}
            className="p-1 rounded hover:bg-muted transition-colors"
          >
            <X className="h-3.5 w-3.5" />
          </button>
        </div>
        <div className="p-4">
          {paths ? (
            <div className="space-y-0">
              {pathItems.map((item) => (
                <PathItem
                  key={item.label}
                  label={item.label}
                  path={item.path}
                />
              ))}
            </div>
          ) : (
            <div className="text-xs text-muted-foreground py-2">
              No path information available
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function PathsCard() {
  const [open, setOpen] = React.useState(false);

  return (
    <>
      <Button onClick={() => setOpen(true)}>
        <FolderOpen className="h-4 w-4" />
        Paths
      </Button>
      <PathsDialog open={open} onClose={() => setOpen(false)} />
    </>
  );
}

export default PathsCard;
