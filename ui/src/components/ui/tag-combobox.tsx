import * as React from 'react';
import { X, ChevronDown } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';

interface TagComboboxProps {
  selectedTags: string[];
  onTagsChange: (tags: string[]) => void;
  availableTags: string[];
  placeholder?: string;
  className?: string;
}

function TagCombobox({
  selectedTags,
  onTagsChange,
  availableTags,
  placeholder = 'Filter by tags...',
  className,
}: TagComboboxProps) {
  const [inputValue, setInputValue] = React.useState('');
  const [isOpen, setIsOpen] = React.useState(false);
  const [highlightedIndex, setHighlightedIndex] = React.useState(-1);
  const containerRef = React.useRef<HTMLDivElement>(null);
  const inputRef = React.useRef<HTMLInputElement>(null);

  // Filter suggestions based on input value
  const filteredSuggestions = React.useMemo(() => {
    const selectedLower = new Set(selectedTags.map((t) => t.toLowerCase()));
    const available = availableTags.filter(
      (tag) => !selectedLower.has(tag.toLowerCase())
    );

    if (!inputValue.trim()) {
      return available;
    }

    const searchLower = inputValue.toLowerCase().trim();
    return available.filter((tag) =>
      tag.toLowerCase().includes(searchLower)
    );
  }, [inputValue, availableTags, selectedTags]);

  // Reset highlighted index when suggestions change
  React.useEffect(() => {
    setHighlightedIndex(-1);
  }, [filteredSuggestions]);

  // Handle click outside to close dropdown
  React.useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (
        containerRef.current &&
        !containerRef.current.contains(event.target as Node)
      ) {
        setIsOpen(false);
      }
    };

    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const addTag = (tag: string) => {
    const normalized = tag.toLowerCase().trim();
    if (!normalized) return;

    // Check for duplicates (case-insensitive)
    if (selectedTags.some((t) => t.toLowerCase() === normalized)) {
      return;
    }

    onTagsChange([...selectedTags, normalized]);
    setInputValue('');
    setIsOpen(false);
    inputRef.current?.focus();
  };

  const removeTag = (tagToRemove: string) => {
    onTagsChange(selectedTags.filter((t) => t !== tagToRemove));
    inputRef.current?.focus();
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    switch (e.key) {
      case 'Enter':
        e.preventDefault();
        if (highlightedIndex >= 0 && highlightedIndex < filteredSuggestions.length) {
          const tag = filteredSuggestions[highlightedIndex];
          if (tag) addTag(tag);
        } else if (inputValue.trim()) {
          addTag(inputValue);
        }
        break;

      case 'Backspace':
        if (!inputValue && selectedTags.length > 0) {
          const lastTag = selectedTags[selectedTags.length - 1];
          if (lastTag) removeTag(lastTag);
        }
        break;

      case 'Escape':
        setIsOpen(false);
        setHighlightedIndex(-1);
        break;

      case 'ArrowDown':
        e.preventDefault();
        setIsOpen(true);
        setHighlightedIndex((prev) =>
          prev < filteredSuggestions.length - 1 ? prev + 1 : 0
        );
        break;

      case 'ArrowUp':
        e.preventDefault();
        setIsOpen(true);
        setHighlightedIndex((prev) =>
          prev > 0 ? prev - 1 : filteredSuggestions.length - 1
        );
        break;

      default:
        break;
    }
  };

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setInputValue(e.target.value);
    setIsOpen(true);
  };

  const handleInputFocus = () => {
    setIsOpen(true);
  };

  const handleSuggestionClick = (tag: string) => {
    addTag(tag);
  };

  const clearAll = () => {
    onTagsChange([]);
    setInputValue('');
    inputRef.current?.focus();
  };

  return (
    <div ref={containerRef} className={cn('relative', className)}>
      <div
        className={cn(
          'flex flex-wrap items-center gap-1 min-h-[36px] px-2 py-1 rounded-md border border-border bg-input cursor-text',
          'focus-within:border-ring focus-within:ring-ring/50 focus-within:ring-[3px]'
        )}
        onClick={() => inputRef.current?.focus()}
      >
        {selectedTags.map((tag) => (
          <Badge
            key={tag}
            variant="secondary"
            className="text-xs h-6 px-2 gap-1"
          >
            {tag}
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                e.preventDefault();
                removeTag(tag);
              }}
              onMouseDown={(e) => {
                e.stopPropagation();
                e.preventDefault();
              }}
              className="ml-0.5 rounded-sm hover:bg-muted-foreground/20 hover:text-destructive focus:outline-none"
            >
              <X className="h-3 w-3" />
            </button>
          </Badge>
        ))}
        <input
          ref={inputRef}
          type="text"
          value={inputValue}
          onChange={handleInputChange}
          onKeyDown={handleKeyDown}
          onFocus={handleInputFocus}
          placeholder={selectedTags.length === 0 ? placeholder : ''}
          className="flex-1 min-w-[80px] h-6 bg-transparent border-none outline-none text-sm placeholder:text-muted-foreground"
        />
        <div className="flex items-center gap-1 ml-auto">
          {selectedTags.length > 0 && (
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                clearAll();
              }}
              className="p-0.5 rounded hover:bg-muted text-muted-foreground hover:text-foreground"
              title="Clear all tags"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          )}
          <ChevronDown
            className={cn(
              'h-4 w-4 text-muted-foreground transition-transform',
              isOpen && 'rotate-180'
            )}
          />
        </div>
      </div>

      {isOpen && filteredSuggestions.length > 0 && (
        <div className="absolute z-50 mt-1 w-full max-h-[200px] overflow-y-auto rounded-md border border-border bg-popover shadow-md">
          {filteredSuggestions.map((tag, index) => (
            <div
              key={tag}
              className={cn(
                'px-3 py-1.5 text-sm cursor-pointer',
                index === highlightedIndex
                  ? 'bg-accent text-accent-foreground'
                  : 'hover:bg-muted'
              )}
              onClick={() => handleSuggestionClick(tag)}
              onMouseEnter={() => setHighlightedIndex(index)}
            >
              {tag}
            </div>
          ))}
        </div>
      )}

      {isOpen && inputValue.trim() && !filteredSuggestions.some(
        (t) => t.toLowerCase() === inputValue.toLowerCase().trim()
      ) && (
        <div className="absolute z-50 mt-1 w-full rounded-md border border-border bg-popover shadow-md">
          <div
            className={cn(
              'px-3 py-1.5 text-sm cursor-pointer hover:bg-muted',
              filteredSuggestions.length === 0 && highlightedIndex === -1
                ? 'bg-accent text-accent-foreground'
                : ''
            )}
            onClick={() => addTag(inputValue)}
          >
            Add "{inputValue.trim()}"
          </div>
          {filteredSuggestions.map((tag, index) => (
            <div
              key={tag}
              className={cn(
                'px-3 py-1.5 text-sm cursor-pointer',
                index === highlightedIndex
                  ? 'bg-accent text-accent-foreground'
                  : 'hover:bg-muted'
              )}
              onClick={() => handleSuggestionClick(tag)}
              onMouseEnter={() => setHighlightedIndex(index)}
            >
              {tag}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

export { TagCombobox };
