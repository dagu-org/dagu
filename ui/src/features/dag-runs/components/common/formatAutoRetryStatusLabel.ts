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
  return `${baseLabel} ${normalizedCount}/${autoRetryLimit}`;
}

export default formatAutoRetryStatusLabel;
