import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';

interface PropertyTypeDisplayProps {
  type: string | string[];
  required?: boolean;
  className?: string;
}

const typeColors: Record<string, string> = {
  string: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200',
  number: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200',
  integer: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200',
  boolean: 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200',
  array: 'bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200',
  object: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200',
  enum: 'bg-pink-100 text-pink-800 dark:bg-pink-900 dark:text-pink-200',
  null: 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200',
  unknown: 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400',
};

export function PropertyTypeDisplay({
  type,
  required,
  className,
}: PropertyTypeDisplayProps) {
  const types = Array.isArray(type) ? type : [type];

  return (
    <div className={cn('flex flex-wrap items-center gap-1', className)}>
      {types.map((t, i) => (
        <Badge
          key={i}
          variant="outline"
          className={cn(
            'text-[10px] px-1.5 py-0 h-4 font-mono border-0',
            typeColors[t] || typeColors.unknown
          )}
        >
          {t}
        </Badge>
      ))}
      {required && (
        <Badge
          variant="outline"
          className="text-[10px] px-1.5 py-0 h-4 bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-200 border-0"
        >
          required
        </Badge>
      )}
    </div>
  );
}
