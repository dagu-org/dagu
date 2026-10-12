// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { useEffect, useState } from 'react';
import { ExternalLink, Loader2 } from 'lucide-react';
import { LicenseConnectStatusState } from '@/api/v1/schema';
import { Button } from '@/components/ui/button';
import { useI18n } from '@/i18n/I18nProvider';
import dayjs from '@/lib/dayjs';
import { LicenseCallout } from './LicenseAlerts';
import type { useLicenseConnect } from './useLicenseConnect';

function Countdown({ expiresAt }: { expiresAt?: string }) {
  const { ts } = useI18n();
  const [now, setNow] = useState(Date.now);
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, []);
  if (!expiresAt || !dayjs(expiresAt).isValid()) return null;
  const seconds = Math.max(0, dayjs(expiresAt).diff(dayjs(now), 'second'));
  const time = `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, '0')}`;
  return (
    <span className="text-xs text-muted-foreground">
      {ts('Expires in {time}', { time })}
    </span>
  );
}

/**
 * Shows a request to connect the selected server to Dagu Console while it
 * waits for approval, or why the last request failed with a way to retry.
 * Otherwise it offers to connect the server so that Dagu Console monitors it
 * for free; the page header starts the same request.
 */
export function ConnectSection({
  connect,
  disabled,
}: {
  connect: ReturnType<typeof useLicenseConnect>;
  disabled: boolean;
}) {
  const { ts } = useI18n();
  const { status, starting, error, failure, popupBlocked, start, cancel } =
    connect;
  const pending = status?.state === LicenseConnectStatusState.pending;

  if (failure)
    return (
      <LicenseCallout
        tone="error"
        title={ts('Could not connect to Dagu Console')}
        action={
          <Button
            size="sm"
            variant="outline"
            disabled={disabled || starting}
            onClick={() => void start()}
          >
            {starting && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
            {ts('Try again')}
          </Button>
        }
      >
        {failure}
      </LicenseCallout>
    );
  if (!pending)
    return (
      <section
        className="card-obsidian p-4"
        aria-labelledby="license-free-monitoring"
      >
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="min-w-0 flex-1 space-y-1">
            <h2 id="license-free-monitoring" className="text-sm font-medium">
              {ts('Monitor this server for free')}
            </h2>
            <p className="text-sm">
              {ts(
                "Dagu Console shows this server's runs, failures, and trends, and emails you when a workflow fails or the server stops reporting."
              )}
            </p>
            <p className="text-xs text-muted-foreground">
              {ts(
                'Free for one server: reports every 5 minutes and keeps 7 days of run history. Paid features stay off.'
              )}
            </p>
          </div>
          <Button
            size="sm"
            variant="primary"
            disabled={disabled || starting}
            onClick={() => void start()}
          >
            {starting ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <ExternalLink className="h-3.5 w-3.5" />
            )}
            {ts('Connect for free')}
          </Button>
        </div>
      </section>
    );
  return (
    <section
      className="card-obsidian p-4 space-y-3"
      aria-labelledby="license-connect"
    >
      <h2 id="license-connect" className="text-sm font-medium">
        {ts('Waiting for approval')}
      </h2>
      <p className="text-sm">
        {ts(
          'Approve this server in Dagu Console. Check that the console shows this code:'
        )}
      </p>
      <p className="font-mono text-lg tracking-widest">{status.code}</p>
      {popupBlocked && (
        <p className="text-sm text-warning">
          {ts(
            'Your browser blocked the new tab. Open Dagu Console to continue.'
          )}
        </p>
      )}
      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}
      <div className="flex flex-wrap items-center gap-2">
        <Button asChild size="sm" variant="primary">
          <a href={status.connectUrl} target="_blank" rel="noopener noreferrer">
            <ExternalLink className="h-3.5 w-3.5" />
            {ts('Open Dagu Console')}
          </a>
        </Button>
        <Button size="sm" variant="outline" onClick={() => void cancel()}>
          {ts('Cancel')}
        </Button>
        <Countdown expiresAt={status.expiresAt} />
      </div>
    </section>
  );
}
