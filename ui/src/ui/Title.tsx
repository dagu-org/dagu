import * as React from 'react';
import { cn } from '@/lib/utils';

interface TitleProps {
  children?: React.ReactNode;
  className?: string;
}

export default function Title({ children, className }: TitleProps) {
  return (
    <h2
      className={cn(
        'text-2xl font-bold text-slate-800 dark:text-slate-100 mb-4',
        className
      )}
    >
      {children}
    </h2>
  );
}
