import * as React from 'react';
import { cn } from '@/lib/utils';

interface SubTitleProps {
  children?: React.ReactNode;
  className?: string;
}

export default function SubTitle({ children, className }: SubTitleProps) {
  return (
    <h3
      className={cn(
        'text-xl font-semibold text-slate-700 dark:text-slate-300 mb-3',
        className
      )}
    >
      {children}
    </h3>
  );
}
