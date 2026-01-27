import * as React from 'react';
import { useCallback, useId } from 'react';

import { cn } from '@/lib/utils';

import './switch.css';

interface SwitchProps extends Omit<React.ButtonHTMLAttributes<HTMLButtonElement>, 'onChange'> {
  checked?: boolean;
  defaultChecked?: boolean;
  onCheckedChange?: (checked: boolean) => void;
  disabled?: boolean;
  required?: boolean;
  name?: string;
  value?: string;
}

/**
 * Custom Switch component that replaces Radix Switch to avoid
 * "Maximum update depth exceeded" errors with React 19.
 * See: https://github.com/radix-ui/primitives/issues/2152
 */
function Switch({
  className,
  checked,
  defaultChecked = false,
  onCheckedChange,
  disabled,
  required,
  name,
  value = 'on',
  id,
  ...props
}: SwitchProps) {
  const [internalChecked, setInternalChecked] = React.useState(defaultChecked);
  const isControlled = checked !== undefined;
  const isChecked = isControlled ? checked : internalChecked;
  const generatedId = useId();
  const switchId = id || generatedId;

  const handleClick = useCallback(() => {
    if (disabled) return;

    const newValue = !isChecked;
    if (!isControlled) {
      setInternalChecked(newValue);
    }
    onCheckedChange?.(newValue);
  }, [disabled, isChecked, isControlled, onCheckedChange]);

  const handleKeyDown = useCallback((event: React.KeyboardEvent) => {
    if (event.key === 'Enter') {
      event.preventDefault();
      handleClick();
    }
  }, [handleClick]);

  const state = isChecked ? 'checked' : 'unchecked';

  return (
    <button
      type="button"
      role="switch"
      aria-checked={isChecked}
      aria-required={required}
      data-state={state}
      data-disabled={disabled ? '' : undefined}
      disabled={disabled}
      id={switchId}
      data-slot="switch"
      className={cn('switch-root', className)}
      onClick={handleClick}
      onKeyDown={handleKeyDown}
      {...props}
    >
      <span
        data-state={state}
        data-slot="switch-thumb"
        className="switch-thumb"
      />
      {name && (
        <input
          type="checkbox"
          name={name}
          value={value}
          checked={isChecked}
          required={required}
          disabled={disabled}
          onChange={() => {}}
          style={{
            position: 'absolute',
            pointerEvents: 'none',
            opacity: 0,
            margin: 0,
            width: 1,
            height: 1,
          }}
          tabIndex={-1}
        />
      )}
    </button>
  );
}

export { Switch };
