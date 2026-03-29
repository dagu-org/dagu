// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import React from 'react';

type LegacyReactInternals = {
  ReactCurrentDispatcher?: { current: unknown };
  ReactCurrentOwner?: { current: unknown };
  ReactDebugCurrentFrame?: {
    getStackAddendum: () => string;
    setExtraStackFrame: (stack: string | null | undefined) => void;
  };
};

type ReactWithLegacyInternals = typeof React & {
  __SECRET_INTERNALS_DO_NOT_USE_OR_YOU_WILL_BE_FIRED?: LegacyReactInternals;
};

// Scalar's React wrapper bundles a React 18 runtime that still reaches into
// legacy internals removed from React 19.
export function ensureReactLegacyInternals(): void {
  const reactWithLegacyInternals = React as ReactWithLegacyInternals;
  const legacyInternals = reactWithLegacyInternals.__SECRET_INTERNALS_DO_NOT_USE_OR_YOU_WILL_BE_FIRED ?? {};

  legacyInternals.ReactCurrentDispatcher ??= { current: null };
  legacyInternals.ReactCurrentOwner ??= { current: null };
  legacyInternals.ReactDebugCurrentFrame ??= {
    getStackAddendum: () => '',
    setExtraStackFrame: () => undefined,
  };

  reactWithLegacyInternals.__SECRET_INTERNALS_DO_NOT_USE_OR_YOU_WILL_BE_FIRED = legacyInternals;
}

ensureReactLegacyInternals();
