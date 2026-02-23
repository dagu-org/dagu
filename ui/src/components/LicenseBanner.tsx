import { useConfig } from '@/contexts/ConfigContext';
import { LICENSE_CONSOLE_URL } from '@/lib/constants';
import dayjs from '@/lib/dayjs';
import { X } from 'lucide-react';
import * as React from 'react';

// Must match the backend grace period duration
const GRACE_PERIOD_DAYS = 14;

function daysUntilExpiry(expiryISO: string): number {
  if (!expiryISO) return Infinity;
  return Math.max(0, Math.ceil(dayjs(expiryISO).diff(dayjs(), 'day', true)));
}

export function LicenseBanner() {
  const config = useConfig();
  const { license } = config;

  const [dismissed30d, setDismissed30d] = React.useState(() => {
    return localStorage.getItem(`license-banner-dismissed-30d-${license.expiry}`) === 'true';
  });
  const [dismissed7d, setDismissed7d] = React.useState(() => {
    return localStorage.getItem(`license-banner-dismissed-7d-${license.expiry}`) === 'true';
  });

  // Community mode: no banner
  if (license.community) return null;

  // Grace period: non-dismissible amber banner
  if (license.gracePeriod) {
    const graceEnd = license.expiry
      ? dayjs(license.expiry).add(GRACE_PERIOD_DAYS, 'day').format('YYYY-MM-DD')
      : 'soon';
    return (
      <div role="alert" className="bg-amber-50 dark:bg-amber-950 border-b border-amber-200 dark:border-amber-800 px-4 py-1.5 flex items-center text-sm">
        <span className="text-amber-800 dark:text-amber-200">
          Your Dagu Pro license has expired. Features will be disabled on {graceEnd}. Please{' '}
          <a
            href={LICENSE_CONSOLE_URL}
            target="_blank"
            rel="noopener noreferrer"
            className="underline hover:no-underline"
          >
            renew your license
          </a>.
        </span>
      </div>
    );
  }

  // Not valid license (no grace period either): no banner
  if (!license.valid) return null;

  const days = daysUntilExpiry(license.expiry);

  // 7-day urgent banner
  if (days <= 7 && !dismissed7d) {
    return (
      <div role="alert" className="bg-orange-50 dark:bg-orange-950 border-b border-orange-200 dark:border-orange-800 px-4 py-1.5 flex items-center justify-between text-sm">
        <span className="text-orange-800 dark:text-orange-200">
          Your Dagu Pro license {days === 0 ? 'expires today' : `expires in ${days} day${days !== 1 ? 's' : ''}`}! Please{' '}
          <a
            href={LICENSE_CONSOLE_URL}
            target="_blank"
            rel="noopener noreferrer"
            className="underline hover:no-underline"
          >
            renew now
          </a>{' '}to keep Pro features.
        </span>
        <button
          onClick={() => {
            localStorage.setItem(`license-banner-dismissed-7d-${license.expiry}`, 'true');
            setDismissed7d(true);
          }}
          className="p-0.5 hover:bg-orange-100 dark:hover:bg-orange-900 rounded"
          aria-label="Dismiss license expiry notification"
        >
          <X className="h-4 w-4" />
        </button>
      </div>
    );
  }

  // 30-day warning banner
  if (days <= 30 && !dismissed30d) {
    return (
      <div role="status" className="bg-yellow-50 dark:bg-yellow-950 border-b border-yellow-200 dark:border-yellow-800 px-4 py-1.5 flex items-center justify-between text-sm">
        <span className="text-yellow-800 dark:text-yellow-200">
          Your Dagu Pro license {days === 0 ? 'expires today' : `expires in ${days} day${days !== 1 ? 's' : ''}`}. Please{' '}
          <a
            href={LICENSE_CONSOLE_URL}
            target="_blank"
            rel="noopener noreferrer"
            className="underline hover:no-underline"
          >
            renew to avoid disruption
          </a>.
        </span>
        <button
          onClick={() => {
            localStorage.setItem(`license-banner-dismissed-30d-${license.expiry}`, 'true');
            setDismissed30d(true);
          }}
          className="p-0.5 hover:bg-yellow-100 dark:hover:bg-yellow-900 rounded"
          aria-label="Dismiss license expiry notification"
        >
          <X className="h-4 w-4" />
        </button>
      </div>
    );
  }

  return null;
}
