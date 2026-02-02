import { useConfig } from '@/contexts/ConfigContext';
import { X } from 'lucide-react';
import * as React from 'react';

export function UpdateBanner() {
  const config = useConfig();
  const [dismissed, setDismissed] = React.useState(() => {
    return (
      localStorage.getItem('update-banner-dismissed') === config.latestVersion
    );
  });

  if (!config.updateAvailable || dismissed) return null;

  const handleDismiss = () => {
    localStorage.setItem('update-banner-dismissed', config.latestVersion);
    setDismissed(true);
  };

  return (
    <div className="bg-blue-50 dark:bg-blue-950 border-b border-blue-200 dark:border-blue-800 px-4 py-1.5 flex items-center justify-between text-sm">
      <span className="text-blue-800 dark:text-blue-200">
        Update available: v{config.version} &rarr; {config.latestVersion}
        <a
          href="https://github.com/dagu-org/dagu/releases"
          target="_blank"
          rel="noopener noreferrer"
          className="ml-2 underline hover:no-underline"
        >
          View release
        </a>
      </span>
      <button
        onClick={handleDismiss}
        className="p-0.5 hover:bg-blue-100 dark:hover:bg-blue-900 rounded"
        aria-label="Dismiss update notification"
      >
        <X className="h-4 w-4" />
      </button>
    </div>
  );
}
