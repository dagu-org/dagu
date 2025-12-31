import { cn } from '@/lib/utils';
import type { SchemaPropertyInfo as SchemaPropertyInfoType } from '@/lib/schema-utils';
import { PropertyTypeDisplay } from './PropertyTypeDisplay';
import { NestedPropertiesTree } from './NestedPropertiesTree';

interface SchemaPropertyInfoProps {
  propertyInfo: SchemaPropertyInfoType;
  className?: string;
}

export function SchemaPropertyInfo({
  propertyInfo,
  className,
}: SchemaPropertyInfoProps) {
  return (
    <div className={cn('space-y-3', className)}>
      {/* Property Name and Type */}
      <div>
        <h3 className="text-sm font-semibold text-foreground mb-1">
          {propertyInfo.title || propertyInfo.name}
        </h3>
        <PropertyTypeDisplay
          type={propertyInfo.type}
          required={propertyInfo.required}
        />
      </div>

      {/* Description */}
      {propertyInfo.description && (
        <div>
          <p className="text-xs text-muted-foreground leading-relaxed">
            {propertyInfo.description}
          </p>
        </div>
      )}

      {/* Default Value */}
      {propertyInfo.default !== undefined && (
        <div>
          <h4 className="text-[10px] font-medium text-muted-foreground uppercase tracking-wide mb-0.5">
            Default
          </h4>
          <code className="text-xs bg-slate-200 dark:bg-slate-700 text-slate-700 dark:text-slate-200 px-1.5 py-0.5 rounded font-mono">
            {JSON.stringify(propertyInfo.default)}
          </code>
        </div>
      )}

      {/* Enum Values */}
      {propertyInfo.enum && propertyInfo.enum.length > 0 && (
        <div>
          <h4 className="text-[10px] font-medium text-muted-foreground uppercase tracking-wide mb-1">
            Allowed Values
          </h4>
          <div className="flex flex-wrap gap-1">
            {propertyInfo.enum.map((value, i) => (
              <code
                key={i}
                className="text-[10px] bg-slate-200 dark:bg-slate-700 text-slate-700 dark:text-slate-200 px-1.5 py-0.5 rounded font-mono"
              >
                {JSON.stringify(value)}
              </code>
            ))}
          </div>
        </div>
      )}

      {/* Format */}
      {propertyInfo.format && (
        <div>
          <h4 className="text-[10px] font-medium text-muted-foreground uppercase tracking-wide mb-0.5">
            Format
          </h4>
          <code className="text-xs bg-slate-200 dark:bg-slate-700 text-slate-700 dark:text-slate-200 px-1.5 py-0.5 rounded font-mono">
            {propertyInfo.format}
          </code>
        </div>
      )}

      {/* Pattern */}
      {propertyInfo.pattern && (
        <div>
          <h4 className="text-[10px] font-medium text-muted-foreground uppercase tracking-wide mb-0.5">
            Pattern
          </h4>
          <code className="text-[10px] bg-slate-200 dark:bg-slate-700 text-slate-700 dark:text-slate-200 px-1.5 py-0.5 rounded font-mono break-all">
            {propertyInfo.pattern}
          </code>
        </div>
      )}

      {/* Examples */}
      {propertyInfo.examples && propertyInfo.examples.length > 0 && (
        <div>
          <h4 className="text-[10px] font-medium text-muted-foreground uppercase tracking-wide mb-1">
            Examples
          </h4>
          <div className="space-y-1">
            {propertyInfo.examples.map((example, i) => (
              <code
                key={i}
                className="block text-[10px] bg-slate-200 dark:bg-slate-700 text-slate-700 dark:text-slate-200 px-1.5 py-0.5 rounded font-mono break-all"
              >
                {typeof example === 'string'
                  ? example
                  : JSON.stringify(example, null, 2)}
              </code>
            ))}
          </div>
        </div>
      )}

      {/* OneOf Options */}
      {propertyInfo.oneOf && propertyInfo.oneOf.length > 0 && (
        <div>
          <h4 className="text-[10px] font-medium text-muted-foreground uppercase tracking-wide mb-1">
            One Of
          </h4>
          <div className="space-y-1">
            {propertyInfo.oneOf.map((option, i) => (
              <div
                key={i}
                className="text-xs p-1.5 bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-200 rounded border border-slate-200 dark:border-slate-700"
              >
                <PropertyTypeDisplay type={option.type} />
                {option.description && (
                  <p className="text-[10px] text-slate-600 dark:text-slate-400 mt-0.5">
                    {option.description}
                  </p>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Nested Properties */}
      {propertyInfo.properties &&
        Object.keys(propertyInfo.properties).length > 0 && (
          <div>
            <h4 className="text-[10px] font-medium text-muted-foreground uppercase tracking-wide mb-1">
              Properties
            </h4>
            <div className="border border-slate-200 dark:border-slate-700 rounded p-1">
              <NestedPropertiesTree properties={propertyInfo.properties} />
            </div>
          </div>
        )}

      {/* Array Items */}
      {propertyInfo.items && (
        <div>
          <h4 className="text-[10px] font-medium text-muted-foreground uppercase tracking-wide mb-1">
            Array Items
          </h4>
          <div className="text-xs p-1.5 bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-200 rounded border border-slate-200 dark:border-slate-700">
            <PropertyTypeDisplay type={propertyInfo.items.type} />
            {propertyInfo.items.description && (
              <p className="text-[10px] text-slate-600 dark:text-slate-400 mt-0.5">
                {propertyInfo.items.description}
              </p>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
