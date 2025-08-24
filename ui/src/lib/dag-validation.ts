/**
 * DAG name validation utilities
 */

// Regex pattern for valid DAG names
export const DAG_NAME_PATTERN = /^[a-zA-Z0-9_.-]+$/;

// Pattern string for display
export const DAG_NAME_PATTERN_STRING = '^[a-zA-Z0-9_.-]+$';

// Error messages
export const DAG_NAME_ERROR_MESSAGES = {
  empty: 'DAG name cannot be empty',
  invalid: 'DAG name can only contain letters, numbers, underscores, dots, and hyphens',
  space: 'DAG name cannot contain spaces',
} as const;

/**
 * Validates a DAG name
 * @param name The DAG name to validate
 * @returns Object containing validation result and error message
 */
export function validateDAGName(name: string): { isValid: boolean; error?: string } {
  const trimmedName = name.trim();
  
  if (!trimmedName) {
    return { isValid: false, error: DAG_NAME_ERROR_MESSAGES.empty };
  }
  
  if (!DAG_NAME_PATTERN.test(trimmedName)) {
    // Check for spaces specifically to provide better error message
    if (name.includes(' ')) {
      return { isValid: false, error: DAG_NAME_ERROR_MESSAGES.space };
    }
    return { isValid: false, error: DAG_NAME_ERROR_MESSAGES.invalid };
  }
  
  return { isValid: true };
}

/**
 * Quick check if a DAG name is valid
 * @param name The DAG name to check
 * @returns true if valid, false otherwise
 */
export function isValidDAGName(name: string): boolean {
  return validateDAGName(name).isValid;
}