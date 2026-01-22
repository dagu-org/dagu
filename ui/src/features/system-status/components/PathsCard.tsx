import { Check, Copy, FolderOpen } from 'lucide-react';
import React from 'react';
import { Button } from '../../../components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '../../../components/ui/dialog';
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
    <div className="flex items-center gap-2 py-1 group text-sm min-w-0">
      <span className="text-muted-foreground shrink-0 w-24">{label}</span>
      <div className="flex-1 min-w-0 overflow-hidden">
        <code
          className={cn(
            'font-mono px-2 py-1 rounded text-xs block overflow-x-auto whitespace-nowrap no-scrollbar',
            'bg-muted text-foreground'
          )}
        >
          {path || '-'}
        </code>
      </div>
      <button
        onClick={handleCopy}
        className={cn(
          'p-1 rounded transition-all shrink-0',
          'opacity-0 group-hover:opacity-100',
          'hover:bg-accent text-muted-foreground hover:text-foreground',
          copied && 'opacity-100 text-success'
        )}
        title="Copy path"
      >
        {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
      </button>
    </div>
  );
}

function PathsCard() {
  const [open, setOpen] = React.useState(false);
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
      { label: 'Audit Logs', path: paths.auditLogsDir },
      { label: 'Queue', path: paths.queueDir },
      { label: 'Process', path: paths.procDir },
      { label: 'Services', path: paths.serviceRegistryDir },
      { label: 'Suspend', path: paths.suspendFlagsDir },
      { label: 'Git Sync', path: paths.gitSyncDir },
    ];
  }, [paths]);

  return (
    <>
      <Button onClick={() => setOpen(true)}>
        <FolderOpen className="h-4 w-4" />
        Paths
      </Button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="sm:max-w-lg overflow-hidden">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <FolderOpen className="h-4 w-4" />
              System Paths
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-1 overflow-hidden">
            {paths ? (
              pathItems.map((item) => (
                <PathItem
                  key={item.label}
                  label={item.label}
                  path={item.path}
                />
              ))
            ) : (
              <div className="text-sm text-muted-foreground py-2">
                No path information available
              </div>
            )}
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}

export default PathsCard;
