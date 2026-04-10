import type { components } from '@/api/v1/schema';

type Step = components['schemas']['Step'];

type HarnessConfig = Record<string, unknown>;

const reservedHarnessKeys = new Set(['provider', 'fallback']);

export type HarnessAttemptSummary = {
  label: string;
  provider: string | null;
  options: string[];
};

export type HarnessStepSummary = {
  prompt: string | null;
  attempts: HarnessAttemptSummary[];
};

export function isHarnessStep(step: Step): boolean {
  return step.executorConfig?.type === 'harness';
}

export function getHarnessStepSummary(
  step: Step
): HarnessStepSummary | null {
  if (!isHarnessStep(step)) {
    return null;
  }

  const config = toHarnessConfig(step.executorConfig?.config);
  const attempts: HarnessAttemptSummary[] = [];

  if (config) {
    attempts.push({
      label: 'Primary',
      provider: getProvider(config),
      options: getHarnessOptionLabels(config),
    });

    const fallbackConfigs = getFallbackConfigs(config.fallback);
    for (const [index, fallbackConfig] of fallbackConfigs.entries()) {
      attempts.push({
        label: `Fallback ${index + 1}`,
        provider: getProvider(fallbackConfig),
        options: getHarnessOptionLabels(fallbackConfig),
      });
    }
  }

  return {
    prompt: getStepPrompt(step),
    attempts,
  };
}

function getStepPrompt(step: Step): string | null {
  const firstCommand = step.commands?.[0];

  if (!firstCommand) {
    return null;
  }

  const commandWithArgs = (
    firstCommand as components['schemas']['CommandEntry'] & {
      cmdWithArgs?: string;
    }
  ).cmdWithArgs;

  if (typeof commandWithArgs === 'string' && commandWithArgs.trim()) {
    return commandWithArgs.trim();
  }

  if (firstCommand.args?.length) {
    return `${firstCommand.command} ${firstCommand.args.join(' ')}`.trim();
  }

  return firstCommand.command?.trim() || null;
}

function toHarnessConfig(value: unknown): HarnessConfig | null {
  if (!isRecord(value)) {
    return null;
  }

  return value;
}

function getProvider(config: HarnessConfig): string | null {
  const provider = config.provider;
  return typeof provider === 'string' && provider.trim()
    ? provider.trim()
    : null;
}

function getFallbackConfigs(value: unknown): HarnessConfig[] {
  if (!Array.isArray(value)) {
    return [];
  }

  return value.filter(isRecord);
}

function getHarnessOptionLabels(config: HarnessConfig): string[] {
  return Object.keys(config)
    .sort((left, right) => left.localeCompare(right))
    .filter((key) => !reservedHarnessKeys.has(key))
    .map((key) => formatHarnessOption(key, config[key]))
    .filter((value): value is string => Boolean(value));
}

function formatHarnessOption(
  key: string,
  value: unknown
): string | null {
  if (typeof value === 'boolean') {
    return value ? key : null;
  }

  const formattedValue = formatHarnessValue(value);
  if (!formattedValue) {
    return null;
  }

  return `${key}=${formattedValue}`;
}

function formatHarnessValue(value: unknown): string | null {
  if (typeof value === 'string') {
    return value.trim() || null;
  }

  if (typeof value === 'number') {
    return Number.isFinite(value) ? String(value) : null;
  }

  if (Array.isArray(value)) {
    const items = value
      .map((item) => formatArrayItem(item))
      .filter((item): item is string => Boolean(item));

    return items.length > 0 ? `[${items.join(', ')}]` : null;
  }

  if (isRecord(value)) {
    return JSON.stringify(value);
  }

  return null;
}

function formatArrayItem(value: unknown): string | null {
  if (typeof value === 'string') {
    return value.trim() || null;
  }

  if (typeof value === 'number') {
    return Number.isFinite(value) ? String(value) : null;
  }

  if (typeof value === 'boolean') {
    return value ? 'true' : 'false';
  }

  if (isRecord(value)) {
    return JSON.stringify(value);
  }

  return null;
}

function isRecord(value: unknown): value is HarnessConfig {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}
