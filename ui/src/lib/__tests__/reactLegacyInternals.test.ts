// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import React from 'react';
import { describe, expect, it } from 'vitest';
import { ensureReactLegacyInternals } from '../reactLegacyInternals';

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

describe('ensureReactLegacyInternals', () => {
  it('hydrates the legacy fields expected by the Scalar wrapper', () => {
    const reactWithLegacyInternals = React as ReactWithLegacyInternals;
    const originalLegacyInternals = reactWithLegacyInternals.__SECRET_INTERNALS_DO_NOT_USE_OR_YOU_WILL_BE_FIRED;

    try {
      delete reactWithLegacyInternals.__SECRET_INTERNALS_DO_NOT_USE_OR_YOU_WILL_BE_FIRED;

      ensureReactLegacyInternals();

      expect(reactWithLegacyInternals.__SECRET_INTERNALS_DO_NOT_USE_OR_YOU_WILL_BE_FIRED).toBeDefined();
      expect(
        reactWithLegacyInternals.__SECRET_INTERNALS_DO_NOT_USE_OR_YOU_WILL_BE_FIRED?.ReactCurrentDispatcher
      ).toEqual({ current: null });
      expect(
        reactWithLegacyInternals.__SECRET_INTERNALS_DO_NOT_USE_OR_YOU_WILL_BE_FIRED?.ReactCurrentOwner
      ).toEqual({ current: null });
      expect(
        reactWithLegacyInternals.__SECRET_INTERNALS_DO_NOT_USE_OR_YOU_WILL_BE_FIRED?.ReactDebugCurrentFrame
      ).toMatchObject({
        getStackAddendum: expect.any(Function),
        setExtraStackFrame: expect.any(Function),
      });
    } finally {
      if (originalLegacyInternals === undefined) {
        delete reactWithLegacyInternals.__SECRET_INTERNALS_DO_NOT_USE_OR_YOU_WILL_BE_FIRED;
      } else {
        reactWithLegacyInternals.__SECRET_INTERNALS_DO_NOT_USE_OR_YOU_WILL_BE_FIRED = originalLegacyInternals;
      }
    }
  });
});
