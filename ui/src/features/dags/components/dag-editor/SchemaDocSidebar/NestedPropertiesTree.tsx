import { useState } from 'react';
import { cn } from '@/lib/utils';
import { ChevronDown, ChevronRight } from 'lucide-react';
import type { SchemaPropertyInfo } from '@/lib/schema-utils';
import { PropertyTypeDisplay } from './PropertyTypeDisplay';

interface NestedPropertiesTreeProps {
  properties: Record<string, SchemaPropertyInfo>;
  className?: string;
  maxDepth?: number;
  currentDepth?: number;
}

export function NestedPropertiesTree({
  properties,
  className,
  maxDepth = 2,
  currentDepth = 0,
}: NestedPropertiesTreeProps) {
  const entries = Object.entries(properties);

  if (entries.length === 0) {
    return null;
  }

  return (
    <div className={cn('space-y-0.5', className)}>
      {entries.map(([key, prop]) => (
        <PropertyItem
          key={key}
          name={key}
          property={prop}
          maxDepth={maxDepth}
          currentDepth={currentDepth}
        />
      ))}
    </div>
  );
}

interface PropertyItemProps {
  name: string;
  property: SchemaPropertyInfo;
  maxDepth: number;
  currentDepth: number;
}

function PropertyItem({
  name,
  property,
  maxDepth,
  currentDepth,
}: PropertyItemProps) {
  const [isExpanded, setIsExpanded] = useState(false);
  const hasNestedProperties =
    property.properties && Object.keys(property.properties).length > 0;
  const canExpand = hasNestedProperties && currentDepth < maxDepth;

  return (
    <div className="text-xs">
      <div
        className={cn(
          'flex items-center gap-1 py-0.5 px-1 rounded hover:bg-muted/50',
          canExpand && 'cursor-pointer'
        )}
        onClick={() => canExpand && setIsExpanded(!isExpanded)}
      >
        {canExpand ? (
          isExpanded ? (
            <ChevronDown className="w-3 h-3 text-muted-foreground shrink-0" />
          ) : (
            <ChevronRight className="w-3 h-3 text-muted-foreground shrink-0" />
          )
        ) : (
          <span className="w-3 shrink-0" />
        )}
        <span
          className={cn(
            'font-medium truncate',
            property.required
              ? 'text-foreground'
              : 'text-muted-foreground'
          )}
        >
          {name}
        </span>
        <PropertyTypeDisplay
          type={property.type}
          className="ml-auto shrink-0"
        />
      </div>
      {property.description && (
        <div className="pl-5 pr-1 text-xs text-muted-foreground truncate">
          {property.description}
        </div>
      )}
      {isExpanded && hasNestedProperties && (
        <div className="pl-3 border-l border-border ml-1.5 mt-0.5">
          <NestedPropertiesTree
            properties={property.properties!}
            maxDepth={maxDepth}
            currentDepth={currentDepth + 1}
          />
        </div>
      )}
    </div>
  );
}
