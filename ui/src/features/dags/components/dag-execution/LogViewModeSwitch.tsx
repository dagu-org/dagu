// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import React from 'react';
import { ToggleButton, ToggleGroup } from '@/components/ui/toggle-group';
import { useI18n } from '@/i18n/I18nProvider';

export type LogViewMode = 'tail' | 'head' | 'page';

const MODES: { value: LogViewMode; label: string }[] = [
  { value: 'tail', label: 'Show end' },
  { value: 'head', label: 'Show beginning' },
  { value: 'page', label: 'Page View' },
];

/** Chooses which part of a log is loaded: its end, its start, or one page. */
export function LogViewModeSwitch({
  value,
  onChange,
  disabled,
}: {
  value: LogViewMode;
  onChange: (mode: LogViewMode) => void;
  disabled?: boolean;
}): React.ReactElement {
  const { ts } = useI18n();
  return (
    <ToggleGroup aria-label={ts('Lines shown')} className="p-0.5">
      {MODES.map((mode) => (
        <ToggleButton
          key={mode.value}
          value={mode.value}
          groupValue={value}
          onClick={() => onChange(mode.value)}
          disabled={disabled}
          className="h-7 px-2.5"
        >
          {ts(mode.label)}
        </ToggleButton>
      ))}
    </ToggleGroup>
  );
}
