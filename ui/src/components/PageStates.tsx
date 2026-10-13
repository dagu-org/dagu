// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { AlertTriangle, FileQuestion, RefreshCw } from 'lucide-react';
import type { ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { Button } from '@/components/ui/button';
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty';
import LoadingIndicator from '@/components/ui/loading-indicator';
import { useI18n } from '@/i18n/I18nProvider';
import { getQueryErrorMessage } from '@/lib/queryErrors';

/** Placeholder shown while a page's primary resource is loading. */
export function PageLoading(): ReactNode {
  const { ts } = useI18n();
  return (
    <div
      role="status"
      aria-label={ts('Loading...')}
      className="flex min-h-48 flex-1 items-center justify-center"
    >
      <LoadingIndicator />
    </div>
  );
}

type NotFoundStateProps = {
  title: ReactNode;
  description: ReactNode;
  /** Primary way out, such as the parent list. */
  action?: { to: string; label: ReactNode };
};

/** Explains that the requested resource does not exist. */
export function NotFoundState({
  title,
  description,
  action,
}: NotFoundStateProps): ReactNode {
  return (
    <Empty>
      <EmptyHeader>
        <EmptyMedia>
          <FileQuestion />
        </EmptyMedia>
        <EmptyTitle>{title}</EmptyTitle>
        <EmptyDescription>{description}</EmptyDescription>
      </EmptyHeader>
      {action && (
        <EmptyContent>
          <Button asChild variant="primary">
            <Link to={action.to}>{action.label}</Link>
          </Button>
        </EmptyContent>
      )}
    </Empty>
  );
}

type LoadErrorStateProps = {
  title: ReactNode;
  error: unknown;
  onRetry?: () => void;
};

/** Reports a failed load with the server's message and an optional retry. */
export function LoadErrorState({
  title,
  error,
  onRetry,
}: LoadErrorStateProps): ReactNode {
  const { ts } = useI18n();
  return (
    <Empty role="alert">
      <EmptyHeader>
        <EmptyMedia tone="error">
          <AlertTriangle />
        </EmptyMedia>
        <EmptyTitle>{title}</EmptyTitle>
        <EmptyDescription>
          {getQueryErrorMessage(error, ts('The server did not respond.'))}
        </EmptyDescription>
      </EmptyHeader>
      {onRetry && (
        <EmptyContent>
          <Button onClick={onRetry}>
            <RefreshCw />
            {ts('Try again')}
          </Button>
        </EmptyContent>
      )}
    </Empty>
  );
}

type InlineLoadErrorProps = {
  error: unknown;
  onRetry?: () => void;
};

/** Compact load failure for a region inside a page, such as a table body. */
export function InlineLoadError({
  error,
  onRetry,
}: InlineLoadErrorProps): ReactNode {
  const { ts } = useI18n();
  return (
    <div
      role="alert"
      className="flex flex-wrap items-center justify-center gap-x-3 gap-y-1 text-sm text-error"
    >
      <span>
        {getQueryErrorMessage(error, ts('The server did not respond.'))}
      </span>
      {onRetry && (
        <Button variant="link" size="xs" className="px-0" onClick={onRetry}>
          {ts('Try again')}
        </Button>
      )}
    </div>
  );
}
