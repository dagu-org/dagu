import { cn } from '@/lib/utils';
import { ReactNode } from 'react';

type LabeledItemProps = {
  label: string;
  children: string | ReactNode;
  className?: string;
};

export default function LabeledItem({
  label,
  children,
  className,
}: LabeledItemProps) {
  return (
    <div className={cn('flex flex-row items-center', className)}>
      <span className="text-sm font-semibold text-foreground/90">
        {label}:&nbsp;
      </span>
      {typeof children === 'string' ? (
        <span className="text-sm text-muted-foreground">
          {children}
        </span>
      ) : (
        children
      )}
    </div>
  );
}
