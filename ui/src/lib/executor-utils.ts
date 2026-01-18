/**
 * Utility functions for executor-specific display logic.
 *
 * @module lib/executor-utils
 */
import type { components } from '@/api/v2/schema';

/**
 * Get displayable command from executor config when step.commands is empty.
 * Returns null if no displayable command can be extracted.
 */
export function getExecutorCommand(
  step: components['schemas']['Step']
): string | null {
  const type = step.executorConfig?.type;
  const config = step.executorConfig?.config as Record<string, unknown>;

  if (!type || !config) return null;

  switch (type) {
    case 'redis':
      if (config.command) {
        const parts = [config.command as string];
        if (config.key) parts.push(config.key as string);
        return parts.join(' ');
      }
      return null;
    case 'sql':
      return config.query ? String(config.query) : null;
    case 'http':
      return config.url ? `${config.method || 'GET'} ${config.url}` : null;
    case 'mail':
      return config.to ? `Mail to ${config.to}` : null;
    case 'jq':
      return config.expression ? `jq: ${config.expression}` : null;
    case 'docker':
      return config.image ? `docker: ${config.image}` : null;
    default:
      return null;
  }
}
