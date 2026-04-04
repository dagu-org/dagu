import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';
import * as React from 'react';

type AutocompleteInputProps = Omit<
  React.ComponentProps<typeof Input>,
  'onChange' | 'value'
> & {
  value: string;
  onValueChange: (value: string) => void;
  options: string[];
  onOptionSelect?: (value: string) => void;
  onOpenChange?: (open: boolean) => void;
  onSubmit?: () => void;
  loading?: boolean;
  emptyText?: string;
  listClassName?: string;
};

export function AutocompleteInput({
  value,
  onValueChange,
  options,
  onOptionSelect,
  onOpenChange,
  onSubmit,
  loading = false,
  emptyText = 'No matches found.',
  className,
  listClassName,
  disabled,
  ...props
}: AutocompleteInputProps) {
  const inputRef = React.useRef<HTMLInputElement>(null);
  const containerRef = React.useRef<HTMLDivElement>(null);
  const listboxID = React.useId();
  const optionIDPrefix = React.useId();
  const normalizedValue = value.trim();
  const hasQuery = normalizedValue.length > 0;
  const [isOpen, setIsOpen] = React.useState(false);
  const [highlightedIndex, setHighlightedIndex] = React.useState(-1);

  const setOpen = React.useCallback(
    (next: boolean) => {
      setIsOpen(next);
      onOpenChange?.(next);
    },
    [onOpenChange]
  );

  React.useEffect(() => {
    setHighlightedIndex(-1);
  }, [options]);

  React.useEffect(() => {
    if (!isOpen) {
      setHighlightedIndex(-1);
    }
  }, [isOpen]);

  React.useEffect(() => {
    if (disabled) {
      setOpen(false);
    }
  }, [disabled, setOpen]);

  React.useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (
        containerRef.current &&
        !containerRef.current.contains(event.target as Node)
      ) {
        setOpen(false);
      }
    };

    document.addEventListener('mousedown', handleClickOutside);
    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
    };
  }, [setOpen]);

  const handleSelect = React.useCallback(
    (nextValue: string) => {
      onValueChange(nextValue);
      onOptionSelect?.(nextValue);
      setOpen(false);
      inputRef.current?.focus();
    },
    [onOptionSelect, onValueChange, setOpen]
  );

  const handleChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const nextValue = event.target.value;
    onValueChange(nextValue);
    setOpen(nextValue.trim().length > 0);
  };

  const handleFocus = () => {
    if (!disabled && hasQuery) {
      setOpen(true);
    }
  };

  const handleKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
    if (disabled) {
      return;
    }

    switch (event.key) {
      case 'ArrowDown':
        if (!hasQuery) {
          return;
        }
        event.preventDefault();
        setOpen(true);
        setHighlightedIndex((prev) => {
          if (options.length === 0) {
            return -1;
          }
          return prev < options.length - 1 ? prev + 1 : 0;
        });
        break;
      case 'ArrowUp':
        if (!hasQuery) {
          return;
        }
        event.preventDefault();
        setOpen(true);
        setHighlightedIndex((prev) => {
          if (options.length === 0) {
            return -1;
          }
          return prev > 0 ? prev - 1 : options.length - 1;
        });
        break;
      case 'Enter':
        if (
          isOpen &&
          highlightedIndex >= 0 &&
          highlightedIndex < options.length
        ) {
          event.preventDefault();
          const option = options[highlightedIndex];
          if (option) {
            handleSelect(option);
          }
          return;
        }
        event.preventDefault();
        onSubmit?.();
        setOpen(false);
        break;
      case 'Escape':
        if (isOpen) {
          event.preventDefault();
          setOpen(false);
        }
        break;
      default:
        break;
    }
  };

  const activeDescendant =
    highlightedIndex >= 0 && highlightedIndex < options.length
      ? `${optionIDPrefix}-${highlightedIndex}`
      : undefined;
  const showListbox = isOpen && hasQuery;
  const showEmptyState = !loading && options.length === 0;

  return (
    <div ref={containerRef} className="relative">
      <Input
        ref={inputRef}
        value={value}
        onChange={handleChange}
        onFocus={handleFocus}
        onKeyDown={handleKeyDown}
        role="combobox"
        aria-expanded={showListbox}
        aria-haspopup="listbox"
        aria-autocomplete="list"
        aria-controls={showListbox ? listboxID : undefined}
        aria-activedescendant={activeDescendant}
        autoComplete="off"
        disabled={disabled}
        className={className}
        {...props}
      />
      {showListbox ? (
        <div
          id={listboxID}
          role="listbox"
          className={cn(
            'absolute z-50 mt-1 w-full overflow-hidden rounded-md border border-border bg-popover shadow-md',
            listClassName
          )}
        >
          {loading ? (
            <div className="px-3 py-2 text-sm text-muted-foreground">
              Loading...
            </div>
          ) : null}
          {showEmptyState ? (
            <div className="px-3 py-2 text-sm text-muted-foreground">
              {emptyText}
            </div>
          ) : null}
          {!loading
            ? options.map((option, index) => {
                const isHighlighted = highlightedIndex === index;
                return (
                  <button
                    key={`${option}-${index}`}
                    id={`${optionIDPrefix}-${index}`}
                    type="button"
                    role="option"
                    aria-selected={isHighlighted}
                    className={cn(
                      'block w-full px-3 py-2 text-left text-sm',
                      'hover:bg-accent hover:text-accent-foreground',
                      isHighlighted && 'bg-accent text-accent-foreground'
                    )}
                    onMouseDown={(event) => {
                      event.preventDefault();
                    }}
                    onClick={() => handleSelect(option)}
                    onMouseEnter={() => setHighlightedIndex(index)}
                  >
                    {option}
                  </button>
                );
              })
            : null}
        </div>
      ) : null}
    </div>
  );
}
