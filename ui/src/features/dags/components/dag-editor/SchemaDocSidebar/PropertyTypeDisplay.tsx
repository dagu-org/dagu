import { cn } from '@/lib/utils';

interface PropertyTypeDisplayProps {
  type: string | string[];
  required?: boolean;
  className?: string;
}

// Sepia-compatible, muted color palette matching existing StatusChip patterns
const typeColors: Record<string, string> = {
  string: 'bg-[rgba(107,168,107,0.12)] text-[#5a8a5a]',
  number: 'bg-[rgba(138,159,196,0.12)] text-[#6a7fa4]',
  integer: 'bg-[rgba(138,159,196,0.12)] text-[#6a7fa4]',
  boolean: 'bg-[rgba(154,122,196,0.12)] text-[#7a5aa4]',
  array: 'bg-[rgba(196,158,106,0.12)] text-[#9a7a4a]',
  object: 'bg-[rgba(107,168,147,0.12)] text-[#5a8a7a]',
  enum: 'bg-[rgba(196,122,156,0.12)] text-[#a45a7a]',
  null: 'bg-muted text-muted-foreground',
  unknown: 'bg-muted text-muted-foreground',
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
        <span
          key={i}
          className={cn(
            'inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium',
            typeColors[t] || typeColors.unknown
          )}
        >
          {t}
        </span>
      ))}
      {required && (
        <span className="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium bg-[rgba(196,114,106,0.12)] text-[#b05a52]">
          required
        </span>
      )}
    </div>
  );
}
