// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

type AutoRetryLabelInput = {
  statusLabel?: string | null;
  autoRetryCount?: number | null;
  autoRetryLimit?: number | null;
};

function formatAutoRetryStatusLabel({
  statusLabel,
  autoRetryCount,
  autoRetryLimit,
}: AutoRetryLabelInput): string {
  const baseLabel = statusLabel || '';
  if (!autoRetryLimit || autoRetryLimit <= 0) {
    return baseLabel;
  }

  const normalizedCount = Math.max(autoRetryCount ?? 0, 0);
  const retryLabel = `${normalizedCount}/${autoRetryLimit}`;
  return baseLabel ? `${baseLabel} ${retryLabel}` : retryLabel;
}

export default formatAutoRetryStatusLabel;
