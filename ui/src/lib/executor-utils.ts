/**
 * Utility functions for executor-specific display logic.
 *
 * @module lib/executor-utils
 */
import type { components } from '@/api/v1/schema';

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
    case 'redis': {
      if (!config.command) return null;
      const cmd = config.command as string;
      return config.key ? `${cmd} ${config.key}` : cmd;
    }
    case 's3': {
      const parts: string[] = [];
      if (config.command) {
        parts.push(String(config.command));
      }
      if (config.bucket) {
        parts.push(`s3://${config.bucket}`);
      }
      if (config.key) {
        parts.push(String(config.key));
      } else if (config.prefix) {
        parts.push(`${config.prefix}*`);
      }
      return parts.length > 0 ? parts.join(' ') : null;
    }
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
    case 'router':
      return config.value ? `route: ${config.value}` : 'router';
    default:
      return null;
  }
}
